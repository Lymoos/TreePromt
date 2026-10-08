import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

/// Главный экран телефона: дерево на весь экран, файл открывается отдельным экраном.
class MobileShell extends ConsumerWidget {
  const MobileShell({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final rows = ref.watch(treeRowsProvider);
    final loaded = ref.watch(projectsProvider).hasValue;
    final session = ref.watch(sessionProvider);
    final stats = ref.watch(treeStatsProvider);
    final authLost = ref.watch(syncStateProvider).valueOrNull?.phase == SyncPhase.authRequired;
    final actions = TreeActions(
      ref,
      openNode: (id) => Navigator.push(context, MaterialPageRoute(builder: (_) => EditorScreen(nodeId: id))),
    );

    final header = Padding(
      padding: const EdgeInsets.fromLTRB(20, 10, 8, 6),
      child: Row(children: [
        const AnimatedAppIcon(size: 30, duration: Duration(milliseconds: 900)),
        const SizedBox(width: 12),
        Expanded(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
            Text('PromptTree',
                style: ui(20, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.4),
                overflow: TextOverflow.ellipsis,
                maxLines: 1),
            AnimatedSwitcher(
              duration: PtMotion.normal,
              child: Text(
                stats.projects == 0
                    ? 'идеи на ходу'
                    : '${plural(stats.projects, 'проект', 'проекта', 'проектов')} · '
                        '${plural(stats.notes, 'заметка', 'заметки', 'заметок')}',
                key: ValueKey(stats),
                style: mono(11, color: c.faint),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ]),
        ),
        const SyncBadge(),
        const SizedBox(width: 2),
        PopupMenuButton<String>(
          key: const Key('menu'),
          tooltip: 'Меню',
          icon: Icon(Icons.more_horiz_rounded, color: c.fg),
          position: PopupMenuPosition.under,
          offset: const Offset(0, 6),
          onSelected: (v) async {
            switch (v) {
              case 'trash':
                Navigator.push(context, MaterialPageRoute(builder: (_) => const TrashScreen()));
              case 'login':
                ref.read(sessionProvider.notifier).state = Session(mode: SessionMode.none, serverUrl: session.serverUrl);
              case 'logout':
                await logout(context, ref);
            }
          },
          itemBuilder: (_) => [
            menuItem('trash', Icons.delete_outline_rounded, 'Корзина', c),
            if (session.mode == SessionMode.server)
              menuItem('logout', Icons.logout_rounded, 'Выйти', c)
            else
              menuItem('login', Icons.cloud_outlined, 'Подключить сервер', c),
          ],
        ),
      ]),
    );

    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: Column(children: [
          header,
          if (authLost)
            SessionNotice(
              onLogin: () =>
                  ref.read(sessionProvider.notifier).state = Session(mode: SessionMode.none, serverUrl: session.serverUrl),
            ),
          Expanded(
            child: FadeSwitcher(
              expand: true,
              child: !loaded
                  ? const SizedBox.shrink(key: ValueKey('loading'))
                  : rows.isEmpty
                      ? EmptyTree(key: const ValueKey('empty'), onCreate: () => actions.createProject(context))
                      : TreePanel(
                          key: const ValueKey('tree'),
                          rows: rows,
                          rowHeight: 44,
                          padding: const EdgeInsets.fromLTRB(10, 6, 10, 120),
                          onTap: actions.tap,
                          onLongPress: (r) => _sheet(context, r.name.toUpperCase(), actions.rowMenu(r)),
                        ),
            ),
          ),
        ]),
      ),
      floatingActionButton: AnimatedScale(
        scale: rows.isEmpty ? 0 : 1,
        duration: PtMotion.normal,
        curve: rows.isEmpty ? Curves.easeIn : Curves.easeOutBack,
        child: rows.isEmpty
            ? const SizedBox.shrink()
            : DecoratedBox(
                decoration: ShapeDecoration(shape: const StadiumBorder(), shadows: c.elevation(1.2)),
                child: FloatingActionButton.extended(
                  key: const Key('create'),
                  heroTag: null,
                  onPressed: () async {
                    final target = await actions.target();
                    if (!context.mounted) return;
                    await _sheet(
                      context,
                      target == null ? 'СОЗДАТЬ' : 'СОЗДАТЬ В «${target.label.toUpperCase()}»',
                      actions.createMenu(target),
                    );
                  },
                  icon: const Icon(Icons.add_rounded),
                  label: const Text('Добавить'),
                ),
              ),
      ),
    );
  }

  /// Нижнее меню: действие запускается уже после закрытия меню, в контексте экрана.
  Future<void> _sheet(BuildContext context, String title, List<TreeAction> items) async {
    final c = context.pt;
    final picked = await showSheet<TreeAction>(context, title: title, children: [
      for (final a in items) ...[
        if (a.dividerBefore) Divider(color: c.line, indent: 24, endIndent: 24, height: 13),
        Builder(
          builder: (sheet) => SheetItem(
            label: a.label,
            caption: a.caption,
            leading: a.marker != null
                ? Marker(a.marker!, size: a.marker == MarkerType.project ? 10 : 12, color: c.fg)
                : a.icon != null
                    ? Icon(a.icon, size: 18, color: c.fg)
                    : null,
            onTap: () => Navigator.pop(sheet, a),
          ),
        ),
      ],
    ]);
    if (picked != null && context.mounted) await picked.run(context);
  }
}

/// Пункт выпадающего меню со значком.
PopupMenuItem<String> menuItem(String value, IconData icon, String label, PtColors c) => PopupMenuItem(
      value: value,
      height: 42,
      child: Row(children: [
        Icon(icon, size: 18, color: c.muted),
        const SizedBox(width: 12),
        Flexible(child: Text(label, style: ui(14, color: c.fg), overflow: TextOverflow.ellipsis, maxLines: 1)),
      ]),
    );
