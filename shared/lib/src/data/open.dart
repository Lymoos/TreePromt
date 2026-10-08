import 'package:drift_flutter/drift_flutter.dart';
import 'package:path_provider/path_provider.dart';

import 'database.dart';
import 'drift_store.dart';

/// Открывает локальную БД: нативный SQLite (WAL) на Android/iOS/Windows/macOS/Linux,
/// SQLite в WebAssembly (OPFS или IndexedDB) в браузере.
DriftLocalStore openLocalStore({String name = 'prompttree'}) {
  final db = AppDatabase(driftDatabase(
    name: name,
    // Служебная папка приложения (AppData на Windows), а не «Документы» пользователя.
    native: const DriftNativeOptions(databaseDirectory: getApplicationSupportDirectory),
    web: DriftWebOptions(
      sqlite3Wasm: Uri.parse('sqlite3.wasm'),
      driftWorker: Uri.parse('drift_worker.js'),
    ),
  ));
  return DriftLocalStore(db);
}
