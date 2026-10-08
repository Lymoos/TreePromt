// Генерирует favicon и иконки PWA из рисунка AppIcon (вариант C).
// Запуск: flutter test test/tool/web_icons_test.dart
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/icon_tool.dart';

void main() {
  testWidgets('generate web icons', (tester) async {
    await tester.runAsync(() async {
      await writeAppIcon('web/favicon.png', 32, radius: 0.24, glyphScale: 0.95);
      await writeAppIcon('web/icons/Icon-192.png', 192, radius: 0.24);
      await writeAppIcon('web/icons/Icon-512.png', 512, radius: 0.24);
      // iOS сам скругляет углы — квадрат без прозрачности.
      await writeAppIcon('web/icons/apple-touch-icon.png', 180, glyphScale: 0.8, opaque: true);
      // Maskable: система обрежет до круга — рисунок в безопасной зоне 80 %.
      await writeAppIcon('web/icons/Icon-maskable-192.png', 192, glyphScale: 0.68);
      await writeAppIcon('web/icons/Icon-maskable-512.png', 512, glyphScale: 0.68);
    });
  });
}
