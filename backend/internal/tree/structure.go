package tree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Этап 6 (docs/stage6-ai-structuring.md): хранение и транзакционное применение
// результатов структурирования. Сама модель и валидатор — в пакете ai.

// Generated — ответ модели по JSON-схеме (ТЗ п. 6.5).
type Generated struct {
	Role          string   `json:"role"`
	Facts         []string `json:"facts"`
	Constraints   []string `json:"constraints"`
	OpenQuestions []string `json:"open_questions"`
	FormattedText string   `json:"formatted_text"`
}

// StructuredDoc — содержимое node_content.structured_content.
type StructuredDoc struct {
	Generated
	// Последний текст от ИИ: абзацы, которых в нём нет, написал человек.
	AIFormattedText string   `json:"ai_formatted_text"`
	HumanParagraphs []string `json:"human_paragraphs"`
	// Исходник, из которого получен результат, — для дельты при повторе.
	SourceRaw     string    `json:"source_raw"`
	RequestID     uuid.UUID `json:"request_id"`
	PromptVersion string    `json:"prompt_version"`
}

// Finding — замечание детерминированного валидатора.
type Finding struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// Proposal — результат, который не применён автоматически.
type Proposal struct {
	RequestID      uuid.UUID  `json:"request_id"`
	Reason         string     `json:"reason"` // stale | flagged | failed
	SourceRevision int64      `json:"source_revision"`
	SourceRaw      string     `json:"source_raw,omitempty"`
	PromptVersion  string     `json:"prompt_version,omitempty"`
	Generated      *Generated `json:"generated,omitempty"`
	Findings       []Finding  `json:"findings,omitempty"`
	Error          string     `json:"error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

const (
	ProposalStale   = "stale"
	ProposalFlagged = "flagged"
	ProposalFailed  = "failed"
)

var (
	ErrStaleSource = errors.New("raw text changed since the given revision; sync and retry")
	ErrNotTask     = errors.New("only AI tasks can be structured")
	ErrNoAccess    = errors.New("node not found")
)

// StructureJob — всё, что нужно обработчику очереди для одного запроса.
type StructureJob struct {
	RequestID      uuid.UUID
	NodeID         uuid.UUID
	UserID         uuid.UUID
	SourceRevision int64
	Raw            string
	Previous       *StructuredDoc
}

func HashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Paragraphs делит текст на абзацы по пустым строкам.
func Paragraphs(s string) []string {
	var out []string
	for _, p := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// humanParagraphs — абзацы текста, которых нет в версии ИИ.
func humanParagraphs(text, aiText string) []string {
	ai := map[string]bool{}
	for _, p := range Paragraphs(aiText) {
		ai[p] = true
	}
	out := []string{}
	for _, p := range Paragraphs(text) {
		if !ai[p] {
			out = append(out, p)
		}
	}
	return out
}

// write выполняет изменение вне конвейера операций, но под той же блокировкой
// записи и с новой server_revision — чтобы pull по курсору ничего не пропустил.
func (s *Service) write(ctx context.Context, fn func(tx pgx.Tx, rev int64) (uuid.UUID, error)) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, writeLockKey); err != nil {
		return err
	}
	var rev int64
	if err := tx.QueryRow(ctx, `SELECT nextval('tree.server_revision_seq')`).Scan(&rev); err != nil {
		return err
	}
	projectID, err := fn(tx, rev)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if s.notifier != nil && projectID != uuid.Nil {
		s.notify(ctx, projectID, rev)
	}
	return nil
}

type contentRow struct {
	projectID   uuid.UUID
	kind        string
	raw         string
	rawRevision int64
	doc         *StructuredDoc
	structRev   int64
	structFrom  *int64
	status      string
	proposal    *Proposal
}

func lockContentRow(ctx context.Context, tx pgx.Tx, nodeID uuid.UUID) (contentRow, error) {
	var r contentRow
	var docRaw, propRaw []byte
	err := tx.QueryRow(ctx, `SELECT n.project_id, n.kind, c.raw_content, c.raw_revision, c.structured_content,
			c.structured_revision, c.structured_from_revision, c.structure_status, c.structure_proposal
		FROM tree.nodes n JOIN tree.node_content c ON c.node_id = n.id
		WHERE n.id = $1 FOR UPDATE OF n, c`, nodeID).
		Scan(&r.projectID, &r.kind, &r.raw, &r.rawRevision, &docRaw, &r.structRev, &r.structFrom, &r.status, &propRaw)
	if err != nil {
		return r, err
	}
	if len(docRaw) > 0 {
		r.doc = &StructuredDoc{}
		if err := json.Unmarshal(docRaw, r.doc); err != nil {
			return r, err
		}
	}
	if len(propRaw) > 0 {
		r.proposal = &Proposal{}
		if err := json.Unmarshal(propRaw, r.proposal); err != nil {
			return r, err
		}
	}
	return r, nil
}

// settledStatus — статус структуры без учёта идущего запроса.
func (r contentRow) settledStatus() string {
	switch {
	case r.doc == nil:
		return "none"
	case r.structFrom != nil && *r.structFrom == r.rawRevision:
		return "done"
	default:
		return "stale"
	}
}

func touchNodeRev(ctx context.Context, tx pgx.Tx, nodeID uuid.UUID, rev int64) error {
	_, err := tx.Exec(ctx, `UPDATE tree.nodes SET revision = $2, updated_at = now() WHERE id = $1`, nodeID, rev)
	return err
}

// StartStructuring регистрирует запрос и помечает узел «структурируется…».
// Исходник должен быть ровно той ревизии, которую видел клиент.
func (s *Service) StartStructuring(ctx context.Context, actor Actor, nodeID uuid.UUID, sourceRev int64,
	requestID uuid.UUID, promptVersion, model string) error {
	return s.write(ctx, func(tx pgx.Tx, rev int64) (uuid.UUID, error) {
		var role string
		err := tx.QueryRow(ctx, `SELECT a.role FROM tree.nodes n
			JOIN core.project_access a ON a.project_id = n.project_id AND a.user_id = $2 WHERE n.id = $1`,
			nodeID, actor.UserID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && role != "owner" && role != "editor") {
			return uuid.Nil, ErrNoAccess
		}
		if err != nil {
			return uuid.Nil, err
		}
		r, err := lockContentRow(ctx, tx, nodeID)
		if err != nil {
			return uuid.Nil, err
		}
		if r.kind != KindAITask {
			return uuid.Nil, ErrNotTask
		}
		if r.rawRevision != sourceRev {
			return uuid.Nil, ErrStaleSource
		}
		// Новый запрос отменяет незавершённые старые по этому узлу.
		if _, err := tx.Exec(ctx, `UPDATE ai.structure_requests SET status = 'superseded', completed_at = now()
			WHERE node_id = $1 AND status IN ('queued', 'running')`, nodeID); err != nil {
			return uuid.Nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ai.structure_requests
			(request_id, node_id, user_id, device_id, source_revision, source_content_hash, prompt_version, model, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'queued')`,
			requestID, nodeID, actor.UserID, actor.DeviceID, sourceRev, HashContent(r.raw), promptVersion, model); err != nil {
			return uuid.Nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE tree.node_content SET structure_status = 'pending', structure_proposal = NULL
			WHERE node_id = $1`, nodeID); err != nil {
			return uuid.Nil, err
		}
		return r.projectID, touchNodeRev(ctx, tx, nodeID, rev)
	})
}

// ClaimStructureRequest переводит запрос в работу. nil — запрос уже не нужен.
func (s *Service) ClaimStructureRequest(ctx context.Context, requestID uuid.UUID) (*StructureJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	job := &StructureJob{RequestID: requestID}
	var status string
	err = tx.QueryRow(ctx, `SELECT node_id, user_id, source_revision, status FROM ai.structure_requests
		WHERE request_id = $1 FOR UPDATE`, requestID).Scan(&job.NodeID, &job.UserID, &job.SourceRevision, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if status != "queued" {
		return nil, nil
	}
	var docRaw []byte
	if err := tx.QueryRow(ctx, `SELECT raw_content, structured_content FROM tree.node_content WHERE node_id = $1`,
		job.NodeID).Scan(&job.Raw, &docRaw); err != nil {
		return nil, err
	}
	if len(docRaw) > 0 {
		job.Previous = &StructuredDoc{}
		if err := json.Unmarshal(docRaw, job.Previous); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE ai.structure_requests SET status = 'running', attempts = attempts + 1, started_at = now()
		WHERE request_id = $1`, requestID); err != nil {
		return nil, err
	}
	return job, tx.Commit(ctx)
}

// RequeueInterrupted возвращает в очередь запросы, прерванные перезапуском сервера.
func (s *Service) RequeueInterrupted(ctx context.Context) ([]uuid.UUID, error) {
	if _, err := s.pool.Exec(ctx, `UPDATE ai.structure_requests SET status = 'queued' WHERE status = 'running'`); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT request_id FROM ai.structure_requests WHERE status = 'queued' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// applyGenerated записывает результат ИИ как текущую структуру. Абзацы человека
// не теряются никогда: если модель их потеряла, они дописываются в конец.
func applyGenerated(ctx context.Context, tx pgx.Tx, nodeID uuid.UUID, r contentRow, rev int64, gen Generated,
	sourceRev int64, sourceRaw string, requestID uuid.UUID, promptVersion string, deviceID *uuid.UUID) error {
	var human []string
	if r.doc != nil {
		human = r.doc.HumanParagraphs
		if strings.TrimSpace(r.doc.FormattedText) != "" {
			if err := insertVersion(ctx, tx, nodeID, r.projectID, "structured", r.doc.FormattedText,
				r.structRev, "before_structure", deviceID, rev); err != nil {
				return err
			}
		}
	}
	aiText := gen.FormattedText
	formatted := aiText
	for _, p := range human {
		if !strings.Contains(formatted, p) {
			formatted = strings.TrimRight(formatted, "\n") + "\n\n" + p
		}
	}
	if human == nil {
		human = []string{}
	}
	gen.FormattedText = formatted
	doc := StructuredDoc{Generated: gen, AIFormattedText: aiText, HumanParagraphs: human,
		SourceRaw: sourceRaw, RequestID: requestID, PromptVersion: promptVersion}
	status := "stale"
	if sourceRev == r.rawRevision {
		status = "done"
	}
	if _, err := tx.Exec(ctx, `UPDATE tree.node_content SET structured_content = $2, structured_revision = $3,
			structured_from_revision = $4, structure_status = $5, structure_proposal = NULL
		WHERE node_id = $1`, nodeID, doc, rev, sourceRev, status); err != nil {
		return err
	}
	return touchNodeRev(ctx, tx, nodeID, rev)
}

func setRequestStatus(ctx context.Context, tx pgx.Tx, requestID uuid.UUID, status string, gen *Generated,
	findings []Finding, errMsg string) error {
	_, err := tx.Exec(ctx, `UPDATE ai.structure_requests SET status = $2, generated = $3, findings = $4,
		error = NULLIF($5, ''), completed_at = now() WHERE request_id = $1`, requestID, status, gen, findings, errMsg)
	return err
}

// FinishStructure применяет ответ модели (ТЗ п. 6.2): только если исходник
// не менялся и валидатор доволен; иначе сохраняет предложение.
func (s *Service) FinishStructure(ctx context.Context, requestID uuid.UUID, gen Generated, findings []Finding, promptVersion string) error {
	return s.write(ctx, func(tx pgx.Tx, rev int64) (uuid.UUID, error) {
		var nodeID uuid.UUID
		var sourceRev int64
		var status string
		if err := tx.QueryRow(ctx, `SELECT node_id, source_revision, status FROM ai.structure_requests
			WHERE request_id = $1 FOR UPDATE`, requestID).Scan(&nodeID, &sourceRev, &status); err != nil {
			return uuid.Nil, err
		}
		if status != "running" {
			return uuid.Nil, nil // запрос заменён более новым — результат не нужен
		}
		r, err := lockContentRow(ctx, tx, nodeID)
		if err != nil {
			return uuid.Nil, err
		}
		reason := ""
		switch {
		case len(findings) > 0:
			reason = ProposalFlagged
		case r.rawRevision != sourceRev:
			reason = ProposalStale
		}
		if reason == "" {
			if err := applyGenerated(ctx, tx, nodeID, r, rev, gen, sourceRev, r.raw, requestID, promptVersion, nil); err != nil {
				return uuid.Nil, err
			}
			return r.projectID, setRequestStatus(ctx, tx, requestID, "applied", &gen, nil, "")
		}
		// Исходник, из которого получен ответ, нужен для дельты, если предложение применят.
		var sourceRaw string
		if reason == ProposalStale {
			sourceRaw = "" // текст той ревизии уже перезаписан; дельта при следующем запросе пойдёт от текущего
		} else {
			sourceRaw = r.raw
		}
		p := Proposal{RequestID: requestID, Reason: reason, SourceRevision: sourceRev, SourceRaw: sourceRaw,
			PromptVersion: promptVersion, Generated: &gen, Findings: findings, CreatedAt: time.Now().UTC()}
		if _, err := tx.Exec(ctx, `UPDATE tree.node_content SET structure_proposal = $2, structure_status = $3
			WHERE node_id = $1`, nodeID, p, r.settledStatus()); err != nil {
			return uuid.Nil, err
		}
		reqStatus := "proposed"
		if reason == ProposalFlagged {
			reqStatus = "flagged"
		}
		if err := setRequestStatus(ctx, tx, requestID, reqStatus, &gen, findings, ""); err != nil {
			return uuid.Nil, err
		}
		return r.projectID, touchNodeRev(ctx, tx, nodeID, rev)
	})
}

// FailStructure: модель не ответила. Пользователь увидит ошибку и кнопку «Повторить».
func (s *Service) FailStructure(ctx context.Context, requestID uuid.UUID, msg string) error {
	return s.write(ctx, func(tx pgx.Tx, rev int64) (uuid.UUID, error) {
		var nodeID uuid.UUID
		var sourceRev int64
		var status string
		if err := tx.QueryRow(ctx, `SELECT node_id, source_revision, status FROM ai.structure_requests
			WHERE request_id = $1 FOR UPDATE`, requestID).Scan(&nodeID, &sourceRev, &status); err != nil {
			return uuid.Nil, err
		}
		if status != "running" {
			return uuid.Nil, nil
		}
		r, err := lockContentRow(ctx, tx, nodeID)
		if err != nil {
			return uuid.Nil, err
		}
		p := Proposal{RequestID: requestID, Reason: ProposalFailed, SourceRevision: sourceRev, Error: msg,
			CreatedAt: time.Now().UTC()}
		if _, err := tx.Exec(ctx, `UPDATE tree.node_content SET structure_proposal = $2, structure_status = $3
			WHERE node_id = $1`, nodeID, p, r.settledStatus()); err != nil {
			return uuid.Nil, err
		}
		if err := setRequestStatus(ctx, tx, requestID, "failed", nil, nil, msg); err != nil {
			return uuid.Nil, err
		}
		return r.projectID, touchNodeRev(ctx, tx, nodeID, rev)
	})
}

// StructureRequestInfo — состояние запроса для клиента (запасной путь к WebSocket).
type StructureRequestInfo struct {
	RequestID uuid.UUID `json:"request_id"`
	NodeID    uuid.UUID `json:"node_id"`
	Status    string    `json:"status"`
	Findings  []Finding `json:"findings,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func (s *Service) StructureRequest(ctx context.Context, userID, requestID uuid.UUID) (StructureRequestInfo, error) {
	var info StructureRequestInfo
	var findings []byte
	var errMsg *string
	err := s.pool.QueryRow(ctx, `SELECT request_id, node_id, status, findings, error FROM ai.structure_requests
		WHERE request_id = $1 AND user_id = $2`, requestID, userID).Scan(&info.RequestID, &info.NodeID, &info.Status, &findings, &errMsg)
	if errors.Is(err, pgx.ErrNoRows) {
		return info, ErrNoAccess
	}
	if err != nil {
		return info, err
	}
	if len(findings) > 0 {
		_ = json.Unmarshal(findings, &info.Findings)
	}
	if errMsg != nil {
		info.Error = *errMsg
	}
	return info, nil
}

// CountRecentRequests — для дневного лимита.
func (s *Service) CountRecentRequests(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ai.structure_requests WHERE user_id = $1 AND created_at > $2`,
		userID, since).Scan(&n)
	return n, err
}

// ── операции синхронизации для структуры ──

// setStructuredText — пользователь правит итоговый текст. Его абзацы становятся
// «человеческими», и повторное структурирование их не перезапишет (ТЗ п. 6.1).
func (oc *opCtx) setStructuredText() (outcome, error) {
	p, err := decode[struct {
		Text string `json:"text"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validContent(p.Text); err != nil {
		return outcome{}, err
	}
	if _, err := oc.lockNode(); err != nil {
		return outcome{}, err
	}
	r, err := lockContentRow(oc.ctx, oc.tx, oc.op.EntityID)
	if err != nil {
		return outcome{}, err
	}
	if r.doc == nil {
		return outcome{}, reject(CodeInvalidPayload, "the task has no structured text yet")
	}
	if r.doc.FormattedText == p.Text {
		return applied(r.projectID), nil
	}
	// Структуру с base_revision уже меняли (ИИ или другое устройство): прежний текст — в историю.
	if r.structRev > oc.op.BaseRevision {
		if err := insertVersion(oc.ctx, oc.tx, oc.op.EntityID, r.projectID, "structured", r.doc.FormattedText,
			r.structRev, "conflict_server", &oc.actor.DeviceID, oc.rev); err != nil {
			return outcome{}, err
		}
	}
	doc := *r.doc
	doc.FormattedText = p.Text
	doc.HumanParagraphs = humanParagraphs(p.Text, doc.AIFormattedText)
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.node_content SET structured_content = $2, structured_revision = $3
		WHERE node_id = $1`, oc.op.EntityID, doc, oc.rev); err != nil {
		return outcome{}, err
	}
	if err := oc.touchNode(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	return applied(r.projectID), oc.restoreChain(oc.op.EntityID)
}

// applyProposal — пользователь сам решил применить отложенный результат.
func (oc *opCtx) applyProposal() (outcome, error) {
	p, err := decode[struct {
		RequestID uuid.UUID `json:"request_id"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if _, err := oc.lockNode(); err != nil {
		return outcome{}, err
	}
	r, err := lockContentRow(oc.ctx, oc.tx, oc.op.EntityID)
	if err != nil {
		return outcome{}, err
	}
	if r.proposal == nil || r.proposal.RequestID != p.RequestID || r.proposal.Generated == nil {
		return outcome{}, reject(CodeNotFound, "this proposal is no longer available")
	}
	sourceRaw := r.proposal.SourceRaw
	if sourceRaw == "" {
		sourceRaw = r.raw
	}
	if err := applyGenerated(oc.ctx, oc.tx, oc.op.EntityID, r, oc.rev, *r.proposal.Generated, r.proposal.SourceRevision,
		sourceRaw, r.proposal.RequestID, r.proposal.PromptVersion, &oc.actor.DeviceID); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE ai.structure_requests SET status = 'applied' WHERE request_id = $1`,
		p.RequestID); err != nil {
		return outcome{}, err
	}
	return applied(r.projectID), oc.audit("node", oc.op.EntityID, "apply_proposal", nil, map[string]any{"request_id": p.RequestID})
}

func (oc *opCtx) dismissProposal() (outcome, error) {
	p, err := decode[struct {
		RequestID uuid.UUID `json:"request_id"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if _, err := oc.lockNode(); err != nil {
		return outcome{}, err
	}
	r, err := lockContentRow(oc.ctx, oc.tx, oc.op.EntityID)
	if err != nil {
		return outcome{}, err
	}
	if r.proposal != nil && r.proposal.RequestID == p.RequestID {
		if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.node_content SET structure_proposal = NULL WHERE node_id = $1`,
			oc.op.EntityID); err != nil {
			return outcome{}, err
		}
	}
	return applied(r.projectID), oc.touchNode(oc.op.EntityID)
}

// restoreStructuredVersion — откат итогового текста к версии из истории.
func (oc *opCtx) restoreStructuredVersion(content string) error {
	r, err := lockContentRow(oc.ctx, oc.tx, oc.op.EntityID)
	if err != nil {
		return err
	}
	if r.doc == nil {
		return reject(CodeInvalidPayload, "the task has no structured text")
	}
	if err := insertVersion(oc.ctx, oc.tx, oc.op.EntityID, r.projectID, "structured", r.doc.FormattedText,
		r.structRev, "before_restore", &oc.actor.DeviceID, oc.rev); err != nil {
		return err
	}
	doc := *r.doc
	doc.FormattedText = content
	doc.HumanParagraphs = humanParagraphs(content, doc.AIFormattedText)
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.node_content SET structured_content = $2, structured_revision = $3
		WHERE node_id = $1`, oc.op.EntityID, doc, oc.rev); err != nil {
		return err
	}
	return oc.touchNode(oc.op.EntityID)
}
