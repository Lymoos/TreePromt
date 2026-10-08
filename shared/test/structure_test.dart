import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

void main() {
  late DriftLocalStore store;
  late TreeService tree;

  setUp(() {
    store = memoryStore();
    tree = TreeService(store);
  });
  tearDown(() => store.db.close());

  Future<String> taskWithStructure() async {
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'n', text: 'исходник');
    await ServerStateApplier(store).node({
      'id': n, 'project_id': p, 'parent_id': null, 'kind': 'ai_task', 'name': 'n', 'sort_key': 'V',
      'revision': 5, 'has_conflict': false, 'created_at': '2026-10-08T00:00:00Z', 'updated_at': '2026-10-08T00:00:00Z',
      'deleted_at': null, 'raw_content': 'исходник', 'raw_revision': 2, 'structured_revision': 5,
      'structured_content': {'role': 'Act as X.', 'formatted_text': '## Задача\n- пункт', 'human_paragraphs': []},
      'structure_status': 'done',
      'structure_proposal': {'request_id': 'r1', 'reason': 'flagged', 'findings': [
        {'code': 'new_term', 'detail': '«Redux» нет в исходнике'}
      ], 'generated': {'role': 'Act as Y.', 'formatted_text': 'Redux'}},
    });
    return n;
  }

  test('server structure and proposal are parsed', () async {
    final n = await taskWithStructure();
    final c = (await store.content(n))!;
    final doc = StructuredDoc.tryParse(c.structuredContent)!;
    expect(doc.finalTask, 'Act as X.\n\n## Задача\n- пункт');
    expect(c.structuredRevision, 5);
    final p = StructureProposal.tryParse(c.structureProposal)!;
    expect(p.reason, StructureProposal.flagged);
    expect(p.findings.single.detail, contains('Redux'));
    expect(p.generated!.formattedText, 'Redux');
  });

  test('editing structured text is local-first and coalesced with the right base', () async {
    final n = await taskWithStructure();
    await tree.setStructuredText(n, '## Задача\n- пункт\n\nмой абзац');
    await tree.setStructuredText(n, '## Задача\n- пункт\n\nмой абзац, дописан');
    final doc = StructuredDoc.tryParse((await store.content(n))!.structuredContent)!;
    expect(doc.formattedText, endsWith('дописан'));
    final ops = (await store.db.select(store.db.outbox).get()).where((o) => o.type == OpType.setStructuredText).toList();
    expect(ops, hasLength(1));
    expect(ops.single.baseRevision, 5);
    expect(jsonDecode(ops.single.payload)['text'], endsWith('дописан'));
  });

  test('dismissing a proposal hides it immediately and queues the operation', () async {
    final n = await taskWithStructure();
    await tree.dismissProposal(n, 'r1');
    expect((await store.content(n))!.structureProposal, isNull);
    final op = (await store.db.select(store.db.outbox).get()).last;
    expect(op.type, OpType.dismissProposal);
    expect(jsonDecode(op.payload)['request_id'], 'r1');
  });

  test('restoring a structured version changes the structure, not the raw text', () async {
    final n = await taskWithStructure();
    await store.putVersion(ContentVersionsCompanion.insert(
      id: 'v1', nodeId: n, projectId: 'p', field: 'structured', content: 'старая структура',
      atRevision: 3, reason: 'before_structure', serverRevision: 6, createdAt: DateTime.utc(2026, 10, 8),
    ));
    await tree.restoreVersion(n, 'v1');
    final c = (await store.content(n))!;
    expect(c.rawContent, 'исходник');
    expect(StructuredDoc.tryParse(c.structuredContent)!.formattedText, 'старая структура');
  });
}
