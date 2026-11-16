import 'dart:convert';

import 'package:click_spirit/engine/click_engine.dart';
import 'package:click_spirit/model/app_settings.dart';
import 'package:click_spirit/model/script.dart';
import 'package:click_spirit/model/script_library.dart';
import 'package:click_spirit/ui/home_page.dart';
import 'package:click_spirit/ui/video_review_page.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:flutter_test/flutter_test.dart';

import 'fake_click_engine.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  void bigView(WidgetTester tester) {
    tester.view.physicalSize = const Size(1280, 1000);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  const recorded =
      '{"version":1,"name":"recorded","steps":[{"x":11,"y":22,"button":"left","clicks":1,"interval_ms":300,"hold_ms":40}],"repeat":1,"default_interval_ms":500,"start_delay_s":3}';

  Future<void> bubble(WidgetTester tester, FakeClickEngine engine, String action) async {
    engine.emit(EngineEvent(kind: 'bubble', loop: 0, step: -1, message: action));
    await tester.pump();
    await tester.pump();
    await tester.pumpAndSettle();
  }

  Future<(FakeClickEngine, MemoryScriptLibrary, MemorySettingsStore)> home(WidgetTester tester) async {
    bigView(tester);
    final engine = FakeClickEngine()
      ..bubble = true
      ..startRecordingResult = '{"ok":true}'
      ..stopRecordingResult = recorded;
    final library = MemoryScriptLibrary();
    final settings = MemorySettingsStore();
    await tester.pumpWidget(MaterialApp(
      home: HomePage(
        engine: engine,
        enableCursorCapture: true,
        enableAccessibility: false,
        settings: settings,
        library: library,
      ),
    ));
    await tester.pumpAndSettle();
    return (engine, library, settings);
  }

  testWidgets('bubble 開始錄製 twice saves a timestamped script into the library', (tester) async {
    final (engine, library, _) = await home(tester);
    expect(find.text('還沒有腳本。錄製的腳本會自動存進來。'), findsOneWidget);

    await bubble(tester, engine, 'record');
    expect(find.text('錄製中'), findsOneWidget);
    await bubble(tester, engine, 'record');

    final entries = await library.list();
    expect(entries, hasLength(1));
    expect(entries.single.name, startsWith('錄製 '));
    expect(RegExp(r'^錄製 \d{4}-\d\d-\d\d \d\d:\d\d:\d\d$').hasMatch(entries.single.name), isTrue);
    expect(entries.single.steps, 1);
    expect(find.byKey(Key('library-row-${entries.single.name}')), findsOneWidget);
  });

  testWidgets('設為預設 is persisted and the bubble 開始腳本 runs that script; 停止腳本 stops', (tester) async {
    final (engine, library, settings) = await home(tester);
    final other = Script.sample()..name = '預設的那個';
    await library.save(other);
    // Reload the list through the UI button path.
    await tester.tap(find.byKey(const Key('library-save')));
    await tester.pumpAndSettle();

    await bubble(tester, engine, 'start');
    expect(engine.started, isEmpty, reason: 'no default yet');
    expect(find.textContaining('尚未設定預設腳本'), findsOneWidget);

    final star = find.byKey(const Key('library-default-預設的那個'));
    await tester.ensureVisible(star);
    await tester.tap(star);
    await tester.pumpAndSettle();
    final id = settings.value.defaultScript;
    expect(id, isNotEmpty);
    expect((await library.load(id))!.name, '預設的那個');

    await bubble(tester, engine, 'start');
    expect(engine.started, hasLength(1));
    final started = jsonDecode(engine.started.single) as Map;
    expect(started['name'], '預設的那個');
    expect(started['start_delay_s'], 3, reason: 'the start delay still applies');

    await bubble(tester, engine, 'stop');
    expect(engine.stopCalls, 1);
  });

  testWidgets('顯示小浮窗 switch shows the bubble and is remembered', (tester) async {
    final (engine, _, settings) = await home(tester);
    final sw = find.byKey(const Key('show-bubble-switch'));
    await tester.ensureVisible(sw);
    await tester.tap(sw);
    await tester.pumpAndSettle();
    expect(engine.bubbleVisibleCalls, [true]);
    expect(settings.value.showBubble, isTrue);
    expect(AppSettings.fromJson(jsonDecode(jsonEncode(settings.value.toJson())) as Map<String, dynamic>).showBubble,
        isTrue);
  });

  testWidgets('錄影轉腳本: review lists clicks with timecodes, delete, save', (tester) async {
    final (engine, library, _) = await home(tester);
    engine.screenResult = const ScreenRecordingResult(
      video: '',
      note: '此平台沒有錄影後端',
      screenW: 1600,
      screenH: 1000,
      clicks: [
        TimedClick(x: 100, y: 200, atMs: 1200, holdMs: 50),
        TimedClick(x: 300, y: 400, atMs: 2500, holdMs: 50),
        TimedClick(x: 500, y: 600, atMs: 4000, holdMs: 50),
      ],
    );
    // _startVideo creates the recordings folder with real file I/O.
    await tester.runAsync(() async {
      await tester.tap(find.byKey(const Key('video-record')));
      for (var i = 0; i < 50 && engine.screenPaths.isEmpty; i++) {
        await Future<void>.delayed(const Duration(milliseconds: 20));
      }
    });
    await tester.pumpAndSettle();
    expect(engine.screenPaths, hasLength(1));
    expect(find.text('停止錄影'), findsOneWidget);

    await tester.tap(find.byKey(const Key('video-record')));
    await tester.pumpAndSettle();
    expect(find.byType(VideoReviewPage), findsOneWidget);
    expect(find.text('00:01.2'), findsOneWidget);
    expect(find.text('00:02.5'), findsOneWidget);
    expect(find.text('00:04.0'), findsOneWidget);
    expect(find.textContaining('間隔 1250 ms'), findsOneWidget);

    await tester.tap(find.byKey(const Key('review-delete-2')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('review-row-2')), findsNothing);
    expect(find.textContaining('間隔 2750 ms'), findsOneWidget);

    await tester.tap(find.byKey(const Key('review-save')));
    await tester.pumpAndSettle();
    expect(find.byType(VideoReviewPage), findsNothing);
    expect(engine.converted.single.map((c) => '${c.x},${c.y}'), ['100,200', '500,600']);
    final entries = await library.list();
    expect(entries.single.name, startsWith('錄影 '));
    expect(entries.single.steps, 2);
  });

  test('ScreenRecordingResult parses the native JSON', () {
    final r = ScreenRecordingResult.parse(
        '{"video":"/tmp/a.mp4","clicks":[{"x":1,"y":2,"button":"left","at_ms":30,"hold_ms":5}],"screen_w":10,"screen_h":20}');
    expect(r.hasVideo, isTrue);
    expect(r.clicks.single.atMs, 30);
    expect(r.screenW, 10);
    expect(timecode(61234), '01:01.2');
  });

  test('dropStopClick removes only the press on 停止錄影', () {
    const r = ScreenRecordingResult(clicks: [
      TimedClick(x: 1, y: 1, atMs: 1000),
      TimedClick(x: 2, y: 2, atMs: 4000),
      TimedClick(x: 3, y: 3, atMs: 5850),
    ]);
    final out = dropStopClick(r, 6000);
    expect(out.clicks.map((c) => c.x), [1, 2]);
    expect(out.note, contains('停止錄影'));
    expect(dropStopClick(r, 9000).clicks, hasLength(3));
    expect(dropStopClick(r, 100).clicks, hasLength(3), reason: 'clock mismatch keeps all');
  });
}
