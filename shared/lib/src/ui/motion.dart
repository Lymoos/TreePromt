import 'dart:ui' as ui;

import 'package:flutter/material.dart';

import 'theme.dart';

bool _reduceMotion(BuildContext context) => MediaQuery.maybeDisableAnimationsOf(context) ?? false;

/// Появление при первом показе: проявляется и чуть поднимается.
/// [delay] — для «лесенки» в списках. Без таймеров: задержка — часть длительности
/// контроллера, поэтому виджет можно снять в любой момент.
class FadeSlideIn extends StatefulWidget {
  const FadeSlideIn({
    super.key,
    required this.child,
    this.delay = Duration.zero,
    this.duration = const Duration(milliseconds: 420),
    this.offset = const Offset(0, 12),
    this.scale = 1,
  });

  /// Задержка для i-го элемента списка: первые появляются по очереди, дальше — сразу.
  static Duration stagger(int index, {int step = 40, int max = 8}) =>
      Duration(milliseconds: step * (index < max ? index : max));

  final Widget child;
  final Duration delay, duration;

  /// Сдвиг в логических пикселях в начале анимации.
  final Offset offset;

  /// Начальный масштаб (1 — без масштабирования).
  final double scale;

  @override
  State<FadeSlideIn> createState() => _FadeSlideInState();
}

class _FadeSlideInState extends State<FadeSlideIn> with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(vsync: this, duration: widget.delay + widget.duration);
  late final Animation<double> _t = CurvedAnimation(
    parent: _c,
    curve: Interval(
      widget.delay.inMicroseconds / (widget.delay + widget.duration).inMicroseconds,
      1,
      curve: PtMotion.curve,
    ),
  );

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_c.status == AnimationStatus.dismissed) {
      _reduceMotion(context) ? _c.value = 1 : _c.forward();
    }
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AnimatedBuilder(
        animation: _t,
        child: widget.child,
        // Структура дерева не меняется по ходу анимации: иначе вложенные
        // виджеты (поле ввода с фокусом) пересоздались бы в последнем кадре.
        builder: (context, child) {
          final v = _t.value;
          return Opacity(
            opacity: v.clamp(0, 1),
            child: Transform.translate(
              offset: widget.offset * (1 - v),
              child: Transform.scale(scale: widget.scale + (1 - widget.scale) * v, child: child),
            ),
          );
        },
      );
}

/// Нажимаемая поверхность: плавная подсветка при наведении,
/// лёгкое «вдавливание» при нажатии.
class Pressable extends StatefulWidget {
  const Pressable({
    super.key,
    required this.child,
    this.onTap,
    this.onLongPress,
    this.onSecondaryTapUp,
    this.color,
    this.hoverColor,
    this.borderRadius = const BorderRadius.all(Radius.circular(PtRadius.md)),
    this.pressedScale = 0.97,
    this.border,
    this.shadow,
    this.semanticLabel,
  });

  final Widget child;
  final VoidCallback? onTap;
  final VoidCallback? onLongPress;
  final GestureTapUpCallback? onSecondaryTapUp;
  final Color? color;
  final Color? hoverColor;
  final BorderRadius borderRadius;
  final double pressedScale;
  final BoxBorder? border;
  final List<BoxShadow>? shadow;
  final String? semanticLabel;

  @override
  State<Pressable> createState() => _PressableState();
}

class _PressableState extends State<Pressable> {
  bool _hover = false;
  bool _down = false;

  void _setDown(bool v) {
    if (_down != v) setState(() => _down = v);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.pt;
    final enabled = widget.onTap != null || widget.onLongPress != null || widget.onSecondaryTapUp != null;
    final base = widget.color ?? Colors.transparent;
    final hover = widget.hoverColor ?? c.hover;
    final bg = !enabled
        ? base
        : _down
            ? Color.alphaBlend(c.fg.withValues(alpha: 0.05), hover)
            : _hover
                ? hover
                : base;
    return MouseRegion(
      cursor: enabled ? SystemMouseCursors.click : MouseCursor.defer,
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() {
        _hover = false;
        _down = false;
      }),
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTapDown: enabled ? (_) => _setDown(true) : null,
        onTapUp: enabled ? (_) => _setDown(false) : null,
        onTapCancel: enabled ? () => _setDown(false) : null,
        onTap: widget.onTap,
        onLongPress: widget.onLongPress == null
            ? null
            : () {
                _setDown(false);
                widget.onLongPress!();
              },
        onSecondaryTapUp: widget.onSecondaryTapUp,
        child: Semantics(
          button: enabled,
          label: widget.semanticLabel,
          child: AnimatedScale(
            scale: _down ? widget.pressedScale : 1,
            duration: PtMotion.fast,
            curve: PtMotion.out,
            child: AnimatedContainer(
              duration: PtMotion.fast,
              curve: PtMotion.out,
              decoration: BoxDecoration(
                color: bg,
                borderRadius: widget.borderRadius,
                border: widget.border,
                boxShadow: widget.shadow,
              ),
              child: widget.child,
            ),
          ),
        ),
      ),
    );
  }
}

/// Диалог с размытым фоном: появляется с лёгким увеличением.
Future<T?> showPtDialog<T>(BuildContext context, {required WidgetBuilder builder, bool dismissible = true}) {
  final dark = Theme.of(context).brightness == Brightness.dark;
  return showGeneralDialog<T>(
    context: context,
    barrierDismissible: dismissible,
    barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
    barrierColor: Colors.black.withValues(alpha: dark ? 0.5 : 0.22),
    transitionDuration: const Duration(milliseconds: 280),
    pageBuilder: (context, _, __) => Builder(builder: builder),
    transitionBuilder: (context, animation, _, child) {
      final t = CurvedAnimation(parent: animation, curve: PtMotion.curve, reverseCurve: Curves.easeInCubic);
      return AnimatedBuilder(
        animation: t,
        child: child,
        builder: (context, child) {
          final v = t.value;
          final sigma = 6 * v;
          final content = Opacity(
            opacity: v.clamp(0, 1),
            child: Transform.scale(scale: 0.94 + 0.06 * v, child: child),
          );
          if (sigma < 0.1) return content;
          return BackdropFilter(filter: ui.ImageFilter.blur(sigmaX: sigma, sigmaY: sigma), child: content);
        },
      );
    },
  );
}

/// Плавная смена содержимого: старое растворяется, новое проявляется со сдвигом.
class FadeSwitcher extends StatelessWidget {
  const FadeSwitcher({
    super.key,
    required this.child,
    this.duration = PtMotion.normal,
    this.offset = 0.02,
    this.expand = false,
  });
  final Widget child;
  final Duration duration;

  /// Растянуть содержимое на всё доступное место (панели и экраны).
  final bool expand;

  /// Сдвиг по вертикали в долях высоты.
  final double offset;

  @override
  Widget build(BuildContext context) => AnimatedSwitcher(
        duration: duration,
        switchInCurve: PtMotion.curve,
        switchOutCurve: Curves.easeIn,
        layoutBuilder: (current, previous) => Stack(
          alignment: Alignment.topLeft,
          fit: expand ? StackFit.expand : StackFit.loose,
          children: [...previous, if (current != null) current],
        ),
        transitionBuilder: (child, animation) => FadeTransition(
          opacity: animation,
          child: SlideTransition(
            position: Tween(begin: Offset(0, offset), end: Offset.zero).animate(animation),
            child: child,
          ),
        ),
        child: child,
      );
}
