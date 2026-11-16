import 'dart:convert';

/// One click or tap from a recording, timed from the start of the video.
class TimedClick {
  const TimedClick({required this.x, required this.y, this.button = 'left', required this.atMs, this.holdMs = 0});

  final int x;
  final int y;
  final String button;

  /// Milliseconds from the start of the recording (video timecode).
  final int atMs;

  /// How long the button or finger was held.
  final int holdMs;

  factory TimedClick.fromJson(Map<String, dynamic> json) => TimedClick(
        x: _int(json['x']),
        y: _int(json['y']),
        button: (json['button'] ?? 'left').toString(),
        atMs: _int(json['at_ms']),
        holdMs: _int(json['hold_ms']),
      );

  Map<String, Object?> toJson() => {'x': x, 'y': y, 'button': button, 'at_ms': atMs, 'hold_ms': holdMs};
}

/// Result of stopping 錄影轉腳本: the video (empty when only clicks were
/// recorded), a note explaining why, and the clicks.
class ScreenRecordingResult {
  const ScreenRecordingResult({
    this.video = '',
    this.note = '',
    this.clicks = const [],
    this.error = '',
    this.screenW = 0,
    this.screenH = 0,
  });

  final String video;
  final String note;
  final List<TimedClick> clicks;
  final String error;

  /// Recorded screen size in pixels; 0 when unknown.
  final int screenW;
  final int screenH;

  bool get hasVideo => video.isNotEmpty;

  factory ScreenRecordingResult.parse(String raw) {
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map) {
        final list = decoded['clicks'];
        return ScreenRecordingResult(
          video: (decoded['video'] ?? '').toString(),
          note: (decoded['note'] ?? '').toString(),
          error: (decoded['error'] ?? '').toString(),
          screenW: _int(decoded['screen_w']),
          screenH: _int(decoded['screen_h']),
          clicks: list is List
              ? list.whereType<Map>().map((m) => TimedClick.fromJson(Map<String, dynamic>.from(m))).toList()
              : const [],
        );
      }
    } catch (_) {}
    return ScreenRecordingResult(error: raw.trim().isEmpty ? '沒有回應' : raw);
  }
}

/// `mm:ss.d` timecode for the review screen.
String timecode(int ms) {
  final m = ms ~/ 60000;
  final s = (ms % 60000) ~/ 1000;
  final d = (ms % 1000) ~/ 100;
  return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}.$d';
}

int _int(Object? v) {
  if (v is int) return v;
  if (v is num) return v.toInt();
  return int.tryParse('$v') ?? 0;
}

/// The press on 「停止錄影」 (in the app, or the replayed tap on Android) is
/// recorded too. Drops trailing clicks made within [windowMs] before the
/// stop at [stopMs] (milliseconds since the recording started).
ScreenRecordingResult dropStopClick(ScreenRecordingResult r, int stopMs, {int windowMs = 800}) {
  final kept = List.of(r.clicks);
  var dropped = 0;
  // A click "after" the stop means the clocks disagree: keep everything.
  while (kept.isNotEmpty && kept.last.atMs >= stopMs - windowMs && kept.last.atMs <= stopMs + 200) {
    kept.removeLast();
    dropped++;
  }
  if (dropped == 0) return r;
  final note = '已自動去掉按「停止錄影」的那一下';
  return ScreenRecordingResult(
    video: r.video,
    note: r.note.isEmpty ? note : '${r.note}；$note',
    clicks: kept,
    error: r.error,
    screenW: r.screenW,
    screenH: r.screenH,
  );
}
