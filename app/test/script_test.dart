import 'dart:convert';

import 'package:click_spirit/model/script.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('json round trip uses Go snake_case keys', () {
    final script = Script(
      version: 1,
      name: '示範',
      repeat: 0,
      defaultIntervalMs: 100,
      loopIntervalMs: 500,
      startDelayMs: 200,
      steps: [
        Step(x: 1, y: 2, button: 'left', clicks: 1, intervalMs: 10, holdMs: 0),
        Step(x: 3, y: 4, button: 'right', clicks: 2, intervalMs: 0, holdMs: 30),
      ],
    );
    final encoded = jsonEncode(script.toJson());
    expect(encoded.contains('default_interval_ms'), isTrue);
    expect(encoded.contains('loop_interval_ms'), isTrue);
    expect(encoded.contains('start_delay_ms'), isTrue);
    expect(encoded.contains('interval_ms'), isTrue);
    expect(encoded.contains('hold_ms'), isTrue);
    expect(encoded.contains('IntervalMs'), isFalse);

    final back = Script.fromJson(jsonDecode(encoded) as Map<String, dynamic>);
    expect(back.toJson(), script.toJson());
    expect(back.name, '示範');
    expect(back.repeat, 0);
    expect(back.steps.length, 2);
    expect(back.steps[1].button, 'right');
    expect(back.steps[1].holdMs, 30);
    expect(back.steps[1].intervalMs, 0);
  });

  test('sample matches the documented schema', () {
    final raw = jsonEncode(Script.sample().toJson());
    final back = Script.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    expect(back.name, '示範腳本');
    expect(back.version, 1);
    expect(back.steps.first.x, 100);
    expect(back.steps.first.button, 'left');
    expect(back.steps[1].clicks, 2);
    expect(back.defaultIntervalMs, 100);
    expect(back.loopIntervalMs, 500);
    expect(back.startDelayMs, 200);
  });

  test('missing numbers default to zero', () {
    final script = Script.fromJson({
      'name': '空白',
      'steps': [
        {'x': 1, 'y': 2, 'button': 'middle'},
      ],
    });
    expect(script.version, 0);
    expect(script.repeat, 0);
    expect(script.steps.single.clicks, 0);
    expect(script.steps.single.intervalMs, 0);
    expect(script.steps.single.holdMs, 0);
    expect(script.steps.single.repeat, 0);
    expect(script.steps.single.target, isNull);
    expect(script.emergencyHotkey, '');
  });

  test('old point-only JSON stays identical on re-save', () {
    const raw = '''
{
  "version": 1,
  "name": "示範腳本",
  "steps": [
    {"x": 100, "y": 200, "button": "left", "clicks": 1, "interval_ms": 50, "hold_ms": 0},
    {"x": 300, "y": 400, "button": "right", "clicks": 2, "interval_ms": 0, "hold_ms": 30}
  ],
  "repeat": 1,
  "default_interval_ms": 100,
  "loop_interval_ms": 500,
  "start_delay_ms": 200
}''';
    final original = jsonDecode(raw) as Map<String, dynamic>;
    final script = Script.fromJson(original);
    expect(script.steps.first.target, isNull);
    expect(script.steps.first.repeat, 0);
    expect(script.emergencyHotkey, '');
    final saved = script.toJson();
    expect(saved.containsKey('emergency_hotkey'), isFalse);
    final first = (saved['steps'] as List).first as Map;
    expect(first.containsKey('target'), isFalse);
    expect(first.containsKey('repeat'), isFalse);
    expect(jsonEncode(saved), jsonEncode(original));
    expect(jsonEncode(Script.sample().toJson()), jsonEncode(original));
  });

  test('circle and image targets round-trip', () {
    final script = Script(
      version: 1,
      name: '進階',
      repeat: 2,
      defaultIntervalMs: 10,
      loopIntervalMs: 0,
      startDelayMs: 0,
      emergencyHotkey: 'F12',
      steps: [
        Step(
          x: 10,
          y: 20,
          button: 'left',
          clicks: 1,
          intervalMs: 50,
          holdMs: 0,
          repeat: 2,
        ),
        Step(
          x: 0,
          y: 0,
          button: 'left',
          clicks: 1,
          intervalMs: 80,
          holdMs: 0,
          target: Target(kind: 'circle', cx: 400, cy: 300, r: 40),
        ),
        Step(
          x: 0,
          y: 0,
          button: 'middle',
          clicks: 1,
          intervalMs: 0,
          holdMs: 0,
          target: Target(
            kind: 'image',
            template: 'btn.png',
            threshold: 0.9,
            offsetX: -2,
            offsetY: 5,
            onNotFound: 'wait',
            timeoutMs: 3000,
            pollMs: 200,
            region: Region(x: 0, y: 0, w: 800, h: 600),
          ),
        ),
      ],
    );
    final encoded = script.toJson();
    expect(encoded['emergency_hotkey'], 'F12');
    final again = Script.fromJson(jsonDecode(jsonEncode(encoded)) as Map<String, dynamic>);
    expect(jsonEncode(again.toJson()), jsonEncode(encoded));

    expect(again.steps[0].repeat, 2);
    expect(again.steps[0].effectiveRepeat, 2);
    expect(again.steps[0].target, isNull);

    final circle = again.steps[1].target!;
    expect(circle.kind, 'circle');
    expect(circle.cx, 400);
    expect(circle.cy, 300);
    expect(circle.r, 40);
    expect(circle.toJson().containsKey('template'), isFalse);

    final image = again.steps[2].target!;
    expect(image.kind, 'image');
    expect(image.template, 'btn.png');
    expect(image.threshold, 0.9);
    expect(image.offsetX, -2);
    expect(image.offsetY, 5);
    expect(image.onNotFound, 'wait');
    expect(image.timeoutMs, 3000);
    expect(image.pollMs, 200);
    expect(image.region!.x, 0);
    expect(image.region!.w, 800);
    expect(image.region!.h, 600);
    expect(image.toJson()['offset_x'], -2);
    expect(image.toJson().containsKey('offset_y'), isTrue);
  });

  test('zero threshold and empty hotkey are omitted', () {
    final target = Target(kind: 'image', template: 'a.png', threshold: 0, onNotFound: '');
    expect(target.toJson().containsKey('threshold'), isFalse);
    expect(target.toJson().containsKey('on_not_found'), isFalse);
    expect(target.effectiveThreshold, 0.9);
    final script = Script(
      version: 1,
      name: 'x',
      steps: [
        Step(x: 1, y: 2, button: 'left', clicks: 1, intervalMs: 0, holdMs: 0, target: target),
      ],
      repeat: 0,
      defaultIntervalMs: 0,
      loopIntervalMs: 0,
      startDelayMs: 0,
    );
    expect(script.toJson().containsKey('emergency_hotkey'), isFalse);
  });
}
