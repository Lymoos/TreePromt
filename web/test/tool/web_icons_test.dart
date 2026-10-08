// Генерирует favicon и иконки PWA из рисунка AppIcon (вариант C).
// Запуск: flutter test test/tool/web_icons_test.dart
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

/// [maskable]: иконка с запасом по краям — система сама обрежет её в круг или скруглённый квадрат.
Future<void> render(int size, String path, {bool maskable = false}) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  final s = size.toDouble();
  if (maskable) {
    canvas.drawRect(Rect.fromLTWH(0, 0, s, s), Paint()..color = const Color(0xFF111111));
    canvas.translate(s * 0.1, s * 0.1);
    AppIconPainter(tile: const Color(0xFF111111), inner: Colors.white).paint(canvas, Size.square(s * 0.8));
  } else {
    AppIconPainter(tile: const Color(0xFF111111), inner: Colors.white).paint(canvas, Size.square(s));
  }
  final image = await recorder.endRecording().toImage(size, size);
  final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
  File(path).writeAsBytesSync(bytes!.buffer.asUint8List());
}

void main() {
  testWidgets('generate web icons', (tester) async {
    await tester.runAsync(() async {
      await render(32, 'web/favicon.png');
      await render(192, 'web/icons/Icon-192.png');
      await render(512, 'web/icons/Icon-512.png');
      await render(192, 'web/icons/Icon-maskable-192.png', maskable: true);
      await render(512, 'web/icons/Icon-maskable-512.png', maskable: true);
    });
  });
}
