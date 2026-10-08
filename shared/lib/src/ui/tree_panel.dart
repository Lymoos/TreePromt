import 'package:flutter/material.dart';

import '../domain/tree_view.dart';
import 'glyphs.dart';
import 'theme.dart';

const _indent = 16.0;

/// Левая панель, вариант C «Активная ветвь»: иконок-папок нет, иерархия —
/// отступом, линии только на пути от проекта до открытого файла.
class TreePanel extends StatelessWidget {
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
  Widget build(BuildContext context) => ListView.builder(
        padding: padding,
        itemCount: rows.length,
        itemBuilder: (context, i) => TreeRowTile(
          key: ValueKey(rows[i].id),
          row: rows[i],
          height: rowHeight,
          topGap: rows[i].type == RowType.project && i > 0 ? 10 : 0,
          onTap: () => onTap(rows[i]),
          onLongPress: onLongPress == null ? null : () => onLongPress!(rows[i]),
          onSecondaryTap: onSecondaryTap == null ? null : (pos) => onSecondaryTap!(rows[i], pos),
        ),
      );
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
      14,
      weight: switch (row.type) {
        RowType.project => FontWeight.w600,
        RowType.folder => FontWeight.w500,
        RowType.file => FontWeight.w400,
      },
      color: dimmed ? c.muted : c.fg,
    );

    Widget? marker;
    if (row.type == RowType.project) {
      marker = Marker(MarkerType.project, size: 9, color: c.fg);
    } else if (isFile) {
      marker = Marker(markerFor(kind: row.kind!, structureStatus: row.structureStatus), color: c.fg);
    }

    final trailing = <Widget>[
      if (row.hasConflict) Text('конфликт', style: mono(10.5, color: c.fg)),
      if (row.syncError != null)
        Tooltip(message: 'Не синхронизировано: ${row.syncError}', child: Text('×', style: mono(13, color: c.fg))),
      if (!isFile) ...[
        Text('${row.childCount}', style: mono(11, color: c.faint)),
        AnimatedRotation(
          turns: row.expanded ? 0.25 : 0,
          duration: const Duration(milliseconds: 150),
          child: Icon(Icons.chevron_right, size: 14, color: c.faint),
        ),
      ],
    ];

    return Padding(
      padding: EdgeInsets.only(top: topGap),
      child: Semantics(
        selected: row.selected,
        expanded: isFile ? null : row.expanded,
        button: true,
        child: Material(
          color: row.selected ? c.selected : Colors.transparent,
          borderRadius: BorderRadius.circular(5),
          child: InkWell(
            borderRadius: BorderRadius.circular(5),
            hoverColor: c.hover,
            highlightColor: c.hover,
            splashColor: Colors.transparent,
            onTap: onTap,
            onLongPress: onLongPress,
            onSecondaryTapUp: onSecondaryTap == null ? null : (d) => onSecondaryTap!(d.globalPosition),
            child: SizedBox(
              height: height,
              child: Row(children: [
                const SizedBox(width: 6),
                for (final g in row.guides)
                  CustomPaint(size: Size(_indent, height), painter: _GuidePainter(g, c.fg)),
                if (marker != null) ...[marker, const SizedBox(width: 8)] else if (row.guides.isNotEmpty) const SizedBox(width: 4),
                Expanded(child: Text(row.name, style: nameStyle, overflow: TextOverflow.ellipsis, maxLines: 1)),
                for (final w in trailing) ...[const SizedBox(width: 6), w],
                const SizedBox(width: 8),
              ]),
            ),
          ),
        ),
      ),
    );
  }
}

class _GuidePainter extends CustomPainter {
  _GuidePainter(this.guide, this.color);
  final Guide guide;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    if (guide == Guide.none) return;
    final p = Paint()
      ..color = color
      ..strokeWidth = 1.5
      ..style = PaintingStyle.stroke;
    const x = 7.0;
    if (guide == Guide.through) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), p);
      return;
    }
    final mid = size.height / 2;
    canvas.drawPath(
      Path()
        ..moveTo(x, 0)
        ..lineTo(x, mid - 5)
        ..quadraticBezierTo(x, mid, x + 5, mid)
        ..lineTo(size.width - 1, mid),
      p,
    );
  }

  @override
  bool shouldRepaint(_GuidePainter oldDelegate) => oldDelegate.guide != guide || oldDelegate.color != color;
}
