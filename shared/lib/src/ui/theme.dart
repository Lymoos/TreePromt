import 'package:flutter/material.dart';

/// Токены цвета из design/concept-v1.html. Только чёрный, белый и серые.
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
  });

  final Color bg, fg, muted, faint, line, lineStrong, hover, selected, panel;

  static const light = PtColors(
    bg: Color(0xFFFFFFFF),
    fg: Color(0xFF111111),
    muted: Color(0xFF6E6E6E),
    faint: Color(0xFFA8A8A8),
    line: Color(0xFFE7E7E7),
    lineStrong: Color(0xFFC9C9C9),
    hover: Color(0xFFF3F3F3),
    selected: Color(0xFFEBEBEB),
    panel: Color(0xFFFAFAFA),
  );

  static const dark = PtColors(
    bg: Color(0xFF0E0E0E),
    fg: Color(0xFFECECEC),
    muted: Color(0xFF9A9A9A),
    faint: Color(0xFF5C5C5C),
    line: Color(0xFF232323),
    lineStrong: Color(0xFF3A3A3A),
    hover: Color(0xFF1A1A1A),
    selected: Color(0xFF242424),
    panel: Color(0xFF131313),
  );

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
    );
  }
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
  return base.copyWith(
    scaffoldBackgroundColor: c.bg,
    canvasColor: c.bg,
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
      outline: c.lineStrong,
      outlineVariant: c.line,
    ),
    textTheme: text.copyWith(
      bodyMedium: ui(15, color: c.fg, height: 1.55),
      bodyLarge: ui(16, color: c.fg, height: 1.55),
      titleLarge: ui(22, weight: FontWeight.w700, color: c.fg),
      titleMedium: ui(16, weight: FontWeight.w600, color: c.fg),
      labelLarge: ui(14, weight: FontWeight.w500, color: c.fg),
    ),
    dividerColor: c.line,
    dividerTheme: DividerThemeData(color: c.line, thickness: 1, space: 1),
    appBarTheme: AppBarTheme(
      backgroundColor: c.bg,
      foregroundColor: c.fg,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      titleTextStyle: ui(16, weight: FontWeight.w600, color: c.fg),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: c.fg,
        foregroundColor: c.bg,
        disabledBackgroundColor: c.line,
        disabledForegroundColor: c.faint,
        textStyle: ui(14, weight: FontWeight.w500),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: c.fg,
        side: BorderSide(color: c.lineStrong),
        textStyle: ui(14, weight: FontWeight.w500),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(foregroundColor: c.fg, textStyle: ui(14, weight: FontWeight.w500)),
    ),
    inputDecorationTheme: InputDecorationTheme(
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: c.lineStrong)),
      enabledBorder:
          OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: c.lineStrong)),
      focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: c.fg)),
      labelStyle: ui(14, color: c.muted),
      hintStyle: ui(14, color: c.faint),
      isDense: true,
    ),
    bottomSheetTheme: BottomSheetThemeData(
      backgroundColor: c.bg,
      surfaceTintColor: Colors.transparent,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(12))),
    ),
    dialogTheme: DialogTheme(
      backgroundColor: c.bg,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10), side: BorderSide(color: c.line)),
      titleTextStyle: ui(17, weight: FontWeight.w600, color: c.fg),
      contentTextStyle: ui(14, color: c.muted, height: 1.5),
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: c.fg,
      contentTextStyle: ui(14, color: c.bg),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
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
