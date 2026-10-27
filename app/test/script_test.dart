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
  });
}
