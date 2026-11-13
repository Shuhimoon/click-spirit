import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';

import 'capture_mapping.dart';
import 'glass.dart';
import 'markers.dart';
import 'theme.dart';

enum CapturePickMode { point, circle, rect }

/// Pixel result from the full-screen capture view.
class CaptureSelection {
  const CaptureSelection.point(this.x, this.y)
      : cx = 0,
        cy = 0,
        r = 0,
        rectX = 0,
        rectY = 0,
        rectW = 0,
        rectH = 0,
        mode = CapturePickMode.point;

  const CaptureSelection.circle(this.cx, this.cy, this.r)
      : x = 0,
        y = 0,
        rectX = 0,
        rectY = 0,
        rectW = 0,
        rectH = 0,
        mode = CapturePickMode.circle;

  const CaptureSelection.rect(this.rectX, this.rectY, this.rectW, this.rectH)
      : x = 0,
        y = 0,
        cx = 0,
        cy = 0,
        r = 0,
        mode = CapturePickMode.rect;

  final CapturePickMode mode;
  final int x;
  final int y;
  final int cx;
  final int cy;
  final int r;
  final int rectX;
  final int rectY;
  final int rectW;
  final int rectH;
}

Future<CaptureSelection?> showCaptureScreen(
  BuildContext context,
  Uint8List pngBytes,
  CapturePickMode mode, {
  int number = 1,
  List<StepMarker> others = const [],
  CaptureSelection? initial,
}) {
  return Navigator.of(context).push<CaptureSelection>(
    MaterialPageRoute(
      fullscreenDialog: true,
      builder: (context) => CaptureScreenPage(
        pngBytes: pngBytes,
        mode: mode,
        number: number,
        others: others,
        initial: initial,
      ),
    ),
  );
}

class CaptureScreenPage extends StatefulWidget {
  const CaptureScreenPage({
    super.key,
    required this.pngBytes,
    required this.mode,
    this.number = 1,
    this.others = const [],
    this.initial,
  });

  final Uint8List pngBytes;
  final CapturePickMode mode;

  /// One-based number of the step being edited, drawn on its marker.
  final int number;

  /// The script's other steps, drawn dimmed for reference.
  final List<StepMarker> others;

  /// Current value of the step, so its marker shows before the first tap.
  final CaptureSelection? initial;

  @override
  State<CaptureScreenPage> createState() => _CaptureScreenPageState();
}

class _CaptureScreenPageState extends State<CaptureScreenPage> {
  final _x = TextEditingController(text: '0');
  final _y = TextEditingController(text: '0');
  final _cx = TextEditingController(text: '0');
  final _cy = TextEditingController(text: '0');
  final _r = TextEditingController(text: '0');
  final _rx = TextEditingController(text: '0');
  final _ry = TextEditingController(text: '0');
  final _rw = TextEditingController(text: '0');
  final _rh = TextEditingController(text: '0');

  ui.Image? _image;
  Size? _imageSize;
  Size? _viewport;
  String? _error;
  String? _formError;
  bool _pan = false;
  Offset? _press;

  @override
  void initState() {
    super.initState();
    final init = widget.initial;
    if (init != null) {
      _x.text = '${init.x}';
      _y.text = '${init.y}';
      _cx.text = '${init.cx}';
      _cy.text = '${init.cy}';
      _r.text = '${init.r}';
    }
    _decode();
  }

  Future<void> _decode() async {
    try {
      final codec = await ui.instantiateImageCodec(widget.pngBytes);
      final frame = await codec.getNextFrame();
      codec.dispose();
      if (!mounted) {
        frame.image.dispose();
        return;
      }
      setState(() {
        _image = frame.image;
        _imageSize = Size(frame.image.width.toDouble(), frame.image.height.toDouble());
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = '無法解碼截圖：$e');
    }
  }

  @override
  void dispose() {
    _image?.dispose();
    _x.dispose();
    _y.dispose();
    _cx.dispose();
    _cy.dispose();
    _r.dispose();
    _rx.dispose();
    _ry.dispose();
    _rw.dispose();
    _rh.dispose();
    super.dispose();
  }

  String get _title {
    switch (widget.mode) {
      case CapturePickMode.point:
        return '點選座標';
      case CapturePickMode.circle:
        return '設定圓形';
      case CapturePickMode.rect:
        return '裁切模板';
    }
  }

  String get _hint {
    switch (widget.mode) {
      case CapturePickMode.point:
        return '點一下畫面選擇座標。下方數字可再修改。';
      case CapturePickMode.circle:
        return '按住起點是圓心，拖曳距離是半徑。下方數字可再修改。';
      case CapturePickMode.rect:
        return '按住並拖曳選取矩形。下方數字可再修改。';
    }
  }

  void _drag(Offset local, {Offset? from}) {
    final viewport = _viewport;
    final imageSize = _imageSize;
    if (viewport == null || imageSize == null || viewport.width <= 0 || viewport.height <= 0) return;
    final origin = from ?? local;
    switch (widget.mode) {
      case CapturePickMode.point:
        final picked = pointFromLocal(local, viewport, imageSize);
        _x.text = '${picked.$1}';
        _y.text = '${picked.$2}';
      case CapturePickMode.circle:
        final circle = circleFromDrag(
          press: origin,
          current: local,
          widgetSize: viewport,
          imageSize: imageSize,
        );
        _cx.text = '${circle.cx}';
        _cy.text = '${circle.cy}';
        _r.text = '${circle.r}';
      case CapturePickMode.rect:
        final rect = rectFromDrag(
          press: origin,
          current: local,
          widgetSize: viewport,
          imageSize: imageSize,
        );
        _rx.text = '${rect.x}';
        _ry.text = '${rect.y}';
        _rw.text = '${rect.w}';
        _rh.text = '${rect.h}';
    }
    setState(() => _formError = null);
  }

  void _confirm() {
    switch (widget.mode) {
      case CapturePickMode.point:
        final x = int.tryParse(_x.text.trim());
        final y = int.tryParse(_y.text.trim());
        if (x == null || y == null || x < 0 || y < 0) {
          setState(() => _formError = 'X、Y 必須是大於等於 0 的整數');
          return;
        }
        Navigator.of(context).pop(CaptureSelection.point(x, y));
      case CapturePickMode.circle:
        final cx = int.tryParse(_cx.text.trim());
        final cy = int.tryParse(_cy.text.trim());
        final r = int.tryParse(_r.text.trim());
        if (cx == null || cy == null || r == null || cx < 0 || cy < 0 || r < 0) {
          setState(() => _formError = '圓心與半徑必須是大於等於 0 的整數');
          return;
        }
        Navigator.of(context).pop(CaptureSelection.circle(cx, cy, r));
      case CapturePickMode.rect:
        final x = int.tryParse(_rx.text.trim());
        final y = int.tryParse(_ry.text.trim());
        final w = int.tryParse(_rw.text.trim());
        final h = int.tryParse(_rh.text.trim());
        if (x == null || y == null || w == null || h == null || x < 0 || y < 0 || w < 1 || h < 1) {
          setState(() => _formError = '請拖曳矩形，或輸入大於 0 的寬高');
          return;
        }
        Navigator.of(context).pop(CaptureSelection.rect(x, y, w, h));
    }
  }

  @override
  Widget build(BuildContext context) {
    final image = _image;
    final imageSize = _imageSize;
    final secondary = ClickSpiritColors.secondary(context);
    return Scaffold(
      backgroundColor: const Color(0xFF0B0D12),
      body: SafeArea(
        child: Stack(
          children: [
            Positioned.fill(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final viewport = Size(constraints.maxWidth, constraints.maxHeight);
                  _viewport = viewport;
                  if (image == null || imageSize == null) {
                    return const Center(child: CircularProgressIndicator());
                  }
                  final fit = containFit(viewport, imageSize);
                  return InteractiveViewer(
                    constrained: true,
                    panEnabled: _pan,
                    scaleEnabled: true,
                    minScale: 1,
                    maxScale: 8,
                    child: Listener(
                      behavior: HitTestBehavior.opaque,
                      onPointerDown: _pan
                          ? null
                          : (event) {
                              _press = event.localPosition;
                              _drag(event.localPosition);
                            },
                      onPointerMove: _pan
                          ? null
                          : (event) {
                              final origin = _press;
                              if (origin == null) return;
                              _drag(event.localPosition, from: origin);
                            },
                      onPointerUp: _pan
                          ? null
                          : (event) {
                              final origin = _press;
                              _press = null;
                              if (origin == null) return;
                              _drag(event.localPosition, from: origin);
                            },
                      child: CustomPaint(
                        painter: _CapturePainter(
                          image: image,
                          fit: fit,
                          number: widget.number,
                          others: widget.others,
                          point: widget.mode == CapturePickMode.point ? _offset(_x, _y) : null,
                          circleCenter: widget.mode == CapturePickMode.circle ? _offset(_cx, _cy) : null,
                          circleRadius: widget.mode == CapturePickMode.circle ? double.tryParse(_r.text.trim()) : null,
                          rect: widget.mode == CapturePickMode.rect ? _rect() : null,
                        ),
                        child: const SizedBox.expand(),
                      ),
                    ),
                  );
                },
              ),
            ),
            Positioned(
              top: 12,
              left: 16,
              right: 16,
              child: GlassPanel(
                radius: 18,
                padding: const EdgeInsets.fromLTRB(4, 4, 12, 4),
                child: Row(
                  children: [
                    IconButton(
                      tooltip: '關閉',
                      onPressed: () => Navigator.of(context).pop(),
                      icon: const Icon(Icons.close_rounded),
                    ),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            _title,
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w600,
                              letterSpacing: -0.3,
                              height: 1.2,
                              color: ClickSpiritColors.label(context),
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(_hint, style: TextStyle(fontSize: 12, height: 1.35, color: secondary)),
                          if (_error != null)
                            Text(_error!, style: const TextStyle(fontSize: 12, height: 1.35, color: ClickSpiritColors.systemRed)),
                        ],
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text('平移', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: secondary)),
                    Switch(
                      value: _pan,
                      onChanged: (value) => setState(() => _pan = value),
                    ),
                  ],
                ),
              ),
            ),
            Positioned(
              left: 16,
              right: 16,
              bottom: 12,
              child: GlassPanel(
                radius: 20,
                padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _fields(),
                    if (_formError != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 6),
                        child: Text(
                          _formError!,
                          style: const TextStyle(color: ClickSpiritColors.systemRed, fontSize: 13),
                        ),
                      ),
                    const SizedBox(height: 10),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('取消')),
                        const SizedBox(width: 8),
                        GlassButton(
                          onPressed: image == null ? null : _confirm,
                          label: '確定',
                          variant: GlassButtonVariant.primary,
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Offset? _offset(TextEditingController a, TextEditingController b) {
    final x = double.tryParse(a.text.trim());
    final y = double.tryParse(b.text.trim());
    if (x == null || y == null) return null;
    return Offset(x, y);
  }

  Rect? _rect() {
    final x = double.tryParse(_rx.text.trim());
    final y = double.tryParse(_ry.text.trim());
    final w = double.tryParse(_rw.text.trim());
    final h = double.tryParse(_rh.text.trim());
    if (x == null || y == null || w == null || h == null || w <= 0 || h <= 0) return null;
    return Rect.fromLTWH(x, y, w, h);
  }

  Widget _fields() {
    switch (widget.mode) {
      case CapturePickMode.point:
        return Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _box(_x, 'X'),
            _box(_y, 'Y'),
          ],
        );
      case CapturePickMode.circle:
        return Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _box(_cx, '圓心 X'),
            _box(_cy, '圓心 Y'),
            _box(_r, '半徑'),
          ],
        );
      case CapturePickMode.rect:
        return Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _box(_rx, 'X'),
            _box(_ry, 'Y'),
            _box(_rw, '寬'),
            _box(_rh, '高'),
          ],
        );
    }
  }

  Widget _box(TextEditingController controller, String label) {
    return SizedBox(
      width: 120,
      child: GlassTextField(
        controller: controller,
        label: label,
        keyboardType: TextInputType.number,
        onChanged: (_) => setState(() => _formError = null),
      ),
    );
  }
}

class _CapturePainter extends CustomPainter {
  _CapturePainter({
    required this.image,
    required this.fit,
    this.number = 1,
    this.others = const [],
    this.point,
    this.circleCenter,
    this.circleRadius,
    this.rect,
  });

  final ui.Image image;
  final ContainFit fit;
  final int number;
  final List<StepMarker> others;
  final Offset? point;
  final Offset? circleCenter;
  final double? circleRadius;
  final Rect? rect;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawRect(Offset.zero & size, Paint()..color = const Color(0xFF0B0D12));
    if (fit.scale == 0) return;
    canvas.drawImageRect(
      image,
      Rect.fromLTWH(0, 0, image.width.toDouble(), image.height.toDouble()),
      fit.dest,
      Paint()..filterQuality = FilterQuality.medium,
    );
    const accent = ClickSpiritColors.systemBlue;
    final stroke = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2
      ..strokeCap = StrokeCap.round
      ..color = accent;
    final halo = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 4
      ..strokeCap = StrokeCap.round
      ..color = const Color(0xE6FFFFFF);
    final fill = Paint()
      ..style = PaintingStyle.fill
      ..color = accent.withValues(alpha: 0.22);
    final scrim = Paint()..color = const Color(0x66000000);

    // The script's other steps, dimmed, so the new point can be placed
    // relative to them.
    if (others.isNotEmpty) {
      canvas.saveLayer(Offset.zero & size, Paint()..color = const Color(0x99FFFFFF));
      for (final m in others) {
        paintStepMarker(canvas, m, imageToWidget(m.center, fit), radius: m.radius * fit.scale);
      }
      canvas.restore();
    }

    final center = circleCenter;
    if (center != null) {
      final r = circleRadius ?? 0;
      final c = imageToWidget(center, fit);
      paintStepMarker(
        canvas,
        StepMarker.circle(number, center, r),
        c,
        radius: r > 0 ? r * fit.scale : 0,
      );
      paintCoordLabel(
        canvas,
        c,
        '圓心 ${center.dx.round()}, ${center.dy.round()} · r=${r.round()}',
        Offset.zero & size,
      );
    }
    final box = rect;
    if (box != null) {
      final a = imageToWidget(box.topLeft, fit);
      final b = imageToWidget(box.bottomRight, fit);
      final drawn = Rect.fromPoints(a, b);
      final path = Path()
        ..fillType = PathFillType.evenOdd
        ..addRect(Offset.zero & size)
        ..addRect(drawn);
      canvas.drawPath(path, scrim);
      canvas.drawRect(drawn, fill);
      canvas.drawRect(drawn, halo);
      canvas.drawRect(drawn, stroke);
    }
    final mark = point;
    if (mark != null) {
      final c = imageToWidget(mark, fit);
      paintStepMarker(canvas, StepMarker.point(number, mark), c, scale: 1.3);
      paintCoordLabel(canvas, c, '${mark.dx.round()}, ${mark.dy.round()}', Offset.zero & size);
    }
  }

  @override
  bool shouldRepaint(covariant _CapturePainter oldDelegate) => true;
}
