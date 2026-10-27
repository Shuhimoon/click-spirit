import 'dart:async';

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
  void dispose() {}
}
