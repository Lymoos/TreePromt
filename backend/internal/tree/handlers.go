package tree

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ── доступ и общие действия ──

func (oc *opCtx) role(projectID uuid.UUID) (string, error) {
	var role string
	err := oc.tx.QueryRow(oc.ctx, `SELECT role FROM core.project_access WHERE project_id = $1 AND user_id = $2`,
		projectID, oc.actor.UserID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", reject(CodeNotFound, "project not found") // не раскрываем существование чужого проекта
	}
	return role, err
}

func (oc *opCtx) requireWrite(projectID uuid.UUID) error {
	role, err := oc.role(projectID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "editor" {
		return reject(CodeForbidden, "no write access to project")
	}
	return nil
}

func (oc *opCtx) requireOwner(projectID uuid.UUID) error {
	role, err := oc.role(projectID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return reject(CodeForbidden, "only the owner can do this")
	}
	return nil
}

func (oc *opCtx) audit(entity string, id uuid.UUID, action string, before, after any) error {
	_, err := oc.tx.Exec(oc.ctx, `INSERT INTO tree.audit_events
		(entity, entity_id, action, before, after, operation_id, device_id, server_revision)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entity, id, action, before, after, oc.op.OperationID, oc.actor.DeviceID, oc.rev)
	return err
}

// restoreProject снимает удаление с проекта: правка внутри удалённого проекта
// не должна теряться (правило «ничего не теряется», п. 4.3 архитектуры).
func (oc *opCtx) restoreProject(projectID uuid.UUID) error {
	tag, err := oc.tx.Exec(oc.ctx, `UPDATE tree.projects SET deleted_at = NULL, revision = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL`, projectID, oc.rev)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	return oc.audit("project", projectID, "auto_restore", nil, nil)
}

// restoreChain восстанавливает узел и всех его удалённых предков.
func (oc *opCtx) restoreChain(id uuid.UUID) error {
	cur := &id
	var projectID uuid.UUID
	for cur != nil {
		var parent *uuid.UUID
		var deleted bool
		err := oc.tx.QueryRow(oc.ctx, `SELECT project_id, parent_id, deleted_at IS NOT NULL FROM tree.nodes WHERE id = $1 FOR UPDATE`, *cur).
			Scan(&projectID, &parent, &deleted)
		if err != nil {
			return err
		}
		if deleted {
			if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET deleted_at = NULL, revision = $2, updated_at = now() WHERE id = $1`,
				*cur, oc.rev); err != nil {
				return err
			}
			if err := oc.audit("node", *cur, "auto_restore", nil, nil); err != nil {
				return err
			}
		}
		cur = parent
	}
	return oc.restoreProject(projectID)
}

type nodeRow struct {
	projectID uuid.UUID
	parentID  *uuid.UUID
	kind      string
	name      string
	sortKey   string
	deleted   bool
}

// lockNode блокирует узел операции и проверяет право записи.
func (oc *opCtx) lockNode() (nodeRow, error) {
	var n nodeRow
	err := oc.tx.QueryRow(oc.ctx, `SELECT project_id, parent_id, kind, name, sort_key, deleted_at IS NOT NULL
		FROM tree.nodes WHERE id = $1 FOR UPDATE`, oc.op.EntityID).
		Scan(&n.projectID, &n.parentID, &n.kind, &n.name, &n.sortKey, &n.deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return n, reject(CodeNotFound, "node not found")
	}
	if err != nil {
		return n, err
	}
	return n, oc.requireWrite(n.projectID)
}

func (oc *opCtx) touchNode(id uuid.UUID) error {
	_, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET revision = $2, updated_at = now() WHERE id = $1`, id, oc.rev)
	return err
}

// checkParent проверяет нового родителя: существует, в том же проекте, это папка.
func (oc *opCtx) checkParent(projectID, parentID uuid.UUID) error {
	var pProject uuid.UUID
	var pKind string
	err := oc.tx.QueryRow(oc.ctx, `SELECT project_id, kind FROM tree.nodes WHERE id = $1`, parentID).Scan(&pProject, &pKind)
	if errors.Is(err, pgx.ErrNoRows) {
		return reject(CodeInvalidParent, "parent not found")
	}
	if err != nil {
		return err
	}
	if pProject != projectID {
		return reject(CodeInvalidParent, "parent is in another project")
	}
	if pKind != KindFolder {
		return reject(CodeInvalidParent, "parent must be a folder")
	}
	return nil
}

func (oc *opCtx) insertVersion(nodeID, projectID uuid.UUID, content string, atRevision int64, reason string) error {
	return insertVersion(oc.ctx, oc.tx, nodeID, projectID, "raw", content, atRevision, reason, &oc.actor.DeviceID, oc.rev)
}

func insertVersion(ctx context.Context, tx pgx.Tx, nodeID, projectID uuid.UUID, field, content string,
	atRevision int64, reason string, deviceID *uuid.UUID, rev int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO tree.content_versions
		(id, node_id, project_id, field, content, at_revision, reason, device_id, server_revision)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		uuid.Must(uuid.NewV7()), nodeID, projectID, field, content, atRevision, reason, deviceID, rev)
	return err
}

func applied(projectID uuid.UUID) outcome {
	return outcome{result: ResultApplied, projectID: projectID}
}

// ── проекты ──

func (oc *opCtx) createProject() (outcome, error) {
	p, err := decode[struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validName(p.Name); err != nil {
		return outcome{}, err
	}
	if err := validDescription(p.Description); err != nil {
		return outcome{}, err
	}
	var exists bool
	if err := oc.tx.QueryRow(oc.ctx, `SELECT EXISTS (SELECT 1 FROM tree.projects WHERE id = $1)`, oc.op.EntityID).Scan(&exists); err != nil {
		return outcome{}, err
	}
	if exists {
		return outcome{}, reject(CodeAlreadyExists, "project already exists")
	}
	if _, err := oc.tx.Exec(oc.ctx, `INSERT INTO tree.projects (id, name, description, revision, created_by) VALUES ($1, $2, $3, $4, $5)`,
		oc.op.EntityID, p.Name, p.Description, oc.rev, oc.actor.UserID); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `INSERT INTO core.project_access (project_id, user_id, role) VALUES ($1, $2, 'owner')`,
		oc.op.EntityID, oc.actor.UserID); err != nil {
		return outcome{}, err
	}
	return applied(oc.op.EntityID), oc.audit("project", oc.op.EntityID, "create", nil, map[string]any{"name": p.Name})
}

func (oc *opCtx) updateProject() (outcome, error) {
	p, err := decode[struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if p.Name != nil {
		if err := validName(*p.Name); err != nil {
			return outcome{}, err
		}
	}
	if p.Description != nil {
		if err := validDescription(*p.Description); err != nil {
			return outcome{}, err
		}
	}
	if err := oc.requireWrite(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	var before struct{ Name, Description string }
	if err := oc.tx.QueryRow(oc.ctx, `SELECT name, description FROM tree.projects WHERE id = $1 FOR UPDATE`, oc.op.EntityID).
		Scan(&before.Name, &before.Description); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.projects SET name = COALESCE($2, name), description = COALESCE($3, description),
		revision = $4, updated_at = now() WHERE id = $1`, oc.op.EntityID, p.Name, p.Description, oc.rev); err != nil {
		return outcome{}, err
	}
	if err := oc.restoreProject(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	return applied(oc.op.EntityID), oc.audit("project", oc.op.EntityID, "update", before, p)
}

func (oc *opCtx) setProjectDeleted(deleted bool) (outcome, error) {
	if _, err := decode[struct{}](oc.op.Payload); err != nil {
		return outcome{}, err
	}
	if err := oc.requireOwner(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	q := `UPDATE tree.projects SET deleted_at = NULL, revision = $2, updated_at = now() WHERE id = $1`
	action := "restore"
	if deleted {
		q = `UPDATE tree.projects SET deleted_at = COALESCE(deleted_at, now()), revision = $2, updated_at = now() WHERE id = $1`
		action = "delete"
	}
	if _, err := oc.tx.Exec(oc.ctx, q, oc.op.EntityID, oc.rev); err != nil {
		return outcome{}, err
	}
	return applied(oc.op.EntityID), oc.audit("project", oc.op.EntityID, action, nil, nil)
}

// ── узлы ──

func (oc *opCtx) createNode() (outcome, error) {
	p, err := decode[struct {
		ProjectID  uuid.UUID  `json:"project_id"`
		ParentID   *uuid.UUID `json:"parent_id"`
		Kind       string     `json:"kind"`
		Name       string     `json:"name"`
		SortKey    string     `json:"sort_key"`
		RawContent string     `json:"raw_content"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	for _, check := range []error{validKind(p.Kind), validName(p.Name), validSortKey(p.SortKey), validContent(p.RawContent)} {
		if check != nil {
			return outcome{}, check
		}
	}
	if p.Kind == KindFolder && p.RawContent != "" {
		return outcome{}, reject(CodeInvalidPayload, "folders have no content")
	}
	if err := oc.requireWrite(p.ProjectID); err != nil {
		return outcome{}, err
	}
	var exists bool
	if err := oc.tx.QueryRow(oc.ctx, `SELECT EXISTS (SELECT 1 FROM tree.nodes WHERE id = $1)`, oc.op.EntityID).Scan(&exists); err != nil {
		return outcome{}, err
	}
	if exists {
		return outcome{}, reject(CodeAlreadyExists, "node already exists")
	}
	if p.ParentID != nil {
		if err := oc.checkParent(p.ProjectID, *p.ParentID); err != nil {
			return outcome{}, err
		}
		// Узел, созданный офлайн в папке, которую удалили на другом устройстве, не теряется: папка возвращается.
		if err := oc.restoreChain(*p.ParentID); err != nil {
			return outcome{}, err
		}
	} else if err := oc.restoreProject(p.ProjectID); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `INSERT INTO tree.nodes (id, project_id, parent_id, kind, name, sort_key, revision)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		oc.op.EntityID, p.ProjectID, p.ParentID, p.Kind, p.Name, p.SortKey, oc.rev); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `INSERT INTO tree.node_content (node_id, raw_content, raw_revision) VALUES ($1, $2, $3)`,
		oc.op.EntityID, p.RawContent, oc.rev); err != nil {
		return outcome{}, err
	}
	return applied(p.ProjectID), oc.audit("node", oc.op.EntityID, "create", nil,
		map[string]any{"name": p.Name, "kind": p.Kind, "parent_id": p.ParentID})
}

// renameNode: при одновременном переименовании побеждает операция, пришедшая позже;
// проигравшее значение остаётся в audit_events.before.
func (oc *opCtx) renameNode() (outcome, error) {
	p, err := decode[struct {
		Name string `json:"name"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validName(p.Name); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if err := oc.restoreChain(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET name = $2, revision = $3, updated_at = now() WHERE id = $1`,
		oc.op.EntityID, p.Name, oc.rev); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "rename", map[string]any{"name": n.name}, map[string]any{"name": p.Name})
}

func (oc *opCtx) moveNode() (outcome, error) {
	p, err := decode[struct {
		ParentID *uuid.UUID `json:"parent_id"`
		SortKey  string     `json:"sort_key"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validSortKey(p.SortKey); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if p.ParentID != nil {
		if *p.ParentID == oc.op.EntityID {
			return outcome{}, reject(CodeCycle, "node cannot be its own parent")
		}
		if err := oc.checkParent(n.projectID, *p.ParentID); err != nil {
			return outcome{}, err
		}
		// Цикл: новый родитель не должен быть потомком перемещаемого узла.
		var cyclic bool
		if err := oc.tx.QueryRow(oc.ctx, `WITH RECURSIVE up AS (
				SELECT id, parent_id FROM tree.nodes WHERE id = $1
				UNION ALL
				SELECT n.id, n.parent_id FROM tree.nodes n JOIN up ON n.id = up.parent_id
			) SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, *p.ParentID, oc.op.EntityID).Scan(&cyclic); err != nil {
			return outcome{}, err
		}
		if cyclic {
			return outcome{}, reject(CodeCycle, "cannot move a folder into its own descendant")
		}
		if err := oc.restoreChain(*p.ParentID); err != nil {
			return outcome{}, err
		}
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET parent_id = $2, sort_key = $3, revision = $4, updated_at = now() WHERE id = $1`,
		oc.op.EntityID, p.ParentID, p.SortKey, oc.rev); err != nil {
		return outcome{}, err
	}
	if err := oc.restoreChain(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "move",
		map[string]any{"parent_id": n.parentID, "sort_key": n.sortKey}, p)
}

func (oc *opCtx) changeKind() (outcome, error) {
	p, err := decode[struct {
		Kind string `json:"kind"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validKind(p.Kind); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if (n.kind == KindFolder) != (p.Kind == KindFolder) {
		return outcome{}, reject(CodeKindChange, "a folder cannot become a note and vice versa")
	}
	if err := oc.restoreChain(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET kind = $2, revision = $3, updated_at = now() WHERE id = $1`,
		oc.op.EntityID, p.Kind, oc.rev); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "change_kind", map[string]any{"kind": n.kind}, p)
}

// deleteNode — мягкое удаление; потомки скрываются вместе с узлом на клиенте.
func (oc *opCtx) deleteNode() (outcome, error) {
	if _, err := decode[struct{}](oc.op.Payload); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET deleted_at = COALESCE(deleted_at, now()), revision = $2, updated_at = now() WHERE id = $1`,
		oc.op.EntityID, oc.rev); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "delete", nil, nil)
}

func (oc *opCtx) restoreNode() (outcome, error) {
	if _, err := decode[struct{}](oc.op.Payload); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if err := oc.restoreChain(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	if err := oc.touchNode(oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "restore", nil, nil)
}

// ── текст ──

type rawState struct {
	content   string
	revision  int64
	updatedAt time.Time
}

func (oc *opCtx) lockContent() (rawState, error) {
	var r rawState
	err := oc.tx.QueryRow(oc.ctx, `SELECT raw_content, raw_revision, raw_updated_at FROM tree.node_content WHERE node_id = $1 FOR UPDATE`,
		oc.op.EntityID).Scan(&r.content, &r.revision, &r.updatedAt)
	return r, err
}

// rawChangedByOthers: менялся ли текст после base_revision с другого устройства.
// Свои последовательные правки одного устройства конфликтом не считаются.
func (oc *opCtx) rawChangedByOthers(base int64) (bool, error) {
	var changed bool
	err := oc.tx.QueryRow(oc.ctx, `SELECT EXISTS (
		SELECT 1 FROM tree.operations
		WHERE entity_id = $1 AND server_revision > $2 AND device_id <> $3 AND result = 'applied'
		  AND type IN ('create_node', 'set_raw_content', 'restore_version', 'resolve_conflict'))`,
		oc.op.EntityID, base, oc.actor.DeviceID).Scan(&changed)
	return changed, err
}

func (oc *opCtx) writeRaw(content string) error {
	_, err := oc.tx.Exec(oc.ctx, `UPDATE tree.node_content SET raw_content = $2, raw_revision = $3, raw_updated_at = now(),
		structure_status = CASE WHEN structure_status = 'done' THEN 'stale' ELSE structure_status END
		WHERE node_id = $1`, oc.op.EntityID, content, oc.rev)
	if err != nil {
		return err
	}
	if err := oc.touchNode(oc.op.EntityID); err != nil {
		return err
	}
	return oc.restoreChain(oc.op.EntityID)
}

func (oc *opCtx) setRawContent() (outcome, error) {
	p, err := decode[struct {
		Content string `json:"content"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validContent(p.Content); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if n.kind == KindFolder {
		return outcome{}, reject(CodeInvalidPayload, "folders have no content")
	}
	cur, err := oc.lockContent()
	if err != nil {
		return outcome{}, err
	}
	if cur.content == p.Content {
		// Тот же текст: применять нечего, конфликта нет.
		if err := oc.touchNode(oc.op.EntityID); err != nil {
			return outcome{}, err
		}
		return applied(n.projectID), oc.restoreChain(oc.op.EntityID)
	}
	if cur.revision > oc.op.BaseRevision {
		conflict, err := oc.rawChangedByOthers(oc.op.BaseRevision)
		if err != nil {
			return outcome{}, err
		}
		if conflict {
			// Обе версии сохраняются, текст на сервере не меняется, пользователь выбирает (п. 4.3).
			if err := oc.insertVersion(oc.op.EntityID, n.projectID, cur.content, cur.revision, "conflict_server"); err != nil {
				return outcome{}, err
			}
			if err := oc.insertVersion(oc.op.EntityID, n.projectID, p.Content, oc.op.BaseRevision, "conflict_local"); err != nil {
				return outcome{}, err
			}
			if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET has_conflict = true, revision = $2, updated_at = now() WHERE id = $1`,
				oc.op.EntityID, oc.rev); err != nil {
				return outcome{}, err
			}
			if err := oc.restoreChain(oc.op.EntityID); err != nil {
				return outcome{}, err
			}
			return outcome{result: ResultConflict, projectID: n.projectID,
				err: &OpError{Code: "text_conflict", Message: "text was changed on another device; both versions are kept"}}, nil
		}
	}
	if cur.content != "" && oc.now.Sub(cur.updatedAt) > checkpointAfter {
		if err := oc.insertVersion(oc.op.EntityID, n.projectID, cur.content, cur.revision, "checkpoint"); err != nil {
			return outcome{}, err
		}
	}
	return applied(n.projectID), oc.writeRaw(p.Content)
}

func (oc *opCtx) restoreVersion() (outcome, error) {
	p, err := decode[struct {
		VersionID uuid.UUID `json:"version_id"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	var field, content string
	err = oc.tx.QueryRow(oc.ctx, `SELECT field, content FROM tree.content_versions WHERE id = $1 AND node_id = $2`,
		p.VersionID, oc.op.EntityID).Scan(&field, &content)
	if errors.Is(err, pgx.ErrNoRows) {
		return outcome{}, reject(CodeNotFound, "version not found")
	}
	if err != nil {
		return outcome{}, err
	}
	if field == "structured" {
		if err := oc.restoreStructuredVersion(content); err != nil {
			return outcome{}, err
		}
		return applied(n.projectID), oc.audit("node", oc.op.EntityID, "restore_version", nil, map[string]any{"version_id": p.VersionID})
	}
	cur, err := oc.lockContent()
	if err != nil {
		return outcome{}, err
	}
	if err := oc.insertVersion(oc.op.EntityID, n.projectID, cur.content, cur.revision, "before_restore"); err != nil {
		return outcome{}, err
	}
	if err := oc.writeRaw(content); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "restore_version", nil, map[string]any{"version_id": p.VersionID})
}

// resolveConflict — пользователь выбрал или собрал итоговый текст.
func (oc *opCtx) resolveConflict() (outcome, error) {
	p, err := decode[struct {
		Content string `json:"content"`
	}](oc.op.Payload)
	if err != nil {
		return outcome{}, err
	}
	if err := validContent(p.Content); err != nil {
		return outcome{}, err
	}
	n, err := oc.lockNode()
	if err != nil {
		return outcome{}, err
	}
	if n.kind == KindFolder {
		return outcome{}, reject(CodeInvalidPayload, "folders have no content")
	}
	if _, err := oc.lockContent(); err != nil {
		return outcome{}, err
	}
	if _, err := oc.tx.Exec(oc.ctx, `UPDATE tree.nodes SET has_conflict = false WHERE id = $1`, oc.op.EntityID); err != nil {
		return outcome{}, err
	}
	if err := oc.writeRaw(p.Content); err != nil {
		return outcome{}, err
	}
	return applied(n.projectID), oc.audit("node", oc.op.EntityID, "resolve_conflict", nil, nil)
}
