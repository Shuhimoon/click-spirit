import 'dart:math' as math;

import 'package:flutter/material.dart' hide Step;

import '../model/script.dart';
import 'capture_mapping.dart';
import 'theme.dart';

/// Where a step clicks, in screen pixels, for drawing markers.
///
/// Points and circles have a fixed place. Image steps are only known at run
/// time, so [markerOf] returns null for them.
class StepMarker {
  const StepMarker.point(this.number, this.center)
      : radius = 0,
        isCircle = false;

  const StepMarker.circle(this.number, this.center, this.radius) : isCircle = true;

  /// One-based sequence number shown on the marker.
  final int number;
  final Offset center;
  final double radius;
  final bool isCircle;
}

StepMarker? markerOf(Step step, int number) {
  final target = step.target;
  switch (step.kindOrPoint) {
    case 'circle':
      if (target == null) return null;
      return StepMarker.circle(
        number,
        Offset(target.cx.toDouble(), target.cy.toDouble()),
        target.r.toDouble(),
      );
    case 'image':
      return null;
    default:
      return StepMarker.point(number, Offset(step.x.toDouble(), step.y.toDouble()));
  }
}

/// Markers for [steps], numbered 1, 2, 3… in sequence order.
List<StepMarker> markersOf(List<Step> steps) {
  final out = <StepMarker>[];
  for (var i = 0; i < steps.length; i++) {
    final m = markerOf(steps[i], i + 1);
    if (m != null) out.add(m);
  }
  return out;
}

/// A screen-sized box that holds every marker (and [extra] points), with a
/// small margin, never smaller than 1280×720.
Size screenExtent(List<StepMarker> markers, {Iterable<Offset> extra = const []}) {
  var w = 1280.0;
  var h = 720.0;
  void grow(double x, double y) {
    w = math.max(w, x * 1.05 + 40);
    h = math.max(h, y * 1.05 + 40);
  }

  for (final m in markers) {
    grow(m.center.dx + m.radius, m.center.dy + m.radius);
  }
  for (final p in extra) {
    grow(p.dx, p.dy);
  }
  return Size(w, h);
}

const _accent = ClickSpiritColors.systemBlue;
const _active = ClickSpiritColors.systemOrange;

/// Draws one numbered marker at [c] (widget coordinates).
///
/// Points are a crosshair with a number badge to the upper right. Circles are
/// a semi-transparent disc of [radius] widget pixels with the number centered.
void paintStepMarker(
  Canvas canvas,
  StepMarker marker,
  Offset c, {
  double radius = 0,
  bool highlighted = false,
  double scale = 1,
}) {
  final color = highlighted ? _active : _accent;
  final halo = Paint()
    ..style = PaintingStyle.stroke
    ..strokeWidth = 4 * scale
    ..strokeCap = StrokeCap.round
    ..color = const Color(0xE6FFFFFF);
  final stroke = Paint()
    ..style = PaintingStyle.stroke
    ..strokeWidth = 2 * scale
    ..strokeCap = StrokeCap.round
    ..color = color;
  if (marker.isCircle) {
    final r = math.max(radius, 6 * scale);
    canvas.drawCircle(c, r, Paint()..color = color.withValues(alpha: 0.24));
    canvas.drawCircle(c, r, halo);
    canvas.drawCircle(c, r, stroke);
    _paintBadge(canvas, c, '${marker.number}', color, scale);
    return;
  }
  final arm = 12 * scale;
  final gap = 4 * scale;
  for (final p in [halo, stroke]) {
    canvas.drawLine(Offset(c.dx - arm, c.dy), Offset(c.dx - gap, c.dy), p);
    canvas.drawLine(Offset(c.dx + gap, c.dy), Offset(c.dx + arm, c.dy), p);
    canvas.drawLine(Offset(c.dx, c.dy - arm), Offset(c.dx, c.dy - gap), p);
    canvas.drawLine(Offset(c.dx, c.dy + gap), Offset(c.dx, c.dy + arm), p);
  }
  canvas.drawCircle(c, 2.2 * scale, Paint()..color = color);
  _paintBadge(canvas, c + Offset(arm + 2 * scale, -arm - 2 * scale), '${marker.number}', color, scale);
}

void _paintBadge(Canvas canvas, Offset center, String text, Color color, double scale) {
  final tp = TextPainter(
    text: TextSpan(
      text: text,
      style: TextStyle(
        color: Colors.white,
        fontSize: 11 * scale,
        fontWeight: FontWeight.w700,
        height: 1,
      ),
    ),
    textDirection: TextDirection.ltr,
  )..layout();
  final d = math.max(tp.width + 8 * scale, 18 * scale);
  final rect = Rect.fromCenter(center: center, width: d, height: 18 * scale);
  final rr = RRect.fromRectAndRadius(rect, Radius.circular(9 * scale));
  canvas.drawRRect(rr.inflate(1.5 * scale), Paint()..color = const Color(0xE6FFFFFF));
  canvas.drawRRect(rr, Paint()..color = color);
  tp.paint(canvas, center - Offset(tp.width / 2, tp.height / 2));
}

/// A small dark pill with [text] (e.g. live "x, y"), placed below [anchor]
/// and kept inside [bounds].
void paintCoordLabel(Canvas canvas, Offset anchor, String text, Rect bounds) {
  final tp = TextPainter(
    text: TextSpan(
      text: text,
      style: const TextStyle(
        color: Colors.white,
        fontSize: 12,
        fontWeight: FontWeight.w600,
        height: 1.1,
        fontFeatures: [FontFeature.tabularFigures()],
      ),
    ),
    textDirection: TextDirection.ltr,
  )..layout();
  final w = tp.width + 16;
  const h = 22.0;
  var left = anchor.dx - w / 2;
  var top = anchor.dy + 20;
  if (top + h > bounds.bottom) top = anchor.dy - 20 - h;
  left = left.clamp(bounds.left + 2, math.max(bounds.left + 2, bounds.right - w - 2));
  final rect = RRect.fromRectAndRadius(Rect.fromLTWH(left, top, w, h), const Radius.circular(11));
  canvas.drawRRect(rect, Paint()..color = const Color(0xD91C1C1E));
  tp.paint(canvas, Offset(left + 8, top + (h - tp.height) / 2));
}

/// Draws an expanding ripple at [c]. [t] runs 0 → 1 over the animation.
void paintRipple(Canvas canvas, Offset c, double t, {double maxRadius = 26}) {
  final ease = Curves.easeOutCubic.transform(t.clamp(0.0, 1.0));
  final r = 5 + (maxRadius - 5) * ease;
  final fade = 1 - t.clamp(0.0, 1.0) * 0.7;
  canvas.drawCircle(
    c,
    r,
    Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 3
      ..color = _active.withValues(alpha: fade),
  );
  canvas.drawCircle(c, 4, Paint()..color = _active);
}

/// Last click reported by the engine, for the in-app indicator.
class ClickMark {
  const ClickMark({
    required this.x,
    required this.y,
    required this.step,
    required this.repetition,
    required this.repeats,
    required this.serial,
  });

  final int x;
  final int y;
  final int step;
  final int repetition;
  final int repeats;

  /// Increases with every click so the ripple restarts.
  final int serial;

  /// e.g. "步驟 2 · 第 3/10 次".
  String get label => clickLabel(step, repetition, repeats);
}

/// Same caption the native overlay draws.
String clickLabel(int step, int repetition, int repeats) {
  if (repeats <= 1) return '步驟 ${step + 1}';
  return '步驟 ${step + 1} · 第 ${repetition + 1}/$repeats 次';
}

/// A miniature of the screen showing where every step clicks.
///
/// Fixed points are numbered crosshairs and circle regions are translucent
/// discs with their number. [highlight] is the selected or running step.
/// While a script runs, [lastClick] pulses at the clicked position, which is
/// also the fallback indicator where no native overlay exists (Wayland,
/// Windows, macOS).
class StepMapPreview extends StatefulWidget {
  const StepMapPreview({
    super.key,
    required this.markers,
    this.highlight,
    this.lastClick,
  });

  final List<StepMarker> markers;

  /// Zero-based step index drawn in the highlight color.
  final int? highlight;
  final ClickMark? lastClick;

  @override
  State<StepMapPreview> createState() => _StepMapPreviewState();
}

class _StepMapPreviewState extends State<StepMapPreview> with SingleTickerProviderStateMixin {
  late final AnimationController _pulse = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 650),
  );

  @override
  void didUpdateWidget(covariant StepMapPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    final click = widget.lastClick;
    if (click != null && click.serial != oldWidget.lastClick?.serial) {
      _pulse.forward(from: 0);
    }
  }

  @override
  void dispose() {
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final dark = ClickSpiritColors.isDark(context);
    final markers = widget.markers;
    final click = widget.lastClick;
    final extent = screenExtent(
      markers,
      extra: [if (click != null) Offset(click.x.toDouble(), click.y.toDouble())],
    );
    return Semantics(
      label: '點擊位置預覽',
      child: AnimatedBuilder(
        animation: _pulse,
        builder: (context, _) => CustomPaint(
          key: const Key('step-map'),
          painter: _MapPainter(
            markers: markers,
            extent: extent,
            highlight: widget.highlight,
            click: click,
            t: _pulse.isAnimating || _pulse.value > 0 ? _pulse.value : 1,
            dark: dark,
          ),
          child: const SizedBox.expand(),
        ),
      ),
    );
  }
}

class _MapPainter extends CustomPainter {
  _MapPainter({
    required this.markers,
    required this.extent,
    required this.highlight,
    required this.click,
    required this.t,
    required this.dark,
  });

  final List<StepMarker> markers;
  final Size extent;
  final int? highlight;
  final ClickMark? click;
  final double t;
  final bool dark;

  @override
  void paint(Canvas canvas, Size size) {
    // Leave room around the screen box so edge badges are not cut off.
    const pad = 14.0;
    final inner = Size(math.max(0, size.width - 2 * pad), math.max(0, size.height - 2 * pad));
    final base = containFit(inner, extent);
    if (base.isEmpty) return;
    final fit = ContainFit(scale: base.scale, dest: base.dest.shift(const Offset(pad, pad)));
    final screen = RRect.fromRectAndRadius(fit.dest, const Radius.circular(10));
    canvas.drawRRect(
      screen,
      Paint()..color = dark ? const Color(0x33FFFFFF) : const Color(0x14000000),
    );
    canvas.drawRRect(
      screen,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1
        ..color = dark ? const Color(0x33FFFFFF) : const Color(0x1F000000),
    );
    // Faint grid every 200 screen pixels.
    final grid = Paint()
      ..strokeWidth = 0.5
      ..color = dark ? const Color(0x14FFFFFF) : const Color(0x0F000000);
    for (var x = 200.0; x < extent.width; x += 200) {
      final p = imageToWidget(Offset(x, 0), fit).dx;
      canvas.drawLine(Offset(p, fit.dest.top), Offset(p, fit.dest.bottom), grid);
    }
    for (var y = 200.0; y < extent.height; y += 200) {
      final p = imageToWidget(Offset(0, y), fit).dy;
      canvas.drawLine(Offset(fit.dest.left, p), Offset(fit.dest.right, p), grid);
    }
    for (final m in markers) {
      paintStepMarker(
        canvas,
        m,
        imageToWidget(m.center, fit),
        radius: m.radius * fit.scale,
        highlighted: highlight != null && highlight == m.number - 1,
        scale: 0.9,
      );
    }
    final c = click;
    if (c != null) {
      final at = imageToWidget(Offset(c.x.toDouble(), c.y.toDouble()), fit);
      paintRipple(canvas, at, t);
      paintCoordLabel(canvas, at, '${c.label} · ${c.x}, ${c.y}', Offset.zero & size);
    }
  }

  @override
  bool shouldRepaint(covariant _MapPainter old) =>
      old.markers.length != markers.length ||
      old.extent != extent ||
      old.highlight != highlight ||
      old.click?.serial != click?.serial ||
      old.t != t ||
      old.dark != dark ||
      !_sameMarkers(old.markers, markers);

  static bool _sameMarkers(List<StepMarker> a, List<StepMarker> b) {
    for (var i = 0; i < a.length; i++) {
      if (a[i].center != b[i].center || a[i].radius != b[i].radius || a[i].number != b[i].number) return false;
    }
    return true;
  }
}
