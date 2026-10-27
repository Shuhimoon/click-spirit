import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:path_provider/path_provider.dart';

/// Crops [png] to the image-pixel rectangle and returns a new PNG.
Future<Uint8List> cropPng(
  Uint8List png, {
  required int x,
  required int y,
  required int w,
  required int h,
}) async {
  final codec = await ui.instantiateImageCodec(png);
  try {
    final frame = await codec.getNextFrame();
    final src = frame.image;
    try {
      return await cropImage(src, x: x, y: y, w: w, h: h);
    } finally {
      src.dispose();
    }
  } finally {
    codec.dispose();
  }
}

/// Draws the source rectangle into a new PNG via [ui.PictureRecorder].
Future<Uint8List> cropImage(
  ui.Image src, {
  required int x,
  required int y,
  required int w,
  required int h,
}) async {
  if (src.width <= 0 || src.height <= 0) {
    throw StateError('截圖是空的');
  }
  var left = x;
  var top = y;
  var width = w;
  var height = h;
  if (left < 0) left = 0;
  if (top < 0) top = 0;
  if (left >= src.width) left = src.width - 1;
  if (top >= src.height) top = src.height - 1;
  if (left + width > src.width) width = src.width - left;
  if (top + height > src.height) height = src.height - top;
  if (width < 1 || height < 1) {
    throw StateError('裁切範圍是空的');
  }
  final recorder = ui.PictureRecorder();
  final canvas = ui.Canvas(recorder);
  canvas.drawImageRect(
    src,
    ui.Rect.fromLTWH(left.toDouble(), top.toDouble(), width.toDouble(), height.toDouble()),
    ui.Rect.fromLTWH(0, 0, width.toDouble(), height.toDouble()),
    ui.Paint(),
  );
  final picture = recorder.endRecording();
  try {
    final out = await picture.toImage(width, height);
    try {
      final data = await out.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('無法編碼 PNG');
      return data.buffer.asUint8List();
    } finally {
      out.dispose();
    }
  } finally {
    picture.dispose();
  }
}

/// Writes [png] under the application support directory and returns the path.
Future<String> saveTemplatePng(Uint8List png) async {
  final root = await getApplicationSupportDirectory();
  final folder = Directory('${root.path}/templates');
  if (!await folder.exists()) {
    await folder.create(recursive: true);
  }
  final file = File('${folder.path}/template_${DateTime.now().millisecondsSinceEpoch}.png');
  await file.writeAsBytes(png, flush: true);
  return file.path;
}
