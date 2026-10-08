import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'providers.dart';
import 'widgets/dialogs.dart';

/// Куда создавать новый узел.
typedef CreateTarget = ({String projectId, String? parentId, String label});

/// Действие над деревом. Как его показать (нижнее меню на телефоне,
/// контекстное меню на ПК) решает оболочка приложения.
class TreeAction {
  const TreeAction({
    required this.label,
    required this.run,
    this.caption,
    this.marker,
    this.icon,
    this.dividerBefore = false,
  });

  final String label;
  final String? caption;
  final MarkerType? marker;
  final IconData? icon;
  final bool dividerBefore;

  /// [context] — контекст экрана (меню к этому моменту уже закрыто).
  final Future<void> Function(BuildContext context) run;
}

class TreeActions {
  TreeActions(this.ref, {required this.openNode});

  final WidgetRef ref;

  /// Открыть файл: телефон — новый экран, ПК — правая панель.
  final void Function(String nodeId) openNode;

  TreeService get _tree => ref.read(treeServiceProvider);

  void expand(Iterable<String> ids) =>
      ref.read(expandedProvider.notifier).state = {...ref.read(expandedProvider), ...ids};

  /// Нажатие на строку: файл открывается, проект и папка раскрываются.
  void tap(TreeRow r) {
    ref.read(selectedProvider.notifier).state = r.id;
    if (r.type == RowType.file) {
      openNode(r.id);
      return;
    }
    final exp = ref.read(expandedProvider.notifier);
    final next = {...exp.state};
    next.contains(r.id) ? next.remove(r.id) : next.add(r.id);
    exp.state = next;
  }

  /// В выбранную папку, рядом с выбранным файлом или в корень выбранного проекта.
  Future<CreateTarget?> target() async {
    final store = ref.read(storeProvider);
    final sel = ref.read(selectedProvider);
    if (sel == null) return null;
    final p = await store.project(sel);
    if (p != null && p.deletedAt == null) return (projectId: p.id, parentId: null, label: p.name);
    final n = await store.node(sel);
    if (n == null || n.deletedAt != null) return null;
    if (n.kind == NodeKind.folder) return (projectId: n.projectId, parentId: n.id, label: n.name);
    final parent = n.parentId == null ? null : await store.node(n.parentId!);
    final project = await store.project(n.projectId);
    return (projectId: n.projectId, parentId: n.parentId, label: parent?.name ?? project?.name ?? '');
  }

  Future<void> createProject(BuildContext context) async {
    final name = await askName(context, title: 'Новый проект', action: 'Создать');
    if (name == null) return;
    final id = await _tree.createProject(name);
    expand([id]);
    ref.read(selectedProvider.notifier).state = id;
  }

  Future<void> _create(BuildContext context, CreateTarget target, String kind, String title) async {
    final name = await askName(context, title: title, action: 'Создать');
    if (name == null) return;
    final id = await _tree.createNode(projectId: target.projectId, parentId: target.parentId, kind: kind, name: name);
    expand([target.projectId, if (target.parentId != null) target.parentId!]);
    ref.read(selectedProvider.notifier).state = id;
    if (kind != NodeKind.folder) openNode(id);
  }

  List<TreeAction> createMenu(CreateTarget? target) => [
        if (target != null) ...[
          TreeAction(
            label: 'AI Task',
            caption: 'Задача, которую можно структурировать для Claude',
            marker: MarkerType.aiNone,
            run: (c) => _create(c, target, NodeKind.aiTask, 'Новая задача'),
          ),
          TreeAction(
            label: 'Raw Note',
            caption: 'Черновик: мысли на ходу',
            marker: MarkerType.rawNote,
            run: (c) => _create(c, target, NodeKind.rawNote, 'Новый черновик'),
          ),
          TreeAction(
            label: 'Папка',
            icon: Icons.subdirectory_arrow_right,
            run: (c) => _create(c, target, NodeKind.folder, 'Новая папка'),
          ),
        ],
        TreeAction(
          label: 'Проект',
          marker: MarkerType.project,
          dividerBefore: target != null,
          run: createProject,
        ),
      ];

  List<TreeAction> rowMenu(TreeRow r) {
    if (r.type == RowType.project) {
      return [
        TreeAction(
          label: 'Переименовать',
          run: (c) async {
            final name = await askName(c, title: 'Переименовать проект', initial: r.name);
            if (name != null) await _tree.renameProject(r.id, name);
          },
        ),
        TreeAction(
          label: 'Удалить проект',
          dividerBefore: true,
          run: (c) async {
            if (await confirm(c,
                title: 'Удалить «${r.name}»?',
                message: 'Проект со всеми заметками уйдёт в корзину. Его можно вернуть.',
                action: 'Удалить')) {
              await _tree.deleteProject(r.id);
            }
          },
        ),
      ];
    }
    final isTask = r.kind == NodeKind.aiTask;
    return [
      if (r.type == RowType.file) TreeAction(label: 'Открыть', run: (_) async => tap(r)),
      TreeAction(
        label: 'Переименовать',
        run: (c) async {
          final name = await askName(c, title: 'Переименовать', initial: r.name);
          if (name != null) await _tree.rename(r.id, name);
        },
      ),
      TreeAction(label: 'Переместить', run: (c) => move(c, r)),
      if (r.type == RowType.file)
        TreeAction(
          label: isTask ? 'Сделать черновиком' : 'Сделать AI Task',
          marker: isTask ? MarkerType.rawNote : MarkerType.aiNone,
          run: (_) => _tree.changeKind(r.id, isTask ? NodeKind.rawNote : NodeKind.aiTask),
        ),
      TreeAction(
        label: 'Удалить',
        dividerBefore: true,
        run: (c) async {
          if (await confirm(c,
              title: 'Удалить «${r.name}»?',
              message: r.type == RowType.folder
                  ? 'Папка со всем содержимым уйдёт в корзину. Её можно вернуть.'
                  : 'Заметка уйдёт в корзину. Её можно вернуть.',
              action: 'Удалить')) {
            await _tree.delete(r.id);
          }
        },
      ),
    ];
  }

  /// Выбор папки назначения внутри того же проекта.
  Future<void> move(BuildContext context, TreeRow r) async {
    final nodes = ref.read(nodesProvider).valueOrNull ?? const <TreeNode>[];
    final project = await ref.read(storeProvider).project(r.projectId);
    if (project == null || !context.mounted) return;

    // Папки проекта, кроме самого узла и его потомков.
    final byParent = <String?, List<TreeNode>>{};
    for (final n in nodes) {
      if (n.projectId == r.projectId && n.kind == NodeKind.folder && n.deletedAt == null && n.id != r.id) {
        byParent.putIfAbsent(n.parentId, () => []).add(n);
      }
    }
    final dests = <({String? id, String name, int depth})>[(id: null, name: project.name, depth: 0)];
    void walk(String? parent, int depth) {
      final list = [...?byParent[parent]]..sort((a, b) => a.sortKey.compareTo(b.sortKey));
      for (final f in list) {
        dests.add((id: f.id, name: f.name, depth: depth));
        walk(f.id, depth + 1);
      }
    }

    walk(null, 1);
    final c = context.pt;
    final picked = await showDialog<({String? id})>(
      context: context,
      builder: (context) => SimpleDialog(
        title: Text('Переместить «${r.name}»'),
        children: [
          for (final d in dests)
            SimpleDialogOption(
              onPressed: () => Navigator.pop(context, (id: d.id)),
              padding: EdgeInsets.fromLTRB(24 + d.depth * 16.0, 10, 24, 10),
              child: Row(children: [
                if (d.id == null) ...[Marker(MarkerType.project, size: 9, color: c.fg), const SizedBox(width: 10)],
                Expanded(
                  child: Text(d.name,
                      style: ui(15, weight: d.id == null ? FontWeight.w600 : FontWeight.w500, color: c.fg)),
                ),
              ]),
            ),
        ],
      ),
    );
    if (picked == null) return;
    await _tree.move(r.id, picked.id);
    expand([r.projectId, if (picked.id != null) picked.id!]);
  }
}
