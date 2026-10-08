import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/common.dart';
import 'versions_screen.dart' show formatTime;

/// Корзина: удалённое мягко, всё можно вернуть. Окончательного удаления в приложении нет.
class TrashScreen extends ConsumerWidget {
  const TrashScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final projects = ref.watch(projectsProvider).valueOrNull ?? const <Project>[];
    final nodes = ref.watch(nodesProvider).valueOrNull ?? const <TreeNode>[];
    final byId = {for (final n in nodes) n.id: n};
    final deletedProjects = {for (final p in projects) if (p.deletedAt != null) p.id};

    // Показываем только верхний удалённый узел ветки: вместе с ним вернётся всё содержимое.
    bool parentDeleted(TreeNode n) {
      var p = n.parentId == null ? null : byId[n.parentId];
      while (p != null) {
        if (p.deletedAt != null) return true;
        p = p.parentId == null ? null : byId[p.parentId];
      }
      return deletedProjects.contains(n.projectId);
    }

    final items = <({String id, String name, DateTime at, bool project, MarkerType? marker})>[
      for (final p in projects)
        if (p.deletedAt != null) (id: p.id, name: p.name, at: p.deletedAt!, project: true, marker: MarkerType.project),
      for (final n in nodes)
        if (n.deletedAt != null && !parentDeleted(n))
          (
            id: n.id,
            name: n.name,
            at: n.deletedAt!,
            project: false,
            marker: n.kind == NodeKind.folder ? null : markerFor(kind: n.kind, structureStatus: n.structureStatus),
          ),
    ]..sort((a, b) => b.at.compareTo(a.at));

    final tree = ref.read(treeServiceProvider);
    return Scaffold(
      appBar: AppBar(
        title: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const Text('Корзина'),
          if (items.isNotEmpty)
            Text(plural(items.length, 'элемент', 'элемента', 'элементов'), style: mono(11, color: c.faint)),
        ]),
      ),
      body: FadeSwitcher(
        expand: true,
        child: items.isEmpty
            ? const EmptyState(
                key: ValueKey('empty'),
                icon: Icons.delete_outline_rounded,
                title: 'Корзина пуста',
                text: 'Удалённые проекты и заметки появятся здесь. Любой из них можно будет вернуть.',
              )
            : ListView(
                key: const ValueKey('list'),
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
                children: [
                  for (final (i, it) in items.indexed)
                    FadeSlideIn(
                      key: ValueKey(it.id),
                      delay: FadeSlideIn.stagger(i),
                      child: Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Center(
                          child: ConstrainedBox(
                            constraints: const BoxConstraints(maxWidth: 720),
                            child: PtCard(
                              padding: const EdgeInsets.fromLTRB(12, 10, 8, 10),
                              child: Row(children: [
                                Container(
                                  width: 36,
                                  height: 36,
                                  decoration: BoxDecoration(
                                    color: c.panel,
                                    borderRadius: BorderRadius.circular(PtRadius.sm + 2),
                                    border: Border.all(color: c.line),
                                  ),
                                  child: Center(
                                    child: it.marker == null
                                        ? Icon(Icons.subdirectory_arrow_right_rounded, size: 16, color: c.muted)
                                        : Marker(it.marker!, size: it.project ? 9 : 10, color: c.fg),
                                  ),
                                ),
                                const SizedBox(width: 12),
                                Expanded(
                                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                                    Text(it.name,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: ui(15, weight: it.project ? FontWeight.w600 : FontWeight.w500, color: c.fg)),
                                    const SizedBox(height: 2),
                                    Text('удалено ${formatTime(it.at)}', style: mono(11, color: c.faint)),
                                  ]),
                                ),
                                TextButton.icon(
                                  onPressed: () async {
                                    final messenger = ScaffoldMessenger.of(context);
                                    await (it.project ? tree.restoreProject(it.id) : tree.restore(it.id));
                                    messenger.showSnackBar(SnackBar(content: Text('«${it.name}» возвращено')));
                                  },
                                  icon: const Icon(Icons.restore_rounded, size: 17),
                                  label: const Text('Вернуть'),
                                ),
                              ]),
                            ),
                          ),
                        ),
                      ),
                    ),
                ],
              ),
      ),
    );
  }
}
