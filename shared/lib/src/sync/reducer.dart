import 'dart:convert';

import '../domain/local_store.dart';
import '../models/ops.dart';

/// Применяет операцию к локальному состоянию. Один и тот же код работает
/// и при действии пользователя, и при перебазировании: после ответа сервера
/// неподтверждённые операции заново накатываются поверх серверного состояния
/// (архитектура п. 4.4). Должен быть детерминированным и не писать в очередь.
Future<void> applyLocal(LocalStore s, String type, String entityId, Map<String, dynamic> p, DateTime now) async {
  switch (type) {
    case OpType.createProject:
      if (await s.project(entityId) != null) return;
      await s.putProject(ProjectsCompanion.insert(
        id: entityId,
        name: p['name'] as String,
        description: Value(p['description'] as String? ?? ''),
        createdAt: now,
        updatedAt: now,
      ));
    case OpType.updateProject:
      await s.patchProject(
          entityId,
          ProjectsCompanion(
            name: p.containsKey('name') ? Value(p['name'] as String) : const Value.absent(),
            description: p.containsKey('description') ? Value(p['description'] as String) : const Value.absent(),
            updatedAt: Value(now),
          ));
    case OpType.deleteProject:
      await s.patchProject(entityId, ProjectsCompanion(deletedAt: Value(now)));
    case OpType.restoreProject:
      await s.patchProject(entityId, const ProjectsCompanion(deletedAt: Value(null)));
    case OpType.createNode:
      if (await s.node(entityId) != null) return;
      await s.putNode(NodesCompanion.insert(
        id: entityId,
        projectId: p['project_id'] as String,
        parentId: Value(p['parent_id'] as String?),
        kind: p['kind'] as String,
        name: p['name'] as String,
        sortKey: p['sort_key'] as String,
        createdAt: now,
        updatedAt: now,
      ));
      await s.putContent(NodeContentsCompanion.insert(
        nodeId: entityId,
        rawContent: Value(p['raw_content'] as String? ?? ''),
      ));
    case OpType.renameNode:
      await s.patchNode(entityId, NodesCompanion(name: Value(p['name'] as String), updatedAt: Value(now)));
    case OpType.moveNode:
      await s.patchNode(
          entityId,
          NodesCompanion(
            parentId: Value(p['parent_id'] as String?),
            sortKey: Value(p['sort_key'] as String),
            updatedAt: Value(now),
          ));
    case OpType.changeKind:
      await s.patchNode(entityId, NodesCompanion(kind: Value(p['kind'] as String), updatedAt: Value(now)));
    case OpType.deleteNode:
      await s.patchNode(entityId, NodesCompanion(deletedAt: Value(now)));
    case OpType.restoreNode:
      await _restoreChain(s, entityId);
    case OpType.setRawContent:
      await s.patchContent(entityId, NodeContentsCompanion(rawContent: Value(p['content'] as String)));
      await s.patchNode(entityId, NodesCompanion(updatedAt: Value(now)));
    case OpType.restoreVersion:
      final v = await s.version(p['version_id'] as String);
      if (v == null) return;
      if (v.field == 'structured') {
        await applyLocal(s, OpType.setStructuredText, entityId, {'text': v.content}, now);
      } else {
        await s.patchContent(entityId, NodeContentsCompanion(rawContent: Value(v.content)));
      }
    case OpType.setStructuredText:
      final c = await s.content(entityId);
      final doc = c?.structuredContent == null ? null : jsonDecode(c!.structuredContent!) as Map<String, dynamic>;
      if (doc != null) {
        doc['formatted_text'] = p['text'] as String;
        await s.patchContent(entityId, NodeContentsCompanion(structuredContent: Value(jsonEncode(doc))));
      }
    case OpType.applyProposal:
    case OpType.dismissProposal:
      // Итог применения считает сервер (абзацы человека, статус); локально убираем плашку.
      await s.patchContent(entityId, const NodeContentsCompanion(structureProposal: Value(null)));
    case OpType.resolveConflict:
      await s.patchContent(entityId, NodeContentsCompanion(rawContent: Value(p['content'] as String)));
      await s.patchNode(entityId, NodesCompanion(hasConflict: const Value(false), updatedAt: Value(now)));
  }
}

Future<void> _restoreChain(LocalStore s, String id) async {
  String? cur = id;
  while (cur != null) {
    final n = await s.node(cur);
    if (n == null) return;
    if (n.deletedAt != null) {
      await s.patchNode(cur, const NodesCompanion(deletedAt: Value(null)));
    }
    cur = n.parentId;
  }
}
