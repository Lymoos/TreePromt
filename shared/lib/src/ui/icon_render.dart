import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';

import 'glyphs.dart';

/// Рендер иконки приложения в PNG для лаунчеров, магазинов и браузера.
/// Используется инструментами в test/tool приложений.
///
/// [bleed] — плитка на весь холст без полей; [radius] — скругление в долях стороны;
/// [inset] — прозрачный отступ в долях стороны (только macOS: сетка Apple);
/// [opaque] — PNG без альфа-канала (App Store не принимает иконки с прозрачностью).
Future<Uint8List> renderAppIcon(
  int size, {
  bool bleed = true,
  double radius = 0,
  double glyphScale = 0.85,
  double inset = 0,
  bool shadow = false,
  bool tileVisible = true,
  bool opaque = false,
}) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  final s = size.toDouble();
  final box = s * (1 - inset * 2);
  canvas.translate(s * inset, s * inset);
  if (shadow) {
    final r = RRect.fromRectAndRadius(Rect.fromLTWH(0, box * 0.012, box, box), Radius.circular(box * radius));
    canvas.drawRRect(
      r,
      Paint()
        ..color = const Color(0x55000000)
        ..maskFilter = MaskFilter.blur(BlurStyle.normal, box * 0.02),
    );
  }
  AppIconPainter(
    tile: tileVisible ? iconTile : const Color(0x00000000),
    inner: Colors.white,
    bleed: bleed,
    gradient: tileVisible,
    radius: radius,
    glyphScale: glyphScale,
  ).paint(canvas, Size.square(box));
  final image = await recorder.endRecording().toImage(size, size);
  if (!opaque) {
    final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
    return bytes!.buffer.asUint8List();
  }
  final rgba = (await image.toByteData(format: ui.ImageByteFormat.rawRgba))!.buffer.asUint8List();
  return encodeRgbPng(size, size, rgba);
}

Future<void> writeAppIcon(String path, int size,
    {bool bleed = true,
    double radius = 0,
    double glyphScale = 0.85,
    double inset = 0,
    bool shadow = false,
    bool tileVisible = true,
    bool opaque = false}) async {
  final png = await renderAppIcon(size,
      bleed: bleed,
      radius: radius,
      glyphScale: glyphScale,
      inset: inset,
      shadow: shadow,
      tileVisible: tileVisible,
      opaque: opaque);
  File(path)
    ..createSync(recursive: true)
    ..writeAsBytesSync(png);
}

/// Минимальный PNG-кодировщик: RGB 8 бит, без альфа-канала.
Uint8List encodeRgbPng(int width, int height, Uint8List rgba) {
  final raw = BytesBuilder();
  for (var y = 0; y < height; y++) {
    raw.addByte(0); // фильтр None
    for (var x = 0; x < width; x++) {
      final i = (y * width + x) * 4;
      raw.add([rgba[i], rgba[i + 1], rgba[i + 2]]);
    }
  }
  final out = BytesBuilder()..add([137, 80, 78, 71, 13, 10, 26, 10]);
  void chunk(String type, List<int> data) {
    final typeBytes = type.codeUnits;
    out.add((ByteData(4)..setUint32(0, data.length)).buffer.asUint8List());
    out.add(typeBytes);
    out.add(data);
    out.add((ByteData(4)..setUint32(0, _crc32([...typeBytes, ...data]))).buffer.asUint8List());
  }

  final ihdr = ByteData(13)
    ..setUint32(0, width)
    ..setUint32(4, height)
    ..setUint8(8, 8) // глубина
    ..setUint8(9, 2) // RGB
    ..setUint8(10, 0)
    ..setUint8(11, 0)
    ..setUint8(12, 0);
  chunk('IHDR', ihdr.buffer.asUint8List());
  chunk('IDAT', ZLibEncoder(level: 9).convert(raw.toBytes()));
  chunk('IEND', const []);
  return out.toBytes();
}

final _crcTable = List<int>.generate(256, (n) {
  var c = n;
  for (var k = 0; k < 8; k++) {
    c = (c & 1) != 0 ? 0xEDB88320 ^ (c >> 1) : c >> 1;
  }
  return c;
});

int _crc32(List<int> bytes) {
  var c = 0xFFFFFFFF;
  for (final b in bytes) {
    c = _crcTable[(c ^ b) & 0xFF] ^ (c >> 8);
  }
  return c ^ 0xFFFFFFFF;
}
