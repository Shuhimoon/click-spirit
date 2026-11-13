import 'dart:convert';
import 'dart:io';

import 'package:path_provider/path_provider.dart';

/// User preferences that outlive a session.
class AppSettings {
  const AppSettings({this.showClicks = true});

  /// 顯示點擊位置: draw the run-time click overlay. On by default.
  final bool showClicks;

  AppSettings copyWith({bool? showClicks}) => AppSettings(showClicks: showClicks ?? this.showClicks);

  Map<String, Object?> toJson() => {'show_clicks': showClicks};

  factory AppSettings.fromJson(Map<String, dynamic> json) {
    final show = json['show_clicks'];
    return AppSettings(showClicks: show is bool ? show : true);
  }
}

/// Loads and saves [AppSettings]. Failures fall back to defaults.
abstract class SettingsStore {
  Future<AppSettings> load();
  Future<void> save(AppSettings settings);
}

/// Keeps settings in memory. Used by tests.
class MemorySettingsStore implements SettingsStore {
  MemorySettingsStore([this.value = const AppSettings()]);

  AppSettings value;
  int saves = 0;

  @override
  Future<AppSettings> load() async => value;

  @override
  Future<void> save(AppSettings settings) async {
    value = settings;
    saves++;
  }
}

/// Stores settings as `settings.json` in the app support directory.
class FileSettingsStore implements SettingsStore {
  FileSettingsStore({this.directory});

  /// Overrides the folder. Null uses path_provider's support directory.
  final Directory? directory;

  Future<File> _file() async {
    final dir = directory ?? await getApplicationSupportDirectory();
    return File('${dir.path}${Platform.pathSeparator}settings.json');
  }

  @override
  Future<AppSettings> load() async {
    try {
      final file = await _file();
      if (!await file.exists()) return const AppSettings();
      final decoded = jsonDecode(await file.readAsString());
      if (decoded is Map) return AppSettings.fromJson(Map<String, dynamic>.from(decoded));
    } catch (_) {}
    return const AppSettings();
  }

  @override
  Future<void> save(AppSettings settings) async {
    try {
      final file = await _file();
      await file.parent.create(recursive: true);
      await file.writeAsString(jsonEncode(settings.toJson()));
    } catch (_) {}
  }
}
