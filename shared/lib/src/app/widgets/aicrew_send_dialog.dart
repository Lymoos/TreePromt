import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';

/// «Отправить в AiCrew…» (решение Максима 12.10.2026): AI Task из дерева уходит исполнителю —
/// одной задачей или через Manager (план из подзадач). Статусы потом — в канбане AiCrew.
Future<void> sendToAiCrew(BuildContext context, WidgetRef ref, {required String nodeId, required String projectId,
    required String name}) async {
  final sent = await showPtDialog<String>(
    context,
    builder: (_) => _SendDialog(nodeId: nodeId, projectId: projectId, name: name),
  );
  if (sent != null && context.mounted) {
    ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(sent)));
  }
}

class _SendDialog extends ConsumerStatefulWidget {
  const _SendDialog({required this.nodeId, required this.projectId, required this.name});
  final String nodeId, projectId, name;

  @override
  ConsumerState<_SendDialog> createState() => _SendDialogState();
}

class _SendDialogState extends ConsumerState<_SendDialog> {
  List<ExecRepository>? _repos;
  String? _repo;
  bool _plan = false;
  String _mode = 'manual';
  bool _techLead = false;
  bool _touchesUI = false;
  bool _sending = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final api = ref.read(apiProvider);
    if (api == null) return;
    try {
      final all = await api.execRepositories();
      final mine = all.where((r) => r.projectId == widget.projectId).toList();
      if (!mounted) return;
      setState(() {
        _repos = mine;
        _repo = mine.isEmpty ? null : mine.first.id;
      });
    } catch (e) {
      if (mounted) setState(() => _error = _explain(e));
    }
  }

  String _explain(Object e) => switch (e) {
        NetworkException() => 'Нет связи с сервером.',
        ApiException(code: 'not_found') =>
          'Сервер ещё не знает эту заметку: дождитесь синхронизации (значок вверху) и отправьте снова.',
        ApiException(code: 'invalid_input') => 'Сервер не принял задачу: заметка должна быть AI Task с текстом.',
        ApiException(:final message) => 'Сервер отказал: $message',
        _ => '$e',
      };

  Future<void> _send() async {
    final api = ref.read(apiProvider);
    if (api == null || _repo == null) return;
    setState(() {
      _sending = true;
      _error = null;
    });
    try {
      if (_plan) {
        await api.createExecPlan(nodeId: widget.nodeId, repositoryId: _repo!, mode: _mode);
      } else {
        await api.createExecTask(
          nodeId: widget.nodeId,
          repositoryId: _repo!,
          mode: _mode,
          reviewLevel: _techLead ? 'tech_lead' : 'logic',
          touchesUI: _touchesUI,
        );
      }
      if (mounted) {
        Navigator.pop(context, _plan ? 'Manager составляет план — он появится в AiCrew' : 'Задача отправлена в AiCrew');
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _sending = false;
          _error = _explain(e);
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final repos = _repos;
    Widget label(String text) =>
        Padding(padding: const EdgeInsets.only(top: 14, bottom: 6), child: Text(text, style: ui(13, weight: FontWeight.w600, color: c.muted)));
    Widget choice<T>(T value, T group, String text, ValueChanged<T> on, {String? hint}) => InkWell(
          borderRadius: BorderRadius.circular(6),
          onTap: () => setState(() => on(value)),
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Icon(value == group ? Icons.radio_button_checked : Icons.radio_button_off, size: 18, color: c.fg),
              const SizedBox(width: 8),
              Expanded(
                child: Text.rich(TextSpan(children: [
                  TextSpan(text: text, style: ui(14, color: c.fg)),
                  if (hint != null) TextSpan(text: '\n$hint', style: ui(12, color: c.muted)),
                ])),
              ),
            ]),
          ),
        );
    Widget check(bool value, String text, ValueChanged<bool> on, {String? hint}) => InkWell(
          borderRadius: BorderRadius.circular(6),
          onTap: () => setState(() => on(!value)),
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Icon(value ? Icons.check_box : Icons.check_box_outline_blank, size: 18, color: c.fg),
              const SizedBox(width: 8),
              Expanded(
                child: Text.rich(TextSpan(children: [
                  TextSpan(text: text, style: ui(14, color: c.fg)),
                  if (hint != null) TextSpan(text: '\n$hint', style: ui(12, color: c.muted)),
                ])),
              ),
            ]),
          ),
        );

    return AlertDialog(
      title: const Text('Отправить в AiCrew'),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: SingleChildScrollView(
          child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Text('«${widget.name}»', style: ui(15, weight: FontWeight.w500, color: c.fg)),
            if (repos == null && _error == null)
              const Padding(padding: EdgeInsets.all(16), child: Center(child: CircularProgressIndicator(strokeWidth: 2))),
            if (repos != null && repos.isEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: Text(
                    'К этому проекту не привязан репозиторий. Привяжите его на ПК с AiCrew: aicrew-host add-repo -project <id проекта> -path <папка>.',
                    style: ui(13, color: c.muted)),
              ),
            if (repos != null && repos.isNotEmpty) ...[
              label('Репозиторий'),
              DropdownButtonFormField<String>(
                value: _repo,
                isExpanded: true,
                items: [for (final r in repos) DropdownMenuItem(value: r.id, child: Text(r.name, overflow: TextOverflow.ellipsis))],
                onChanged: (v) => setState(() => _repo = v),
              ),
              label('Как выполнять'),
              choice(false, _plan, 'Одной задачей', (v) => _plan = v, hint: 'всё делает Claude'),
              choice(true, _plan, 'Через Manager', (v) => _plan = v,
                  hint: 'Claude разложит задачу; механические части по его тестам сделает локальная Qwen, Claude соберёт'),
              label('Режим'),
              choice('manual', _mode, 'Ручной', (v) => _mode = v, hint: 'перед слиянием спросят вас'),
              choice('semi_auto', _mode, 'Полуавтомат', (v) => _mode = v, hint: 'проверенное сливается само'),
              if (!_plan) ...[
                label('Ревью'),
                check(_techLead, '+ Tech Lead', (v) => _techLead = v, hint: 'ещё одно ревью — для сложных задач'),
                check(_touchesUI, 'Меняет интерфейс', (v) => _touchesUI = v, hint: 'Vision QA посмотрит на снимки экрана'),
              ],
            ],
            if (_error != null)
              Padding(padding: const EdgeInsets.only(top: 12), child: Text(_error!, style: ui(13, color: c.fg))),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: _sending ? null : () => Navigator.pop(context), child: const Text('Отмена')),
        FilledButton(onPressed: _sending || _repo == null ? null : _send, child: Text(_sending ? 'Отправка…' : 'Отправить')),
      ],
    );
  }
}
