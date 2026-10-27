import 'dart:io';

import 'package:flutter/material.dart';

import 'engine/channel_click_engine.dart';
import 'engine/click_engine.dart';
import 'engine/ffi_click_engine.dart';
import 'ui/home_page.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  final ClickEngine engine;
  if (Platform.isAndroid) {
    engine = ChannelClickEngine();
  } else {
    try {
      engine = FfiClickEngine();
    } catch (e) {
      engine = UnavailableClickEngine(e);
    }
  }
  runApp(ClickSpiritApp(engine: engine));
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
