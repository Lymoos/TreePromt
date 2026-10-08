import 'package:flutter/material.dart';

/// Токены цвета из design/concept-v1.html. Только чёрный, белый и серые:
/// глубину дают тени, полупрозрачность и движение, а не цвет.
@immutable
class PtColors extends ThemeExtension<PtColors> {
  const PtColors({
    required this.bg,
    required this.fg,
    required this.muted,
    required this.faint,
    required this.line,
    required this.lineStrong,
    required this.hover,
    required this.selected,
    required this.panel,
    required this.card,
    required this.shadow,
  });

  final Color bg, fg, muted, faint, line, lineStrong, hover, selected, panel;

  /// Поверхность карточек, меню и диалогов (в тёмной теме — чуть светлее фона).
  final Color card;

  /// Цвет мягких теней.
  final Color shadow;

  static const light = PtColors(
    bg: Color(0xFFFFFFFF),
    fg: Color(0xFF111111),
    muted: Color(0xFF6B6B6F),
    faint: Color(0xFFA6A6AA),
    line: Color(0xFFEBEBED),
    lineStrong: Color(0xFFD3D3D7),
    hover: Color(0xFFF2F2F4),
    selected: Color(0xFFE9E9EC),
    panel: Color(0xFFF7F7F8),
    card: Color(0xFFFFFFFF),
    shadow: Color(0x1A101014),
  );

  static const dark = PtColors(
    bg: Color(0xFF0E0E0F),
    fg: Color(0xFFECECEE),
    muted: Color(0xFF9A9AA0),
    faint: Color(0xFF5E5E64),
    line: Color(0xFF232326),
    lineStrong: Color(0xFF38383D),
    hover: Color(0xFF1B1B1E),
    selected: Color(0xFF26262A),
    panel: Color(0xFF131315),
    card: Color(0xFF18181B),
    shadow: Color(0x66000000),
  );

  /// Мягкая многослойная тень для карточек, меню и плавающих кнопок.
  List<BoxShadow> elevation([double level = 1]) => [
        BoxShadow(color: shadow.withValues(alpha: shadow.a * 0.6), blurRadius: 2 * level, offset: Offset(0, level)),
        BoxShadow(color: shadow, blurRadius: 18 * level, offset: Offset(0, 6 * level), spreadRadius: -4 * level),
      ];

  @override
  PtColors copyWith() => this;

  @override
  PtColors lerp(ThemeExtension<PtColors>? other, double t) {
    if (other is! PtColors) return this;
    Color l(Color a, Color b) => Color.lerp(a, b, t)!;
    return PtColors(
      bg: l(bg, other.bg),
      fg: l(fg, other.fg),
      muted: l(muted, other.muted),
      faint: l(faint, other.faint),
      line: l(line, other.line),
      lineStrong: l(lineStrong, other.lineStrong),
      hover: l(hover, other.hover),
      selected: l(selected, other.selected),
      panel: l(panel, other.panel),
      card: l(card, other.card),
      shadow: l(shadow, other.shadow),
    );
  }
}

/// Длительности и кривые анимаций — одни на всё приложение.
abstract final class PtMotion {
  static const fast = Duration(milliseconds: 160);
  static const normal = Duration(milliseconds: 260);
  static const slow = Duration(milliseconds: 420);

  /// Основная кривая: быстрый старт, мягкая остановка.
  static const curve = Cubic(0.2, 0, 0, 1);
  static const out = Curves.easeOutCubic;
}

/// Скругления.
abstract final class PtRadius {
  static const sm = 8.0;
  static const md = 12.0;
  static const lg = 16.0;
  static const xl = 24.0;
}

extension PtTheme on BuildContext {
  PtColors get pt => Theme.of(this).extension<PtColors>()!;
}

const _pkg = 'prompttree_shared';

/// Onest — переменный шрифт: вес задаётся осью wght, иначе Flutter рисует всё обычным.
TextStyle ui(double size, {FontWeight weight = FontWeight.w400, Color? color, double? height, double? letterSpacing}) =>
    TextStyle(
      fontFamily: 'Onest',
      package: _pkg,
      fontSize: size,
      fontWeight: weight,
      fontVariations: [FontVariation('wght', weight.value.toDouble())],
      color: color,
      height: height,
      letterSpacing: letterSpacing,
    );

TextStyle mono(double size, {FontWeight weight = FontWeight.w400, Color? color, double? letterSpacing}) => TextStyle(
      fontFamily: 'JetBrainsMono',
      package: _pkg,
      fontSize: size,
      fontWeight: weight,
      fontVariations: [FontVariation('wght', weight.value.toDouble())],
      color: color,
      letterSpacing: letterSpacing,
    );

ThemeData buildTheme(Brightness b) {
  final c = b == Brightness.light ? PtColors.light : PtColors.dark;
  final base = ThemeData(brightness: b, useMaterial3: true);
  final text = base.textTheme.apply(fontFamily: 'packages/$_pkg/Onest', bodyColor: c.fg, displayColor: c.fg);
  final rMd = BorderRadius.circular(PtRadius.md);
  WidgetStateProperty<Color?> overlay(Color color) => WidgetStateProperty.resolveWith((s) {
        if (s.contains(WidgetState.pressed)) return color.withValues(alpha: 0.14);
        if (s.contains(WidgetState.hovered)) return color.withValues(alpha: 0.07);
        if (s.contains(WidgetState.focused)) return color.withValues(alpha: 0.10);
        return null;
      });
  const buttonPadding = EdgeInsets.symmetric(horizontal: 18, vertical: 14);
  return base.copyWith(
    scaffoldBackgroundColor: c.bg,
    canvasColor: c.bg,
    splashFactory: InkRipple.splashFactory,
    splashColor: c.fg.withValues(alpha: 0.06),
    highlightColor: c.fg.withValues(alpha: 0.04),
    hoverColor: c.hover,
    focusColor: c.selected,
    colorScheme: ColorScheme(
      brightness: b,
      primary: c.fg,
      onPrimary: c.bg,
      secondary: c.fg,
      onSecondary: c.bg,
      error: c.fg,
      onError: c.bg,
      surface: c.bg,
      onSurface: c.fg,
      surfaceContainerHighest: c.panel,
      surfaceContainerHigh: c.card,
      surfaceContainer: c.card,
      surfaceContainerLow: c.card,
      outline: c.lineStrong,
      outlineVariant: c.line,
      shadow: c.shadow,
    ),
    textTheme: text.copyWith(
      bodyMedium: ui(15, color: c.fg, height: 1.55),
      bodyLarge: ui(16, color: c.fg, height: 1.55),
      titleLarge: ui(22, weight: FontWeight.w700, color: c.fg),
      titleMedium: ui(16, weight: FontWeight.w600, color: c.fg),
      labelLarge: ui(14, weight: FontWeight.w500, color: c.fg),
    ),
    // Плавные переходы между экранами на всех платформах.
    pageTransitionsTheme: const PageTransitionsTheme(builders: {
      TargetPlatform.android: PtPageTransitionsBuilder(),
      TargetPlatform.iOS: PtPageTransitionsBuilder(),
      TargetPlatform.macOS: PtPageTransitionsBuilder(),
      TargetPlatform.windows: PtPageTransitionsBuilder(),
      TargetPlatform.linux: PtPageTransitionsBuilder(),
      TargetPlatform.fuchsia: PtPageTransitionsBuilder(),
    }),
    dividerColor: c.line,
    dividerTheme: DividerThemeData(color: c.line, thickness: 1, space: 1),
    appBarTheme: AppBarTheme(
      backgroundColor: c.bg,
      foregroundColor: c.fg,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      titleSpacing: 4,
      titleTextStyle: ui(18, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.2),
      iconTheme: IconThemeData(color: c.fg, size: 22),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(
        foregroundColor: c.fg,
        shape: RoundedRectangleBorder(borderRadius: rMd),
      ).copyWith(overlayColor: overlay(c.fg)),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: c.fg,
        foregroundColor: c.bg,
        disabledBackgroundColor: c.selected,
        disabledForegroundColor: c.faint,
        textStyle: ui(14, weight: FontWeight.w600),
        shape: RoundedRectangleBorder(borderRadius: rMd),
        padding: buttonPadding,
        animationDuration: PtMotion.fast,
      ).copyWith(overlayColor: overlay(c.bg)),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: c.fg,
        backgroundColor: c.card,
        side: BorderSide(color: c.lineStrong),
        textStyle: ui(14, weight: FontWeight.w600),
        shape: RoundedRectangleBorder(borderRadius: rMd),
        padding: buttonPadding,
        animationDuration: PtMotion.fast,
      ).copyWith(overlayColor: overlay(c.fg)),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        foregroundColor: c.fg,
        textStyle: ui(14, weight: FontWeight.w600),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(PtRadius.sm + 2)),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      ).copyWith(overlayColor: overlay(c.fg)),
    ),
    floatingActionButtonTheme: FloatingActionButtonThemeData(
      backgroundColor: c.fg,
      foregroundColor: c.bg,
      elevation: 0,
      focusElevation: 0,
      hoverElevation: 0,
      highlightElevation: 0,
      shape: const StadiumBorder(),
      extendedTextStyle: ui(15, weight: FontWeight.w600),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: c.panel,
      hoverColor: c.hover,
      border: OutlineInputBorder(borderRadius: rMd, borderSide: BorderSide(color: c.line)),
      enabledBorder: OutlineInputBorder(borderRadius: rMd, borderSide: BorderSide(color: c.line)),
      focusedBorder: OutlineInputBorder(borderRadius: rMd, borderSide: BorderSide(color: c.fg, width: 1.5)),
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
      labelStyle: ui(14, color: c.muted),
      floatingLabelStyle: ui(13, color: c.fg, weight: FontWeight.w500),
      hintStyle: ui(14, color: c.faint),
      prefixIconColor: c.muted,
      isDense: true,
    ),
    bottomSheetTheme: BottomSheetThemeData(
      backgroundColor: c.card,
      surfaceTintColor: Colors.transparent,
      modalBarrierColor: Colors.black.withValues(alpha: b == Brightness.light ? 0.28 : 0.55),
      dragHandleColor: c.lineStrong,
      dragHandleSize: const Size(36, 4),
      elevation: 0,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(PtRadius.xl + 4))),
    ),
    dialogTheme: DialogTheme(
      backgroundColor: c.card,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(PtRadius.xl - 4), side: BorderSide(color: c.line)),
      titleTextStyle: ui(18, weight: FontWeight.w700, color: c.fg, letterSpacing: -0.2),
      contentTextStyle: ui(14, color: c.muted, height: 1.5),
      actionsPadding: const EdgeInsets.fromLTRB(20, 4, 20, 20),
    ),
    popupMenuTheme: PopupMenuThemeData(
      color: c.card,
      surfaceTintColor: Colors.transparent,
      elevation: 10,
      shadowColor: c.shadow,
      menuPadding: const EdgeInsets.all(6),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(PtRadius.md + 2), side: BorderSide(color: c.line)),
      textStyle: ui(14, color: c.fg),
      labelTextStyle: WidgetStatePropertyAll(ui(14, color: c.fg)),
    ),
    tooltipTheme: TooltipThemeData(
      decoration: BoxDecoration(color: c.fg, borderRadius: BorderRadius.circular(PtRadius.sm)),
      textStyle: ui(12, color: c.bg, weight: FontWeight.w500),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      waitDuration: const Duration(milliseconds: 400),
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: c.fg,
      contentTextStyle: ui(14, color: c.bg, weight: FontWeight.w500),
      behavior: SnackBarBehavior.floating,
      elevation: 0,
      insetPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(PtRadius.md + 2)),
    ),
    scrollbarTheme: ScrollbarThemeData(
      thickness: const WidgetStatePropertyAll(6),
      radius: const Radius.circular(3),
      thumbColor: WidgetStatePropertyAll(c.lineStrong),
    ),
    progressIndicatorTheme: ProgressIndicatorThemeData(color: c.fg),
    textSelectionTheme: TextSelectionThemeData(
      cursorColor: c.fg,
      selectionColor: c.fg.withValues(alpha: 0.18),
      selectionHandleColor: c.fg,
    ),
    extensions: [c],
  );
}

/// Переход между экранами: новый выезжает чуть справа и проявляется,
/// старый уходит влево и слегка тускнеет.
class PtPageTransitionsBuilder extends PageTransitionsBuilder {
  const PtPageTransitionsBuilder();

  @override
  Widget buildTransitions<T>(PageRoute<T> route, BuildContext context, Animation<double> animation,
      Animation<double> secondaryAnimation, Widget child) {
    if (Theme.of(context).platform == TargetPlatform.iOS) {
      // На iOS — родной переход с жестом «назад» от края экрана.
      return const CupertinoPageTransitionsBuilder()
          .buildTransitions(route, context, animation, secondaryAnimation, child);
    }
    final inCurve = CurvedAnimation(parent: animation, curve: PtMotion.curve, reverseCurve: Curves.easeInCubic);
    final outCurve = CurvedAnimation(parent: secondaryAnimation, curve: PtMotion.curve);
    return FadeTransition(
      opacity: Tween<double>(begin: 0, end: 1).animate(CurvedAnimation(parent: animation, curve: const Interval(0, 0.7))),
      child: SlideTransition(
        position: Tween(begin: const Offset(0.08, 0), end: Offset.zero).animate(inCurve),
        child: SlideTransition(
          position: Tween(begin: Offset.zero, end: const Offset(-0.04, 0)).animate(outCurve),
          child: FadeTransition(
            opacity: Tween<double>(begin: 1, end: 0.6).animate(outCurve),
            child: child,
          ),
        ),
      ),
    );
  }
}
