import 'package:click_spirit/engine/click_engine.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('step event keeps index and repetition', () {
    final ev = EngineEvent.fromJson({
      'kind': 'step',
      'loop': 1,
      'step': 2,
      'repetition': 1,
      'message': '點擊 left (3,4) x1 #2',
      'x': 3,
      'y': 4,
      'button': 'left',
      'clicks': 1,
    });
    expect(ev.kind, 'step');
    expect(ev.loop, 1);
    expect(ev.step, 2);
    expect(ev.repetition, 1);
    expect(ev.message, contains('#2'));
  });

  test('emergency_stop event', () {
    final ev = EngineEvent.fromJson({
      'kind': 'emergency_stop',
      'loop': 0,
      'step': -1,
      'message': '緊急停止',
    });
    expect(ev.kind, 'emergency_stop');
    expect(ev.step, -1);
    expect(ev.repetition, 0);
    expect(ev.message, '緊急停止');
  });

  test('match hit and recording payloads', () {
    final hit = MatchHit.parse('{"found":true,"x":12,"y":34,"score":0.95,"error":""}');
    expect(hit.found, isTrue);
    expect(hit.x, 12);
    expect(hit.y, 34);
    expect(hit.describe(), '找到 (12, 34)，分數 0.950');

    final miss = MatchHit.parse('{"found":false,"x":0,"y":0,"score":0.2,"error":""}');
    expect(miss.describe(), contains('未找到'));

    final failed = MatchHit.parse('{"found":false,"x":0,"y":0,"score":0,"error":"no screen"}');
    expect(failed.describe(), '比對失敗：no screen');

    expect(payloadError('{"ok":true}'), isNull);
    expect(payloadError('{"error":"input hooks are not implemented yet"}'), 'input hooks are not implemented yet');
    expect(payloadError('{"version":1,"steps":[]}'), isNull);
    expect(payloadError('plain error'), 'plain error');
  });
}