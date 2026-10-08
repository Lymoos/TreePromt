import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

/// Фейковый сервер: отвечает тем, что подготовил тест, и запоминает запросы.
class FakeApi implements SyncApi {
  final pushed = <List<Map<String, dynamic>>>[];
  List<Map<String, dynamic>> Function(List<Map<String, dynamic>> ops)? onPush;
  final pages = <Map<String, dynamic>>[];
  int pullCalls = 0;
  Object? failWith;

  @override
  Future<List<Map<String, dynamic>>> push(List<Map<String, dynamic>> operations) async {
    if (failWith != null) throw failWith!;
    pushed.add(operations);
    return onPush?.call(operations) ?? [for (final o in operations) {'operation_id': o['operation_id'], 'result': 'applied'}];
  }

  @override
  Future<Map<String, dynamic>> pull(int cursor, {int limit = 500}) async {
    pullCalls++;
    if (pages.isEmpty) return {'projects': [], 'nodes': [], 'versions': [], 'cursor': cursor, 'has_more': false};
    return pages.removeAt(0);
  }

  @override
  Future<String> freshAccessToken() async => 'token';

  @override
  Uri get wsUrl => Uri.parse('ws://localhost/ws');
}

Map<String, dynamic> serverNode(String id, String project, {required int rev, required String text, int rawRev = 1, String name = 'Заметка'}) => {
      'id': id,
      'project_id': project,
      'parent_id': null,
      'kind': 'ai_task',
      'name': name,
      'sort_key': 'V',
      'revision': rev,
      'has_conflict': false,
      'created_at': '2026-10-08T00:00:00Z',
      'updated_at': '2026-10-08T00:00:00Z',
      'deleted_at': null,
      'raw_content': text,
      'raw_revision': rawRev,
      'structured_content': null,
      'structured_revision': 0,
      'structured_from_revision': null,
      'structure_status': 'none',
    };

void main() {
  late DriftLocalStore store;
  late TreeService tree;
  late FakeApi api;
  late SyncEngine engine;

  setUp(() {
    store = memoryStore();
    tree = TreeService(store);
    api = FakeApi();
    engine = SyncEngine(store: store, api: api);
  });
  tearDown(() => store.db.close());

  Future<List<OutboxEntry>> outbox() => store.db.select(store.db.outbox).get();

  test('actions work offline: data is local and queued in one transaction', () async {
    final p = await tree.createProject('PromptTree');
    final n = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'Дерево', text: 'идея 🌳');
    expect((await store.content(n))!.rawContent, 'идея 🌳');
    final q = await outbox();
    expect(q.map((o) => o.type), [OpType.createProject, OpType.createNode]);
    expect(q[0].seq < q[1].seq, isTrue);
  });

  test('typing coalesces into one pending operation with the original base', () async {
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'n');
    await store.patchContent(n, const NodeContentsCompanion(rawRevision: Value(7)));
    for (final t in ['а', 'аб', 'абв']) {
      await tree.setText(n, t);
    }
    final ops = (await outbox()).where((o) => o.type == OpType.setRawContent).toList();
    expect(ops, hasLength(1));
    expect(ops.single.baseRevision, 7);
    expect(jsonDecode(ops.single.payload)['content'], 'абв');
  });

  test('an in-flight operation is never rewritten', () async {
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'n');
    await tree.setText(n, 'v1');
    await store.takeBatch(100); // ушло на сервер
    await tree.setText(n, 'v2');
    final ops = (await outbox()).where((o) => o.type == OpType.setRawContent).toList();
    expect(ops.map((o) => jsonDecode(o.payload)['content']), ['v1', 'v2']);
  });

  test('server state is applied and pending local edits are replayed on top', () async {
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'n', text: 'v0');
    await engine.syncNow(); // create_* подтверждены (фейк отвечает applied без состояния)

    // Пользователь печатает, а в это время с другого устройства пришло переименование.
    await tree.setText(n, 'моя офлайн-правка');
    api.failWith = NetworkException('offline');
    await engine.syncNow();
    expect(engine.state.phase, SyncPhase.offline);

    api.failWith = null;
    api.onPush = (ops) => [
          for (final o in ops)
            {
              'operation_id': o['operation_id'],
              'result': 'applied',
              'node': serverNode(n, p, rev: 10, text: 'моя офлайн-правка', rawRev: 10, name: 'Имя с ПК'),
            }
        ];
    await engine.syncNow();
    final node = await store.node(n);
    expect(node!.name, 'Имя с ПК');
    expect((await store.content(n))!.rawContent, 'моя офлайн-правка');
    expect((await store.content(n))!.rawRevision, 10);
    expect(await outbox(), isEmpty);
    expect(engine.state.phase, SyncPhase.idle);
  });

  test('pull does not overwrite unsent local text', () async {
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'n', text: 'v0');
    await engine.syncNow();
    await tree.setText(n, 'неотправленное');
    // Сервер пока недоступен для push, но pull отдаёт чужое переименование.
    final applier = ServerStateApplier(store);
    await applier.node(serverNode(n, p, rev: 5, text: 'v0', name: 'Новое имя'));
    expect((await store.node(n))!.name, 'Новое имя');
    expect((await store.content(n))!.rawContent, 'неотправленное');
  });

  test('rejected operation is marked, other operations continue', () async {
    final p = await tree.createProject('P');
    final a = await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'a');
    final b = await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'b');
    api.onPush = (ops) => [
          for (final o in ops)
            {
              'operation_id': o['operation_id'],
              'result': o['entity_id'] == a ? 'rejected' : 'applied',
              if (o['entity_id'] == a) 'error': {'code': 'invalid_parent', 'message': 'x'},
            }
        ];
    await engine.syncNow();
    expect((await store.node(a))!.syncError, 'invalid_parent');
    expect((await store.node(b))!.syncError, isNull);
    expect(await outbox(), isEmpty);
  });

  test('partial failure keeps unprocessed operations for retry', () async {
    final p = await tree.createProject('P');
    await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'a');
    api.onPush = (ops) => [
          {'operation_id': ops.first['operation_id'], 'result': 'applied'}
        ];
    await engine.syncNow();
    final left = await outbox();
    expect(left, hasLength(1));
    expect(left.single.type, OpType.createNode);
    expect(left.single.inFlight, isFalse);
    await engine.stop();
  });

  test('pull pages are applied with the cursor in the same transaction', () async {
    api.pages.addAll([
      {
        'projects': [
          {'id': 'p', 'name': 'P', 'description': '', 'revision': 1, 'role': 'owner',
           'created_at': '2026-10-08T00:00:00Z', 'updated_at': '2026-10-08T00:00:00Z', 'deleted_at': null}
        ],
        'nodes': [serverNode('n', 'p', rev: 2, text: 'x')],
        'versions': [],
        'cursor': 2,
        'has_more': true,
      },
      {
        'projects': [],
        'nodes': [],
        'versions': [
          {'id': 'v', 'node_id': 'n', 'project_id': 'p', 'field': 'raw', 'content': 'старое', 'at_revision': 1,
           'reason': 'conflict_server', 'device_id': null, 'server_revision': 3, 'created_at': '2026-10-08T00:00:00Z'}
        ],
        'cursor': 3,
        'has_more': false,
      },
    ]);
    await engine.syncNow();
    expect(await store.getValue(cursorKey), '3');
    expect((await store.project('p'))!.name, 'P');
    expect((await store.version('v'))!.content, 'старое');
    expect(api.pullCalls, 2);
  });

  test('stale server state never rolls back a newer one', () async {
    final applier = ServerStateApplier(store);
    await applier.project({'id': 'p', 'name': 'P', 'description': '', 'revision': 1, 'role': 'owner',
      'created_at': '2026-10-08T00:00:00Z', 'updated_at': '2026-10-08T00:00:00Z', 'deleted_at': null});
    await applier.node(serverNode('n', 'p', rev: 9, text: 'новое'));
    await applier.node(serverNode('n', 'p', rev: 4, text: 'старое'));
    expect((await store.content('n'))!.rawContent, 'новое');
  });

  test('copy-then-delete removes the task softly and it can be restored', () async {
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'n');
    await tree.delete(n);
    expect((await store.node(n))!.deletedAt, isNotNull);
    await tree.restore(n);
    expect((await store.node(n))!.deletedAt, isNull);
  });
}
