import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
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
      appBar: AppBar(title: const Text('Корзина')),
      body: items.isEmpty
          ? Padding(
              padding: const EdgeInsets.all(24),
              child: Text('Корзина пуста.', style: ui(15, color: c.muted)),
            )
          : ListView(padding: const EdgeInsets.symmetric(vertical: 8), children: [
              for (final it in items)
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, 6, 12, 6),
                  child: Row(children: [
                    SizedBox(width: 16, child: it.marker == null ? null : Marker(it.marker!, color: c.fg)),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        Text(it.name, style: ui(15, weight: it.project ? FontWeight.w600 : FontWeight.w400, color: c.fg)),
                        Text('удалено ${formatTime(it.at)}', style: mono(11, color: c.faint)),
                      ]),
                    ),
                    TextButton(
                      onPressed: () => it.project ? tree.restoreProject(it.id) : tree.restore(it.id),
                      child: const Text('Вернуть'),
                    ),
                  ]),
                ),
            ]),
    );
  }
}
