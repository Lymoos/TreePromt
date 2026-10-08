import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../state/providers.dart';
import '../widgets/dialogs.dart';
import '../widgets/sync_badge.dart';
import 'editor_screen.dart';
import 'trash_screen.dart';

class TreeScreen extends ConsumerWidget {
  const TreeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final rows = ref.watch(treeRowsProvider);
    final loaded = ref.watch(projectsProvider).hasValue;
    final session = ref.watch(sessionProvider);
    final authLost = ref.watch(syncStateProvider).valueOrNull?.phase == SyncPhase.authRequired;

    return Scaffold(
      appBar: AppBar(
        titleSpacing: 16,
        title: const Row(children: [AppIcon(size: 22), SizedBox(width: 10), Text('PromptTree')]),
        actions: [
          const SyncBadge(),
          PopupMenuButton<String>(
            key: const Key('menu'),
            icon: Icon(Icons.more_horiz, color: c.fg),
            color: c.bg,
            onSelected: (v) async {
              switch (v) {
                case 'trash':
                  Navigator.push(context, MaterialPageRoute(builder: (_) => const TrashScreen()));
                case 'login':
                  ref.read(sessionProvider.notifier).state =
                      Session(mode: SessionMode.none, serverUrl: session.serverUrl);
                case 'logout':
                  await _logout(context, ref);
              }
            },
            itemBuilder: (_) => [
              const PopupMenuItem(value: 'trash', child: Text('Корзина')),
              if (session.mode == SessionMode.server)
                const PopupMenuItem(value: 'logout', child: Text('Выйти'))
              else
                const PopupMenuItem(value: 'login', child: Text('Подключить сервер')),
            ],
          ),
          const SizedBox(width: 4),
        ],
      ),
      body: Column(children: [
        if (authLost)
          _Notice(
            text: 'Сессия истекла. Заметки сохранены на телефоне, войдите снова, чтобы они ушли на сервер.',
            action: 'Войти',
            onAction: () => ref.read(sessionProvider.notifier).state =
                Session(mode: SessionMode.none, serverUrl: session.serverUrl),
          ),
        Expanded(
          child: !loaded
              ? const SizedBox.shrink()
              : rows.isEmpty
                  ? _Empty(onCreate: () => _createProject(context, ref))
                  : TreePanel(
                      rows: rows,
                      rowHeight: 40,
                      onTap: (r) => _tap(context, ref, r),
                      onLongPress: (r) => _actions(context, ref, r),
                    ),
        ),
      ]),
      floatingActionButton: rows.isEmpty
          ? null
          : FloatingActionButton(
              key: const Key('create'),
              backgroundColor: c.fg,
              foregroundColor: c.bg,
              elevation: 0,
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
              onPressed: () => _createSheet(context, ref),
              child: const Icon(Icons.add),
            ),
    );
  }

  Future<void> _logout(BuildContext context, WidgetRef ref) async {
    final pending = ref.read(pendingCountProvider).valueOrNull ?? 0;
    if (pending > 0) {
      final ok = await confirm(
        context,
        title: 'Выйти?',
        message: 'Ещё $pending изменений не ушли на сервер. Они останутся на телефоне и отправятся после следующего входа.',
        action: 'Выйти',
      );
      if (!ok) return;
    }
    await ref.read(apiProvider)?.logout();
    final session = ref.read(sessionProvider);
    ref.read(sessionProvider.notifier).state = Session(mode: SessionMode.none, serverUrl: session.serverUrl);
  }

  void _tap(BuildContext context, WidgetRef ref, TreeRow r) {
    ref.read(selectedProvider.notifier).state = r.id;
    if (r.type == RowType.file) {
      Navigator.push(context, MaterialPageRoute(builder: (_) => EditorScreen(nodeId: r.id)));
      return;
    }
    final exp = ref.read(expandedProvider.notifier);
    final next = {...exp.state};
    next.contains(r.id) ? next.remove(r.id) : next.add(r.id);
    exp.state = next;
  }

  void _expand(WidgetRef ref, Iterable<String> ids) =>
      ref.read(expandedProvider.notifier).state = {...ref.read(expandedProvider), ...ids};

  Future<void> _createProject(BuildContext context, WidgetRef ref) async {
    final name = await askName(context, title: 'Новый проект', action: 'Создать');
    if (name == null) return;
    final id = await ref.read(treeServiceProvider).createProject(name);
    _expand(ref, [id]);
    ref.read(selectedProvider.notifier).state = id;
  }

  /// Куда создавать: в выбранную папку, рядом с выбранным файлом или в корень выбранного проекта.
  Future<({String projectId, String? parentId, String label})?> _target(WidgetRef ref) async {
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

  Future<void> _createSheet(BuildContext context, WidgetRef ref) async {
    final target = await _target(ref);
    if (!context.mounted) return;
    final c = context.pt;
    Future<void> create(String kind, String title) async {
      Navigator.pop(context);
      final name = await askName(context, title: title, action: 'Создать');
      if (name == null || target == null) return;
      final id = await ref.read(treeServiceProvider).createNode(
            projectId: target.projectId,
            parentId: target.parentId,
            kind: kind,
            name: name,
          );
      _expand(ref, [target.projectId, if (target.parentId != null) target.parentId!]);
      ref.read(selectedProvider.notifier).state = id;
      if (kind != NodeKind.folder && context.mounted) {
        Navigator.push(context, MaterialPageRoute(builder: (_) => EditorScreen(nodeId: id)));
      }
    }

    await showSheet<void>(
      context,
      title: target == null ? 'СОЗДАТЬ' : 'СОЗДАТЬ В «${target.label.toUpperCase()}»',
      children: [
        if (target != null) ...[
          SheetItem(
            label: 'AI Task',
            caption: 'Задача, которую можно структурировать для Claude',
            leading: Marker(MarkerType.aiNone, color: c.fg),
            onTap: () => create(NodeKind.aiTask, 'Новая задача'),
          ),
          SheetItem(
            label: 'Raw Note',
            caption: 'Черновик: мысли на ходу',
            leading: Marker(MarkerType.rawNote, color: c.fg),
            onTap: () => create(NodeKind.rawNote, 'Новый черновик'),
          ),
          SheetItem(
            label: 'Папка',
            leading: Icon(Icons.subdirectory_arrow_right, size: 16, color: c.fg),
            onTap: () => create(NodeKind.folder, 'Новая папка'),
          ),
          Divider(color: c.line, indent: 20, endIndent: 20),
        ],
        SheetItem(
          label: 'Проект',
          leading: Marker(MarkerType.project, size: 9, color: c.fg),
          onTap: () {
            Navigator.pop(context);
            _createProject(context, ref);
          },
        ),
      ],
    );
  }

  Future<void> _actions(BuildContext context, WidgetRef ref, TreeRow r) async {
    final tree = ref.read(treeServiceProvider);
    final c = context.pt;
    void close() => Navigator.pop(context);

    if (r.type == RowType.project) {
      await showSheet<void>(context, title: r.name.toUpperCase(), children: [
        SheetItem(
          label: 'Переименовать',
          onTap: () async {
            close();
            final name = await askName(context, title: 'Переименовать проект', initial: r.name);
            if (name != null) await tree.renameProject(r.id, name);
          },
        ),
        SheetItem(
          label: 'Удалить проект',
          onTap: () async {
            close();
            if (await confirm(context,
                title: 'Удалить «${r.name}»?',
                message: 'Проект со всеми заметками уйдёт в корзину. Его можно вернуть.',
                action: 'Удалить')) {
              await tree.deleteProject(r.id);
            }
          },
        ),
      ]);
      return;
    }

    await showSheet<void>(context, title: r.name.toUpperCase(), children: [
      SheetItem(
        label: 'Переименовать',
        onTap: () async {
          close();
          final name = await askName(context, title: 'Переименовать', initial: r.name);
          if (name != null) await tree.rename(r.id, name);
        },
      ),
      SheetItem(
        label: 'Переместить',
        onTap: () async {
          close();
          await _move(context, ref, r);
        },
      ),
      if (r.type == RowType.file)
        SheetItem(
          label: r.kind == NodeKind.aiTask ? 'Сделать черновиком' : 'Сделать AI Task',
          leading: Marker(r.kind == NodeKind.aiTask ? MarkerType.rawNote : MarkerType.aiNone, color: c.fg),
          onTap: () async {
            close();
            await tree.changeKind(r.id, r.kind == NodeKind.aiTask ? NodeKind.rawNote : NodeKind.aiTask);
          },
        ),
      SheetItem(
        label: 'Удалить',
        onTap: () async {
          close();
          if (await confirm(context,
              title: 'Удалить «${r.name}»?',
              message: r.type == RowType.folder
                  ? 'Папка со всем содержимым уйдёт в корзину. Её можно вернуть.'
                  : 'Заметка уйдёт в корзину. Её можно вернуть.',
              action: 'Удалить')) {
            await tree.delete(r.id);
          }
        },
      ),
    ]);
  }

  Future<void> _move(BuildContext context, WidgetRef ref, TreeRow r) async {
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
    await showSheet<void>(context, title: 'ПЕРЕМЕСТИТЬ «${r.name.toUpperCase()}»', children: [
      Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          for (final d in dests)
            InkWell(
              onTap: () async {
                Navigator.pop(context);
                await ref.read(treeServiceProvider).move(r.id, d.id);
                _expand(ref, [r.projectId, if (d.id != null) d.id!]);
              },
              child: Padding(
                padding: EdgeInsets.fromLTRB(20 + d.depth * 16.0, 13, 20, 13),
                child: Row(children: [
                  if (d.id == null) ...[Marker(MarkerType.project, size: 9, color: c.fg), const SizedBox(width: 10)],
                  Expanded(
                    child: Text(d.name,
                        style: ui(15, weight: d.id == null ? FontWeight.w600 : FontWeight.w500, color: c.fg)),
                  ),
                ]),
              ),
            ),
      ]),
    ]);
  }
}

class _Empty extends StatelessWidget {
  const _Empty({required this.onCreate});
  final VoidCallback onCreate;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          AppIcon(size: 36, tile: c.lineStrong, inner: c.bg),
          const SizedBox(height: 18),
          Text('Пока пусто', style: ui(20, weight: FontWeight.w600, color: c.fg)),
          const SizedBox(height: 6),
          Text('Создайте проект. В нём будут папки, черновики и задачи для Claude.',
              style: ui(15, color: c.muted, height: 1.5)),
          const SizedBox(height: 20),
          FilledButton(key: const Key('create-first'), onPressed: onCreate, child: const Text('Создать проект')),
        ]),
      ),
    );
  }
}

class _Notice extends StatelessWidget {
  const _Notice({required this.text, required this.action, required this.onAction});
  final String text, action;
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 4, 12, 8),
      padding: const EdgeInsets.fromLTRB(14, 10, 8, 10),
      decoration: BoxDecoration(border: Border.all(color: c.fg), borderRadius: BorderRadius.circular(8)),
      child: Row(children: [
        Expanded(child: Text(text, style: ui(13, color: c.fg))),
        TextButton(onPressed: onAction, child: Text(action)),
      ]),
    );
  }
}
