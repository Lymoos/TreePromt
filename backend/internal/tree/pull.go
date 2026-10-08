package tree

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	DefaultPullLimit = 500
	MaxPullLimit     = 2000
)

// Pull returns every change with server_revision > cursor in projects the user
// can access. A page never splits one revision: the new cursor is chosen so that
// everything at or below it has been returned.
func (s *Service) Pull(ctx context.Context, userID uuid.UUID, cursor int64, limit int) (PullResult, error) {
	if limit <= 0 || limit > MaxPullLimit {
		limit = DefaultPullLimit
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PullResult{}, err
	}
	defer tx.Rollback(ctx)

	res, err := pullPage(ctx, tx, userID, cursor, 0, limit)
	if err != nil {
		return PullResult{}, err
	}
	full := len(res.Projects) == limit || len(res.Nodes) == limit || len(res.Versions) == limit
	if !full {
		res.Cursor = max(cursor, maxRevision(res))
		return res, tx.Commit(ctx)
	}

	// Хотя бы один список упёрся в лимит: за его последней ревизией могут быть ещё строки.
	upper := int64(math.MaxInt64)
	if len(res.Projects) == limit {
		upper = min(upper, res.Projects[limit-1].Revision)
	}
	if len(res.Nodes) == limit {
		upper = min(upper, res.Nodes[limit-1].Revision)
	}
	if len(res.Versions) == limit {
		upper = min(upper, res.Versions[limit-1].ServerRevision)
	}
	next := upper - 1
	if next <= cursor {
		// Одна ревизия больше страницы (например, восстановление большой ветки): отдаём её целиком.
		res, err = pullPage(ctx, tx, userID, cursor, upper, 0)
		if err != nil {
			return PullResult{}, err
		}
		res.Cursor, res.HasMore = upper, true
		return res, tx.Commit(ctx)
	}
	res.Projects = filterRev(res.Projects, next, func(p ProjectState) int64 { return p.Revision })
	res.Nodes = filterRev(res.Nodes, next, func(n NodeState) int64 { return n.Revision })
	res.Versions = filterRev(res.Versions, next, func(v VersionState) int64 { return v.ServerRevision })
	res.Cursor, res.HasMore = next, true
	return res, tx.Commit(ctx)
}

// pullPage reads rows with after < revision [<= upTo when upTo > 0], up to limit (0 = no limit).
func pullPage(ctx context.Context, tx pgx.Tx, userID uuid.UUID, after, upTo int64, limit int) (PullResult, error) {
	var res PullResult
	upper := int64(math.MaxInt64)
	if upTo > 0 {
		upper = upTo
	}
	lim := any(nil)
	if limit > 0 {
		lim = limit
	}

	rows, err := tx.Query(ctx, `SELECT `+projectCols+`
		FROM tree.projects p JOIN core.project_access a ON a.project_id = p.id AND a.user_id = $1
		WHERE p.revision > $2 AND p.revision <= $3 ORDER BY p.revision, p.id LIMIT $4`, userID, after, upper, lim)
	if err != nil {
		return res, err
	}
	if res.Projects, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ProjectState, error) { return scanProject(r) }); err != nil {
		return res, err
	}

	rows, err = tx.Query(ctx, `SELECT `+nodeCols+`
		FROM tree.nodes n
		JOIN tree.node_content c ON c.node_id = n.id
		JOIN core.project_access a ON a.project_id = n.project_id AND a.user_id = $1
		WHERE n.revision > $2 AND n.revision <= $3 ORDER BY n.revision, n.id LIMIT $4`, userID, after, upper, lim)
	if err != nil {
		return res, err
	}
	if res.Nodes, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (NodeState, error) { return scanNode(r) }); err != nil {
		return res, err
	}

	rows, err = tx.Query(ctx, `SELECT v.id, v.node_id, v.project_id, v.field, v.content, v.at_revision, v.reason,
			v.device_id, v.server_revision, v.created_at
		FROM tree.content_versions v JOIN core.project_access a ON a.project_id = v.project_id AND a.user_id = $1
		WHERE v.server_revision > $2 AND v.server_revision <= $3 ORDER BY v.server_revision, v.id LIMIT $4`, userID, after, upper, lim)
	if err != nil {
		return res, err
	}
	res.Versions, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (VersionState, error) {
		var v VersionState
		err := r.Scan(&v.ID, &v.NodeID, &v.ProjectID, &v.Field, &v.Content, &v.AtRevision, &v.Reason,
			&v.DeviceID, &v.ServerRevision, &v.CreatedAt)
		return v, err
	})
	if res.Projects == nil {
		res.Projects = []ProjectState{}
	}
	if res.Nodes == nil {
		res.Nodes = []NodeState{}
	}
	if res.Versions == nil {
		res.Versions = []VersionState{}
	}
	return res, err
}

func maxRevision(r PullResult) int64 {
	var m int64
	for _, p := range r.Projects {
		m = max(m, p.Revision)
	}
	for _, n := range r.Nodes {
		m = max(m, n.Revision)
	}
	for _, v := range r.Versions {
		m = max(m, v.ServerRevision)
	}
	return m
}

func filterRev[T any](items []T, upTo int64, rev func(T) int64) []T {
	out := items[:0]
	for _, it := range items {
		if rev(it) <= upTo {
			out = append(out, it)
		}
	}
	return out
}
