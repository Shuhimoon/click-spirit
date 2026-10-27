import 'dart:async';
import 'dart:convert';

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
    this.x = 0,
    this.y = 0,
    this.button = '',
    this.clicks = 0,
  });

  final String kind;
  final int loop;
  final int step;
  final String message;
  final int x;
  final int y;
  final String button;
  final int clicks;

  factory EngineEvent.fromJson(Map<String, dynamic> json) {
    return EngineEvent(
      kind: (json['kind'] ?? '') as String,
      loop: _asInt(json['loop']),
      step: _asInt(json['step']),
      message: (json['message'] ?? '') as String,
      x: _asInt(json['x']),
      y: _asInt(json['y']),
      button: (json['button'] ?? '') as String,
      clicks: _asInt(json['clicks']),
    );
  }
}

int _asInt(Object? value) {
  if (value == null) return 0;
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value.toString()) ?? 0;
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
  void dispose() {
    _events.close();
  }
}
