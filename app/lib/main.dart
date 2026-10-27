import 'dart:io';

import 'package:flutter/material.dart';

import 'engine/channel_click_engine.dart';
import 'engine/click_engine.dart';
import 'engine/ffi_click_engine.dart';
import 'ui/home_page.dart';

Future<void> main() async {
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
  runApp(ClickSpiritApp(engine: engine));
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
  const ClickSpiritApp({super.key, required this.engine});

  final ClickEngine engine;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '點擊精靈',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0F6B4C)),
        useMaterial3: true,
      ),
      home: HomePage(
        engine: engine,
        enableCursorCapture: !Platform.isAndroid,
        enableAccessibility: Platform.isAndroid,
      ),
    );
  }
}
