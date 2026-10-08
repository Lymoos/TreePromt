import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

const sidebarWidth = 280.0;

/// Сочетание для «создать»: в браузере Ctrl+N занят самим браузером
/// (новое окно), поэтому там Alt+N; в приложении для ПК — Ctrl+N (Cmd+N на macOS).
String get newShortcutLabel => kIsWeb ? 'Alt N' : 'Ctrl N';

bool _isNewShortcut(KeyEvent e) {
  if (e is! KeyDownEvent || e.logicalKey != LogicalKeyboardKey.keyN) return false;
  final kb = HardwareKeyboard.instance;
  return kIsWeb ? kb.isAltPressed : (kb.isControlPressed || kb.isMetaPressed);
}

/// Окно ПК по концепту v1: слева панель-дерево (вариант C), справа редактор.
class DesktopShell extends ConsumerStatefulWidget {
  const DesktopShell({super.key});

  @override
  ConsumerState<DesktopShell> createState() => _DesktopShellState();
}

class _DesktopShellState extends ConsumerState<DesktopShell> {
  late final TreeActions _actions =
      TreeActions(ref, openNode: (id) => ref.read(selectedProvider.notifier).state = id);

  @override
  void initState() {
    super.initState();
    // Глобальный обработчик: работает, где бы ни был фокус (в том числе после закрытия диалогов).
    HardwareKeyboard.instance.addHandler(_onKey);
  }

  @override
  void dispose() {
    HardwareKeyboard.instance.removeHandler(_onKey);
    super.dispose();
  }

  bool _onKey(KeyEvent e) {
    if (!_isNewShortcut(e)) return false;
    // Поверх окна открыт диалог или другой экран — не мешаем ему.
    if (!(ModalRoute.of(context)?.isCurrent ?? true)) return false;
    _quickCreate(HardwareKeyboard.instance.isShiftPressed ? NodeKind.rawNote : NodeKind.aiTask);
    return true;
  }

  Future<void> _quickCreate(String kind) async {
    final target = await _actions.target();
    if (!mounted) return;
    final items = _actions.createMenu(target);
    final item = items.where((a) => a.label == (kind == NodeKind.aiTask ? 'AI Task' : 'Raw Note')).firstOrNull;
    await (item ?? items.last).run(context);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final selected = ref.watch(selectedProvider);
    final nodes = ref.watch(nodesProvider).valueOrNull ?? const <TreeNode>[];
    final open = nodes.where((n) => n.id == selected && n.kind != NodeKind.folder && n.deletedAt == null).firstOrNull;

    return Scaffold(
      body: Row(children: [
        SizedBox(width: sidebarWidth, child: _Sidebar(actions: _actions)),
        VerticalDivider(width: 1, thickness: 1, color: c.line),
        Expanded(
          child: FadeSwitcher(
            expand: true,
            offset: 0.012,
            child: open == null
                ? const _Placeholder(key: ValueKey('placeholder'))
                : EditorPane(
                    key: ValueKey(open.id), // другой файл — новый редактор, старый сохраняет текст в dispose
                    nodeId: open.id,
                    onClose: () => ref.read(selectedProvider.notifier).state = null,
                  ),
          ),
        ),
      ]),
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
      position: RelativeRect.fromLTRB(at.dx, at.dy, at.dx, at.dy),
      popUpAnimationStyle: AnimationStyle(duration: PtMotion.normal, curve: PtMotion.curve),
      constraints: const BoxConstraints(minWidth: 220, maxWidth: 300),
      items: [
        for (final a in items) ...[
          if (a.dividerBefore) const PopupMenuDivider(height: 9),
          PopupMenuItem<TreeAction>(
            value: a,
            height: 38,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Row(children: [
              SizedBox(
                width: 18,
                child: Center(
                  child: a.marker != null
                      ? Marker(a.marker!, size: a.marker == MarkerType.project ? 9 : 10, color: c.fg)
                      : a.icon != null
                          ? Icon(a.icon, size: 16, color: c.muted)
                          : null,
                ),
              ),
              const SizedBox(width: 12),
              Flexible(child: Text(a.label, style: ui(14, color: c.fg), overflow: TextOverflow.ellipsis, maxLines: 1)),
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
          padding: const EdgeInsets.fromLTRB(16, 16, 10, 12),
          child: Row(children: [
            const AnimatedAppIcon(size: 22, duration: Duration(milliseconds: 900)),
            const SizedBox(width: 10),
            Expanded(
              child: Text('PromptTree',
                  style: ui(15, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.3),
                  overflow: TextOverflow.ellipsis,
                  maxLines: 1),
            ),
            const SyncBadge(),
          ]),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(10, 0, 10, 6),
          child: Builder(
            builder: (btn) => Pressable(
              key: const Key('create'),
              color: c.fg,
              hoverColor: Color.alphaBlend(c.bg.withValues(alpha: 0.12), c.fg),
              borderRadius: BorderRadius.circular(PtRadius.sm + 2),
              shadow: c.elevation(0.5),
              onTap: () async {
                final box = btn.findRenderObject() as RenderBox;
                final at = box.localToGlobal(Offset(0, box.size.height + 6));
                final target = await actions.target();
                if (btn.mounted) await _menu(btn, at, actions.createMenu(target));
              },
              child: SizedBox(
                height: 36,
                child: Row(children: [
                  const SizedBox(width: 12),
                  Icon(Icons.add_rounded, size: 18, color: c.bg),
                  const SizedBox(width: 8),
                  Expanded(child: Text('Создать', style: ui(13.5, weight: FontWeight.w600, color: c.bg))),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                    decoration: BoxDecoration(
                      color: c.bg.withValues(alpha: 0.14),
                      borderRadius: BorderRadius.circular(5),
                    ),
                    child: Text(newShortcutLabel, style: mono(10.5, color: c.bg.withValues(alpha: 0.75))),
                  ),
                  const SizedBox(width: 8),
                ]),
              ),
            ),
          ),
        ),
        if (authLost)
          SessionNotice(
            onLogin: () => ref.read(sessionProvider.notifier).state =
                Session(mode: SessionMode.none, serverUrl: session.serverUrl),
          ),
        if (rows.isNotEmpty)
          Padding(
            padding: const EdgeInsets.fromLTRB(18, 12, 18, 4),
            child: Row(children: [
              Text('ПРОЕКТЫ', style: mono(10.5, color: c.faint, letterSpacing: 1, weight: FontWeight.w500)),
              const Spacer(),
              Text('${ref.watch(treeStatsProvider).projects}', style: mono(10.5, color: c.faint)),
            ]),
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
                        rowHeight: 32,
                        padding: const EdgeInsets.fromLTRB(8, 2, 8, 24),
                        onTap: actions.tap,
                        onSecondaryTap: (r, at) {
                          ref.read(selectedProvider.notifier).state = r.id;
                          _menu(context, at, actions.rowMenu(r));
                        },
                      ),
          ),
        ),
        Divider(height: 1, color: c.line),
        const SizedBox(height: 6),
        _SideButton(
          icon: Icons.delete_outline_rounded,
          label: 'Корзина',
          onTap: () => Navigator.push(context, MaterialPageRoute(builder: (_) => const TrashScreen())),
        ),
        if (session.mode == SessionMode.server)
          _SideButton(icon: Icons.logout_rounded, label: 'Выйти', onTap: () => logout(context, ref))
        else
          _SideButton(
            icon: Icons.cloud_outlined,
            label: 'Подключить сервер',
            onTap: () => ref.read(sessionProvider.notifier).state =
                Session(mode: SessionMode.none, serverUrl: session.serverUrl),
          ),
        const SizedBox(height: 10),
      ]),
    );
  }
}

class _SideButton extends StatelessWidget {
  const _SideButton({required this.icon, required this.label, required this.onTap});
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 1),
      child: Pressable(
        onTap: onTap,
        borderRadius: BorderRadius.circular(PtRadius.sm),
        pressedScale: 0.98,
        child: SizedBox(
          height: 32,
          child: Row(children: [
            const SizedBox(width: 10),
            Icon(icon, size: 16, color: c.muted),
            const SizedBox(width: 10),
            Expanded(child: Text(label, style: ui(13, weight: FontWeight.w500, color: c.muted))),
          ]),
        ),
      ),
    );
  }
}

class _Placeholder extends StatelessWidget {
  const _Placeholder({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    Widget key(String k) => Container(
          margin: const EdgeInsets.only(right: 4),
          padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 3),
          decoration: BoxDecoration(
            color: c.card,
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: c.line),
            // Нижняя грань клавиши.
            boxShadow: [BoxShadow(color: c.lineStrong, offset: const Offset(0, 1.5))],
          ),
          child: Text(k, style: mono(11, color: c.fg, weight: FontWeight.w500)),
        );
    Widget hint(int i, List<String> keys, String text) => FadeSlideIn(
          delay: FadeSlideIn.stagger(i + 3, step: 60),
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 6),
            child: Row(children: [
              ...keys.map(key),
              const SizedBox(width: 8),
              Expanded(child: Text(text, style: ui(13.5, color: c.muted))),
            ]),
          ),
        );
    final mod = newShortcutLabel.split(' ').first;

    return ColoredBox(
      color: c.bg,
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 380),
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            Container(
              width: 72,
              height: 72,
              decoration: BoxDecoration(
                color: c.panel,
                borderRadius: BorderRadius.circular(PtRadius.xl),
                border: Border.all(color: c.line),
              ),
              child: Center(child: AnimatedAppIcon(size: 38, tile: c.lineStrong, inner: c.panel)),
            ),
            const SizedBox(height: 22),
            FadeSlideIn(
              delay: FadeSlideIn.stagger(1, step: 60),
              child: Text('Откройте заметку слева',
                  style: ui(22, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.3)),
            ),
            const SizedBox(height: 8),
            FadeSlideIn(
              delay: FadeSlideIn.stagger(2, step: 60),
              child: Text('Или начните новую — мысль на ходу или задачу для Claude.',
                  style: ui(14.5, color: c.muted, height: 1.5)),
            ),
            const SizedBox(height: 18),
            hint(0, [mod, 'N'], 'новая AI Task'),
            hint(1, [mod, 'Shift', 'N'], 'новый черновик'),
            hint(2, ['ПКМ'], 'действия с проектом или заметкой'),
          ]),
        ),
      ),
    );
  }
}
