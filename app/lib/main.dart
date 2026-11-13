import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';

import 'engine/channel_click_engine.dart';
import 'engine/click_engine.dart';
import 'engine/ffi_click_engine.dart';
import 'model/script.dart';
import 'ui/home_page.dart';
import 'ui/theme.dart';

/// `click_spirit [script.json]` opens that script at launch on desktop.
Future<void> main(List<String> args) async {
  final initial = _scriptFromArgs(args);
  final ClickEngine engine;
  if (Platform.isAndroid) {
    WidgetsFlutterBinding.ensureInitialized();
    engine = ChannelClickEngine();
  } else {
    // Open the c-shared library before GTK. On Linux, Go's init calls
    // XInitThreads, which has to be the first Xlib call in the process.
    engine = await _desktopEngine();
    WidgetsFlutterBinding.ensureInitialized();
  }
  runApp(ClickSpiritApp(engine: engine, initialScript: initial));
}

Script? _scriptFromArgs(List<String> args) {
  for (final arg in args) {
    if (!arg.toLowerCase().endsWith('.json')) continue;
    try {
      final decoded = jsonDecode(File(arg).readAsStringSync());
      if (decoded is Map) return Script.fromJson(Map<String, dynamic>.from(decoded));
    } catch (e) {
      print('click-spirit: cannot open $arg: $e');
    }
  }
  return null;
}

Future<ClickEngine> _desktopEngine() async {
  try {
    final engine = FfiClickEngine();
    final name = await engine.backendName();
    print('click-spirit backend: $name');
    await stdout.flush();
    return engine;
  } catch (e) {
    print('click-spirit backend: unavailable $e');
    await stdout.flush();
    return UnavailableClickEngine(e);
  }
}

class ClickSpiritApp extends StatelessWidget {
  const ClickSpiritApp({super.key, required this.engine, this.initialScript});

  final ClickEngine engine;
  final Script? initialScript;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '點擊精靈',
      theme: ClickSpiritTheme.light(),
      darkTheme: ClickSpiritTheme.dark(),
      themeMode: ThemeMode.system,
      home: HomePage(
        engine: engine,
        enableCursorCapture: !Platform.isAndroid,
        enableAccessibility: Platform.isAndroid,
        initialScript: initialScript,
      ),
    );
  }
}
