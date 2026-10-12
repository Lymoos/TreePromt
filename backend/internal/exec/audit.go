package exec

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Аудит (ТЗ п. 14, docs/stage7-6-security-audit.md, раздел 3). Хост присылает записи пачками;
// таблица — только добавление (триггер базы), секреты заменяются ещё раз здесь.

type AuditEntry struct {
	TaskID  *uuid.UUID      `json:"task_id"`
	Attempt *int            `json:"attempt"`
	Kind    string          `json:"kind"` // command | model | attach
	At      *time.Time      `json:"at"`
	Data    json.RawMessage `json:"data"`
}

type PromptSnapshot struct {
	Hash string `json:"hash"`
	Text string `json:"text"`
}

type AuditRecord struct {
	ID      int64           `json:"id"`
	At      time.Time       `json:"at"`
	TaskID  *uuid.UUID      `json:"task_id"`
	Attempt *int            `json:"attempt"`
	Kind    string          `json:"kind"`
	Data    json.RawMessage `json:"data"`
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

const maxAuditBatch = 500

// WriteAudit — записи хоста этого устройства.
func (s *Service) WriteAudit(ctx context.Context, id Identity, entries []AuditEntry, prompts []PromptSnapshot) error {
	if len(entries) > maxAuditBatch || len(prompts) > maxAuditBatch {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var hostID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM exec.hosts WHERE user_id = $1 AND device_id = $2`, id.UserID, id.DeviceID).Scan(&hostID); err != nil {
		return ErrForbidden
	}
	for _, e := range entries {
		if e.Kind != "command" && e.Kind != "model" && e.Kind != "attach" {
			return ErrInvalid
		}
		at := time.Now()
		if e.At != nil {
			at = *e.At
		}
		data := e.Data
		if len(data) == 0 || !json.Valid(data) {
			data = json.RawMessage(`{}`)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO exec.audit (at, user_id, host_id, task_id, attempt, kind, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, at, id.UserID, hostID, e.TaskID, e.Attempt, e.Kind, redactJSON(data)); err != nil {
			return err
		}
	}
	for _, p := range prompts {
		if !hashRe.MatchString(p.Hash) || len(p.Text) > 1<<20 {
			return ErrInvalid
		}
		if _, err := tx.Exec(ctx, `INSERT INTO exec.prompt_snapshots (hash, user_id, text) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, p.Hash, id.UserID, Redact(p.Text)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) TaskAudit(ctx context.Context, userID, taskID uuid.UUID) ([]AuditRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.requireTaskWrite(ctx, tx, userID, taskID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id, at, task_id, attempt, kind, data FROM exec.audit WHERE task_id = $1 ORDER BY id LIMIT 5000`, taskID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AuditRecord, error) {
		var a AuditRecord
		err := r.Scan(&a.ID, &a.At, &a.TaskID, &a.Attempt, &a.Kind, &a.Data)
		return a, err
	})
}

func (s *Service) PromptSnapshot(ctx context.Context, userID uuid.UUID, hash string) (string, error) {
	var text string
	err := s.pool.QueryRow(ctx, `SELECT text FROM exec.prompt_snapshots WHERE user_id = $1 AND hash = $2`, userID, hash).Scan(&text)
	if err != nil {
		return "", ErrNotFound
	}
	return text, nil
}
