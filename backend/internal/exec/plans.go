package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Планы Manager'а (ТЗ п. 10.1, docs/stage7-5-manager-skills-mcp.md, раздел 1):
// Manager предлагает декомпозицию, сервер проверяет её по спискам хоста и применяет.

const (
	PlanQueued           = "QUEUED"
	PlanPlanning         = "PLANNING"
	PlanAwaitingApproval = "AWAITING_APPROVAL"
	PlanApproved         = "APPROVED"
	PlanRejected         = "REJECTED"
	PlanFailed           = "FAILED"

	MaxPlanTasks    = 12
	PlanLeaseTTL    = 15 * time.Minute // Manager работает минуты, heartbeat у плана нет
	maxPlanAttempts = 2
)

type NewPlan struct {
	RepositoryID  uuid.UUID  `json:"repository_id"`
	NodeID        *uuid.UUID `json:"node_id"`
	Request       string     `json:"request"`
	ExecutionMode string     `json:"execution_mode"`
}

// PlanTask — одна задача, предложенная Manager'ом. DependsOn — индексы задач этого же плана.
type PlanTask struct {
	Title                string   `json:"title"`
	Prompt               string   `json:"prompt"`
	DependsOn            []int    `json:"depends_on"`
	RequiredCapabilities []string `json:"required_capabilities"`
	ExecutionEnv         string   `json:"execution_env"`
	ReviewLevel          string   `json:"review_level"`
	TouchesUI            bool     `json:"touches_ui"`
	RequiredSkills       []string `json:"required_skills"`
	MCPProfile           string   `json:"mcp_profile"`
	Executor             string   `json:"executor"`
	// Contract — обязателен для executor=junior: Junior делает только это.
	Contract *Contract `json:"contract,omitempty"`
}

// Contract — граница работы Junior. Тесты пишет задача Claude, от которой Junior зависит, и помечает их
// пропущенными с причиной ContractMarker; Junior реализует код и снимает пропуск. Хост проверяет и то, и другое.
type Contract struct {
	Files []string `json:"files"` // что Junior может менять (кроме тестов)
	Tests []string `json:"tests"` // файлы с тестами-контрактом
}

// ContractMarker — причина пропуска тестов-контракта (skip: 'aicrew-contract', t.Skip("aicrew-contract")).
const ContractMarker = "aicrew-contract"

func cleanContractPath(p string) (string, bool) {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, ":") || strings.Contains("/"+p+"/", "/../") || strings.HasPrefix(p, ".git/") {
		return "", false
	}
	return strings.TrimPrefix(p, "./"), true
}

type Proposal struct {
	Tasks     []PlanTask `json:"tasks"`
	Rationale string     `json:"rationale"`
}

type PlanView struct {
	ID            uuid.UUID       `json:"id"`
	RepositoryID  uuid.UUID       `json:"repository_id"`
	Request       string          `json:"request"`
	ExecutionMode string          `json:"execution_mode"`
	Status        string          `json:"status"`
	Attempt       int             `json:"attempt"`
	Proposal      json.RawMessage `json:"proposal"`
	Error         string          `json:"error"`
	TaskIDs       []uuid.UUID     `json:"task_ids"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (s *Service) CreatePlan(ctx context.Context, userID uuid.UUID, in NewPlan) (uuid.UUID, error) {
	if in.ExecutionMode == "" {
		in.ExecutionMode = "manual"
	}
	in.Request = strings.TrimSpace(in.Request)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var project uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM exec.repositories WHERE id = $1`, in.RepositoryID).Scan(&project); err != nil {
		return uuid.Nil, ErrNotFound
	}
	if err := requireProjectWrite(ctx, tx, userID, project); err != nil {
		return uuid.Nil, err
	}
	if in.Request == "" && in.NodeID != nil {
		_, text, nodeProject, err := finalTaskText(ctx, tx, *in.NodeID)
		if err != nil {
			return uuid.Nil, err
		}
		if nodeProject != project {
			return uuid.Nil, ErrInvalid
		}
		in.Request = text
	}
	if in.Request == "" || utf8.RuneCountInString(in.Request) > 50000 {
		return uuid.Nil, ErrInvalid
	}
	id := uuid.Must(uuid.NewV7())
	if _, err := tx.Exec(ctx, `INSERT INTO exec.plans (id, project_id, repository_id, node_id, created_by, request, execution_mode, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'QUEUED')`, id, project, in.RepositoryID, in.NodeID, userID, in.Request, in.ExecutionMode); err != nil {
		if strings.Contains(err.Error(), "check constraint") {
			return uuid.Nil, ErrInvalid
		}
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	s.notify(userID)
	return id, nil
}

// claimPlan отдаёт хосту план на декомпозицию (вид работы plan). Attempt — номер попытки Manager'а.
func (s *Service) claimPlan(ctx context.Context, tx pgx.Tx, hostID, workerID uuid.UUID, now time.Time) (*ClaimedTask, error) {
	t := ClaimedTask{Kind: KindPlan, PreviousAttempts: []AttemptFacts{}}
	err := tx.QueryRow(ctx, `SELECT p.id, p.attempt, p.request,
			r.id, r.project_id, r.host_id, r.name, r.local_path, r.default_branch, r.integration_branch, r.verification_profile
		FROM exec.plans p JOIN exec.repositories r ON r.id = p.repository_id JOIN exec.hosts h ON h.id = r.host_id
		WHERE p.status = 'QUEUED' AND r.host_id = $1 AND `+pausedSQL+`
		ORDER BY p.created_at LIMIT 1 FOR UPDATE OF p SKIP LOCKED`, hostID).
		Scan(&t.ID, &t.Attempt, &t.Prompt, &t.Repository.ID, &t.Repository.ProjectID, &t.Repository.HostID, &t.Repository.Name,
			&t.Repository.LocalPath, &t.Repository.DefaultBranch, &t.Repository.IntegrationBranch, &t.Repository.VerificationProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Attempt++
	t.Title = "plan"
	t.LeaseExpiresAt = now.Add(PlanLeaseTTL)
	_, err = tx.Exec(ctx, `UPDATE exec.plans SET status = 'PLANNING', worker_id = $2, attempt = $3, lease_expires_at = $4,
		updated_at = now() WHERE id = $1`, t.ID, workerID, t.Attempt, t.LeaseExpiresAt)
	return &t, err
}

type lockedPlan struct {
	status, mode    string
	workerID        *uuid.UUID
	attempt         int
	project, repo   uuid.UUID
	createdBy       uuid.UUID
	proposal        []byte
	leaseExpiresAt  *time.Time
	requestedByNode *uuid.UUID
}

func lockPlan(ctx context.Context, tx pgx.Tx, planID uuid.UUID) (lockedPlan, error) {
	var p lockedPlan
	err := tx.QueryRow(ctx, `SELECT status, execution_mode, worker_id, attempt, project_id, repository_id, created_by, proposal,
			lease_expires_at, node_id
		FROM exec.plans WHERE id = $1 FOR UPDATE`, planID).
		Scan(&p.status, &p.mode, &p.workerID, &p.attempt, &p.project, &p.repo, &p.createdBy, &p.proposal, &p.leaseExpiresAt, &p.requestedByNode)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// hostLists — что умеет хост репозитория: по этим спискам проверяется план.
type hostLists struct {
	caps, envs, skills, mcp, executors map[string]bool
	required, forbidden                []string
}

func loadHostLists(ctx context.Context, tx pgx.Tx, repoID uuid.UUID) (hostLists, error) {
	var caps, envs, skills, mcp, executors, policy []byte
	err := tx.QueryRow(ctx, `SELECT h.capabilities, h.environments, h.skills, h.mcp_profiles, h.executors, r.skill_policy
		FROM exec.repositories r JOIN exec.hosts h ON h.id = r.host_id WHERE r.id = $1`, repoID).
		Scan(&caps, &envs, &skills, &mcp, &executors, &policy)
	if err != nil {
		return hostLists{}, err
	}
	set := func(b []byte) map[string]bool {
		var xs []string
		_ = json.Unmarshal(b, &xs)
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	l := hostLists{caps: set(caps), envs: set(envs), skills: set(skills), mcp: set(mcp), executors: set(executors)}
	l.mcp["default"] = true
	var pol struct {
		Required  []string `json:"required"`
		Forbidden []string `json:"forbidden"`
	}
	_ = json.Unmarshal(policy, &pol)
	l.required, l.forbidden = pol.Required, pol.Forbidden
	return l, nil
}

// ValidateProposal — проверка плана кодом: граф, лимиты, списки хоста, политика скиллов.
// Применяет умолчания и обязательные скиллы репозитория.
func ValidateProposal(p *Proposal, l hostLists) error {
	n := len(p.Tasks)
	if n == 0 || n > MaxPlanTasks {
		return fmt.Errorf("plan must have 1..%d tasks, got %d", MaxPlanTasks, n)
	}
	forbidden := map[string]bool{}
	for _, f := range l.forbidden {
		forbidden[f] = true
	}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		t.Title, t.Prompt = strings.TrimSpace(t.Title), strings.TrimSpace(t.Prompt)
		if t.Title == "" || utf8.RuneCountInString(t.Title) > 200 {
			return fmt.Errorf("task %d: title must be 1..200 characters", i)
		}
		if t.Prompt == "" || utf8.RuneCountInString(t.Prompt) > 20000 {
			return fmt.Errorf("task %d: prompt must be 1..20000 characters", i)
		}
		if t.ExecutionEnv == "" {
			t.ExecutionEnv = "container"
		}
		if !l.envs[t.ExecutionEnv] {
			return fmt.Errorf("task %d: environment %q is not available on the host", i, t.ExecutionEnv)
		}
		for _, c := range t.RequiredCapabilities {
			if !l.caps[c] {
				return fmt.Errorf("task %d: capability %q is not available on the host", i, c)
			}
		}
		if t.ReviewLevel == "" {
			t.ReviewLevel = "logic"
		}
		if t.ReviewLevel != "logic" && t.ReviewLevel != "tech_lead" {
			return fmt.Errorf("task %d: review_level %q is not allowed", i, t.ReviewLevel)
		}
		if t.Executor == "" {
			t.Executor = "claude"
		}
		if !l.executors[t.Executor] {
			return fmt.Errorf("task %d: executor %q is not available on the host", i, t.Executor)
		}
		if t.MCPProfile == "" {
			t.MCPProfile = "default"
		}
		if !l.mcp[t.MCPProfile] {
			return fmt.Errorf("task %d: MCP profile %q is not in the registry", i, t.MCPProfile)
		}
		seen := map[string]bool{}
		var skills []string
		for _, sk := range append(append([]string{}, l.required...), t.RequiredSkills...) {
			if seen[sk] {
				continue
			}
			seen[sk] = true
			if !l.skills[sk] {
				return fmt.Errorf("task %d: skill %q is not in the registry", i, sk)
			}
			if forbidden[sk] {
				return fmt.Errorf("task %d: skill %q is forbidden for this repository", i, sk)
			}
			skills = append(skills, sk)
		}
		t.RequiredSkills = skills
		for _, d := range t.DependsOn {
			if d < 0 || d >= n || d == i {
				return fmt.Errorf("task %d: bad dependency %d", i, d)
			}
		}
		if err := validateContract(i, t); err != nil {
			return err
		}
	}
	// Junior никогда не работает сам по себе: перед ним задача Claude с тестами, после — задача Claude, которая собирает.
	for i, t := range p.Tasks {
		if t.Executor != "junior" {
			continue
		}
		before := false
		for _, d := range t.DependsOn {
			before = before || p.Tasks[d].Executor == "claude"
		}
		after := false
		for _, o := range p.Tasks {
			if o.Executor != "claude" {
				continue
			}
			for _, d := range o.DependsOn {
				after = after || d == i
			}
		}
		if !before || !after {
			return fmt.Errorf("task %d: a junior task must depend on a claude task that writes its contract tests "+
				"and be followed by a claude task that integrates it", i)
		}
	}
	if _, err := topoOrder(p.Tasks); err != nil {
		return err
	}
	if utf8.RuneCountInString(p.Rationale) > 4000 {
		p.Rationale = string([]rune(p.Rationale)[:4000])
	}
	return nil
}

func validateContract(i int, t *PlanTask) error {
	if t.Executor != "junior" {
		if t.Contract != nil {
			return fmt.Errorf("task %d: contract is only for junior tasks", i)
		}
		return nil
	}
	if t.TouchesUI {
		return fmt.Errorf("task %d: junior does not do user-facing work (touches_ui)", i)
	}
	c := t.Contract
	if c == nil || len(c.Files) == 0 || len(c.Tests) == 0 || len(c.Files)+len(c.Tests) > 12 {
		return fmt.Errorf("task %d: a junior task needs a contract with 1..12 files and tests", i)
	}
	for _, list := range []*[]string{&c.Files, &c.Tests} {
		for k, p := range *list {
			clean, ok := cleanContractPath(p)
			if !ok {
				return fmt.Errorf("task %d: bad contract path %q", i, p)
			}
			(*list)[k] = clean
		}
	}
	return nil
}

// topoOrder — порядок создания задач; цикл — ошибка.
func topoOrder(tasks []PlanTask) ([]int, error) {
	n := len(tasks)
	indeg := make([]int, n)
	next := make([][]int, n)
	for i, t := range tasks {
		for _, d := range t.DependsOn {
			indeg[i]++
			next[d] = append(next[d], i)
		}
	}
	var order, queue []int
	for i := 0; i < n; i++ {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		order = append(order, i)
		for _, j := range next[i] {
			if indeg[j]--; indeg[j] == 0 {
				queue = append(queue, j)
			}
		}
	}
	if len(order) != n {
		return nil, errors.New("plan dependencies contain a cycle")
	}
	return order, nil
}

// SubmitPlan — хост сдаёт результат Manager'а. Невалидный план → FAILED с причиной (без ошибки HTTP:
// хосту важно знать причину). Ответ — итоговый статус плана.
func (s *Service) SubmitPlan(ctx context.Context, id Identity, planID, workerID uuid.UUID, attempt int, p *Proposal, failure string) (string, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	if _, err := workerOf(ctx, tx, id, workerID); err != nil {
		return "", "", err
	}
	pl, err := lockPlan(ctx, tx, planID)
	if err != nil {
		return "", "", err
	}
	if pl.status != PlanPlanning || pl.workerID == nil || *pl.workerID != workerID || pl.attempt != attempt {
		return "", "", ErrNotOwner
	}
	status, reason := PlanFailed, strings.TrimSpace(failure)
	if reason == "" && p == nil {
		reason = "empty plan"
	}
	if reason == "" {
		lists, err := loadHostLists(ctx, tx, pl.repo)
		if err != nil {
			return "", "", err
		}
		if err := ValidateProposal(p, lists); err != nil {
			reason = "plan rejected by the orchestrator: " + err.Error()
		}
	}
	if reason != "" {
		_, err := tx.Exec(ctx, `UPDATE exec.plans SET status = 'FAILED', error = $2, lease_expires_at = NULL, updated_at = now()
			WHERE id = $1`, planID, clip(reason, 4000))
		if err != nil {
			return "", "", err
		}
	} else {
		proposal, _ := json.Marshal(p)
		status = PlanAwaitingApproval
		if _, err := tx.Exec(ctx, `UPDATE exec.plans SET status = 'AWAITING_APPROVAL', proposal = $2, error = '', lease_expires_at = NULL,
				updated_at = now() WHERE id = $1`, planID, proposal); err != nil {
			return "", "", err
		}
		// semi_auto и night: задачи создаются сразу; manual ждёт «Утвердить план» (решение 7.5/2).
		if pl.mode == "manual" {
			if err := s.requestPlanApproval(ctx, tx, planID, pl.project, p); err != nil {
				return "", "", err
			}
		} else {
			if err := s.applyPlan(ctx, tx, planID, pl, p); err != nil {
				return "", "", err
			}
			status = PlanApproved
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	s.notify(pl.createdBy)
	return status, reason, nil
}

// applyPlan создаёт задачи плана в порядке зависимостей.
func (s *Service) applyPlan(ctx context.Context, tx pgx.Tx, planID uuid.UUID, pl lockedPlan, p *Proposal) error {
	order, err := topoOrder(p.Tasks)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(p.Tasks))
	for _, i := range order {
		t := p.Tasks[i]
		deps := make([]uuid.UUID, 0, len(t.DependsOn))
		for _, d := range t.DependsOn {
			deps = append(deps, ids[d])
		}
		pid := planID
		id, err := insertTask(ctx, tx, pl.createdBy, pl.project, NewTask{
			RepositoryID: pl.repo, Title: t.Title, Prompt: t.Prompt, ExecutionMode: pl.mode, ExecutionEnv: t.ExecutionEnv,
			RequiredCapabilities: t.RequiredCapabilities, DependsOn: deps, ReviewLevel: t.ReviewLevel,
			RequiredSkills: t.RequiredSkills, MCPProfile: t.MCPProfile, TouchesUI: t.TouchesUI, Executor: t.Executor, planID: &pid,
			contract: t.Contract,
		})
		if err != nil {
			return err
		}
		ids[i] = id
	}
	_, err = tx.Exec(ctx, `UPDATE exec.plans SET status = 'APPROVED', updated_at = now() WHERE id = $1`, planID)
	return err
}

// ApprovePlan — «Утвердить план»: задачи создаются по проверенному предложению.
func (s *Service) ApprovePlan(ctx context.Context, userID, planID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pl, err := lockPlan(ctx, tx, planID)
	if err != nil {
		return err
	}
	if err := requireProjectWrite(ctx, tx, userID, pl.project); err != nil {
		return err
	}
	if pl.status != PlanAwaitingApproval {
		return fmt.Errorf("%w: plan is %s", ErrBadTransition, pl.status)
	}
	var p Proposal
	if err := json.Unmarshal(pl.proposal, &p); err != nil {
		return err
	}
	if err := s.applyPlan(ctx, tx, planID, pl, &p); err != nil {
		return err
	}
	if err := closeApprovals(ctx, tx, "plan_id", planID, ApprovalApproved, &userID, ""); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

func (s *Service) RejectPlan(ctx context.Context, userID, planID uuid.UUID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pl, err := lockPlan(ctx, tx, planID)
	if err != nil {
		return err
	}
	if err := requireProjectWrite(ctx, tx, userID, pl.project); err != nil {
		return err
	}
	if pl.status != PlanAwaitingApproval && pl.status != PlanQueued {
		return fmt.Errorf("%w: plan is %s", ErrBadTransition, pl.status)
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.plans SET status = 'REJECTED', error = $2, updated_at = now() WHERE id = $1`,
		planID, clip(strings.TrimSpace("rejected by user: "+reason), 4000)); err != nil {
		return err
	}
	if err := closeApprovals(ctx, tx, "plan_id", planID, ApprovalRejected, &userID, reason); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

func (s *Service) ListPlans(ctx context.Context, userID uuid.UUID) ([]PlanView, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id, p.repository_id, p.request, p.execution_mode, p.status, p.attempt,
			COALESCE(p.proposal, 'null'::jsonb), p.error, p.updated_at,
			COALESCE((SELECT jsonb_agg(t.id ORDER BY t.created_at) FROM exec.tasks t WHERE t.plan_id = p.id), '[]'::jsonb)
		FROM exec.plans p JOIN core.project_access a ON a.project_id = p.project_id AND a.user_id = $1
		ORDER BY p.updated_at DESC LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (PlanView, error) {
		var v PlanView
		var ids []byte
		err := r.Scan(&v.ID, &v.RepositoryID, &v.Request, &v.ExecutionMode, &v.Status, &v.Attempt, &v.Proposal, &v.Error, &v.UpdatedAt, &ids)
		if err == nil {
			err = json.Unmarshal(ids, &v.TaskIDs)
		}
		return v, err
	})
}

// reapPlans: Manager пропал (ПК выключили) — план в очередь, после двух попыток — FAILED.
func (s *Service) reapPlans(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE exec.plans SET
			status = CASE WHEN attempt >= $2 THEN 'FAILED' ELSE 'QUEUED' END,
			error = CASE WHEN attempt >= $2 THEN 'manager stopped responding' ELSE error END,
			worker_id = NULL, lease_expires_at = NULL, updated_at = now()
		WHERE status = 'PLANNING' AND lease_expires_at < $1`, s.now(), maxPlanAttempts)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// SetHostRegistries — что хост умеет подключать: скиллы, MCP-профили, исполнители.
func (s *Service) SetHostRegistries(ctx context.Context, id Identity, hostID uuid.UUID, skills, mcp, executors []string) error {
	if len(executors) == 0 {
		executors = []string{"claude"}
	}
	sk, _ := json.Marshal(nonNil(skills))
	mc, _ := json.Marshal(nonNil(mcp))
	ex, _ := json.Marshal(executors)
	tag, err := s.pool.Exec(ctx, `UPDATE exec.hosts SET skills = $3, mcp_profiles = $4, executors = $5
		WHERE id = $1 AND user_id = $2 AND device_id = $6`, hostID, id.UserID, sk, mc, ex, id.DeviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

// SetSkillPolicy — обязательные и запрещённые скиллы репозитория (ТЗ п. 10.2).
func (s *Service) SetSkillPolicy(ctx context.Context, userID, repoID uuid.UUID, required, forbidden []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var project uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM exec.repositories WHERE id = $1`, repoID).Scan(&project); err != nil {
		return ErrNotFound
	}
	if err := requireProjectWrite(ctx, tx, userID, project); err != nil {
		return err
	}
	pol, _ := json.Marshal(map[string][]string{"required": nonNil(required), "forbidden": nonNil(forbidden)})
	if _, err := tx.Exec(ctx, `UPDATE exec.repositories SET skill_policy = $2 WHERE id = $1`, repoID, pol); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
