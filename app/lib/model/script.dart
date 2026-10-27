/// Dart mirror of the Go script JSON schema.
/// Keys are snake_case and match `core.Script` / `core.Step` exactly.
class Step {
  Step({
    required this.x,
    required this.y,
    required this.button,
    required this.clicks,
    required this.intervalMs,
    required this.holdMs,
  });

  int x;
  int y;
  String button;
  int clicks;
  int intervalMs;
  int holdMs;

  Step copyWith({
    int? x,
    int? y,
    String? button,
    int? clicks,
    int? intervalMs,
    int? holdMs,
  }) {
    return Step(
      x: x ?? this.x,
      y: y ?? this.y,
      button: button ?? this.button,
      clicks: clicks ?? this.clicks,
      intervalMs: intervalMs ?? this.intervalMs,
      holdMs: holdMs ?? this.holdMs,
    );
  }

  Map<String, dynamic> toJson() => {
        'x': x,
        'y': y,
        'button': button,
        'clicks': clicks,
        'interval_ms': intervalMs,
        'hold_ms': holdMs,
      };

  factory Step.fromJson(Map<String, dynamic> json) {
    return Step(
      x: _asInt(json['x']),
      y: _asInt(json['y']),
      button: (json['button'] ?? '') as String,
      clicks: _asInt(json['clicks']),
      intervalMs: _asInt(json['interval_ms']),
      holdMs: _asInt(json['hold_ms']),
    );
  }
}

class Script {
  Script({
    required this.version,
    required this.name,
    required this.steps,
    required this.repeat,
    required this.defaultIntervalMs,
    required this.loopIntervalMs,
    required this.startDelayMs,
  });

  int version;
  String name;
  List<Step> steps;
  int repeat;
  int defaultIntervalMs;
  int loopIntervalMs;
  int startDelayMs;

  /// Matches [core.NewSample] in the Go module.
  factory Script.sample() {
    return Script(
      version: 1,
      name: '示範腳本',
      repeat: 1,
      defaultIntervalMs: 100,
      loopIntervalMs: 500,
      startDelayMs: 200,
      steps: [
        Step(x: 100, y: 200, button: 'left', clicks: 1, intervalMs: 50, holdMs: 0),
        Step(x: 300, y: 400, button: 'right', clicks: 2, intervalMs: 0, holdMs: 30),
      ],
    );
  }

  Map<String, dynamic> toJson() => {
        'version': version,
        'name': name,
        'steps': steps.map((s) => s.toJson()).toList(),
        'repeat': repeat,
        'default_interval_ms': defaultIntervalMs,
        'loop_interval_ms': loopIntervalMs,
        'start_delay_ms': startDelayMs,
      };

  factory Script.fromJson(Map<String, dynamic> json) {
    final rawSteps = json['steps'];
    final steps = <Step>[];
    if (rawSteps is List) {
      for (final item in rawSteps) {
        if (item is Map) {
          steps.add(Step.fromJson(Map<String, dynamic>.from(item)));
        }
      }
    }
    return Script(
      version: _asInt(json['version']),
      name: (json['name'] ?? '') as String,
      steps: steps,
      repeat: _asInt(json['repeat']),
      defaultIntervalMs: _asInt(json['default_interval_ms']),
      loopIntervalMs: _asInt(json['loop_interval_ms']),
      startDelayMs: _asInt(json['start_delay_ms']),
    );
  }
}

int _asInt(Object? value) {
  if (value == null) return 0;
  if (value is int) return value;
  if (value is num) return value.toInt();
  throw FormatException('expected int, got $value');
}
