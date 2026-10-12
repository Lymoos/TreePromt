package exec

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Incident Kill Switch (ТЗ п. 12, docs/stage7-6-security-audit.md, раздел 2). Открытый инцидент
// ставит на паузу репозиторий (или все репозитории владельца) и останавливает ночную смену.
// Снимает паузу только человек.

const (
	IncidentIntegrationRed = "integration_red"
	IncidentSecretLeak     = "secret_leak"
	IncidentQASpike        = "qa_spike"
	IncidentManual         = "manual"

	integrationRedAfter = 3  // слияний подряд, не прошедших проверку после слияния
	qaWindow            = 10 // последних ревью
	qaSpikeRejects      = 8  // «больше 7 отказов из 10»
)

type Incident struct {
	ID           uuid.UUID  `json:"id"`
	RepositoryID *uuid.UUID `json:"repository_id"`
	TaskID       *uuid.UUID `json:"task_id"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	Reason       string     `json:"reason"`
	OpenedAt     time.Time  `json:"opened_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	Comment      string     `json:"comment"`
}

// pausedSQL — условие «репозиторий r не на паузе»; для ночных задач — «у владельца нет открытых инцидентов».
const pausedSQL = `NOT EXISTS (SELECT 1 FROM exec.incidents i WHERE i.status = 'open' AND i.user_id = h.user_id
	AND (i.repository_id IS NULL OR i.repository_id = r.id))`

const nightStopSQL = `(t.execution_mode <> 'night' OR NOT EXISTS (SELECT 1 FROM exec.incidents i2
	WHERE i2.status = 'open' AND i2.user_id = h.user_id))`

// openIncident открывает инцидент (если такого же открытого по репозиторию ещё нет) и запрос на решение.
func (s *Service) openIncident(ctx context.Context, tx pgx.Tx, repoID *uuid.UUID, taskID *uuid.UUID, kind, reason string) error {
	var owner, project uuid.UUID
	if repoID != nil {
		if err := tx.QueryRow(ctx, `SELECT h.user_id, r.project_id FROM exec.repositories r JOIN exec.hosts h ON h.id = r.host_id
			WHERE r.id = $1`, *repoID).Scan(&owner, &project); err != nil {
			return err
		}
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM exec.incidents WHERE status = 'open' AND user_id = $1
		AND repository_id IS NOT DISTINCT FROM $2 AND kind = $3)`, owner, repoID, kind).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	id := uuid.Must(uuid.NewV7())
	if _, err := tx.Exec(ctx, `INSERT INTO exec.incidents (id, user_id, repository_id, task_id, kind, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`, id, owner, repoID, taskID, kind, clip(Redact(reason), 4000)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.audit (user_id, task_id, kind, data) VALUES ($1, $2, 'incident',
		jsonb_build_object('incident', $3::text, 'kind', $4::text, 'reason', $5::text))`, owner, taskID, id.String(), kind, clip(Redact(reason), 2000)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.approval_requests (id, project_id, incident_id, type, payload)
		VALUES ($1, $2, $3, 'incident', jsonb_build_object('kind', $4::text, 'reason', $5::text))`,
		uuid.Must(uuid.NewV7()), project, id, kind, clip(Redact(reason), 2000)); err != nil {
		return err
	}
	s.log.Warn("incident opened: AiCrew paused", "kind", kind, "repository", repoID, "reason", clip(Redact(reason), 300))
	return nil
}

// afterFailure — правила Kill Switch по классу ошибки (вызывается из fail в той же транзакции).
func (s *Service) afterFailure(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, class, reason string) error {
	var repoID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT repository_id FROM exec.tasks WHERE id = $1`, taskID).Scan(&repoID); err != nil {
		return err
	}
	switch class {
	case ClassSecret:
		return s.openIncident(ctx, tx, &repoID, &taskID, IncidentSecretLeak, "secret found in agent changes: "+reason)
	case ClassIntegration:
		var n int
		if err := tx.QueryRow(ctx, `UPDATE exec.repositories SET integration_failures = integration_failures + 1 WHERE id = $1
			RETURNING integration_failures`, repoID).Scan(&n); err != nil {
			return err
		}
		if n >= integrationRedAfter {
			return s.openIncident(ctx, tx, &repoID, &taskID, IncidentIntegrationRed,
				fmt.Sprintf("%d merges in a row failed checks after merging: %s", n, reason))
		}
	case ClassQAReject:
		// QA Kill Switch: ревьюер отклоняет почти всё — скорее сломан ревьюер, чем код.
		var rejects, total int
		if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE rejected), count(*) FROM (
				SELECT e.to_status = 'FAILED' AS rejected FROM exec.task_events e JOIN exec.tasks t ON t.id = e.task_id
				WHERE t.repository_id = $1 AND (e.to_status = 'MERGEABLE' OR (e.to_status = 'FAILED' AND e.reason LIKE 'QA_REJECT%'))
				ORDER BY e.id DESC LIMIT $2) last`, repoID, qaWindow).Scan(&rejects, &total); err != nil {
			return err
		}
		if total >= qaWindow && rejects >= qaSpikeRejects {
			return s.openIncident(ctx, tx, &repoID, &taskID, IncidentQASpike,
				fmt.Sprintf("%d of the last %d reviews were rejections — the reviewer may be broken", rejects, total))
		}
	}
	return nil
}

// ReportIntegrationRed — integration не прошла проверку при переносе в main (aicrew-host promote).
func (s *Service) ReportIntegrationRed(ctx context.Context, userID, repoID uuid.UUID, reason string) error {
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
	if err := s.openIncident(ctx, tx, &repoID, nil, IncidentIntegrationRed, "promote checks failed: "+reason); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

// StopAll / StopRepository — ручной рубильник.
func (s *Service) Stop(ctx context.Context, userID uuid.UUID, repoID *uuid.UUID, reason string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var project uuid.UUID
	if repoID != nil {
		if err := tx.QueryRow(ctx, `SELECT project_id FROM exec.repositories WHERE id = $1`, *repoID).Scan(&project); err != nil {
			return uuid.Nil, ErrNotFound
		}
		if err := requireProjectWrite(ctx, tx, userID, project); err != nil {
			return uuid.Nil, err
		}
	}
	id := uuid.Must(uuid.NewV7())
	if _, err := tx.Exec(ctx, `INSERT INTO exec.incidents (id, user_id, repository_id, kind, reason) VALUES ($1, $2, $3, 'manual', $4)`,
		id, userID, repoID, clip(strings.TrimSpace("stopped by user: "+reason), 4000)); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.audit (user_id, kind, data) VALUES ($1, 'incident',
		jsonb_build_object('incident', $2::text, 'kind', 'manual', 'reason', $3::text))`, userID, id.String(), reason); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	s.notify(userID)
	return id, nil
}

// ResolveIncident — «Возобновить» с комментарием. Счётчик провалов integration сбрасывается.
func (s *Service) ResolveIncident(ctx context.Context, userID, incidentID uuid.UUID, comment string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owner uuid.UUID
	var repoID *uuid.UUID
	var status string
	err = tx.QueryRow(ctx, `SELECT user_id, repository_id, status FROM exec.incidents WHERE id = $1 FOR UPDATE`, incidentID).Scan(&owner, &repoID, &status)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != userID) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return fmt.Errorf("%w: incident is %s", ErrBadTransition, status)
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.incidents SET status = 'resolved', resolved_at = now(), resolved_by = $2, comment = $3
		WHERE id = $1`, incidentID, userID, clip(strings.TrimSpace(comment), 4000)); err != nil {
		return err
	}
	if repoID != nil {
		if _, err := tx.Exec(ctx, `UPDATE exec.repositories SET integration_failures = 0 WHERE id = $1`, *repoID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE exec.approval_requests SET status = 'approved', reviewer = $2, comment = $3, resolved_at = now()
		WHERE incident_id = $1 AND status = 'pending'`, incidentID, userID, clip(comment, 4000)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO exec.audit (user_id, kind, data) VALUES ($1, 'incident',
		jsonb_build_object('incident', $2::text, 'resolved', true, 'comment', $3::text))`, userID, incidentID.String(), comment); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notify(userID)
	return nil
}

func (s *Service) ListIncidents(ctx context.Context, userID uuid.UUID, onlyOpen bool) ([]Incident, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, repository_id, task_id, kind, status, reason, opened_at, resolved_at, comment
		FROM exec.incidents WHERE user_id = $1 AND ($2 = false OR status = 'open') ORDER BY opened_at DESC LIMIT 200`, userID, onlyOpen)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Incident, error) {
		var i Incident
		err := r.Scan(&i.ID, &i.RepositoryID, &i.TaskID, &i.Kind, &i.Status, &i.Reason, &i.OpenedAt, &i.ResolvedAt, &i.Comment)
		return i, err
	})
}
