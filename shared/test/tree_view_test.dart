import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

final _t = DateTime.utc(2026, 10, 8);

Project _p(String id) => Project(id: id, name: id, description: '', revision: 1, role: 'owner', createdAt: _t, updatedAt: _t);

TreeNode _n(String id, String project, String? parent, String kind, {String key = 'a', DateTime? deleted}) => TreeNode(
      id: id,
      projectId: project,
      parentId: parent,
      kind: kind,
      name: id,
      sortKey: key,
      revision: 1,
      hasConflict: false,
      structureStatus: 'none',
      createdAt: _t,
      updatedAt: _t,
      deletedAt: deleted,
    );

void main() {
  // p1
  //   f1
  //     n1
  //     n2   ← выбран
  //   n3
  final projects = [_p('p1')];
  final nodes = [
    _n('f1', 'p1', null, NodeKind.folder, key: 'a'),
    _n('n1', 'p1', 'f1', NodeKind.rawNote, key: 'a'),
    _n('n2', 'p1', 'f1', NodeKind.aiTask, key: 'b'),
    _n('n3', 'p1', null, NodeKind.aiTask, key: 'b'),
  ];

  test('draws only the branch to the selected file', () {
    final rows = buildTree(projects: projects, nodes: nodes, expanded: {'p1', 'f1'}, selectedId: 'n2');
    expect(rows.map((r) => r.id), ['p1', 'f1', 'n1', 'n2', 'n3']);
    Map<String, List<Guide>> g = {for (final r in rows) r.id: r.guides};
    expect(g['p1'], isEmpty);
    expect(g['f1'], [Guide.elbow]); // проект → папка
    expect(g['n1'], [Guide.none, Guide.through]); // путь проходит мимо n1 к n2
    expect(g['n2'], [Guide.none, Guide.elbow]);
    expect(g['n3'], [Guide.none]); // ниже выбранного линий нет
    expect(rows.firstWhere((r) => r.id == 'n2').selected, isTrue);
  });

  test('no lines without selection, collapsed folders hide children', () {
    final rows = buildTree(projects: projects, nodes: nodes, expanded: {'p1'});
    expect(rows.map((r) => r.id), ['p1', 'f1', 'n3']);
    expect(rows.every((r) => r.guides.every((g) => g == Guide.none)), isTrue);
    expect(rows.firstWhere((r) => r.id == 'f1').childCount, 2);
  });

  test('deleted nodes and their subtree are hidden', () {
    final withDeleted = [...nodes.where((n) => n.id != 'f1'), _n('f1', 'p1', null, NodeKind.folder, deleted: _t)];
    final rows = buildTree(projects: projects, nodes: withDeleted, expanded: {'p1', 'f1'});
    expect(rows.map((r) => r.id), ['p1', 'n3']);
  });
}
