// Генерирует иконки лаунчера Android из того же рисунка, что AppIcon (вариант C).
// Запуск: flutter test test/tool/launcher_icons_test.dart
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

Future<void> renderIcon(int size, String path) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  // Белое поле с отступом, как требуют гайды Android для legacy-иконок.
  canvas.drawRect(Rect.fromLTWH(0, 0, size.toDouble(), size.toDouble()), Paint()..color = Colors.white);
  final inset = size * 0.08;
  canvas.translate(inset, inset);
  AppIconPainter(tile: const Color(0xFF111111), inner: Colors.white)
      .paint(canvas, Size.square(size - inset * 2));
  final image = await recorder.endRecording().toImage(size, size);
  final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
  File(path)
    ..createSync(recursive: true)
    ..writeAsBytesSync(bytes!.buffer.asUint8List());
}

void main() {
  testWidgets('generate launcher icons', (tester) async {
    await tester.runAsync(() async {
      const sizes = {'mdpi': 48, 'hdpi': 72, 'xhdpi': 96, 'xxhdpi': 144, 'xxxhdpi': 192};
      for (final e in sizes.entries) {
        await renderIcon(e.value, 'android/app/src/main/res/mipmap-${e.key}/ic_launcher.png');
      }
      await renderIcon(1024, 'ios/Runner/Assets.xcassets/AppIcon.appiconset/Icon-App-1024x1024@1x.png');
    });
  });
}
