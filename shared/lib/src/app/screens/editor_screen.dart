import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/dialogs.dart';
import 'conflict_screen.dart';
import 'versions_screen.dart';

/// Через сколько после остановки набора текст пишется в локальную БД (архитектура п. 2.3).
const saveDelay = Duration(milliseconds: 300);

/// Редактор на весь экран (телефон).
class EditorScreen extends StatelessWidget {
  const EditorScreen({super.key, required this.nodeId});
  final String nodeId;

  @override
  Widget build(BuildContext context) => Scaffold(
        body: SafeArea(
          bottom: false,
          child: EditorPane(nodeId: nodeId, compact: true, onClose: () => Navigator.maybePop(context)),
        ),
      );
}

/// Редактор узла: шапка с путём и меню, текст, панель действий.
/// На телефоне — экран целиком, на ПК — правая часть окна.
class EditorPane extends ConsumerStatefulWidget {
  const EditorPane({super.key, required this.nodeId, required this.onClose, this.compact = false});

  final String nodeId;

  /// Узел удалён или закрыт: телефон возвращается к дереву, ПК снимает выбор.
  final VoidCallback onClose;

  /// Узкий экран: кнопка «назад», меньше полей.
  final bool compact;

  @override
  ConsumerState<EditorPane> createState() => _EditorPaneState();
}

class _EditorPaneState extends ConsumerState<EditorPane> {
  final _text = TextEditingController();
  final _focus = FocusNode();
  late final AppLifecycleListener _lifecycle;
  // Берём заранее: в dispose() обращаться к ref уже нельзя, а сохранить текст нужно.
  late final TreeService _tree = ref.read(treeServiceProvider);
  Timer? _debounce;
  bool _loaded = false;
  bool _autofocus = false;

  /// Индикатор в шапке: null — ничего не менялось, false — пишем, true — сохранено.
  bool? _saved;
  DateTime _lastEdit = DateTime.fromMillisecondsSinceEpoch(0);

  @override
  void initState() {
    super.initState();
    _tree; // инициализировать до dispose
    // Свернули, переключились, закрыли — сохраняем сразу, не дожидаясь паузы.
    _lifecycle = AppLifecycleListener(onInactive: _flush, onHide: _flush, onPause: _flush, onDetach: _flush);
    ref.read(storeProvider).content(widget.nodeId).then((c) {
      if (!mounted) return;
      _text.text = c?.rawContent ?? '';
      setState(() {
        _loaded = true;
        // Новая пустая заметка — сразу можно печатать. autofocus срабатывает,
        // когда экран снова получает фокус (после закрытия диалога с названием).
        _autofocus = _text.text.isEmpty;
      });
    });
  }

  @override
  void dispose() {
    _debounce?.cancel();
    if (_loaded) _tree.setText(widget.nodeId, _text.text);
    _lifecycle.dispose();
    _text.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _changed(String _) {
    _lastEdit = DateTime.now();
    _debounce?.cancel();
    _debounce = Timer(saveDelay, _flush);
    if (_saved != false) setState(() => _saved = false);
  }

  Future<void> _flush() async {
    _debounce?.cancel();
    _debounce = null;
    if (!_loaded) return;
    await _tree.setText(widget.nodeId, _text.text);
    if (mounted && _saved == false) setState(() => _saved = true);
  }

  /// Текст изменился не здесь (другое устройство, откат версии, разбор конфликта).
  /// Подменяем, только если пользователь сейчас не печатает.
  void _onExternalContent(NodeContent? c) {
    if (!_loaded || c == null || c.rawContent == _text.text) return;
    if (_debounce != null || DateTime.now().difference(_lastEdit) < const Duration(seconds: 2)) return;
    final sel = _text.selection;
    _text.value = TextEditingValue(
      text: c.rawContent,
      selection:
          sel.isValid && sel.end <= c.rawContent.length ? sel : TextSelection.collapsed(offset: c.rawContent.length),
    );
  }

  Future<void> _copyTask(TreeNode node) async {
    await _flush();
    final text = _text.text.trim();
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    if (text.isEmpty) {
      messenger.showSnackBar(const SnackBar(content: Text('Задача пустая, копировать нечего')));
      return;
    }
    await Clipboard.setData(ClipboardData(text: text));
    if (!mounted) return;
    messenger.showSnackBar(const SnackBar(content: Text('Задача скопирована')));
    final delete = await confirm(
      context,
      title: 'Удалить задачу?',
      message: 'Текст уже в буфере обмена. Задача уйдёт в корзину, её можно будет вернуть.',
      action: 'Удалить',
      cancel: 'Оставить',
    );
    if (!delete) return;
    await _tree.delete(node.id);
    widget.onClose();
  }

  Future<void> _menu(String v, TreeNode node) async {
    switch (v) {
      case 'versions':
        await _flush();
        if (mounted) {
          await Navigator.push(context, MaterialPageRoute(builder: (_) => VersionsScreen(nodeId: node.id)));
        }
      case 'rename':
        final name = await askName(context, title: 'Переименовать', initial: node.name);
        if (name != null) await _tree.rename(node.id, name);
      case 'kind':
        await _tree.changeKind(node.id, node.kind == NodeKind.aiTask ? NodeKind.rawNote : NodeKind.aiTask);
      case 'delete':
        if (await confirm(context,
            title: 'Удалить «${node.name}»?', message: 'Заметка уйдёт в корзину. Её можно вернуть.', action: 'Удалить')) {
          await _flush();
          await _tree.delete(node.id);
          widget.onClose();
        }
    }
  }

  List<String> _crumbs(TreeNode node) {
    final nodes = {for (final n in ref.watch(nodesProvider).valueOrNull ?? const <TreeNode>[]) n.id: n};
    final projects = {for (final p in ref.watch(projectsProvider).valueOrNull ?? const <Project>[]) p.id: p};
    final out = <String>[];
    String? cur = node.parentId;
    while (cur != null && nodes[cur] != null) {
      out.insert(0, nodes[cur]!.name);
      cur = nodes[cur]!.parentId;
    }
    out.insert(0, projects[node.projectId]?.name ?? '');
    return out;
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    ref.listen(contentProvider(widget.nodeId), (_, next) => _onExternalContent(next.valueOrNull));
    final node = ref.watch(nodeProvider(widget.nodeId)).valueOrNull;
    final hPad = widget.compact ? 20.0 : 48.0;

    final crumbs = node == null ? const <String>[] : _crumbs(node);
    final topBar = SizedBox(
      height: 56,
      child: Row(children: [
        if (widget.compact && Navigator.canPop(context))
          Padding(padding: const EdgeInsets.only(left: 6), child: BackButton(color: c.fg))
        else
          SizedBox(width: hPad - 8),
        if (node != null)
          Expanded(
            child: Text.rich(
              TextSpan(children: [
                for (final (i, name) in crumbs.indexed) ...[
                  if (i > 0) TextSpan(text: '  /  ', style: ui(13, color: c.faint)),
                  TextSpan(text: name, style: ui(13, color: c.muted, weight: i == 0 ? FontWeight.w500 : FontWeight.w400)),
                ],
              ]),
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
            ),
          )
        else
          const Spacer(),
        _SaveIndicator(saved: _saved),
        if (node != null)
          PopupMenuButton<String>(
            key: const Key('editor-menu'),
            tooltip: 'Действия',
            icon: Icon(Icons.more_horiz_rounded, color: c.fg),
            position: PopupMenuPosition.under,
            offset: const Offset(0, 6),
            onSelected: (v) => _menu(v, node),
            itemBuilder: (_) => [
              _item('versions', Icons.history_rounded, 'История версий', c),
              _item('rename', Icons.edit_outlined, 'Переименовать', c),
              _item(
                'kind',
                node.kind == NodeKind.aiTask ? Icons.radio_button_unchecked_rounded : Icons.diamond_outlined,
                node.kind == NodeKind.aiTask ? 'Сделать черновиком' : 'Сделать AI Task',
                c,
              ),
              const PopupMenuDivider(height: 9),
              _item('delete', Icons.delete_outline_rounded, 'Удалить', c),
            ],
          ),
        const SizedBox(width: 8),
      ]),
    );

    if (node == null) return Column(children: [topBar, const Spacer()]);

    final isTask = node.kind == NodeKind.aiTask;
    final status = switch (node.structureStatus) {
      StructureStatus.done => 'структурирована',
      StructureStatus.stale => 'есть правки после структурирования',
      StructureStatus.pending => 'структурируется…',
      _ => 'не структурирована',
    };

    final content = ListView(
      padding: EdgeInsets.fromLTRB(hPad, widget.compact ? 8 : 32, hPad, 32),
      children: [
        Align(
          alignment: Alignment.topLeft,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 760),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              FadeSlideIn(
                offset: const Offset(0, 8),
                child: MouseRegion(
                  cursor: SystemMouseCursors.text,
                  child: GestureDetector(
                    onTap: () => _menu('rename', node),
                    child: AnimatedSwitcher(
                      duration: PtMotion.normal,
                      child: Text(
                        node.name,
                        key: ValueKey(node.name),
                        style: ui(widget.compact ? 26 : 30, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.6, height: 1.2),
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              FadeSlideIn(
                delay: const Duration(milliseconds: 60),
                offset: const Offset(0, 8),
                child: Wrap(spacing: 8, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
                  _Pill(
                    marker: markerFor(kind: node.kind, structureStatus: node.structureStatus),
                    label: isTask ? 'AI Task' : 'Raw Note',
                  ),
                  AnimatedSize(
                    duration: PtMotion.normal,
                    curve: PtMotion.curve,
                    child: isTask ? Text(status, style: ui(12.5, color: c.muted)) : const SizedBox.shrink(),
                  ),
                  if (node.syncError != null)
                    Row(mainAxisSize: MainAxisSize.min, children: [
                      Icon(Icons.cloud_off_rounded, size: 14, color: c.muted),
                      const SizedBox(width: 5),
                      Text('не синхронизировано: ${node.syncError}', style: mono(11, color: c.fg)),
                    ]),
                ]),
              ),
              AnimatedSize(
                duration: PtMotion.normal,
                curve: PtMotion.curve,
                alignment: Alignment.topLeft,
                child: node.hasConflict
                    ? Padding(
                        padding: const EdgeInsets.only(top: 16),
                        child: _ConflictBanner(onOpen: () async {
                          await _flush();
                          if (context.mounted) {
                            await Navigator.push(
                                context, MaterialPageRoute(builder: (_) => ConflictScreen(nodeId: node.id)));
                          }
                        }),
                      )
                    : const SizedBox(width: double.infinity),
              ),
              const SizedBox(height: 20),
              Divider(height: 1, color: c.line),
              const SizedBox(height: 20),
              if (_loaded)
                FadeSlideIn(
                  delay: const Duration(milliseconds: 100),
                  offset: const Offset(0, 6),
                  child: TextField(
                    key: const Key('editor-text'),
                    controller: _text,
                    focusNode: _focus,
                    autofocus: _autofocus,
                    onChanged: _changed,
                    maxLines: null,
                    minLines: 12,
                    keyboardType: TextInputType.multiline,
                    textCapitalization: TextCapitalization.sentences,
                    style: ui(16, color: c.fg, height: 1.65),
                    // Без рамки: текст стоит прямо на странице, как в Notion.
                    decoration: InputDecoration(
                      isCollapsed: true,
                      filled: false,
                      border: InputBorder.none,
                      enabledBorder: InputBorder.none,
                      focusedBorder: InputBorder.none,
                      contentPadding: EdgeInsets.zero,
                      hintText: isTask ? 'Опишите задачу как есть, своими словами…' : 'Пишите как есть…',
                      hintStyle: ui(16, color: c.faint, height: 1.65),
                    ),
                  ),
                ),
            ]),
          ),
        ),
      ],
    );

    final buttons = Row(children: [
      if (isTask) ...[
        Tooltip(
          message: 'Появится вместе с AI Structuring Engine (этап 6)',
          child: FilledButton.icon(
            onPressed: null,
            icon: const Icon(Icons.auto_awesome_outlined, size: 16),
            label: const Text('Структурировать'),
          ),
        ),
        const SizedBox(width: 8),
        Flexible(
          fit: widget.compact ? FlexFit.tight : FlexFit.loose,
          child: OutlinedButton.icon(
            key: const Key('copy-task'),
            onPressed: () => _copyTask(node),
            icon: const Icon(Icons.content_copy_rounded, size: 16),
            label: Text(widget.compact ? 'Копировать' : 'Скопировать задачу', overflow: TextOverflow.ellipsis),
          ),
        ),
      ] else
        Flexible(
          fit: widget.compact ? FlexFit.tight : FlexFit.loose,
          child: OutlinedButton.icon(
            onPressed: () => _tree.changeKind(node.id, NodeKind.aiTask),
            icon: Marker(MarkerType.aiNone, size: 11, color: c.fg),
            label: const Text('Превратить в AI Task'),
          ),
        ),
    ]);

    final actions = Container(
      decoration: BoxDecoration(
        color: c.bg,
        border: Border(top: BorderSide(color: c.line)),
      ),
      padding: EdgeInsets.fromLTRB(widget.compact ? 16 : hPad, 12, widget.compact ? 16 : hPad, 12),
      child: SafeArea(
        top: false,
        child: AnimatedSwitcher(
          duration: PtMotion.normal,
          switchInCurve: PtMotion.curve,
          transitionBuilder: (child, a) => FadeTransition(
            opacity: a,
            child: SlideTransition(
              position: Tween(begin: const Offset(0, 0.25), end: Offset.zero).animate(a),
              child: child,
            ),
          ),
          child: KeyedSubtree(key: ValueKey(isTask), child: buttons),
        ),
      ),
    );

    return Column(children: [
      topBar,
      if (!widget.compact) Divider(height: 1, color: c.line),
      Expanded(child: content),
      actions,
    ]);
  }

  PopupMenuItem<String> _item(String value, IconData icon, String label, PtColors c) => PopupMenuItem(
        value: value,
        height: 40,
        child: Row(children: [
          Icon(icon, size: 17, color: c.muted),
          const SizedBox(width: 12),
          Flexible(child: Text(label, style: ui(14, color: c.fg), overflow: TextOverflow.ellipsis, maxLines: 1)),
        ]),
      );
}

/// «Сохранено» в шапке: появляется после паузы в наборе и тихо гаснет.
class _SaveIndicator extends StatelessWidget {
  const _SaveIndicator({required this.saved});
  final bool? saved;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return AnimatedSwitcher(
      duration: PtMotion.normal,
      transitionBuilder: (child, a) => FadeTransition(opacity: a, child: child),
      child: switch (saved) {
        null => const SizedBox.shrink(key: ValueKey('none')),
        false => Padding(
            key: const ValueKey('saving'),
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Text('…', style: mono(12, color: c.faint)),
          ),
        true => Padding(
            key: const ValueKey('saved'),
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Row(mainAxisSize: MainAxisSize.min, children: [
              Icon(Icons.check_rounded, size: 14, color: c.faint),
              const SizedBox(width: 4),
              Text('сохранено', style: mono(11, color: c.faint)),
            ]),
          ),
      },
    );
  }
}

class _Pill extends StatelessWidget {
  const _Pill({required this.marker, required this.label});
  final MarkerType marker;
  final String label;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return AnimatedContainer(
      duration: PtMotion.normal,
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: c.panel,
        border: Border.all(color: c.line),
        borderRadius: BorderRadius.circular(99),
      ),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        AnimatedSwitcher(
          duration: PtMotion.normal,
          switchInCurve: Curves.easeOutBack,
          transitionBuilder: (child, a) => ScaleTransition(scale: a, child: child),
          child: Marker(marker, key: ValueKey(marker), color: c.fg),
        ),
        const SizedBox(width: 7),
        Text(label, style: ui(12.5, weight: FontWeight.w500, color: c.fg)),
      ]),
    );
  }
}

class _ConflictBanner extends StatelessWidget {
  const _ConflictBanner({required this.onOpen});
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 10, 8, 10),
      decoration: BoxDecoration(
        color: c.card,
        border: Border.all(color: c.fg),
        borderRadius: BorderRadius.circular(PtRadius.md + 2),
        boxShadow: c.elevation(0.6),
      ),
      child: Row(children: [
        Icon(Icons.call_split_rounded, size: 18, color: c.fg),
        const SizedBox(width: 10),
        Expanded(
          child: Text('Текст изменили на двух устройствах. Обе версии сохранены.',
              style: ui(13.5, color: c.fg, height: 1.4)),
        ),
        TextButton(key: const Key('open-conflict'), onPressed: onOpen, child: const Text('Разобрать')),
      ]),
    );
  }
}
