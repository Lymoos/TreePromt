import 'dart:ffi';
import 'dart:io';

import 'package:drift/drift.dart' show driftRuntimeOptions;
import 'package:drift/native.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_desktop/src/app.dart';
import 'package:prompttree_shared/app.dart';

import 'package:prompttree_shared/prompttree_shared.dart';
import 'package:sqlite3/open.dart';

DriftLocalStore memoryStore() {
  driftRuntimeOptions.dontWarnAboutMultipleDatabases = true;
  if (Platform.isWindows) {
    open.overrideFor(OperatingSystem.windows, () {
      try {
        return DynamicLibrary.open('sqlite3.dll');
      } catch (_) {
        return DynamicLibrary.open('winsqlite3.dll');
      }
    });
  }
  return DriftLocalStore(AppDatabase(NativeDatabase.memory()));
}

Widget app(LocalStore store, {Session session = const Session(mode: SessionMode.local)}) => ProviderScope(
      overrides: [
        storeProvider.overrideWithValue(store),
        tokenStoreProvider.overrideWithValue(MemoryTokenStore()),
        sessionProvider.overrideWith((ref) => session),
      ],
      child: const PromptTreeDesktopApp(),
    );

/// Настоящие шрифты для скриншотов: Onest, JetBrains Mono и иконки Material из SDK.
Future<void> loadFonts() async {
  Future<void> load(String family, String asset) async {
    final loader = FontLoader(family)..addFont(rootBundle.load(asset));
    await loader.load();
  }

  await load('packages/prompttree_shared/Onest', 'packages/prompttree_shared/assets/fonts/Onest-Variable.ttf');
  await load('packages/prompttree_shared/JetBrainsMono',
      'packages/prompttree_shared/assets/fonts/JetBrainsMono-Variable.ttf');
  final sdk = Platform.environment['FLUTTER_ROOT'] ?? '';
  final icons = File('$sdk/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf');
  if (icons.existsSync()) {
    final loader = FontLoader('MaterialIcons')..addFont(Future.value(ByteData.sublistView(icons.readAsBytesSync())));
    await loader.load();
  }
}

/// Закрыть дерево виджетов и БД так, чтобы не осталось висящих таймеров drift.
Future<void> tearDownApp(WidgetTester tester, DriftLocalStore store) async {
  await tester.pumpWidget(const SizedBox());
  await tester.pump(const Duration(milliseconds: 10));
  await store.db.close();
}
