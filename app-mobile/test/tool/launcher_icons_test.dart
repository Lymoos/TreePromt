// Генерирует иконки Android и iOS из того же рисунка, что AppIcon (вариант C).
// Плитка залита на всю площадь, без белых полей: форму (круг, «сквиркл»)
// задаёт сама система.
// Запуск: flutter test test/tool/launcher_icons_test.dart
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/icon_tool.dart';

const _res = 'android/app/src/main/res';
const _ios = 'ios/Runner/Assets.xcassets';

void main() {
  testWidgets('generate launcher icons', (tester) async {
    await tester.runAsync(() async {
      // Android до 8.0: квадрат, залитый целиком.
      const legacy = {'mdpi': 48, 'hdpi': 72, 'xhdpi': 96, 'xxhdpi': 144, 'xxxhdpi': 192};
      for (final e in legacy.entries) {
        await writeAppIcon('$_res/mipmap-${e.key}/ic_launcher.png', e.value, glyphScale: 0.8);
      }
      // Android 8+: адаптивная иконка. Фон — градиент (drawable/ic_launcher_background.xml),
      // передний план — только рисунок в безопасной зоне 66 из 108 dp.
      for (final e in legacy.entries) {
        final size = e.value * 108 ~/ 48;
        await writeAppIcon('$_res/drawable-${e.key}/ic_launcher_foreground.png', size,
            glyphScale: 0.62, tileVisible: false);
      }
      // Заставка при запуске: плитка 72 dp по центру экрана.
      for (final e in legacy.entries) {
        await writeAppIcon('$_res/drawable-${e.key}/splash_logo.png', e.value * 72 ~/ 48, radius: 0.24);
      }

      // iOS: все размеры, непрозрачные (App Store не принимает альфа-канал).
      final contents = jsonDecode(File('$_ios/AppIcon.appiconset/Contents.json').readAsStringSync()) as Map;
      for (final img in (contents['images'] as List).cast<Map>()) {
        final pt = double.parse((img['size'] as String).split('x').first);
        final scale = int.parse((img['scale'] as String).replaceAll('x', ''));
        await writeAppIcon('$_ios/AppIcon.appiconset/${img['filename']}', (pt * scale).round(),
            glyphScale: 0.8, opaque: true);
      }
      for (final (name, scale) in [('LaunchImage.png', 1), ('LaunchImage@2x.png', 2), ('LaunchImage@3x.png', 3)]) {
        await writeAppIcon('$_ios/LaunchImage.imageset/$name', 72 * scale, radius: 0.24, glyphScale: 0.85);
      }
    });
  });
}
