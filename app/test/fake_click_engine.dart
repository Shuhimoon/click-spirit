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

  int stopCalls = 0;

  @override
  Future<void> stop() async => stopCalls++;

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

  final List<bool> showClicksCalls = [];
  bool overlay = true;

  @override
  Future<void> setShowClicks(bool on) async => showClicksCalls.add(on);

  @override
  Future<bool> hasClickOverlay() async => overlay;

  bool picker = false;
  PickOutcome pickResult = const PickOutcome.canceled();
  final List<String> pickModes = [];

  @override
  Future<bool> hasScreenPicker() async => picker;

  @override
  Future<PickOutcome> pickOnScreen(String mode) async {
    pickModes.add(mode);
    return pickResult;
  }

  bool bubble = false;
  final List<bool> bubbleVisibleCalls = [];
  bool video = true;
  String startScreenResult = '';
  final List<String> screenPaths = [];
  ScreenRecordingResult screenResult = const ScreenRecordingResult();
  final List<List<TimedClick>> converted = [];

  @override
  Future<bool> hasBubble() async => bubble;

  @override
  Future<void> setBubbleVisible(bool on) async => bubbleVisibleCalls.add(on);

  @override
  Future<bool> hasVideoRecorder() async => video;

  @override
  Future<String> startScreenRecording(String videoPath) async {
    screenPaths.add(videoPath);
    return startScreenResult;
  }

  @override
  Future<ScreenRecordingResult> stopScreenRecording() async => screenResult;

  @override
  Future<String> videoFrame(String video, int atMs, String out, {int width = 240}) async => 'no video in tests';

  /// Mirrors the Go conversion closely enough for widget tests.
  @override
  Future<String> clicksToScript(List<TimedClick> clicks, String name) async {
    converted.add(List.of(clicks));
    final steps = <Map<String, Object?>>[];
    for (var i = 0; i < clicks.length; i++) {
      final c = clicks[i];
      final gap = i + 1 < clicks.length ? clicks[i + 1].atMs - c.atMs - c.holdMs : 500;
      steps.add({
        'x': c.x,
        'y': c.y,
        'button': c.button,
        'clicks': 1,
        'interval_ms': gap < 1 ? 1 : gap,
        'hold_ms': c.holdMs,
      });
    }
    return jsonEncode({
      'version': 1,
      'name': name.isEmpty ? '錄影腳本' : name,
      'steps': steps,
      'repeat': 1,
      'default_interval_ms': 500,
      'start_delay_s': 3,
    });
  }

  @override
  void dispose() {
    if (!_events.isClosed) _events.close();
  }
}
