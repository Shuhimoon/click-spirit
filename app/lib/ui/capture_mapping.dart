import 'dart:ui';

/// Where a screenshot is drawn inside a widget with [BoxFit.contain].
///
/// [scale] is widget pixels per image pixel. [dest] is the image rectangle
/// inside the widget. Empty bands around [dest] are letterboxing.
class ContainFit {
  const ContainFit({required this.scale, required this.dest});

  final double scale;
  final Rect dest;

  bool get isEmpty => scale == 0 || dest.isEmpty;
}

/// Uniform scale that fits [imageSize] inside [widgetSize], centered.
///
/// A wider image leaves bars above and below. A taller image leaves bars on
/// the sides. Either size being empty yields a zero fit.
ContainFit containFit(Size widgetSize, Size imageSize) {
  if (widgetSize.width <= 0 || widgetSize.height <= 0 || imageSize.width <= 0 || imageSize.height <= 0) {
    return const ContainFit(scale: 0, dest: Rect.zero);
  }
  final sx = widgetSize.width / imageSize.width;
  final sy = widgetSize.height / imageSize.height;
  final scale = sx < sy ? sx : sy;
  final width = imageSize.width * scale;
  final height = imageSize.height * scale;
  final left = (widgetSize.width - width) / 2;
  final top = (widgetSize.height - height) / 2;
  return ContainFit(scale: scale, dest: Rect.fromLTWH(left, top, width, height));
}

/// Maps a point in the widget to continuous image coordinates.
///
/// Returns null when [local] is in the letterbox, outside the fitted image.
Offset? widgetToImage(Offset local, Size widgetSize, Size imageSize) {
  final fit = containFit(widgetSize, imageSize);
  if (fit.scale == 0) return null;
  final dx = local.dx - fit.dest.left;
  final dy = local.dy - fit.dest.top;
  const eps = 1e-9;
  if (dx < -eps || dy < -eps || dx > fit.dest.width + eps || dy > fit.dest.height + eps) {
    return null;
  }
  return Offset(dx / fit.scale, dy / fit.scale);
}

/// Like [widgetToImage], but a letterbox point snaps to the nearest image edge.
Offset widgetToImageClamped(Offset local, Size widgetSize, Size imageSize) {
  final fit = containFit(widgetSize, imageSize);
  if (fit.scale == 0) return Offset.zero;
  final dx = (local.dx - fit.dest.left).clamp(0.0, fit.dest.width);
  final dy = (local.dy - fit.dest.top).clamp(0.0, fit.dest.height);
  return Offset(dx / fit.scale, dy / fit.scale);
}

/// Maps an image point back into the widget using [fit].
Offset imageToWidget(Offset imagePoint, ContainFit fit) {
  return Offset(
    fit.dest.left + imagePoint.dx * fit.scale,
    fit.dest.top + imagePoint.dy * fit.scale,
  );
}

/// Rounds [value] to an image pixel in `0 .. size-1`.
int imagePixel(double value, int size) {
  if (size <= 0) return 0;
  final rounded = value.round();
  if (rounded < 0) return 0;
  if (rounded >= size) return size - 1;
  return rounded;
}

/// Press point in the capture view, as an image pixel.
(int, int) pointFromLocal(Offset local, Size widgetSize, Size imageSize) {
  final p = widgetToImageClamped(local, widgetSize, imageSize);
  return (imagePixel(p.dx, imageSize.width.round()), imagePixel(p.dy, imageSize.height.round()));
}

class CirclePixels {
  const CirclePixels(this.cx, this.cy, this.r);
  final int cx;
  final int cy;
  final int r;
}

/// Press is the center. The drag distance, in image pixels, is the radius.
CirclePixels circleFromDrag({
  required Offset press,
  required Offset current,
  required Size widgetSize,
  required Size imageSize,
}) {
  final center = widgetToImageClamped(press, widgetSize, imageSize);
  final edge = widgetToImageClamped(current, widgetSize, imageSize);
  final radius = (edge - center).distance.round();
  return CirclePixels(
    imagePixel(center.dx, imageSize.width.round()),
    imagePixel(center.dy, imageSize.height.round()),
    radius < 0 ? 0 : radius,
  );
}

class RectPixels {
  const RectPixels(this.x, this.y, this.w, this.h);
  final int x;
  final int y;
  final int w;
  final int h;
}

/// Axis-aligned rectangle from a press-drag, in image pixels.
///
/// Width and height are at least 1 when the image is non-empty.
RectPixels rectFromDrag({
  required Offset press,
  required Offset current,
  required Size widgetSize,
  required Size imageSize,
}) {
  final a = widgetToImageClamped(press, widgetSize, imageSize);
  final b = widgetToImageClamped(current, widgetSize, imageSize);
  final width = imageSize.width.round();
  final height = imageSize.height.round();
  if (width <= 0 || height <= 0) return const RectPixels(0, 0, 0, 0);
  final left = a.dx < b.dx ? a.dx : b.dx;
  final top = a.dy < b.dy ? a.dy : b.dy;
  final right = a.dx > b.dx ? a.dx : b.dx;
  final bottom = a.dy > b.dy ? a.dy : b.dy;
  var x = left.floor();
  var y = top.floor();
  var r = right.ceil();
  var btm = bottom.ceil();
  if (x < 0) x = 0;
  if (y < 0) y = 0;
  if (x >= width) x = width - 1;
  if (y >= height) y = height - 1;
  if (r <= x) r = x + 1;
  if (btm <= y) btm = y + 1;
  if (r > width) r = width;
  if (btm > height) btm = height;
  return RectPixels(x, y, r - x, btm - y);
}
