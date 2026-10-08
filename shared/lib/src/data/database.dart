import 'package:drift/drift.dart';

part 'database.g.dart';

/// Подтверждённое сервером состояние проектов плюс локальные неотправленные
/// правки поверх него (см. sync/reducer.dart). Интерфейс читает только эти таблицы.
@DataClassName('Project')
class Projects extends Table {
  TextColumn get id => text()();
  TextColumn get name => text()();
  TextColumn get description => text().withDefault(const Constant(''))();
  IntColumn get revision => integer().withDefault(const Constant(0))();
  TextColumn get role => text().withDefault(const Constant('owner'))();
  DateTimeColumn get createdAt => dateTime()();
  DateTimeColumn get updatedAt => dateTime()();
  DateTimeColumn get deletedAt => dateTime().nullable()();

  /// Код отказа сервера, если последняя операция по проекту не прошла.
  TextColumn get syncError => text().nullable()();

  @override
  Set<Column> get primaryKey => {id};
}

/// Узлы дерева без текста: дерево перестраивается часто, тексты в нём не нужны.
@DataClassName('TreeNode')
class Nodes extends Table {
  TextColumn get id => text()();
  TextColumn get projectId => text()();
  TextColumn get parentId => text().nullable()();
  TextColumn get kind => text()();
  TextColumn get name => text()();
  TextColumn get sortKey => text()();
  IntColumn get revision => integer().withDefault(const Constant(0))();
  BoolColumn get hasConflict => boolean().withDefault(const Constant(false))();
  TextColumn get structureStatus => text().withDefault(const Constant('none'))();
  DateTimeColumn get createdAt => dateTime()();
  DateTimeColumn get updatedAt => dateTime()();
  DateTimeColumn get deletedAt => dateTime().nullable()();
  TextColumn get syncError => text().nullable()();

  @override
  Set<Column> get primaryKey => {id};
}

@DataClassName('NodeContent')
class NodeContents extends Table {
  TextColumn get nodeId => text()();
  TextColumn get rawContent => text().withDefault(const Constant(''))();

  /// Ревизия текста, подтверждённая сервером. Это base_revision для set_raw_content.
  IntColumn get rawRevision => integer().withDefault(const Constant(0))();
  TextColumn get structuredContent => text().nullable()();

  @override
  Set<Column> get primaryKey => {nodeId};
}

@DataClassName('ContentVersion')
class ContentVersions extends Table {
  TextColumn get id => text()();
  TextColumn get nodeId => text()();
  TextColumn get projectId => text()();
  TextColumn get field => text()();
  TextColumn get content => text()();
  IntColumn get atRevision => integer()();
  TextColumn get reason => text()();
  IntColumn get serverRevision => integer()();
  DateTimeColumn get createdAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

/// Очередь операций. seq — это client_seq; запись в очередь всегда в одной
/// транзакции с изменением данных (архитектура п. 2.3).
@DataClassName('OutboxEntry')
class Outbox extends Table {
  IntColumn get seq => integer().autoIncrement()();
  TextColumn get operationId => text().unique()();
  TextColumn get type => text()();
  TextColumn get entityId => text()();
  IntColumn get baseRevision => integer().withDefault(const Constant(0))();
  TextColumn get payload => text()();
  BoolColumn get inFlight => boolean().withDefault(const Constant(false))();
  DateTimeColumn get createdAt => dateTime()();
}

@DataClassName('KeyValue')
class KeyValues extends Table {
  TextColumn get key => text()();
  TextColumn get value => text()();

  @override
  Set<Column> get primaryKey => {key};
}

@DriftDatabase(tables: [Projects, Nodes, NodeContents, ContentVersions, Outbox, KeyValues])
class AppDatabase extends _$AppDatabase {
  AppDatabase(super.e);

  @override
  int get schemaVersion => 1;

  @override
  MigrationStrategy get migration => MigrationStrategy(
        onCreate: (m) async {
          await m.createAll();
          await m.createIndex(Index('nodes_project', 'CREATE INDEX nodes_project ON nodes (project_id)'));
          await m.createIndex(Index('outbox_entity', 'CREATE INDEX outbox_entity ON outbox (entity_id, seq)'));
          await m.createIndex(Index('versions_node', 'CREATE INDEX versions_node ON content_versions (node_id)'));
        },
        // Будущие миграции — шаги по schemaVersion, каждый с тестом (ТЗ п. 16).
        beforeOpen: (details) async {
          // WAL: запись не блокирует чтение и переживает падение процесса.
          // В вебе (WASM) прагма не поддерживается — там это не ошибка.
          try {
            await customStatement('PRAGMA journal_mode = WAL');
          } catch (_) {}
        },
      );
}
