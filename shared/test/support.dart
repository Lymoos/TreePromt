import 'dart:ffi';
import 'dart:io';

import 'package:drift/drift.dart' show driftRuntimeOptions;
import 'package:drift/native.dart';
import 'package:prompttree_shared/prompttree_shared.dart';
import 'package:sqlite3/open.dart';

bool _ready = false;

/// В `flutter test` на Windows нет sqlite3.dll из приложения: берём системный winsqlite3.
void setupSqlite() {
  driftRuntimeOptions.dontWarnAboutMultipleDatabases = true;
  if (_ready) return;
  _ready = true;
  if (Platform.isWindows) {
    open.overrideFor(OperatingSystem.windows, () {
      try {
        return DynamicLibrary.open('sqlite3.dll');
      } catch (_) {
        return DynamicLibrary.open('winsqlite3.dll');
      }
    });
  }
}

DriftLocalStore memoryStore() {
  setupSqlite();
  return DriftLocalStore(AppDatabase(NativeDatabase.memory()));
}
