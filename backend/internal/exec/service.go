package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrForbidden     = errors.New("forbidden")
	ErrBadTransition = errors.New("transition is not allowed")
	ErrNotOwner      = errors.New("task is not leased by this worker")
	ErrInvalid       = errors.New("invalid input")
)

// Notifier будит хосты и интерфейсы пользователя: «у задач AiCrew что-то изменилось».
type Notifier interface {
	NotifyExec(userIDs []uuid.UUID)
}

type Service struct {
	pool     *pgxpool.Pool
	notifier Notifier
	log      *slog.Logger
	now      func() time.Time
}

func NewService(pool *pgxpool.Pool, notifier Notifier, log *slog.Logger) *Service {
	return &Service{pool: pool, notifier: notifier, log: log, now: time.Now}
}

// Identity — кто обращается: пользователь с устройства (хост — тоже устройство).
type Identity struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
}

func (s *Service) notify(userIDs ...uuid.UUID) {
	if s.notifier != nil && len(userIDs) > 0 {
		s.notifier.NotifyExec(userIDs)
	}
}

func event(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, from *string, to, actor, reason string, attempt *int) error {
	_, err := tx.Exec(ctx, `INSERT INTO exec.task_events (task_id, from_status, to_status, actor, reason, attempt)
		VALUES ($1, $2, $3, $4, $5, $6)`, taskID, from, to, actor, reason, attempt)
	return err
}

func strp(s string) *string { return &s }

// ── хосты и воркеры ──

type WorkerSpec struct {
	Kind  string `json:"kind"`
	Model string `json:"model"`
}

type Host struct {
	ID      uuid.UUID         `json:"id"`
	Workers map[string]string `json:"workers"` // "kind/model" → worker_id
}

// RegisterHost: хост — это устройство пользователя. Повторная регистрация обновляет данные.
func (s *Service) RegisterHost(ctx context.Context, id Identity, name, os string, caps, envs []string, workers []WorkerSpec) (Host, error) {
	if strings.TrimSpace(name) == "" || len(workers) == 0 {
		return Host{}, ErrInvalid
	}
	capsJSON, _ := json.Marshal(nonNil(caps))
	envsJSON, _ := json.Marshal(nonNil(envs))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Host{}, err
	}
	defer tx.Rollback(ctx)
	var hostID uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO exec.hosts (id, user_id, device_id, name, os, capabilities, environments, status, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'online', now())
		ON CONFLICT (device_id) DO UPDATE SET name = $4, os = $5, capabilities = $6, environments = $7,
			status = 'online', last_seen_at = now()
		WHERE exec.hosts.user_id = $2
		RETURNING id`, uuid.Must(uuid.NewV7()), id.UserID, id.DeviceID, name, os, capsJSON, envsJSON).Scan(&hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Host{}, ErrForbidden
	}
	if err != nil {
		return Host{}, err
	}
	h := Host{ID: hostID, Workers: map[string]string{}}
	for _, w := range workers {
		var wid uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO exec.workers (id, host_id, kind, model, status, heartbeat_at)
			VALUES ($1, $2, $3, $4, 'idle', now())
			ON CONFLICT (host_id, kind, model) DO UPDATE SET status = 'idle', heartbeat_at = now()
			RETURNING id`, uuid.Must(uuid.NewV7()), hostID, w.Kind, w.Model).Scan(&wid); err != nil {
			return Host{}, err
		}
		h.Workers[w.Kind+"/"+w.Model] = wid.String()
	}
	return h, tx.Commit(ctx)
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// workerOf проверяет, что воркер принадлежит хосту этого устройства.
func workerOf(ctx context.Context, tx pgx.Tx, id Identity, workerID uuid.UUID) (hostID uuid.UUID, err error) {
	err = tx.QueryRow(ctx, `SELECT h.id FROM exec.workers w JOIN exec.hosts h ON h.id = w.host_id
		WHERE w.id = $1 AND h.user_id = $2 AND h.device_id = $3`, workerID, id.UserID, id.DeviceID).Scan(&hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrForbidden
	}
	return hostID, err
}

// ── репозитории ──

type Repository struct {
	ID                  uuid.UUID       `json:"id"`
	ProjectID           uuid.UUID       `json:"project_id"`
	HostID              uuid.UUID       `json:"host_id"`
	Name                string          `json:"name"`
	LocalPath           string          `json:"local_path"`
	DefaultBranch       string          `json:"default_branch"`
	IntegrationBranch   string          `json:"integration_branch"`
	VerificationProfile json.RawMessage `json:"verification_profile"`
}

func requireProjectWrite(ctx context.Context, q pgx.Tx, userID, projectID uuid.UUID) error {
	var role string
	err := q.QueryRow(ctx, `SELECT role FROM core.project_access WHERE project_id = $1 AND user_id = $2`, projectID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role != "owner" && role != "editor" {
		return ErrForbidden
	}
	return nil
}

func (s *Service) CreateRepository(ctx context.Context, userID uuid.UUID, r Repository) (Repository, error) {
	if r.Name == "" || r.LocalPath == "" {
		return r, ErrInvalid
	}
	if r.DefaultBranch == "" {
		r.DefaultBranch = "main"
	}
	if r.IntegrationBranch == "" {
		r.IntegrationBranch = "aicrew/integration"
	}
	if r.IntegrationBranch == r.DefaultBranch {
		return r, ErrInvalid // AiCrew не пишет в основную ветку (7.3)
	}
	if len(r.VerificationProfile) == 0 {
		r.VerificationProfile = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(ctx)
	if err := requireProjectWrite(ctx, tx, userID, r.ProjectID); err != nil {
		return r, err
	}
	var owner uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT user_id FROM exec.hosts WHERE id = $1`, r.HostID).Scan(&owner); err != nil || owner != userID {
		return r, ErrNotFound
	}
	r.ID = uuid.Must(uuid.NewV7())
	if _, err := tx.Exec(ctx, `INSERT INTO exec.repositories (id, project_id, host_id, name, local_path, default_branch,
			integration_branch, verification_profile)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, r.ID, r.ProjectID, r.HostID, r.Name, r.LocalPath, r.DefaultBranch,
		r.IntegrationBranch, r.VerificationProfile); err != nil {
		return r, err
	}
	return r, tx.Commit(ctx)
}

// ── задачи ──

type NewTask struct {
	NodeID               *uuid.UUID  `json:"node_id"`
	RepositoryID         uuid.UUID   `json:"repository_id"`
	Title                string      `json:"title"`
	Prompt               string      `json:"prompt"` // пусто — взять итоговую задачу из узла
	ExecutionMode        string      `json:"execution_mode"`
	ExecutionEnv         string      `json:"execution_env"`
	RequiredCapabilities []string    `json:"required_capabilities"`
	DependsOn            []uuid.UUID `json:"depends_on"`
	MaxAttempts          int         `json:"max_attempts"`
}

// finalTaskText — снимок итоговой задачи: роль + структура, если она есть, иначе исходник.
func finalTaskText(ctx context.Context, tx pgx.Tx, nodeID uuid.UUID) (title, text string, projectID uuid.UUID, err error) {
	var raw string
	var structured []byte
	err = tx.QueryRow(ctx, `SELECT n.name, n.project_id, c.raw_content, c.structured_content
		FROM tree.nodes n JOIN tree.node_content c ON c.node_id = n.id WHERE n.id = $1 AND n.kind = 'ai_task'`, nodeID).
		Scan(&title, &projectID, &raw, &structured)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", uuid.Nil, ErrNotFound
	}
	if err != nil {
		return
	}
	text = raw
	if len(structured) > 0 {
		var d struct {
			Role          string `json:"role"`
			FormattedText string `json:"formatted_text"`
		}
		if json.Unmarshal(structured, &d) == nil && strings.TrimSpace(d.FormattedText) != "" {
			text = strings.TrimSpace(d.Role + "\n\n" + d.FormattedText)
		}
	}
	return
}

func (s *Service) CreateTask(ctx context.Context, userID uuid.UUID, in NewTask) (uuid.UUID, error) {
	if in.ExecutionMode == "" {
		in.ExecutionMode = "manual"
	}
	if in.ExecutionEnv == "" {
		in.ExecutionEnv = "container"
	}
	if in.MaxAttempts <= 0 {
		in.MaxAttempts = 3
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var repoProject uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM exec.repositories WHERE id = $1`, in.RepositoryID).Scan(&repoProject); err != nil {
		return uuid.Nil, ErrNotFound
	}
	if err := requireProjectWrite(ctx, tx, userID, repoProject); err != nil {
		return uuid.Nil, err
	}
	title, prompt := strings.TrimSpace(in.Title), strings.TrimSpace(in.Prompt)
	if in.NodeID != nil {
		nodeTitle, text, nodeProject, err := finalTaskText(ctx, tx, *in.NodeID)
		if err != nil {
			return uuid.Nil, err
		}
		if nodeProject != repoProject {
			return uuid.Nil, ErrInvalid
		}
		if title == "" {
			title = nodeTitle
		}
		if prompt == "" {
			prompt = text
		}
	}
	if title == "" || prompt == "" {
		return uuid.Nil, ErrInvalid
	}
	caps, _ := json.Marshal(nonNil(in.RequiredCapabilities))
	id := uuid.Must(uuid.NewV7())
	if _, err := tx.Exec(ctx, `INSERT INTO exec.tasks (id, project_id, node_id, repository_id, created_by, title, prompt, status,
			execution_mode, execution_env, required_capabilities, max_attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'QUEUED', $8, $9, $10, $11)`,
		id, repoProject, in.NodeID, in.RepositoryID, userID, title, prompt, in.ExecutionMode, in.ExecutionEnv, caps, in.MaxAttempts); err != nil {
		if strings.Contains(err.Error(), "check constraint") {
			return uuid.Nil, ErrInvalid
		}
		return uuid.Nil, err
	}
	for _, dep := range in.DependsOn {
		if _, err := tx.Exec(ctx, `INSERT INTO exec.task_dependencies (task_id, depends_on_task_id)
			SELECT $1, id FROM exec.tasks WHERE id = $2 AND project_id = $3`, id, dep, repoProject); err != nil {
			return uuid.Nil, err
		}
	}
	if err := event(ctx, tx, id, nil, StatusQueued, "user:"+userID.String(), "created", nil); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	s.notify(userID)
	return id, nil
}

// ClaimedTask — всё, что нужно хосту для выполнения.
type ClaimedTask struct {
	// Kind — что делать: execute (агент и проверка), merge (слить в integration), rollback (откатить слияние).
	Kind             string         `json:"kind"`
	ID               uuid.UUID      `json:"id"`
	Attempt          int            `json:"attempt"`
	Title            string         `json:"title"`
	Prompt           string         `json:"prompt"`
	ExecutionEnv     string         `json:"execution_env"`
	LeaseExpiresAt   time.Time      `json:"lease_expires_at"`
	AgentRuntimeSec  int            `json:"agent_runtime_sec"`
	VerifyRuntimeSec int            `json:"verification_runtime_sec"`
	Repository       Repository     `json:"repository"`
	PreviousAttempts []AttemptFacts `json:"previous_attempts"`
	MergeCommit      string         `json:"merge_commit,omitempty"` // для rollback
}

const (
	KindExecute  = "execute"
	KindMerge    = "merge"
	KindRollback = "rollback"
)

type AttemptFacts struct {
	Attempt      int             `json:"attempt"`
	Status       string          `json:"status"`
	FailureClass *string         `json:"failure_class"`
	FailureCode  *string         `json:"failure_code"`
	Facts        json.RawMessage `json:"facts"`
}

// Claim атомарно берёт одну задачу: строка блокируется FOR UPDATE SKIP LOCKED,
// поэтому два воркера не возьмут одну задачу. Учитываются зависимости, retry_after,
// окружения и возможности хоста.
func (s *Service) Claim(ctx context.Context, id Identity, workerID uuid.UUID) (*ClaimedTask, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	hostID, err := workerOf(ctx, tx, id, workerID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.hosts SET last_seen_at = now(), status = 'online' WHERE id = $1`, hostID); err != nil {
		return nil, err
	}
	now := s.now()
	// Сначала очередь слияния: integration должна продвинуться раньше, чем начнётся следующая задача.
	job, err := s.claimMergeJob(ctx, tx, hostID, workerID, now)
	if err != nil {
		return nil, err
	}
	if job != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		s.notify(id.UserID)
		return job, nil
	}
	t := ClaimedTask{Kind: KindExecute}
	err = tx.QueryRow(ctx, `SELECT t.id, t.attempt, t.title, t.prompt, t.execution_env, t.agent_runtime_sec, t.verification_runtime_sec,
			r.id, r.project_id, r.host_id, r.name, r.local_path, r.default_branch, r.integration_branch, r.verification_profile
		FROM exec.tasks t
		JOIN exec.repositories r ON r.id = t.repository_id
		JOIN exec.hosts h ON h.id = r.host_id
		WHERE t.status = 'QUEUED' AND r.host_id = $1
		  AND (t.retry_after IS NULL OR t.retry_after <= $2)
		  AND h.environments @> to_jsonb(t.execution_env)
		  AND h.capabilities @> t.required_capabilities
		  AND NOT EXISTS (
			SELECT 1 FROM exec.task_dependencies d JOIN exec.tasks dt ON dt.id = d.depends_on_task_id
			WHERE d.task_id = t.id AND dt.status <> 'DONE')
		ORDER BY t.created_at
		LIMIT 1
		FOR UPDATE OF t SKIP LOCKED`, hostID, now).
		Scan(&t.ID, &t.Attempt, &t.Title, &t.Prompt, &t.ExecutionEnv, &t.AgentRuntimeSec, &t.VerifyRuntimeSec,
			&t.Repository.ID, &t.Repository.ProjectID, &t.Repository.HostID, &t.Repository.Name, &t.Repository.LocalPath,
			&t.Repository.DefaultBranch, &t.Repository.IntegrationBranch, &t.Repository.VerificationProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Attempt++
	t.LeaseExpiresAt = now.Add(LeaseTTL)
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = 'CLAIMED', worker_id = $2, attempt = $3,
			lease_expires_at = $4, heartbeat_at = $5, retry_after = NULL, failure_code = NULL, failure_class = NULL,
			retryable = NULL, updated_at = now()
		WHERE id = $1`, t.ID, workerID, t.Attempt, t.LeaseExpiresAt, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.attempts (id, task_id, attempt, worker_id, status)
		VALUES ($1, $2, $3, $4, 'CLAIMED')`, uuid.Must(uuid.NewV7()), t.ID, t.Attempt, workerID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.workers SET status = 'busy', heartbeat_at = now() WHERE id = $1`, workerID); err != nil {
		return nil, err
	}
	if err := event(ctx, tx, t.ID, strp(StatusQueued), StatusClaimed, "worker:"+workerID.String(), "", &t.Attempt); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT attempt, status, failure_class, failure_code, facts FROM exec.attempts
		WHERE task_id = $1 AND attempt < $2 ORDER BY attempt`, t.ID, t.Attempt)
	if err != nil {
		return nil, err
	}
	t.PreviousAttempts, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (AttemptFacts, error) {
		var a AttemptFacts
		err := r.Scan(&a.Attempt, &a.Status, &a.FailureClass, &a.FailureCode, &a.Facts)
		return a, err
	})
	if err != nil {
		return nil, err
	}
	if t.PreviousAttempts == nil {
		t.PreviousAttempts = []AttemptFacts{}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.notify(id.UserID)
	return &t, nil
}

// claimMergeJob берёт слияние или откат на репозитории хоста. Строка репозитория блокируется,
// поэтому integration одного репозитория меняет строго одна задача за раз.
func (s *Service) claimMergeJob(ctx context.Context, tx pgx.Tx, hostID, workerID uuid.UUID, now time.Time) (*ClaimedTask, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT t.repository_id FROM exec.tasks t JOIN exec.repositories r ON r.id = t.repository_id
		WHERE r.host_id = $1 AND t.status IN ('MERGE_QUEUED', 'ROLLBACK_QUEUED')`, hostID)
	if err != nil {
		return nil, err
	}
	repos, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, err
	}
	for _, repoID := range repos {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM exec.repositories WHERE id = $1 FOR UPDATE`, repoID); err != nil {
			return nil, err
		}
		var busy bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM exec.tasks WHERE repository_id = $1 AND status = ANY($2))`,
			repoID, busyMergeStatuses).Scan(&busy); err != nil {
			return nil, err
		}
		if busy {
			continue
		}
		var t ClaimedTask
		var status string
		var merge *string
		err := tx.QueryRow(ctx, `SELECT t.id, t.attempt, t.title, t.status, t.merge_commit, t.verification_runtime_sec,
				r.id, r.project_id, r.host_id, r.name, r.local_path, r.default_branch, r.integration_branch, r.verification_profile
			FROM exec.tasks t JOIN exec.repositories r ON r.id = t.repository_id
			WHERE t.repository_id = $1 AND t.status IN ('MERGE_QUEUED', 'ROLLBACK_QUEUED')
			ORDER BY t.updated_at LIMIT 1 FOR UPDATE OF t`, repoID).
			Scan(&t.ID, &t.Attempt, &t.Title, &status, &merge, &t.VerifyRuntimeSec,
				&t.Repository.ID, &t.Repository.ProjectID, &t.Repository.HostID, &t.Repository.Name, &t.Repository.LocalPath,
				&t.Repository.DefaultBranch, &t.Repository.IntegrationBranch, &t.Repository.VerificationProfile)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		to := StatusMerging
		t.Kind = KindMerge
		if status == StatusRollbackQueued {
			to, t.Kind = StatusRollingBack, KindRollback
			if merge != nil {
				t.MergeCommit = *merge
			}
		}
		t.LeaseExpiresAt = now.Add(LeaseTTL)
		t.PreviousAttempts = []AttemptFacts{}
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = $2, worker_id = $3, lease_expires_at = $4, heartbeat_at = $5,
				failure_code = NULL, failure_class = NULL, retryable = NULL, updated_at = now()
			WHERE id = $1`, t.ID, to, workerID, t.LeaseExpiresAt, now); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE exec.workers SET status = 'busy', heartbeat_at = now() WHERE id = $1`, workerID); err != nil {
			return nil, err
		}
		if err := event(ctx, tx, t.ID, &status, to, "worker:"+workerID.String(), "", &t.Attempt); err != nil {
			return nil, err
		}
		return &t, nil
	}
	return nil, nil
}

type leasedTask struct {
	status    string
	workerID  *uuid.UUID
	attempt   int
	maxAtt    int
	createdBy uuid.UUID
	cancel    bool
	mode      string
}

func lockTask(ctx context.Context, tx pgx.Tx, taskID uuid.UUID) (leasedTask, error) {
	var t leasedTask
	err := tx.QueryRow(ctx, `SELECT status, worker_id, attempt, max_attempts, created_by, cancel_requested, execution_mode
		FROM exec.tasks WHERE id = $1 FOR UPDATE`, taskID).Scan(&t.status, &t.workerID, &t.attempt, &t.maxAtt, &t.createdBy, &t.cancel, &t.mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (t leasedTask) ownedBy(workerID uuid.UUID, attempt int) bool {
	return t.workerID != nil && *t.workerID == workerID && t.attempt == attempt
}

// Heartbeat продлевает lease. Ответ говорит, просил ли пользователь отмену.
func (s *Service) Heartbeat(ctx context.Context, id Identity, taskID, workerID uuid.UUID, attempt int) (time.Time, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	defer tx.Rollback(ctx)
	if _, err := workerOf(ctx, tx, id, workerID); err != nil {
		return time.Time{}, false, err
	}
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return time.Time{}, false, err
	}
	if !t.ownedBy(workerID, attempt) || !leased(t.status) {
		return time.Time{}, false, ErrNotOwner
	}
	now := s.now()
	lease := now.Add(LeaseTTL)
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET lease_expires_at = $2, heartbeat_at = $3 WHERE id = $1`, taskID, lease, now); err != nil {
		return time.Time{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.workers SET heartbeat_at = now() WHERE id = $1`, workerID); err != nil {
		return time.Time{}, false, err
	}
	return lease, t.cancel || t.status == StatusCancelling, tx.Commit(ctx)
}

func leased(status string) bool {
	for _, s := range leasedStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// Facts — машинные факты от хоста (ТЗ п. 7: собирает код, а не модель).
type Facts struct {
	BaseCommit    string          `json:"base_commit,omitempty"`
	ResultCommit  string          `json:"result_commit,omitempty"`
	MergeCommit   string          `json:"merge_commit,omitempty"`
	RevertCommit  string          `json:"revert_commit,omitempty"`
	FailureCode   string          `json:"failure_code,omitempty"`
	FailureClass  string          `json:"failure_class,omitempty"`
	RetryAfterSec int             `json:"retry_after_sec,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	Details       json.RawMessage `json:"details,omitempty"` // exit-коды, файлы, diff --stat, расход, итог агента
}

// Transition — воркер просит переход. Сервер проверяет владельца и таблицу переходов;
// FAILED сразу обрабатывается политикой повторов.
func (s *Service) Transition(ctx context.Context, id Identity, taskID, workerID uuid.UUID, attempt int, to string, f Facts) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err := workerOf(ctx, tx, id, workerID); err != nil {
		return "", err
	}
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return "", err
	}
	if !t.ownedBy(workerID, attempt) {
		return "", ErrNotOwner
	}
	if !WorkerCanMove(t.status, to) {
		return "", fmt.Errorf("%w: %s → %s", ErrBadTransition, t.status, to)
	}
	// Отмена уже запрошена: воркер может лишь подтвердить её или упасть.
	if t.cancel && to != StatusCancelled && to != StatusFailed {
		return "", fmt.Errorf("%w: cancellation requested", ErrBadTransition)
	}
	actor := "worker:" + workerID.String()
	if f.BaseCommit != "" {
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET base_commit = $2 WHERE id = $1`, taskID, f.BaseCommit); err != nil {
			return "", err
		}
	}
	if f.ResultCommit != "" {
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET result_commit = $2 WHERE id = $1`, taskID, f.ResultCommit); err != nil {
			return "", err
		}
	}
	if f.MergeCommit != "" {
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET merge_commit = $2 WHERE id = $1`, taskID, f.MergeCommit); err != nil {
			return "", err
		}
	}
	if f.RevertCommit != "" {
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET revert_commit = $2 WHERE id = $1`, taskID, f.RevertCommit); err != nil {
			return "", err
		}
	}
	details := f.Details
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.attempts SET status = $3,
			base_commit = COALESCE(NULLIF($4, ''), base_commit), result_commit = COALESCE(NULLIF($5, ''), result_commit),
			facts = facts || $6::jsonb
		WHERE task_id = $1 AND attempt = $2`, taskID, attempt, to, f.BaseCommit, f.ResultCommit, details); err != nil {
		return "", err
	}

	final := to
	switch {
	case to == StatusFailed && t.status == StatusRollingBack:
		// Откат не удался: integration не тронута, задача остаётся слитой (DONE) с пометкой.
		if !ValidClass(f.FailureClass) {
			return "", fmt.Errorf("%w: unknown failure class %q", ErrInvalid, f.FailureClass)
		}
		if err := s.release(ctx, tx, taskID, StatusDone, workerID); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET failure_class = $2, failure_code = 'rollback_failed' WHERE id = $1`,
			taskID, f.FailureClass); err != nil {
			return "", err
		}
		if err := event(ctx, tx, taskID, &t.status, StatusDone, actor, "rollback failed: "+f.FailureClass+": "+f.Reason, &attempt); err != nil {
			return "", err
		}
		final = StatusDone
	case to == StatusFailed:
		if !ValidClass(f.FailureClass) {
			return "", fmt.Errorf("%w: unknown failure class %q", ErrInvalid, f.FailureClass)
		}
		var ra *time.Duration
		if f.RetryAfterSec > 0 {
			d := time.Duration(f.RetryAfterSec) * time.Second
			ra = &d
		}
		final, err = s.fail(ctx, tx, taskID, t, f.FailureClass, f.FailureCode, ra, actor, f.Reason)
		if err != nil {
			return "", err
		}
	case to == StatusCancelled:
		if err := s.release(ctx, tx, taskID, StatusCancelled, workerID); err != nil {
			return "", err
		}
		if err := event(ctx, tx, taskID, &t.status, StatusCancelled, actor, f.Reason, &attempt); err != nil {
			return "", err
		}
	default:
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = $2, updated_at = now() WHERE id = $1`, taskID, to); err != nil {
			return "", err
		}
		if !leased(to) {
			// Задача вышла из-под lease (MERGEABLE и далее): воркер свободен.
			if err := s.release(ctx, tx, taskID, to, workerID); err != nil {
				return "", err
			}
		}
		if err := event(ctx, tx, taskID, &t.status, to, actor, f.Reason, &attempt); err != nil {
			return "", err
		}
		// semi_auto и night: проверенный результат сам встаёт в очередь слияния; manual ждёт «Слить».
		if to == StatusMergeable && t.mode != "manual" {
			if err := s.queueMerge(ctx, tx, taskID, "system", "auto merge: "+t.mode); err != nil {
				return "", err
			}
			final = StatusMergeQueued
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	s.notify(id.UserID)
	return final, nil
}

func (s *Service) release(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, status string, workerID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = $2, lease_expires_at = NULL, updated_at = now() WHERE id = $1`,
		taskID, status); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.attempts SET finished_at = COALESCE(finished_at, now())
		WHERE task_id = $1 AND finished_at IS NULL`, taskID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE exec.workers SET status = 'idle' WHERE id = $1`, workerID)
	return err
}

// fail записывает FAILED и сразу применяет политику класса.
func (s *Service) fail(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, t leasedTask, class, code string,
	retryAfter *time.Duration, actor, reason string) (string, error) {
	if code == "" {
		code = strings.ToLower(class)
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.attempts SET status = 'FAILED', failure_class = $3, failure_code = $4
		WHERE task_id = $1 AND attempt = $2`, taskID, t.attempt, class, code); err != nil {
		return "", err
	}
	if err := event(ctx, tx, taskID, &t.status, StatusFailed, actor, class+": "+reason, &t.attempt); err != nil {
		return "", err
	}
	var workerID uuid.UUID
	if t.workerID != nil {
		workerID = *t.workerID
	}
	if err := s.release(ctx, tx, taskID, StatusFailed, workerID); err != nil {
		return "", err
	}
	d := decideRetry(class, t.attempt, t.maxAtt, retryAfter)
	if t.cancel {
		d = retryDecision{status: StatusCancelled}
	}
	var after *time.Time
	if d.delay > 0 {
		at := s.now().Add(d.delay)
		after = &at
	}
	// RATE_LIMIT не вина задачи: номер попытки уже занят, поэтому добавляем ещё одну к лимиту.
	extra := 0
	if d.status == StatusQueued && !d.spendAttempt {
		extra = 1
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = $2, failure_class = $3, failure_code = $4, retryable = $5,
			retry_after = $6, worker_id = CASE WHEN $2 = 'QUEUED' THEN NULL ELSE worker_id END,
			max_attempts = max_attempts + $7, updated_at = now()
		WHERE id = $1`, taskID, d.status, class, code, d.status == StatusQueued, after, extra); err != nil {
		return "", err
	}
	if err := event(ctx, tx, taskID, strp(StatusFailed), d.status, "system", "retry policy: "+class, &t.attempt); err != nil {
		return "", err
	}
	return d.status, nil
}

// ReapExpired — сборщик: задачи с истёкшим lease (воркер умер, ПК выключили).
func (s *Service) ReapExpired(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM exec.tasks WHERE lease_expires_at < $1 AND status = ANY($2)`,
		s.now(), leasedStatuses)
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return 0, err
	}
	n := 0
	for _, taskID := range ids {
		ok, err := s.reapOne(ctx, taskID)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

func (s *Service) reapOne(ctx context.Context, taskID uuid.UUID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return false, err
	}
	var lease *time.Time
	if err := tx.QueryRow(ctx, `SELECT lease_expires_at FROM exec.tasks WHERE id = $1`, taskID).Scan(&lease); err != nil {
		return false, err
	}
	if lease == nil || !lease.Before(s.now()) || !leased(t.status) {
		return false, nil // heartbeat успел
	}
	if back := requeueAfterLostLease(t.status); back != "" {
		// Слияние или откат прерваны до сдвига integration: просто ставим обратно в очередь.
		var w uuid.UUID
		if t.workerID != nil {
			w = *t.workerID
		}
		if err := s.release(ctx, tx, taskID, back, w); err != nil {
			return false, err
		}
		if err := event(ctx, tx, taskID, &t.status, back, "system", "lease expired during merge queue work", &t.attempt); err != nil {
			return false, err
		}
	} else if t.status == StatusCancelling {
		var w uuid.UUID
		if t.workerID != nil {
			w = *t.workerID
		}
		if err := s.release(ctx, tx, taskID, StatusCancelled, w); err != nil {
			return false, err
		}
		if err := event(ctx, tx, taskID, &t.status, StatusCancelled, "system", "lease expired during cancellation", &t.attempt); err != nil {
			return false, err
		}
	} else if _, err := s.fail(ctx, tx, taskID, t, ClassEnvironment, "lease_expired", nil, "system", "worker stopped sending heartbeats"); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	s.notify(t.createdBy)
	return true, nil
}

func requeueAfterLostLease(status string) string {
	switch status {
	case StatusMerging, StatusPostMergeVerify:
		return StatusMergeQueued
	case StatusRollingBack:
		return StatusRollbackQueued
	}
	return ""
}

// RunReaper запускает сборщик раз в interval до отмены контекста.
func (s *Service) RunReaper(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if n, err := s.ReapExpired(ctx); err != nil {
				s.log.Error("exec reaper", "err", err)
			} else if n > 0 {
				s.log.Warn("exec reaper: leases expired", "tasks", n)
			}
		}
	}
}

// Cancel — пользователь отменяет задачу. Не взятая задача отменяется сразу,
// взятая — переходит в CANCELLING, и хост узнаёт об этом из heartbeat.
func (s *Service) Cancel(ctx context.Context, userID, taskID uuid.UUID) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return "", err
	}
	if err := s.requireTaskWrite(ctx, tx, userID, taskID); err != nil {
		return "", err
	}
	switch {
	case IsFinal(t.status), t.status == StatusMerged:
		return t.status, nil
	case t.status == StatusRollingBack:
		return "", fmt.Errorf("%w: rollback is already running", ErrBadTransition)
	case t.status == StatusRollbackQueued:
		// Отмена отката: задача остаётся слитой.
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = 'DONE', updated_at = now() WHERE id = $1`, taskID); err != nil {
			return "", err
		}
		if err := event(ctx, tx, taskID, &t.status, StatusDone, "user:"+userID.String(), "rollback cancelled by user", nil); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		s.notify(userID)
		return StatusDone, nil
	}
	to := StatusCancelling
	if !leased(t.status) {
		to = StatusCancelled // QUEUED, FAILED, AWAITING_HUMAN, MERGEABLE…: воркера нет
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = $2, cancel_requested = true, updated_at = now() WHERE id = $1`,
		taskID, to); err != nil {
		return "", err
	}
	if to == StatusCancelled {
		if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET lease_expires_at = NULL, worker_id = NULL WHERE id = $1`, taskID); err != nil {
			return "", err
		}
	}
	if err := event(ctx, tx, taskID, &t.status, to, "user:"+userID.String(), "cancelled by user", nil); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	s.notify(userID)
	return to, nil
}

// Retry — пользователь возвращает задачу из AWAITING_HUMAN / BLOCKED / REPLAN в очередь с одной новой попыткой.
func (s *Service) Retry(ctx context.Context, userID, taskID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return err
	}
	if err := s.requireTaskWrite(ctx, tx, userID, taskID); err != nil {
		return err
	}
	if t.status != StatusAwaitingHuman && t.status != StatusBlocked && t.status != StatusReplan {
		return fmt.Errorf("%w: %s → %s", ErrBadTransition, t.status, StatusQueued)
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = 'QUEUED', max_attempts = GREATEST(max_attempts, attempt + 1),
			retry_after = NULL, worker_id = NULL, updated_at = now() WHERE id = $1`, taskID); err != nil {
		return err
	}
	if err := event(ctx, tx, taskID, &t.status, StatusQueued, "user:"+userID.String(), "retry by user", nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

func (s *Service) queueMerge(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, actor, reason string) error {
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = 'MERGE_QUEUED', updated_at = now() WHERE id = $1`, taskID); err != nil {
		return err
	}
	return event(ctx, tx, taskID, strp(StatusMergeable), StatusMergeQueued, actor, reason, nil)
}

// Merge — пользователь одобряет слияние проверенной задачи в integration-ветку (режим manual).
func (s *Service) Merge(ctx context.Context, userID, taskID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return err
	}
	if err := s.requireTaskWrite(ctx, tx, userID, taskID); err != nil {
		return err
	}
	if t.status != StatusMergeable {
		return fmt.Errorf("%w: %s → %s", ErrBadTransition, t.status, StatusMergeQueued)
	}
	if err := s.queueMerge(ctx, tx, taskID, "user:"+userID.String(), "merge approved"); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

// Rollback — пользователь откатывает слитую задачу. Откат идёт через очередь слияния
// с проверкой. Нельзя откатить задачу, на которую опираются уже слитые задачи.
func (s *Service) Rollback(ctx context.Context, userID, taskID uuid.UUID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	t, err := lockTask(ctx, tx, taskID)
	if err != nil {
		return err
	}
	if err := s.requireTaskWrite(ctx, tx, userID, taskID); err != nil {
		return err
	}
	if t.status != StatusDone {
		return fmt.Errorf("%w: %s → %s", ErrBadTransition, t.status, StatusRollbackQueued)
	}
	var merge *string
	if err := tx.QueryRow(ctx, `SELECT merge_commit FROM exec.tasks WHERE id = $1`, taskID).Scan(&merge); err != nil {
		return err
	}
	if merge == nil || *merge == "" {
		return fmt.Errorf("%w: task was not merged by the queue", ErrBadTransition)
	}
	rows, err := tx.Query(ctx, `SELECT t.title FROM exec.task_dependencies d JOIN exec.tasks t ON t.id = d.task_id
		WHERE d.depends_on_task_id = $1 AND t.status IN ('MERGE_QUEUED', 'MERGING', 'POST_MERGE_VERIFY', 'MERGED', 'DONE',
			'ROLLBACK_QUEUED', 'ROLLING_BACK') ORDER BY t.created_at`, taskID)
	if err != nil {
		return err
	}
	dependents, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(dependents) > 0 {
		return fmt.Errorf("%w: roll back dependent tasks first: %s", ErrBadTransition, strings.Join(dependents, "; "))
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = 'ROLLBACK_QUEUED', failure_class = NULL, failure_code = NULL,
			updated_at = now() WHERE id = $1`, taskID); err != nil {
		return err
	}
	if reason == "" {
		reason = "rollback requested"
	}
	if err := event(ctx, tx, taskID, &t.status, StatusRollbackQueued, "user:"+userID.String(), reason, nil); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

func (s *Service) requireTaskWrite(ctx context.Context, tx pgx.Tx, userID, taskID uuid.UUID) error {
	var projectID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM exec.tasks WHERE id = $1`, taskID).Scan(&projectID); err != nil {
		return ErrNotFound
	}
	return requireProjectWrite(ctx, tx, userID, projectID)
}

// ── чтение для интерфейса и хоста ──

type TaskView struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	NodeID       *uuid.UUID `json:"node_id"`
	RepositoryID uuid.UUID  `json:"repository_id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Attempt      int        `json:"attempt"`
	MaxAttempts  int        `json:"max_attempts"`
	FailureClass *string    `json:"failure_class"`
	FailureCode  *string    `json:"failure_code"`
	BaseCommit   *string    `json:"base_commit"`
	ResultCommit *string    `json:"result_commit"`
	MergeCommit  *string    `json:"merge_commit"`
	Mode         string     `json:"execution_mode"`
	RetryAfter   *time.Time `json:"retry_after"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (s *Service) ListTasks(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) ([]TaskView, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.id, t.project_id, t.node_id, t.repository_id, t.title, t.status, t.attempt,
			t.max_attempts, t.failure_class, t.failure_code, t.base_commit, t.result_commit, t.merge_commit, t.execution_mode,
			t.retry_after, t.updated_at
		FROM exec.tasks t JOIN core.project_access a ON a.project_id = t.project_id AND a.user_id = $1
		WHERE ($2::uuid IS NULL OR t.project_id = $2) ORDER BY t.updated_at DESC LIMIT 500`, userID, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (TaskView, error) {
		var v TaskView
		err := r.Scan(&v.ID, &v.ProjectID, &v.NodeID, &v.RepositoryID, &v.Title, &v.Status, &v.Attempt, &v.MaxAttempts,
			&v.FailureClass, &v.FailureCode, &v.BaseCommit, &v.ResultCommit, &v.MergeCommit, &v.Mode, &v.RetryAfter, &v.UpdatedAt)
		return v, err
	})
}

func (s *Service) ListRepositories(ctx context.Context, userID uuid.UUID) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.project_id, r.host_id, r.name, r.local_path, r.default_branch, r.integration_branch,
			r.verification_profile
		FROM exec.repositories r JOIN core.project_access a ON a.project_id = r.project_id AND a.user_id = $1
		ORDER BY r.created_at`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Repository, error) {
		var v Repository
		err := r.Scan(&v.ID, &v.ProjectID, &v.HostID, &v.Name, &v.LocalPath, &v.DefaultBranch, &v.IntegrationBranch, &v.VerificationProfile)
		return v, err
	})
}

// KeepWorktrees — задачи на репозиториях хоста этого устройства, чьи worktree нужны:
// под lease, ждут слияния или сливаются. Остальные worktree хост удаляет при старте (ТЗ п. 8.2).
func (s *Service) KeepWorktrees(ctx context.Context, id Identity) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.id FROM exec.tasks t
		JOIN exec.repositories r ON r.id = t.repository_id JOIN exec.hosts h ON h.id = r.host_id
		WHERE h.user_id = $1 AND h.device_id = $2 AND t.status = ANY($3)`, id.UserID, id.DeviceID,
		append(append([]string{}, leasedStatuses...), StatusMergeable, StatusMergeQueued, StatusMerged))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}
