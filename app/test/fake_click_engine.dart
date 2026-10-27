import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:click_spirit/engine/click_engine.dart';
import 'package:click_spirit/model/script.dart';

class FakeClickEngine implements ClickEngine {
  final StreamController<EngineEvent> _events = StreamController<EngineEvent>.broadcast();
  String validateResult = '';
  String startResult = '';
  String hotkeyResult = '';
  String startRecordingResult = '{"error":"input hooks are not implemented yet"}';
  String stopRecordingResult = '{"error":"not recording"}';
  String backend = 'fake';
  final List<String> hotkeys = [];
  final List<String> started = [];

  void emit(EngineEvent event) {
    if (!_events.isClosed) _events.add(event);
  }

  @override
  Stream<EngineEvent> get events => _events.stream;

  @override
  Future<String> version() async => 'test';

  @override
  Future<String> sampleScript() async => jsonEncode(Script.sample().toJson());

  @override
  Future<String> validate(String json) async => validateResult;

  @override
  Future<String> start(String json) async {
    started.add(json);
    return startResult;
  }

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
  Future<String> backendName() async => backend;

  @override
  Future<String> captureScreen(String path) async => '';

  @override
  Future<void> emergencyStop() async {
    emit(const EngineEvent(kind: 'emergency_stop', loop: 0, step: -1, message: '緊急停止'));
  }

  @override
  Future<String> setHotkey(String name) async {
    hotkeys.add(name);
    return hotkeyResult;
  }

  @override
  Future<String> startRecording() async => startRecordingResult;

  @override
  Future<String> stopRecording() async => stopRecordingResult;

  @override
  Future<String> matchImage(String templatePath, double threshold, {Uint8List? screenPng}) async {
    return '{"found":true,"x":8,"y":9,"score":0.91,"error":""}';
  }

  @override
  void dispose() {
    if (!_events.isClosed) _events.close();
  }
}
