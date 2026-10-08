package tree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// writeLockKey — глобальная advisory-блокировка записи. Ревизия берётся из
// sequence уже под блокировкой, а блокировка снимается только при commit,
// поэтому ревизии становятся видимыми строго по возрастанию и pull по курсору
// ничего не пропускает. Нагрузка записи у PromptTree мала, это допустимо.
const writeLockKey int64 = 0x5052_5452_4545 // "PRTREE"

// Пауза в редактировании, после которой перед новой правкой сохраняется checkpoint-версия.
const checkpointAfter = 5 * time.Minute

// Notifier сообщает подключённым клиентам о новой ревизии (WebSocket).
type Notifier interface {
	NotifyRevision(userIDs []uuid.UUID, revision int64)
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

// Push applies operations in client_seq order, each in its own transaction, so a
// rejected operation does not undo the others. It stops at the first server
// failure and returns the results so far; the client retries the rest.
func (s *Service) Push(ctx context.Context, actor Actor, ops []Op) ([]Result, error) {
	sorted := append([]Op(nil), ops...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ClientSeq < sorted[j].ClientSeq })

	results := make([]Result, 0, len(sorted))
	for _, op := range sorted {
		r, err := s.applyOne(ctx, actor, op)
		if err != nil {
			return results, fmt.Errorf("operation %s: %w", op.OperationID, err)
		}
		results = append(results, r)
	}
	return results, nil
}

type outcome struct {
	result    string
	err       *OpError
	projectID uuid.UUID
}

// opCtx — контекст применения одной операции внутри транзакции.
type opCtx struct {
	ctx   context.Context
	tx    pgx.Tx
	actor Actor
	op    Op
	rev   int64
	now   time.Time
}

func (s *Service) applyOne(ctx context.Context, actor Actor, op Op) (Result, error) {
	if op.OperationID == uuid.Nil || op.EntityID == uuid.Nil {
		return Result{OperationID: op.OperationID, Result: ResultRejected,
			Error: &OpError{Code: CodeInvalidPayload, Message: "operation_id and entity_id are required"}}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, writeLockKey); err != nil {
		return Result{}, err
	}

	// Идемпотентность: повтор уже обработанной операции возвращает прежний результат.
	var storedUser uuid.UUID
	var stored []byte
	err = tx.QueryRow(ctx, `SELECT user_id, response FROM tree.operations WHERE operation_id = $1`, op.OperationID).
		Scan(&storedUser, &stored)
	if err == nil {
		if storedUser != actor.UserID {
			return Result{OperationID: op.OperationID, Result: ResultRejected,
				Error: &OpError{Code: CodeForbidden, Message: "operation_id belongs to another user"}}, nil
		}
		var r Result
		if err := json.Unmarshal(stored, &r); err != nil {
			return Result{}, err
		}
		if err := attachState(ctx, tx, actor.UserID, op, &r); err != nil {
			return Result{}, err
		}
		return r, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}

	var rev int64
	if err := tx.QueryRow(ctx, `SELECT nextval('tree.server_revision_seq')`).Scan(&rev); err != nil {
		return Result{}, err
	}

	// Обработчик работает в savepoint: при отказе все его изменения откатываются,
	// а сама операция всё равно записывается в журнал как rejected.
	sp, err := tx.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	oc := &opCtx{ctx: ctx, tx: sp, actor: actor, op: op, rev: rev, now: s.now()}
	out, err := dispatch(oc)
	if err != nil {
		var re *rejectErr
		if !errors.As(err, &re) {
			return Result{}, err
		}
		if err := sp.Rollback(ctx); err != nil {
			return Result{}, err
		}
		out = outcome{result: ResultRejected, err: &re.OpError}
	} else if err := sp.Commit(ctx); err != nil {
		return Result{}, err
	}

	res := Result{OperationID: op.OperationID, Result: out.result, Error: out.err}
	var serverRev *int64
	if out.result != ResultRejected {
		serverRev = &rev
		res.ServerRevision = serverRev
	}
	response, err := json.Marshal(res) // без состояния объекта: его всегда читаем заново
	if err != nil {
		return Result{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO tree.operations
		(operation_id, user_id, device_id, client_seq, type, entity_id, base_revision, payload, result, server_revision, response)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		op.OperationID, actor.UserID, actor.DeviceID, op.ClientSeq, op.Type, op.EntityID, op.BaseRevision,
		sanitizePayload(op.Payload), out.result, serverRev, response)
	if err != nil {
		return Result{}, err
	}
	if err := attachState(ctx, tx, actor.UserID, op, &res); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}

	if out.result != ResultRejected && s.notifier != nil {
		s.notify(ctx, out.projectID, rev)
	}
	return res, nil
}

func (s *Service) notify(ctx context.Context, projectID uuid.UUID, rev int64) {
	rows, err := s.pool.Query(ctx, `SELECT user_id FROM core.project_access WHERE project_id = $1`, projectID)
	if err != nil {
		s.log.Warn("notify: load project users", "err", err)
		return
	}
	users, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		s.log.Warn("notify: scan project users", "err", err)
		return
	}
	s.notifier.NotifyRevision(users, rev)
}

// attachState adds the current state of the affected object so the client can
// rebase its pending operations on top of it (п. 4.4).
func attachState(ctx context.Context, q pgx.Tx, userID uuid.UUID, op Op, r *Result) error {
	if isProjectOp(op.Type) {
		p, err := loadProject(ctx, q, userID, op.EntityID)
		if err == nil {
			r.Project = &p
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	n, err := loadNode(ctx, q, userID, op.EntityID)
	if err == nil {
		r.Node = &n
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func dispatch(oc *opCtx) (outcome, error) {
	switch oc.op.Type {
	case OpCreateProject:
		return oc.createProject()
	case OpUpdateProject:
		return oc.updateProject()
	case OpDeleteProject:
		return oc.setProjectDeleted(true)
	case OpRestoreProject:
		return oc.setProjectDeleted(false)
	case OpCreateNode:
		return oc.createNode()
	case OpRenameNode:
		return oc.renameNode()
	case OpMoveNode:
		return oc.moveNode()
	case OpChangeKind:
		return oc.changeKind()
	case OpDeleteNode:
		return oc.deleteNode()
	case OpRestoreNode:
		return oc.restoreNode()
	case OpSetRawContent:
		return oc.setRawContent()
	case OpRestoreVersion:
		return oc.restoreVersion()
	case OpResolveConflict:
		return oc.resolveConflict()
	}
	return outcome{}, reject(CodeUnknownOp, "unknown operation type %q", oc.op.Type)
}

const projectCols = `p.id, p.name, p.description, p.revision, a.role, p.created_at, p.updated_at, p.deleted_at`

func scanProject(row pgx.Row) (ProjectState, error) {
	var p ProjectState
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Revision, &p.Role, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	return p, err
}

func loadProject(ctx context.Context, q pgx.Tx, userID, id uuid.UUID) (ProjectState, error) {
	return scanProject(q.QueryRow(ctx, `SELECT `+projectCols+`
		FROM tree.projects p JOIN core.project_access a ON a.project_id = p.id AND a.user_id = $1
		WHERE p.id = $2`, userID, id))
}

const nodeCols = `n.id, n.project_id, n.parent_id, n.kind, n.name, n.sort_key, n.revision, n.has_conflict,
	n.created_at, n.updated_at, n.deleted_at, c.raw_content, c.raw_revision, c.structured_content,
	c.structured_revision, c.structured_from_revision, c.structure_status`

func scanNode(row pgx.Row) (NodeState, error) {
	var n NodeState
	err := row.Scan(&n.ID, &n.ProjectID, &n.ParentID, &n.Kind, &n.Name, &n.SortKey, &n.Revision, &n.HasConflict,
		&n.CreatedAt, &n.UpdatedAt, &n.DeletedAt, &n.RawContent, &n.RawRevision, &n.StructuredContent,
		&n.StructuredRevision, &n.StructuredFromRevision, &n.StructureStatus)
	return n, err
}

func loadNode(ctx context.Context, q pgx.Tx, userID, id uuid.UUID) (NodeState, error) {
	return scanNode(q.QueryRow(ctx, `SELECT `+nodeCols+`
		FROM tree.nodes n
		JOIN tree.node_content c ON c.node_id = n.id
		JOIN core.project_access a ON a.project_id = n.project_id AND a.user_id = $1
		WHERE n.id = $2`, userID, id))
}
