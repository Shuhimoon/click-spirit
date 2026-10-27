import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:path_provider/path_provider.dart';

import '../engine/click_engine.dart';
import '../model/script.dart';

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

class _StepRow {
  _StepRow(this.step) : id = _nextId++;
  static int _nextId = 1;
  final int id;
  Step step;
}

class _HomePageState extends State<HomePage> with WidgetsBindingObserver {
  final _name = TextEditingController();
  final _repeat = TextEditingController();
  final _defaultInterval = TextEditingController();
  final _loopInterval = TextEditingController();
  final _startDelay = TextEditingController();
  final _logScroll = ScrollController();
  final List<_StepRow> _rows = [];
  final List<EngineEvent> _log = [];

  StreamSubscription<EngineEvent>? _sub;
  int? _selected;
  bool _running = false;
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
    _rows
      ..clear()
      ..addAll(script.steps.map(_StepRow.new));
    _selected = null;
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
      if (ev.kind == 'started') {
        _running = true;
        _status = '執行中';
      } else if (ev.kind == 'finished') {
        _running = false;
        _status = '執行完成';
      } else if (ev.kind == 'stopped') {
        _running = false;
        _status = '已停止';
      } else if (ev.kind == 'error') {
        _running = false;
        _status = ev.message.isEmpty ? '錯誤' : ev.message;
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
    final step = await _editDialog(null);
    if (step == null || !mounted) return;
    setState(() => _rows.add(_StepRow(step)));
  }

  Future<void> _edit(int index) async {
    final step = await _editDialog(_rows[index].step);
    if (step == null || !mounted) return;
    setState(() => _rows[index].step = step);
  }

  Future<void> _delete(int index) async {
    setState(() {
      _rows.removeAt(index);
      if (_selected == index) {
        _selected = null;
      } else if (_selected != null && _selected! > index) {
        _selected = _selected! - 1;
      }
    });
  }

  void _reorder(int oldIndex, int newIndex) {
    setState(() {
      final row = _rows.removeAt(oldIndex);
      _rows.insert(newIndex, row);
      if (_selected == oldIndex) {
        _selected = newIndex;
      }
    });
  }

  Future<Step?> _editDialog(Step? initial) {
    return showDialog<Step>(
      context: context,
      builder: (context) => _StepDialog(initial: initial),
    );
  }

  Future<void> _start() async {
    final raw = jsonEncode(_current().toJson());
    final invalid = await widget.engine.validate(raw);
    if (!mounted) return;
    if (invalid.isNotEmpty) {
      setState(() => _status = '驗證失敗：$invalid');
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
        _status = '執行中';
      }
    });
  }

  Future<void> _stop() async {
    await widget.engine.stop();
    if (!mounted) return;
    setState(() {
      _running = false;
      _status = '已停止';
    });
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
      if (index != null && index >= 0 && index < _rows.length) {
        _rows[index].step = _rows[index].step.copyWith(x: pos.x, y: pos.y);
        _status = '已更新步驟 ${index + 1}：${pos.x}, ${pos.y}';
      } else {
        _rows.add(_StepRow(Step(
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
          IconButton(tooltip: '新增步驟', onPressed: _add, icon: const Icon(Icons.add)),
        ],
      ),
      body: Column(
        children: [
          ConstrainedBox(
            constraints: const BoxConstraints(maxHeight: 250),
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
          Expanded(child: _stepList()),
          const Divider(height: 1),
          _logPane(),
        ],
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
          ],
        ),
      ],
    );
  }

  Widget _numField(TextEditingController controller, String label) {
    return SizedBox(
      width: 220,
      child: TextField(
        controller: controller,
        keyboardType: TextInputType.number,
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
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        FilledButton.icon(
          onPressed: _running ? null : _start,
          icon: const Icon(Icons.play_arrow),
          label: const Text('開始'),
        ),
        OutlinedButton.icon(
          onPressed: _running ? _stop : null,
          icon: const Icon(Icons.stop),
          label: const Text('停止'),
        ),
        if (widget.enableCursorCapture)
          OutlinedButton.icon(
            onPressed: _running ? null : _capture,
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
        Text(_status),
      ],
    );
  }

  Widget _stepList() {
    if (_rows.isEmpty) {
      return const Center(child: Text('尚未加入步驟'));
    }
    return ReorderableListView.builder(
      itemCount: _rows.length,
      onReorderItem: _reorder,
      buildDefaultDragHandles: false,
      itemBuilder: (context, index) {
        final row = _rows[index];
        final step = row.step;
        final selected = _selected == index;
        return Material(
          key: ValueKey(row.id),
          color: selected ? Theme.of(context).colorScheme.secondaryContainer : null,
          child: ListTile(
            onTap: () => setState(() => _selected = index),
            title: Text('${index + 1}. ${ _buttonLabel(step.button)}  (${step.x}, ${step.y})'),
            subtitle: Text(
              '次數 ${step.clicks == 0 ? "1（預設）" : step.clicks.toString()}  間隔 ${step.intervalMs} ms  按住 ${step.holdMs} ms',
            ),
            trailing: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                IconButton(
                  tooltip: '編輯',
                  onPressed: () => _edit(index),
                  icon: const Icon(Icons.edit),
                ),
                IconButton(
                  tooltip: '刪除',
                  onPressed: () => _delete(index),
                  icon: const Icon(Icons.delete),
                ),
                ReorderableDragStartListener(
                  index: index,
                  child: const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 8),
                    child: Icon(Icons.drag_handle),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _logPane() {
    return SizedBox(
      height: 150,
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

  String _buttonLabel(String button) {
    switch (button) {
      case 'left':
        return '左鍵';
      case 'right':
        return '右鍵';
      case 'middle':
        return '中鍵';
      default:
        return button;
    }
  }
}

class _StepDialog extends StatefulWidget {
  const _StepDialog({this.initial});

  final Step? initial;

  @override
  State<_StepDialog> createState() => _StepDialogState();
}

class _StepDialogState extends State<_StepDialog> {
  late final TextEditingController _x;
  late final TextEditingController _y;
  late final TextEditingController _interval;
  late final TextEditingController _hold;
  late String _button;
  late int _clicks;
  String? _error;

  @override
  void initState() {
    super.initState();
    final step = widget.initial;
    _x = TextEditingController(text: '${step?.x ?? 0}');
    _y = TextEditingController(text: '${step?.y ?? 0}');
    _interval = TextEditingController(text: '${step?.intervalMs ?? 0}');
    _hold = TextEditingController(text: '${step?.holdMs ?? 0}');
    _button = step?.button ?? 'left';
    if (_button != 'left' && _button != 'right' && _button != 'middle') {
      _button = 'left';
    }
    final clicks = step?.clicks ?? 1;
    _clicks = clicks == 0 ? 1 : clicks.clamp(1, 3).toInt();
  }

  @override
  void dispose() {
    _x.dispose();
    _y.dispose();
    _interval.dispose();
    _hold.dispose();
    super.dispose();
  }

  void _submit() {
    final x = int.tryParse(_x.text.trim());
    final y = int.tryParse(_y.text.trim());
    final interval = int.tryParse(_interval.text.trim());
    final hold = int.tryParse(_hold.text.trim());
    if (x == null || y == null || x < 0 || y < 0) {
      setState(() => _error = 'X、Y 必須是大於等於 0 的整數');
      return;
    }
    if (interval == null || hold == null || interval < 0 || hold < 0) {
      setState(() => _error = '間隔與按住時間必須是大於等於 0 的整數');
      return;
    }
    Navigator.of(context).pop(Step(
      x: x,
      y: y,
      button: _button,
      clicks: _clicks,
      intervalMs: interval,
      holdMs: hold,
    ));
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.initial == null ? '新增步驟' : '編輯步驟'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: _x,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(labelText: 'X 座標'),
            ),
            TextField(
              controller: _y,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(labelText: 'Y 座標'),
            ),
            DropdownButtonFormField<String>(
              initialValue: _button,
              decoration: const InputDecoration(labelText: '按鍵'),
              items: const [
                DropdownMenuItem(value: 'left', child: Text('左鍵')),
                DropdownMenuItem(value: 'right', child: Text('右鍵')),
                DropdownMenuItem(value: 'middle', child: Text('中鍵')),
              ],
              onChanged: (value) => setState(() => _button = value ?? 'left'),
            ),
            DropdownButtonFormField<int>(
              initialValue: _clicks,
              decoration: const InputDecoration(labelText: '點擊次數'),
              items: const [
                DropdownMenuItem(value: 1, child: Text('1')),
                DropdownMenuItem(value: 2, child: Text('2')),
                DropdownMenuItem(value: 3, child: Text('3')),
              ],
              onChanged: (value) => setState(() => _clicks = value ?? 1),
            ),
            TextField(
              controller: _interval,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(labelText: '間隔（毫秒，0 表示用預設）'),
            ),
            TextField(
              controller: _hold,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(labelText: '按住（毫秒）'),
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('取消')),
        FilledButton(onPressed: _submit, child: const Text('確定')),
      ],
    );
  }
}
