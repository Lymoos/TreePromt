import 'dart:convert';

import '../domain/local_store.dart';
import '../models/ops.dart';
import 'reducer.dart';

DateTime _t(Object? v) => DateTime.parse(v as String).toUtc();
DateTime? _tn(Object? v) => v == null ? null : _t(v);

/// Записывает подтверждённое сервером состояние и заново применяет поверх него
/// неотправленные операции этого объекта (архитектура п. 4.4).
class ServerStateApplier {
  ServerStateApplier(this.store, {DateTime Function()? clock}) : _now = clock ?? DateTime.now;

  final LocalStore store;
  final DateTime Function() _now;

  Future<void> project(Map<String, dynamic> j) async {
    final id = j['id'] as String;
    final local = await store.project(id);
    final rev = j['revision'] as int;
    if (local != null && local.revision > rev) return; // у нас уже более новое состояние
    await store.putProject(ProjectsCompanion.insert(
      id: id,
      name: j['name'] as String,
      description: Value(j['description'] as String? ?? ''),
      revision: Value(rev),
      role: Value(j['role'] as String? ?? 'owner'),
      createdAt: _t(j['created_at']),
      updatedAt: _t(j['updated_at']),
      deletedAt: Value(_tn(j['deleted_at'])),
      syncError: const Value(null),
    ));
    await _replay(id);
  }

  Future<void> node(Map<String, dynamic> j) async {
    final id = j['id'] as String;
    final local = await store.node(id);
    final rev = j['revision'] as int;
    if (local != null && local.revision > rev) return;
    await store.putNode(NodesCompanion.insert(
      id: id,
      projectId: j['project_id'] as String,
      parentId: Value(j['parent_id'] as String?),
      kind: j['kind'] as String,
      name: j['name'] as String,
      sortKey: j['sort_key'] as String,
      revision: Value(rev),
      hasConflict: Value(j['has_conflict'] as bool? ?? false),
      structureStatus: Value(j['structure_status'] as String? ?? StructureStatus.none),
      createdAt: _t(j['created_at']),
      updatedAt: _t(j['updated_at']),
      deletedAt: Value(_tn(j['deleted_at'])),
      syncError: const Value(null),
    ));
    final structured = j['structured_content'];
    await store.putContent(NodeContentsCompanion.insert(
      nodeId: id,
      rawContent: Value(j['raw_content'] as String? ?? ''),
      rawRevision: Value(j['raw_revision'] as int? ?? 0),
      structuredContent: Value(structured == null ? null : jsonEncode(structured)),
    ));
    await _replay(id);
  }

  Future<void> version(Map<String, dynamic> j) => store.putVersion(ContentVersionsCompanion.insert(
        id: j['id'] as String,
        nodeId: j['node_id'] as String,
        projectId: j['project_id'] as String,
        field: j['field'] as String,
        content: j['content'] as String,
        atRevision: j['at_revision'] as int,
        reason: j['reason'] as String,
        serverRevision: j['server_revision'] as int,
        createdAt: _t(j['created_at']),
      ));

  Future<void> markRejected(String type, String entityId, String code) async {
    if (OpType.isProjectOp(type)) {
      await store.patchProject(entityId, ProjectsCompanion(syncError: Value(code)));
    } else {
      await store.patchNode(entityId, NodesCompanion(syncError: Value(code)));
    }
  }

  Future<void> _replay(String entityId) async {
    for (final op in await store.pendingOpsFor(entityId)) {
      await applyLocal(store, op.type, op.entityId, jsonDecode(op.payload) as Map<String, dynamic>, _now());
    }
  }
}
