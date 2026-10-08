// Генерирует иконки ПК из рисунка AppIcon (вариант C):
// windows/runner/resources/app_icon.ico и AppIcon.appiconset для macOS.
// Запуск: flutter test test/tool/app_icon_test.dart
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/icon_tool.dart';

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
  testWidgets('generate desktop icons', (tester) async {
    await tester.runAsync(() async {
      // Windows: скруглённая плитка на всю площадь, без полей.
      final images = [
        for (final s in [16, 24, 32, 48, 64, 128, 256])
          (s, await renderAppIcon(s, radius: 0.22, glyphScale: s <= 24 ? 0.95 : 0.85)),
      ];
      File('windows/runner/resources/app_icon.ico').writeAsBytesSync(ico(images));

      // macOS: сетка Apple — плитка 824 из 1024 с мягкой тенью.
      for (final s in [16, 32, 64, 128, 256, 512, 1024]) {
        await writeAppIcon('macos/Runner/Assets.xcassets/AppIcon.appiconset/app_icon_$s.png', s,
            radius: 0.2237, inset: s <= 32 ? 0.04 : 100 / 1024, shadow: s > 32, glyphScale: 0.85);
      }
    });
  });
}
