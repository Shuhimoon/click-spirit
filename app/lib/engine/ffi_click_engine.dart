import 'dart:async';
import 'dart:convert';
import 'dart:ffi';
import 'dart:io';

import 'package:ffi/ffi.dart';
import 'package:flutter/services.dart';

import 'click_engine.dart';

typedef _StrNative = Pointer<Utf8> Function();
typedef _StrDart = Pointer<Utf8> Function();
typedef _StrInNative = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _StrInDart = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _FreeNative = Void Function(Pointer<Utf8>);
typedef _FreeDart = void Function(Pointer<Utf8>);
typedef _VoidNative = Void Function();
typedef _VoidDart = void Function();
typedef _IntNative = Int32 Function();
typedef _IntDart = int Function();
typedef _CursorNative = Int32 Function(Pointer<Int32>, Pointer<Int32>);
typedef _CursorDart = int Function(Pointer<Int32>, Pointer<Int32>);
typedef _MatchNative = Pointer<Utf8> Function(Pointer<Utf8>, Double);
typedef _MatchDart = Pointer<Utf8> Function(Pointer<Utf8>, double);
typedef _IntInNative = Void Function(Int32);
typedef _IntInDart = void Function(int);
typedef _FrameNative = Pointer<Utf8> Function(Pointer<Utf8>, Int64, Pointer<Utf8>, Int32);
typedef _FrameDart = Pointer<Utf8> Function(Pointer<Utf8>, int, Pointer<Utf8>, int);
typedef _Str2Native = Pointer<Utf8> Function(Pointer<Utf8>, Pointer<Utf8>);
typedef _Str2Dart = Pointer<Utf8> Function(Pointer<Utf8>, Pointer<Utf8>);

/// Loads click_spirit.dll, libclick_spirit.dylib, or libclick_spirit.so
/// and polls [CS_PollEvent]. On Linux the shared library is the bundle's
/// lib/libclick_spirit.so, next to the executable.
class FfiClickEngine implements ClickEngine {
  FfiClickEngine() : _lib = _openLibrary() {
    _version = _lib.lookupFunction<_StrNative, _StrDart>('CS_Version');
    _sample = _lib.lookupFunction<_StrNative, _StrDart>('CS_SampleScript');
    _validate = _lib.lookupFunction<_StrInNative, _StrInDart>('CS_Validate');
    _start = _lib.lookupFunction<_StrInNative, _StrInDart>('CS_Start');
    _stop = _lib.lookupFunction<_VoidNative, _VoidDart>('CS_Stop');
    _isRunning = _lib.lookupFunction<_IntNative, _IntDart>('CS_IsRunning');
    _cursor = _lib.lookupFunction<_CursorNative, _CursorDart>('CS_CursorPos');
    _poll = _lib.lookupFunction<_StrNative, _StrDart>('CS_PollEvent');
    _trusted = _lib.lookupFunction<_IntNative, _IntDart>('CS_AccessibilityTrusted');
    _backend = _lib.lookupFunction<_StrNative, _StrDart>('CS_Backend');
    _capture = _lib.lookupFunction<_StrInNative, _StrInDart>('CS_CaptureScreenPNG');
    _emergency = _lib.lookupFunction<_VoidNative, _VoidDart>('CS_EmergencyStop');
    _setHotkey = _lib.lookupFunction<_StrInNative, _StrInDart>('CS_SetHotkey');
    _startRecording = _lib.lookupFunction<_StrNative, _StrDart>('CS_StartRecording');
    _stopRecording = _lib.lookupFunction<_StrNative, _StrDart>('CS_StopRecording');
    _match = _lib.lookupFunction<_MatchNative, _MatchDart>('CS_MatchImage');
    _free = _lib.lookupFunction<_FreeNative, _FreeDart>('CS_FreeString');
    _setShowClicks = _lib.lookupFunction<_IntInNative, _IntInDart>('CS_SetShowClicks');
    _hasOverlay = _lib.lookupFunction<_IntNative, _IntDart>('CS_HasClickOverlay');
    _startPick = _lib.lookupFunction<_StrInNative, _StrInDart>('CS_StartPick');
    _hasPicker = _lib.lookupFunction<_IntNative, _IntDart>('CS_HasScreenPicker');
    _hasBubble = _lib.lookupFunction<_IntNative, _IntDart>('CS_HasBubble');
    _setBubble = _lib.lookupFunction<_IntInNative, _IntInDart>('CS_SetBubbleVisible');
    _hasVideo = _lib.lookupFunction<_IntNative, _IntDart>('CS_HasVideoRecorder');
    _startScreenRec = _lib.lookupFunction<_StrInNative, _StrInDart>('CS_StartScreenRecording');
    _stopScreenRec = _lib.lookupFunction<_StrNative, _StrDart>('CS_StopScreenRecording');
    _videoFrame = _lib.lookupFunction<_FrameNative, _FrameDart>('CS_VideoFrame');
    _clicksToScript = _lib.lookupFunction<_Str2Native, _Str2Dart>('CS_ClicksToScript');
    _timer = Timer.periodic(const Duration(milliseconds: 100), (_) => _pollOnce());
  }

  final DynamicLibrary _lib;
  late final _StrDart _version;
  late final _StrDart _sample;
  late final _StrInDart _validate;
  late final _StrInDart _start;
  late final _VoidDart _stop;
  late final _IntDart _isRunning;
  late final _CursorDart _cursor;
  late final _StrDart _poll;
  late final _IntDart _trusted;
  late final _StrDart _backend;
  late final _StrInDart _capture;
  late final _VoidDart _emergency;
  late final _StrInDart _setHotkey;
  late final _StrDart _startRecording;
  late final _StrDart _stopRecording;
  late final _MatchDart _match;
  late final _FreeDart _free;
  late final _IntInDart _setShowClicks;
  late final _IntDart _hasBubble;
  late final _IntInDart _setBubble;
  late final _IntDart _hasVideo;
  late final _StrInDart _startScreenRec;
  late final _StrDart _stopScreenRec;
  late final _FrameDart _videoFrame;
  late final _Str2Dart _clicksToScript;
  late final _IntDart _hasOverlay;
  late final _StrInDart _startPick;
  late final _IntDart _hasPicker;
  static const MethodChannel _window = MethodChannel('click_spirit/window');
  late final Timer _timer;
  final StreamController<EngineEvent> _events = StreamController<EngineEvent>.broadcast();

  static DynamicLibrary _openLibrary() {
    final candidates = <String>[];
    if (Platform.isWindows) {
      final dir = File(Platform.resolvedExecutable).parent.path;
      candidates.add(_join(dir, 'click_spirit.dll'));
      candidates.add('click_spirit.dll');
    } else if (Platform.isMacOS) {
      final exe = File(Platform.resolvedExecutable);
      final contents = exe.parent.parent.path;
      candidates.add(_join(_join(contents, 'Frameworks'), 'libclick_spirit.dylib'));
      candidates.add('libclick_spirit.dylib');
    } else if (Platform.isLinux) {
      final dir = File(Platform.resolvedExecutable).parent.path;
      candidates.add(_join(_join(dir, 'lib'), 'libclick_spirit.so'));
      candidates.add(_join(dir, 'libclick_spirit.so'));
      candidates.add('libclick_spirit.so');
    } else {
      throw UnsupportedError('不支援的桌面平台 ${Platform.operatingSystem}');
    }
    Object? last;
    for (final path in candidates) {
      try {
        return DynamicLibrary.open(path);
      } catch (e) {
        last = e;
      }
    }
    throw StateError('找不到點擊精靈原生函式庫（試過 ${candidates.join(', ')}）：$last');
  }

  static String _join(String dir, String name) {
    if (dir.endsWith('/') || dir.endsWith(r'\')) return '$dir$name';
    final sep = Platform.isWindows ? r'\' : '/';
    return '$dir$sep$name';
  }

  String _take(Pointer<Utf8> p) {
    if (p == nullptr) return '';
    final value = p.toDartString();
    _free(p);
    return value;
  }

  String _callIn(Pointer<Utf8> Function(Pointer<Utf8>) fn, String json) {
    final native = json.toNativeUtf8();
    try {
      return _take(fn(native));
    } finally {
      calloc.free(native);
    }
  }

  /// Drains up to 64 queued events per tick. Every click now produces a
  /// `step` and a `click` event, so one event per tick would fall behind.
  void _pollOnce() {
    for (var i = 0; i < 64; i++) {
      if (_events.isClosed) return;
      try {
        final raw = _take(_poll());
        if (raw.isEmpty) return;
        final decoded = jsonDecode(raw);
        if (decoded is Map) {
          _events.add(EngineEvent.fromJson(Map<String, dynamic>.from(decoded)));
        }
      } catch (e, st) {
        if (!_events.isClosed) {
          _events.addError(e, st);
        }
      }
    }
  }

  @override
  Stream<EngineEvent> get events => _events.stream;

  @override
  Future<String> version() async => _take(_version());

  @override
  Future<String> sampleScript() async => _take(_sample());

  @override
  Future<String> validate(String json) async => _callIn(_validate, json);

  @override
  Future<String> start(String json) async => _callIn(_start, json);

  @override
  Future<void> stop() async => _stop();

  @override
  Future<bool> isRunning() async => _isRunning() != 0;

  @override
  Future<CursorPos?> cursorPos() async {
    final x = calloc<Int32>();
    final y = calloc<Int32>();
    try {
      final ok = _cursor(x, y);
      if (ok == 0) return null;
      return CursorPos(x.value, y.value);
    } finally {
      calloc.free(x);
      calloc.free(y);
    }
  }

  @override
  Future<bool> accessibilityTrusted() async => _trusted() != 0;

  @override
  Future<void> openAccessibilitySettings() async {}

  @override
  Future<String> backendName() async => _take(_backend());

  @override
  Future<String> captureScreen(String path) async => _callIn(_capture, path);

  @override
  Future<void> emergencyStop() async => _emergency();

  @override
  Future<String> setHotkey(String name) async => _callIn(_setHotkey, name);

  @override
  Future<String> startRecording() async => _take(_startRecording());

  @override
  Future<String> stopRecording() async => _take(_stopRecording());

  @override
  Future<String> matchImage(String templatePath, double threshold, {Uint8List? screenPng}) async {
    // CS_MatchImage captures the screen itself. screenPng is the Android path.
    final native = templatePath.toNativeUtf8();
    try {
      return _take(_match(native, threshold));
    } finally {
      calloc.free(native);
    }
  }

  @override
  Future<void> setShowClicks(bool on) async => _setShowClicks(on ? 1 : 0);

  @override
  Future<bool> hasClickOverlay() async => _hasOverlay() != 0;

  @override
  Future<bool> hasScreenPicker() async => _hasPicker() != 0;

  /// Hides the window (Linux runner channel), waits for the compositor to
  /// drop it from the screen, starts the native picker, and waits for its
  /// single result event. The window comes back whatever happens.
  @override
  Future<PickOutcome> pickOnScreen(String mode) async {
    final result = events.firstWhere((e) => isPickEvent(e.kind)).timeout(const Duration(minutes: 3));
    var hidden = false;
    try {
      hidden = await _window.invokeMethod<bool>('hide') ?? false;
    } catch (_) {
      // No window channel on this runner: pick with the window visible.
    }
    try {
      if (hidden) await Future<void>.delayed(const Duration(milliseconds: 400));
      final err = _callIn(_startPick, mode);
      if (err.isNotEmpty) {
        result.ignore();
        return PickOutcome.failed(err);
      }
      return PickOutcome.fromEvent(await result);
    } on TimeoutException {
      return const PickOutcome.failed('選取逾時');
    } finally {
      if (hidden) {
        try {
          await _window.invokeMethod<bool>('show');
        } catch (_) {}
      }
    }
  }

  @override
  Future<bool> hasBubble() async => _hasBubble() != 0;

  @override
  Future<void> setBubbleVisible(bool on) async => _setBubble(on ? 1 : 0);

  @override
  Future<bool> hasVideoRecorder() async => _hasVideo() != 0;

  @override
  Future<String> startScreenRecording(String videoPath) async => _callIn(_startScreenRec, videoPath);

  @override
  Future<ScreenRecordingResult> stopScreenRecording() async => ScreenRecordingResult.parse(_take(_stopScreenRec()));

  @override
  Future<String> videoFrame(String video, int atMs, String out, {int width = 240}) async {
    final v = video.toNativeUtf8();
    final o = out.toNativeUtf8();
    try {
      return _take(_videoFrame(v, atMs, o, width));
    } finally {
      calloc.free(v);
      calloc.free(o);
    }
  }

  @override
  Future<String> clicksToScript(List<TimedClick> clicks, String name) async {
    final c = jsonEncode(clicks.map((e) => e.toJson()).toList()).toNativeUtf8();
    final n = name.toNativeUtf8();
    try {
      final out = _take(_clicksToScript(c, n));
      return out.startsWith('error: ') ? jsonEncode({'error': out.substring(7)}) : out;
    } finally {
      calloc.free(c);
      calloc.free(n);
    }
  }

  @override
  void dispose() {
    _timer.cancel();
    if (!_events.isClosed) _events.close();
  }
}
