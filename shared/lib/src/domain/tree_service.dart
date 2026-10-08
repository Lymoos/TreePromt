import 'dart:convert';

import '../core/ids.dart';
import '../core/sort_key.dart';
import '../models/ops.dart';
import '../sync/reducer.dart';
import 'local_store.dart';

/// Действия пользователя над деревом. Каждое действие — одна локальная транзакция:
/// изменение применяется к локальной БД и попадает в очередь на сервер.
/// Сеть здесь не участвует, поэтому всё работает офлайн.
class TreeService {
  TreeService(this.store, {DateTime Function()? clock}) : _now = clock ?? DateTime.now;

  final LocalStore store;
  final DateTime Function() _now;

  Future<void> _do(String type, String entityId, Map<String, dynamic> payload, {int baseRevision = 0}) =>
      store.transaction(() async {
        await applyLocal(store, type, entityId, payload, _now());
        await store.enqueue(
          operationId: newId(),
          type: type,
          entityId: entityId,
          baseRevision: baseRevision,
          payload: jsonEncode(payload),
        );
      });

  // ── проекты ──

  Future<String> createProject(String name, {String description = ''}) async {
    final id = newId();
    await _do(OpType.createProject, id, {'name': name.trim(), 'description': description});
    return id;
  }

  Future<void> renameProject(String id, String name) => _do(OpType.updateProject, id, {'name': name.trim()});

  Future<void> deleteProject(String id) => _do(OpType.deleteProject, id, {});

  Future<void> restoreProject(String id) => _do(OpType.restoreProject, id, {});

  // ── узлы ──

  Future<String> createNode({
    required String projectId,
    String? parentId,
    required String kind,
    required String name,
    String text = '',
  }) async {
    final id = newId();
    final sortKey = keyAfter(await store.lastChildKey(projectId, parentId));
    await _do(OpType.createNode, id, {
      'project_id': projectId,
      'parent_id': parentId,
      'kind': kind,
      'name': name.trim(),
      'sort_key': sortKey,
      'raw_content': text,
    });
    return id;
  }

  Future<void> rename(String id, String name) => _do(OpType.renameNode, id, {'name': name.trim()});

  /// Переносит узел в конец списка детей [parentId] (null — корень проекта).
  Future<void> move(String id, String? parentId) async {
    final node = await store.node(id);
    if (node == null) return;
    final key = keyAfter(await store.lastChildKey(node.projectId, parentId));
    await _do(OpType.moveNode, id, {'parent_id': parentId, 'sort_key': key});
  }

  /// Переносит узел между двумя соседями (для перетаскивания).
  Future<void> reorder(String id, String? parentId, {String? afterKey, String? beforeKey}) =>
      _do(OpType.moveNode, id, {'parent_id': parentId, 'sort_key': keyBetween(afterKey, beforeKey)});

  Future<void> changeKind(String id, String kind) => _do(OpType.changeKind, id, {'kind': kind});

  /// Мягкое удаление: узел уходит в корзину (ТЗ п. 6.4).
  Future<void> delete(String id) => _do(OpType.deleteNode, id, {});

  Future<void> restore(String id) => _do(OpType.restoreNode, id, {});

  // ── текст ──

  /// Сохраняет текст. Пока операция по узлу не ушла на сервер, новые правки
  /// заменяют её payload, а base_revision остаётся прежним (архитектура п. 4.1).
  Future<void> setText(String nodeId, String text) => store.transaction(() async {
        final content = await store.content(nodeId);
        if (content == null || content.rawContent == text) return;
        await applyLocal(store, OpType.setRawContent, nodeId, {'content': text}, _now());
        final payload = jsonEncode({'content': text});
        final last = await store.lastOpFor(nodeId);
        if (last != null && last.type == OpType.setRawContent && !last.inFlight) {
          await store.replaceOpPayload(last.seq, payload);
          return;
        }
        await store.enqueue(
          operationId: newId(),
          type: OpType.setRawContent,
          entityId: nodeId,
          baseRevision: content.rawRevision,
          payload: payload,
        );
      });

  Future<void> restoreVersion(String nodeId, String versionId) =>
      _do(OpType.restoreVersion, nodeId, {'version_id': versionId});

  /// Пользователь выбрал или собрал итоговый текст после конфликта.
  Future<void> resolveConflict(String nodeId, String text) =>
      _do(OpType.resolveConflict, nodeId, {'content': text});
}
