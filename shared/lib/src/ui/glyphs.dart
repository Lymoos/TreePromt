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

/// Цвета иконки для лаунчеров и магазинов: глубокий чёрный с лёгким
/// градиентом сверху вниз и белый рисунок.
const iconTileTop = Color(0xFF2B2B2B);
const iconTileBottom = Color(0xFF070707);
const iconTile = Color(0xFF111111);

class AppIconPainter extends CustomPainter {
  /// [bleed] — плитка занимает весь холст без полей (иконки лаунчеров:
  /// система сама скругляет углы). [radius] — скругление в долях стороны,
  /// 0 — квадрат. [glyphScale] — размер рисунка внутри плитки.
  AppIconPainter({
    required this.tile,
    required this.inner,
    this.bleed = false,
    this.gradient = false,
    this.radius,
    this.glyphScale = 1,
    this.progress = 1,
  });

  final Color tile, inner;
  final bool bleed;
  final bool gradient;
  final double? radius;
  final double glyphScale;

  /// 0…1: ветви «прорастают» от проекта к файлам (анимация логотипа).
  final double progress;

  static const _tileRect = Rect.fromLTWH(1.5, 1.5, 21, 21);

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.width;
    final tileRect = bleed ? Offset.zero & size : Rect.fromLTWH(s * 1.5 / 24, s * 1.5 / 24, s * 21 / 24, s * 21 / 24);
    final r = (radius ?? (bleed ? 0 : 5 / 21)) * tileRect.width;
    final tilePaint = Paint()..color = tile;
    if (gradient) {
      tilePaint.shader = const LinearGradient(
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
        colors: [iconTileTop, iconTileBottom],
      ).createShader(tileRect);
    }
    canvas.drawRRect(RRect.fromRectAndRadius(tileRect, Radius.circular(r)), tilePaint);

    // Рисунок — в той же сетке 24×24, вписанной в плитку.
    canvas.save();
    final k = tileRect.width / _tileRect.width * glyphScale;
    canvas.translate(tileRect.center.dx, tileRect.center.dy);
    canvas.scale(k);
    canvas.translate(-12, -12);
    _glyph(canvas);
    canvas.restore();
  }

  void _glyph(Canvas canvas) {
    final t = Curves.easeOutCubic.transform(progress.clamp(0, 1));
    final fill = Paint()..color = inner;
    // Проект — появляется первым.
    final pScale = Curves.easeOutBack.transform((progress * 2.2).clamp(0, 1));
    canvas.save();
    canvas.translate(8.5, 8);
    canvas.scale(pScale);
    canvas.translate(-8.5, -8);
    canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(6, 5.5, 5, 5), const Radius.circular(1)), fill);
    canvas.restore();
    if (t <= 0) return;

    final stroke = Paint()
      ..color = inner
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.8
      ..strokeCap = StrokeCap.square;
    final trunk = Path()
      ..moveTo(8.5, 10.5)
      ..lineTo(8.5, 17.5)
      ..lineTo(12.5, 17.5);
    final branch = Path()
      ..moveTo(8.5, 13.5)
      ..lineTo(12.5, 13.5);
    if (t >= 1) {
      canvas.drawPath(trunk, stroke);
      canvas.drawPath(branch, stroke);
    } else {
      for (final path in [trunk, branch]) {
        for (final m in path.computeMetrics()) {
          canvas.drawPath(m.extractPath(0, m.length * t), stroke);
        }
      }
    }
    // Файлы — в конце.
    final f = Curves.easeOut.transform(((progress - 0.55) / 0.45).clamp(0, 1));
    if (f <= 0) return;
    final fileFill = Paint()..color = inner.withValues(alpha: inner.a * f);
    canvas.drawRRect(
        RRect.fromRectAndRadius(Rect.fromLTWH(14, 12.25, 4.5 * f, 2.5), const Radius.circular(0.6)), fileFill);
    canvas.drawRRect(
        RRect.fromRectAndRadius(Rect.fromLTWH(14, 16.25, 3 * f, 2.5), const Radius.circular(0.6)), fileFill);
  }

  @override
  bool shouldRepaint(AppIconPainter oldDelegate) =>
      oldDelegate.tile != tile ||
      oldDelegate.inner != inner ||
      oldDelegate.bleed != bleed ||
      oldDelegate.gradient != gradient ||
      oldDelegate.radius != radius ||
      oldDelegate.glyphScale != glyphScale ||
      oldDelegate.progress != progress;
}

/// Логотип, который «вырастает» при появлении: проект, затем ветви, затем файлы.
class AnimatedAppIcon extends StatefulWidget {
  const AnimatedAppIcon({
    super.key,
    this.size = 40,
    this.tile,
    this.inner,
    this.delay = Duration.zero,
    this.duration = const Duration(milliseconds: 1100),
  });

  final double size;
  final Color? tile, inner;
  final Duration delay, duration;

  @override
  State<AnimatedAppIcon> createState() => _AnimatedAppIconState();
}

class _AnimatedAppIconState extends State<AnimatedAppIcon> with SingleTickerProviderStateMixin {
  late final _c = AnimationController(vsync: this, duration: widget.duration);

  @override
  void initState() {
    super.initState();
    Future.delayed(widget.delay, () {
      if (mounted) _c.forward();
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (MediaQuery.maybeDisableAnimationsOf(context) ?? false) _c.value = 1;
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return AnimatedBuilder(
      animation: _c,
      builder: (context, _) {
        final appear = Curves.easeOutCubic.transform((_c.value * 2.5).clamp(0, 1));
        return Opacity(
          opacity: appear,
          child: Transform.scale(
            scale: 0.85 + 0.15 * appear,
            child: CustomPaint(
              size: Size.square(widget.size),
              painter: AppIconPainter(
                tile: widget.tile ?? cs.onSurface,
                inner: widget.inner ?? cs.surface,
                progress: _c.value,
              ),
            ),
          ),
        );
      },
    );
  }
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
