import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

/// Главный экран телефона: дерево на весь экран, файл открывается отдельным экраном.
class TreeScreen extends ConsumerWidget {
  const TreeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final rows = ref.watch(treeRowsProvider);
    final loaded = ref.watch(projectsProvider).hasValue;
    final session = ref.watch(sessionProvider);
    final authLost = ref.watch(syncStateProvider).valueOrNull?.phase == SyncPhase.authRequired;
    final actions = TreeActions(
      ref,
      openNode: (id) => Navigator.push(context, MaterialPageRoute(builder: (_) => EditorScreen(nodeId: id))),
    );

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
                  await logout(context, ref);
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
          SessionNotice(
            onLogin: () => ref.read(sessionProvider.notifier).state =
                Session(mode: SessionMode.none, serverUrl: session.serverUrl),
          ),
        Expanded(
          child: !loaded
              ? const SizedBox.shrink()
              : rows.isEmpty
                  ? EmptyTree(onCreate: () => actions.createProject(context))
                  : TreePanel(
                      rows: rows,
                      rowHeight: 40,
                      onTap: actions.tap,
                      onLongPress: (r) => _sheet(context, r.name.toUpperCase(), actions.rowMenu(r)),
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
              onPressed: () async {
                final target = await actions.target();
                if (!context.mounted) return;
                await _sheet(
                  context,
                  target == null ? 'СОЗДАТЬ' : 'СОЗДАТЬ В «${target.label.toUpperCase()}»',
                  actions.createMenu(target),
                );
              },
              child: const Icon(Icons.add),
            ),
    );
  }

  /// Нижнее меню: действие запускается уже после закрытия меню, в контексте экрана.
  Future<void> _sheet(BuildContext context, String title, List<TreeAction> items) async {
    final c = context.pt;
    final picked = await showSheet<TreeAction>(context, title: title, children: [
      for (final a in items) ...[
        if (a.dividerBefore) Divider(color: c.line, indent: 20, endIndent: 20),
        Builder(
          builder: (sheet) => SheetItem(
            label: a.label,
            caption: a.caption,
            leading: a.marker != null
                ? Marker(a.marker!, size: a.marker == MarkerType.project ? 9 : 10, color: c.fg)
                : a.icon != null
                    ? Icon(a.icon, size: 16, color: c.fg)
                    : null,
            onTap: () => Navigator.pop(sheet, a),
          ),
        ),
      ],
    ]);
    if (picked != null && context.mounted) await picked.run(context);
  }
}
