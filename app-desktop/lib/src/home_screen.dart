import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

const sidebarWidth = 272.0;

class NewAiTaskIntent extends Intent {
  const NewAiTaskIntent();
}

class NewRawNoteIntent extends Intent {
  const NewRawNoteIntent();
}

/// Окно ПК по концепту v1: слева панель-дерево (вариант C), справа редактор.
class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final actions = TreeActions(ref, openNode: (id) => ref.read(selectedProvider.notifier).state = id);
    final selected = ref.watch(selectedProvider);
    final nodes = ref.watch(nodesProvider).valueOrNull ?? const <TreeNode>[];
    final open = nodes.where((n) => n.id == selected && n.kind != NodeKind.folder && n.deletedAt == null).firstOrNull;

    Future<void> quickCreate(String kind) async {
      final target = await actions.target();
      if (!context.mounted) return;
      final items = actions.createMenu(target);
      final item = items.where((a) => a.label == (kind == NodeKind.aiTask ? 'AI Task' : 'Raw Note')).firstOrNull;
      await (item ?? items.last).run(context);
    }

    return Shortcuts(
      shortcuts: const {
        SingleActivator(LogicalKeyboardKey.keyN, control: true): NewAiTaskIntent(),
        SingleActivator(LogicalKeyboardKey.keyN, control: true, shift: true): NewRawNoteIntent(),
        SingleActivator(LogicalKeyboardKey.keyN, meta: true): NewAiTaskIntent(),
        SingleActivator(LogicalKeyboardKey.keyN, meta: true, shift: true): NewRawNoteIntent(),
      },
      child: Actions(
        actions: {
          NewAiTaskIntent: CallbackAction<NewAiTaskIntent>(onInvoke: (_) => quickCreate(NodeKind.aiTask)),
          NewRawNoteIntent: CallbackAction<NewRawNoteIntent>(onInvoke: (_) => quickCreate(NodeKind.rawNote)),
        },
        child: Focus(
          autofocus: true,
          child: Scaffold(
            body: Row(children: [
              SizedBox(width: sidebarWidth, child: _Sidebar(actions: actions)),
              VerticalDivider(width: 1, thickness: 1, color: c.line),
              Expanded(
                child: open == null
                    ? const _Placeholder()
                    : EditorPane(
                        key: ValueKey(open.id), // другой файл — новый редактор, старый сохраняет текст в dispose
                        nodeId: open.id,
                        onClose: () => ref.read(selectedProvider.notifier).state = null,
                      ),
              ),
            ]),
          ),
        ),
      ),
    );
  }
}

class _Sidebar extends ConsumerWidget {
  const _Sidebar({required this.actions});
  final TreeActions actions;

  Future<void> _menu(BuildContext context, Offset at, List<TreeAction> items) async {
    final c = context.pt;
    final picked = await showMenu<TreeAction>(
      context: context,
      color: c.bg,
      position: RelativeRect.fromLTRB(at.dx, at.dy, at.dx, at.dy),
      items: [
        for (final a in items) ...[
          if (a.dividerBefore) const PopupMenuDivider(),
          PopupMenuItem<TreeAction>(
            value: a,
            height: 36,
            child: Row(children: [
              SizedBox(
                width: 18,
                child: a.marker != null
                    ? Marker(a.marker!, size: a.marker == MarkerType.project ? 9 : 10, color: c.fg)
                    : a.icon != null
                        ? Icon(a.icon, size: 15, color: c.fg)
                        : null,
              ),
              const SizedBox(width: 10),
              Text(a.label, style: ui(14, color: c.fg)),
            ]),
          ),
        ],
      ],
    );
    if (picked != null && context.mounted) await picked.run(context);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final rows = ref.watch(treeRowsProvider);
    final loaded = ref.watch(projectsProvider).hasValue;
    final session = ref.watch(sessionProvider);
    final authLost = ref.watch(syncStateProvider).valueOrNull?.phase == SyncPhase.authRequired;

    return ColoredBox(
      color: c.panel,
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(14, 12, 6, 6),
          child: Row(children: [
            const AppIcon(size: 16),
            const SizedBox(width: 8),
            Expanded(
              child: Text('PromptTree',
                  style: ui(13, weight: FontWeight.w600, color: c.fg), overflow: TextOverflow.ellipsis, maxLines: 1),
            ),
            const SyncBadge(),
          ]),
        ),
        Builder(
          builder: (btn) => _SideButton(
            key: const Key('create'),
            icon: Icons.add,
            label: 'Создать',
            hint: 'Ctrl N',
            onTap: () async {
              final box = btn.findRenderObject() as RenderBox;
              final at = box.localToGlobal(Offset(8, box.size.height));
              final target = await actions.target();
              if (btn.mounted) await _menu(btn, at, actions.createMenu(target));
            },
          ),
        ),
        if (authLost)
          SessionNotice(
            onLogin: () => ref.read(sessionProvider.notifier).state =
                Session(mode: SessionMode.none, serverUrl: session.serverUrl),
          ),
        const SizedBox(height: 4),
        Expanded(
          child: !loaded
              ? const SizedBox.shrink()
              : rows.isEmpty
                  ? EmptyTree(onCreate: () => actions.createProject(context))
                  : TreePanel(
                      rows: rows,
                      rowHeight: 30,
                      padding: const EdgeInsets.fromLTRB(6, 2, 6, 24),
                      onTap: actions.tap,
                      onSecondaryTap: (r, at) {
                        ref.read(selectedProvider.notifier).state = r.id;
                        _menu(context, at, actions.rowMenu(r));
                      },
                    ),
        ),
        Divider(height: 1, color: c.line),
        _SideButton(
          icon: Icons.delete_outline,
          label: 'Корзина',
          onTap: () => Navigator.push(context, MaterialPageRoute(builder: (_) => const TrashScreen())),
        ),
        if (session.mode == SessionMode.server)
          _SideButton(icon: Icons.logout, label: 'Выйти', onTap: () => logout(context, ref))
        else
          _SideButton(
            icon: Icons.cloud_outlined,
            label: 'Подключить сервер',
            onTap: () => ref.read(sessionProvider.notifier).state =
                Session(mode: SessionMode.none, serverUrl: session.serverUrl),
          ),
        const SizedBox(height: 8),
      ]),
    );
  }
}

class _SideButton extends StatelessWidget {
  const _SideButton({super.key, required this.icon, required this.label, required this.onTap, this.hint});
  final IconData icon;
  final String label;
  final String? hint;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 6),
      child: InkWell(
        borderRadius: BorderRadius.circular(5),
        hoverColor: c.hover,
        onTap: onTap,
        child: SizedBox(
          height: 30,
          child: Row(children: [
            const SizedBox(width: 8),
            Icon(icon, size: 15, color: c.muted),
            const SizedBox(width: 8),
            Expanded(child: Text(label, style: ui(13, color: c.muted))),
            if (hint != null)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 5),
                decoration:
                    BoxDecoration(border: Border.all(color: c.lineStrong), borderRadius: BorderRadius.circular(4)),
                child: Text(hint!, style: mono(10.5, color: c.faint)),
              ),
            const SizedBox(width: 8),
          ]),
        ),
      ),
    );
  }
}

class _Placeholder extends StatelessWidget {
  const _Placeholder();

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 360),
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          AppIcon(size: 32, tile: c.line, inner: c.bg),
          const SizedBox(height: 16),
          Text('Откройте заметку слева', style: ui(17, weight: FontWeight.w600, color: c.fg)),
          const SizedBox(height: 6),
          Text('Ctrl N — новая AI Task, Ctrl Shift N — черновик. Правый клик по дереву — действия.',
              style: ui(14, color: c.muted, height: 1.5)),
        ]),
      ),
    );
  }
}
