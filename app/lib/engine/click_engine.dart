import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import '../model/recording.dart';
import '../model/script.dart';

export '../model/recording.dart';

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
    this.r = 0,
    this.remaining = 0,
    this.mode = '',
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

  /// Radius for a circle `pick` event.
  final int r;

  /// Seconds left for a `countdown` event (N..1).
  final int remaining;

  /// `point` or `circle` for pick events.
  final String mode;

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
      r: _asInt(json['r']),
      remaining: _asInt(json['remaining']),
      mode: _asString(json['mode']),
    );
  }
}

/// Result of [ClickEngine.pickOnScreen]: a true screen coordinate (and a
/// radius in circle mode), a cancel, or an error message.
class PickOutcome {
  const PickOutcome({this.x = 0, this.y = 0, this.r = 0, this.canceled = false, this.error = ''});

  const PickOutcome.canceled() : this(canceled: true);

  const PickOutcome.failed(String message) : this(error: message);

  final int x;
  final int y;
  final int r;
  final bool canceled;
  final String error;

  bool get ok => !canceled && error.isEmpty;

  /// From a native `pick`, `pick_canceled`, or `pick_error` event.
  factory PickOutcome.fromEvent(EngineEvent ev) {
    switch (ev.kind) {
      case 'pick':
        return PickOutcome(x: ev.x, y: ev.y, r: ev.r);
      case 'pick_canceled':
        return const PickOutcome.canceled();
      default:
        return PickOutcome.failed(ev.message.isEmpty ? '選取失敗' : ev.message);
    }
  }

  /// From the Android method-channel reply `{ok, canceled, x, y, r, error}`.
  factory PickOutcome.fromMap(Map<Object?, Object?>? map) {
    if (map == null) return const PickOutcome.failed('沒有回應');
    if (map['canceled'] == true) return const PickOutcome.canceled();
    final err = map['error'];
    if (map['ok'] != true) {
      return PickOutcome.failed(err == null || err.toString().isEmpty ? '選取失敗' : err.toString());
    }
    return PickOutcome(x: _asInt(map['x']), y: _asInt(map['y']), r: _asInt(map['r']));
  }
}

/// True for the event kinds that end a native pick.
bool isPickEvent(String kind) => kind == 'pick' || kind == 'pick_canceled' || kind == 'pick_error';

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

  /// Whether a native on-screen picker exists (Linux X11 and Android). When
  /// false the step editor falls back to its in-app capture screen.
  Future<bool> hasScreenPicker();

  /// Hides the app, lets the user click a point (`point`) or drag a circle
  /// (`circle`) on the real screen, and returns true screen coordinates.
  Future<PickOutcome> pickOnScreen(String mode);

  /// Whether this platform has the always-on-top floating bubble (小浮窗):
  /// Linux X11 and Android. Its buttons arrive as `bubble` events with
  /// message `record`, `start`, or `stop`.
  Future<bool> hasBubble();

  /// Shows or hides the floating bubble.
  Future<void> setBubbleVisible(bool on);

  /// Whether a screen video backend exists (Linux with ffmpeg, Android).
  Future<bool> hasVideoRecorder();

  /// Starts 錄影轉腳本: clicks with timestamps plus a screen video saved to
  /// [videoPath] when possible. An empty return means success.
  Future<String> startScreenRecording(String videoPath);

  /// Stops 錄影轉腳本 and returns the video path, a note and the clicks.
  Future<ScreenRecordingResult> stopScreenRecording();

  /// Writes a PNG thumbnail of [video] at [atMs] to [out]. Empty means success.
  Future<String> videoFrame(String video, int atMs, String out, {int width = 240});

  /// Builds script JSON from reviewed clicks: one point step per click and
  /// the gaps as intervals. Returns `{"error":"..."}` on failure.
  Future<String> clicksToScript(List<TimedClick> clicks, String name);

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
  Future<bool> hasScreenPicker() async => false;

  @override
  Future<PickOutcome> pickOnScreen(String mode) async => PickOutcome.failed('原生函式庫無法載入：$error');

  @override
  Future<bool> hasBubble() async => false;

  @override
  Future<void> setBubbleVisible(bool on) async {}

  @override
  Future<bool> hasVideoRecorder() async => false;

  @override
  Future<String> startScreenRecording(String videoPath) async => '原生函式庫無法載入：$error';

  @override
  Future<ScreenRecordingResult> stopScreenRecording() async => ScreenRecordingResult(error: '原生函式庫無法載入：$error');

  @override
  Future<String> videoFrame(String video, int atMs, String out, {int width = 240}) async => '原生函式庫無法載入：$error';

  @override
  Future<String> clicksToScript(List<TimedClick> clicks, String name) async =>
      jsonEncode({'error': '原生函式庫無法載入：$error'});

  @override
  void dispose() {
    if (!_events.isClosed) _events.close();
  }
}
