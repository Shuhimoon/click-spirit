import 'dart:io';

import 'package:click_spirit/engine/click_engine.dart';
import 'package:click_spirit/model/app_settings.dart';
import 'package:click_spirit/model/script.dart';
import 'package:click_spirit/ui/home_page.dart';
import 'package:click_spirit/ui/markers.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:flutter_test/flutter_test.dart';

import 'fake_click_engine.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('markers', () {
    test('numbers follow sequence order and skip image steps', () {
      final steps = <Step>[
        Step(x: 10, y: 20, button: 'left', clicks: 1, intervalMs: 0, holdMs: 0),
        Step(
          x: 0,
          y: 0,
          button: 'left',
          clicks: 1,
          intervalMs: 0,
          holdMs: 0,
          target: Target(kind: 'image', template: 'a.png'),
        ),
        Step(
          x: 0,
          y: 0,
          button: 'left',
          clicks: 1,
          intervalMs: 0,
          holdMs: 0,
          target: Target(kind: 'circle', cx: 400, cy: 300, r: 50),
        ),
      ];
      final markers = markersOf(steps);
      expect(markers, hasLength(2));
      expect(markers[0].number, 1);
      expect(markers[0].isCircle, isFalse);
      expect(markers[0].center, const Offset(10, 20));
      expect(markers[1].number, 3);
      expect(markers[1].isCircle, isTrue);
      expect(markers[1].center, const Offset(400, 300));
      expect(markers[1].radius, 50);
    });

    test('screen extent grows to fit far points', () {
      expect(screenExtent(const []), const Size(1280, 720));
      final big = screenExtent([const StepMarker.circle(1, Offset(1900, 1000), 40)]);
      expect(big.width, greaterThanOrEqualTo(1940));
      expect(big.height, greaterThanOrEqualTo(1040));
    });

    test('click label matches the native overlay caption', () {
      expect(clickLabel(1, 2, 10), '步驟 2 · 第 3/10 次');
      expect(clickLabel(0, 0, 1), '步驟 1');
    });
  });

  test('click event carries position, step and repeats', () {
    final ev = EngineEvent.fromJson({
      'kind': 'click',
      'loop': 0,
      'step': 1,
      'repetition': 2,
      'repeats': 10,
      'x': 412,
      'y': 288,
      'message': '步驟 2 · 第 3/10 次 點在 (412,288)',
    });
    expect(ev.kind, 'click');
    expect(ev.step, 1);
    expect(ev.repetition, 2);
    expect(ev.repeats, 10);
    expect(ev.x, 412);
    expect(ev.y, 288);
  });

  test('file settings store persists show_clicks', () async {
    final dir = await Directory.systemTemp.createTemp('cs_settings');
    addTearDown(() => dir.delete(recursive: true));
    final store = FileSettingsStore(directory: dir);
    expect((await store.load()).showClicks, isTrue);
    await store.save(const AppSettings(showClicks: false));
    expect((await FileSettingsStore(directory: dir).load()).showClicks, isFalse);
  });

  Future<void> pumpHome(WidgetTester tester, FakeClickEngine engine, SettingsStore store) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(MaterialApp(
      home: HomePage(
        engine: engine,
        enableCursorCapture: true,
        enableAccessibility: false,
        settings: store,
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('顯示點擊位置 loads, toggles and persists', (tester) async {
    final engine = FakeClickEngine();
    final store = MemorySettingsStore(const AppSettings(showClicks: false));
    await pumpHome(tester, engine, store);

    final switchFinder = find.byKey(const Key('show-clicks-switch'));
    await tester.ensureVisible(switchFinder);
    expect(find.text('顯示點擊位置'), findsOneWidget);
    expect(tester.widget<Switch>(switchFinder).value, isFalse);
    expect(engine.showClicksCalls, [false]);

    await tester.tap(switchFinder);
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(switchFinder).value, isTrue);
    expect(engine.showClicksCalls, [false, true]);
    expect(store.value.showClicks, isTrue);
    expect(store.saves, 1);
  });

  testWidgets('click events update status and the in-app map', (tester) async {
    final engine = FakeClickEngine();
    await pumpHome(tester, engine, MemorySettingsStore());
    expect(find.byKey(const Key('step-map')), findsOneWidget);

    engine.emit(const EngineEvent(kind: 'started', loop: 0, step: -1, message: '開始執行'));
    engine.emit(const EngineEvent(
      kind: 'click',
      loop: 0,
      step: 1,
      repetition: 2,
      repeats: 10,
      x: 305,
      y: 398,
      message: '',
    ));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 700));
    final status = tester.widget<Text>(find.byKey(const Key('status-text'))).data!;
    expect(status, contains('步驟 2 · 第 3/10 次'));
    expect(status, contains('(305, 398)'));
    final map = tester.widget<StepMapPreview>(find.byType(StepMapPreview));
    expect(map.lastClick?.x, 305);
    expect(map.lastClick?.y, 398);
    expect(map.highlight, 1);
    expect(map.markers.map((m) => m.number), [1, 2]);
  });

  testWidgets('turning 顯示點擊位置 off hides the in-app click mark', (tester) async {
    final engine = FakeClickEngine();
    await pumpHome(tester, engine, MemorySettingsStore(const AppSettings(showClicks: false)));
    engine.emit(const EngineEvent(kind: 'click', loop: 0, step: 0, x: 1, y: 2, repeats: 1, message: ''));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 700));
    expect(tester.widget<StepMapPreview>(find.byType(StepMapPreview)).lastClick, isNull);
  });
}
