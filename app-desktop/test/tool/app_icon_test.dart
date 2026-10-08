// Генерирует windows/runner/resources/app_icon.ico из рисунка AppIcon (вариант C).
// Запуск: flutter test test/tool/app_icon_test.dart
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

Future<Uint8List> png(int size) async {
  final recorder = ui.PictureRecorder();
  AppIconPainter(tile: const Color(0xFF111111), inner: Colors.white).paint(Canvas(recorder), Size.square(size.toDouble()));
  final image = await recorder.endRecording().toImage(size, size);
  return (await image.toByteData(format: ui.ImageByteFormat.png))!.buffer.asUint8List();
}

/// ICO с PNG внутри (поддерживается с Windows Vista).
Uint8List ico(List<(int, Uint8List)> images) {
  final header = BytesBuilder();
  final dir = ByteData(6)
    ..setUint16(0, 0, Endian.little)
    ..setUint16(2, 1, Endian.little)
    ..setUint16(4, images.length, Endian.little);
  header.add(dir.buffer.asUint8List());
  var offset = 6 + 16 * images.length;
  for (final (size, data) in images) {
    final e = ByteData(16)
      ..setUint8(0, size >= 256 ? 0 : size)
      ..setUint8(1, size >= 256 ? 0 : size)
      ..setUint16(4, 1, Endian.little)
      ..setUint16(6, 32, Endian.little)
      ..setUint32(8, data.length, Endian.little)
      ..setUint32(12, offset, Endian.little);
    header.add(e.buffer.asUint8List());
    offset += data.length;
  }
  for (final (_, data) in images) {
    header.add(data);
  }
  return header.toBytes();
}

void main() {
  testWidgets('generate app_icon.ico', (tester) async {
    await tester.runAsync(() async {
      final images = [for (final s in [16, 24, 32, 48, 64, 128, 256]) (s, await png(s))];
      File('windows/runner/resources/app_icon.ico').writeAsBytesSync(ico(images));
    });
  });
}
