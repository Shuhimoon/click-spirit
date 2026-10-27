import 'package:click_spirit/engine/click_engine.dart';
import 'package:click_spirit/ui/home_page.dart';
import 'package:click_spirit/ui/step_dialog.dart';
import 'package:click_spirit/model/script.dart';
import 'package:flutter/material.dart' hide Step;
import 'package:flutter_test/flutter_test.dart';

import 'fake_click_engine.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpHome(WidgetTester tester, FakeClickEngine engine) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(MaterialApp(
      home: HomePage(
        engine: engine,
        enableCursorCapture: true,
        enableAccessibility: false,
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('step list numbers steps and reorder updates order', (tester) async {
    final engine = FakeClickEngine();
    await pumpHome(tester, engine);

    expect(find.text('步驟 1'), findsOneWidget);
    expect(find.text('步驟 2'), findsOneWidget);
    expect(
      tester.getTopLeft(find.byKey(const ValueKey('step-title-0'))).dy,
      lessThan(tester.getTopLeft(find.byKey(const ValueKey('step-title-1'))).dy),
    );
    expect(tester.widget<Text>(find.byKey(const ValueKey('step-summary-0'))).data, contains('100, 200'));
    expect(tester.widget<Text>(find.byKey(const ValueKey('step-summary-1'))).data, contains('300, 400'));

    await tester.ensureVisible(find.byKey(const ValueKey('drag-0')));
    await tester.pumpAndSettle();
    final gesture = await tester.startGesture(tester.getCenter(find.byKey(const ValueKey('drag-0'))));
    await tester.pump(const Duration(milliseconds: 50));
    await gesture.moveBy(const Offset(0, 40));
    await tester.pump();
    await gesture.moveBy(const Offset(0, 280));
    await tester.pump(const Duration(milliseconds: 300));
    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.text('步驟 1'), findsOneWidget);
    expect(find.text('步驟 2'), findsOneWidget);
    expect(tester.widget<Text>(find.byKey(const ValueKey('step-summary-0'))).data, contains('300, 400'));
    expect(tester.widget<Text>(find.byKey(const ValueKey('step-summary-1'))).data, contains('100, 200'));
    expect(
      tester.getTopLeft(find.byKey(const ValueKey('step-title-0'))).dy,
      lessThan(tester.getTopLeft(find.byKey(const ValueKey('step-title-1'))).dy),
    );
  });

  testWidgets('duplicate adds the next number', (tester) async {
    await pumpHome(tester, FakeClickEngine());
    await tester.ensureVisible(find.byTooltip('複製').first);
    await tester.tap(find.byTooltip('複製').first);
    await tester.pumpAndSettle();
    expect(find.text('步驟 3'), findsOneWidget);
    expect(tester.widget<Text>(find.byKey(const ValueKey('step-summary-1'))).data, contains('100, 200'));
  });

  testWidgets('running step shows repetition and emergency stop clears it', (tester) async {
    final engine = FakeClickEngine();
    await pumpHome(tester, engine);
    engine.emit(const EngineEvent(
      kind: 'step',
      loop: 0,
      step: 0,
      repetition: 0,
      message: '點擊 left (100,200) x1',
    ));
    // Broadcast streams deliver on a microtask. The first pump flushes that
    // task and schedules a frame; the second pump draws it.
    await tester.pump();
    await tester.pump();
    expect(find.text('第 1/1 次'), findsOneWidget);
    expect(find.textContaining('步驟 1，第 1/1 次'), findsOneWidget);

    engine.emit(const EngineEvent(
      kind: 'emergency_stop',
      loop: 0,
      step: -1,
      message: '緊急停止',
    ));
    await tester.pump();
    await tester.pump();
    expect(find.text('第 1/1 次'), findsNothing);
    expect(find.text('緊急停止'), findsWidgets);
  });

  testWidgets('edit dialog for circle saves values', (tester) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    Step? saved;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Builder(
          builder: (context) => TextButton(
            onPressed: () async {
              saved = await showStepEditor(context);
            },
            child: const Text('開啟'),
          ),
        ),
      ),
    ));
    await tester.tap(find.text('開啟'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('圓形'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('circle-cx')), '400');
    await tester.enterText(find.byKey(const Key('circle-cy')), '300');
    await tester.enterText(find.byKey(const Key('circle-r')), '40');
    await tester.tap(find.text('確定'));
    await tester.pumpAndSettle();

    expect(saved, isNotNull);
    expect(saved!.target, isNotNull);
    expect(saved!.target!.kind, 'circle');
    expect(saved!.target!.cx, 400);
    expect(saved!.target!.cy, 300);
    expect(saved!.target!.r, 40);
    expect(saved!.button, 'left');
    expect(saved!.toJson()['target'], {'kind': 'circle', 'cx': 400, 'cy': 300, 'r': 40});
  });
}
