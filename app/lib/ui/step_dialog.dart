import 'dart:io';
import 'dart:math' as math;
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:path_provider/path_provider.dart';

import '../engine/click_engine.dart';
import '../model/script.dart';
import 'capture_dialog.dart';
import 'glass.dart';
import 'markers.dart';
import 'png_crop.dart';
import 'theme.dart';

Future<Step?> showStepEditor(
  BuildContext context, {
  Step? initial,
  ClickEngine? engine,
  int number = 1,
  List<StepMarker> others = const [],
}) {
  return showDialog<Step>(
    context: context,
    barrierColor: Colors.black.withValues(alpha: 0.28),
    builder: (context) => StepEditorDialog(initial: initial, engine: engine, number: number, others: others),
  );
}

class StepEditorDialog extends StatefulWidget {
  const StepEditorDialog({super.key, this.initial, this.engine, this.number = 1, this.others = const []});

  final Step? initial;
  final ClickEngine? engine;

  /// One-based position of this step in the script.
  final int number;

  /// Markers of the script's other steps, shown dimmed on the capture screen.
  final List<StepMarker> others;

  @override
  State<StepEditorDialog> createState() => _StepEditorDialogState();
}

class _StepEditorDialogState extends State<StepEditorDialog> {
  late final TextEditingController _x;
  late final TextEditingController _y;
  late final TextEditingController _interval;
  late final TextEditingController _hold;
  late final TextEditingController _repeat;
  late final TextEditingController _cx;
  late final TextEditingController _cy;
  late final TextEditingController _r;
  late final TextEditingController _template;
  late final TextEditingController _threshold;
  late final TextEditingController _timeout;
  late final TextEditingController _poll;
  late final TextEditingController _offsetX;
  late final TextEditingController _offsetY;
  late final TextEditingController _regionX;
  late final TextEditingController _regionY;
  late final TextEditingController _regionW;
  late final TextEditingController _regionH;

  late String _kind;
  late String _button;
  late int _clicks;
  late String _onNotFound;
  String? _templateB64;
  String? _error;
  String? _matchText;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    final step = widget.initial;
    final target = step?.target;
    _x = TextEditingController(text: '${step?.x ?? 0}');
    _y = TextEditingController(text: '${step?.y ?? 0}');
    _interval = TextEditingController(text: '${step?.intervalMs ?? 0}');
    _hold = TextEditingController(text: '${step?.holdMs ?? 0}');
    _repeat = TextEditingController(text: '${step?.repeat ?? 1}');
    _cx = TextEditingController(text: '${target?.cx ?? 0}');
    _cy = TextEditingController(text: '${target?.cy ?? 0}');
    _r = TextEditingController(text: '${target?.r ?? 0}');
    _template = TextEditingController(text: target?.template ?? '');
    _threshold = TextEditingController(text: '${target?.threshold ?? 0}');
    _timeout = TextEditingController(text: '${target?.timeoutMs ?? 0}');
    _poll = TextEditingController(text: '${target?.pollMs ?? 0}');
    _offsetX = TextEditingController(text: '${target?.offsetX ?? 0}');
    _offsetY = TextEditingController(text: '${target?.offsetY ?? 0}');
    _regionX = TextEditingController(text: '${target?.region?.x ?? 0}');
    _regionY = TextEditingController(text: '${target?.region?.y ?? 0}');
    _regionW = TextEditingController(text: '${target?.region?.w ?? 0}');
    _regionH = TextEditingController(text: '${target?.region?.h ?? 0}');
    _templateB64 = target?.templateB64;
    _kind = step?.kindOrPoint ?? 'point';
    if (_kind != 'point' && _kind != 'circle' && _kind != 'image') _kind = 'point';
    _button = step?.button ?? 'left';
    if (_button != 'left' && _button != 'right' && _button != 'middle') _button = 'left';
    final clicks = step?.clicks ?? 1;
    _clicks = clicks == 0 ? 1 : clicks.clamp(1, 3).toInt();
    var mode = target?.onNotFound ?? '';
    if (mode == 'skip') mode = '';
    if (mode != '' && mode != 'wait' && mode != 'stop') mode = '';
    _onNotFound = mode;
  }

  @override
  void dispose() {
    for (final controller in [
      _x,
      _y,
      _interval,
      _hold,
      _repeat,
      _cx,
      _cy,
      _r,
      _template,
      _threshold,
      _timeout,
      _poll,
      _offsetX,
      _offsetY,
      _regionX,
      _regionY,
      _regionW,
      _regionH,
    ]) {
      controller.dispose();
    }
    super.dispose();
  }

  int? _nonNegative(TextEditingController controller) {
    final value = int.tryParse(controller.text.trim());
    if (value == null || value < 0) return null;
    return value;
  }

  int? _anyInt(TextEditingController controller) {
    final text = controller.text.trim();
    if (text.isEmpty) return 0;
    return int.tryParse(text);
  }

  Future<Uint8List?> _captureBytes() async {
    final engine = widget.engine;
    if (engine == null) {
      setState(() => _error = '沒有可用的引擎');
      return null;
    }
    try {
      final dir = await getTemporaryDirectory();
      final path = '${dir.path}/click_spirit_cap_${DateTime.now().microsecondsSinceEpoch}.png';
      final err = await engine.captureScreen(path);
      if (!mounted) return null;
      if (err.isNotEmpty) {
        setState(() => _error = err);
        return null;
      }
      return await File(path).readAsBytes();
    } catch (e) {
      if (!mounted) return null;
      setState(() => _error = '截圖失敗：$e');
      return null;
    }
  }

  CaptureSelection? _initialSelection(CapturePickMode mode) {
    int? n(TextEditingController c) => int.tryParse(c.text.trim());
    switch (mode) {
      case CapturePickMode.point:
        final x = n(_x), y = n(_y);
        return x == null || y == null ? null : CaptureSelection.point(x, y);
      case CapturePickMode.circle:
        final cx = n(_cx), cy = n(_cy), r = n(_r);
        return cx == null || cy == null || r == null ? null : CaptureSelection.circle(cx, cy, r);
      case CapturePickMode.rect:
        return null;
    }
  }

  Future<void> _pickOnScreen(CapturePickMode mode) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    final bytes = await _captureBytes();
    if (!mounted) return;
    setState(() => _busy = false);
    if (bytes == null) return;
    final picked = await showCaptureScreen(
      context,
      bytes,
      mode,
      number: widget.number,
      others: widget.others,
      initial: _initialSelection(mode),
    );
    if (!mounted || picked == null) return;
    if (mode == CapturePickMode.rect) {
      setState(() => _busy = true);
      try {
        final cropped = await cropPng(bytes, x: picked.rectX, y: picked.rectY, w: picked.rectW, h: picked.rectH);
        final saved = await saveTemplatePng(cropped);
        if (!mounted) return;
        setState(() {
          _template.text = saved;
          _templateB64 = '';
          _matchText = null;
          _busy = false;
        });
      } catch (e) {
        if (!mounted) return;
        setState(() {
          _busy = false;
          _error = '無法儲存模板：$e';
        });
      }
      return;
    }
    setState(() {
      if (mode == CapturePickMode.point) {
        _x.text = '${picked.x}';
        _y.text = '${picked.y}';
      } else {
        _cx.text = '${picked.cx}';
        _cy.text = '${picked.cy}';
        _r.text = '${picked.r}';
      }
    });
  }

  Future<void> _pickPng() async {
    final file = await openFile(
      acceptedTypeGroups: const [
        XTypeGroup(label: 'PNG', extensions: ['png'], mimeTypes: ['image/png']),
      ],
    );
    if (file == null || !mounted) return;
    setState(() {
      _template.text = file.path;
      _templateB64 = '';
      _matchText = null;
      _error = null;
    });
  }

  Future<void> _testMatch() async {
    final engine = widget.engine;
    if (engine == null) {
      setState(() => _error = '沒有可用的引擎');
      return;
    }
    final path = _template.text.trim();
    if (path.isEmpty) {
      setState(() => _error = '請先選擇模板');
      return;
    }
    final threshold = double.tryParse(_threshold.text.trim());
    if (threshold == null || threshold < 0 || threshold > 1) {
      setState(() => _error = '門檻必須是 0 到 1 的數字');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
      _matchText = '比對中…';
    });
    try {
      final raw = await engine.matchImage(path, threshold);
      if (!mounted) return;
      setState(() {
        _busy = false;
        _matchText = MatchHit.parse(raw).describe();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _matchText = '比對失敗：$e';
      });
    }
  }

  void _submit() {
    final interval = _nonNegative(_interval);
    final hold = _nonNegative(_hold);
    final repeat = _nonNegative(_repeat);
    if (interval == null || hold == null || repeat == null) {
      setState(() => _error = '間隔、按住與重複必須是大於等於 0 的整數');
      return;
    }
    final x = _nonNegative(_x) ?? 0;
    final y = _nonNegative(_y) ?? 0;
    if (_kind == 'point' && (_nonNegative(_x) == null || _nonNegative(_y) == null)) {
      setState(() => _error = 'X、Y 必須是大於等於 0 的整數');
      return;
    }

    Target? target;
    if (_kind == 'circle') {
      final cx = _nonNegative(_cx);
      final cy = _nonNegative(_cy);
      final r = _nonNegative(_r);
      if (cx == null || cy == null || r == null) {
        setState(() => _error = '圓心與半徑必須是大於等於 0 的整數');
        return;
      }
      target = Target(kind: 'circle', cx: cx, cy: cy, r: r);
    } else if (_kind == 'image') {
      final path = _template.text.trim();
      final b64 = _templateB64 ?? '';
      if (path.isEmpty && b64.isEmpty) {
        setState(() => _error = '找圖需要 PNG 模板');
        return;
      }
      final threshold = double.tryParse(_threshold.text.trim());
      if (threshold == null || threshold < 0 || threshold > 1) {
        setState(() => _error = '門檻必須是 0 到 1 的數字');
        return;
      }
      final timeout = _nonNegative(_timeout);
      final poll = _nonNegative(_poll);
      final offsetX = _anyInt(_offsetX);
      final offsetY = _anyInt(_offsetY);
      final rx = _nonNegative(_regionX);
      final ry = _nonNegative(_regionY);
      final rw = _nonNegative(_regionW);
      final rh = _nonNegative(_regionH);
      if (timeout == null || poll == null || rx == null || ry == null || rw == null || rh == null) {
        setState(() => _error = '逾時、輪詢與搜尋範圍必須是大於等於 0 的整數');
        return;
      }
      if (offsetX == null || offsetY == null) {
        setState(() => _error = '偏移必須是整數');
        return;
      }
      if ((rw == 0) != (rh == 0)) {
        setState(() => _error = '搜尋範圍的寬與高要同時為 0（全畫面）或同時大於 0');
        return;
      }
      target = Target(
        kind: 'image',
        template: path,
        templateB64: b64,
        threshold: threshold,
        offsetX: offsetX,
        offsetY: offsetY,
        onNotFound: _onNotFound,
        timeoutMs: timeout,
        pollMs: poll,
        region: rw > 0 && rh > 0 ? Region(x: rx, y: ry, w: rw, h: rh) : null,
      );
    }

    Navigator.of(context).pop(Step(
      x: x,
      y: y,
      button: _button,
      clicks: _clicks,
      intervalMs: interval,
      holdMs: hold,
      repeat: repeat,
      target: target,
    ));
  }

  @override
  Widget build(BuildContext context) {
    final width = math.min(560.0, math.max(280.0, MediaQuery.sizeOf(context).width - 80));
    final maxHeight = MediaQuery.sizeOf(context).height * 0.86;
    return Dialog(
      backgroundColor: Colors.transparent,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      insetPadding: const EdgeInsets.symmetric(horizontal: 28, vertical: 24),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: width, maxHeight: maxHeight),
        child: GlassPanel(
          radius: 24,
          padding: const EdgeInsets.fromLTRB(22, 18, 22, 14),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                widget.initial == null ? '新增步驟' : '編輯步驟',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w600,
                  letterSpacing: -0.4,
                  height: 1.25,
                  color: ClickSpiritColors.label(context),
                ),
              ),
              const SizedBox(height: 14),
              Flexible(
                child: SingleChildScrollView(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SegmentedButton<String>(
                        showSelectedIcon: false,
                        segments: const [
                          ButtonSegment(value: 'point', label: Text('定點')),
                          ButtonSegment(value: 'circle', label: Text('圓形')),
                          ButtonSegment(value: 'image', label: Text('找圖')),
                        ],
                        selected: {_kind},
                        onSelectionChanged: (next) => setState(() => _kind = next.first),
                      ),
                      const SizedBox(height: 14),
                      if (_kind == 'point') ..._pointFields(),
                      if (_kind == 'circle') ..._circleFields(),
                      if (_kind == 'image') ..._imageFields(),
                      const SizedBox(height: 10),
                      GlassDropdownField<String>(
                        label: '按鍵',
                        value: _button,
                        items: const [
                          DropdownMenuItem(value: 'left', child: Text('左鍵')),
                          DropdownMenuItem(value: 'right', child: Text('右鍵')),
                          DropdownMenuItem(value: 'middle', child: Text('中鍵')),
                        ],
                        onChanged: (value) => setState(() => _button = value ?? 'left'),
                      ),
                      const SizedBox(height: 10),
                      GlassDropdownField<int>(
                        label: '點擊次數',
                        value: _clicks,
                        items: const [
                          DropdownMenuItem(value: 1, child: Text('1')),
                          DropdownMenuItem(value: 2, child: Text('2')),
                          DropdownMenuItem(value: 3, child: Text('3')),
                        ],
                        onChanged: (value) => setState(() => _clicks = value ?? 1),
                      ),
                      const SizedBox(height: 10),
                      GlassTextField(
                        controller: _interval,
                        keyboardType: TextInputType.number,
                        label: '間隔（毫秒，0 表示用預設）',
                      ),
                      const SizedBox(height: 10),
                      GlassTextField(
                        controller: _hold,
                        keyboardType: TextInputType.number,
                        label: '按住（毫秒）',
                      ),
                      const SizedBox(height: 10),
                      GlassTextField(
                        controller: _repeat,
                        keyboardType: TextInputType.number,
                        label: '重複次數（0 或 1 代表一次）',
                      ),
                      if (_error != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text(_error!, style: const TextStyle(color: ClickSpiritColors.systemRed, fontSize: 13)),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 14),
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('取消')),
                  const SizedBox(width: 8),
                  GlassButton(
                    onPressed: _busy ? null : _submit,
                    label: '確定',
                    variant: GlassButtonVariant.primary,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Live miniature of where this step clicks, next to the script's other steps.
  Widget _livePreview() {
    return ListenableBuilder(
      listenable: Listenable.merge([_x, _y, _cx, _cy, _r]),
      builder: (context, _) {
        int v(TextEditingController c) => int.tryParse(c.text.trim()) ?? 0;
        final StepMarker self = _kind == 'circle'
            ? StepMarker.circle(widget.number, Offset(v(_cx).toDouble(), v(_cy).toDouble()), v(_r).toDouble())
            : StepMarker.point(widget.number, Offset(v(_x).toDouble(), v(_y).toDouble()));
        return Padding(
          padding: const EdgeInsets.only(top: 10),
          child: SizedBox(
            height: 130,
            child: StepMapPreview(
              key: const Key('step-editor-preview'),
              markers: [...widget.others, self],
              highlight: widget.number - 1,
            ),
          ),
        );
      },
    );
  }

  List<Widget> _pointFields() {
    return [
      GlassTextField(
        fieldKey: const Key('point-x'),
        controller: _x,
        keyboardType: TextInputType.number,
        label: 'X 座標',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        fieldKey: const Key('point-y'),
        controller: _y,
        keyboardType: TextInputType.number,
        label: 'Y 座標',
      ),
      const SizedBox(height: 10),
      Align(
        alignment: Alignment.centerLeft,
        child: GlassButton(
          onPressed: _busy ? null : () => _pickOnScreen(CapturePickMode.point),
          icon: Icons.screenshot_monitor_outlined,
          label: '從螢幕點選座標',
          variant: GlassButtonVariant.tinted,
        ),
      ),
      _livePreview(),
    ];
  }

  List<Widget> _circleFields() {
    return [
      GlassTextField(
        fieldKey: const Key('circle-cx'),
        controller: _cx,
        keyboardType: TextInputType.number,
        label: '圓心 X',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        fieldKey: const Key('circle-cy'),
        controller: _cy,
        keyboardType: TextInputType.number,
        label: '圓心 Y',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        fieldKey: const Key('circle-r'),
        controller: _r,
        keyboardType: TextInputType.number,
        label: '半徑',
      ),
      const SizedBox(height: 10),
      Align(
        alignment: Alignment.centerLeft,
        child: GlassButton(
          onPressed: _busy ? null : () => _pickOnScreen(CapturePickMode.circle),
          icon: Icons.circle_outlined,
          label: '從螢幕擷取設定圓形',
          variant: GlassButtonVariant.tinted,
        ),
      ),
      _livePreview(),
    ];
  }

  List<Widget> _imageFields() {
    return [
      GlassTextField(
        controller: _template,
        label: '模板路徑',
        onChanged: (_) {
          _templateB64 = '';
          _matchText = null;
        },
      ),
      const SizedBox(height: 10),
      Wrap(
        spacing: 8,
        runSpacing: 8,
        children: [
          GlassButton(
            onPressed: _busy ? null : _pickPng,
            icon: Icons.file_open_outlined,
            label: '選擇 PNG',
            variant: GlassButtonVariant.tinted,
          ),
          GlassButton(
            onPressed: _busy ? null : () => _pickOnScreen(CapturePickMode.rect),
            icon: Icons.crop_outlined,
            label: '從螢幕裁切',
            variant: GlassButtonVariant.tinted,
          ),
          GlassButton(
            onPressed: _busy ? null : _testMatch,
            icon: Icons.search_rounded,
            label: '測試比對',
            variant: GlassButtonVariant.primary,
          ),
        ],
      ),
      if (_matchText != null)
        Padding(
          padding: const EdgeInsets.only(top: 8),
          child: Text(_matchText!),
        ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _threshold,
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        label: '門檻（0 到 1，0 代表 0.9）',
      ),
      const SizedBox(height: 10),
      GlassDropdownField<String>(
        label: '找不到時',
        value: _onNotFound,
        items: const [
          DropdownMenuItem(value: '', child: Text('略過')),
          DropdownMenuItem(value: 'wait', child: Text('等待')),
          DropdownMenuItem(value: 'stop', child: Text('停止')),
        ],
        onChanged: (value) => setState(() => _onNotFound = value ?? ''),
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _timeout,
        keyboardType: TextInputType.number,
        label: '逾時（毫秒）',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _poll,
        keyboardType: TextInputType.number,
        label: '輪詢（毫秒，0 代表 100）',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _offsetX,
        keyboardType: const TextInputType.numberWithOptions(signed: true),
        label: '偏移 X',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _offsetY,
        keyboardType: const TextInputType.numberWithOptions(signed: true),
        label: '偏移 Y',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _regionX,
        keyboardType: TextInputType.number,
        label: '搜尋範圍 X',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _regionY,
        keyboardType: TextInputType.number,
        label: '搜尋範圍 Y',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _regionW,
        keyboardType: TextInputType.number,
        label: '搜尋範圍寬（0 代表全畫面）',
      ),
      const SizedBox(height: 10),
      GlassTextField(
        controller: _regionH,
        keyboardType: TextInputType.number,
        label: '搜尋範圍高（0 代表全畫面）',
      ),
    ];
  }
}
