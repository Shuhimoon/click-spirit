import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import '../model/script.dart';

class CursorPos {
  const CursorPos(this.x, this.y);
  final int x;
  final int y;
}

class EngineEvent {
  const EngineEvent({
    required this.kind,
    required this.loop,
    required this.step,
    required this.message,
    this.repetition = 0,
    this.repeats = 0,
    this.x = 0,
    this.y = 0,
    this.button = '',
    this.clicks = 0,
  });

  final String kind;
  final int loop;

  /// Step index. -1 when the event is not tied to a step.
  final int step;

  /// Zero-based repetition inside the step. Missing JSON means 0.
  final int repetition;

  /// The step's repeat count for step and click events. 0 when unknown.
  final int repeats;
  final String message;
  final int x;
  final int y;
  final String button;
  final int clicks;

  factory EngineEvent.fromJson(Map<String, dynamic> json) {
    return EngineEvent(
      kind: _asString(json['kind']),
      loop: _asInt(json['loop']),
      step: _asInt(json['step']),
      repetition: _asInt(json['repetition']),
      repeats: _asInt(json['repeats']),
      message: _asString(json['message']),
      x: _asInt(json['x']),
      y: _asInt(json['y']),
      button: _asString(json['button']),
      clicks: _asInt(json['clicks']),
    );
  }
}

/// JSON from [ClickEngine.matchImage].
class MatchHit {
  const MatchHit({
    required this.found,
    required this.x,
    required this.y,
    required this.score,
    required this.error,
  });

  final bool found;
  final int x;
  final int y;
  final double score;
  final String error;

  factory MatchHit.parse(String raw) {
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map) {
        final err = decoded['error'];
        return MatchHit(
          found: decoded['found'] == true,
          x: _asInt(decoded['x']),
          y: _asInt(decoded['y']),
          score: _asDouble(decoded['score']),
          error: err == null ? '' : err.toString(),
        );
      }
    } catch (_) {}
    return MatchHit(found: false, x: 0, y: 0, score: 0, error: raw.isEmpty ? '沒有回應' : raw);
  }

  String describe() {
    if (error.isNotEmpty) return '比對失敗：$error';
    final scoreText = score.toStringAsFixed(3);
    if (found) return '找到 ($x, $y)，分數 $scoreText';
    return '未找到，分數 $scoreText';
  }
}

/// Error text from a recording payload.
///
/// `{"ok":true}` and a script object are success. `{"error":"..."}` and a
/// non-JSON string are failures. An empty string means the engine said nothing.
String? payloadError(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return '沒有回應';
  try {
    final decoded = jsonDecode(trimmed);
    if (decoded is Map) {
      final err = decoded['error'];
      if (err is String && err.isNotEmpty && decoded['steps'] == null) return err;
      if (err != null && err.toString().isNotEmpty && decoded['steps'] == null) {
        return err.toString();
      }
      return null;
    }
  } catch (_) {
    return trimmed;
  }
  return '無法辨識的回應';
}

int _asInt(Object? value) {
  if (value == null) return 0;
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value.toString()) ?? 0;
}

double _asDouble(Object? value) {
  if (value == null) return 0;
  if (value is double) return value;
  if (value is num) return value.toDouble();
  return double.tryParse(value.toString()) ?? 0;
}

String _asString(Object? value) {
  if (value == null) return '';
  if (value is String) return value;
  return value.toString();
}

String matchErrorJson(String message) {
  return jsonEncode({
    'found': false,
    'x': 0,
    'y': 0,
    'score': 0,
    'error': message,
  });
}

/// Desktop and Android share this surface. An empty string from [validate]
/// or [start] means success.
abstract class ClickEngine {
  Future<String> version();
  Future<String> sampleScript();
  Future<String> validate(String json);
  Future<String> start(String json);
  Future<void> stop();
  Future<bool> isRunning();
  Future<CursorPos?> cursorPos();
  Stream<EngineEvent> get events;
  Future<bool> accessibilityTrusted();
  Future<void> openAccessibilitySettings();

  /// Native backend name, such as x11, uinput, windows, darwin, or android.
  Future<String> backendName();

  /// Writes a PNG screenshot to [path]. An empty string means success.
  Future<String> captureScreen(String path);

  /// Stops the active script and emits an `emergency_stop` event.
  Future<void> emergencyStop();

  /// Arms [name] as the emergency chord. An empty string disarms it.
  /// An empty return means success. Any other string is an error.
  Future<String> setHotkey(String name);

  /// Starts pointer recording. Success is `{"ok":true}`.
  /// Failure is `{"error":"..."}` or a plain error string.
  Future<String> startRecording();

  /// Stops recording. Success is script JSON. Failure is `{"error":"..."}`.
  Future<String> stopRecording();

  /// Searches the screen for the PNG at [templatePath].
  ///
  /// Returns JSON `{"found","x","y","score","error"}`. Desktop captures the
  /// screen in the native library. Android needs PNG bytes: pass [screenPng]
  /// when a capture is available. [threshold] `<= 0` uses 0.9.
  Future<String> matchImage(String templatePath, double threshold, {Uint8List? screenPng});

  /// Turns the run-time click overlay on or off. The engine still reports
  /// `click` events either way.
  Future<void> setShowClicks(bool on);

  /// Whether this platform draws a click overlay above other apps (Linux X11
  /// and Android). Elsewhere the app shows clicks in its own window only.
  Future<bool> hasClickOverlay();

  void dispose();
}

/// Used when the desktop library cannot be loaded. Script editing still works.
class UnavailableClickEngine implements ClickEngine {
  UnavailableClickEngine(this.error);
  final Object error;

  final StreamController<EngineEvent> _events = StreamController<EngineEvent>.broadcast();

  @override
  Stream<EngineEvent> get events => _events.stream;

  @override
  Future<String> version() async => '';

  @override
  Future<String> sampleScript() async => jsonEncode(Script.sample().toJson());

  @override
  Future<String> validate(String json) async => '';

  @override
  Future<String> start(String json) async => '原生函式庫無法載入：$error';

  @override
  Future<void> stop() async {}

  @override
  Future<bool> isRunning() async => false;

  @override
  Future<CursorPos?> cursorPos() async => null;

  @override
  Future<bool> accessibilityTrusted() async => false;

  @override
  Future<void> openAccessibilitySettings() async {}

  @override
  Future<String> backendName() async => '';

  @override
  Future<String> captureScreen(String path) async => '原生函式庫無法載入：$error';

  @override
  Future<void> emergencyStop() async {}

  @override
  Future<String> setHotkey(String name) async => '原生函式庫無法載入：$error';

  @override
  Future<String> startRecording() async => jsonEncode({'error': '原生函式庫無法載入：$error'});

  @override
  Future<String> stopRecording() async => jsonEncode({'error': '原生函式庫無法載入：$error'});

  @override
  Future<String> matchImage(String templatePath, double threshold, {Uint8List? screenPng}) async {
    return matchErrorJson('原生函式庫無法載入：$error');
  }

  @override
  Future<void> setShowClicks(bool on) async {}

  @override
  Future<bool> hasClickOverlay() async => false;

  @override
  void dispose() {
    if (!_events.isClosed) _events.close();
  }
}
