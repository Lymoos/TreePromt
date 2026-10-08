import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';

import '../domain/tree_view.dart';
import 'glyphs.dart';
import 'motion.dart';
import 'theme.dart';

const _indent = 16.0;

/// Левая панель, вариант C «Активная ветвь»: иконок-папок нет, иерархия —
/// отступом, линии только на пути от проекта до открытого файла.
///
/// Новые строки (раскрыли папку, создали заметку) плавно появляются,
/// при первом показе дерево выстраивается «лесенкой».
class TreePanel extends StatefulWidget {
  const TreePanel({
    super.key,
    required this.rows,
    required this.onTap,
    this.onLongPress,
    this.onSecondaryTap,
    this.rowHeight = 32,
    this.padding = const EdgeInsets.fromLTRB(6, 4, 6, 96),
  });

  final List<TreeRow> rows;
  final ValueChanged<TreeRow> onTap;
  final ValueChanged<TreeRow>? onLongPress;

  /// Правый клик (ПК): строка и точка на экране для контекстного меню.
  final void Function(TreeRow row, Offset globalPosition)? onSecondaryTap;
  final double rowHeight;
  final EdgeInsets padding;

  @override
  State<TreePanel> createState() => _TreePanelState();
}

class _TreePanelState extends State<TreePanel> {
  /// Строки, которые появились в этом кадре и должны въехать с анимацией.
  Set<String> _fresh = {};

  /// Порядковый номер новой строки — для «лесенки».
  final Map<String, int> _order = {};

  @override
  void initState() {
    super.initState();
    _mark(widget.rows.map((r) => r.id));
  }

  @override
  void didUpdateWidget(TreePanel old) {
    super.didUpdateWidget(old);
    final before = {for (final r in old.rows) r.id};
    _mark(widget.rows.where((r) => !before.contains(r.id)).map((r) => r.id));
  }

  void _mark(Iterable<String> ids) {
    final list = ids.toList();
    if (list.isEmpty) return;
    _fresh = {..._fresh, ...list};
    for (var i = 0; i < list.length; i++) {
      _order[list[i]] = i;
    }
    // Строки, построенные позже (прокрутка), уже не анимируются.
    SchedulerBinding.instance.addPostFrameCallback((_) {
      _fresh = {};
      _order.clear();
    });
  }

  @override
  Widget build(BuildContext context) {
    final rows = widget.rows;
    return ListView.builder(
      padding: widget.padding,
      itemCount: rows.length,
      itemBuilder: (context, i) {
        final r = rows[i];
        Widget tile = TreeRowTile(
          key: ValueKey(r.id),
          row: r,
          height: widget.rowHeight,
          topGap: r.type == RowType.project && i > 0 ? 12 : 0,
          onTap: () => widget.onTap(r),
          onLongPress: widget.onLongPress == null ? null : () => widget.onLongPress!(r),
          onSecondaryTap: widget.onSecondaryTap == null ? null : (pos) => widget.onSecondaryTap!(r, pos),
        );
        if (_fresh.contains(r.id)) {
          tile = FadeSlideIn(
            key: ValueKey('in-${r.id}'),
            delay: FadeSlideIn.stagger(_order[r.id] ?? 0, step: 28, max: 12),
            duration: const Duration(milliseconds: 360),
            offset: const Offset(0, -6),
            child: tile,
          );
        }
        return tile;
      },
    );
  }
}

class TreeRowTile extends StatelessWidget {
  const TreeRowTile({
    super.key,
    required this.row,
    required this.height,
    required this.onTap,
    this.onLongPress,
    this.onSecondaryTap,
    this.topGap = 0,
  });

  final TreeRow row;
  final double height;
  final double topGap;
  final VoidCallback onTap;
  final VoidCallback? onLongPress;
  final ValueChanged<Offset>? onSecondaryTap;

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final isFile = row.type == RowType.file;
    final dimmed = isFile && row.kind == 'raw_note' && !row.selected;
    final nameStyle = ui(
      row.type == RowType.project ? 14.5 : 14,
      weight: switch (row.type) {
        RowType.project => FontWeight.w600,
        RowType.folder => FontWeight.w500,
        RowType.file => row.selected ? FontWeight.w500 : FontWeight.w400,
      },
      color: dimmed ? c.muted : c.fg,
      letterSpacing: row.type == RowType.project ? -0.1 : 0,
    );

    Widget? marker;
    if (row.type == RowType.project) {
      marker = Marker(MarkerType.project, size: 9, color: c.fg);
    } else if (isFile) {
      final type = markerFor(kind: row.kind!, structureStatus: row.structureStatus);
      marker = AnimatedSwitcher(
        duration: PtMotion.normal,
        switchInCurve: Curves.easeOutBack,
        transitionBuilder: (child, a) => ScaleTransition(scale: a, child: FadeTransition(opacity: a, child: child)),
        child: Marker(type, key: ValueKey(type), color: dimmed ? c.muted : c.fg),
      );
    }

    final trailing = <Widget>[
      if (row.hasConflict)
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
          decoration: BoxDecoration(border: Border.all(color: c.fg), borderRadius: BorderRadius.circular(99)),
          child: Text('конфликт', style: mono(10, color: c.fg, weight: FontWeight.w500)),
        ),
      if (row.syncError != null)
        Tooltip(
          message: 'Не синхронизировано: ${row.syncError}',
          child: Icon(Icons.cloud_off_rounded, size: 14, color: c.muted),
        ),
      if (!isFile) ...[
        AnimatedOpacity(
          duration: PtMotion.fast,
          opacity: row.childCount == 0 ? 0.5 : 1,
          child: Text('${row.childCount}', style: mono(11, color: c.faint)),
        ),
        AnimatedRotation(
          turns: row.expanded ? 0.25 : 0,
          duration: PtMotion.normal,
          curve: PtMotion.curve,
          child: Icon(Icons.chevron_right_rounded, size: 16, color: c.faint),
        ),
      ],
    ];

    return Padding(
      padding: EdgeInsets.only(top: topGap, bottom: 1),
      child: Semantics(
        selected: row.selected,
        expanded: isFile ? null : row.expanded,
        child: Pressable(
          onTap: onTap,
          onLongPress: onLongPress,
          onSecondaryTapUp: onSecondaryTap == null ? null : (d) => onSecondaryTap!(d.globalPosition),
          color: row.selected ? c.selected : Colors.transparent,
          hoverColor: row.selected ? c.selected : c.hover,
          borderRadius: BorderRadius.circular(PtRadius.sm + 1),
          pressedScale: 0.985,
          child: SizedBox(
            height: height,
            child: Row(children: [
              const SizedBox(width: 8),
              for (final g in row.guides) _AnimatedGuide(guide: g, height: height, color: c.fg),
              if (marker != null) ...[
                SizedBox(width: 12, child: Center(child: marker)),
                const SizedBox(width: 9),
              ] else if (row.guides.isNotEmpty)
                const SizedBox(width: 4),
              Expanded(
                child: AnimatedDefaultTextStyle(
                  duration: PtMotion.fast,
                  style: nameStyle,
                  child: Text(row.name, overflow: TextOverflow.ellipsis, maxLines: 1),
                ),
              ),
              for (final w in trailing) ...[const SizedBox(width: 6), w],
              const SizedBox(width: 8),
            ]),
          ),
        ),
      ),
    );
  }
}

/// Линия ветви: при смене открытого файла «прорисовывается» заново.
class _AnimatedGuide extends StatelessWidget {
  const _AnimatedGuide({required this.guide, required this.height, required this.color});
  final Guide guide;
  final double height;
  final Color color;

  @override
  Widget build(BuildContext context) {
    if (guide == Guide.none) return SizedBox(width: _indent, height: height);
    return TweenAnimationBuilder<double>(
      key: ValueKey(guide),
      tween: Tween(begin: 0, end: 1),
      duration: const Duration(milliseconds: 380),
      curve: PtMotion.curve,
      builder: (context, t, _) =>
          CustomPaint(size: Size(_indent, height), painter: _GuidePainter(guide, color, t)),
    );
  }
}

class _GuidePainter extends CustomPainter {
  _GuidePainter(this.guide, this.color, this.progress);
  final Guide guide;
  final Color color;
  final double progress;

  @override
  void paint(Canvas canvas, Size size) {
    if (guide == Guide.none || progress <= 0) return;
    final p = Paint()
      ..color = color
      ..strokeWidth = 1.5
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;
    const x = 7.0;
    final Path path;
    if (guide == Guide.through) {
      path = Path()
        ..moveTo(x, 0)
        ..lineTo(x, size.height);
    } else {
      final mid = size.height / 2;
      path = Path()
        ..moveTo(x, 0)
        ..lineTo(x, mid - 5)
        ..quadraticBezierTo(x, mid, x + 5, mid)
        ..lineTo(size.width - 1, mid);
    }
    if (progress >= 1) {
      canvas.drawPath(path, p);
      return;
    }
    for (final m in path.computeMetrics()) {
      canvas.drawPath(m.extractPath(0, m.length * progress), p);
    }
  }

  @override
  bool shouldRepaint(_GuidePainter oldDelegate) =>
      oldDelegate.guide != guide || oldDelegate.color != color || oldDelegate.progress != progress;
}
