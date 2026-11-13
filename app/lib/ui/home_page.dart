import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:path_provider/path_provider.dart';

import '../engine/click_engine.dart';
import '../model/app_settings.dart';
import '../model/script.dart';
import 'glass.dart';
import 'markers.dart';
import 'step_dialog.dart';
import 'step_list.dart';
import 'theme.dart';

class HomePage extends StatefulWidget {
  const HomePage({
    super.key,
    required this.engine,
    required this.enableCursorCapture,
    required this.enableAccessibility,
    this.settings,
    this.initialScript,
  });

  /// Script shown at launch. Null shows the built-in sample.
  final Script? initialScript;

  final ClickEngine engine;
  final bool enableCursorCapture;
  final bool enableAccessibility;

  /// Where preferences such as 顯示點擊位置 are kept. Null uses a JSON file
  /// in the app support directory.
  final SettingsStore? settings;

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> with WidgetsBindingObserver {
  final _name = TextEditingController();
  final _repeat = TextEditingController();
  final _defaultInterval = TextEditingController();
  final _loopInterval = TextEditingController();
  final _startDelay = TextEditingController();
  final _hotkey = TextEditingController(text: 'F12');
  final _logScroll = ScrollController();
  final List<StepRow> _rows = [];
  final List<EngineEvent> _log = [];

  StreamSubscription<EngineEvent>? _sub;
  int? _selected;
  int? _activeStep;
  int _activeRepetition = 0;
  bool _running = false;
  bool _recording = false;
  bool _logOpen = true;
  bool _sawEmergency = false;
  bool? _a11y;
  String _status = '就緒';
  String _backend = '';
  int _captureGen = 0;
  late final SettingsStore _store = widget.settings ?? FileSettingsStore();
  bool _showClicks = true;
  bool _hasOverlay = false;
  ClickMark? _lastClick;
  int _clickSerial = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _apply(widget.initialScript ?? Script.sample());
    _sub = widget.engine.events.listen(_onEvent);
    _loadBackend();
    _loadSettings();
    if (widget.enableAccessibility) {
      _refreshA11y();
    }
  }

  Future<void> _loadBackend() async {
    try {
      final name = await widget.engine.backendName();
      if (!mounted) return;
      setState(() => _backend = name);
    } catch (_) {}
  }

  Future<void> _loadSettings() async {
    final settings = await _store.load();
    var overlay = false;
    try {
      overlay = await widget.engine.hasClickOverlay();
    } catch (_) {}
    if (!mounted) return;
    setState(() {
      _showClicks = settings.showClicks;
      _hasOverlay = overlay;
    });
    await widget.engine.setShowClicks(settings.showClicks);
  }

  Future<void> _setShowClicks(bool on) async {
    setState(() {
      _showClicks = on;
      if (!on) _lastClick = null;
    });
    await widget.engine.setShowClicks(on);
    await _store.save(AppSettings(showClicks: on));
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _captureGen++;
    _sub?.cancel();
    widget.engine.dispose();
    _name.dispose();
    _repeat.dispose();
    _defaultInterval.dispose();
    _loopInterval.dispose();
    _startDelay.dispose();
    _hotkey.dispose();
    _logScroll.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _refreshA11y();
    }
  }

  void _apply(Script script) {
    _name.text = script.name;
    _repeat.text = '${script.repeat}';
    _defaultInterval.text = '${script.defaultIntervalMs}';
    _loopInterval.text = '${script.loopIntervalMs}';
    _startDelay.text = '${script.startDelayMs}';
    _hotkey.text = script.emergencyHotkey.isEmpty ? 'F12' : script.emergencyHotkey;
    _rows
      ..clear()
      ..addAll(script.steps.map(StepRow.new));
    _selected = null;
    _activeStep = null;
  }

  Script _current() {
    return Script(
      version: 1,
      name: _name.text.trim().isEmpty ? '未命名' : _name.text.trim(),
      steps: _rows.map((row) => row.step).toList(),
      repeat: _readInt(_repeat),
      defaultIntervalMs: _readInt(_defaultInterval),
      loopIntervalMs: _readInt(_loopInterval),
      startDelayMs: _readInt(_startDelay),
      emergencyHotkey: _hotkey.text.trim(),
    );
  }

  int _readInt(TextEditingController controller) {
    return int.tryParse(controller.text.trim()) ?? 0;
  }

  void _onEvent(EngineEvent ev) {
    if (!mounted) return;
    setState(() {
      _log.add(ev);
      if (_log.length > 200) {
        _log.removeRange(0, _log.length - 200);
      }
      switch (ev.kind) {
        case 'started':
          _running = true;
          _sawEmergency = false;
          _activeStep = null;
          _lastClick = null;
          _status = '執行中';
        case 'click':
          _running = true;
          if (ev.step >= 0 && ev.step < _rows.length) {
            _activeStep = ev.step;
            _activeRepetition = ev.repetition;
            final total = ev.repeats > 0 ? ev.repeats : _rows[ev.step].step.effectiveRepeat;
            _lastClick = ClickMark(
              x: ev.x,
              y: ev.y,
              step: ev.step,
              repetition: ev.repetition,
              repeats: total,
              serial: ++_clickSerial,
            );
            _status = '執行中：${clickLabel(ev.step, ev.repetition, total)} · 點在 (${ev.x}, ${ev.y})';
          }
        case 'step':
          _running = true;
          if (ev.step >= 0 && ev.step < _rows.length) {
            _activeStep = ev.step;
            _activeRepetition = ev.repetition;
            final total = _rows[ev.step].step.effectiveRepeat;
            _status = '執行中：步驟 ${ev.step + 1}，第 ${ev.repetition + 1}/$total 次';
          } else {
            _status = '執行中';
          }
        case 'finished':
          _running = false;
          _activeStep = null;
          _status = '執行完成';
        case 'stopped':
          _running = false;
          _activeStep = null;
          if (!_sawEmergency) _status = '已停止';
        case 'emergency_stop':
          _running = false;
          _sawEmergency = true;
          _activeStep = null;
          _status = '緊急停止';
        case 'error':
          _running = false;
          _activeStep = null;
          _status = ev.message.isEmpty ? '錯誤' : ev.message;
        default:
          break;
      }
    });
    _scrollLog();
  }

  void _scrollLog() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_logScroll.hasClients) return;
      _logScroll.jumpTo(_logScroll.position.maxScrollExtent);
    });
  }

  Future<void> _refreshA11y() async {
    if (!widget.enableAccessibility) return;
    try {
      final ok = await widget.engine.accessibilityTrusted();
      if (!mounted) return;
      setState(() => _a11y = ok);
    } catch (_) {
      if (!mounted) return;
      setState(() => _a11y = false);
    }
  }

  Future<String?> _documentsDir() async {
    try {
      final dir = await getApplicationDocumentsDirectory();
      return dir.path;
    } catch (_) {
      return null;
    }
  }

  Future<void> _open() async {
    final initial = await _documentsDir();
    if (!mounted) return;
    final file = await openFile(
      acceptedTypeGroups: const [
        XTypeGroup(label: 'JSON', extensions: ['json']),
      ],
      initialDirectory: initial,
    );
    if (file == null || !mounted) return;
    try {
      final text = await file.readAsString();
      final script = Script.fromJson(jsonDecode(text) as Map<String, dynamic>);
      if (!mounted) return;
      setState(() {
        _apply(script);
        _status = '已開啟 ${file.name}';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _status = '無法開啟：$e');
    }
  }

  Future<void> _save() async {
    final script = _current();
    final initial = await _documentsDir();
    if (!mounted) return;
    final suggested = script.name.endsWith('.json') ? script.name : '${script.name}.json';
    final loc = await getSaveLocation(
      suggestedName: suggested,
      initialDirectory: initial,
      acceptedTypeGroups: const [
        XTypeGroup(label: 'JSON', extensions: ['json']),
      ],
    );
    if (loc == null || !mounted) return;
    try {
      final text = '${const JsonEncoder.withIndent('  ').convert(script.toJson())}\n';
      final data = Uint8List.fromList(utf8.encode(text));
      final out = XFile.fromData(data, name: suggested, mimeType: 'application/json');
      await out.saveTo(loc.path);
      if (!mounted) return;
      setState(() => _status = '已儲存 ${loc.path}');
    } catch (e) {
      if (!mounted) return;
      setState(() => _status = '無法儲存：$e');
    }
  }

  /// Numbered markers of every step except [skip], for the editor previews.
  List<StepMarker> _markersExcept(int? skip) {
    final steps = _rows.map((row) => row.step).toList();
    return markersOf(steps).where((m) => m.number - 1 != skip).toList();
  }

  Future<void> _add() async {
    final step = await showStepEditor(
      context,
      engine: widget.engine,
      number: _rows.length + 1,
      others: _markersExcept(null),
    );
    if (step == null || !mounted) return;
    setState(() => _rows.add(StepRow(step)));
  }

  Future<void> _edit(int index) async {
    final step = await showStepEditor(
      context,
      initial: _rows[index].step,
      engine: widget.engine,
      number: index + 1,
      others: _markersExcept(index),
    );
    if (step == null || !mounted) return;
    setState(() => _rows[index].step = step);
  }

  void _delete(int index) {
    setState(() {
      _rows.removeAt(index);
      if (_selected != null) {
        if (_selected == index) {
          _selected = null;
        } else if (_selected! > index) {
          _selected = _selected! - 1;
        }
      }
      if (_activeStep != null) {
        if (_activeStep == index) {
          _activeStep = null;
        } else if (_activeStep! > index) {
          _activeStep = _activeStep! - 1;
        }
      }
    });
  }

  void _duplicate(int index) {
    setState(() => _rows.insert(index + 1, StepRow(_rows[index].step.copy())));
  }

  void _reorder(int oldIndex, int newIndex) {
    setState(() {
      final row = _rows.removeAt(oldIndex);
      _rows.insert(newIndex, row);
      _selected = _selected == null ? null : shiftAfterReorder(_selected!, oldIndex, newIndex);
      _activeStep = _activeStep == null ? null : shiftAfterReorder(_activeStep!, oldIndex, newIndex);
    });
  }

  Future<void> _start() async {
    final script = _current();
    final raw = jsonEncode(script.toJson());
    final invalid = await widget.engine.validate(raw);
    if (!mounted) return;
    if (invalid.isNotEmpty) {
      setState(() => _status = '驗證失敗：$invalid');
      return;
    }
    final hotkeyError = await widget.engine.setHotkey(script.emergencyHotkey);
    if (!mounted) return;
    if (hotkeyError.isNotEmpty && widget.enableCursorCapture) {
      setState(() => _status = '無法設定熱鍵：$hotkeyError');
      return;
    }
    final err = await widget.engine.start(raw);
    if (!mounted) return;
    setState(() {
      if (err.isNotEmpty) {
        _running = false;
        _status = '無法開始：$err';
      } else {
        _running = true;
        _sawEmergency = false;
        _status = hotkeyError.isEmpty ? '執行中' : '執行中（$hotkeyError）';
      }
    });
  }

  Future<void> _stop() async {
    await widget.engine.stop();
    if (!mounted) return;
    setState(() {
      _running = false;
      _sawEmergency = false;
      _activeStep = null;
      _status = '已停止';
    });
  }

  Future<void> _emergency() async {
    await widget.engine.emergencyStop();
    if (!mounted) return;
    setState(() {
      _running = false;
      _sawEmergency = true;
      _activeStep = null;
      _status = '緊急停止';
    });
  }

  Future<void> _startRecording() async {
    final raw = await widget.engine.startRecording();
    if (!mounted) return;
    final err = payloadError(raw);
    setState(() {
      if (err != null) {
        _recording = false;
        _status = '無法錄製：$err';
      } else {
        _recording = true;
        _status = '錄製中';
      }
    });
  }

  Future<void> _stopRecording() async {
    final raw = await widget.engine.stopRecording();
    if (!mounted) return;
    final err = payloadError(raw);
    if (err != null) {
      setState(() {
        _recording = false;
        _status = '無法停止錄製：$err';
      });
      return;
    }
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map) throw const FormatException('不是腳本');
      final script = Script.fromJson(Map<String, dynamic>.from(decoded));
      if (script.steps.isEmpty) {
        setState(() {
          _recording = false;
          _status = '錄製結果沒有步驟';
        });
        return;
      }
      setState(() {
        _apply(script);
        _recording = false;
        _status = '已載入錄製腳本';
      });
    } catch (e) {
      setState(() {
        _recording = false;
        _status = '錄製結果無法解析：$e';
      });
    }
  }

  Future<void> _capture() async {
    final gen = ++_captureGen;
    for (var sec = 3; sec >= 1; sec--) {
      if (!mounted || gen != _captureGen) return;
      setState(() => _status = '請把游標移到目標上，$sec 秒後擷取');
      await Future<void>.delayed(const Duration(seconds: 1));
    }
    if (!mounted || gen != _captureGen) return;
    final pos = await widget.engine.cursorPos();
    if (!mounted || gen != _captureGen) return;
    if (pos == null) {
      setState(() => _status = '此平台不支援座標擷取');
      return;
    }
    setState(() {
      final index = _selected;
      if (index != null && index >= 0 && index < _rows.length && _rows[index].step.kindOrPoint == 'point') {
        _rows[index].step = _rows[index].step.copyWith(x: pos.x, y: pos.y);
        _status = '已更新步驟 ${index + 1}：${pos.x}, ${pos.y}';
      } else {
        _rows.add(StepRow(Step(
          x: pos.x,
          y: pos.y,
          button: 'left',
          clicks: 1,
          intervalMs: 0,
          holdMs: 0,
        )));
        _status = '已新增步驟：${pos.x}, ${pos.y}';
      }
    });
  }

  Future<void> _openA11y() async {
    await widget.engine.openAccessibilitySettings();
    await _refreshA11y();
  }

  @override
  Widget build(BuildContext context) {
    final dark = ClickSpiritColors.isDark(context);
    final barColor = dark ? Colors.black.withValues(alpha: 0.28) : Colors.white.withValues(alpha: 0.46);
    final hairline = dark ? Colors.white.withValues(alpha: 0.10) : Colors.black.withValues(alpha: 0.06);
    return GlassBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        extendBodyBehindAppBar: true,
        appBar: PreferredSize(
          preferredSize: const Size.fromHeight(64),
          child: ClipRect(
            child: BackdropFilter(
              filter: ImageFilter.blur(sigmaX: 24, sigmaY: 24),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: barColor,
                  border: Border(bottom: BorderSide(color: hairline)),
                ),
                child: AppBar(
                  forceMaterialTransparency: true,
                  title: const Text('點擊精靈'),
                  actions: [
                    IconButton(
                      tooltip: '開啟',
                      onPressed: _open,
                      icon: const Icon(Icons.folder_open_outlined),
                    ),
                    IconButton(
                      tooltip: '儲存',
                      onPressed: _save,
                      icon: const Icon(Icons.save_outlined),
                    ),
                    IconButton(
                      tooltip: '新增步驟',
                      onPressed: _running ? null : _add,
                      icon: const Icon(Icons.add),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
        body: LayoutBuilder(
          builder: (context, constraints) {
            final wide = constraints.maxWidth >= 700;
            final pad = wide ? 24.0 : 20.0;
            // extendBodyBehindAppBar already includes the bar in padding.top.
            final top = MediaQuery.paddingOf(context).top + 12;
            return Padding(
              padding: EdgeInsets.fromLTRB(pad, top, pad, 16),
              child: wide ? _wideLayout() : _narrowLayout(),
            );
          },
        ),
      ),
    );
  }

  Widget _wideLayout() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _statusCard(),
        const SizedBox(height: 14),
        Expanded(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(flex: 5, child: _settingsPanel()),
              const SizedBox(width: 16),
              Expanded(flex: 6, child: _stepPanel()),
            ],
          ),
        ),
        const SizedBox(height: 14),
        _controlBar(),
        const SizedBox(height: 14),
        _logPane(),
      ],
    );
  }

  Widget _narrowLayout() {
    return LayoutBuilder(
      builder: (context, constraints) {
        final headerMax = (constraints.maxHeight * 0.46).clamp(160.0, 300.0);
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            SizedBox(
              height: headerMax,
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    _statusCard(),
                    const SizedBox(height: 12),
                    _settingsFields(),
                    const SizedBox(height: 12),
                    _controlBar(),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            Expanded(child: _stepPanel()),
            const SizedBox(height: 12),
            _logPane(),
          ],
        );
      },
    );
  }

  Widget _statusCard() {
    final secondary = ClickSpiritColors.secondary(context);
    final a11y = _a11y;
    return GlassPanel(
      padding: const EdgeInsets.fromLTRB(18, 14, 18, 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('狀態', style: _sectionStyle(secondary)),
          if (_backend.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(
              '後端：$_backend',
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                letterSpacing: -0.2,
                height: 1.3,
                color: ClickSpiritColors.label(context),
              ),
            ),
          ],
          if (Platform.isLinux) ...[
            const SizedBox(height: 4),
            Text(
              'Wayland（uinput）需要輸入裝置權限：使用者需加入 input 群組，或設定 udev 規則 KERNEL=="uinput", GROUP="input", MODE="0660"。',
              style: TextStyle(fontSize: 12, height: 1.35, color: secondary),
            ),
          ],
          if (widget.enableAccessibility) ...[
            const SizedBox(height: 4),
            Text(
              a11y == null
                  ? '無障礙服務：讀取中'
                  : a11y
                      ? '無障礙服務：已啟用'
                      : '無障礙服務：未啟用',
              style: TextStyle(fontSize: 12, height: 1.35, color: secondary),
            ),
          ],
        ],
      ),
    );
  }

  Widget _settingsPanel() {
    return GlassPanel(
      padding: const EdgeInsets.fromLTRB(18, 14, 18, 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('腳本設定', style: _sectionStyle(ClickSpiritColors.label(context), title: true)),
          const SizedBox(height: 12),
          Expanded(child: SingleChildScrollView(child: _settingsFields())),
        ],
      ),
    );
  }

  Widget _settingsFields() {
    return LayoutBuilder(
      builder: (context, constraints) {
        final max = constraints.maxWidth;
        final columns = max >= 460 ? 2 : 1;
        const gap = 12.0;
        final width = columns == 1 ? max : (max - gap) / 2;
        Widget box(Widget child, {bool full = false}) {
          return SizedBox(width: full || columns == 1 ? max : width, child: child);
        }

        return Wrap(
          spacing: gap,
          runSpacing: gap,
          children: [
            box(
              GlassTextField(controller: _name, label: '腳本名稱'),
              full: true,
            ),
            box(GlassTextField(
              controller: _repeat,
              label: '重複次數（0 代表無限）',
              keyboardType: TextInputType.number,
            )),
            box(GlassTextField(
              controller: _defaultInterval,
              label: '預設間隔（毫秒）',
              keyboardType: TextInputType.number,
            )),
            box(GlassTextField(
              controller: _loopInterval,
              label: '迴圈間隔（毫秒）',
              keyboardType: TextInputType.number,
            )),
            box(GlassTextField(
              controller: _startDelay,
              label: '開始延遲（毫秒）',
              keyboardType: TextInputType.number,
            )),
            box(GlassTextField(controller: _hotkey, label: '緊急停止熱鍵')),
            box(_showClicksTile(), full: true),
          ],
        );
      },
    );
  }

  Widget _showClicksTile() {
    final secondary = ClickSpiritColors.secondary(context);
    final detail = _hasOverlay
        ? '執行時在每次點擊處顯示漣漪與步驟編號。浮層在所有視窗上方，不會擋住滑鼠或觸控。'
        : '此平台目前沒有系統浮層，點擊位置只顯示在本視窗的預覽中。';
    return Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '顯示點擊位置',
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                  height: 1.3,
                  color: ClickSpiritColors.label(context),
                ),
              ),
              const SizedBox(height: 2),
              Text(detail, style: TextStyle(fontSize: 12, height: 1.35, color: secondary)),
            ],
          ),
        ),
        const SizedBox(width: 8),
        Switch(
          key: const Key('show-clicks-switch'),
          value: _showClicks,
          onChanged: _setShowClicks,
        ),
      ],
    );
  }

  Widget _mapPreview() {
    final steps = _rows.map((row) => row.step).toList();
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 6),
      child: SizedBox(
        height: MediaQuery.sizeOf(context).width >= 700 ? 168 : 120,
        child: StepMapPreview(
          markers: markersOf(steps),
          highlight: _activeStep ?? _selected,
          lastClick: _showClicks ? _lastClick : null,
        ),
      ),
    );
  }

  Widget _stepPanel() {
    return GlassPanel(
      padding: const EdgeInsets.fromLTRB(6, 12, 6, 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 2, 12, 4),
            child: Row(
              children: [
                Text('步驟', style: _sectionStyle(ClickSpiritColors.label(context), title: true)),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: ClickSpiritColors.systemBlue.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(999),
                  ),
                  child: Text(
                    '${_rows.length}',
                    style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      height: 1.2,
                      color: ClickSpiritColors.systemBlue,
                    ),
                  ),
                ),
              ],
            ),
          ),
          if (_rows.isNotEmpty || _lastClick != null) _mapPreview(),
          Expanded(
            child: StepList(
              rows: _rows,
              selected: _selected,
              activeIndex: _activeStep,
              activeRepetition: _activeRepetition,
              onSelect: (index) => setState(() => _selected = index),
              onEdit: _edit,
              onDelete: _delete,
              onDuplicate: _duplicate,
              onReorder: _reorder,
            ),
          ),
        ],
      ),
    );
  }

  Widget _controlBar() {
    final busy = _running || _recording;
    final neutral = ClickSpiritColors.isDark(context) ? const Color(0xFFE5E5EA) : const Color(0xFF1C1C1E);
    return GlassPanel(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          GlassButton(
            onPressed: busy ? null : _start,
            icon: Icons.play_arrow_rounded,
            label: '開始',
            variant: GlassButtonVariant.filled,
          ),
          GlassButton(
            onPressed: _running ? _stop : null,
            icon: Icons.stop_rounded,
            label: '停止',
            variant: GlassButtonVariant.tinted,
            color: neutral,
          ),
          GlassButton(
            onPressed: _emergency,
            icon: Icons.front_hand_outlined,
            label: '緊急停止',
            variant: GlassButtonVariant.destructive,
          ),
          GlassButton(
            onPressed: busy ? null : _startRecording,
            icon: Icons.fiber_manual_record_rounded,
            label: '開始錄製',
            variant: GlassButtonVariant.tinted,
            color: ClickSpiritColors.systemRed,
          ),
          GlassButton(
            onPressed: _recording ? _stopRecording : null,
            icon: Icons.stop_circle_outlined,
            label: '停止錄製',
            variant: GlassButtonVariant.tinted,
            color: ClickSpiritColors.systemRed,
          ),
          if (widget.enableCursorCapture)
            GlassButton(
              onPressed: busy ? null : _capture,
              icon: Icons.my_location_outlined,
              label: '擷取座標',
              variant: GlassButtonVariant.tinted,
            ),
          if (widget.enableAccessibility)
            GlassButton(
              onPressed: _openA11y,
              icon: Icons.accessibility_new_outlined,
              label: '開啟無障礙設定',
              variant: GlassButtonVariant.tinted,
            ),
          _statusChip(),
        ],
      ),
    );
  }

  Widget _statusChip() {
    final color = _statusColor();
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 420),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: color.withValues(alpha: 0.18)),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 7,
                height: 7,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              ),
              const SizedBox(width: 8),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 360),
                child: Text(
                  _status,
                  key: const Key('status-text'),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    height: 1.25,
                    color: color,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Color _statusColor() {
    final text = _status;
    if (text.contains('緊急') ||
        text.contains('無法') ||
        text.contains('失敗') ||
        text.contains('錯誤') ||
        text.contains('驗證')) {
      return ClickSpiritColors.systemRed;
    }
    if (text.startsWith('錄製')) return ClickSpiritColors.systemRed;
    if (text.startsWith('執行')) return ClickSpiritColors.systemGreen;
    if (text.startsWith('已')) return ClickSpiritColors.systemBlue;
    return ClickSpiritColors.systemGray;
  }

  Widget _logPane() {
    final secondary = ClickSpiritColors.secondary(context);
    final hairline = ClickSpiritColors.isDark(context)
        ? Colors.white.withValues(alpha: 0.08)
        : Colors.black.withValues(alpha: 0.06);
    return GlassPanel(
      padding: EdgeInsets.zero,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          InkWell(
            onTap: () => setState(() => _logOpen = !_logOpen),
            borderRadius: BorderRadius.circular(20),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 10, 8, 10),
              child: Row(
                children: [
                  Text('事件紀錄', style: _sectionStyle(ClickSpiritColors.label(context), title: true)),
                  const Spacer(),
                  if (_log.isNotEmpty)
                    Text(
                      '${_log.length}',
                      style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: secondary),
                    ),
                  Icon(
                    _logOpen ? Icons.expand_less_rounded : Icons.expand_more_rounded,
                    color: secondary,
                    size: 22,
                  ),
                ],
              ),
            ),
          ),
          if (_logOpen) ...[
            ColoredBox(color: hairline, child: const SizedBox(height: 1)),
            SizedBox(
              height: 72,
              child: _log.isEmpty
                  ? Center(
                      child: Text('尚無事件', style: TextStyle(fontSize: 12, color: secondary)),
                    )
                  : ListView.builder(
                      controller: _logScroll,
                      padding: const EdgeInsets.symmetric(vertical: 6),
                      itemCount: _log.length,
                      itemBuilder: (context, index) {
                        final ev = _log[index];
                        final text = ev.message.isEmpty ? ev.kind : '${ev.kind}  ${ev.message}';
                        return Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 1),
                          child: Text(
                            text,
                            style: TextStyle(
                              fontFamily: 'Noto Sans Mono CJK TC',
                              fontFamilyFallback: const [
                                'Noto Sans Mono',
                                'DejaVu Sans Mono',
                                'monospace',
                              ],
                              fontSize: 12,
                              height: 1.35,
                              color: ClickSpiritColors.secondary(context),
                            ),
                          ),
                        );
                      },
                    ),
            ),
          ],
        ],
      ),
    );
  }

  TextStyle _sectionStyle(Color color, {bool title = false}) {
    return TextStyle(
      fontSize: title ? 17 : 12,
      fontWeight: FontWeight.w600,
      letterSpacing: title ? -0.4 : 0,
      height: 1.2,
      color: color,
    );
  }
}
