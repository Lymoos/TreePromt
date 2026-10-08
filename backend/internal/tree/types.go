// Package tree implements the PromptTree domain: projects, nodes, content,
// versions and the operation-log sync protocol (docs/architecture-prompttree.md, п. 4).
package tree

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	OpCreateProject   = "create_project"
	OpUpdateProject   = "update_project"
	OpDeleteProject   = "delete_project"
	OpRestoreProject  = "restore_project"
	OpCreateNode      = "create_node"
	OpRenameNode      = "rename_node"
	OpMoveNode        = "move_node"
	OpChangeKind      = "change_kind"
	OpDeleteNode      = "delete_node"
	OpRestoreNode     = "restore_node"
	OpSetRawContent   = "set_raw_content"
	OpRestoreVersion  = "restore_version"
	OpResolveConflict = "resolve_conflict"
	// Этап 6: структурированный текст.
	OpSetStructuredText = "set_structured_text"
	OpApplyProposal     = "apply_proposal"
	OpDismissProposal   = "dismiss_proposal"
)

const (
	ResultApplied  = "applied"
	ResultConflict = "conflict"
	ResultRejected = "rejected"
)

const (
	KindFolder  = "folder"
	KindRawNote = "raw_note"
	KindAITask  = "ai_task"
)

// Actor — кто прислал операцию.
type Actor struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
}

// Op — операция клиента. BaseRevision для операций с текстом — ревизия поля
// (raw_revision), которую видел клиент; для остальных хранится только для аудита.
type Op struct {
	OperationID  uuid.UUID       `json:"operation_id"`
	ClientSeq    int64           `json:"client_seq"`
	Type         string          `json:"type"`
	EntityID     uuid.UUID       `json:"entity_id"`
	BaseRevision int64           `json:"base_revision"`
	Payload      json.RawMessage `json:"payload"`
}

type OpError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	OperationID    uuid.UUID     `json:"operation_id"`
	Result         string        `json:"result"`
	ServerRevision *int64        `json:"server_revision,omitempty"`
	Error          *OpError      `json:"error,omitempty"`
	Project        *ProjectState `json:"project,omitempty"`
	Node           *NodeState    `json:"node,omitempty"`
}

type ProjectState struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Revision    int64      `json:"revision"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at"`
}

type NodeState struct {
	ID                     uuid.UUID       `json:"id"`
	ProjectID              uuid.UUID       `json:"project_id"`
	ParentID               *uuid.UUID      `json:"parent_id"`
	Kind                   string          `json:"kind"`
	Name                   string          `json:"name"`
	SortKey                string          `json:"sort_key"`
	Revision               int64           `json:"revision"`
	HasConflict            bool            `json:"has_conflict"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
	DeletedAt              *time.Time      `json:"deleted_at"`
	RawContent             string          `json:"raw_content"`
	RawRevision            int64           `json:"raw_revision"`
	StructuredContent      json.RawMessage `json:"structured_content"`
	StructuredRevision     int64           `json:"structured_revision"`
	StructuredFromRevision *int64          `json:"structured_from_revision"`
	StructureStatus        string          `json:"structure_status"`
	StructureProposal      json.RawMessage `json:"structure_proposal"`
}

type VersionState struct {
	ID             uuid.UUID  `json:"id"`
	NodeID         uuid.UUID  `json:"node_id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	Field          string     `json:"field"`
	Content        string     `json:"content"`
	AtRevision     int64      `json:"at_revision"`
	Reason         string     `json:"reason"`
	DeviceID       *uuid.UUID `json:"device_id"`
	ServerRevision int64      `json:"server_revision"`
	CreatedAt      time.Time  `json:"created_at"`
}

type PullResult struct {
	Projects []ProjectState `json:"projects"`
	Nodes    []NodeState    `json:"nodes"`
	Versions []VersionState `json:"versions"`
	Cursor   int64          `json:"cursor"`
	HasMore  bool           `json:"has_more"`
}

func isProjectOp(t string) bool {
	switch t {
	case OpCreateProject, OpUpdateProject, OpDeleteProject, OpRestoreProject:
		return true
	}
	return false
}
