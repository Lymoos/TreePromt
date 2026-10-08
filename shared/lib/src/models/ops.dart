/// Типы операций — те же, что на сервере (backend/internal/tree/types.go).
abstract final class OpType {
  static const createProject = 'create_project';
  static const updateProject = 'update_project';
  static const deleteProject = 'delete_project';
  static const restoreProject = 'restore_project';
  static const createNode = 'create_node';
  static const renameNode = 'rename_node';
  static const moveNode = 'move_node';
  static const changeKind = 'change_kind';
  static const deleteNode = 'delete_node';
  static const restoreNode = 'restore_node';
  static const setRawContent = 'set_raw_content';
  static const restoreVersion = 'restore_version';
  static const resolveConflict = 'resolve_conflict';
  static const setStructuredText = 'set_structured_text';
  static const applyProposal = 'apply_proposal';
  static const dismissProposal = 'dismiss_proposal';

  static bool isProjectOp(String t) =>
      t == createProject || t == updateProject || t == deleteProject || t == restoreProject;
}

abstract final class NodeKind {
  static const folder = 'folder';
  static const rawNote = 'raw_note';
  static const aiTask = 'ai_task';

  static bool isFile(String k) => k != folder;
}

abstract final class StructureStatus {
  static const none = 'none';
  static const pending = 'pending';
  static const done = 'done';
  static const stale = 'stale';
  static const failed = 'failed';
}

abstract final class VersionReason {
  static const beforeStructure = 'before_structure';
  static const conflictLocal = 'conflict_local';
  static const conflictServer = 'conflict_server';
  static const beforeRestore = 'before_restore';
  static const checkpoint = 'checkpoint';
}
