import 'dart:convert';
import 'dart:io';

import 'package:path_provider/path_provider.dart';

import 'script.dart';

/// One saved script in the library. [id] is stable (the file name).
class LibraryEntry {
  const LibraryEntry({required this.id, required this.name, required this.modified, this.steps = 0});

  final String id;
  final String name;
  final DateTime modified;
  final int steps;
}

/// The app's script list (腳本庫). The floating bubble saves recordings here
/// and runs the default script from here.
abstract class ScriptLibrary {
  /// Newest first.
  Future<List<LibraryEntry>> list();

  /// Saves [script]; overwrites [id] when given. Returns the entry id.
  Future<String> save(Script script, {String? id});

  Future<Script?> load(String id);

  Future<void> delete(String id);
}

/// `錄製 2026-11-16 21:50:03` style names for recordings.
String timestampName(String prefix, DateTime t) {
  String two(int v) => v.toString().padLeft(2, '0');
  return '$prefix ${t.year}-${two(t.month)}-${two(t.day)} ${two(t.hour)}:${two(t.minute)}:${two(t.second)}';
}

/// File-safe stem for a script name.
String safeFileStem(String name) {
  final cleaned = name.trim().replaceAll(RegExp(r'[\\/:*?"<>|\x00-\x1f]'), '_');
  return cleaned.isEmpty ? '未命名' : cleaned;
}

/// In-memory library for tests.
class MemoryScriptLibrary implements ScriptLibrary {
  final Map<String, Script> scripts = {};
  final Map<String, DateTime> times = {};
  int _serial = 0;

  @override
  Future<List<LibraryEntry>> list() async {
    final ids = scripts.keys.toList()..sort((a, b) => times[b]!.compareTo(times[a]!));
    return [
      for (final id in ids)
        LibraryEntry(id: id, name: scripts[id]!.name, modified: times[id]!, steps: scripts[id]!.steps.length),
    ];
  }

  @override
  Future<String> save(Script script, {String? id}) async {
    final key = id ?? '${safeFileStem(script.name)}#${_serial++}.json';
    scripts[key] = _clone(script);
    times[key] = DateTime.now().add(Duration(microseconds: _serial++));
    return key;
  }

  @override
  Future<Script?> load(String id) async {
    final s = scripts[id];
    return s == null ? null : _clone(s);
  }

  @override
  Future<void> delete(String id) async {
    scripts.remove(id);
    times.remove(id);
  }
}

Script _clone(Script s) => Script.fromJson(jsonDecode(jsonEncode(s.toJson())) as Map<String, dynamic>);

/// Stores each script as `<support>/scripts/<name>.json`.
class FileScriptLibrary implements ScriptLibrary {
  FileScriptLibrary({this.directory});

  /// Overrides the folder. Null uses `<app support>/scripts`.
  final Directory? directory;

  Future<Directory> _dir() async {
    final base =
        directory ?? Directory('${(await getApplicationSupportDirectory()).path}${Platform.pathSeparator}scripts');
    await base.create(recursive: true);
    return base;
  }

  @override
  Future<List<LibraryEntry>> list() async {
    final out = <LibraryEntry>[];
    try {
      final dir = await _dir();
      await for (final f in dir.list()) {
        if (f is! File || !f.path.endsWith('.json')) continue;
        try {
          final decoded = jsonDecode(await f.readAsString());
          if (decoded is! Map) continue;
          final s = Script.fromJson(Map<String, dynamic>.from(decoded));
          out.add(LibraryEntry(
            id: f.uri.pathSegments.last,
            name: s.name,
            modified: await f.lastModified(),
            steps: s.steps.length,
          ));
        } catch (_) {}
      }
    } catch (_) {}
    out.sort((a, b) => b.modified.compareTo(a.modified));
    return out;
  }

  @override
  Future<String> save(Script script, {String? id}) async {
    final dir = await _dir();
    var name = id;
    if (name == null) {
      final stem = safeFileStem(script.name);
      name = '$stem.json';
      for (var i = 2; await File('${dir.path}${Platform.pathSeparator}$name').exists(); i++) {
        name = '$stem-$i.json';
      }
    }
    final text = '${const JsonEncoder.withIndent('  ').convert(script.toJson())}\n';
    await File('${dir.path}${Platform.pathSeparator}$name').writeAsString(text);
    return name!;
  }

  @override
  Future<Script?> load(String id) async {
    try {
      final dir = await _dir();
      final decoded = jsonDecode(await File('${dir.path}${Platform.pathSeparator}$id').readAsString());
      if (decoded is Map) return Script.fromJson(Map<String, dynamic>.from(decoded));
    } catch (_) {}
    return null;
  }

  @override
  Future<void> delete(String id) async {
    try {
      final dir = await _dir();
      await File('${dir.path}${Platform.pathSeparator}$id').delete();
    } catch (_) {}
  }
}
