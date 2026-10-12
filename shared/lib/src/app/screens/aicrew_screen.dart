import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import '../providers.dart';
import '../widgets/common.dart';
import '../widgets/dialogs.dart';

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
    final count = board.valueOrNull == null
        ? null
        : board.valueOrNull!.tasks.where((t) => taskColumn(t.status) != null).length +
            board.valueOrNull!.plans.where((p) => planColumn(p.status) != null).length;
    return Scaffold(
      appBar: AppBar(
        title: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const Text('AiCrew'),
          AnimatedSwitcher(
            duration: PtMotion.normal,
            child: Text(
              count == null ? 'загрузка…' : 'команда ИИ · ${plural(count, 'карточка', 'карточки', 'карточек')}',
              key: ValueKey(count),
              style: mono(11, color: c.faint),
            ),
          ),
        ]),
        actions: [
          IconButton(
            tooltip: 'Обновить',
            onPressed: () => ref.invalidate(aicrewBoardProvider),
            icon: Icon(Icons.refresh_rounded, color: c.fg),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: FadeSwitcher(
        expand: true,
        child: board.when(
          loading: () => Center(
            key: const ValueKey('loading'),
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              const AnimatedAppIcon(size: 40),
              const SizedBox(height: 16),
              Text('Загружаем доску', style: mono(11, color: c.faint, letterSpacing: 0.6)),
            ]),
          ),
          error: (e, _) => _Problem(
            key: const ValueKey('error'),
            text: e is NetworkException
                ? 'Нет связи с сервером. Канбан обновится, когда связь вернётся.'
                : 'Не удалось загрузить: $e',
            onRetry: () => ref.invalidate(aicrewBoardProvider),
          ),
          data: (b) => LayoutBuilder(
            key: const ValueKey('board'),
            builder: (context, box) => KanbanView(board: b, wide: box.maxWidth >= kanbanBoardMinWidth),
          ),
        ),
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
      if (col != null) {
        items[col]!.add(PlanCard(key: ValueKey('plan-${p.id}'), plan: p, approval: board.approvalFor(planId: p.id)));
      }
    }
    final done = <ExecTask>[];
    for (final t in board.tasks) {
      final col = taskColumn(t.status);
      if (col == KanbanCol.done) {
        done.add(t);
      } else if (col != null) {
        items[col]!
            .add(TaskCard(key: ValueKey('task-${t.id}'), task: t, col: col, approval: board.approvalFor(taskId: t.id)));
      }
    }
    done.sort((a, b) => b.updatedAt.compareTo(a.updatedAt));
    items[KanbanCol.done] = [
      for (final t in done.take(_doneLimit)) TaskCard(key: ValueKey('task-${t.id}'), task: t, col: KanbanCol.done),
    ];

    final header = <Widget>[
      _Crew(workers: board.workers, paused: board.incidents.isNotEmpty),
      for (final i in board.incidents) _IncidentBar(incident: i),
      const _Legend(),
    ];
    Widget headerBlock() => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          for (final (i, w) in header.indexed)
            FadeSlideIn(
              delay: FadeSlideIn.stagger(i, step: 50),
              child: Padding(padding: const EdgeInsets.only(bottom: 12), child: w),
            ),
        ]);

    if (wide) {
      return LayoutBuilder(builder: (context, box) {
        // Колонки делят ширину окна; на узком окне ПК — не уже 200 px и прокрутка вбок.
        const gap = 12.0, side = 20.0;
        final colWidth = ((box.maxWidth - 2 * side - 5 * gap) / 6).clamp(200.0, 420.0);
        return ListView(padding: const EdgeInsets.fromLTRB(side, 12, side, 32), children: [
          headerBlock(),
          const SizedBox(height: 4),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              for (final (i, col) in KanbanCol.values.indexed)
                Padding(
                  padding: EdgeInsets.only(right: i == 5 ? 0 : gap),
                  child: FadeSlideIn(
                    delay: FadeSlideIn.stagger(i + 2, step: 45),
                    offset: const Offset(0, 16),
                    child: SizedBox(width: colWidth, child: _Column(col: col, children: items[col]!)),
                  ),
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
      headerBlock(),
      if (groups.isEmpty) const _Empty(),
      for (final (i, g) in groups.indexed)
        FadeSlideIn(
          delay: FadeSlideIn.stagger(i + 2, step: 60),
          child: Padding(padding: const EdgeInsets.only(bottom: 22), child: g),
        ),
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
        Tooltip(
          message: !w.online ? 'Исполнитель не в сети' : (w.busy ? 'Выполняет задачу' : 'Свободен'),
          child: Container(
            padding: const EdgeInsets.fromLTRB(4, 4, 12, 4),
            decoration: BoxDecoration(
              color: c.card,
              border: Border.all(color: c.line),
              borderRadius: BorderRadius.circular(999),
              boxShadow: c.elevation(0.3),
            ),
            child: Row(mainAxisSize: MainAxisSize.min, children: [
              Stack(clipBehavior: Clip.none, children: [
                AnimatedContainer(
                  duration: PtMotion.normal,
                  width: 26,
                  height: 26,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(color: w.busy ? c.fg : c.selected, shape: BoxShape.circle),
                  child: Text(_initials(w), style: mono(9.5, weight: FontWeight.w600, color: w.busy ? c.bg : c.fg)),
                ),
                // Точка «в сети»: залитая — на связи, пустая — нет.
                Positioned(
                  right: -1,
                  bottom: -1,
                  child: Container(
                    width: 9,
                    height: 9,
                    decoration: BoxDecoration(
                      color: w.online ? c.fg : c.card,
                      shape: BoxShape.circle,
                      border: Border.all(color: c.card, width: 1.5),
                    ),
                    child: w.online
                        ? null
                        : DecoratedBox(
                            decoration:
                                BoxDecoration(shape: BoxShape.circle, border: Border.all(color: c.muted, width: 1))),
                  ),
                ),
              ]),
              const SizedBox(width: 8),
              Text('${w.hostName} · ${w.model}', style: ui(13, weight: FontWeight.w500, color: c.fg)),
              const SizedBox(width: 6),
              Text(!w.online ? '· не в сети' : (w.busy ? '· задача' : '· свободен'), style: ui(12, color: c.muted)),
            ]),
          ),
        ),
      if (!paused)
        OutlinedButton.icon(
          style: _compactOutlined,
          onPressed: () async {
            final ok = await confirm(
              context,
              title: 'Остановить всё?',
              message: 'Исполнители прервут текущие задачи, новые не будут браться, пока вы не возобновите работу.',
              action: 'Остановить',
            );
            if (!ok || !context.mounted) return;
            await _act(context, ref, (api) => api.stopAll('из приложения'), 'Всё остановлено');
          },
          icon: const Icon(Icons.pause_circle_outline_rounded, size: 17),
          label: const Text('Остановить всё'),
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
      padding: const EdgeInsets.fromLTRB(16, 14, 14, 14),
      decoration: BoxDecoration(
        color: c.fg,
        borderRadius: BorderRadius.circular(PtRadius.lg),
        boxShadow: c.elevation(1),
      ),
      child: Wrap(spacing: 16, runSpacing: 12, crossAxisAlignment: WrapCrossAlignment.center, children: [
        Row(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: Icon(Icons.pause_circle_filled_rounded, size: 20, color: c.bg),
          ),
          const SizedBox(width: 10),
          Flexible(
            child: Text.rich(
              TextSpan(children: [
                TextSpan(text: 'AiCrew на паузе', style: ui(14, weight: FontWeight.w700, color: c.bg)),
                TextSpan(
                    text: ' · ${_incidentText(incident)}. Задачи не берутся, слияния не идут, ночная смена стоит.',
                    style: ui(14, color: c.bg, height: 1.45)),
                if (incident.reason.isNotEmpty)
                  TextSpan(text: '\n${incident.reason}', style: mono(12, color: c.bg.withValues(alpha: 0.75))),
              ]),
            ),
          ),
        ]),
        FilledButton.icon(
          style: FilledButton.styleFrom(backgroundColor: c.bg, foregroundColor: c.fg),
          onPressed: () async {
            final comment = await _ask(context, 'Возобновить работу', 'Что сделано (попадёт в аудит)', required: false);
            if (comment == null || !context.mounted) return;
            await _act(context, ref, (api) => api.resolveIncident(incident.id, comment), 'Работа возобновлена');
          },
          icon: const Icon(Icons.play_arrow_rounded, size: 18),
          label: const Text('Возобновить'),
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
    return Wrap(spacing: 6, runSpacing: 6, children: [
      for (final e in labels.entries)
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
          decoration: BoxDecoration(
            color: c.panel,
            border: Border.all(color: c.line),
            borderRadius: BorderRadius.circular(99),
          ),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            KanbanMarker(e.key, size: 9),
            const SizedBox(width: 6),
            Text(e.value, style: ui(12, color: c.muted)),
          ]),
        ),
    ]);
  }
}

// ── колонки и группы ──

/// Карточки колонки или группы: появляются «лесенкой», новые — плавно.
List<Widget> _cards(List<Widget> children, double gap) => [
      for (final (i, w) in children.indexed)
        FadeSlideIn(
          key: w.key == null ? null : ValueKey(('in', w.key)),
          delay: FadeSlideIn.stagger(i + 3, step: 40, max: 10),
          offset: const Offset(0, 10),
          child: Padding(padding: EdgeInsets.only(bottom: gap), child: w),
        ),
    ];

class _Column extends StatelessWidget {
  const _Column({required this.col, required this.children});
  final KanbanCol col;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.fromLTRB(8, 10, 8, 2),
      decoration: BoxDecoration(
        color: c.panel,
        border: Border.all(color: c.line),
        borderRadius: BorderRadius.circular(PtRadius.lg),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(6, 0, 4, 10),
          child: Row(children: [
            KanbanMarker(col),
            const SizedBox(width: 8),
            Expanded(
              child: Text(col.title.toUpperCase(),
                  overflow: TextOverflow.ellipsis,
                  maxLines: 1,
                  style: ui(11.5, weight: FontWeight.w600, color: c.muted, letterSpacing: .7)),
            ),
            _Count(children.length),
          ]),
        ),
        if (children.isEmpty)
          Container(
            height: 64,
            margin: const EdgeInsets.only(bottom: 8),
            alignment: Alignment.center,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(PtRadius.md),
              border: Border.all(color: c.line),
            ),
            child: Text('пусто', style: mono(11, color: c.faint)),
          ),
        ..._cards(children, 8),
      ]),
    );
  }
}

class _Count extends StatelessWidget {
  const _Count(this.n);
  final int n;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return AnimatedContainer(
      duration: PtMotion.normal,
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 1),
      decoration: BoxDecoration(
        color: n == 0 ? Colors.transparent : c.card,
        border: Border.all(color: c.line),
        borderRadius: BorderRadius.circular(99),
      ),
      child: AnimatedSwitcher(
        duration: PtMotion.normal,
        transitionBuilder: (child, a) => ScaleTransition(scale: a, child: FadeTransition(opacity: a, child: child)),
        child: Text('$n', key: ValueKey(n), style: mono(11, color: c.muted)),
      ),
    );
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
      Padding(
        padding: const EdgeInsets.fromLTRB(4, 0, 4, 10),
        child: Row(children: [
          KanbanMarker(col),
          const SizedBox(width: 8),
          Text('${col.title.toUpperCase()} · ${children.length}',
              style: ui(11.5, weight: FontWeight.w600, color: c.muted, letterSpacing: .7)),
          const SizedBox(width: 10),
          Expanded(child: Divider(height: 1, color: c.line)),
        ]),
      ),
      ..._cards(children, 8),
    ]);
  }
}

class _Empty extends StatelessWidget {
  const _Empty();

  @override
  Widget build(BuildContext context) => const Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: EmptyState(
          icon: Icons.view_kanban_outlined,
          title: 'Задач пока нет',
          text: 'Отправьте AI Task из дерева — она появится здесь.',
        ),
      );
}

// ── карточки ──

/// Карточка канбана: при наведении приподнимается, «ждёт решения» — с тёмной рамкой.
class _CardFrame extends StatefulWidget {
  const _CardFrame(
      {required this.col, required this.title, required this.meta, required this.children, this.strong = false});
  final KanbanCol col;
  final String title, meta;
  final List<Widget> children;
  final bool strong;

  @override
  State<_CardFrame> createState() => _CardFrameState();
}

class _CardFrameState extends State<_CardFrame> {
  bool _hover = false;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return MouseRegion(
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: AnimatedContainer(
        duration: PtMotion.fast,
        curve: PtMotion.out,
        transform: Matrix4.translationValues(0, _hover ? -2 : 0, 0),
        padding: const EdgeInsets.fromLTRB(12, 12, 12, 12),
        decoration: BoxDecoration(
          color: c.card,
          border: Border.all(
              color: widget.strong ? c.fg : (_hover ? c.lineStrong : c.line), width: widget.strong ? 1.4 : 1),
          borderRadius: BorderRadius.circular(PtRadius.md + 2),
          boxShadow: c.elevation(_hover ? 1.1 : 0.45),
        ),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Padding(padding: const EdgeInsets.only(top: 5), child: KanbanMarker(widget.col)),
            const SizedBox(width: 9),
            Expanded(
              child: Text(widget.title, style: ui(14, weight: FontWeight.w600, color: c.fg, height: 1.35)),
            ),
          ]),
          const SizedBox(height: 6),
          Padding(
            padding: const EdgeInsets.only(left: 19),
            child: Text(widget.meta, style: mono(11.5, color: c.muted)),
          ),
          for (final w in widget.children) Padding(padding: const EdgeInsets.only(top: 10), child: w),
        ]),
      ),
    );
  }
}

class _Chips extends StatelessWidget {
  const _Chips(this.items);
  final List<(String, bool)> items; // (текст, контурная)

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Wrap(spacing: 5, runSpacing: 5, children: [
      for (final (text, outline) in items)
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: outline ? null : c.panel,
            border: Border.all(color: outline ? c.lineStrong : c.line),
            borderRadius: BorderRadius.circular(99),
          ),
          child: Text(text, style: mono(11, color: outline ? c.muted : c.fg)),
        ),
    ]);
  }
}

class _Note extends StatelessWidget {
  const _Note(this.text, {this.icon = Icons.info_outline_rounded});
  final String text;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return Container(
      padding: const EdgeInsets.fromLTRB(10, 8, 10, 8),
      decoration: BoxDecoration(color: c.panel, borderRadius: BorderRadius.circular(PtRadius.sm + 2)),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Padding(padding: const EdgeInsets.only(top: 1), child: Icon(icon, size: 14, color: c.muted)),
        const SizedBox(width: 8),
        Expanded(child: Text(text, style: ui(12.5, color: c.muted, height: 1.4))),
      ]),
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
        if (t.approvalRequired.isNotEmpty)
          _Note('Нужно ваше одобрение: изменён ${t.approvalRequired.join(', ')}', icon: Icons.verified_user_outlined),
        if (widget.col == KanbanCol.wait && t.reviewSummary != null)
          _Note('Ревью: ${t.reviewSummary}', icon: Icons.rate_review_outlined),
        if (widget.col == KanbanCol.fail && t.failureClass != null)
          _Note('${t.failureClass} · ${t.failureCode ?? ''}', icon: Icons.error_outline_rounded),
        if (a != null && !_rejecting)
          Wrap(spacing: 6, runSpacing: 6, children: [
            FilledButton.icon(
              style: _compactFilled,
              onPressed: () => _act(context, ref, (api) => api.resolveApproval(a.id, 'approve'), 'Слито в integration'),
              icon: const Icon(Icons.merge_rounded, size: 16),
              label: const Text('Слить'),
            ),
            OutlinedButton.icon(
              style: _compactOutlined,
              onPressed: () => setState(() => _rejecting = true),
              icon: const Icon(Icons.undo_rounded, size: 16),
              label: const Text('Вернуть с комментарием…'),
            ),
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
              _act(
                  context, ref, (api) => api.resolveApproval(a.id, 'reject', comment: text), 'Отправлено на доработку');
            },
          ),
        if (widget.col == KanbanCol.fail)
          Wrap(spacing: 6, runSpacing: 6, children: [
            if (t.status == 'AWAITING_HUMAN')
              OutlinedButton.icon(
                style: _compactOutlined,
                onPressed: () => _act(context, ref, (api) => api.retryTask(t.id), 'Добавлена попытка'),
                icon: const Icon(Icons.replay_rounded, size: 16),
                label: const Text('Ещё попытка'),
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
            Pressable(
              onTap: a == null ? null : () => setState(() => _chosen.contains(i) ? _chosen.remove(i) : _chosen.add(i)),
              borderRadius: BorderRadius.circular(PtRadius.sm),
              pressedScale: 0.98,
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 4, horizontal: 2),
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
                      TextSpan(
                          text: p.tasks[i].title,
                          style: ui(13, color: _chosen.contains(i) || a == null ? c.fg : c.faint, height: 1.4)),
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
          FilledButton.icon(
            style: _compactFilled,
            icon: const Icon(Icons.check_rounded, size: 16),
            onPressed: _chosen.isEmpty
                ? null
                : () {
                    final all = _chosen.length == p.tasks.length;
                    final picked = _chosen.toList()..sort();
                    _act(
                      context,
                      ref,
                      (api) => all
                          ? api.resolveApproval(a.id, 'approve')
                          : api.resolveApproval(a.id, 'partial', tasks: picked),
                      'План утверждён: ${_chosen.length} из ${p.tasks.length} задач',
                    );
                  },
            label: AnimatedSwitcher(
              duration: PtMotion.fast,
              child: Text(
                _chosen.length == p.tasks.length ? 'Утвердить план' : 'Утвердить выбранные',
                key: ValueKey(_chosen.length == p.tasks.length),
              ),
            ),
          ),
          OutlinedButton.icon(
            style: _compactOutlined,
            onPressed: () => setState(() => _rejecting = true),
            icon: const Icon(Icons.close_rounded, size: 16),
            label: const Text('Отклонить…'),
          ),
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
  const _CommentBox(
      {required this.controller, required this.hint, required this.send, required this.onSend, required this.onCancel});
  final TextEditingController controller;
  final String hint, send;
  final VoidCallback onSend, onCancel;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    return FadeSlideIn(
      offset: const Offset(0, 6),
      duration: PtMotion.normal,
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        TextField(
          controller: controller,
          minLines: 3,
          maxLines: 8,
          autofocus: true,
          style: ui(14, color: c.fg, height: 1.45),
          decoration: InputDecoration(
            hintText: hint,
            hintMaxLines: 3,
            hintStyle: ui(13, color: c.faint, height: 1.4),
            contentPadding: const EdgeInsets.all(12),
          ),
        ),
        const SizedBox(height: 8),
        Wrap(spacing: 6, children: [
          FilledButton.icon(
            style: _compactFilled,
            onPressed: onSend,
            icon: const Icon(Icons.send_rounded, size: 15),
            label: Text(send),
          ),
          TextButton(style: _compactText, onPressed: onCancel, child: const Text('Отмена')),
        ]),
      ]),
    );
  }
}

class _Problem extends StatelessWidget {
  const _Problem({super.key, required this.text, required this.onRetry});
  final String text;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Center(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          EmptyState(icon: Icons.cloud_off_rounded, title: 'Доска недоступна', text: text),
          FadeSlideIn(
            delay: const Duration(milliseconds: 200),
            child: OutlinedButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh_rounded, size: 18),
              label: const Text('Повторить'),
            ),
          ),
        ]),
      );
}

// Кнопки внутри карточек — компактнее кнопок экранов.
const _compactPad = EdgeInsets.symmetric(horizontal: 12, vertical: 8);
// Скругление как у остальных кнопок, но меньше по высоте.
final _compactFilled = FilledButton.styleFrom(
    padding: _compactPad, minimumSize: const Size(0, 34), textStyle: ui(13, weight: FontWeight.w500));
final _compactOutlined = OutlinedButton.styleFrom(
    padding: _compactPad, minimumSize: const Size(0, 34), textStyle: ui(13, weight: FontWeight.w500));
final _compactText = TextButton.styleFrom(
    padding: _compactPad, minimumSize: const Size(0, 34), textStyle: ui(13, weight: FontWeight.w500));

// ── действия ──

void _toast(BuildContext context, String text) => ScaffoldMessenger.maybeOf(context)
    ?.showSnackBar(SnackBar(content: Text(text), duration: const Duration(seconds: 2)));

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
    showPtDialog<String>(context, builder: (_) => _AskDialog(title: title, hint: hint, required: required));

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
        content: SizedBox(
          width: 380,
          child: TextField(controller: _ctrl, autofocus: true, decoration: InputDecoration(hintText: widget.hint)),
        ),
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
