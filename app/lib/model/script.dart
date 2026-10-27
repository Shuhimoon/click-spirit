/// Dart mirror of the Go script JSON schema.
/// Keys are snake_case and match `core.Script`, `core.Step`, and `core.Target`.
///
/// Optional Go fields use `omitempty`: zero numbers and empty strings are left
/// out so an old point-only script is unchanged when it is loaded and saved.
class Region {
  Region({required this.x, required this.y, required this.w, required this.h});

  int x;
  int y;
  int w;
  int h;

  Region copy() => Region(x: x, y: y, w: w, h: h);

  Map<String, dynamic> toJson() => {'x': x, 'y': y, 'w': w, 'h': h};

  factory Region.fromJson(Map<String, dynamic> json) {
    return Region(
      x: _asInt(json['x']),
      y: _asInt(json['y']),
      w: _asInt(json['w']),
      h: _asInt(json['h']),
    );
  }
}

/// Click location. A null target on a step is a point at the step's x, y.
class Target {
  Target({
    required this.kind,
    this.cx = 0,
    this.cy = 0,
    this.r = 0,
    this.template = '',
    this.templateB64 = '',
    this.threshold = 0,
    this.region,
    this.offsetX = 0,
    this.offsetY = 0,
    this.onNotFound = '',
    this.timeoutMs = 0,
    this.pollMs = 0,
  });

  String kind;
  int cx;
  int cy;
  int r;
  String template;
  String templateB64;
  double threshold;
  Region? region;
  int offsetX;
  int offsetY;
  String onNotFound;
  int timeoutMs;
  int pollMs;

  /// Empty kind is a point, matching [Target.KindOrPoint] in Go.
  String get kindOrPoint => kind.isEmpty ? 'point' : kind;

  /// Zero means the Go default of 0.9.
  double get effectiveThreshold => threshold <= 0 ? 0.9 : threshold;

  Target copy() => Target(
        kind: kind,
        cx: cx,
        cy: cy,
        r: r,
        template: template,
        templateB64: templateB64,
        threshold: threshold,
        region: region?.copy(),
        offsetX: offsetX,
        offsetY: offsetY,
        onNotFound: onNotFound,
        timeoutMs: timeoutMs,
        pollMs: pollMs,
      );

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{'kind': kind.isEmpty ? 'point' : kind};
    if (cx != 0) map['cx'] = cx;
    if (cy != 0) map['cy'] = cy;
    if (r != 0) map['r'] = r;
    if (template.isNotEmpty) map['template'] = template;
    if (templateB64.isNotEmpty) map['template_b64'] = templateB64;
    if (threshold != 0) map['threshold'] = threshold;
    final reg = region;
    if (reg != null) map['region'] = reg.toJson();
    if (offsetX != 0) map['offset_x'] = offsetX;
    if (offsetY != 0) map['offset_y'] = offsetY;
    if (onNotFound.isNotEmpty) map['on_not_found'] = onNotFound;
    if (timeoutMs != 0) map['timeout_ms'] = timeoutMs;
    if (pollMs != 0) map['poll_ms'] = pollMs;
    return map;
  }

  factory Target.fromJson(Map<String, dynamic> json) {
    final rawRegion = json['region'];
    return Target(
      kind: _asString(json['kind']),
      cx: _asInt(json['cx']),
      cy: _asInt(json['cy']),
      r: _asInt(json['r']),
      template: _asString(json['template']),
      templateB64: _asString(json['template_b64']),
      threshold: _asDouble(json['threshold']),
      region: rawRegion is Map ? Region.fromJson(Map<String, dynamic>.from(rawRegion)) : null,
      offsetX: _asInt(json['offset_x']),
      offsetY: _asInt(json['offset_y']),
      onNotFound: _asString(json['on_not_found']),
      timeoutMs: _asInt(json['timeout_ms']),
      pollMs: _asInt(json['poll_ms']),
    );
  }
}

class Step {
  Step({
    required this.x,
    required this.y,
    required this.button,
    required this.clicks,
    required this.intervalMs,
    required this.holdMs,
    this.repeat = 0,
    this.target,
  });

  int x;
  int y;
  String button;
  int clicks;
  int intervalMs;
  int holdMs;
  int repeat;
  Target? target;

  /// Zero means one click.
  int get effectiveClicks => clicks == 0 ? 1 : clicks;

  /// Zero and one both run the step once.
  int get effectiveRepeat => repeat <= 1 ? 1 : repeat;

  String get kindOrPoint => target?.kindOrPoint ?? 'point';

  Step copy() => Step(
        x: x,
        y: y,
        button: button,
        clicks: clicks,
        intervalMs: intervalMs,
        holdMs: holdMs,
        repeat: repeat,
        target: target?.copy(),
      );

  Step copyWith({
    int? x,
    int? y,
    String? button,
    int? clicks,
    int? intervalMs,
    int? holdMs,
    int? repeat,
    Target? target,
    bool clearTarget = false,
  }) {
    return Step(
      x: x ?? this.x,
      y: y ?? this.y,
      button: button ?? this.button,
      clicks: clicks ?? this.clicks,
      intervalMs: intervalMs ?? this.intervalMs,
      holdMs: holdMs ?? this.holdMs,
      repeat: repeat ?? this.repeat,
      target: clearTarget ? null : (target ?? this.target?.copy()),
    );
  }

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{
      'x': x,
      'y': y,
      'button': button,
      'clicks': clicks,
      'interval_ms': intervalMs,
      'hold_ms': holdMs,
    };
    if (repeat != 0) map['repeat'] = repeat;
    final t = target;
    if (t != null) map['target'] = t.toJson();
    return map;
  }

  factory Step.fromJson(Map<String, dynamic> json) {
    final rawTarget = json['target'];
    return Step(
      x: _asInt(json['x']),
      y: _asInt(json['y']),
      button: _asString(json['button']),
      clicks: _asInt(json['clicks']),
      intervalMs: _asInt(json['interval_ms']),
      holdMs: _asInt(json['hold_ms']),
      repeat: _asInt(json['repeat']),
      target: rawTarget is Map ? Target.fromJson(Map<String, dynamic>.from(rawTarget)) : null,
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
    this.emergencyHotkey = '',
  });

  int version;
  String name;
  List<Step> steps;
  int repeat;
  int defaultIntervalMs;
  int loopIntervalMs;
  int startDelayMs;
  String emergencyHotkey;

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

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{
      'version': version,
      'name': name,
      'steps': steps.map((s) => s.toJson()).toList(),
      'repeat': repeat,
      'default_interval_ms': defaultIntervalMs,
      'loop_interval_ms': loopIntervalMs,
      'start_delay_ms': startDelayMs,
    };
    if (emergencyHotkey.isNotEmpty) map['emergency_hotkey'] = emergencyHotkey;
    return map;
  }

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
      name: _asString(json['name']),
      steps: steps,
      repeat: _asInt(json['repeat']),
      defaultIntervalMs: _asInt(json['default_interval_ms']),
      loopIntervalMs: _asInt(json['loop_interval_ms']),
      startDelayMs: _asInt(json['start_delay_ms']),
      emergencyHotkey: _asString(json['emergency_hotkey']),
    );
  }
}

int _asInt(Object? value) {
  if (value == null) return 0;
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value.toString()) ?? 0;
}

double _asDouble(Object? value) {
  if (value == null) return 0;
  if (value is double) return value;
  if (value is num) return value.toDouble();
  return double.tryParse(value.toString()) ?? 0;
}

String _asString(Object? value) {
  if (value == null) return '';
  if (value is String) return value;
  return value.toString();
}
