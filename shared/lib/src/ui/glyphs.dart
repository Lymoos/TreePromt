import 'package:flutter/material.dart';

import '../models/ops.dart';

/// Иконка приложения, вариант C «Плитка-узел» (design/DECISIONS.md).
/// Нарисована в сетке 24×24, как SVG из концепта.
class AppIcon extends StatelessWidget {
  const AppIcon({super.key, this.size = 24, this.tile, this.inner});

  final double size;
  final Color? tile;
  final Color? inner;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return CustomPaint(
      size: Size.square(size),
      painter: AppIconPainter(tile: tile ?? cs.onSurface, inner: inner ?? cs.surface),
    );
  }
}

class AppIconPainter extends CustomPainter {
  AppIconPainter({required this.tile, required this.inner});
  final Color tile, inner;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 24, size.height / 24);
    canvas.drawRRect(
      RRect.fromRectAndRadius(const Rect.fromLTWH(1.5, 1.5, 21, 21), const Radius.circular(5)),
      Paint()..color = tile,
    );
    final stroke = Paint()
      ..color = inner
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.8
      ..strokeCap = StrokeCap.square;
    canvas.drawPath(
      Path()
        ..moveTo(8.5, 10.5)
        ..lineTo(8.5, 17.5)
        ..lineTo(12.5, 17.5),
      stroke,
    );
    canvas.drawLine(const Offset(8.5, 13.5), const Offset(12.5, 13.5), stroke);
    final fill = Paint()..color = inner;
    canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(6, 5.5, 5, 5), const Radius.circular(1)), fill);
    canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(14, 12.25, 4.5, 2.5), const Radius.circular(0.6)), fill);
    canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(14, 16.25, 3, 2.5), const Radius.circular(0.6)), fill);
  }

  @override
  bool shouldRepaint(AppIconPainter oldDelegate) => oldDelegate.tile != tile || oldDelegate.inner != inner;
}

/// Маркеры типа и состояния (design/DECISIONS.md): форма вместо цвета.
enum MarkerType { project, rawNote, aiNone, aiDone, aiStale }

MarkerType markerFor({required String kind, required String structureStatus}) {
  if (kind == NodeKind.rawNote) return MarkerType.rawNote;
  return switch (structureStatus) {
    StructureStatus.done => MarkerType.aiDone,
    StructureStatus.stale => MarkerType.aiStale,
    _ => MarkerType.aiNone,
  };
}

class Marker extends StatelessWidget {
  const Marker(this.type, {super.key, this.size = 10, this.color});
  final MarkerType type;
  final double size;
  final Color? color;

  @override
  Widget build(BuildContext context) => CustomPaint(
        size: Size.square(size),
        painter: _MarkerPainter(type, color ?? Theme.of(context).colorScheme.onSurface),
      );
}

class _MarkerPainter extends CustomPainter {
  _MarkerPainter(this.type, this.color);
  final MarkerType type;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 10, size.height / 10);
    final fill = Paint()..color = color;
    final stroke = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.3
      ..strokeJoin = StrokeJoin.round;
    final diamond = Path()
      ..moveTo(5, 0.9)
      ..lineTo(9.1, 5)
      ..lineTo(5, 9.1)
      ..lineTo(0.9, 5)
      ..close();
    switch (type) {
      case MarkerType.project:
        canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(0.5, 0.5, 9, 9), const Radius.circular(1.5)), fill);
      case MarkerType.rawNote:
        canvas.drawCircle(const Offset(5, 5), 3.4, stroke);
      case MarkerType.aiNone:
        canvas.drawPath(diamond, stroke);
      case MarkerType.aiDone:
        canvas.drawPath(diamond, fill);
        canvas.drawPath(diamond, stroke);
      case MarkerType.aiStale:
        canvas.drawPath(
            Path()
              ..moveTo(5, 0.9)
              ..lineTo(5, 9.1)
              ..lineTo(0.9, 5)
              ..close(),
            fill);
        canvas.drawPath(diamond, stroke);
    }
  }

  @override
  bool shouldRepaint(_MarkerPainter oldDelegate) => oldDelegate.type != type || oldDelegate.color != color;
}
