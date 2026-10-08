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
  _Autosave(this.save);
  final Future<void> Function(String text) save;
  final controller = TextEditingController();
  Timer? _debounce;
  DateTime _lastEdit = DateTime.fromMillisecondsSinceEpoch(0);
  bool loaded = false;

  void load(String text) {
    controller.text = text;
    loaded = true;
  }

  void changed(String _) {
    _lastEdit = DateTime.now();
    _debounce?.cancel();
    _debounce = Timer(saveDelay, flush);
  }

  Future<void> flush() async {
    _debounce?.cancel();
    _debounce = null;
    if (loaded) await save(controller.text);
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
  late final _raw = _Autosave((t) => _tree.setText(widget.nodeId, t));
  late final _struct = _Autosave((t) => _tree.setStructuredText(widget.nodeId, t));
  late final AppLifecycleListener _lifecycle;
  _Tab? _tab;
  bool _requesting = false;

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
      setState(() {});
    });
  }

  @override
  void dispose() {
    _raw.dispose();
    _struct.dispose();
    _lifecycle.dispose();
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
        final choice = await showDialog<bool>(
          context: context,
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

  InputDecoration _bare(String hint, PtColors c) => InputDecoration(
        isCollapsed: true,
        border: InputBorder.none,
        enabledBorder: InputBorder.none,
        focusedBorder: InputBorder.none,
        contentPadding: EdgeInsets.zero,
        hintText: hint,
        hintStyle: ui(16, color: c.faint, height: 1.6),
      );

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    ref.listen(contentProvider(widget.nodeId), (_, next) => _onContent(next.valueOrNull));
    final node = ref.watch(nodeProvider(widget.nodeId)).valueOrNull;
    final content = ref.watch(contentProvider(widget.nodeId)).valueOrNull;
    final hPad = widget.compact ? 20.0 : 40.0;

    final topBar = SizedBox(
      height: 52,
      child: Row(children: [
        if (widget.compact && Navigator.canPop(context)) BackButton(color: c.fg) else SizedBox(width: hPad - 8),
        if (node != null)
          Expanded(
            child: Text(_crumbs(node).join('  /  '),
                style: ui(13, color: c.muted), overflow: TextOverflow.ellipsis, maxLines: 1),
          )
        else
          const Spacer(),
        if (node != null)
          PopupMenuButton<String>(
            key: const Key('editor-menu'),
            tooltip: 'Действия',
            icon: Icon(Icons.more_horiz, color: c.fg),
            color: c.bg,
            onSelected: (v) => _menu(v, node),
            itemBuilder: (_) => [
              const PopupMenuItem(value: 'versions', child: Text('История версий')),
              const PopupMenuItem(value: 'rename', child: Text('Переименовать')),
              PopupMenuItem(
                  value: 'kind', child: Text(node.kind == NodeKind.aiTask ? 'Сделать черновиком' : 'Сделать AI Task')),
              const PopupMenuItem(value: 'delete', child: Text('Удалить')),
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

    final body = <Widget>[];
    if (tab == _Tab.raw) {
      if (_raw.loaded) {
        body.add(TextField(
          key: const Key('editor-text'),
          controller: _raw.controller,
          onChanged: _raw.changed,
          maxLines: null,
          minLines: 12,
          keyboardType: TextInputType.multiline,
          textCapitalization: TextCapitalization.sentences,
          style: ui(16, color: c.fg, height: 1.6),
          // Без рамки: текст стоит прямо на странице, как в Notion.
          decoration: _bare(isTask ? 'Опишите задачу как есть, своими словами…' : 'Пишите как есть…', c),
        ));
      }
    } else if (doc == null) {
      body.add(Text(
        pending
            ? 'ИИ приводит исходник в порядок. Можно продолжать писать — правки не потеряются.'
            : 'Структуры пока нет. «Структурировать» приведёт исходник в порядок: ИИ только чистит и группирует, '
                'ничего не добавляя. Неясное он вынесет в открытые вопросы.',
        style: ui(15, color: c.muted, height: 1.55),
      ));
    } else {
      if (doc.role.isNotEmpty) {
        body.add(Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(
            padding: const EdgeInsets.only(top: 3),
            child: Text('РОЛЬ', style: mono(10.5, color: c.muted, letterSpacing: 0.8)),
          ),
          const SizedBox(width: 10),
          Expanded(child: SelectableText(doc.role, style: ui(14, color: c.fg, height: 1.5))),
        ]));
        body.add(const SizedBox(height: 14));
      }
      if (_struct.loaded) {
        body.add(TextField(
          key: const Key('structured-text'),
          controller: _struct.controller,
          onChanged: _struct.changed,
          maxLines: null,
          minLines: 8,
          keyboardType: TextInputType.multiline,
          style: ui(16, color: c.fg, height: 1.6),
          decoration: _bare('', c),
        ));
      }
      if (doc.humanParagraphs.isNotEmpty) {
        body.add(Padding(
          padding: const EdgeInsets.only(top: 14),
          child: Text(
            'Ваших абзацев: ${doc.humanParagraphs.length}. При повторном структурировании ИИ оставит их как есть.',
            style: ui(12, color: c.muted),
          ),
        ));
      }
    }

    final scroll = ListView(
      padding: EdgeInsets.fromLTRB(hPad, widget.compact ? 12 : 28, hPad, 32),
      children: [
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 760),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            GestureDetector(
              onTap: () => _menu('rename', node),
              child: Text(node.name, style: ui(26, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.4)),
            ),
            const SizedBox(height: 10),
            Wrap(spacing: 10, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
              _Pill(
                marker: markerFor(kind: node.kind, structureStatus: node.structureStatus),
                label: isTask ? 'AI Task' : 'Raw Note',
              ),
              if (isTask) Text(status, style: ui(12, color: c.muted)),
              if (isTask)
                _Seg(
                  value: tab,
                  onChanged: (t) async {
                    await _flush();
                    setState(() => _tab = t);
                  },
                ),
              if (node.syncError != null)
                Text('не синхронизировано: ${node.syncError}', style: mono(11, color: c.fg)),
            ]),
            if (node.hasConflict) ...[
              const SizedBox(height: 14),
              _Banner(
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
            ],
            if (proposal != null) ...[
              const SizedBox(height: 14),
              _ProposalBanner(
                proposal: proposal,
                onApply: () => _tree.applyProposal(node.id, proposal.requestId),
                onDismiss: () => _tree.dismissProposal(node.id, proposal.requestId),
                onRetry: _structure,
              ),
            ],
            const SizedBox(height: 18),
            ...body,
          ]),
        ),
      ],
    );

    final busy = pending || _requesting;
    final actions = Container(
      decoration: BoxDecoration(border: Border(top: BorderSide(color: c.line))),
      padding: EdgeInsets.fromLTRB(widget.compact ? 16 : hPad, 10, widget.compact ? 16 : hPad, 10),
      child: SafeArea(
        top: false,
        child: Row(children: [
          if (isTask) ...[
            FilledButton(
              key: const Key('structure'),
              onPressed: busy ? null : _structure,
              child: busy
                  ? Row(mainAxisSize: MainAxisSize.min, children: [
                      SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, color: c.faint)),
                      const SizedBox(width: 8),
                      const Text('Структурируется…'),
                    ])
                  : Text(doc == null ? 'Структурировать' : 'Структурировать заново'),
            ),
            const SizedBox(width: 8),
            Flexible(
              fit: widget.compact ? FlexFit.tight : FlexFit.loose,
              child: OutlinedButton(
                key: const Key('copy-task'),
                onPressed: () => _copyTask(node, doc),
                child: const Text('Скопировать задачу', overflow: TextOverflow.ellipsis),
              ),
            ),
          ] else
            Flexible(
              fit: widget.compact ? FlexFit.tight : FlexFit.loose,
              child: OutlinedButton(
                onPressed: () => _tree.changeKind(node.id, NodeKind.aiTask),
                child: const Text('Превратить в AI Task'),
              ),
            ),
        ]),
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

/// Переключатель «Исходник / Структура», как в концепте v1.
class _Seg extends StatelessWidget {
  const _Seg({required this.value, required this.onChanged});
  final _Tab value;
  final ValueChanged<_Tab> onChanged;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    Widget item(_Tab t, String label) {
      final on = t == value;
      return Semantics(
        selected: on,
        button: true,
        child: InkWell(
          key: Key('tab-${t.name}'),
          borderRadius: BorderRadius.circular(5),
          onTap: on ? null : () => onChanged(t),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 3),
            decoration: BoxDecoration(color: on ? c.fg : null, borderRadius: BorderRadius.circular(5)),
            child: Text(label, style: ui(12, color: on ? c.bg : c.muted)),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(2),
      decoration: BoxDecoration(border: Border.all(color: c.lineStrong), borderRadius: BorderRadius.circular(7)),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        item(_Tab.raw, 'Исходник'),
        const SizedBox(width: 2),
        item(_Tab.structured, 'Структура'),
      ]),
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
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 2),
      decoration: BoxDecoration(border: Border.all(color: c.lineStrong), borderRadius: BorderRadius.circular(99)),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Marker(marker, color: c.fg),
        const SizedBox(width: 6),
        Text(label, style: ui(12, color: c.fg)),
      ]),
    );
  }
}

class _Banner extends StatelessWidget {
  const _Banner({super.key, required this.text, required this.actions, this.details = const []});
  final String text;
  final List<String> details;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(14, 10, 8, 6),
      decoration: BoxDecoration(border: Border.all(color: c.fg), borderRadius: BorderRadius.circular(8)),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Padding(padding: const EdgeInsets.only(right: 6), child: Text(text, style: ui(13, color: c.fg, height: 1.45))),
        for (final d in details)
          Padding(
            padding: const EdgeInsets.only(top: 4, right: 6),
            child: Text('— $d', style: ui(12.5, color: c.muted, height: 1.4)),
          ),
        Wrap(alignment: WrapAlignment.end, children: actions),
      ]),
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
            text: 'Пока ИИ работал, исходник изменился. Результат не применён, чтобы не потерять ваши правки.',
            actions: [
              TextButton(onPressed: onDismiss, child: const Text('Отклонить')),
              TextButton(onPressed: onRetry, child: const Text('Структурировать заново')),
              TextButton(onPressed: onApply, child: const Text('Применить')),
            ],
          ),
        StructureProposal.flagged => _Banner(
            key: const Key('proposal-flagged'),
            text: 'Проверка нашла в ответе ИИ то, чего нет в исходнике. Результат не применён.',
            details: [for (final f in proposal.findings) f.detail],
            actions: [
              TextButton(onPressed: onDismiss, child: const Text('Отклонить')),
              TextButton(onPressed: onApply, child: const Text('Применить всё равно')),
            ],
          ),
        _ => _Banner(
            key: const Key('proposal-failed'),
            text: proposal.error ?? 'Не удалось структурировать.',
            actions: [
              TextButton(onPressed: onDismiss, child: const Text('Скрыть')),
              TextButton(onPressed: onRetry, child: const Text('Повторить')),
            ],
          ),
      };
}
