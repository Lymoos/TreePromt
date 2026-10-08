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

enum _Tab { raw, structured }

/// Поле с автосохранением: пишет в БД через [saveDelay] после остановки набора и
/// подменяет текст, пришедший извне, только когда пользователь не печатает.
class _Autosave {
  _Autosave(this.save, {this.onSaving});
  final Future<void> Function(String text) save;

  /// true — правки ждут записи, false — записаны (для индикатора «сохранено»).
  final void Function(bool saving)? onSaving;
  final controller = TextEditingController();
  Timer? _debounce;
  DateTime _lastEdit = DateTime.fromMillisecondsSinceEpoch(0);
  bool loaded = false;
  bool _dirty = false;

  void load(String text) {
    controller.text = text;
    loaded = true;
  }

  void changed(String _) {
    _lastEdit = DateTime.now();
    _debounce?.cancel();
    _debounce = Timer(saveDelay, flush);
    if (!_dirty) {
      _dirty = true;
      onSaving?.call(true);
    }
  }

  Future<void> flush() async {
    _debounce?.cancel();
    _debounce = null;
    if (!loaded) return;
    await save(controller.text);
    if (_dirty) {
      _dirty = false;
      onSaving?.call(false);
    }
  }

  void external(String text) {
    if (!loaded || text == controller.text) return;
    if (_debounce != null || DateTime.now().difference(_lastEdit) < const Duration(seconds: 2)) return;
    final sel = controller.selection;
    controller.value = TextEditingValue(
      text: text,
      selection: sel.isValid && sel.end <= text.length ? sel : TextSelection.collapsed(offset: text.length),
    );
  }

  void dispose() {
    _debounce?.cancel();
    if (loaded) save(controller.text);
    controller.dispose();
  }
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
  // Берём заранее: в dispose() обращаться к ref уже нельзя, а сохранить текст нужно.
  late final TreeService _tree = ref.read(treeServiceProvider);
  late final _raw = _Autosave((t) => _tree.setText(widget.nodeId, t), onSaving: _onSaving);
  late final _struct = _Autosave((t) => _tree.setStructuredText(widget.nodeId, t), onSaving: _onSaving);
  late final AppLifecycleListener _lifecycle;
  final _rawFocus = FocusNode();
  _Tab? _tab;
  bool _requesting = false;

  /// Новая пустая заметка — курсор сразу в тексте.
  bool _autofocus = false;

  /// Индикатор в шапке: null — ничего не менялось, false — пишем, true — сохранено.
  bool? _saved;

  void _onSaving(bool saving) {
    if (mounted) setState(() => _saved = !saving);
  }

  @override
  void initState() {
    super.initState();
    _tree;
    // Свернули, переключились, закрыли — сохраняем сразу, не дожидаясь паузы.
    _lifecycle = AppLifecycleListener(onInactive: _flush, onHide: _flush, onPause: _flush, onDetach: _flush);
    ref.read(storeProvider).content(widget.nodeId).then((c) {
      if (!mounted) return;
      _raw.load(c?.rawContent ?? '');
      final doc = StructuredDoc.tryParse(c?.structuredContent);
      if (doc != null) _struct.load(doc.formattedText);
      // autofocus срабатывает, когда экран снова получает фокус (после закрытия диалога с названием).
      setState(() => _autofocus = _raw.controller.text.isEmpty && doc == null);
    });
  }

  @override
  void dispose() {
    _raw.dispose();
    _struct.dispose();
    _lifecycle.dispose();
    _rawFocus.dispose();
    super.dispose();
  }

  Future<void> _flush() async {
    await _raw.flush();
    await _struct.flush();
  }

  void _onContent(NodeContent? c) {
    if (c == null) return;
    _raw.external(c.rawContent);
    final doc = StructuredDoc.tryParse(c.structuredContent);
    if (doc == null) return;
    if (!_struct.loaded) {
      setState(() => _struct.load(doc.formattedText));
    } else {
      _struct.external(doc.formattedText);
    }
  }

  void _snack(String text) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  /// «Структурировать»: сначала отправить правки (сервер структурирует ровно то, что видит
  /// пользователь), затем поставить запрос. Результат придёт через синхронизацию.
  Future<void> _structure() async {
    final session = ref.read(sessionProvider);
    final api = ref.read(apiProvider);
    final engine = ref.read(syncEngineProvider);
    if (session.mode != SessionMode.server || api == null || engine == null) {
      _snack('Структурирование работает через сервер. Подключите сервер в меню.');
      return;
    }
    setState(() => _requesting = true);
    final store = ref.read(storeProvider);
    try {
      await _flush();
      for (var attempt = 0; attempt < 2; attempt++) {
        await engine.syncNow();
        if ((await store.pendingOpsFor(widget.nodeId)).isNotEmpty) {
          _snack('Нет связи с сервером: правки ещё не отправлены. Попробуйте, когда появится сеть.');
          return;
        }
        final c = await store.content(widget.nodeId);
        try {
          await api.requestStructure(widget.nodeId, c!.rawRevision);
          engine.schedule(Duration.zero); // подтянуть статус «структурируется…»
          if (mounted) setState(() => _tab = _Tab.structured);
          return;
        } on ApiException catch (e) {
          if (e.code == 'stale_source' && attempt == 0) continue; // текст успел измениться — синхронизируемся ещё раз
          rethrow;
        }
      }
    } on NetworkException {
      _snack('Нет связи с сервером. Попробуйте, когда появится сеть.');
    } on AuthRequiredException {
      _snack('Сессия истекла. Войдите снова.');
    } on ApiException catch (e) {
      _snack(switch (e.code) {
        'ai_disabled' => 'На сервере не настроен ключ Gemini.',
        'ai_daily_limit' => 'Дневной лимит структурирований исчерпан.',
        'stale_source' => 'Текст меняется на другом устройстве. Попробуйте ещё раз.',
        _ => 'Не получилось: ${e.message}',
      });
    } finally {
      if (mounted) setState(() => _requesting = false);
    }
  }

  Future<void> _copyTask(TreeNode node, StructuredDoc? doc) async {
    await _flush();
    if (!mounted) return;
    var text = _raw.controller.text.trim();
    if (doc != null && doc.formattedText.trim().isNotEmpty) {
      var useStructure = true;
      if (node.structureStatus == StructureStatus.stale) {
        final choice = await showPtDialog<bool>(
          context,
          builder: (context) => AlertDialog(
            title: const Text('Структура устарела'),
            content: const Text('Исходник менялся после структурирования. Что скопировать?'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Исходник')),
              FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Структуру')),
            ],
          ),
        );
        if (choice == null || !mounted) return;
        useStructure = choice;
      }
      if (useStructure) text = StructuredDoc(role: doc.role, formattedText: _struct.controller.text).finalTask.trim();
    }
    if (text.isEmpty) {
      _snack('Задача пустая, копировать нечего');
      return;
    }
    await Clipboard.setData(ClipboardData(text: text));
    if (!mounted) return;
    _snack('Задача скопирована');
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

  /// Без рамки: текст стоит прямо на странице, как в Notion.
  InputDecoration _bare(String hint, PtColors c) => InputDecoration(
        isCollapsed: true,
        filled: false,
        border: InputBorder.none,
        enabledBorder: InputBorder.none,
        focusedBorder: InputBorder.none,
        contentPadding: EdgeInsets.zero,
        hintText: hint,
        hintStyle: ui(16, color: c.faint, height: 1.65),
      );

  PopupMenuItem<String> _item(String value, IconData icon, String label, PtColors c) => PopupMenuItem(
        value: value,
        height: 40,
        child: Row(children: [
          Icon(icon, size: 17, color: c.muted),
          const SizedBox(width: 12),
          Flexible(child: Text(label, style: ui(14, color: c.fg), overflow: TextOverflow.ellipsis, maxLines: 1)),
        ]),
      );

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    ref.listen(contentProvider(widget.nodeId), (_, next) => _onContent(next.valueOrNull));
    final node = ref.watch(nodeProvider(widget.nodeId)).valueOrNull;
    final content = ref.watch(contentProvider(widget.nodeId)).valueOrNull;
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
    final doc = StructuredDoc.tryParse(content?.structuredContent);
    final proposal = isTask ? StructureProposal.tryParse(content?.structureProposal) : null;
    final pending = node.structureStatus == StructureStatus.pending;
    final tab = !isTask ? _Tab.raw : (_tab ?? (doc != null ? _Tab.structured : _Tab.raw));
    final status = switch (node.structureStatus) {
      StructureStatus.done => 'структурирована',
      StructureStatus.stale => 'есть правки после структурирования',
      StructureStatus.pending => 'структурируется…',
      _ => 'не структурирована',
    };

    // Тело вкладки: исходник или структура.
    final Widget body;
    if (tab == _Tab.raw) {
      body = !_raw.loaded
          ? const SizedBox(key: ValueKey('raw-loading'), height: 200)
          : KeyedSubtree(
              key: const ValueKey('raw'),
              child: TextField(
                key: const Key('editor-text'),
                controller: _raw.controller,
                focusNode: _rawFocus,
                autofocus: _autofocus,
                onChanged: _raw.changed,
                maxLines: null,
                minLines: 12,
                keyboardType: TextInputType.multiline,
                textCapitalization: TextCapitalization.sentences,
                style: ui(16, color: c.fg, height: 1.65),
                decoration: _bare(isTask ? 'Опишите задачу как есть, своими словами…' : 'Пишите как есть…', c),
              ),
            );
    } else if (doc == null) {
      body = _StructureEmpty(key: ValueKey('empty-$pending'), pending: pending);
    } else {
      body = Column(key: const ValueKey('structured'), crossAxisAlignment: CrossAxisAlignment.start, children: [
        if (doc.role.isNotEmpty) ...[
          _RoleCard(role: doc.role),
          const SizedBox(height: 18),
        ],
        if (_struct.loaded)
          TextField(
            key: const Key('structured-text'),
            controller: _struct.controller,
            onChanged: _struct.changed,
            maxLines: null,
            minLines: 8,
            keyboardType: TextInputType.multiline,
            style: ui(16, color: c.fg, height: 1.65),
            decoration: _bare('', c),
          ),
        if (doc.humanParagraphs.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 18),
            child: Row(children: [
              Icon(Icons.edit_note_rounded, size: 16, color: c.muted),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Ваших абзацев: ${doc.humanParagraphs.length}. При повторном структурировании ИИ оставит их как есть.',
                  style: ui(12.5, color: c.muted, height: 1.4),
                ),
              ),
            ]),
          ),
      ]);
    }

    final scroll = ListView(
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
                        style: ui(widget.compact ? 26 : 30,
                            weight: FontWeight.w700, color: c.fg, letterSpacing: -0.6, height: 1.2),
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
                  if (isTask)
                    AnimatedSwitcher(
                      duration: PtMotion.normal,
                      transitionBuilder: (child, a) => FadeTransition(opacity: a, child: child),
                      child: Row(key: ValueKey(status), mainAxisSize: MainAxisSize.min, children: [
                        if (pending) ...[
                          SizedBox(
                            width: 10,
                            height: 10,
                            child: CircularProgressIndicator(strokeWidth: 1.5, color: c.muted),
                          ),
                          const SizedBox(width: 6),
                        ],
                        Text(status, style: ui(12.5, color: c.muted)),
                      ]),
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
                child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                  const SizedBox(width: double.infinity),
                  if (node.hasConflict)
                    Padding(
                      padding: const EdgeInsets.only(top: 16),
                      child: _Banner(
                        icon: Icons.call_split_rounded,
                        text: 'Текст изменили на двух устройствах. Обе версии сохранены.',
                        actions: [
                          TextButton(
                            key: const Key('open-conflict'),
                            onPressed: () async {
                              await _flush();
                              if (context.mounted) {
                                await Navigator.push(
                                    context, MaterialPageRoute(builder: (_) => ConflictScreen(nodeId: node.id)));
                              }
                            },
                            child: const Text('Разобрать'),
                          ),
                        ],
                      ),
                    ),
                  if (proposal != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 16),
                      child: _ProposalBanner(
                        proposal: proposal,
                        onApply: () => _tree.applyProposal(node.id, proposal.requestId),
                        onDismiss: () => _tree.dismissProposal(node.id, proposal.requestId),
                        onRetry: _structure,
                      ),
                    ),
                ]),
              ),
              const SizedBox(height: 20),
              if (isTask)
                Padding(
                  padding: const EdgeInsets.only(bottom: 20),
                  child: _Seg(
                    value: tab,
                    hasStructure: doc != null,
                    onChanged: (t) async {
                      await _flush();
                      setState(() => _tab = t);
                    },
                  ),
                )
              else ...[
                Divider(height: 1, color: c.line),
                const SizedBox(height: 20),
              ],
              FadeSwitcher(offset: 0.015, child: body),
            ]),
          ),
        ),
      ],
    );

    final busy = pending || _requesting;
    final buttons = Row(children: [
      if (isTask) ...[
        FilledButton(
          key: const Key('structure'),
          onPressed: busy ? null : _structure,
          child: AnimatedSwitcher(
            duration: PtMotion.fast,
            child: busy
                ? Row(key: const ValueKey('busy'), mainAxisSize: MainAxisSize.min, children: [
                    SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, color: c.faint)),
                    const SizedBox(width: 8),
                    Text(widget.compact ? 'ИИ работает…' : 'Структурируется…'),
                  ])
                : Row(key: ValueKey(doc == null), mainAxisSize: MainAxisSize.min, children: [
                    const Icon(Icons.auto_awesome_outlined, size: 16),
                    const SizedBox(width: 8),
                    Text(doc == null ? 'Структурировать' : (widget.compact ? 'Заново' : 'Структурировать заново')),
                  ]),
          ),
        ),
        const SizedBox(width: 8),
        Flexible(
          fit: widget.compact ? FlexFit.tight : FlexFit.loose,
          child: OutlinedButton.icon(
            key: const Key('copy-task'),
            onPressed: () => _copyTask(node, doc),
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
      decoration: BoxDecoration(color: c.bg, border: Border(top: BorderSide(color: c.line))),
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
      Expanded(child: scroll),
      actions,
    ]);
  }
}

/// «Сохранено» в шапке: появляется после паузы в наборе.
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

/// Переключатель «Исходник / Структура»: плашка плавно переезжает под выбранную вкладку.
class _Seg extends StatelessWidget {
  const _Seg({required this.value, required this.onChanged, required this.hasStructure});
  final _Tab value;
  final bool hasStructure;
  final ValueChanged<_Tab> onChanged;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    const w = 140.0, h = 34.0;
    Widget item(_Tab t, String label, IconData icon) {
      final on = t == value;
      return Semantics(
        selected: on,
        button: true,
        child: MouseRegion(
          cursor: on ? MouseCursor.defer : SystemMouseCursors.click,
          child: GestureDetector(
            key: Key('tab-${t.name}'),
            behavior: HitTestBehavior.opaque,
            onTap: on ? null : () => onChanged(t),
            child: SizedBox(
              width: w,
              height: h,
              child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                Icon(icon, size: 15, color: on ? c.fg : c.muted),
                const SizedBox(width: 6),
                Flexible(
                  child: AnimatedDefaultTextStyle(
                    duration: PtMotion.fast,
                    style: ui(13, weight: on ? FontWeight.w600 : FontWeight.w500, color: on ? c.fg : c.muted),
                    child: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis),
                  ),
                ),
                if (t == _Tab.structured && hasStructure && !on) ...[
                  const SizedBox(width: 6),
                  Container(width: 5, height: 5, decoration: BoxDecoration(color: c.fg, shape: BoxShape.circle)),
                ],
              ]),
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(3),
      decoration: BoxDecoration(
        color: c.panel,
        border: Border.all(color: c.line),
        borderRadius: BorderRadius.circular(PtRadius.md),
      ),
      child: Stack(children: [
        AnimatedPositioned(
          duration: PtMotion.normal,
          curve: PtMotion.curve,
          left: value == _Tab.raw ? 0 : w,
          top: 0,
          width: w,
          height: h,
          child: Container(
            decoration: BoxDecoration(
              color: c.card,
              borderRadius: BorderRadius.circular(PtRadius.sm + 1),
              border: Border.all(color: c.line),
              boxShadow: c.elevation(0.4),
            ),
          ),
        ),
        Row(mainAxisSize: MainAxisSize.min, children: [
          item(_Tab.raw, 'Исходник', Icons.notes_rounded),
          item(_Tab.structured, 'Структура', Icons.auto_awesome_outlined),
        ]),
      ]),
    );
  }
}

/// Вкладка «Структура», пока результата нет: пояснение или «ИИ работает».
class _StructureEmpty extends StatelessWidget {
  const _StructureEmpty({super.key, required this.pending});
  final bool pending;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(18, 18, 18, 18),
      decoration: BoxDecoration(
        color: c.panel,
        borderRadius: BorderRadius.circular(PtRadius.lg),
        border: Border.all(color: c.line),
      ),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Container(
          width: 36,
          height: 36,
          decoration: BoxDecoration(
            color: c.card,
            borderRadius: BorderRadius.circular(PtRadius.sm + 2),
            border: Border.all(color: c.line),
          ),
          child: Center(
            child: pending
                ? SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 1.8, color: c.fg))
                : Icon(Icons.auto_awesome_outlined, size: 18, color: c.fg),
          ),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(pending ? 'ИИ приводит исходник в порядок' : 'Структуры пока нет',
                style: ui(15, weight: FontWeight.w600, color: c.fg)),
            const SizedBox(height: 4),
            Text(
              pending
                  ? 'Можно продолжать писать — правки не потеряются.'
                  : '«Структурировать» приведёт исходник в порядок: ИИ только чистит и группирует, '
                      'ничего не добавляя. Неясное он вынесет в открытые вопросы.',
              style: ui(14, color: c.muted, height: 1.5),
            ),
          ]),
        ),
      ]),
    );
  }
}

/// Роль для Claude — карточка над текстом структуры.
class _RoleCard extends StatelessWidget {
  const _RoleCard({required this.role});
  final String role;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return FadeSlideIn(
      offset: const Offset(0, 6),
      child: Container(
        width: double.infinity,
        clipBehavior: Clip.antiAlias,
        decoration: BoxDecoration(
          color: c.panel,
          borderRadius: BorderRadius.circular(PtRadius.md),
          border: Border.all(color: c.line),
        ),
        // Тёмная полоса слева отмечает роль, как цитату.
        child: IntrinsicHeight(
          child: Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Container(width: 3, color: c.fg),
            Expanded(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Icon(Icons.person_outline_rounded, size: 14, color: c.muted),
            const SizedBox(width: 6),
            Text('РОЛЬ', style: mono(10.5, color: c.muted, letterSpacing: 0.8, weight: FontWeight.w500)),
          ]),
          const SizedBox(height: 6),
          SelectableText(role, style: ui(14.5, color: c.fg, height: 1.5)),
                ]),
              ),
            ),
          ]),
        ),
      ),
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

/// Плашка над текстом: значок, сообщение, подробности и действия.
class _Banner extends StatelessWidget {
  const _Banner({super.key, required this.icon, required this.text, required this.actions, this.details = const []});
  final IconData icon;
  final String text;
  final List<String> details;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return FadeSlideIn(
      offset: const Offset(0, -6),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.fromLTRB(14, 12, 8, 6),
        decoration: BoxDecoration(
          color: c.card,
          border: Border.all(color: c.fg),
          borderRadius: BorderRadius.circular(PtRadius.md + 2),
          boxShadow: c.elevation(0.6),
        ),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Padding(padding: const EdgeInsets.only(top: 1), child: Icon(icon, size: 18, color: c.fg)),
            const SizedBox(width: 10),
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Padding(
                  padding: const EdgeInsets.only(right: 6),
                  child: Text(text, style: ui(13.5, color: c.fg, height: 1.45)),
                ),
                for (final d in details)
                  Padding(
                    padding: const EdgeInsets.only(top: 4, right: 6),
                    child: Text('— $d', style: ui(12.5, color: c.muted, height: 1.4)),
                  ),
              ]),
            ),
          ]),
          Wrap(alignment: WrapAlignment.end, children: actions),
        ]),
      ),
    );
  }
}

/// Результат ИИ, который сервер не применил сам (ТЗ п. 6.2, 6.5).
class _ProposalBanner extends StatelessWidget {
  const _ProposalBanner({required this.proposal, required this.onApply, required this.onDismiss, required this.onRetry});
  final StructureProposal proposal;
  final VoidCallback onApply, onDismiss, onRetry;

  @override
  Widget build(BuildContext context) => switch (proposal.reason) {
        StructureProposal.stale => _Banner(
            key: const Key('proposal-stale'),
            icon: Icons.update_rounded,
            text: 'Пока ИИ работал, исходник изменился. Результат не применён, чтобы не потерять ваши правки.',
            actions: [
              TextButton(onPressed: onDismiss, child: const Text('Отклонить')),
              TextButton(onPressed: onRetry, child: const Text('Структурировать заново')),
              TextButton(onPressed: onApply, child: const Text('Применить')),
            ],
          ),
        StructureProposal.flagged => _Banner(
            key: const Key('proposal-flagged'),
            icon: Icons.policy_outlined,
            text: 'Проверка нашла в ответе ИИ то, чего нет в исходнике. Результат не применён.',
            details: [for (final f in proposal.findings) f.detail],
            actions: [
              TextButton(onPressed: onDismiss, child: const Text('Отклонить')),
              TextButton(onPressed: onApply, child: const Text('Применить всё равно')),
            ],
          ),
        _ => _Banner(
            key: const Key('proposal-failed'),
            icon: Icons.error_outline_rounded,
            text: proposal.error ?? 'Не удалось структурировать.',
            actions: [
              TextButton(onPressed: onDismiss, child: const Text('Скрыть')),
              TextButton(onPressed: onRetry, child: const Text('Повторить')),
            ],
          ),
      };
}
