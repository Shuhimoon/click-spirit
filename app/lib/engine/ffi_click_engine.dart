import 'dart:async';
import 'dart:convert';
import 'dart:ffi';
import 'dart:io';
import 'dart:typed_data';

import 'package:ffi/ffi.dart';

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

  void _pollOnce() {
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
  void dispose() {
    _timer.cancel();
    if (!_events.isClosed) _events.close();
  }
}
