import 'package:flutter/material.dart';

/// Neutral iOS palette and typography for 點擊精靈.
class ClickSpiritColors {
  const ClickSpiritColors._();

  static const systemBlue = Color(0xFF007AFF);
  static const systemRed = Color(0xFFFF3B30);
  static const systemGreen = Color(0xFF34C759);
  static const systemOrange = Color(0xFFFF9500);

  static const labelLight = Color(0xFF000000);
  static const secondaryLabelLight = Color(0x993C3C43);
  static const tertiaryLabelLight = Color(0x4D3C3C43);
  static const labelDark = Color(0xFFFFFFFF);
  static const secondaryLabelDark = Color(0x99EBEBF5);
  static const tertiaryLabelDark = Color(0x4DEBEBF5);

  static const systemGray = Color(0xFF8E8E93);
  static const systemGray5Light = Color(0xFFE5E5EA);
  static const systemGray6Light = Color(0xFFF2F2F7);
  static const systemGray6Dark = Color(0xFF1C1C1E);

  static bool isDark(BuildContext context) => Theme.of(context).brightness == Brightness.dark;

  static Color label(BuildContext context) => isDark(context) ? labelDark : labelLight;

  static Color secondary(BuildContext context) => isDark(context) ? secondaryLabelDark : secondaryLabelLight;

  static Color tertiary(BuildContext context) => isDark(context) ? tertiaryLabelDark : tertiaryLabelLight;
}

class ClickSpiritTheme {
  const ClickSpiritTheme._();

  static const fontFallback = <String>[
    'SF Pro Text',
    '.AppleSystemUIFont',
    'Inter',
    'Noto Sans CJK TC',
    'PingFang TC',
    'Microsoft JhengHei',
  ];

  static ThemeData light() => _build(Brightness.light);

  static ThemeData dark() => _build(Brightness.dark);

  static InputDecoration fieldDecoration(BuildContext context) {
    final dark = ClickSpiritColors.isDark(context);
    final radius = BorderRadius.circular(12);
    final hairline = dark ? Colors.white.withValues(alpha: 0.10) : Colors.black.withValues(alpha: 0.06);
    final fill = dark ? Colors.white.withValues(alpha: 0.08) : ClickSpiritColors.systemGray6Light;
    OutlineInputBorder edge(Color color, [double width = 1]) {
      return OutlineInputBorder(
        borderRadius: radius,
        borderSide: BorderSide(color: color, width: width),
      );
    }

    return InputDecoration(
      isDense: true,
      filled: true,
      fillColor: fill,
      contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      border: edge(hairline),
      enabledBorder: edge(hairline),
      disabledBorder: edge(hairline),
      focusedBorder: edge(ClickSpiritColors.systemBlue, 1.5),
      errorBorder: edge(ClickSpiritColors.systemRed),
      focusedErrorBorder: edge(ClickSpiritColors.systemRed, 1.5),
    );
  }

  static ThemeData _build(Brightness brightness) {
    final dark = brightness == Brightness.dark;
    final scheme = dark ? _darkScheme : _lightScheme;
    final label = dark ? ClickSpiritColors.labelDark : ClickSpiritColors.labelLight;
    final secondary = dark ? ClickSpiritColors.secondaryLabelDark : ClickSpiritColors.secondaryLabelLight;
    final onGlass = dark ? Colors.white.withValues(alpha: 0.06) : ClickSpiritColors.systemGray6Light;
    final hairline = dark ? Colors.white.withValues(alpha: 0.10) : Colors.black.withValues(alpha: 0.06);

    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: scheme,
      fontFamily: fontFallback.first,
      fontFamilyFallback: fontFallback,
      scaffoldBackgroundColor: Colors.transparent,
      canvasColor: dark ? const Color(0xFF070B16) : const Color(0xFFE7F0FF),
      splashFactory: InkRipple.splashFactory,
      highlightColor: ClickSpiritColors.systemBlue.withValues(alpha: 0.06),
      splashColor: ClickSpiritColors.systemBlue.withValues(alpha: 0.08),
      applyElevationOverlayColor: false,
      textTheme: _text(label, secondary),
      iconTheme: IconThemeData(color: label, size: 22),
      dividerColor: hairline,
      appBarTheme: AppBarThemeData(
        backgroundColor: Colors.transparent,
        foregroundColor: label,
        elevation: 0,
        scrolledUnderElevation: 0,
        shadowColor: Colors.transparent,
        surfaceTintColor: Colors.transparent,
        centerTitle: false,
        titleSpacing: 24,
        toolbarHeight: 64,
        actionsPadding: const EdgeInsets.only(right: 12),
        iconTheme: IconThemeData(color: ClickSpiritColors.systemBlue, size: 22),
        actionsIconTheme: const IconThemeData(color: ClickSpiritColors.systemBlue, size: 22),
        titleTextStyle: _style(
          size: 26,
          weight: FontWeight.w600,
          letterSpacing: -0.4,
          height: 1.1,
          color: label,
        ),
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: Colors.transparent,
        elevation: 0,
        shadowColor: Colors.transparent,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
        clipBehavior: Clip.none,
        titleTextStyle: _style(size: 20, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.25, color: label),
        contentTextStyle: _style(size: 14, weight: FontWeight.w400, letterSpacing: -0.1, height: 1.4, color: label),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: ClickSpiritColors.systemBlue,
          textStyle: _style(size: 15, weight: FontWeight.w600, letterSpacing: -0.2, height: 1.2),
          visualDensity: VisualDensity.compact,
        ),
      ),
      iconButtonTheme: IconButtonThemeData(
        style: IconButton.styleFrom(
          foregroundColor: ClickSpiritColors.systemBlue,
          visualDensity: VisualDensity.compact,
        ),
      ),
      segmentedButtonTheme: SegmentedButtonThemeData(
        style: ButtonStyle(
          visualDensity: VisualDensity.compact,
          textStyle: WidgetStatePropertyAll(
            _style(size: 13, weight: FontWeight.w600, letterSpacing: -0.2, height: 1.2),
          ),
          backgroundColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.selected)) return ClickSpiritColors.systemBlue;
            return onGlass;
          }),
          foregroundColor: WidgetStateProperty.resolveWith((states) {
            if (states.contains(WidgetState.selected)) return Colors.white;
            return label;
          }),
          side: WidgetStatePropertyAll(BorderSide(color: hairline)),
          shape: const WidgetStatePropertyAll(
            RoundedRectangleBorder(borderRadius: BorderRadius.all(Radius.circular(10))),
          ),
        ),
      ),
      textSelectionTheme: TextSelectionThemeData(
        cursorColor: ClickSpiritColors.systemBlue,
        selectionColor: ClickSpiritColors.systemBlue.withValues(alpha: 0.28),
        selectionHandleColor: ClickSpiritColors.systemBlue,
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) return Colors.white;
          return dark ? const Color(0xFFEBEBF5) : Colors.white;
        }),
        trackColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) return ClickSpiritColors.systemBlue;
          return dark ? Colors.white.withValues(alpha: 0.18) : const Color(0xFFE5E5EA);
        }),
        trackOutlineColor: const WidgetStatePropertyAll(Colors.transparent),
      ),
      progressIndicatorTheme: const ProgressIndicatorThemeData(color: ClickSpiritColors.systemBlue),
      tooltipTheme: TooltipThemeData(
        textStyle: _style(size: 12, weight: FontWeight.w500, height: 1.2, color: Colors.white),
        decoration: BoxDecoration(
          color: const Color(0xF01C1C1E),
          borderRadius: BorderRadius.circular(8),
        ),
        waitDuration: const Duration(milliseconds: 400),
      ),
    );
  }

  static const _lightScheme = ColorScheme(
    brightness: Brightness.light,
    primary: ClickSpiritColors.systemBlue,
    onPrimary: Colors.white,
    primaryContainer: Color(0xFFD6EBFF),
    onPrimaryContainer: Color(0xFF003366),
    secondary: ClickSpiritColors.systemGray,
    onSecondary: Colors.white,
    secondaryContainer: ClickSpiritColors.systemGray5Light,
    onSecondaryContainer: Color(0xFF1C1C1E),
    tertiary: ClickSpiritColors.systemGreen,
    onTertiary: Colors.white,
    tertiaryContainer: Color(0xFFD8F8E1),
    onTertiaryContainer: Color(0xFF0B3D1E),
    error: ClickSpiritColors.systemRed,
    onError: Colors.white,
    errorContainer: Color(0xFFFFE5E3),
    onErrorContainer: Color(0xFF410002),
    surface: Color(0xFFF7F8FC),
    onSurface: ClickSpiritColors.labelLight,
    onSurfaceVariant: Color(0xFF636366),
    outline: Color(0xFFD1D1D6),
    outlineVariant: ClickSpiritColors.systemGray5Light,
    shadow: Colors.black,
    scrim: Colors.black,
    inverseSurface: Color(0xFF1C1C1E),
    onInverseSurface: Colors.white,
    inversePrimary: Color(0xFF64B5FF),
    surfaceTint: Colors.transparent,
  );

  static const _darkScheme = ColorScheme(
    brightness: Brightness.dark,
    primary: ClickSpiritColors.systemBlue,
    onPrimary: Colors.white,
    primaryContainer: Color(0xFF0A3A6B),
    onPrimaryContainer: Color(0xFFD6EBFF),
    secondary: ClickSpiritColors.systemGray,
    onSecondary: Colors.white,
    secondaryContainer: Color(0xFF2C2C2E),
    onSecondaryContainer: Color(0xFFE5E5EA),
    tertiary: ClickSpiritColors.systemGreen,
    onTertiary: Color(0xFF04210E),
    tertiaryContainer: Color(0xFF0B3D1E),
    onTertiaryContainer: Color(0xFFD8F8E1),
    error: ClickSpiritColors.systemRed,
    onError: Colors.white,
    errorContainer: Color(0xFF5C100C),
    onErrorContainer: Color(0xFFFFDAD6),
    surface: Color(0xFF12141C),
    onSurface: ClickSpiritColors.labelDark,
    onSurfaceVariant: Color(0xFFAEAEB2),
    outline: Color(0xFF48484A),
    outlineVariant: Color(0xFF3A3A3C),
    shadow: Colors.black,
    scrim: Colors.black,
    inverseSurface: Color(0xFFE5E5EA),
    onInverseSurface: Color(0xFF1C1C1E),
    inversePrimary: Color(0xFF0056B3),
    surfaceTint: Colors.transparent,
  );

  static TextTheme _text(Color label, Color secondary) {
    return TextTheme(
      displayLarge: _style(size: 34, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.15, color: label),
      displayMedium: _style(size: 28, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.15, color: label),
      displaySmall: _style(size: 24, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.2, color: label),
      headlineLarge: _style(size: 28, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.2, color: label),
      headlineMedium: _style(size: 22, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.25, color: label),
      headlineSmall: _style(size: 20, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.25, color: label),
      titleLarge: _style(size: 20, weight: FontWeight.w600, letterSpacing: -0.4, height: 1.25, color: label),
      titleMedium: _style(size: 16, weight: FontWeight.w600, letterSpacing: -0.2, height: 1.3, color: label),
      titleSmall: _style(size: 14, weight: FontWeight.w600, letterSpacing: -0.2, height: 1.3, color: label),
      bodyLarge: _style(size: 16, weight: FontWeight.w400, letterSpacing: -0.2, height: 1.45, color: label),
      bodyMedium: _style(size: 14, weight: FontWeight.w400, letterSpacing: -0.1, height: 1.4, color: label),
      bodySmall: _style(size: 12, weight: FontWeight.w400, height: 1.35, color: secondary),
      labelLarge: _style(size: 14, weight: FontWeight.w600, letterSpacing: -0.1, height: 1.2, color: label),
      labelMedium: _style(size: 12, weight: FontWeight.w600, height: 1.2, color: secondary),
      labelSmall: _style(size: 11, weight: FontWeight.w500, height: 1.2, color: secondary),
    );
  }

  static TextStyle _style({
    required double size,
    required FontWeight weight,
    double letterSpacing = 0,
    double height = 1.35,
    Color? color,
  }) {
    return TextStyle(
      fontSize: size,
      fontWeight: weight,
      letterSpacing: letterSpacing,
      height: height,
      color: color,
      fontFamily: fontFallback.first,
      fontFamilyFallback: fontFallback,
    );
  }
}
