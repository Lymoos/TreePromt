package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ApprovalRequest (ТЗ п. 13, docs/stage7-6-security-audit.md, раздел 4): решения человека —
// слить задачу, утвердить план, снять инцидент. Принять, отклонить с комментарием, принять частично.

const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
	ApprovalPartial  = "partial"
	ApprovalExpired  = "expired"

	approvalTTL = 7 * 24 * time.Hour
)

type Approval struct {
	ID         uuid.UUID       `json:"id"`
	ProjectID  uuid.UUID       `json:"project_id"`
	TaskID     *uuid.UUID      `json:"task_id"`
	PlanID     *uuid.UUID      `json:"plan_id"`
	IncidentID *uuid.UUID      `json:"incident_id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	Status     string          `json:"status"`
	Comment    string          `json:"comment"`
	CreatedAt  time.Time       `json:"created_at"`
	ExpiresAt  *time.Time      `json:"expires_at"`
}

// requestTaskApproval — задача ждёт «Слить»: содержание — то, что нужно для решения.
func (s *Service) requestTaskApproval(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, f Facts) error {
	var project uuid.UUID
	var title string
	var result *string
	if err := tx.QueryRow(ctx, `SELECT project_id, title, result_commit FROM exec.tasks WHERE id = $1`, taskID).Scan(&project, &title, &result); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"title": title, "result_commit": result, "approval_required": nonNil(f.ApprovalRequired),
		"review": f.Review})
	_, err := tx.Exec(ctx, `INSERT INTO exec.approval_requests (id, project_id, task_id, type, payload, expires_at)
		VALUES ($1, $2, $3, 'merge', $4, $5)`, uuid.Must(uuid.NewV7()), project, taskID, redactJSON(payload), s.now().Add(approvalTTL))
	return err
}

func (s *Service) requestPlanApproval(ctx context.Context, tx pgx.Tx, planID, project uuid.UUID, p *Proposal) error {
	payload, _ := json.Marshal(p)
	_, err := tx.Exec(ctx, `INSERT INTO exec.approval_requests (id, project_id, plan_id, type, payload, expires_at)
		VALUES ($1, $2, $3, 'plan', $4, $5)`, uuid.Must(uuid.NewV7()), project, planID, redactJSON(payload), s.now().Add(approvalTTL))
	return err
}

// closeApprovals закрывает ожидающие запросы по задаче или плану (решение принято старой кнопкой или иначе).
func closeApprovals(ctx context.Context, tx pgx.Tx, column string, id uuid.UUID, status string, reviewer *uuid.UUID, comment string) error {
	_, err := tx.Exec(ctx, `UPDATE exec.approval_requests SET status = $2, reviewer = $3, comment = $4, resolved_at = now()
		WHERE `+column+` = $1 AND status = 'pending'`, id, status, reviewer, clip(comment, 4000))
	return err
}

func (s *Service) ListApprovals(ctx context.Context, userID uuid.UUID, status string) ([]Approval, error) {
	rows, err := s.pool.Query(ctx, `SELECT q.id, q.project_id, q.task_id, q.plan_id, q.incident_id, q.type, q.payload, q.status,
			q.comment, q.created_at, q.expires_at
		FROM exec.approval_requests q JOIN core.project_access a ON a.project_id = q.project_id AND a.user_id = $1
		WHERE ($2 = '' OR q.status = $2) ORDER BY q.created_at DESC LIMIT 200`, userID, status)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Approval, error) {
		var a Approval
		err := r.Scan(&a.ID, &a.ProjectID, &a.TaskID, &a.PlanID, &a.IncidentID, &a.Type, &a.Payload, &a.Status, &a.Comment,
			&a.CreatedAt, &a.ExpiresAt)
		return a, err
	})
}

// Decision — ответ человека на запрос.
type Decision struct {
	Decision string `json:"decision"` // approve | reject | partial
	Comment  string `json:"comment"`
	Tasks    []int  `json:"tasks"` // partial: какие задачи плана создать
}

// ResolveApproval — единая точка решений; «Слить» и «Утвердить план» продолжают работать как раньше.
func (s *Service) ResolveApproval(ctx context.Context, userID, approvalID uuid.UUID, d Decision) error {
	var a Approval
	err := s.pool.QueryRow(ctx, `SELECT q.id, q.project_id, q.task_id, q.plan_id, q.incident_id, q.type, q.status
		FROM exec.approval_requests q JOIN core.project_access p ON p.project_id = q.project_id AND p.user_id = $2
		WHERE q.id = $1`, approvalID, userID).Scan(&a.ID, &a.ProjectID, &a.TaskID, &a.PlanID, &a.IncidentID, &a.Type, &a.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if a.Status != ApprovalPending {
		return fmt.Errorf("%w: request is %s", ErrBadTransition, a.Status)
	}
	comment := strings.TrimSpace(d.Comment)
	switch {
	case a.Type == "merge" && d.Decision == "approve":
		return s.Merge(ctx, userID, *a.TaskID)
	case a.Type == "merge" && d.Decision == "reject":
		if comment == "" {
			return fmt.Errorf("%w: a rejection needs a comment for the agent", ErrInvalid)
		}
		return s.rejectTask(ctx, userID, *a.TaskID, comment)
	case a.Type == "plan" && d.Decision == "approve":
		return s.ApprovePlan(ctx, userID, *a.PlanID)
	case a.Type == "plan" && d.Decision == "partial":
		return s.approvePlanPartially(ctx, userID, *a.PlanID, d.Tasks, comment)
	case a.Type == "plan" && d.Decision == "reject":
		if comment == "" {
			return s.RejectPlan(ctx, userID, *a.PlanID, "")
		}
		return s.replan(ctx, userID, *a.PlanID, comment)
	case a.Type == "incident" && d.Decision == "approve":
		return s.ResolveIncident(ctx, userID, *a.IncidentID, comment)
	}
	return fmt.Errorf("%w: decision %q is not possible for a %s request", ErrInvalid, d.Decision, a.Type)
}

// rejectTask — «Отклонить с комментарием»: новая попытка поверх свежего integration, комментарий —
// главный урок для исполнителя. Отказ человека не тратит лимит попыток.
func (s *Service) rejectTask(ctx context.Context, userID, taskID uuid.UUID, comment string) error {
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
		return fmt.Errorf("%w: only a MERGEABLE task can be sent back", ErrBadTransition)
	}
	human, _ := json.Marshal(map[string]any{"human_review": map[string]string{"comment": Redact(comment)}})
	if _, err := tx.Exec(ctx, `UPDATE exec.attempts SET facts = facts || $3::jsonb, failure_class = 'QA_REJECT', failure_code = 'human_rejected'
		WHERE task_id = $1 AND attempt = $2`, taskID, t.attempt, human); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.tasks SET status = 'QUEUED', worker_id = NULL, failure_class = 'QA_REJECT',
			failure_code = 'human_rejected', max_attempts = GREATEST(max_attempts, attempt + 1), approval_required = '[]',
			review = NULL, updated_at = now() WHERE id = $1`, taskID); err != nil {
		return err
	}
	if err := event(ctx, tx, taskID, &t.status, StatusQueued, "user:"+userID.String(), "rejected by owner: "+clip(Redact(comment), 1000), nil); err != nil {
		return err
	}
	if err := closeApprovals(ctx, tx, "task_id", taskID, ApprovalRejected, &userID, comment); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

// replan — отклонённый план с комментарием уходит Manager'у заново вместе с комментарием.
func (s *Service) replan(ctx context.Context, userID, planID uuid.UUID, comment string) error {
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
	if _, err := tx.Exec(ctx, `UPDATE exec.plans SET status = 'QUEUED', attempt = 0, proposal = NULL, worker_id = NULL,
			request = request || E'\n\nOwner''s feedback on the previous plan (follow it):\n' || $2, updated_at = now() WHERE id = $1`,
		planID, clip(Redact(comment), 4000)); err != nil {
		return err
	}
	if err := closeApprovals(ctx, tx, "plan_id", planID, ApprovalRejected, &userID, comment); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

// approvePlanPartially — создаются только выбранные задачи; их зависимости тоже должны быть выбраны.
func (s *Service) approvePlanPartially(ctx context.Context, userID, planID uuid.UUID, keep []int, comment string) error {
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
	sel := map[int]int{} // старый индекс → новый
	for _, i := range keep {
		if i < 0 || i >= len(p.Tasks) {
			return fmt.Errorf("%w: task %d is not in the plan", ErrInvalid, i)
		}
		if _, dup := sel[i]; !dup {
			sel[i] = -1
		}
	}
	if len(sel) == 0 {
		return fmt.Errorf("%w: choose at least one task", ErrInvalid)
	}
	var sub Proposal
	sub.Rationale = p.Rationale
	for i, t := range p.Tasks {
		if _, ok := sel[i]; ok {
			sel[i] = len(sub.Tasks)
			sub.Tasks = append(sub.Tasks, t)
		}
	}
	for k := range sub.Tasks {
		var deps []int
		for _, d := range sub.Tasks[k].DependsOn {
			nd, ok := sel[d]
			if !ok {
				return fmt.Errorf("%w: task %q depends on task %d that was not chosen", ErrInvalid, sub.Tasks[k].Title, d)
			}
			deps = append(deps, nd)
		}
		sub.Tasks[k].DependsOn = deps
	}
	if err := s.applyPlan(ctx, tx, planID, pl, &sub); err != nil {
		return err
	}
	if err := closeApprovals(ctx, tx, "plan_id", planID, ApprovalPartial, &userID, comment); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

// expireApprovals — просроченные запросы помечаются (задача остаётся ждать; срок — сигнал в канбане).
func (s *Service) expireApprovals(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `UPDATE exec.approval_requests SET status = 'expired', resolved_at = now()
		WHERE status = 'pending' AND expires_at < $1`, s.now())
	return err
}
