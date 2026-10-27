import 'dart:ui';

import 'package:flutter/material.dart';

import 'theme.dart';

/// Soft gradient wash with a few large blurred color blobs.
class GlassBackground extends StatelessWidget {
  const GlassBackground({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final dark = ClickSpiritColors.isDark(context);
    return Stack(
      fit: StackFit.expand,
      children: [
        RepaintBoundary(child: _Wash(dark: dark)),
        child,
      ],
    );
  }
}

class _Wash extends StatelessWidget {
  const _Wash({required this.dark});

  final bool dark;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: dark
                  ? const [Color(0xFF070B16), Color(0xFF161433), Color(0xFF0B1428)]
                  : const [Color(0xFFE4EFFF), Color(0xFFF4ECFF), Color(0xFFFFF1E8)],
              stops: const [0, 0.52, 1],
            ),
          ),
        ),
        _blob(
          dark ? const Color(0xFF1D4ED8) : const Color(0xFF8EBEFF),
          420,
          const Alignment(-1.15, -0.95),
          dark ? 0.45 : 0.70,
        ),
        _blob(
          dark ? const Color(0xFF6D28D9) : const Color(0xFFD4C2FF),
          380,
          const Alignment(1.2, -0.55),
          dark ? 0.38 : 0.75,
        ),
        _blob(
          dark ? const Color(0xFF0F766E) : const Color(0xFFFFC7A8),
          340,
          const Alignment(0.35, 1.25),
          dark ? 0.28 : 0.72,
        ),
      ],
    );
  }

  Widget _blob(Color color, double size, Alignment alignment, double alpha) {
    return Align(
      alignment: alignment,
      child: ImageFiltered(
        imageFilter: ImageFilter.blur(sigmaX: 72, sigmaY: 72),
        child: Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: color.withValues(alpha: alpha),
          ),
        ),
      ),
    );
  }
}

/// Frosted panel: blur, translucent fill, hairline border, soft shadow.
class GlassPanel extends StatelessWidget {
  const GlassPanel({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.radius = 20,
    this.sigma = 24,
    this.tint,
    this.shadow = true,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;
  final double radius;
  final double sigma;
  final Color? tint;
  final bool shadow;

  @override
  Widget build(BuildContext context) {
    final dark = ClickSpiritColors.isDark(context);
    final r = BorderRadius.circular(radius);
    final border = dark ? Colors.white.withValues(alpha: 0.12) : Colors.white.withValues(alpha: 0.60);
    var fill = dark ? const Color(0x59000000) : Colors.white.withValues(alpha: 0.60);
    final overlay = tint;
    if (overlay != null) fill = Color.alphaBlend(overlay, fill);

    // Transparent Material keeps InkWell working when a row is lifted into the
    // reorder overlay, which sits outside the scaffold Material.
    Widget body = ColoredBox(
      color: fill,
      child: Material(
        type: MaterialType.transparency,
        child: Padding(padding: padding, child: child),
      ),
    );
    if (sigma > 0) {
      body = BackdropFilter(
        filter: ImageFilter.blur(sigmaX: sigma, sigmaY: sigma),
        child: body,
      );
    }

    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: r,
        boxShadow: shadow
            ? [
                BoxShadow(
                  color: Colors.black.withValues(alpha: dark ? 0.32 : 0.07),
                  blurRadius: 28,
                  offset: const Offset(0, 12),
                ),
                BoxShadow(
                  color: Colors.black.withValues(alpha: dark ? 0.18 : 0.04),
                  blurRadius: 6,
                  offset: const Offset(0, 1),
                ),
              ]
            : const [],
      ),
      child: Container(
        foregroundDecoration: BoxDecoration(
          borderRadius: r,
          border: Border.all(color: border),
        ),
        child: ClipRRect(borderRadius: r, child: body),
      ),
    );
  }
}

enum GlassButtonVariant { primary, filled, destructive, tinted }

/// Pill button, about 36px tall, without a heavy drop shadow.
class GlassButton extends StatelessWidget {
  const GlassButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.icon,
    this.variant = GlassButtonVariant.tinted,
    this.color,
  });

  final String label;
  final VoidCallback? onPressed;
  final IconData? icon;
  final GlassButtonVariant variant;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null;
    final accent = color ?? _defaultColor(variant);
    final dark = ClickSpiritColors.isDark(context);
    final Color background;
    final Color foreground;
    switch (variant) {
      case GlassButtonVariant.primary:
      case GlassButtonVariant.destructive:
        background = accent;
        foreground = Colors.white;
      case GlassButtonVariant.filled:
        background = accent;
        // systemGreen is light; near-black text stays readable on the fill.
        foreground = const Color(0xFF062814);
      case GlassButtonVariant.tinted:
        background = accent.withValues(alpha: dark ? 0.22 : 0.12);
        foreground = accent;
    }
    final faded = enabled ? 1.0 : 0.38;

    return Opacity(
      opacity: faded,
      child: Material(
        color: background,
        shape: const RoundedRectangleBorder(borderRadius: BorderRadius.all(Radius.circular(18))),
        clipBehavior: Clip.antiAlias,
        elevation: 0,
        child: InkWell(
          onTap: onPressed,
          customBorder: const RoundedRectangleBorder(borderRadius: BorderRadius.all(Radius.circular(18))),
          child: DecoratedBox(
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(18),
              border: Border.all(
                color: variant == GlassButtonVariant.tinted
                    ? accent.withValues(alpha: 0.16)
                    : Colors.white.withValues(alpha: 0.28),
              ),
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 14),
              child: SizedBox(
                height: 36,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (icon != null) ...[
                      Icon(icon, size: 18, color: foreground),
                      const SizedBox(width: 6),
                    ],
                    Text(
                      label,
                      style: TextStyle(
                        color: foreground,
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        letterSpacing: -0.2,
                        height: 1.1,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  static Color _defaultColor(GlassButtonVariant variant) {
    switch (variant) {
      case GlassButtonVariant.primary:
        return ClickSpiritColors.systemBlue;
      case GlassButtonVariant.filled:
        return ClickSpiritColors.systemGreen;
      case GlassButtonVariant.destructive:
        return ClickSpiritColors.systemRed;
      case GlassButtonVariant.tinted:
        return ClickSpiritColors.systemBlue;
    }
  }
}

class GlassFieldLabel extends StatelessWidget {
  const GlassFieldLabel(this.text, {super.key});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 12,
          height: 1.2,
          fontWeight: FontWeight.w500,
          color: ClickSpiritColors.secondary(context),
        ),
      ),
    );
  }
}

class GlassTextField extends StatelessWidget {
  const GlassTextField({
    super.key,
    this.fieldKey,
    required this.controller,
    required this.label,
    this.keyboardType,
    this.onChanged,
  });

  final Key? fieldKey;
  final TextEditingController controller;
  final String label;
  final TextInputType? keyboardType;
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        GlassFieldLabel(label),
        TextField(
          key: fieldKey,
          controller: controller,
          keyboardType: keyboardType,
          onChanged: onChanged,
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w400,
            height: 1.2,
            color: ClickSpiritColors.label(context),
          ),
          cursorColor: ClickSpiritColors.systemBlue,
          decoration: ClickSpiritTheme.fieldDecoration(context),
        ),
      ],
    );
  }
}

class GlassDropdownField<T> extends StatelessWidget {
  const GlassDropdownField({
    super.key,
    required this.label,
    required this.value,
    required this.items,
    required this.onChanged,
  });

  final String label;
  final T value;
  final List<DropdownMenuItem<T>> items;
  final ValueChanged<T?> onChanged;

  @override
  Widget build(BuildContext context) {
    final dark = ClickSpiritColors.isDark(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        GlassFieldLabel(label),
        DropdownButtonFormField<T>(
          initialValue: value,
          isExpanded: true,
          borderRadius: BorderRadius.circular(12),
          dropdownColor: dark ? const Color(0xFF2C2C2E) : Colors.white,
          icon: Icon(Icons.keyboard_arrow_down_rounded, color: ClickSpiritColors.secondary(context)),
          decoration: ClickSpiritTheme.fieldDecoration(context),
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w400,
            color: ClickSpiritColors.label(context),
            fontFamily: ClickSpiritTheme.fontFallback.first,
            fontFamilyFallback: ClickSpiritTheme.fontFallback,
          ),
          items: items,
          onChanged: onChanged,
        ),
      ],
    );
  }
}
