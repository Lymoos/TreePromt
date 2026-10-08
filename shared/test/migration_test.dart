import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

/// Миграция локальной БД v1 → v2 на устройстве, где уже есть данные (ТЗ п. 16).
void main() {
  test('v1 database with notes is upgraded to v2 without losing text', () async {
    setupSqlite();
    final db = AppDatabase(NativeDatabase.memory(setup: (raw) {
      // Таблица node_contents в том виде, в каком её создала схема v1.
      raw.execute('''CREATE TABLE node_contents (
        node_id TEXT NOT NULL PRIMARY KEY,
        raw_content TEXT NOT NULL DEFAULT '',
        raw_revision INTEGER NOT NULL DEFAULT 0,
        structured_content TEXT NULL)''');
      raw.execute("INSERT INTO node_contents (node_id, raw_content, raw_revision) VALUES ('n1', 'старый текст 🌳', 7)");
      raw.execute('PRAGMA user_version = 1');
    }));
    final store = DriftLocalStore(db);

    final c = await store.content('n1');
    expect(c!.rawContent, 'старый текст 🌳');
    expect(c.rawRevision, 7);
    expect(c.structuredRevision, 0);
    expect(c.structureProposal, isNull);

    await store.patchContent('n1', const NodeContentsCompanion(structureProposal: Value('{"x":1}')));
    expect((await store.content('n1'))!.structureProposal, '{"x":1}');
    final version = await db.customSelect('PRAGMA user_version').getSingle();
    expect(version.data.values.first, 2);
    await db.close();
  });
}
