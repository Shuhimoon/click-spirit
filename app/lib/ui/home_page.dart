import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:path_provider/path_provider.dart';

import '../engine/click_engine.dart';
import '../model/script.dart';
import 'step_dialog.dart';
import 'step_list.dart';

class HomePage extends StatefulWidget {
  const HomePage({
    super.key,
    required this.engine,
    required this.enableCursorCapture,
    required this.enableAccessibility,
  });

  final ClickEngine engine;
  final bool enableCursorCapture;
  final bool enableAccessibility;

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
  bool _sawEmergency = false;
  bool? _a11y;
  String _status = '就緒';
  String _backend = '';
  int _captureGen = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _apply(Script.sample());
    _sub = widget.engine.events.listen(_onEvent);
    _loadBackend();
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
          _status = '執行中';
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

  Future<void> _add() async {
    final step = await showStepEditor(context, engine: widget.engine);
    if (step == null || !mounted) return;
    setState(() => _rows.add(StepRow(step)));
  }

  Future<void> _edit(int index) async {
    final step = await showStepEditor(context, initial: _rows[index].step, engine: widget.engine);
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
    return Scaffold(
      appBar: AppBar(
        title: const Text('點擊精靈'),
        actions: [
          IconButton(tooltip: '開啟', onPressed: _open, icon: const Icon(Icons.folder_open)),
          IconButton(tooltip: '儲存', onPressed: _save, icon: const Icon(Icons.save)),
          IconButton(tooltip: '新增步驟', onPressed: _running ? null : _add, icon: const Icon(Icons.add)),
        ],
      ),
      body: LayoutBuilder(
        builder: (context, constraints) {
          final preferred = constraints.maxHeight * 0.34;
          final headerMax = preferred < 150 ? 150.0 : (preferred > 230 ? 230.0 : preferred);
          return Column(
            children: [
              ConstrainedBox(
                constraints: BoxConstraints(maxHeight: headerMax),
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(12, 12, 12, 0),
                  child: _settings(),
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                child: _actions(),
              ),
              const Divider(height: 1),
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
              const Divider(height: 1),
              _logPane(),
            ],
          );
        },
      ),
    );
  }

  Widget _settings() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (_backend.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text('後端：$_backend'),
          ),
        if (Platform.isLinux)
          const Padding(
            padding: EdgeInsets.only(bottom: 8),
            child: Text(
              'Wayland（uinput）需要輸入裝置權限：使用者需加入 input 群組，或設定 udev 規則 KERNEL=="uinput", GROUP="input", MODE="0660"。',
              style: TextStyle(fontSize: 12),
            ),
          ),
        TextField(
          controller: _name,
          decoration: const InputDecoration(
            labelText: '腳本名稱',
            isDense: true,
            border: OutlineInputBorder(),
          ),
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _numField(_repeat, '重複次數（0 代表無限）'),
            _numField(_defaultInterval, '預設間隔（毫秒）'),
            _numField(_loopInterval, '迴圈間隔（毫秒）'),
            _numField(_startDelay, '開始延遲（毫秒）'),
            _numField(_hotkey, '緊急停止熱鍵', number: false),
          ],
        ),
      ],
    );
  }

  Widget _numField(TextEditingController controller, String label, {bool number = true}) {
    return SizedBox(
      width: 220,
      child: TextField(
        controller: controller,
        keyboardType: number ? TextInputType.number : TextInputType.text,
        decoration: InputDecoration(
          labelText: label,
          isDense: true,
          border: const OutlineInputBorder(),
        ),
      ),
    );
  }

  Widget _actions() {
    final a11y = _a11y;
    final busy = _running || _recording;
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        FilledButton.icon(
          onPressed: busy ? null : _start,
          icon: const Icon(Icons.play_arrow),
          label: const Text('開始'),
        ),
        OutlinedButton.icon(
          onPressed: _running ? _stop : null,
          icon: const Icon(Icons.stop),
          label: const Text('停止'),
        ),
        FilledButton.icon(
          style: FilledButton.styleFrom(
            backgroundColor: Theme.of(context).colorScheme.error,
            foregroundColor: Theme.of(context).colorScheme.onError,
          ),
          onPressed: _emergency,
          icon: const Icon(Icons.front_hand),
          label: const Text('緊急停止'),
        ),
        OutlinedButton.icon(
          onPressed: busy ? null : _startRecording,
          icon: const Icon(Icons.fiber_manual_record),
          label: const Text('開始錄製'),
        ),
        OutlinedButton.icon(
          onPressed: _recording ? _stopRecording : null,
          icon: const Icon(Icons.stop_circle_outlined),
          label: const Text('停止錄製'),
        ),
        if (widget.enableCursorCapture)
          OutlinedButton.icon(
            onPressed: busy ? null : _capture,
            icon: const Icon(Icons.my_location),
            label: const Text('擷取座標'),
          ),
        if (widget.enableAccessibility) ...[
          OutlinedButton.icon(
            onPressed: _openA11y,
            icon: const Icon(Icons.accessibility_new),
            label: const Text('開啟無障礙設定'),
          ),
          Text(a11y == null
              ? '無障礙服務：讀取中'
              : a11y
                  ? '無障礙服務：已啟用'
                  : '無障礙服務：未啟用'),
        ],
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 520),
          child: Text(_status, key: const Key('status-text')),
        ),
      ],
    );
  }

  Widget _logPane() {
    return SizedBox(
      height: 108,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(12, 6, 12, 0),
            child: Text('事件紀錄'),
          ),
          Expanded(
            child: ListView.builder(
              controller: _logScroll,
              itemCount: _log.length,
              itemBuilder: (context, index) {
                final ev = _log[index];
                final text = ev.message.isEmpty ? ev.kind : '${ev.kind}  ${ev.message}';
                return Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 1),
                  child: Text(text, style: const TextStyle(fontSize: 12)),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
