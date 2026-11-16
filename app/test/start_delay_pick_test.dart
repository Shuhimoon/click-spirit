import 'dart:convert';

import 'package:click_spirit/engine/click_engine.dart';
import 'package:click_spirit/model/app_settings.dart';
import 'package:click_spirit/model/script.dart';
import 'package:click_spirit/ui/home_page.dart';
import 'package:click_spirit/ui/step_dialog.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:flutter_test/flutter_test.dart';

import 'fake_click_engine.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  void bigView(WidgetTester tester) {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  group('start delay (start_delay_s)', () {
    test('missing field means 3 seconds, explicit 0 is kept', () {
      final old = Script.fromJson(
          jsonDecode('{"version":1,"name":"舊","steps":[{"x":1,"y":2,"button":"left"}],"start_delay_ms":200}')
              as Map<String, dynamic>);
      expect(old.startDelaySec, isNull);
      expect(old.effectiveStartDelaySec, 3);
      expect(old.toJson().containsKey('start_delay_s'), isFalse);
      expect((old..startDelaySec = old.effectiveStartDelaySec).toJson()['start_delay_s'], 3);
      expect(old.toJson()['start_delay_ms'], 200);

      final zero = Script.fromJson(jsonDecode('{"version":1,"steps":[],"start_delay_s":0}') as Map<String, dynamic>);
      expect(zero.effectiveStartDelaySec, 0);
      expect(Script.sample().effectiveStartDelaySec, 3);
    });

    testWidgets('home shows the countdown and 停止 cancels it', (tester) async {
      bigView(tester);
      final engine = FakeClickEngine();
      await tester.pumpWidget(MaterialApp(
        home: HomePage(
            engine: engine, enableCursorCapture: true, enableAccessibility: false, settings: MemorySettingsStore()),
      ));
      await tester.pumpAndSettle();
      expect(find.text('幾秒後開始此腳本（0 = 立即）'), findsOneWidget);
      expect(find.byKey(const Key('countdown-card')), findsNothing);

      engine.emit(const EngineEvent(kind: 'started', loop: 0, step: -1, message: ''));
      engine.emit(const EngineEvent(kind: 'countdown', loop: 0, step: -1, message: '3 秒後開始', remaining: 3));
      await tester.pump();
      await tester.pump();
      expect(find.byKey(const Key('countdown-card')), findsOneWidget);
      expect(tester.widget<Text>(find.byKey(const Key('countdown-number'))).data, '3');

      engine.emit(const EngineEvent(kind: 'countdown', loop: 0, step: -1, message: '2 秒後開始', remaining: 2));
      await tester.pump();
      await tester.pump();
      expect(tester.widget<Text>(find.byKey(const Key('countdown-number'))).data, '2');

      await tester.tap(find.byKey(const Key('countdown-stop')));
      await tester.pumpAndSettle();
      expect(engine.stopCalls, 1);
      expect(find.byKey(const Key('countdown-card')), findsNothing);
    });

    testWidgets('the delay field is saved into the started script', (tester) async {
      bigView(tester);
      final engine = FakeClickEngine();
      await tester.pumpWidget(MaterialApp(
        home: HomePage(
            engine: engine, enableCursorCapture: true, enableAccessibility: false, settings: MemorySettingsStore()),
      ));
      await tester.pumpAndSettle();
      final field = find.byKey(const Key('start-delay'));
      await tester.ensureVisible(field);
      await tester.enterText(field, '5');
      await tester.tap(find.text('開始'));
      await tester.pumpAndSettle();
      expect(engine.started, isNotEmpty);
      expect((jsonDecode(engine.started.last) as Map)['start_delay_s'], 5);
    });
  });

  group('on-screen picker', () {
    Future<Step?> openEditor(WidgetTester tester, FakeClickEngine engine, {String? kind}) async {
      bigView(tester);
      Step? saved;
      var done = false;
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: Builder(
            builder: (context) => TextButton(
              onPressed: () async {
                saved = await showStepEditor(context, engine: engine);
                done = true;
              },
              child: const Text('開啟'),
            ),
          ),
        ),
      ));
      await tester.tap(find.text('開啟'));
      await tester.pumpAndSettle();
      if (kind != null) {
        await tester.tap(find.text(kind));
        await tester.pumpAndSettle();
      }
      addTearDown(() => expect(done || saved == null, isTrue));
      return saved;
    }

    testWidgets('從螢幕點選座標 fills X/Y from the native picker', (tester) async {
      final engine = FakeClickEngine()
        ..picker = true
        ..pickResult = const PickOutcome(x: 1234, y: 567);
      await openEditor(tester, engine);
      await tester.tap(find.text('從螢幕點選座標'));
      await tester.pumpAndSettle();
      expect(engine.pickModes, ['point']);
      expect(tester.widget<TextField>(find.byKey(Key('point-x'))).controller!.text, '1234');
      expect(tester.widget<TextField>(find.byKey(Key('point-y'))).controller!.text, '567');
    });

    testWidgets('從螢幕擷取設定圓形 fills centre and radius; cancel keeps values', (tester) async {
      final engine = FakeClickEngine()
        ..picker = true
        ..pickResult = const PickOutcome(x: 200, y: 210, r: 50);
      await openEditor(tester, engine, kind: '圓形');
      await tester.tap(find.text('從螢幕擷取設定圓形'));
      await tester.pumpAndSettle();
      String text(String key) => tester.widget<TextField>(find.byKey(Key(key))).controller!.text;
      expect(engine.pickModes, ['circle']);
      expect([text('circle-cx'), text('circle-cy'), text('circle-r')], ['200', '210', '50']);

      engine.pickResult = const PickOutcome.canceled();
      await tester.tap(find.text('從螢幕擷取設定圓形'));
      await tester.pumpAndSettle();
      expect([text('circle-cx'), text('circle-cy'), text('circle-r')], ['200', '210', '50']);
    });

    test('PickOutcome parses native events and Android replies', () {
      final ok =
          PickOutcome.fromEvent(const EngineEvent(kind: 'pick', loop: 0, step: -1, message: '', x: 5, y: 6, r: 7));
      expect([ok.ok, ok.x, ok.y, ok.r], [true, 5, 6, 7]);
      expect(PickOutcome.fromEvent(const EngineEvent(kind: 'pick_canceled', loop: 0, step: -1, message: '')).canceled,
          isTrue);
      expect(
          PickOutcome.fromEvent(const EngineEvent(kind: 'pick_error', loop: 0, step: -1, message: 'no display')).error,
          'no display');
      final a = PickOutcome.fromMap({'ok': true, 'x': 1080, 'y': 2000, 'r': 0});
      expect([a.ok, a.x, a.y], [true, 1080, 2000]);
      expect(PickOutcome.fromMap({'canceled': true}).canceled, isTrue);
      expect(PickOutcome.fromMap(null).ok, isFalse);
    });
  });
}
