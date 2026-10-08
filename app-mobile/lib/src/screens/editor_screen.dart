import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../state/providers.dart';
import '../widgets/dialogs.dart';
import 'conflict_screen.dart';
import 'versions_screen.dart';

/// Через сколько после остановки набора текст пишется в локальную БД (архитектура п. 2.3).
const saveDelay = Duration(milliseconds: 300);

class EditorScreen extends ConsumerStatefulWidget {
  const EditorScreen({super.key, required this.nodeId});
  final String nodeId;

  @override
  ConsumerState<EditorScreen> createState() => _EditorScreenState();
}

class _EditorScreenState extends ConsumerState<EditorScreen> {
  final _text = TextEditingController();
  late final AppLifecycleListener _lifecycle;
  // Берём заранее: в dispose() обращаться к ref уже нельзя, а сохранить текст нужно.
  late final TreeService _tree = ref.read(treeServiceProvider);
  Timer? _debounce;
  bool _loaded = false;
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
      setState(() => _loaded = true);
    });
  }

  @override
  void dispose() {
    _debounce?.cancel();
    if (_loaded) _tree.setText(widget.nodeId, _text.text);
    _lifecycle.dispose();
    _text.dispose();
    super.dispose();
  }

  void _changed(String _) {
    _lastEdit = DateTime.now();
    _debounce?.cancel();
    _debounce = Timer(saveDelay, _flush);
  }

  Future<void> _flush() async {
    _debounce?.cancel();
    _debounce = null;
    if (!_loaded) return;
    await _tree.setText(widget.nodeId, _text.text);
  }

  /// Текст изменился не здесь (другое устройство, откат версии, разбор конфликта).
  /// Подменяем, только если пользователь сейчас не печатает.
  void _onExternalContent(NodeContent? c) {
    if (!_loaded || c == null || c.rawContent == _text.text) return;
    if (_debounce != null || DateTime.now().difference(_lastEdit) < const Duration(seconds: 2)) return;
    final sel = _text.selection;
    _text.value = TextEditingValue(
      text: c.rawContent,
      selection: sel.isValid && sel.end <= c.rawContent.length ? sel : TextSelection.collapsed(offset: c.rawContent.length),
    );
  }

  Future<void> _copyTask(TreeNode node) async {
    await _flush();
    final text = _text.text.trim();
    if (!mounted) return;
    if (text.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Задача пустая, копировать нечего')));
      return;
    }
    await Clipboard.setData(ClipboardData(text: text));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Задача скопирована')));
    final delete = await confirm(
      context,
      title: 'Удалить задачу?',
      message: 'Текст уже в буфере обмена. Задача уйдёт в корзину, её можно будет вернуть.',
      action: 'Удалить',
      cancel: 'Оставить',
    );
    if (!delete) return;
    await ref.read(treeServiceProvider).delete(node.id);
    if (mounted) Navigator.pop(context);
  }

  Future<void> _menu(String v, TreeNode node) async {
    final tree = ref.read(treeServiceProvider);
    switch (v) {
      case 'versions':
        await _flush();
        if (mounted) {
          await Navigator.push(context, MaterialPageRoute(builder: (_) => VersionsScreen(nodeId: node.id)));
        }
      case 'rename':
        final name = await askName(context, title: 'Переименовать', initial: node.name);
        if (name != null) await tree.rename(node.id, name);
      case 'kind':
        await tree.changeKind(node.id, node.kind == NodeKind.aiTask ? NodeKind.rawNote : NodeKind.aiTask);
      case 'delete':
        if (await confirm(context,
            title: 'Удалить «${node.name}»?', message: 'Заметка уйдёт в корзину. Её можно вернуть.', action: 'Удалить')) {
          await _flush();
          await tree.delete(node.id);
          if (mounted) Navigator.pop(context);
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

    if (node == null) {
      return Scaffold(appBar: AppBar(), body: const SizedBox.shrink());
    }
    final isTask = node.kind == NodeKind.aiTask;
    final status = switch (node.structureStatus) {
      StructureStatus.done => 'структурирована',
      StructureStatus.stale => 'есть правки после структурирования',
      StructureStatus.pending => 'структурируется…',
      _ => 'не структурирована',
    };

    return Scaffold(
      appBar: AppBar(
        titleSpacing: 0,
        title: Text(_crumbs(node).join('  /  '),
            style: ui(13, color: c.muted), overflow: TextOverflow.ellipsis, maxLines: 1),
        actions: [
          PopupMenuButton<String>(
            key: const Key('editor-menu'),
            icon: Icon(Icons.more_horiz, color: c.fg),
            color: c.bg,
            onSelected: (v) => _menu(v, node),
            itemBuilder: (_) => [
              const PopupMenuItem(value: 'versions', child: Text('История версий')),
              const PopupMenuItem(value: 'rename', child: Text('Переименовать')),
              PopupMenuItem(value: 'kind', child: Text(isTask ? 'Сделать черновиком' : 'Сделать AI Task')),
              const PopupMenuItem(value: 'delete', child: Text('Удалить')),
            ],
          ),
          const SizedBox(width: 4),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
        children: [
          GestureDetector(
            onTap: () => _menu('rename', node),
            child: Text(node.name, style: ui(26, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.4)),
          ),
          const SizedBox(height: 10),
          Wrap(spacing: 10, runSpacing: 6, crossAxisAlignment: WrapCrossAlignment.center, children: [
            _Pill(
              marker: markerFor(kind: node.kind, structureStatus: node.structureStatus),
              label: isTask ? 'AI Task' : 'Raw Note',
            ),
            if (isTask) Text(status, style: ui(12, color: c.muted)),
            if (node.syncError != null) Text('не синхронизировано: ${node.syncError}', style: mono(11, color: c.fg)),
          ]),
          if (node.hasConflict) ...[
            const SizedBox(height: 14),
            _ConflictBanner(onOpen: () async {
              await _flush();
              if (context.mounted) {
                await Navigator.push(context, MaterialPageRoute(builder: (_) => ConflictScreen(nodeId: node.id)));
              }
            }),
          ],
          const SizedBox(height: 18),
          if (_loaded)
            TextField(
              key: const Key('editor-text'),
              controller: _text,
              onChanged: _changed,
              maxLines: null,
              minLines: 12,
              keyboardType: TextInputType.multiline,
              textCapitalization: TextCapitalization.sentences,
              style: ui(16, color: c.fg, height: 1.6),
              // Без рамки: текст стоит прямо на странице, как в Notion.
              decoration: InputDecoration(
                isCollapsed: true,
                border: InputBorder.none,
                enabledBorder: InputBorder.none,
                focusedBorder: InputBorder.none,
                contentPadding: EdgeInsets.zero,
                hintText: isTask ? 'Опишите задачу как есть, своими словами…' : 'Пишите как есть…',
                hintStyle: ui(16, color: c.faint, height: 1.6),
              ),
            ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Container(
          decoration: BoxDecoration(border: Border(top: BorderSide(color: c.line))),
          padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
          child: Row(children: [
            if (isTask) ...[
              const Tooltip(
                message: 'Появится вместе с AI Structuring Engine (этап 6)',
                child: FilledButton(onPressed: null, child: Text('Структурировать')),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: OutlinedButton(
                  key: const Key('copy-task'),
                  onPressed: () => _copyTask(node),
                  child: const Text('Скопировать задачу'),
                ),
              ),
            ] else
              Expanded(
                child: OutlinedButton(
                  onPressed: () => ref.read(treeServiceProvider).changeKind(node.id, NodeKind.aiTask),
                  child: const Text('Превратить в AI Task'),
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

class _ConflictBanner extends StatelessWidget {
  const _ConflictBanner({required this.onOpen});
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 10, 8, 10),
      decoration: BoxDecoration(border: Border.all(color: c.fg), borderRadius: BorderRadius.circular(8)),
      child: Row(children: [
        Expanded(
          child: Text('Текст изменили на двух устройствах. Обе версии сохранены.', style: ui(13, color: c.fg)),
        ),
        TextButton(key: const Key('open-conflict'), onPressed: onOpen, child: const Text('Разобрать')),
      ]),
    );
  }
}
