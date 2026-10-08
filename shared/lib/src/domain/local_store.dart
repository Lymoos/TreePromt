import 'package:drift/drift.dart' show Value;

import '../data/database.dart';

export '../data/database.dart'
    show
        Project,
        TreeNode,
        NodeContent,
        ContentVersion,
        OutboxEntry,
        ProjectsCompanion,
        NodesCompanion,
        NodeContentsCompanion,
        ContentVersionsCompanion;
export 'package:drift/drift.dart' show Value;

/// Абстракция локального хранилища (архитектура п. 2.2). Сценарии и синхронизация
/// зависят только от неё; реализация — [DriftLocalStore] на SQLite для всех платформ.
abstract interface class LocalStore {
  /// Атомарная транзакция: изменение данных и запись в очередь — вместе или никак.
  Future<T> transaction<T>(Future<T> Function() action);

  // ── чтение для интерфейса ──
  Stream<List<Project>> watchProjects();
  Stream<List<TreeNode>> watchNodes();
  Stream<TreeNode?> watchNode(String id);
  Stream<NodeContent?> watchContent(String nodeId);
  Stream<List<ContentVersion>> watchVersions(String nodeId);
  Stream<int> watchPendingCount();

  // ── чтение ──
  Future<Project?> project(String id);
  Future<TreeNode?> node(String id);
  Future<NodeContent?> content(String nodeId);
  Future<ContentVersion?> version(String id);

  /// Наибольший sort_key среди детей (parentId == null — корень проекта).
  Future<String?> lastChildKey(String projectId, String? parentId);

  // ── запись состояния ──
  Future<void> putProject(ProjectsCompanion row);
  Future<void> patchProject(String id, ProjectsCompanion patch);
  Future<void> putNode(NodesCompanion row);
  Future<void> patchNode(String id, NodesCompanion patch);
  Future<void> putContent(NodeContentsCompanion row);
  Future<void> patchContent(String nodeId, NodeContentsCompanion patch);
  Future<void> putVersion(ContentVersionsCompanion row);

  // ── очередь операций ──
  Future<int> enqueue({
    required String operationId,
    required String type,
    required String entityId,
    required int baseRevision,
    required String payload,
  });
  Future<OutboxEntry?> lastOpFor(String entityId);
  Future<void> replaceOpPayload(int seq, String payload);
  Future<List<OutboxEntry>> pendingOpsFor(String entityId);

  /// Берёт до [limit] операций по порядку и помечает их «в полёте».
  Future<List<OutboxEntry>> takeBatch(int limit);
  Future<void> removeOp(String operationId);
  Future<void> releaseInFlight();

  // ── служебные значения (курсор, id устройства) ──
  Future<String?> getValue(String key);
  Future<void> setValue(String key, String value);

  /// Стирает все данные (выход с другим аккаунтом).
  Future<void> wipe();
}

extension ValueX<T> on T {
  Value<T> get v => Value(this);
}
