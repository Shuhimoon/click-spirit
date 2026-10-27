import 'package:click_spirit/ui/capture_mapping.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('letterbox maps widget points onto the fitted image', () {
    const widgetSize = Size(200, 100);
    const imageSize = Size(100, 100);
    final fit = containFit(widgetSize, imageSize);
    expect(fit.scale, 1);
    expect(fit.dest, const Rect.fromLTWH(50, 0, 100, 100));

    expect(widgetToImage(const Offset(100, 50), widgetSize, imageSize), const Offset(50, 50));
    expect(widgetToImage(const Offset(50, 0), widgetSize, imageSize), const Offset(0, 0));
    expect(widgetToImage(const Offset(10, 50), widgetSize, imageSize), isNull);
    expect(widgetToImage(const Offset(160, 50), widgetSize, imageSize), isNull);

    final clamped = widgetToImageClamped(const Offset(10, 50), widgetSize, imageSize);
    expect(clamped, const Offset(0, 50));
    expect(pointFromLocal(const Offset(100, 50), widgetSize, imageSize), (50, 50));
  });

  test('taller widget letterboxes above and below', () {
    const widgetSize = Size(100, 200);
    const imageSize = Size(100, 50);
    final fit = containFit(widgetSize, imageSize);
    expect(fit.scale, 1);
    expect(fit.dest, const Rect.fromLTWH(0, 75, 100, 50));

    expect(widgetToImage(const Offset(0, 75), widgetSize, imageSize), const Offset(0, 0));
    expect(widgetToImage(const Offset(50, 100), widgetSize, imageSize), const Offset(50, 25));
    expect(widgetToImage(const Offset(50, 10), widgetSize, imageSize), isNull);
    expect(widgetToImage(const Offset(50, 160), widgetSize, imageSize), isNull);
  });

  test('wide screenshot on a shorter window letterboxes vertically', () {
    const widgetSize = Size(1280, 800);
    const imageSize = Size(1920, 1080);
    final fit = containFit(widgetSize, imageSize);
    expect(fit.dest.left, 0);
    expect(fit.dest.width, closeTo(1280, 0.001));
    expect(fit.dest.height, closeTo(720, 0.001));
    expect(fit.dest.top, closeTo(40, 0.001));

    final center = widgetToImage(const Offset(640, 400), widgetSize, imageSize);
    expect(center, isNotNull);
    expect(center!.dx, closeTo(960, 0.01));
    expect(center.dy, closeTo(540, 0.01));
    expect(widgetToImage(const Offset(640, 10), widgetSize, imageSize), isNull);

    final back = imageToWidget(const Offset(960, 540), fit);
    expect(back.dx, closeTo(640, 0.01));
    expect(back.dy, closeTo(400, 0.01));
  });

  test('circle drag uses image pixels, not widget pixels', () {
    const widgetSize = Size(200, 100);
    const imageSize = Size(100, 50);
    final circle = circleFromDrag(
      press: const Offset(40, 20),
      current: const Offset(46, 28),
      widgetSize: widgetSize,
      imageSize: imageSize,
    );
    expect(circle.cx, 20);
    expect(circle.cy, 10);
    expect(circle.r, 5);
  });

  test('empty sizes do not map', () {
    expect(containFit(Size.zero, const Size(10, 10)).scale, 0);
    expect(widgetToImage(Offset.zero, Size.zero, const Size(10, 10)), isNull);
    expect(imagePixel(-3, 100), 0);
    expect(imagePixel(99.6, 100), 99);
    expect(imagePixel(1000, 100), 99);
  });
}