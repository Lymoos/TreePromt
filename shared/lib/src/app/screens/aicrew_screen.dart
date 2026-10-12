import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';

/// Канбан AiCrew (этап 7.6, макет design/kanban-v1.html, решение Максима 12.10.2026):
/// на ПК — доска с колонками (вариант A), на телефоне — список по группам (вариант B).
/// Состояние — формой маркера, как в дереве; цвет только ч/б.

/// Колонка канбана.
enum KanbanCol {
  queued('Очередь'),
  work('В работе'),
  check('Проверка'),
  wait('Ждёт решения'),
  done('Слито'),
  fail('Ошибки');

  const KanbanCol(this.title);
  final String title;
}

KanbanCol? taskColumn(String status) => switch (status) {
      'QUEUED' || 'BLOCKED' || 'REPLAN' => KanbanCol.queued,
      'CLAIMED' || 'IN_PROGRESS' || 'CANCELLING' || 'MERGING' || 'ROLLBACK_QUEUED' || 'ROLLING_BACK' => KanbanCol.work,
      'VERIFYING' || 'LLM_REVIEW' || 'MERGE_QUEUED' || 'POST_MERGE_VERIFY' || 'MERGED' => KanbanCol.check,
      'MERGEABLE' => KanbanCol.wait,
      'DONE' || 'ROLLED_BACK' => KanbanCol.done,
      'FAILED' || 'AWAITING_HUMAN' => KanbanCol.fail,
      _ => null, // отменённые не показываются
    };

KanbanCol? planColumn(String status) => switch (status) {
      'QUEUED' || 'PLANNING' => KanbanCol.work,
      'AWAITING_APPROVAL' => KanbanCol.wait,
      'FAILED' => KanbanCol.fail,
      _ => null,
    };

/// Ширина, с которой показывается доска.
const kanbanBoardMinWidth = 900.0;
const _doneLimit = 15;

/// Данные канбана: при открытии, по realtime-событию сервера «exec» и раз в 15 секунд.
final aicrewBoardProvider = StreamProvider.autoDispose<AiCrewBoard>((ref) async* {
  final api = ref.watch(apiProvider);
  if (api == null) {
    yield const AiCrewBoard();
    return;
  }
  final ticks = StreamController<void>();
  final timer = Timer.periodic(const Duration(seconds: 15), (_) => ticks.add(null));
  final sub = ref.watch(syncEngineProvider)?.execEvents.listen((_) => ticks.add(null));
  ref.onDispose(() {
    timer.cancel();
    sub?.cancel();
    ticks.close();
  });
  yield await api.aicrewBoard();
  await for (final _ in ticks.stream) {
    try {
      yield await api.aicrewBoard();
    } on NetworkException {
      // нет сети — показываем последнее, что знаем
    }
  }
});

class AiCrewScreen extends ConsumerWidget {
  const AiCrewScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    final board = ref.watch(aicrewBoardProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('AiCrew')),
      body: board.when(
        loading: () => Center(child: CircularProgressIndicator(color: c.fg, strokeWidth: 2)),
        error: (e, _) => _Problem(
          text: e is NetworkException ? 'Нет связи с сервером. Канбан обновится, когда связь вернётся.' : 'Не удалось загрузить: $e',
          onRetry: () => ref.invalidate(aicrewBoardProvider),
        ),
        data: (b) => LayoutBuilder(builder: (context, box) {
          final wide = box.maxWidth >= kanbanBoardMinWidth;
          return KanbanView(board: b, wide: wide);
        }),
      ),
    );
  }
}

/// Канбан без загрузки — отдельно, чтобы проверять его тестами на готовых данных.
class KanbanView extends ConsumerWidget {
  const KanbanView({super.key, required this.board, required this.wide});
  final AiCrewBoard board;
  final bool wide;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final items = <KanbanCol, List<Widget>>{for (final col in KanbanCol.values) col: []};
    for (final p in board.plans) {
      final col = planColumn(p.status);
      if (col != null) items[col]!.add(PlanCard(plan: p, approval: board.approvalFor(planId: p.id)));
    }
    final done = <ExecTask>[];
    for (final t in board.tasks) {
      final col = taskColumn(t.status);
      if (col == KanbanCol.done) {
        done.add(t);
      } else if (col != null) {
        items[col]!.add(TaskCard(task: t, col: col, approval: board.approvalFor(taskId: t.id)));
      }
    }
    done.sort((a, b) => b.updatedAt.compareTo(a.updatedAt));
    items[KanbanCol.done] = [for (final t in done.take(_doneLimit)) TaskCard(task: t, col: KanbanCol.done)];

    final header = <Widget>[
      _Crew(workers: board.workers, paused: board.incidents.isNotEmpty),
      for (final i in board.incidents) _IncidentBar(incident: i),
      const _Legend(),
    ];
    if (wide) {
      return LayoutBuilder(builder: (context, box) {
        // Колонки делят ширину окна; на узком окне ПК — не уже 200 px и прокрутка вбок.
        const gap = 12.0, side = 20.0;
        final colWidth = ((box.maxWidth - 2 * side - 5 * gap) / 6).clamp(200.0, 420.0);
        return ListView(padding: const EdgeInsets.fromLTRB(side, 12, side, 32), children: [
          ...header.map((w) => Padding(padding: const EdgeInsets.only(bottom: 12), child: w)),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              for (final (i, col) in KanbanCol.values.indexed)
                Padding(
                  padding: EdgeInsets.only(right: i == 5 ? 0 : gap),
                  child: SizedBox(width: colWidth, child: _Column(col: col, children: items[col]!)),
                ),
            ]),
          ),
        ]);
      });
    }
    // Телефон: сверху то, что ждёт человека, и ошибки.
    const order = [KanbanCol.wait, KanbanCol.fail, KanbanCol.work, KanbanCol.check, KanbanCol.queued, KanbanCol.done];
    final groups = [
      for (final col in order)
        if (items[col]!.isNotEmpty) _Group(col: col, children: items[col]!),
    ];
    return ListView(padding: const EdgeInsets.fromLTRB(16, 12, 16, 32), children: [
      ...header.map((w) => Padding(padding: const EdgeInsets.only(bottom: 12), child: w)),
      if (groups.isEmpty) const _Empty(),
      ...groups.map((g) => Padding(padding: const EdgeInsets.only(bottom: 18), child: g)),
    ]);
  }
}

// ── маркеры (форма вместо цвета) ──

class KanbanMarker extends StatelessWidget {
  const KanbanMarker(this.col, {super.key, this.size = 10});
  final KanbanCol col;
  final double size;

  @override
  Widget build(BuildContext context) =>
      CustomPaint(size: Size.square(size), painter: _MarkerPainter(col, context.pt.fg));
}

class _MarkerPainter extends CustomPainter {
  _MarkerPainter(this.col, this.color);
  final KanbanCol col;
  final Color color;

  @override
  void paint(Canvas canvas, Size s) {
    final stroke = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    final fill = Paint()..color = color;
    final r = s.width / 2;
    final center = Offset(r, r);
    Path diamond() => Path()
      ..moveTo(r, 0.6)
      ..lineTo(s.width - 0.6, r)
      ..lineTo(r, s.height - 0.6)
      ..lineTo(0.6, r)
      ..close();
    switch (col) {
      case KanbanCol.queued:
        canvas.drawCircle(center, r - 0.75, stroke);
      case KanbanCol.work:
        canvas.drawCircle(center, r - 0.75, stroke);
        canvas.drawArc(Rect.fromCircle(center: center, radius: r - 0.75), -1.5708, 3.1416, true, fill);
      case KanbanCol.check:
        canvas.drawPath(diamond(), stroke);
      case KanbanCol.wait:
        canvas.drawPath(diamond(), fill);
      case KanbanCol.done:
        canvas.drawRRect(RRect.fromRectAndRadius(Offset.zero & s, const Radius.circular(2)), fill);
      case KanbanCol.fail:
        final rect = (Offset.zero & s).deflate(0.75);
        canvas.drawRRect(RRect.fromRectAndRadius(rect, const Radius.circular(2)), stroke);
        canvas.drawLine(rect.bottomLeft, rect.topRight, stroke);
    }
  }

  @override
  bool shouldRepaint(_MarkerPainter old) => old.col != col || old.color != color;
}

// ── шапка ──

class _Crew extends ConsumerWidget {
  const _Crew({required this.workers, required this.paused});
  final List<WorkerView> workers;
  final bool paused;

  static String _initials(WorkerView w) => switch (w.kind) {
        'claude_code' => 'CC',
        _ => w.kind.isEmpty ? '?' : w.kind.substring(0, w.kind.length < 2 ? 1 : 2).toUpperCase(),
      };

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    return Wrap(spacing: 8, runSpacing: 8, crossAxisAlignment: WrapCrossAlignment.center, children: [
      for (final w in workers)
        Container(
          padding: const EdgeInsets.fromLTRB(4, 4, 10, 4),
          decoration: BoxDecoration(border: Border.all(color: c.line), borderRadius: BorderRadius.circular(999)),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            CircleAvatar(
              radius: 11,
              backgroundColor: w.busy ? c.fg : c.selected,
              child: Text(_initials(w), style: mono(9, weight: FontWeight.w600, color: w.busy ? c.bg : c.fg)),
            ),
            const SizedBox(width: 7),
            Text('${w.hostName} · ${w.model}', style: ui(13, color: c.fg)),
            const SizedBox(width: 6),
            Text(!w.online ? '· не в сети' : (w.busy ? '· задача' : '· свободен'), style: ui(12, color: c.muted)),
          ]),
        ),
      if (!paused)
        OutlinedButton(
          onPressed: () => _act(context, ref, (api) => api.stopAll('из приложения'), 'Всё остановлено'),
          child: const Text('Остановить всё'),
        ),
    ]);
  }
}

String _incidentText(Incident i) => switch (i.kind) {
      'manual' => 'вручную',
      'secret_leak' => 'найден секрет в изменениях исполнителя',
      'integration_red' => 'integration не проходит проверки',
      'qa_spike' => 'ревьюер отклоняет почти всё',
      _ => i.kind,
    };

class _IncidentBar extends ConsumerWidget {
  const _IncidentBar({required this.incident});
  final Incident incident;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = context.pt;
    // Инверсия — самое громкое средство ч/б системы: пауза важнее всего на экране.
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: c.fg, borderRadius: BorderRadius.circular(8)),
      child: Wrap(spacing: 16, runSpacing: 10, crossAxisAlignment: WrapCrossAlignment.center, children: [
        Text.rich(
          TextSpan(children: [
            TextSpan(text: 'AiCrew на паузе', style: ui(14, weight: FontWeight.w600, color: c.bg)),
            TextSpan(
                text: ' · ${_incidentText(incident)}. Задачи не берутся, слияния не идут, ночная смена стоит.',
                style: ui(14, color: c.bg)),
            if (incident.reason.isNotEmpty) TextSpan(text: '\n${incident.reason}', style: mono(12, color: c.bg)),
          ]),
        ),
        FilledButton(
          style: FilledButton.styleFrom(backgroundColor: c.bg, foregroundColor: c.fg),
          onPressed: () async {
            final comment = await _ask(context, 'Возобновить работу', 'Что сделано (попадёт в аудит)', required: false);
            if (comment == null || !context.mounted) return;
            await _act(context, ref, (api) => api.resolveIncident(incident.id, comment), 'Работа возобновлена');
          },
          child: const Text('Возобновить'),
        ),
      ]),
    );
  }
}

class _Legend extends StatelessWidget {
  const _Legend();

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    const labels = {
      KanbanCol.queued: 'в очереди',
      KanbanCol.work: 'в работе',
      KanbanCol.check: 'проверка',
      KanbanCol.wait: 'ждёт решения',
      KanbanCol.done: 'слито',
      KanbanCol.fail: 'ошибка',
    };
    return Wrap(spacing: 16, runSpacing: 6, children: [
      for (final e in labels.entries)
        Row(mainAxisSize: MainAxisSize.min, children: [
          KanbanMarker(e.key),
          const SizedBox(width: 6),
          Text(e.value, style: ui(12, color: c.muted)),
        ]),
    ]);
  }
}

// ── колонки и группы ──

class _Column extends StatelessWidget {
  const _Column({required this.col, required this.children});
  final KanbanCol col;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Container(
        padding: const EdgeInsets.only(bottom: 6),
        decoration: BoxDecoration(border: Border(bottom: BorderSide(color: c.line))),
        child: Row(children: [
          KanbanMarker(col),
          const SizedBox(width: 7),
          Text(col.title.toUpperCase(), style: ui(12, weight: FontWeight.w600, color: c.muted, letterSpacing: .7)),
          const Spacer(),
          Text('${children.length}', style: mono(12, color: c.muted)),
        ]),
      ),
      const SizedBox(height: 8),
      for (final w in children) Padding(padding: const EdgeInsets.only(bottom: 8), child: w),
    ]);
  }
}

class _Group extends StatelessWidget {
  const _Group({required this.col, required this.children});
  final KanbanCol col;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Row(children: [
        KanbanMarker(col),
        const SizedBox(width: 7),
        Text('${col.title.toUpperCase()} · ${children.length}',
            style: ui(12, weight: FontWeight.w600, color: c.muted, letterSpacing: .7)),
      ]),
      const SizedBox(height: 8),
      for (final w in children) Padding(padding: const EdgeInsets.only(bottom: 6), child: w),
    ]);
  }
}

class _Empty extends StatelessWidget {
  const _Empty();

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 40),
        child: Text('Задач AiCrew пока нет. Отправьте AI Task из дерева — она появится здесь.',
            textAlign: TextAlign.center, style: ui(14, color: context.pt.muted)),
      );
}

// ── карточки ──

class _CardFrame extends StatelessWidget {
  const _CardFrame({required this.col, required this.title, required this.meta, required this.children, this.strong = false});
  final KanbanCol col;
  final String title, meta;
  final List<Widget> children;
  final bool strong;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 11, 12, 11),
      decoration: BoxDecoration(
        color: c.bg,
        border: Border.all(color: strong ? c.fg : c.line),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(padding: const EdgeInsets.only(top: 5), child: KanbanMarker(col)),
          const SizedBox(width: 8),
          Expanded(child: Text(title, style: ui(14, weight: FontWeight.w500, color: c.fg))),
        ]),
        const SizedBox(height: 6),
        Text(meta, style: mono(12, color: c.muted)),
        for (final w in children) Padding(padding: const EdgeInsets.only(top: 8), child: w),
      ]),
    );
  }
}

class _Chips extends StatelessWidget {
  const _Chips(this.items);
  final List<(String, bool)> items; // (текст, контурная)

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Wrap(spacing: 4, runSpacing: 4, children: [
      for (final (text, outline) in items)
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
          decoration: BoxDecoration(
            color: outline ? null : c.selected,
            border: outline ? Border.all(color: c.lineStrong) : null,
            borderRadius: BorderRadius.circular(4),
          ),
          child: Text(text, style: mono(11, color: outline ? c.muted : c.fg)),
        ),
    ]);
  }
}

class _Note extends StatelessWidget {
  const _Note(this.text);
  final String text;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.only(left: 8),
      decoration: BoxDecoration(border: Border(left: BorderSide(color: c.lineStrong, width: 2))),
      child: Text(text, style: ui(13, color: c.muted, height: 1.4)),
    );
  }
}

String _hm(DateTime t) => '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

String _statusText(ExecTask t) => switch (t.status) {
      'IN_PROGRESS' || 'CLAIMED' => 'исполнитель работает',
      'VERIFYING' => 'проверки',
      'LLM_REVIEW' => 'ревью',
      'MERGE_QUEUED' => 'в очереди слияния',
      'MERGING' || 'POST_MERGE_VERIFY' || 'MERGED' => 'слияние',
      'ROLLBACK_QUEUED' || 'ROLLING_BACK' => 'откат',
      'MERGEABLE' => 'готово к слиянию',
      'DONE' => 'слито в ${_hm(t.updatedAt)}',
      'ROLLED_BACK' => 'откачено в ${_hm(t.updatedAt)}',
      'AWAITING_HUMAN' => 'нужен человек',
      'FAILED' => 'ошибка',
      'QUEUED' => t.attempt > 0 ? 'повтор' : 'ждёт исполнителя',
      'CANCELLING' => 'отменяется',
      _ => t.status,
    };

class TaskCard extends ConsumerStatefulWidget {
  const TaskCard({super.key, required this.task, required this.col, this.approval});
  final ExecTask task;
  final KanbanCol col;
  final Approval? approval;

  @override
  ConsumerState<TaskCard> createState() => _TaskCardState();
}

class _TaskCardState extends ConsumerState<TaskCard> {
  bool _rejecting = false;
  final _comment = TextEditingController();

  @override
  void dispose() {
    _comment.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final t = widget.task;
    final a = widget.approval;
    final executor = t.executor == 'junior' ? 'Junior · Qwen' : 'Claude';
    final meta = [
      _statusText(t),
      if (t.attempt > 0) 'попытка ${t.attempt}${t.maxAttempts >= t.attempt ? '/${t.maxAttempts}' : ''}',
      executor,
      if (t.mode != 'manual') t.mode == 'night' ? 'ночь' : 'полуавтомат',
    ].join(' · ');
    final chips = <(String, bool)>[
      for (final s in t.skills) (s, false),
      if (t.touchesUI) ('интерфейс', true),
    ];
    return _CardFrame(
      col: widget.col,
      title: t.title,
      meta: meta,
      strong: a != null,
      children: [
        if (chips.isNotEmpty) _Chips(chips),
        if (t.approvalRequired.isNotEmpty) _Note('Нужно ваше одобрение: изменён ${t.approvalRequired.join(', ')}'),
        if (widget.col == KanbanCol.wait && t.reviewSummary != null) _Note('Ревью: ${t.reviewSummary}'),
        if (widget.col == KanbanCol.fail && t.failureClass != null) _Note('${t.failureClass} · ${t.failureCode ?? ''}'),
        if (a != null && !_rejecting)
          Wrap(spacing: 6, runSpacing: 6, children: [
            FilledButton(
              style: _compactFilled,
              onPressed: () => _act(context, ref, (api) => api.resolveApproval(a.id, 'approve'), 'Слито в integration'),
              child: const Text('Слить'),
            ),
            OutlinedButton(style: _compactOutlined, onPressed: () => setState(() => _rejecting = true), child: const Text('Вернуть с комментарием…')),
          ]),
        if (a != null && _rejecting)
          _CommentBox(
            controller: _comment,
            hint: 'Что исправить — исполнитель получит это первым уроком',
            send: 'Отправить на доработку',
            onCancel: () => setState(() => _rejecting = false),
            onSend: () {
              final text = _comment.text.trim();
              if (text.isEmpty) {
                _toast(context, 'Напишите, что исправить');
                return;
              }
              _act(context, ref, (api) => api.resolveApproval(a.id, 'reject', comment: text), 'Отправлено на доработку');
            },
          ),
        if (widget.col == KanbanCol.fail)
          Wrap(spacing: 6, runSpacing: 6, children: [
            if (t.status == 'AWAITING_HUMAN')
              OutlinedButton(
                style: _compactOutlined,
                onPressed: () => _act(context, ref, (api) => api.retryTask(t.id), 'Добавлена попытка'),
                child: const Text('Ещё попытка'),
              ),
            TextButton(
              style: _compactText,
              onPressed: () => _act(context, ref, (api) => api.cancelTask(t.id), 'Задача отменена'),
              child: const Text('Отменить'),
            ),
          ]),
      ],
    );
  }
}

class PlanCard extends ConsumerStatefulWidget {
  const PlanCard({super.key, required this.plan, this.approval});
  final ExecPlan plan;
  final Approval? approval;

  @override
  ConsumerState<PlanCard> createState() => _PlanCardState();
}

class _PlanCardState extends ConsumerState<PlanCard> {
  late final Set<int> _chosen = {for (var i = 0; i < widget.plan.tasks.length; i++) i};
  bool _rejecting = false;
  final _comment = TextEditingController();

  @override
  void dispose() {
    _comment.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final p = widget.plan;
    final a = widget.approval;
    final col = planColumn(p.status) ?? KanbanCol.work;
    final firstLine = p.request.split('\n').first;
    final title = 'План: ${firstLine.length > 80 ? '${firstLine.substring(0, 80)}…' : firstLine}';
    final meta = switch (p.status) {
      'QUEUED' || 'PLANNING' => 'Manager составляет план',
      'AWAITING_APPROVAL' => 'Manager · задач: ${p.tasks.length}',
      'FAILED' => 'план не получился',
      _ => p.status,
    };
    return _CardFrame(col: col, title: title, meta: meta, strong: a != null, children: [
      if (p.status == 'FAILED' && p.error.isNotEmpty) _Note(p.error),
      if (p.tasks.isNotEmpty)
        Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          for (var i = 0; i < p.tasks.length; i++)
            InkWell(
              onTap: a == null
                  ? null
                  : () => setState(() => _chosen.contains(i) ? _chosen.remove(i) : _chosen.add(i)),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  SizedBox(
                    width: 24,
                    height: 20,
                    child: Checkbox(
                      value: _chosen.contains(i),
                      onChanged: a == null ? null : (v) => setState(() => v! ? _chosen.add(i) : _chosen.remove(i)),
                      visualDensity: VisualDensity.compact,
                      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                    ),
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text.rich(TextSpan(children: [
                      TextSpan(text: p.tasks[i].title, style: ui(13, color: c.fg)),
                      if (p.tasks[i].dependsOn.isNotEmpty)
                        TextSpan(
                            text: '  ← ${p.tasks[i].dependsOn.map((d) => d + 1).join(', ')}',
                            style: mono(11, color: c.faint)),
                    ])),
                  ),
                ]),
              ),
            ),
        ]),
      if (a != null && !_rejecting)
        Wrap(spacing: 6, runSpacing: 6, children: [
          FilledButton(
            style: _compactFilled,
            onPressed: _chosen.isEmpty
                ? null
                : () {
                    final all = _chosen.length == p.tasks.length;
                    final picked = _chosen.toList()..sort();
                    _act(
                      context,
                      ref,
                      (api) => all ? api.resolveApproval(a.id, 'approve') : api.resolveApproval(a.id, 'partial', tasks: picked),
                      'План утверждён: ${_chosen.length} из ${p.tasks.length} задач',
                    );
                  },
            child: Text(_chosen.length == p.tasks.length ? 'Утвердить план' : 'Утвердить выбранные'),
          ),
          OutlinedButton(style: _compactOutlined, onPressed: () => setState(() => _rejecting = true), child: const Text('Отклонить…')),
        ]),
      if (a != null && _rejecting)
        _CommentBox(
          controller: _comment,
          hint: 'Что изменить в плане — Manager составит новый. Без комментария план просто отклоняется.',
          send: 'Отклонить',
          onCancel: () => setState(() => _rejecting = false),
          onSend: () => _act(context, ref, (api) => api.resolveApproval(a.id, 'reject', comment: _comment.text.trim()),
              _comment.text.trim().isEmpty ? 'План отклонён' : 'План отправлен Manager\'у на переделку'),
        ),
    ]);
  }
}

class _CommentBox extends StatelessWidget {
  const _CommentBox({required this.controller, required this.hint, required this.send, required this.onSend, required this.onCancel});
  final TextEditingController controller;
  final String hint, send;
  final VoidCallback onSend, onCancel;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      TextField(
        controller: controller,
        minLines: 3,
        maxLines: 8,
        autofocus: true,
        style: ui(14, color: c.fg),
        decoration: InputDecoration(
          hintText: hint,
          hintStyle: ui(13, color: c.faint),
          filled: true,
          fillColor: c.panel,
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: c.lineStrong)),
          isDense: true,
        ),
      ),
      const SizedBox(height: 6),
      Wrap(spacing: 6, children: [
        FilledButton(style: _compactFilled, onPressed: onSend, child: Text(send)),
        TextButton(style: _compactText, onPressed: onCancel, child: const Text('Отмена')),
      ]),
    ]);
  }
}

class _Problem extends StatelessWidget {
  const _Problem({required this.text, required this.onRetry});
  final String text;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            Text(text, textAlign: TextAlign.center, style: ui(14, color: context.pt.muted)),
            const SizedBox(height: 12),
            OutlinedButton(onPressed: onRetry, child: const Text('Повторить')),
          ]),
        ),
      );
}

// Кнопки внутри карточек — компактнее кнопок экранов.
const _compactPad = EdgeInsets.symmetric(horizontal: 12, vertical: 8);
final _compactFilled = FilledButton.styleFrom(padding: _compactPad, minimumSize: const Size(0, 34), textStyle: ui(13, weight: FontWeight.w500));
final _compactOutlined = OutlinedButton.styleFrom(padding: _compactPad, minimumSize: const Size(0, 34), textStyle: ui(13, weight: FontWeight.w500));
final _compactText = TextButton.styleFrom(padding: _compactPad, minimumSize: const Size(0, 34), textStyle: ui(13, weight: FontWeight.w500));

// ── действия ──

void _toast(BuildContext context, String text) =>
    ScaffoldMessenger.maybeOf(context)?.showSnackBar(SnackBar(content: Text(text), duration: const Duration(seconds: 2)));

/// Действие на сервере: после — канбан перечитывается; ошибка — понятным текстом.
Future<void> _act(BuildContext context, WidgetRef ref, Future<void> Function(ApiClient api) run, String done) async {
  final api = ref.read(apiProvider);
  if (api == null) return;
  try {
    await run(api);
    if (context.mounted) _toast(context, done);
  } on NetworkException {
    if (context.mounted) _toast(context, 'Нет связи с сервером — повторите позже');
  } on ApiException catch (e) {
    if (context.mounted) _toast(context, 'Сервер отказал: ${e.message}');
  }
  ref.invalidate(aicrewBoardProvider);
}

Future<String?> _ask(BuildContext context, String title, String hint, {bool required = true}) =>
    showDialog<String>(context: context, builder: (_) => _AskDialog(title: title, hint: hint, required: required));

/// Диалог владеет своим полем: контроллер освобождается после анимации закрытия, а не раньше.
class _AskDialog extends StatefulWidget {
  const _AskDialog({required this.title, required this.hint, required this.required});
  final String title, hint;
  final bool required;

  @override
  State<_AskDialog> createState() => _AskDialogState();
}

class _AskDialogState extends State<_AskDialog> {
  final _ctrl = TextEditingController();

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: Text(widget.title),
        content: TextField(controller: _ctrl, autofocus: true, decoration: InputDecoration(hintText: widget.hint)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Отмена')),
          FilledButton(
            onPressed: () {
              if (widget.required && _ctrl.text.trim().isEmpty) return;
              Navigator.pop(context, _ctrl.text.trim());
            },
            child: const Text('Готово'),
          ),
        ],
      );
}
