import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';

import 'click_engine.dart';

/// Android engine. Kotlin forwards calls to the gomobile AAR.
class ChannelClickEngine implements ClickEngine {
  ChannelClickEngine() {
    _events = _eventChannel.receiveBroadcastStream().map((raw) {
      if (raw is Map) {
        return EngineEvent.fromJson(Map<String, dynamic>.from(raw));
      }
      return const EngineEvent(kind: '', loop: 0, step: -1, message: '');
    });
  }

  static const MethodChannel _method = MethodChannel('click_spirit/engine');
  static const EventChannel _eventChannel = EventChannel('click_spirit/events');
  late final Stream<EngineEvent> _events;

  @override
  Stream<EngineEvent> get events => _events;

  Future<String> _stringCall(String method, [Map<String, Object?>? args]) async {
    try {
      final value = await _method.invokeMethod<String>(method, args);
      return value ?? '';
    } on PlatformException catch (e) {
      return e.message ?? e.code;
    }
  }

  @override
  Future<String> version() => _stringCall('version');

  @override
  Future<String> sampleScript() => _stringCall('sample');

  @override
  Future<String> validate(String json) => _stringCall('validate', {'json': json});

  @override
  Future<String> start(String json) => _stringCall('start', {'json': json});

  @override
  Future<void> stop() async {
    await _method.invokeMethod<void>('stop');
  }

  @override
  Future<bool> isRunning() async {
    final value = await _method.invokeMethod<bool>('isRunning');
    return value ?? false;
  }

  @override
  Future<CursorPos?> cursorPos() async => null;

  @override
  Future<bool> accessibilityTrusted() async {
    final value = await _method.invokeMethod<bool>('accessibilityTrusted');
    return value ?? false;
  }

  @override
  Future<void> openAccessibilitySettings() async {
    await _method.invokeMethod<void>('openAccessibilitySettings');
  }

  @override
  Future<String> backendName() async => 'android';

  @override
  Future<String> captureScreen(String path) async => '此平台尚未實作截圖';

  @override
  Future<void> emergencyStop() async {
    await _method.invokeMethod<void>('emergencyStop');
  }

  @override
  Future<String> setHotkey(String name) => _stringCall('setHotkey', {'name': name});

  @override
  Future<String> startRecording() => _stringCall('startRecording');

  @override
  Future<String> stopRecording() => _stringCall('stopRecording');

  @override
  Future<String> matchImage(String templatePath, double threshold, {Uint8List? screenPng}) async {
    Uint8List template;
    try {
      template = await File(templatePath).readAsBytes();
    } catch (e) {
      return matchErrorJson('無法讀取模板：$e');
    }
    final args = <String, Object?>{
      'template': template,
      'threshold': threshold,
    };
    if (screenPng != null) args['screen'] = screenPng;
    final raw = await _stringCall('matchImage', args);
    if (raw.isEmpty) return matchErrorJson('沒有回應');
    try {
      jsonDecode(raw);
      return raw;
    } catch (_) {
      return matchErrorJson(raw);
    }
  }

  @override
  Future<void> setShowClicks(bool on) async {
    try {
      await _method.invokeMethod<void>('setShowClicks', {'on': on});
    } on PlatformException {
      // An older native side without the overlay ignores the setting.
    }
  }

  @override
  Future<bool> hasClickOverlay() async {
    try {
      final value = await _method.invokeMethod<bool>('hasClickOverlay');
      return value ?? false;
    } on PlatformException {
      return false;
    } on MissingPluginException {
      return false;
    }
  }

  @override
  void dispose() {}
}
