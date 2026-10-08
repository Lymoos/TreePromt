import '../models/ops.dart';
import 'local_store.dart';

/// Линия в колонке отступа. Вариант панели C: рисуется только путь
/// от проекта до открытого файла (design/DECISIONS.md).
enum Guide { none, through, elbow }

enum RowType { project, folder, file }

class TreeRow {
  const TreeRow({
    required this.id,
    required this.projectId,
    required this.type,
    required this.name,
    required this.depth,
    required this.guides,
    this.kind,
    this.structureStatus = StructureStatus.none,
    this.childCount = 0,
    this.expanded = false,
    this.selected = false,
    this.hasConflict = false,
    this.syncError,
  });

  final String id;
  final String projectId;
  final RowType type;
  final String name;
  final int depth;

  /// Одна запись на колонку отступа (длина == depth).
  final List<Guide> guides;
  final String? kind;
  final String structureStatus;
  final int childCount;
  final bool expanded;
  final bool selected;
  final bool hasConflict;
  final String? syncError;
}

/// Строит плоский список строк дерева: Проекты → Папки → Файлы.
/// Удалённые узлы и всё под ними скрыты.
List<TreeRow> buildTree({
  required List<Project> projects,
  required List<TreeNode> nodes,
  required Set<String> expanded,
  String? selectedId,
}) {
  final children = <String, List<TreeNode>>{}; // ключ — parentId или 'p:<projectId>'
  final byId = {for (final n in nodes) n.id: n};
  for (final n in nodes) {
    if (n.deletedAt != null) continue;
    children.putIfAbsent(n.parentId ?? 'p:${n.projectId}', () => []).add(n);
  }
  for (final list in children.values) {
    list.sort((a, b) {
      final c = a.sortKey.compareTo(b.sortKey);
      return c != 0 ? c : a.id.compareTo(b.id);
    });
  }

  final rows = <_Row>[];
  void walk(String key, int depth) {
    for (final n in children[key] ?? const <TreeNode>[]) {
      final kids = children[n.id]?.length ?? 0;
      rows.add(_Row(
        TreeRow(
          id: n.id,
          projectId: n.projectId,
          type: n.kind == NodeKind.folder ? RowType.folder : RowType.file,
          name: n.name,
          depth: depth,
          guides: const [],
          kind: n.kind,
          structureStatus: n.structureStatus,
          childCount: kids,
          expanded: expanded.contains(n.id),
          selected: n.id == selectedId,
          hasConflict: n.hasConflict,
          syncError: n.syncError,
        ),
      ));
      if (n.kind == NodeKind.folder && expanded.contains(n.id)) walk(n.id, depth + 1);
    }
  }

  for (final p in projects) {
    if (p.deletedAt != null) continue;
    rows.add(_Row(TreeRow(
      id: p.id,
      projectId: p.id,
      type: RowType.project,
      name: p.name,
      depth: 0,
      guides: const [],
      childCount: children['p:${p.id}']?.length ?? 0,
      expanded: expanded.contains(p.id),
      selected: p.id == selectedId,
      syncError: p.syncError,
    )));
    if (expanded.contains(p.id)) walk('p:${p.id}', 1);
  }

  // Путь до выбранного: проект, папки, сам узел.
  final path = <String>[];
  final sel = selectedId == null ? null : byId[selectedId];
  if (sel != null) {
    String? cur = sel.id;
    while (cur != null) {
      path.insert(0, cur);
      cur = byId[cur]?.parentId;
    }
    path.insert(0, sel.projectId);
  }
  final index = {for (var i = 0; i < rows.length; i++) rows[i].row.id: i};
  final pathRows = [for (final id in path) index[id] ?? -1];

  return [
    for (var i = 0; i < rows.length; i++)
      rows[i].withGuides([
        for (var c = 0; c < rows[i].row.depth; c++) _guide(i, c, rows[i].row.depth, pathRows),
      ]),
  ];
}

Guide _guide(int i, int c, int depth, List<int> pathRows) {
  if (c + 1 >= pathRows.length) return Guide.none;
  final a = pathRows[c], b = pathRows[c + 1];
  if (a < 0 || b < 0) return Guide.none;
  if (i > a && i < b) return Guide.through;
  if (i == b && c == depth - 1) return Guide.elbow;
  return Guide.none;
}

class _Row {
  _Row(this.row);
  final TreeRow row;

  TreeRow withGuides(List<Guide> g) => TreeRow(
        id: row.id,
        projectId: row.projectId,
        type: row.type,
        name: row.name,
        depth: row.depth,
        guides: g,
        kind: row.kind,
        structureStatus: row.structureStatus,
        childCount: row.childCount,
        expanded: row.expanded,
        selected: row.selected,
        hasConflict: row.hasConflict,
        syncError: row.syncError,
      );
}
