import 'package:drift/drift.dart';

import '../domain/local_store.dart';
import 'database.dart';

class DriftLocalStore implements LocalStore {
  DriftLocalStore(this.db);

  final AppDatabase db;

  @override
  Future<T> transaction<T>(Future<T> Function() action) => db.transaction(action);

  @override
  Stream<List<Project>> watchProjects() =>
      (db.select(db.projects)..orderBy([(p) => OrderingTerm.asc(p.createdAt)])).watch();

  @override
  Stream<List<TreeNode>> watchNodes() =>
      (db.select(db.nodes)..orderBy([(n) => OrderingTerm.asc(n.sortKey), (n) => OrderingTerm.asc(n.id)])).watch();

  @override
  Stream<TreeNode?> watchNode(String id) =>
      (db.select(db.nodes)..where((n) => n.id.equals(id))).watchSingleOrNull();

  @override
  Stream<NodeContent?> watchContent(String nodeId) =>
      (db.select(db.nodeContents)..where((c) => c.nodeId.equals(nodeId))).watchSingleOrNull();

  @override
  Stream<List<ContentVersion>> watchVersions(String nodeId) => (db.select(db.contentVersions)
        ..where((v) => v.nodeId.equals(nodeId))
        ..orderBy([(v) => OrderingTerm.desc(v.serverRevision), (v) => OrderingTerm.asc(v.reason)]))
      .watch();

  @override
  Stream<int> watchPendingCount() {
    final count = db.outbox.seq.count();
    return (db.selectOnly(db.outbox)..addColumns([count])).map((r) => r.read(count) ?? 0).watchSingle();
  }

  @override
  Future<Project?> project(String id) =>
      (db.select(db.projects)..where((p) => p.id.equals(id))).getSingleOrNull();

  @override
  Future<TreeNode?> node(String id) => (db.select(db.nodes)..where((n) => n.id.equals(id))).getSingleOrNull();

  @override
  Future<NodeContent?> content(String nodeId) =>
      (db.select(db.nodeContents)..where((c) => c.nodeId.equals(nodeId))).getSingleOrNull();

  @override
  Future<ContentVersion?> version(String id) =>
      (db.select(db.contentVersions)..where((v) => v.id.equals(id))).getSingleOrNull();

  @override
  Future<String?> lastChildKey(String projectId, String? parentId) async {
    final q = db.selectOnly(db.nodes)
      ..addColumns([db.nodes.sortKey.max()])
      ..where(db.nodes.projectId.equals(projectId) &
          (parentId == null ? db.nodes.parentId.isNull() : db.nodes.parentId.equals(parentId)));
    final row = await q.getSingleOrNull();
    return row?.read(db.nodes.sortKey.max());
  }

  @override
  Future<void> putProject(ProjectsCompanion row) => db.into(db.projects).insertOnConflictUpdate(row);

  @override
  Future<void> patchProject(String id, ProjectsCompanion patch) =>
      (db.update(db.projects)..where((p) => p.id.equals(id))).write(patch);

  @override
  Future<void> putNode(NodesCompanion row) => db.into(db.nodes).insertOnConflictUpdate(row);

  @override
  Future<void> patchNode(String id, NodesCompanion patch) =>
      (db.update(db.nodes)..where((n) => n.id.equals(id))).write(patch);

  @override
  Future<void> putContent(NodeContentsCompanion row) => db.into(db.nodeContents).insertOnConflictUpdate(row);

  @override
  Future<void> patchContent(String nodeId, NodeContentsCompanion patch) =>
      (db.update(db.nodeContents)..where((c) => c.nodeId.equals(nodeId))).write(patch);

  @override
  Future<void> putVersion(ContentVersionsCompanion row) => db.into(db.contentVersions).insertOnConflictUpdate(row);

  @override
  Future<int> enqueue({
    required String operationId,
    required String type,
    required String entityId,
    required int baseRevision,
    required String payload,
  }) =>
      db.into(db.outbox).insert(OutboxCompanion.insert(
            operationId: operationId,
            type: type,
            entityId: entityId,
            baseRevision: Value(baseRevision),
            payload: payload,
            createdAt: DateTime.now(),
          ));

  @override
  Future<OutboxEntry?> lastOpFor(String entityId) => (db.select(db.outbox)
        ..where((o) => o.entityId.equals(entityId))
        ..orderBy([(o) => OrderingTerm.desc(o.seq)])
        ..limit(1))
      .getSingleOrNull();

  @override
  Future<void> replaceOpPayload(int seq, String payload) =>
      (db.update(db.outbox)..where((o) => o.seq.equals(seq))).write(OutboxCompanion(payload: Value(payload)));

  @override
  Future<List<OutboxEntry>> pendingOpsFor(String entityId) => (db.select(db.outbox)
        ..where((o) => o.entityId.equals(entityId))
        ..orderBy([(o) => OrderingTerm.asc(o.seq)]))
      .get();

  @override
  Future<List<OutboxEntry>> takeBatch(int limit) => db.transaction(() async {
        final rows = await (db.select(db.outbox)
              ..orderBy([(o) => OrderingTerm.asc(o.seq)])
              ..limit(limit))
            .get();
        if (rows.isNotEmpty) {
          await (db.update(db.outbox)..where((o) => o.seq.isIn(rows.map((r) => r.seq))))
              .write(const OutboxCompanion(inFlight: Value(true)));
        }
        return rows;
      });

  @override
  Future<void> removeOp(String operationId) =>
      (db.delete(db.outbox)..where((o) => o.operationId.equals(operationId))).go();

  @override
  Future<void> releaseInFlight() =>
      db.update(db.outbox).write(const OutboxCompanion(inFlight: Value(false)));

  @override
  Future<String?> getValue(String key) async =>
      (await (db.select(db.keyValues)..where((k) => k.key.equals(key))).getSingleOrNull())?.value;

  @override
  Future<void> setValue(String key, String value) =>
      db.into(db.keyValues).insertOnConflictUpdate(KeyValuesCompanion.insert(key: key, value: value));

  @override
  Future<void> wipe() => db.transaction(() async {
        for (final t in db.allTables) {
          await db.delete(t).go();
        }
      });
}
