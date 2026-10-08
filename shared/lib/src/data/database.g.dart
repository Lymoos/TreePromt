// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'database.dart';

// ignore_for_file: type=lint
class $ProjectsTable extends Projects with TableInfo<$ProjectsTable, Project> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $ProjectsTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
      'id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
      'name', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _descriptionMeta =
      const VerificationMeta('description');
  @override
  late final GeneratedColumn<String> description = GeneratedColumn<String>(
      'description', aliasedName, false,
      type: DriftSqlType.string,
      requiredDuringInsert: false,
      defaultValue: const Constant(''));
  static const VerificationMeta _revisionMeta =
      const VerificationMeta('revision');
  @override
  late final GeneratedColumn<int> revision = GeneratedColumn<int>(
      'revision', aliasedName, false,
      type: DriftSqlType.int,
      requiredDuringInsert: false,
      defaultValue: const Constant(0));
  static const VerificationMeta _roleMeta = const VerificationMeta('role');
  @override
  late final GeneratedColumn<String> role = GeneratedColumn<String>(
      'role', aliasedName, false,
      type: DriftSqlType.string,
      requiredDuringInsert: false,
      defaultValue: const Constant('owner'));
  static const VerificationMeta _createdAtMeta =
      const VerificationMeta('createdAt');
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
      'created_at', aliasedName, false,
      type: DriftSqlType.dateTime, requiredDuringInsert: true);
  static const VerificationMeta _updatedAtMeta =
      const VerificationMeta('updatedAt');
  @override
  late final GeneratedColumn<DateTime> updatedAt = GeneratedColumn<DateTime>(
      'updated_at', aliasedName, false,
      type: DriftSqlType.dateTime, requiredDuringInsert: true);
  static const VerificationMeta _deletedAtMeta =
      const VerificationMeta('deletedAt');
  @override
  late final GeneratedColumn<DateTime> deletedAt = GeneratedColumn<DateTime>(
      'deleted_at', aliasedName, true,
      type: DriftSqlType.dateTime, requiredDuringInsert: false);
  static const VerificationMeta _syncErrorMeta =
      const VerificationMeta('syncError');
  @override
  late final GeneratedColumn<String> syncError = GeneratedColumn<String>(
      'sync_error', aliasedName, true,
      type: DriftSqlType.string, requiredDuringInsert: false);
  @override
  List<GeneratedColumn> get $columns => [
        id,
        name,
        description,
        revision,
        role,
        createdAt,
        updatedAt,
        deletedAt,
        syncError
      ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'projects';
  @override
  VerificationContext validateIntegrity(Insertable<Project> instance,
      {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
          _nameMeta, name.isAcceptableOrUnknown(data['name']!, _nameMeta));
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('description')) {
      context.handle(
          _descriptionMeta,
          description.isAcceptableOrUnknown(
              data['description']!, _descriptionMeta));
    }
    if (data.containsKey('revision')) {
      context.handle(_revisionMeta,
          revision.isAcceptableOrUnknown(data['revision']!, _revisionMeta));
    }
    if (data.containsKey('role')) {
      context.handle(
          _roleMeta, role.isAcceptableOrUnknown(data['role']!, _roleMeta));
    }
    if (data.containsKey('created_at')) {
      context.handle(_createdAtMeta,
          createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta));
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    if (data.containsKey('updated_at')) {
      context.handle(_updatedAtMeta,
          updatedAt.isAcceptableOrUnknown(data['updated_at']!, _updatedAtMeta));
    } else if (isInserting) {
      context.missing(_updatedAtMeta);
    }
    if (data.containsKey('deleted_at')) {
      context.handle(_deletedAtMeta,
          deletedAt.isAcceptableOrUnknown(data['deleted_at']!, _deletedAtMeta));
    }
    if (data.containsKey('sync_error')) {
      context.handle(_syncErrorMeta,
          syncError.isAcceptableOrUnknown(data['sync_error']!, _syncErrorMeta));
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  Project map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return Project(
      id: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}id'])!,
      name: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}name'])!,
      description: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}description'])!,
      revision: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}revision'])!,
      role: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}role'])!,
      createdAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}created_at'])!,
      updatedAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}updated_at'])!,
      deletedAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}deleted_at']),
      syncError: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}sync_error']),
    );
  }

  @override
  $ProjectsTable createAlias(String alias) {
    return $ProjectsTable(attachedDatabase, alias);
  }
}

class Project extends DataClass implements Insertable<Project> {
  final String id;
  final String name;
  final String description;
  final int revision;
  final String role;
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime? deletedAt;

  /// Код отказа сервера, если последняя операция по проекту не прошла.
  final String? syncError;
  const Project(
      {required this.id,
      required this.name,
      required this.description,
      required this.revision,
      required this.role,
      required this.createdAt,
      required this.updatedAt,
      this.deletedAt,
      this.syncError});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['name'] = Variable<String>(name);
    map['description'] = Variable<String>(description);
    map['revision'] = Variable<int>(revision);
    map['role'] = Variable<String>(role);
    map['created_at'] = Variable<DateTime>(createdAt);
    map['updated_at'] = Variable<DateTime>(updatedAt);
    if (!nullToAbsent || deletedAt != null) {
      map['deleted_at'] = Variable<DateTime>(deletedAt);
    }
    if (!nullToAbsent || syncError != null) {
      map['sync_error'] = Variable<String>(syncError);
    }
    return map;
  }

  ProjectsCompanion toCompanion(bool nullToAbsent) {
    return ProjectsCompanion(
      id: Value(id),
      name: Value(name),
      description: Value(description),
      revision: Value(revision),
      role: Value(role),
      createdAt: Value(createdAt),
      updatedAt: Value(updatedAt),
      deletedAt: deletedAt == null && nullToAbsent
          ? const Value.absent()
          : Value(deletedAt),
      syncError: syncError == null && nullToAbsent
          ? const Value.absent()
          : Value(syncError),
    );
  }

  factory Project.fromJson(Map<String, dynamic> json,
      {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return Project(
      id: serializer.fromJson<String>(json['id']),
      name: serializer.fromJson<String>(json['name']),
      description: serializer.fromJson<String>(json['description']),
      revision: serializer.fromJson<int>(json['revision']),
      role: serializer.fromJson<String>(json['role']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
      updatedAt: serializer.fromJson<DateTime>(json['updatedAt']),
      deletedAt: serializer.fromJson<DateTime?>(json['deletedAt']),
      syncError: serializer.fromJson<String?>(json['syncError']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'name': serializer.toJson<String>(name),
      'description': serializer.toJson<String>(description),
      'revision': serializer.toJson<int>(revision),
      'role': serializer.toJson<String>(role),
      'createdAt': serializer.toJson<DateTime>(createdAt),
      'updatedAt': serializer.toJson<DateTime>(updatedAt),
      'deletedAt': serializer.toJson<DateTime?>(deletedAt),
      'syncError': serializer.toJson<String?>(syncError),
    };
  }

  Project copyWith(
          {String? id,
          String? name,
          String? description,
          int? revision,
          String? role,
          DateTime? createdAt,
          DateTime? updatedAt,
          Value<DateTime?> deletedAt = const Value.absent(),
          Value<String?> syncError = const Value.absent()}) =>
      Project(
        id: id ?? this.id,
        name: name ?? this.name,
        description: description ?? this.description,
        revision: revision ?? this.revision,
        role: role ?? this.role,
        createdAt: createdAt ?? this.createdAt,
        updatedAt: updatedAt ?? this.updatedAt,
        deletedAt: deletedAt.present ? deletedAt.value : this.deletedAt,
        syncError: syncError.present ? syncError.value : this.syncError,
      );
  Project copyWithCompanion(ProjectsCompanion data) {
    return Project(
      id: data.id.present ? data.id.value : this.id,
      name: data.name.present ? data.name.value : this.name,
      description:
          data.description.present ? data.description.value : this.description,
      revision: data.revision.present ? data.revision.value : this.revision,
      role: data.role.present ? data.role.value : this.role,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
      updatedAt: data.updatedAt.present ? data.updatedAt.value : this.updatedAt,
      deletedAt: data.deletedAt.present ? data.deletedAt.value : this.deletedAt,
      syncError: data.syncError.present ? data.syncError.value : this.syncError,
    );
  }

  @override
  String toString() {
    return (StringBuffer('Project(')
          ..write('id: $id, ')
          ..write('name: $name, ')
          ..write('description: $description, ')
          ..write('revision: $revision, ')
          ..write('role: $role, ')
          ..write('createdAt: $createdAt, ')
          ..write('updatedAt: $updatedAt, ')
          ..write('deletedAt: $deletedAt, ')
          ..write('syncError: $syncError')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(id, name, description, revision, role,
      createdAt, updatedAt, deletedAt, syncError);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is Project &&
          other.id == this.id &&
          other.name == this.name &&
          other.description == this.description &&
          other.revision == this.revision &&
          other.role == this.role &&
          other.createdAt == this.createdAt &&
          other.updatedAt == this.updatedAt &&
          other.deletedAt == this.deletedAt &&
          other.syncError == this.syncError);
}

class ProjectsCompanion extends UpdateCompanion<Project> {
  final Value<String> id;
  final Value<String> name;
  final Value<String> description;
  final Value<int> revision;
  final Value<String> role;
  final Value<DateTime> createdAt;
  final Value<DateTime> updatedAt;
  final Value<DateTime?> deletedAt;
  final Value<String?> syncError;
  final Value<int> rowid;
  const ProjectsCompanion({
    this.id = const Value.absent(),
    this.name = const Value.absent(),
    this.description = const Value.absent(),
    this.revision = const Value.absent(),
    this.role = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.updatedAt = const Value.absent(),
    this.deletedAt = const Value.absent(),
    this.syncError = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  ProjectsCompanion.insert({
    required String id,
    required String name,
    this.description = const Value.absent(),
    this.revision = const Value.absent(),
    this.role = const Value.absent(),
    required DateTime createdAt,
    required DateTime updatedAt,
    this.deletedAt = const Value.absent(),
    this.syncError = const Value.absent(),
    this.rowid = const Value.absent(),
  })  : id = Value(id),
        name = Value(name),
        createdAt = Value(createdAt),
        updatedAt = Value(updatedAt);
  static Insertable<Project> custom({
    Expression<String>? id,
    Expression<String>? name,
    Expression<String>? description,
    Expression<int>? revision,
    Expression<String>? role,
    Expression<DateTime>? createdAt,
    Expression<DateTime>? updatedAt,
    Expression<DateTime>? deletedAt,
    Expression<String>? syncError,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (name != null) 'name': name,
      if (description != null) 'description': description,
      if (revision != null) 'revision': revision,
      if (role != null) 'role': role,
      if (createdAt != null) 'created_at': createdAt,
      if (updatedAt != null) 'updated_at': updatedAt,
      if (deletedAt != null) 'deleted_at': deletedAt,
      if (syncError != null) 'sync_error': syncError,
      if (rowid != null) 'rowid': rowid,
    });
  }

  ProjectsCompanion copyWith(
      {Value<String>? id,
      Value<String>? name,
      Value<String>? description,
      Value<int>? revision,
      Value<String>? role,
      Value<DateTime>? createdAt,
      Value<DateTime>? updatedAt,
      Value<DateTime?>? deletedAt,
      Value<String?>? syncError,
      Value<int>? rowid}) {
    return ProjectsCompanion(
      id: id ?? this.id,
      name: name ?? this.name,
      description: description ?? this.description,
      revision: revision ?? this.revision,
      role: role ?? this.role,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      deletedAt: deletedAt ?? this.deletedAt,
      syncError: syncError ?? this.syncError,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (description.present) {
      map['description'] = Variable<String>(description.value);
    }
    if (revision.present) {
      map['revision'] = Variable<int>(revision.value);
    }
    if (role.present) {
      map['role'] = Variable<String>(role.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (updatedAt.present) {
      map['updated_at'] = Variable<DateTime>(updatedAt.value);
    }
    if (deletedAt.present) {
      map['deleted_at'] = Variable<DateTime>(deletedAt.value);
    }
    if (syncError.present) {
      map['sync_error'] = Variable<String>(syncError.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('ProjectsCompanion(')
          ..write('id: $id, ')
          ..write('name: $name, ')
          ..write('description: $description, ')
          ..write('revision: $revision, ')
          ..write('role: $role, ')
          ..write('createdAt: $createdAt, ')
          ..write('updatedAt: $updatedAt, ')
          ..write('deletedAt: $deletedAt, ')
          ..write('syncError: $syncError, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $NodesTable extends Nodes with TableInfo<$NodesTable, TreeNode> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $NodesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
      'id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _projectIdMeta =
      const VerificationMeta('projectId');
  @override
  late final GeneratedColumn<String> projectId = GeneratedColumn<String>(
      'project_id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _parentIdMeta =
      const VerificationMeta('parentId');
  @override
  late final GeneratedColumn<String> parentId = GeneratedColumn<String>(
      'parent_id', aliasedName, true,
      type: DriftSqlType.string, requiredDuringInsert: false);
  static const VerificationMeta _kindMeta = const VerificationMeta('kind');
  @override
  late final GeneratedColumn<String> kind = GeneratedColumn<String>(
      'kind', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
      'name', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _sortKeyMeta =
      const VerificationMeta('sortKey');
  @override
  late final GeneratedColumn<String> sortKey = GeneratedColumn<String>(
      'sort_key', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _revisionMeta =
      const VerificationMeta('revision');
  @override
  late final GeneratedColumn<int> revision = GeneratedColumn<int>(
      'revision', aliasedName, false,
      type: DriftSqlType.int,
      requiredDuringInsert: false,
      defaultValue: const Constant(0));
  static const VerificationMeta _hasConflictMeta =
      const VerificationMeta('hasConflict');
  @override
  late final GeneratedColumn<bool> hasConflict = GeneratedColumn<bool>(
      'has_conflict', aliasedName, false,
      type: DriftSqlType.bool,
      requiredDuringInsert: false,
      defaultConstraints: GeneratedColumn.constraintIsAlways(
          'CHECK ("has_conflict" IN (0, 1))'),
      defaultValue: const Constant(false));
  static const VerificationMeta _structureStatusMeta =
      const VerificationMeta('structureStatus');
  @override
  late final GeneratedColumn<String> structureStatus = GeneratedColumn<String>(
      'structure_status', aliasedName, false,
      type: DriftSqlType.string,
      requiredDuringInsert: false,
      defaultValue: const Constant('none'));
  static const VerificationMeta _createdAtMeta =
      const VerificationMeta('createdAt');
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
      'created_at', aliasedName, false,
      type: DriftSqlType.dateTime, requiredDuringInsert: true);
  static const VerificationMeta _updatedAtMeta =
      const VerificationMeta('updatedAt');
  @override
  late final GeneratedColumn<DateTime> updatedAt = GeneratedColumn<DateTime>(
      'updated_at', aliasedName, false,
      type: DriftSqlType.dateTime, requiredDuringInsert: true);
  static const VerificationMeta _deletedAtMeta =
      const VerificationMeta('deletedAt');
  @override
  late final GeneratedColumn<DateTime> deletedAt = GeneratedColumn<DateTime>(
      'deleted_at', aliasedName, true,
      type: DriftSqlType.dateTime, requiredDuringInsert: false);
  static const VerificationMeta _syncErrorMeta =
      const VerificationMeta('syncError');
  @override
  late final GeneratedColumn<String> syncError = GeneratedColumn<String>(
      'sync_error', aliasedName, true,
      type: DriftSqlType.string, requiredDuringInsert: false);
  @override
  List<GeneratedColumn> get $columns => [
        id,
        projectId,
        parentId,
        kind,
        name,
        sortKey,
        revision,
        hasConflict,
        structureStatus,
        createdAt,
        updatedAt,
        deletedAt,
        syncError
      ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'nodes';
  @override
  VerificationContext validateIntegrity(Insertable<TreeNode> instance,
      {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('project_id')) {
      context.handle(_projectIdMeta,
          projectId.isAcceptableOrUnknown(data['project_id']!, _projectIdMeta));
    } else if (isInserting) {
      context.missing(_projectIdMeta);
    }
    if (data.containsKey('parent_id')) {
      context.handle(_parentIdMeta,
          parentId.isAcceptableOrUnknown(data['parent_id']!, _parentIdMeta));
    }
    if (data.containsKey('kind')) {
      context.handle(
          _kindMeta, kind.isAcceptableOrUnknown(data['kind']!, _kindMeta));
    } else if (isInserting) {
      context.missing(_kindMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
          _nameMeta, name.isAcceptableOrUnknown(data['name']!, _nameMeta));
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('sort_key')) {
      context.handle(_sortKeyMeta,
          sortKey.isAcceptableOrUnknown(data['sort_key']!, _sortKeyMeta));
    } else if (isInserting) {
      context.missing(_sortKeyMeta);
    }
    if (data.containsKey('revision')) {
      context.handle(_revisionMeta,
          revision.isAcceptableOrUnknown(data['revision']!, _revisionMeta));
    }
    if (data.containsKey('has_conflict')) {
      context.handle(
          _hasConflictMeta,
          hasConflict.isAcceptableOrUnknown(
              data['has_conflict']!, _hasConflictMeta));
    }
    if (data.containsKey('structure_status')) {
      context.handle(
          _structureStatusMeta,
          structureStatus.isAcceptableOrUnknown(
              data['structure_status']!, _structureStatusMeta));
    }
    if (data.containsKey('created_at')) {
      context.handle(_createdAtMeta,
          createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta));
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    if (data.containsKey('updated_at')) {
      context.handle(_updatedAtMeta,
          updatedAt.isAcceptableOrUnknown(data['updated_at']!, _updatedAtMeta));
    } else if (isInserting) {
      context.missing(_updatedAtMeta);
    }
    if (data.containsKey('deleted_at')) {
      context.handle(_deletedAtMeta,
          deletedAt.isAcceptableOrUnknown(data['deleted_at']!, _deletedAtMeta));
    }
    if (data.containsKey('sync_error')) {
      context.handle(_syncErrorMeta,
          syncError.isAcceptableOrUnknown(data['sync_error']!, _syncErrorMeta));
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  TreeNode map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return TreeNode(
      id: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}id'])!,
      projectId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}project_id'])!,
      parentId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}parent_id']),
      kind: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}kind'])!,
      name: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}name'])!,
      sortKey: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}sort_key'])!,
      revision: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}revision'])!,
      hasConflict: attachedDatabase.typeMapping
          .read(DriftSqlType.bool, data['${effectivePrefix}has_conflict'])!,
      structureStatus: attachedDatabase.typeMapping.read(
          DriftSqlType.string, data['${effectivePrefix}structure_status'])!,
      createdAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}created_at'])!,
      updatedAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}updated_at'])!,
      deletedAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}deleted_at']),
      syncError: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}sync_error']),
    );
  }

  @override
  $NodesTable createAlias(String alias) {
    return $NodesTable(attachedDatabase, alias);
  }
}

class TreeNode extends DataClass implements Insertable<TreeNode> {
  final String id;
  final String projectId;
  final String? parentId;
  final String kind;
  final String name;
  final String sortKey;
  final int revision;
  final bool hasConflict;
  final String structureStatus;
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime? deletedAt;
  final String? syncError;
  const TreeNode(
      {required this.id,
      required this.projectId,
      this.parentId,
      required this.kind,
      required this.name,
      required this.sortKey,
      required this.revision,
      required this.hasConflict,
      required this.structureStatus,
      required this.createdAt,
      required this.updatedAt,
      this.deletedAt,
      this.syncError});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['project_id'] = Variable<String>(projectId);
    if (!nullToAbsent || parentId != null) {
      map['parent_id'] = Variable<String>(parentId);
    }
    map['kind'] = Variable<String>(kind);
    map['name'] = Variable<String>(name);
    map['sort_key'] = Variable<String>(sortKey);
    map['revision'] = Variable<int>(revision);
    map['has_conflict'] = Variable<bool>(hasConflict);
    map['structure_status'] = Variable<String>(structureStatus);
    map['created_at'] = Variable<DateTime>(createdAt);
    map['updated_at'] = Variable<DateTime>(updatedAt);
    if (!nullToAbsent || deletedAt != null) {
      map['deleted_at'] = Variable<DateTime>(deletedAt);
    }
    if (!nullToAbsent || syncError != null) {
      map['sync_error'] = Variable<String>(syncError);
    }
    return map;
  }

  NodesCompanion toCompanion(bool nullToAbsent) {
    return NodesCompanion(
      id: Value(id),
      projectId: Value(projectId),
      parentId: parentId == null && nullToAbsent
          ? const Value.absent()
          : Value(parentId),
      kind: Value(kind),
      name: Value(name),
      sortKey: Value(sortKey),
      revision: Value(revision),
      hasConflict: Value(hasConflict),
      structureStatus: Value(structureStatus),
      createdAt: Value(createdAt),
      updatedAt: Value(updatedAt),
      deletedAt: deletedAt == null && nullToAbsent
          ? const Value.absent()
          : Value(deletedAt),
      syncError: syncError == null && nullToAbsent
          ? const Value.absent()
          : Value(syncError),
    );
  }

  factory TreeNode.fromJson(Map<String, dynamic> json,
      {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return TreeNode(
      id: serializer.fromJson<String>(json['id']),
      projectId: serializer.fromJson<String>(json['projectId']),
      parentId: serializer.fromJson<String?>(json['parentId']),
      kind: serializer.fromJson<String>(json['kind']),
      name: serializer.fromJson<String>(json['name']),
      sortKey: serializer.fromJson<String>(json['sortKey']),
      revision: serializer.fromJson<int>(json['revision']),
      hasConflict: serializer.fromJson<bool>(json['hasConflict']),
      structureStatus: serializer.fromJson<String>(json['structureStatus']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
      updatedAt: serializer.fromJson<DateTime>(json['updatedAt']),
      deletedAt: serializer.fromJson<DateTime?>(json['deletedAt']),
      syncError: serializer.fromJson<String?>(json['syncError']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'projectId': serializer.toJson<String>(projectId),
      'parentId': serializer.toJson<String?>(parentId),
      'kind': serializer.toJson<String>(kind),
      'name': serializer.toJson<String>(name),
      'sortKey': serializer.toJson<String>(sortKey),
      'revision': serializer.toJson<int>(revision),
      'hasConflict': serializer.toJson<bool>(hasConflict),
      'structureStatus': serializer.toJson<String>(structureStatus),
      'createdAt': serializer.toJson<DateTime>(createdAt),
      'updatedAt': serializer.toJson<DateTime>(updatedAt),
      'deletedAt': serializer.toJson<DateTime?>(deletedAt),
      'syncError': serializer.toJson<String?>(syncError),
    };
  }

  TreeNode copyWith(
          {String? id,
          String? projectId,
          Value<String?> parentId = const Value.absent(),
          String? kind,
          String? name,
          String? sortKey,
          int? revision,
          bool? hasConflict,
          String? structureStatus,
          DateTime? createdAt,
          DateTime? updatedAt,
          Value<DateTime?> deletedAt = const Value.absent(),
          Value<String?> syncError = const Value.absent()}) =>
      TreeNode(
        id: id ?? this.id,
        projectId: projectId ?? this.projectId,
        parentId: parentId.present ? parentId.value : this.parentId,
        kind: kind ?? this.kind,
        name: name ?? this.name,
        sortKey: sortKey ?? this.sortKey,
        revision: revision ?? this.revision,
        hasConflict: hasConflict ?? this.hasConflict,
        structureStatus: structureStatus ?? this.structureStatus,
        createdAt: createdAt ?? this.createdAt,
        updatedAt: updatedAt ?? this.updatedAt,
        deletedAt: deletedAt.present ? deletedAt.value : this.deletedAt,
        syncError: syncError.present ? syncError.value : this.syncError,
      );
  TreeNode copyWithCompanion(NodesCompanion data) {
    return TreeNode(
      id: data.id.present ? data.id.value : this.id,
      projectId: data.projectId.present ? data.projectId.value : this.projectId,
      parentId: data.parentId.present ? data.parentId.value : this.parentId,
      kind: data.kind.present ? data.kind.value : this.kind,
      name: data.name.present ? data.name.value : this.name,
      sortKey: data.sortKey.present ? data.sortKey.value : this.sortKey,
      revision: data.revision.present ? data.revision.value : this.revision,
      hasConflict:
          data.hasConflict.present ? data.hasConflict.value : this.hasConflict,
      structureStatus: data.structureStatus.present
          ? data.structureStatus.value
          : this.structureStatus,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
      updatedAt: data.updatedAt.present ? data.updatedAt.value : this.updatedAt,
      deletedAt: data.deletedAt.present ? data.deletedAt.value : this.deletedAt,
      syncError: data.syncError.present ? data.syncError.value : this.syncError,
    );
  }

  @override
  String toString() {
    return (StringBuffer('TreeNode(')
          ..write('id: $id, ')
          ..write('projectId: $projectId, ')
          ..write('parentId: $parentId, ')
          ..write('kind: $kind, ')
          ..write('name: $name, ')
          ..write('sortKey: $sortKey, ')
          ..write('revision: $revision, ')
          ..write('hasConflict: $hasConflict, ')
          ..write('structureStatus: $structureStatus, ')
          ..write('createdAt: $createdAt, ')
          ..write('updatedAt: $updatedAt, ')
          ..write('deletedAt: $deletedAt, ')
          ..write('syncError: $syncError')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
      id,
      projectId,
      parentId,
      kind,
      name,
      sortKey,
      revision,
      hasConflict,
      structureStatus,
      createdAt,
      updatedAt,
      deletedAt,
      syncError);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is TreeNode &&
          other.id == this.id &&
          other.projectId == this.projectId &&
          other.parentId == this.parentId &&
          other.kind == this.kind &&
          other.name == this.name &&
          other.sortKey == this.sortKey &&
          other.revision == this.revision &&
          other.hasConflict == this.hasConflict &&
          other.structureStatus == this.structureStatus &&
          other.createdAt == this.createdAt &&
          other.updatedAt == this.updatedAt &&
          other.deletedAt == this.deletedAt &&
          other.syncError == this.syncError);
}

class NodesCompanion extends UpdateCompanion<TreeNode> {
  final Value<String> id;
  final Value<String> projectId;
  final Value<String?> parentId;
  final Value<String> kind;
  final Value<String> name;
  final Value<String> sortKey;
  final Value<int> revision;
  final Value<bool> hasConflict;
  final Value<String> structureStatus;
  final Value<DateTime> createdAt;
  final Value<DateTime> updatedAt;
  final Value<DateTime?> deletedAt;
  final Value<String?> syncError;
  final Value<int> rowid;
  const NodesCompanion({
    this.id = const Value.absent(),
    this.projectId = const Value.absent(),
    this.parentId = const Value.absent(),
    this.kind = const Value.absent(),
    this.name = const Value.absent(),
    this.sortKey = const Value.absent(),
    this.revision = const Value.absent(),
    this.hasConflict = const Value.absent(),
    this.structureStatus = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.updatedAt = const Value.absent(),
    this.deletedAt = const Value.absent(),
    this.syncError = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  NodesCompanion.insert({
    required String id,
    required String projectId,
    this.parentId = const Value.absent(),
    required String kind,
    required String name,
    required String sortKey,
    this.revision = const Value.absent(),
    this.hasConflict = const Value.absent(),
    this.structureStatus = const Value.absent(),
    required DateTime createdAt,
    required DateTime updatedAt,
    this.deletedAt = const Value.absent(),
    this.syncError = const Value.absent(),
    this.rowid = const Value.absent(),
  })  : id = Value(id),
        projectId = Value(projectId),
        kind = Value(kind),
        name = Value(name),
        sortKey = Value(sortKey),
        createdAt = Value(createdAt),
        updatedAt = Value(updatedAt);
  static Insertable<TreeNode> custom({
    Expression<String>? id,
    Expression<String>? projectId,
    Expression<String>? parentId,
    Expression<String>? kind,
    Expression<String>? name,
    Expression<String>? sortKey,
    Expression<int>? revision,
    Expression<bool>? hasConflict,
    Expression<String>? structureStatus,
    Expression<DateTime>? createdAt,
    Expression<DateTime>? updatedAt,
    Expression<DateTime>? deletedAt,
    Expression<String>? syncError,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (projectId != null) 'project_id': projectId,
      if (parentId != null) 'parent_id': parentId,
      if (kind != null) 'kind': kind,
      if (name != null) 'name': name,
      if (sortKey != null) 'sort_key': sortKey,
      if (revision != null) 'revision': revision,
      if (hasConflict != null) 'has_conflict': hasConflict,
      if (structureStatus != null) 'structure_status': structureStatus,
      if (createdAt != null) 'created_at': createdAt,
      if (updatedAt != null) 'updated_at': updatedAt,
      if (deletedAt != null) 'deleted_at': deletedAt,
      if (syncError != null) 'sync_error': syncError,
      if (rowid != null) 'rowid': rowid,
    });
  }

  NodesCompanion copyWith(
      {Value<String>? id,
      Value<String>? projectId,
      Value<String?>? parentId,
      Value<String>? kind,
      Value<String>? name,
      Value<String>? sortKey,
      Value<int>? revision,
      Value<bool>? hasConflict,
      Value<String>? structureStatus,
      Value<DateTime>? createdAt,
      Value<DateTime>? updatedAt,
      Value<DateTime?>? deletedAt,
      Value<String?>? syncError,
      Value<int>? rowid}) {
    return NodesCompanion(
      id: id ?? this.id,
      projectId: projectId ?? this.projectId,
      parentId: parentId ?? this.parentId,
      kind: kind ?? this.kind,
      name: name ?? this.name,
      sortKey: sortKey ?? this.sortKey,
      revision: revision ?? this.revision,
      hasConflict: hasConflict ?? this.hasConflict,
      structureStatus: structureStatus ?? this.structureStatus,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      deletedAt: deletedAt ?? this.deletedAt,
      syncError: syncError ?? this.syncError,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (projectId.present) {
      map['project_id'] = Variable<String>(projectId.value);
    }
    if (parentId.present) {
      map['parent_id'] = Variable<String>(parentId.value);
    }
    if (kind.present) {
      map['kind'] = Variable<String>(kind.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (sortKey.present) {
      map['sort_key'] = Variable<String>(sortKey.value);
    }
    if (revision.present) {
      map['revision'] = Variable<int>(revision.value);
    }
    if (hasConflict.present) {
      map['has_conflict'] = Variable<bool>(hasConflict.value);
    }
    if (structureStatus.present) {
      map['structure_status'] = Variable<String>(structureStatus.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (updatedAt.present) {
      map['updated_at'] = Variable<DateTime>(updatedAt.value);
    }
    if (deletedAt.present) {
      map['deleted_at'] = Variable<DateTime>(deletedAt.value);
    }
    if (syncError.present) {
      map['sync_error'] = Variable<String>(syncError.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('NodesCompanion(')
          ..write('id: $id, ')
          ..write('projectId: $projectId, ')
          ..write('parentId: $parentId, ')
          ..write('kind: $kind, ')
          ..write('name: $name, ')
          ..write('sortKey: $sortKey, ')
          ..write('revision: $revision, ')
          ..write('hasConflict: $hasConflict, ')
          ..write('structureStatus: $structureStatus, ')
          ..write('createdAt: $createdAt, ')
          ..write('updatedAt: $updatedAt, ')
          ..write('deletedAt: $deletedAt, ')
          ..write('syncError: $syncError, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $NodeContentsTable extends NodeContents
    with TableInfo<$NodeContentsTable, NodeContent> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $NodeContentsTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _nodeIdMeta = const VerificationMeta('nodeId');
  @override
  late final GeneratedColumn<String> nodeId = GeneratedColumn<String>(
      'node_id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _rawContentMeta =
      const VerificationMeta('rawContent');
  @override
  late final GeneratedColumn<String> rawContent = GeneratedColumn<String>(
      'raw_content', aliasedName, false,
      type: DriftSqlType.string,
      requiredDuringInsert: false,
      defaultValue: const Constant(''));
  static const VerificationMeta _rawRevisionMeta =
      const VerificationMeta('rawRevision');
  @override
  late final GeneratedColumn<int> rawRevision = GeneratedColumn<int>(
      'raw_revision', aliasedName, false,
      type: DriftSqlType.int,
      requiredDuringInsert: false,
      defaultValue: const Constant(0));
  static const VerificationMeta _structuredContentMeta =
      const VerificationMeta('structuredContent');
  @override
  late final GeneratedColumn<String> structuredContent =
      GeneratedColumn<String>('structured_content', aliasedName, true,
          type: DriftSqlType.string, requiredDuringInsert: false);
  static const VerificationMeta _structuredRevisionMeta =
      const VerificationMeta('structuredRevision');
  @override
  late final GeneratedColumn<int> structuredRevision = GeneratedColumn<int>(
      'structured_revision', aliasedName, false,
      type: DriftSqlType.int,
      requiredDuringInsert: false,
      defaultValue: const Constant(0));
  static const VerificationMeta _structureProposalMeta =
      const VerificationMeta('structureProposal');
  @override
  late final GeneratedColumn<String> structureProposal =
      GeneratedColumn<String>('structure_proposal', aliasedName, true,
          type: DriftSqlType.string, requiredDuringInsert: false);
  @override
  List<GeneratedColumn> get $columns => [
        nodeId,
        rawContent,
        rawRevision,
        structuredContent,
        structuredRevision,
        structureProposal
      ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'node_contents';
  @override
  VerificationContext validateIntegrity(Insertable<NodeContent> instance,
      {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('node_id')) {
      context.handle(_nodeIdMeta,
          nodeId.isAcceptableOrUnknown(data['node_id']!, _nodeIdMeta));
    } else if (isInserting) {
      context.missing(_nodeIdMeta);
    }
    if (data.containsKey('raw_content')) {
      context.handle(
          _rawContentMeta,
          rawContent.isAcceptableOrUnknown(
              data['raw_content']!, _rawContentMeta));
    }
    if (data.containsKey('raw_revision')) {
      context.handle(
          _rawRevisionMeta,
          rawRevision.isAcceptableOrUnknown(
              data['raw_revision']!, _rawRevisionMeta));
    }
    if (data.containsKey('structured_content')) {
      context.handle(
          _structuredContentMeta,
          structuredContent.isAcceptableOrUnknown(
              data['structured_content']!, _structuredContentMeta));
    }
    if (data.containsKey('structured_revision')) {
      context.handle(
          _structuredRevisionMeta,
          structuredRevision.isAcceptableOrUnknown(
              data['structured_revision']!, _structuredRevisionMeta));
    }
    if (data.containsKey('structure_proposal')) {
      context.handle(
          _structureProposalMeta,
          structureProposal.isAcceptableOrUnknown(
              data['structure_proposal']!, _structureProposalMeta));
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {nodeId};
  @override
  NodeContent map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return NodeContent(
      nodeId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}node_id'])!,
      rawContent: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}raw_content'])!,
      rawRevision: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}raw_revision'])!,
      structuredContent: attachedDatabase.typeMapping.read(
          DriftSqlType.string, data['${effectivePrefix}structured_content']),
      structuredRevision: attachedDatabase.typeMapping.read(
          DriftSqlType.int, data['${effectivePrefix}structured_revision'])!,
      structureProposal: attachedDatabase.typeMapping.read(
          DriftSqlType.string, data['${effectivePrefix}structure_proposal']),
    );
  }

  @override
  $NodeContentsTable createAlias(String alias) {
    return $NodeContentsTable(attachedDatabase, alias);
  }
}

class NodeContent extends DataClass implements Insertable<NodeContent> {
  final String nodeId;
  final String rawContent;

  /// Ревизия текста, подтверждённая сервером. Это base_revision для set_raw_content.
  final int rawRevision;
  final String? structuredContent;

  /// Ревизия структуры, подтверждённая сервером. Это base_revision для set_structured_text. С v2.
  final int structuredRevision;

  /// Результат ИИ, который не применён автоматически (JSON). С v2.
  final String? structureProposal;
  const NodeContent(
      {required this.nodeId,
      required this.rawContent,
      required this.rawRevision,
      this.structuredContent,
      required this.structuredRevision,
      this.structureProposal});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['node_id'] = Variable<String>(nodeId);
    map['raw_content'] = Variable<String>(rawContent);
    map['raw_revision'] = Variable<int>(rawRevision);
    if (!nullToAbsent || structuredContent != null) {
      map['structured_content'] = Variable<String>(structuredContent);
    }
    map['structured_revision'] = Variable<int>(structuredRevision);
    if (!nullToAbsent || structureProposal != null) {
      map['structure_proposal'] = Variable<String>(structureProposal);
    }
    return map;
  }

  NodeContentsCompanion toCompanion(bool nullToAbsent) {
    return NodeContentsCompanion(
      nodeId: Value(nodeId),
      rawContent: Value(rawContent),
      rawRevision: Value(rawRevision),
      structuredContent: structuredContent == null && nullToAbsent
          ? const Value.absent()
          : Value(structuredContent),
      structuredRevision: Value(structuredRevision),
      structureProposal: structureProposal == null && nullToAbsent
          ? const Value.absent()
          : Value(structureProposal),
    );
  }

  factory NodeContent.fromJson(Map<String, dynamic> json,
      {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return NodeContent(
      nodeId: serializer.fromJson<String>(json['nodeId']),
      rawContent: serializer.fromJson<String>(json['rawContent']),
      rawRevision: serializer.fromJson<int>(json['rawRevision']),
      structuredContent:
          serializer.fromJson<String?>(json['structuredContent']),
      structuredRevision: serializer.fromJson<int>(json['structuredRevision']),
      structureProposal:
          serializer.fromJson<String?>(json['structureProposal']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'nodeId': serializer.toJson<String>(nodeId),
      'rawContent': serializer.toJson<String>(rawContent),
      'rawRevision': serializer.toJson<int>(rawRevision),
      'structuredContent': serializer.toJson<String?>(structuredContent),
      'structuredRevision': serializer.toJson<int>(structuredRevision),
      'structureProposal': serializer.toJson<String?>(structureProposal),
    };
  }

  NodeContent copyWith(
          {String? nodeId,
          String? rawContent,
          int? rawRevision,
          Value<String?> structuredContent = const Value.absent(),
          int? structuredRevision,
          Value<String?> structureProposal = const Value.absent()}) =>
      NodeContent(
        nodeId: nodeId ?? this.nodeId,
        rawContent: rawContent ?? this.rawContent,
        rawRevision: rawRevision ?? this.rawRevision,
        structuredContent: structuredContent.present
            ? structuredContent.value
            : this.structuredContent,
        structuredRevision: structuredRevision ?? this.structuredRevision,
        structureProposal: structureProposal.present
            ? structureProposal.value
            : this.structureProposal,
      );
  NodeContent copyWithCompanion(NodeContentsCompanion data) {
    return NodeContent(
      nodeId: data.nodeId.present ? data.nodeId.value : this.nodeId,
      rawContent:
          data.rawContent.present ? data.rawContent.value : this.rawContent,
      rawRevision:
          data.rawRevision.present ? data.rawRevision.value : this.rawRevision,
      structuredContent: data.structuredContent.present
          ? data.structuredContent.value
          : this.structuredContent,
      structuredRevision: data.structuredRevision.present
          ? data.structuredRevision.value
          : this.structuredRevision,
      structureProposal: data.structureProposal.present
          ? data.structureProposal.value
          : this.structureProposal,
    );
  }

  @override
  String toString() {
    return (StringBuffer('NodeContent(')
          ..write('nodeId: $nodeId, ')
          ..write('rawContent: $rawContent, ')
          ..write('rawRevision: $rawRevision, ')
          ..write('structuredContent: $structuredContent, ')
          ..write('structuredRevision: $structuredRevision, ')
          ..write('structureProposal: $structureProposal')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(nodeId, rawContent, rawRevision,
      structuredContent, structuredRevision, structureProposal);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is NodeContent &&
          other.nodeId == this.nodeId &&
          other.rawContent == this.rawContent &&
          other.rawRevision == this.rawRevision &&
          other.structuredContent == this.structuredContent &&
          other.structuredRevision == this.structuredRevision &&
          other.structureProposal == this.structureProposal);
}

class NodeContentsCompanion extends UpdateCompanion<NodeContent> {
  final Value<String> nodeId;
  final Value<String> rawContent;
  final Value<int> rawRevision;
  final Value<String?> structuredContent;
  final Value<int> structuredRevision;
  final Value<String?> structureProposal;
  final Value<int> rowid;
  const NodeContentsCompanion({
    this.nodeId = const Value.absent(),
    this.rawContent = const Value.absent(),
    this.rawRevision = const Value.absent(),
    this.structuredContent = const Value.absent(),
    this.structuredRevision = const Value.absent(),
    this.structureProposal = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  NodeContentsCompanion.insert({
    required String nodeId,
    this.rawContent = const Value.absent(),
    this.rawRevision = const Value.absent(),
    this.structuredContent = const Value.absent(),
    this.structuredRevision = const Value.absent(),
    this.structureProposal = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : nodeId = Value(nodeId);
  static Insertable<NodeContent> custom({
    Expression<String>? nodeId,
    Expression<String>? rawContent,
    Expression<int>? rawRevision,
    Expression<String>? structuredContent,
    Expression<int>? structuredRevision,
    Expression<String>? structureProposal,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (nodeId != null) 'node_id': nodeId,
      if (rawContent != null) 'raw_content': rawContent,
      if (rawRevision != null) 'raw_revision': rawRevision,
      if (structuredContent != null) 'structured_content': structuredContent,
      if (structuredRevision != null) 'structured_revision': structuredRevision,
      if (structureProposal != null) 'structure_proposal': structureProposal,
      if (rowid != null) 'rowid': rowid,
    });
  }

  NodeContentsCompanion copyWith(
      {Value<String>? nodeId,
      Value<String>? rawContent,
      Value<int>? rawRevision,
      Value<String?>? structuredContent,
      Value<int>? structuredRevision,
      Value<String?>? structureProposal,
      Value<int>? rowid}) {
    return NodeContentsCompanion(
      nodeId: nodeId ?? this.nodeId,
      rawContent: rawContent ?? this.rawContent,
      rawRevision: rawRevision ?? this.rawRevision,
      structuredContent: structuredContent ?? this.structuredContent,
      structuredRevision: structuredRevision ?? this.structuredRevision,
      structureProposal: structureProposal ?? this.structureProposal,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (nodeId.present) {
      map['node_id'] = Variable<String>(nodeId.value);
    }
    if (rawContent.present) {
      map['raw_content'] = Variable<String>(rawContent.value);
    }
    if (rawRevision.present) {
      map['raw_revision'] = Variable<int>(rawRevision.value);
    }
    if (structuredContent.present) {
      map['structured_content'] = Variable<String>(structuredContent.value);
    }
    if (structuredRevision.present) {
      map['structured_revision'] = Variable<int>(structuredRevision.value);
    }
    if (structureProposal.present) {
      map['structure_proposal'] = Variable<String>(structureProposal.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('NodeContentsCompanion(')
          ..write('nodeId: $nodeId, ')
          ..write('rawContent: $rawContent, ')
          ..write('rawRevision: $rawRevision, ')
          ..write('structuredContent: $structuredContent, ')
          ..write('structuredRevision: $structuredRevision, ')
          ..write('structureProposal: $structureProposal, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $ContentVersionsTable extends ContentVersions
    with TableInfo<$ContentVersionsTable, ContentVersion> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $ContentVersionsTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
      'id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _nodeIdMeta = const VerificationMeta('nodeId');
  @override
  late final GeneratedColumn<String> nodeId = GeneratedColumn<String>(
      'node_id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _projectIdMeta =
      const VerificationMeta('projectId');
  @override
  late final GeneratedColumn<String> projectId = GeneratedColumn<String>(
      'project_id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _fieldMeta = const VerificationMeta('field');
  @override
  late final GeneratedColumn<String> field = GeneratedColumn<String>(
      'field', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _contentMeta =
      const VerificationMeta('content');
  @override
  late final GeneratedColumn<String> content = GeneratedColumn<String>(
      'content', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _atRevisionMeta =
      const VerificationMeta('atRevision');
  @override
  late final GeneratedColumn<int> atRevision = GeneratedColumn<int>(
      'at_revision', aliasedName, false,
      type: DriftSqlType.int, requiredDuringInsert: true);
  static const VerificationMeta _reasonMeta = const VerificationMeta('reason');
  @override
  late final GeneratedColumn<String> reason = GeneratedColumn<String>(
      'reason', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _serverRevisionMeta =
      const VerificationMeta('serverRevision');
  @override
  late final GeneratedColumn<int> serverRevision = GeneratedColumn<int>(
      'server_revision', aliasedName, false,
      type: DriftSqlType.int, requiredDuringInsert: true);
  static const VerificationMeta _createdAtMeta =
      const VerificationMeta('createdAt');
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
      'created_at', aliasedName, false,
      type: DriftSqlType.dateTime, requiredDuringInsert: true);
  @override
  List<GeneratedColumn> get $columns => [
        id,
        nodeId,
        projectId,
        field,
        content,
        atRevision,
        reason,
        serverRevision,
        createdAt
      ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'content_versions';
  @override
  VerificationContext validateIntegrity(Insertable<ContentVersion> instance,
      {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('node_id')) {
      context.handle(_nodeIdMeta,
          nodeId.isAcceptableOrUnknown(data['node_id']!, _nodeIdMeta));
    } else if (isInserting) {
      context.missing(_nodeIdMeta);
    }
    if (data.containsKey('project_id')) {
      context.handle(_projectIdMeta,
          projectId.isAcceptableOrUnknown(data['project_id']!, _projectIdMeta));
    } else if (isInserting) {
      context.missing(_projectIdMeta);
    }
    if (data.containsKey('field')) {
      context.handle(
          _fieldMeta, field.isAcceptableOrUnknown(data['field']!, _fieldMeta));
    } else if (isInserting) {
      context.missing(_fieldMeta);
    }
    if (data.containsKey('content')) {
      context.handle(_contentMeta,
          content.isAcceptableOrUnknown(data['content']!, _contentMeta));
    } else if (isInserting) {
      context.missing(_contentMeta);
    }
    if (data.containsKey('at_revision')) {
      context.handle(
          _atRevisionMeta,
          atRevision.isAcceptableOrUnknown(
              data['at_revision']!, _atRevisionMeta));
    } else if (isInserting) {
      context.missing(_atRevisionMeta);
    }
    if (data.containsKey('reason')) {
      context.handle(_reasonMeta,
          reason.isAcceptableOrUnknown(data['reason']!, _reasonMeta));
    } else if (isInserting) {
      context.missing(_reasonMeta);
    }
    if (data.containsKey('server_revision')) {
      context.handle(
          _serverRevisionMeta,
          serverRevision.isAcceptableOrUnknown(
              data['server_revision']!, _serverRevisionMeta));
    } else if (isInserting) {
      context.missing(_serverRevisionMeta);
    }
    if (data.containsKey('created_at')) {
      context.handle(_createdAtMeta,
          createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta));
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  ContentVersion map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return ContentVersion(
      id: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}id'])!,
      nodeId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}node_id'])!,
      projectId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}project_id'])!,
      field: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}field'])!,
      content: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}content'])!,
      atRevision: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}at_revision'])!,
      reason: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}reason'])!,
      serverRevision: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}server_revision'])!,
      createdAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}created_at'])!,
    );
  }

  @override
  $ContentVersionsTable createAlias(String alias) {
    return $ContentVersionsTable(attachedDatabase, alias);
  }
}

class ContentVersion extends DataClass implements Insertable<ContentVersion> {
  final String id;
  final String nodeId;
  final String projectId;
  final String field;
  final String content;
  final int atRevision;
  final String reason;
  final int serverRevision;
  final DateTime createdAt;
  const ContentVersion(
      {required this.id,
      required this.nodeId,
      required this.projectId,
      required this.field,
      required this.content,
      required this.atRevision,
      required this.reason,
      required this.serverRevision,
      required this.createdAt});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['node_id'] = Variable<String>(nodeId);
    map['project_id'] = Variable<String>(projectId);
    map['field'] = Variable<String>(field);
    map['content'] = Variable<String>(content);
    map['at_revision'] = Variable<int>(atRevision);
    map['reason'] = Variable<String>(reason);
    map['server_revision'] = Variable<int>(serverRevision);
    map['created_at'] = Variable<DateTime>(createdAt);
    return map;
  }

  ContentVersionsCompanion toCompanion(bool nullToAbsent) {
    return ContentVersionsCompanion(
      id: Value(id),
      nodeId: Value(nodeId),
      projectId: Value(projectId),
      field: Value(field),
      content: Value(content),
      atRevision: Value(atRevision),
      reason: Value(reason),
      serverRevision: Value(serverRevision),
      createdAt: Value(createdAt),
    );
  }

  factory ContentVersion.fromJson(Map<String, dynamic> json,
      {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return ContentVersion(
      id: serializer.fromJson<String>(json['id']),
      nodeId: serializer.fromJson<String>(json['nodeId']),
      projectId: serializer.fromJson<String>(json['projectId']),
      field: serializer.fromJson<String>(json['field']),
      content: serializer.fromJson<String>(json['content']),
      atRevision: serializer.fromJson<int>(json['atRevision']),
      reason: serializer.fromJson<String>(json['reason']),
      serverRevision: serializer.fromJson<int>(json['serverRevision']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'nodeId': serializer.toJson<String>(nodeId),
      'projectId': serializer.toJson<String>(projectId),
      'field': serializer.toJson<String>(field),
      'content': serializer.toJson<String>(content),
      'atRevision': serializer.toJson<int>(atRevision),
      'reason': serializer.toJson<String>(reason),
      'serverRevision': serializer.toJson<int>(serverRevision),
      'createdAt': serializer.toJson<DateTime>(createdAt),
    };
  }

  ContentVersion copyWith(
          {String? id,
          String? nodeId,
          String? projectId,
          String? field,
          String? content,
          int? atRevision,
          String? reason,
          int? serverRevision,
          DateTime? createdAt}) =>
      ContentVersion(
        id: id ?? this.id,
        nodeId: nodeId ?? this.nodeId,
        projectId: projectId ?? this.projectId,
        field: field ?? this.field,
        content: content ?? this.content,
        atRevision: atRevision ?? this.atRevision,
        reason: reason ?? this.reason,
        serverRevision: serverRevision ?? this.serverRevision,
        createdAt: createdAt ?? this.createdAt,
      );
  ContentVersion copyWithCompanion(ContentVersionsCompanion data) {
    return ContentVersion(
      id: data.id.present ? data.id.value : this.id,
      nodeId: data.nodeId.present ? data.nodeId.value : this.nodeId,
      projectId: data.projectId.present ? data.projectId.value : this.projectId,
      field: data.field.present ? data.field.value : this.field,
      content: data.content.present ? data.content.value : this.content,
      atRevision:
          data.atRevision.present ? data.atRevision.value : this.atRevision,
      reason: data.reason.present ? data.reason.value : this.reason,
      serverRevision: data.serverRevision.present
          ? data.serverRevision.value
          : this.serverRevision,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('ContentVersion(')
          ..write('id: $id, ')
          ..write('nodeId: $nodeId, ')
          ..write('projectId: $projectId, ')
          ..write('field: $field, ')
          ..write('content: $content, ')
          ..write('atRevision: $atRevision, ')
          ..write('reason: $reason, ')
          ..write('serverRevision: $serverRevision, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(id, nodeId, projectId, field, content,
      atRevision, reason, serverRevision, createdAt);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is ContentVersion &&
          other.id == this.id &&
          other.nodeId == this.nodeId &&
          other.projectId == this.projectId &&
          other.field == this.field &&
          other.content == this.content &&
          other.atRevision == this.atRevision &&
          other.reason == this.reason &&
          other.serverRevision == this.serverRevision &&
          other.createdAt == this.createdAt);
}

class ContentVersionsCompanion extends UpdateCompanion<ContentVersion> {
  final Value<String> id;
  final Value<String> nodeId;
  final Value<String> projectId;
  final Value<String> field;
  final Value<String> content;
  final Value<int> atRevision;
  final Value<String> reason;
  final Value<int> serverRevision;
  final Value<DateTime> createdAt;
  final Value<int> rowid;
  const ContentVersionsCompanion({
    this.id = const Value.absent(),
    this.nodeId = const Value.absent(),
    this.projectId = const Value.absent(),
    this.field = const Value.absent(),
    this.content = const Value.absent(),
    this.atRevision = const Value.absent(),
    this.reason = const Value.absent(),
    this.serverRevision = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  ContentVersionsCompanion.insert({
    required String id,
    required String nodeId,
    required String projectId,
    required String field,
    required String content,
    required int atRevision,
    required String reason,
    required int serverRevision,
    required DateTime createdAt,
    this.rowid = const Value.absent(),
  })  : id = Value(id),
        nodeId = Value(nodeId),
        projectId = Value(projectId),
        field = Value(field),
        content = Value(content),
        atRevision = Value(atRevision),
        reason = Value(reason),
        serverRevision = Value(serverRevision),
        createdAt = Value(createdAt);
  static Insertable<ContentVersion> custom({
    Expression<String>? id,
    Expression<String>? nodeId,
    Expression<String>? projectId,
    Expression<String>? field,
    Expression<String>? content,
    Expression<int>? atRevision,
    Expression<String>? reason,
    Expression<int>? serverRevision,
    Expression<DateTime>? createdAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (nodeId != null) 'node_id': nodeId,
      if (projectId != null) 'project_id': projectId,
      if (field != null) 'field': field,
      if (content != null) 'content': content,
      if (atRevision != null) 'at_revision': atRevision,
      if (reason != null) 'reason': reason,
      if (serverRevision != null) 'server_revision': serverRevision,
      if (createdAt != null) 'created_at': createdAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  ContentVersionsCompanion copyWith(
      {Value<String>? id,
      Value<String>? nodeId,
      Value<String>? projectId,
      Value<String>? field,
      Value<String>? content,
      Value<int>? atRevision,
      Value<String>? reason,
      Value<int>? serverRevision,
      Value<DateTime>? createdAt,
      Value<int>? rowid}) {
    return ContentVersionsCompanion(
      id: id ?? this.id,
      nodeId: nodeId ?? this.nodeId,
      projectId: projectId ?? this.projectId,
      field: field ?? this.field,
      content: content ?? this.content,
      atRevision: atRevision ?? this.atRevision,
      reason: reason ?? this.reason,
      serverRevision: serverRevision ?? this.serverRevision,
      createdAt: createdAt ?? this.createdAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (nodeId.present) {
      map['node_id'] = Variable<String>(nodeId.value);
    }
    if (projectId.present) {
      map['project_id'] = Variable<String>(projectId.value);
    }
    if (field.present) {
      map['field'] = Variable<String>(field.value);
    }
    if (content.present) {
      map['content'] = Variable<String>(content.value);
    }
    if (atRevision.present) {
      map['at_revision'] = Variable<int>(atRevision.value);
    }
    if (reason.present) {
      map['reason'] = Variable<String>(reason.value);
    }
    if (serverRevision.present) {
      map['server_revision'] = Variable<int>(serverRevision.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('ContentVersionsCompanion(')
          ..write('id: $id, ')
          ..write('nodeId: $nodeId, ')
          ..write('projectId: $projectId, ')
          ..write('field: $field, ')
          ..write('content: $content, ')
          ..write('atRevision: $atRevision, ')
          ..write('reason: $reason, ')
          ..write('serverRevision: $serverRevision, ')
          ..write('createdAt: $createdAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $OutboxTable extends Outbox with TableInfo<$OutboxTable, OutboxEntry> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $OutboxTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _seqMeta = const VerificationMeta('seq');
  @override
  late final GeneratedColumn<int> seq = GeneratedColumn<int>(
      'seq', aliasedName, false,
      hasAutoIncrement: true,
      type: DriftSqlType.int,
      requiredDuringInsert: false,
      defaultConstraints:
          GeneratedColumn.constraintIsAlways('PRIMARY KEY AUTOINCREMENT'));
  static const VerificationMeta _operationIdMeta =
      const VerificationMeta('operationId');
  @override
  late final GeneratedColumn<String> operationId = GeneratedColumn<String>(
      'operation_id', aliasedName, false,
      type: DriftSqlType.string,
      requiredDuringInsert: true,
      defaultConstraints: GeneratedColumn.constraintIsAlways('UNIQUE'));
  static const VerificationMeta _typeMeta = const VerificationMeta('type');
  @override
  late final GeneratedColumn<String> type = GeneratedColumn<String>(
      'type', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _entityIdMeta =
      const VerificationMeta('entityId');
  @override
  late final GeneratedColumn<String> entityId = GeneratedColumn<String>(
      'entity_id', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _baseRevisionMeta =
      const VerificationMeta('baseRevision');
  @override
  late final GeneratedColumn<int> baseRevision = GeneratedColumn<int>(
      'base_revision', aliasedName, false,
      type: DriftSqlType.int,
      requiredDuringInsert: false,
      defaultValue: const Constant(0));
  static const VerificationMeta _payloadMeta =
      const VerificationMeta('payload');
  @override
  late final GeneratedColumn<String> payload = GeneratedColumn<String>(
      'payload', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _inFlightMeta =
      const VerificationMeta('inFlight');
  @override
  late final GeneratedColumn<bool> inFlight = GeneratedColumn<bool>(
      'in_flight', aliasedName, false,
      type: DriftSqlType.bool,
      requiredDuringInsert: false,
      defaultConstraints:
          GeneratedColumn.constraintIsAlways('CHECK ("in_flight" IN (0, 1))'),
      defaultValue: const Constant(false));
  static const VerificationMeta _createdAtMeta =
      const VerificationMeta('createdAt');
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
      'created_at', aliasedName, false,
      type: DriftSqlType.dateTime, requiredDuringInsert: true);
  @override
  List<GeneratedColumn> get $columns => [
        seq,
        operationId,
        type,
        entityId,
        baseRevision,
        payload,
        inFlight,
        createdAt
      ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'outbox';
  @override
  VerificationContext validateIntegrity(Insertable<OutboxEntry> instance,
      {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('seq')) {
      context.handle(
          _seqMeta, seq.isAcceptableOrUnknown(data['seq']!, _seqMeta));
    }
    if (data.containsKey('operation_id')) {
      context.handle(
          _operationIdMeta,
          operationId.isAcceptableOrUnknown(
              data['operation_id']!, _operationIdMeta));
    } else if (isInserting) {
      context.missing(_operationIdMeta);
    }
    if (data.containsKey('type')) {
      context.handle(
          _typeMeta, type.isAcceptableOrUnknown(data['type']!, _typeMeta));
    } else if (isInserting) {
      context.missing(_typeMeta);
    }
    if (data.containsKey('entity_id')) {
      context.handle(_entityIdMeta,
          entityId.isAcceptableOrUnknown(data['entity_id']!, _entityIdMeta));
    } else if (isInserting) {
      context.missing(_entityIdMeta);
    }
    if (data.containsKey('base_revision')) {
      context.handle(
          _baseRevisionMeta,
          baseRevision.isAcceptableOrUnknown(
              data['base_revision']!, _baseRevisionMeta));
    }
    if (data.containsKey('payload')) {
      context.handle(_payloadMeta,
          payload.isAcceptableOrUnknown(data['payload']!, _payloadMeta));
    } else if (isInserting) {
      context.missing(_payloadMeta);
    }
    if (data.containsKey('in_flight')) {
      context.handle(_inFlightMeta,
          inFlight.isAcceptableOrUnknown(data['in_flight']!, _inFlightMeta));
    }
    if (data.containsKey('created_at')) {
      context.handle(_createdAtMeta,
          createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta));
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {seq};
  @override
  OutboxEntry map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return OutboxEntry(
      seq: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}seq'])!,
      operationId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}operation_id'])!,
      type: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}type'])!,
      entityId: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}entity_id'])!,
      baseRevision: attachedDatabase.typeMapping
          .read(DriftSqlType.int, data['${effectivePrefix}base_revision'])!,
      payload: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}payload'])!,
      inFlight: attachedDatabase.typeMapping
          .read(DriftSqlType.bool, data['${effectivePrefix}in_flight'])!,
      createdAt: attachedDatabase.typeMapping
          .read(DriftSqlType.dateTime, data['${effectivePrefix}created_at'])!,
    );
  }

  @override
  $OutboxTable createAlias(String alias) {
    return $OutboxTable(attachedDatabase, alias);
  }
}

class OutboxEntry extends DataClass implements Insertable<OutboxEntry> {
  final int seq;
  final String operationId;
  final String type;
  final String entityId;
  final int baseRevision;
  final String payload;
  final bool inFlight;
  final DateTime createdAt;
  const OutboxEntry(
      {required this.seq,
      required this.operationId,
      required this.type,
      required this.entityId,
      required this.baseRevision,
      required this.payload,
      required this.inFlight,
      required this.createdAt});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['seq'] = Variable<int>(seq);
    map['operation_id'] = Variable<String>(operationId);
    map['type'] = Variable<String>(type);
    map['entity_id'] = Variable<String>(entityId);
    map['base_revision'] = Variable<int>(baseRevision);
    map['payload'] = Variable<String>(payload);
    map['in_flight'] = Variable<bool>(inFlight);
    map['created_at'] = Variable<DateTime>(createdAt);
    return map;
  }

  OutboxCompanion toCompanion(bool nullToAbsent) {
    return OutboxCompanion(
      seq: Value(seq),
      operationId: Value(operationId),
      type: Value(type),
      entityId: Value(entityId),
      baseRevision: Value(baseRevision),
      payload: Value(payload),
      inFlight: Value(inFlight),
      createdAt: Value(createdAt),
    );
  }

  factory OutboxEntry.fromJson(Map<String, dynamic> json,
      {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return OutboxEntry(
      seq: serializer.fromJson<int>(json['seq']),
      operationId: serializer.fromJson<String>(json['operationId']),
      type: serializer.fromJson<String>(json['type']),
      entityId: serializer.fromJson<String>(json['entityId']),
      baseRevision: serializer.fromJson<int>(json['baseRevision']),
      payload: serializer.fromJson<String>(json['payload']),
      inFlight: serializer.fromJson<bool>(json['inFlight']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'seq': serializer.toJson<int>(seq),
      'operationId': serializer.toJson<String>(operationId),
      'type': serializer.toJson<String>(type),
      'entityId': serializer.toJson<String>(entityId),
      'baseRevision': serializer.toJson<int>(baseRevision),
      'payload': serializer.toJson<String>(payload),
      'inFlight': serializer.toJson<bool>(inFlight),
      'createdAt': serializer.toJson<DateTime>(createdAt),
    };
  }

  OutboxEntry copyWith(
          {int? seq,
          String? operationId,
          String? type,
          String? entityId,
          int? baseRevision,
          String? payload,
          bool? inFlight,
          DateTime? createdAt}) =>
      OutboxEntry(
        seq: seq ?? this.seq,
        operationId: operationId ?? this.operationId,
        type: type ?? this.type,
        entityId: entityId ?? this.entityId,
        baseRevision: baseRevision ?? this.baseRevision,
        payload: payload ?? this.payload,
        inFlight: inFlight ?? this.inFlight,
        createdAt: createdAt ?? this.createdAt,
      );
  OutboxEntry copyWithCompanion(OutboxCompanion data) {
    return OutboxEntry(
      seq: data.seq.present ? data.seq.value : this.seq,
      operationId:
          data.operationId.present ? data.operationId.value : this.operationId,
      type: data.type.present ? data.type.value : this.type,
      entityId: data.entityId.present ? data.entityId.value : this.entityId,
      baseRevision: data.baseRevision.present
          ? data.baseRevision.value
          : this.baseRevision,
      payload: data.payload.present ? data.payload.value : this.payload,
      inFlight: data.inFlight.present ? data.inFlight.value : this.inFlight,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('OutboxEntry(')
          ..write('seq: $seq, ')
          ..write('operationId: $operationId, ')
          ..write('type: $type, ')
          ..write('entityId: $entityId, ')
          ..write('baseRevision: $baseRevision, ')
          ..write('payload: $payload, ')
          ..write('inFlight: $inFlight, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(seq, operationId, type, entityId,
      baseRevision, payload, inFlight, createdAt);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is OutboxEntry &&
          other.seq == this.seq &&
          other.operationId == this.operationId &&
          other.type == this.type &&
          other.entityId == this.entityId &&
          other.baseRevision == this.baseRevision &&
          other.payload == this.payload &&
          other.inFlight == this.inFlight &&
          other.createdAt == this.createdAt);
}

class OutboxCompanion extends UpdateCompanion<OutboxEntry> {
  final Value<int> seq;
  final Value<String> operationId;
  final Value<String> type;
  final Value<String> entityId;
  final Value<int> baseRevision;
  final Value<String> payload;
  final Value<bool> inFlight;
  final Value<DateTime> createdAt;
  const OutboxCompanion({
    this.seq = const Value.absent(),
    this.operationId = const Value.absent(),
    this.type = const Value.absent(),
    this.entityId = const Value.absent(),
    this.baseRevision = const Value.absent(),
    this.payload = const Value.absent(),
    this.inFlight = const Value.absent(),
    this.createdAt = const Value.absent(),
  });
  OutboxCompanion.insert({
    this.seq = const Value.absent(),
    required String operationId,
    required String type,
    required String entityId,
    this.baseRevision = const Value.absent(),
    required String payload,
    this.inFlight = const Value.absent(),
    required DateTime createdAt,
  })  : operationId = Value(operationId),
        type = Value(type),
        entityId = Value(entityId),
        payload = Value(payload),
        createdAt = Value(createdAt);
  static Insertable<OutboxEntry> custom({
    Expression<int>? seq,
    Expression<String>? operationId,
    Expression<String>? type,
    Expression<String>? entityId,
    Expression<int>? baseRevision,
    Expression<String>? payload,
    Expression<bool>? inFlight,
    Expression<DateTime>? createdAt,
  }) {
    return RawValuesInsertable({
      if (seq != null) 'seq': seq,
      if (operationId != null) 'operation_id': operationId,
      if (type != null) 'type': type,
      if (entityId != null) 'entity_id': entityId,
      if (baseRevision != null) 'base_revision': baseRevision,
      if (payload != null) 'payload': payload,
      if (inFlight != null) 'in_flight': inFlight,
      if (createdAt != null) 'created_at': createdAt,
    });
  }

  OutboxCompanion copyWith(
      {Value<int>? seq,
      Value<String>? operationId,
      Value<String>? type,
      Value<String>? entityId,
      Value<int>? baseRevision,
      Value<String>? payload,
      Value<bool>? inFlight,
      Value<DateTime>? createdAt}) {
    return OutboxCompanion(
      seq: seq ?? this.seq,
      operationId: operationId ?? this.operationId,
      type: type ?? this.type,
      entityId: entityId ?? this.entityId,
      baseRevision: baseRevision ?? this.baseRevision,
      payload: payload ?? this.payload,
      inFlight: inFlight ?? this.inFlight,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (seq.present) {
      map['seq'] = Variable<int>(seq.value);
    }
    if (operationId.present) {
      map['operation_id'] = Variable<String>(operationId.value);
    }
    if (type.present) {
      map['type'] = Variable<String>(type.value);
    }
    if (entityId.present) {
      map['entity_id'] = Variable<String>(entityId.value);
    }
    if (baseRevision.present) {
      map['base_revision'] = Variable<int>(baseRevision.value);
    }
    if (payload.present) {
      map['payload'] = Variable<String>(payload.value);
    }
    if (inFlight.present) {
      map['in_flight'] = Variable<bool>(inFlight.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('OutboxCompanion(')
          ..write('seq: $seq, ')
          ..write('operationId: $operationId, ')
          ..write('type: $type, ')
          ..write('entityId: $entityId, ')
          ..write('baseRevision: $baseRevision, ')
          ..write('payload: $payload, ')
          ..write('inFlight: $inFlight, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }
}

class $KeyValuesTable extends KeyValues
    with TableInfo<$KeyValuesTable, KeyValue> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $KeyValuesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _keyMeta = const VerificationMeta('key');
  @override
  late final GeneratedColumn<String> key = GeneratedColumn<String>(
      'key', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  static const VerificationMeta _valueMeta = const VerificationMeta('value');
  @override
  late final GeneratedColumn<String> value = GeneratedColumn<String>(
      'value', aliasedName, false,
      type: DriftSqlType.string, requiredDuringInsert: true);
  @override
  List<GeneratedColumn> get $columns => [key, value];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'key_values';
  @override
  VerificationContext validateIntegrity(Insertable<KeyValue> instance,
      {bool isInserting = false}) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('key')) {
      context.handle(
          _keyMeta, key.isAcceptableOrUnknown(data['key']!, _keyMeta));
    } else if (isInserting) {
      context.missing(_keyMeta);
    }
    if (data.containsKey('value')) {
      context.handle(
          _valueMeta, value.isAcceptableOrUnknown(data['value']!, _valueMeta));
    } else if (isInserting) {
      context.missing(_valueMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {key};
  @override
  KeyValue map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return KeyValue(
      key: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}key'])!,
      value: attachedDatabase.typeMapping
          .read(DriftSqlType.string, data['${effectivePrefix}value'])!,
    );
  }

  @override
  $KeyValuesTable createAlias(String alias) {
    return $KeyValuesTable(attachedDatabase, alias);
  }
}

class KeyValue extends DataClass implements Insertable<KeyValue> {
  final String key;
  final String value;
  const KeyValue({required this.key, required this.value});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['key'] = Variable<String>(key);
    map['value'] = Variable<String>(value);
    return map;
  }

  KeyValuesCompanion toCompanion(bool nullToAbsent) {
    return KeyValuesCompanion(
      key: Value(key),
      value: Value(value),
    );
  }

  factory KeyValue.fromJson(Map<String, dynamic> json,
      {ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return KeyValue(
      key: serializer.fromJson<String>(json['key']),
      value: serializer.fromJson<String>(json['value']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'key': serializer.toJson<String>(key),
      'value': serializer.toJson<String>(value),
    };
  }

  KeyValue copyWith({String? key, String? value}) => KeyValue(
        key: key ?? this.key,
        value: value ?? this.value,
      );
  KeyValue copyWithCompanion(KeyValuesCompanion data) {
    return KeyValue(
      key: data.key.present ? data.key.value : this.key,
      value: data.value.present ? data.value.value : this.value,
    );
  }

  @override
  String toString() {
    return (StringBuffer('KeyValue(')
          ..write('key: $key, ')
          ..write('value: $value')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(key, value);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is KeyValue && other.key == this.key && other.value == this.value);
}

class KeyValuesCompanion extends UpdateCompanion<KeyValue> {
  final Value<String> key;
  final Value<String> value;
  final Value<int> rowid;
  const KeyValuesCompanion({
    this.key = const Value.absent(),
    this.value = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  KeyValuesCompanion.insert({
    required String key,
    required String value,
    this.rowid = const Value.absent(),
  })  : key = Value(key),
        value = Value(value);
  static Insertable<KeyValue> custom({
    Expression<String>? key,
    Expression<String>? value,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (key != null) 'key': key,
      if (value != null) 'value': value,
      if (rowid != null) 'rowid': rowid,
    });
  }

  KeyValuesCompanion copyWith(
      {Value<String>? key, Value<String>? value, Value<int>? rowid}) {
    return KeyValuesCompanion(
      key: key ?? this.key,
      value: value ?? this.value,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (key.present) {
      map['key'] = Variable<String>(key.value);
    }
    if (value.present) {
      map['value'] = Variable<String>(value.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('KeyValuesCompanion(')
          ..write('key: $key, ')
          ..write('value: $value, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

abstract class _$AppDatabase extends GeneratedDatabase {
  _$AppDatabase(QueryExecutor e) : super(e);
  $AppDatabaseManager get managers => $AppDatabaseManager(this);
  late final $ProjectsTable projects = $ProjectsTable(this);
  late final $NodesTable nodes = $NodesTable(this);
  late final $NodeContentsTable nodeContents = $NodeContentsTable(this);
  late final $ContentVersionsTable contentVersions =
      $ContentVersionsTable(this);
  late final $OutboxTable outbox = $OutboxTable(this);
  late final $KeyValuesTable keyValues = $KeyValuesTable(this);
  @override
  Iterable<TableInfo<Table, Object?>> get allTables =>
      allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities =>
      [projects, nodes, nodeContents, contentVersions, outbox, keyValues];
}

typedef $$ProjectsTableCreateCompanionBuilder = ProjectsCompanion Function({
  required String id,
  required String name,
  Value<String> description,
  Value<int> revision,
  Value<String> role,
  required DateTime createdAt,
  required DateTime updatedAt,
  Value<DateTime?> deletedAt,
  Value<String?> syncError,
  Value<int> rowid,
});
typedef $$ProjectsTableUpdateCompanionBuilder = ProjectsCompanion Function({
  Value<String> id,
  Value<String> name,
  Value<String> description,
  Value<int> revision,
  Value<String> role,
  Value<DateTime> createdAt,
  Value<DateTime> updatedAt,
  Value<DateTime?> deletedAt,
  Value<String?> syncError,
  Value<int> rowid,
});

class $$ProjectsTableFilterComposer
    extends Composer<_$AppDatabase, $ProjectsTable> {
  $$ProjectsTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
      column: $table.id, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get name => $composableBuilder(
      column: $table.name, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get description => $composableBuilder(
      column: $table.description, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get revision => $composableBuilder(
      column: $table.revision, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get role => $composableBuilder(
      column: $table.role, builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get updatedAt => $composableBuilder(
      column: $table.updatedAt, builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get deletedAt => $composableBuilder(
      column: $table.deletedAt, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get syncError => $composableBuilder(
      column: $table.syncError, builder: (column) => ColumnFilters(column));
}

class $$ProjectsTableOrderingComposer
    extends Composer<_$AppDatabase, $ProjectsTable> {
  $$ProjectsTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
      column: $table.id, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get name => $composableBuilder(
      column: $table.name, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get description => $composableBuilder(
      column: $table.description, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get revision => $composableBuilder(
      column: $table.revision, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get role => $composableBuilder(
      column: $table.role, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get updatedAt => $composableBuilder(
      column: $table.updatedAt, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get deletedAt => $composableBuilder(
      column: $table.deletedAt, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get syncError => $composableBuilder(
      column: $table.syncError, builder: (column) => ColumnOrderings(column));
}

class $$ProjectsTableAnnotationComposer
    extends Composer<_$AppDatabase, $ProjectsTable> {
  $$ProjectsTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<String> get description => $composableBuilder(
      column: $table.description, builder: (column) => column);

  GeneratedColumn<int> get revision =>
      $composableBuilder(column: $table.revision, builder: (column) => column);

  GeneratedColumn<String> get role =>
      $composableBuilder(column: $table.role, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);

  GeneratedColumn<DateTime> get updatedAt =>
      $composableBuilder(column: $table.updatedAt, builder: (column) => column);

  GeneratedColumn<DateTime> get deletedAt =>
      $composableBuilder(column: $table.deletedAt, builder: (column) => column);

  GeneratedColumn<String> get syncError =>
      $composableBuilder(column: $table.syncError, builder: (column) => column);
}

class $$ProjectsTableTableManager extends RootTableManager<
    _$AppDatabase,
    $ProjectsTable,
    Project,
    $$ProjectsTableFilterComposer,
    $$ProjectsTableOrderingComposer,
    $$ProjectsTableAnnotationComposer,
    $$ProjectsTableCreateCompanionBuilder,
    $$ProjectsTableUpdateCompanionBuilder,
    (Project, BaseReferences<_$AppDatabase, $ProjectsTable, Project>),
    Project,
    PrefetchHooks Function()> {
  $$ProjectsTableTableManager(_$AppDatabase db, $ProjectsTable table)
      : super(TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$ProjectsTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$ProjectsTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$ProjectsTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback: ({
            Value<String> id = const Value.absent(),
            Value<String> name = const Value.absent(),
            Value<String> description = const Value.absent(),
            Value<int> revision = const Value.absent(),
            Value<String> role = const Value.absent(),
            Value<DateTime> createdAt = const Value.absent(),
            Value<DateTime> updatedAt = const Value.absent(),
            Value<DateTime?> deletedAt = const Value.absent(),
            Value<String?> syncError = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              ProjectsCompanion(
            id: id,
            name: name,
            description: description,
            revision: revision,
            role: role,
            createdAt: createdAt,
            updatedAt: updatedAt,
            deletedAt: deletedAt,
            syncError: syncError,
            rowid: rowid,
          ),
          createCompanionCallback: ({
            required String id,
            required String name,
            Value<String> description = const Value.absent(),
            Value<int> revision = const Value.absent(),
            Value<String> role = const Value.absent(),
            required DateTime createdAt,
            required DateTime updatedAt,
            Value<DateTime?> deletedAt = const Value.absent(),
            Value<String?> syncError = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              ProjectsCompanion.insert(
            id: id,
            name: name,
            description: description,
            revision: revision,
            role: role,
            createdAt: createdAt,
            updatedAt: updatedAt,
            deletedAt: deletedAt,
            syncError: syncError,
            rowid: rowid,
          ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ));
}

typedef $$ProjectsTableProcessedTableManager = ProcessedTableManager<
    _$AppDatabase,
    $ProjectsTable,
    Project,
    $$ProjectsTableFilterComposer,
    $$ProjectsTableOrderingComposer,
    $$ProjectsTableAnnotationComposer,
    $$ProjectsTableCreateCompanionBuilder,
    $$ProjectsTableUpdateCompanionBuilder,
    (Project, BaseReferences<_$AppDatabase, $ProjectsTable, Project>),
    Project,
    PrefetchHooks Function()>;
typedef $$NodesTableCreateCompanionBuilder = NodesCompanion Function({
  required String id,
  required String projectId,
  Value<String?> parentId,
  required String kind,
  required String name,
  required String sortKey,
  Value<int> revision,
  Value<bool> hasConflict,
  Value<String> structureStatus,
  required DateTime createdAt,
  required DateTime updatedAt,
  Value<DateTime?> deletedAt,
  Value<String?> syncError,
  Value<int> rowid,
});
typedef $$NodesTableUpdateCompanionBuilder = NodesCompanion Function({
  Value<String> id,
  Value<String> projectId,
  Value<String?> parentId,
  Value<String> kind,
  Value<String> name,
  Value<String> sortKey,
  Value<int> revision,
  Value<bool> hasConflict,
  Value<String> structureStatus,
  Value<DateTime> createdAt,
  Value<DateTime> updatedAt,
  Value<DateTime?> deletedAt,
  Value<String?> syncError,
  Value<int> rowid,
});

class $$NodesTableFilterComposer extends Composer<_$AppDatabase, $NodesTable> {
  $$NodesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
      column: $table.id, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get projectId => $composableBuilder(
      column: $table.projectId, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get parentId => $composableBuilder(
      column: $table.parentId, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get kind => $composableBuilder(
      column: $table.kind, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get name => $composableBuilder(
      column: $table.name, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get sortKey => $composableBuilder(
      column: $table.sortKey, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get revision => $composableBuilder(
      column: $table.revision, builder: (column) => ColumnFilters(column));

  ColumnFilters<bool> get hasConflict => $composableBuilder(
      column: $table.hasConflict, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get structureStatus => $composableBuilder(
      column: $table.structureStatus,
      builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get updatedAt => $composableBuilder(
      column: $table.updatedAt, builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get deletedAt => $composableBuilder(
      column: $table.deletedAt, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get syncError => $composableBuilder(
      column: $table.syncError, builder: (column) => ColumnFilters(column));
}

class $$NodesTableOrderingComposer
    extends Composer<_$AppDatabase, $NodesTable> {
  $$NodesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
      column: $table.id, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get projectId => $composableBuilder(
      column: $table.projectId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get parentId => $composableBuilder(
      column: $table.parentId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get kind => $composableBuilder(
      column: $table.kind, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get name => $composableBuilder(
      column: $table.name, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get sortKey => $composableBuilder(
      column: $table.sortKey, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get revision => $composableBuilder(
      column: $table.revision, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<bool> get hasConflict => $composableBuilder(
      column: $table.hasConflict, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get structureStatus => $composableBuilder(
      column: $table.structureStatus,
      builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get updatedAt => $composableBuilder(
      column: $table.updatedAt, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get deletedAt => $composableBuilder(
      column: $table.deletedAt, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get syncError => $composableBuilder(
      column: $table.syncError, builder: (column) => ColumnOrderings(column));
}

class $$NodesTableAnnotationComposer
    extends Composer<_$AppDatabase, $NodesTable> {
  $$NodesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get projectId =>
      $composableBuilder(column: $table.projectId, builder: (column) => column);

  GeneratedColumn<String> get parentId =>
      $composableBuilder(column: $table.parentId, builder: (column) => column);

  GeneratedColumn<String> get kind =>
      $composableBuilder(column: $table.kind, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<String> get sortKey =>
      $composableBuilder(column: $table.sortKey, builder: (column) => column);

  GeneratedColumn<int> get revision =>
      $composableBuilder(column: $table.revision, builder: (column) => column);

  GeneratedColumn<bool> get hasConflict => $composableBuilder(
      column: $table.hasConflict, builder: (column) => column);

  GeneratedColumn<String> get structureStatus => $composableBuilder(
      column: $table.structureStatus, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);

  GeneratedColumn<DateTime> get updatedAt =>
      $composableBuilder(column: $table.updatedAt, builder: (column) => column);

  GeneratedColumn<DateTime> get deletedAt =>
      $composableBuilder(column: $table.deletedAt, builder: (column) => column);

  GeneratedColumn<String> get syncError =>
      $composableBuilder(column: $table.syncError, builder: (column) => column);
}

class $$NodesTableTableManager extends RootTableManager<
    _$AppDatabase,
    $NodesTable,
    TreeNode,
    $$NodesTableFilterComposer,
    $$NodesTableOrderingComposer,
    $$NodesTableAnnotationComposer,
    $$NodesTableCreateCompanionBuilder,
    $$NodesTableUpdateCompanionBuilder,
    (TreeNode, BaseReferences<_$AppDatabase, $NodesTable, TreeNode>),
    TreeNode,
    PrefetchHooks Function()> {
  $$NodesTableTableManager(_$AppDatabase db, $NodesTable table)
      : super(TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$NodesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$NodesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$NodesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback: ({
            Value<String> id = const Value.absent(),
            Value<String> projectId = const Value.absent(),
            Value<String?> parentId = const Value.absent(),
            Value<String> kind = const Value.absent(),
            Value<String> name = const Value.absent(),
            Value<String> sortKey = const Value.absent(),
            Value<int> revision = const Value.absent(),
            Value<bool> hasConflict = const Value.absent(),
            Value<String> structureStatus = const Value.absent(),
            Value<DateTime> createdAt = const Value.absent(),
            Value<DateTime> updatedAt = const Value.absent(),
            Value<DateTime?> deletedAt = const Value.absent(),
            Value<String?> syncError = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              NodesCompanion(
            id: id,
            projectId: projectId,
            parentId: parentId,
            kind: kind,
            name: name,
            sortKey: sortKey,
            revision: revision,
            hasConflict: hasConflict,
            structureStatus: structureStatus,
            createdAt: createdAt,
            updatedAt: updatedAt,
            deletedAt: deletedAt,
            syncError: syncError,
            rowid: rowid,
          ),
          createCompanionCallback: ({
            required String id,
            required String projectId,
            Value<String?> parentId = const Value.absent(),
            required String kind,
            required String name,
            required String sortKey,
            Value<int> revision = const Value.absent(),
            Value<bool> hasConflict = const Value.absent(),
            Value<String> structureStatus = const Value.absent(),
            required DateTime createdAt,
            required DateTime updatedAt,
            Value<DateTime?> deletedAt = const Value.absent(),
            Value<String?> syncError = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              NodesCompanion.insert(
            id: id,
            projectId: projectId,
            parentId: parentId,
            kind: kind,
            name: name,
            sortKey: sortKey,
            revision: revision,
            hasConflict: hasConflict,
            structureStatus: structureStatus,
            createdAt: createdAt,
            updatedAt: updatedAt,
            deletedAt: deletedAt,
            syncError: syncError,
            rowid: rowid,
          ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ));
}

typedef $$NodesTableProcessedTableManager = ProcessedTableManager<
    _$AppDatabase,
    $NodesTable,
    TreeNode,
    $$NodesTableFilterComposer,
    $$NodesTableOrderingComposer,
    $$NodesTableAnnotationComposer,
    $$NodesTableCreateCompanionBuilder,
    $$NodesTableUpdateCompanionBuilder,
    (TreeNode, BaseReferences<_$AppDatabase, $NodesTable, TreeNode>),
    TreeNode,
    PrefetchHooks Function()>;
typedef $$NodeContentsTableCreateCompanionBuilder = NodeContentsCompanion
    Function({
  required String nodeId,
  Value<String> rawContent,
  Value<int> rawRevision,
  Value<String?> structuredContent,
  Value<int> structuredRevision,
  Value<String?> structureProposal,
  Value<int> rowid,
});
typedef $$NodeContentsTableUpdateCompanionBuilder = NodeContentsCompanion
    Function({
  Value<String> nodeId,
  Value<String> rawContent,
  Value<int> rawRevision,
  Value<String?> structuredContent,
  Value<int> structuredRevision,
  Value<String?> structureProposal,
  Value<int> rowid,
});

class $$NodeContentsTableFilterComposer
    extends Composer<_$AppDatabase, $NodeContentsTable> {
  $$NodeContentsTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get nodeId => $composableBuilder(
      column: $table.nodeId, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get rawContent => $composableBuilder(
      column: $table.rawContent, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get rawRevision => $composableBuilder(
      column: $table.rawRevision, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get structuredContent => $composableBuilder(
      column: $table.structuredContent,
      builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get structuredRevision => $composableBuilder(
      column: $table.structuredRevision,
      builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get structureProposal => $composableBuilder(
      column: $table.structureProposal,
      builder: (column) => ColumnFilters(column));
}

class $$NodeContentsTableOrderingComposer
    extends Composer<_$AppDatabase, $NodeContentsTable> {
  $$NodeContentsTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get nodeId => $composableBuilder(
      column: $table.nodeId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get rawContent => $composableBuilder(
      column: $table.rawContent, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get rawRevision => $composableBuilder(
      column: $table.rawRevision, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get structuredContent => $composableBuilder(
      column: $table.structuredContent,
      builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get structuredRevision => $composableBuilder(
      column: $table.structuredRevision,
      builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get structureProposal => $composableBuilder(
      column: $table.structureProposal,
      builder: (column) => ColumnOrderings(column));
}

class $$NodeContentsTableAnnotationComposer
    extends Composer<_$AppDatabase, $NodeContentsTable> {
  $$NodeContentsTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get nodeId =>
      $composableBuilder(column: $table.nodeId, builder: (column) => column);

  GeneratedColumn<String> get rawContent => $composableBuilder(
      column: $table.rawContent, builder: (column) => column);

  GeneratedColumn<int> get rawRevision => $composableBuilder(
      column: $table.rawRevision, builder: (column) => column);

  GeneratedColumn<String> get structuredContent => $composableBuilder(
      column: $table.structuredContent, builder: (column) => column);

  GeneratedColumn<int> get structuredRevision => $composableBuilder(
      column: $table.structuredRevision, builder: (column) => column);

  GeneratedColumn<String> get structureProposal => $composableBuilder(
      column: $table.structureProposal, builder: (column) => column);
}

class $$NodeContentsTableTableManager extends RootTableManager<
    _$AppDatabase,
    $NodeContentsTable,
    NodeContent,
    $$NodeContentsTableFilterComposer,
    $$NodeContentsTableOrderingComposer,
    $$NodeContentsTableAnnotationComposer,
    $$NodeContentsTableCreateCompanionBuilder,
    $$NodeContentsTableUpdateCompanionBuilder,
    (
      NodeContent,
      BaseReferences<_$AppDatabase, $NodeContentsTable, NodeContent>
    ),
    NodeContent,
    PrefetchHooks Function()> {
  $$NodeContentsTableTableManager(_$AppDatabase db, $NodeContentsTable table)
      : super(TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$NodeContentsTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$NodeContentsTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$NodeContentsTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback: ({
            Value<String> nodeId = const Value.absent(),
            Value<String> rawContent = const Value.absent(),
            Value<int> rawRevision = const Value.absent(),
            Value<String?> structuredContent = const Value.absent(),
            Value<int> structuredRevision = const Value.absent(),
            Value<String?> structureProposal = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              NodeContentsCompanion(
            nodeId: nodeId,
            rawContent: rawContent,
            rawRevision: rawRevision,
            structuredContent: structuredContent,
            structuredRevision: structuredRevision,
            structureProposal: structureProposal,
            rowid: rowid,
          ),
          createCompanionCallback: ({
            required String nodeId,
            Value<String> rawContent = const Value.absent(),
            Value<int> rawRevision = const Value.absent(),
            Value<String?> structuredContent = const Value.absent(),
            Value<int> structuredRevision = const Value.absent(),
            Value<String?> structureProposal = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              NodeContentsCompanion.insert(
            nodeId: nodeId,
            rawContent: rawContent,
            rawRevision: rawRevision,
            structuredContent: structuredContent,
            structuredRevision: structuredRevision,
            structureProposal: structureProposal,
            rowid: rowid,
          ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ));
}

typedef $$NodeContentsTableProcessedTableManager = ProcessedTableManager<
    _$AppDatabase,
    $NodeContentsTable,
    NodeContent,
    $$NodeContentsTableFilterComposer,
    $$NodeContentsTableOrderingComposer,
    $$NodeContentsTableAnnotationComposer,
    $$NodeContentsTableCreateCompanionBuilder,
    $$NodeContentsTableUpdateCompanionBuilder,
    (
      NodeContent,
      BaseReferences<_$AppDatabase, $NodeContentsTable, NodeContent>
    ),
    NodeContent,
    PrefetchHooks Function()>;
typedef $$ContentVersionsTableCreateCompanionBuilder = ContentVersionsCompanion
    Function({
  required String id,
  required String nodeId,
  required String projectId,
  required String field,
  required String content,
  required int atRevision,
  required String reason,
  required int serverRevision,
  required DateTime createdAt,
  Value<int> rowid,
});
typedef $$ContentVersionsTableUpdateCompanionBuilder = ContentVersionsCompanion
    Function({
  Value<String> id,
  Value<String> nodeId,
  Value<String> projectId,
  Value<String> field,
  Value<String> content,
  Value<int> atRevision,
  Value<String> reason,
  Value<int> serverRevision,
  Value<DateTime> createdAt,
  Value<int> rowid,
});

class $$ContentVersionsTableFilterComposer
    extends Composer<_$AppDatabase, $ContentVersionsTable> {
  $$ContentVersionsTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
      column: $table.id, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get nodeId => $composableBuilder(
      column: $table.nodeId, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get projectId => $composableBuilder(
      column: $table.projectId, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get field => $composableBuilder(
      column: $table.field, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get content => $composableBuilder(
      column: $table.content, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get atRevision => $composableBuilder(
      column: $table.atRevision, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get reason => $composableBuilder(
      column: $table.reason, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get serverRevision => $composableBuilder(
      column: $table.serverRevision,
      builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnFilters(column));
}

class $$ContentVersionsTableOrderingComposer
    extends Composer<_$AppDatabase, $ContentVersionsTable> {
  $$ContentVersionsTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
      column: $table.id, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get nodeId => $composableBuilder(
      column: $table.nodeId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get projectId => $composableBuilder(
      column: $table.projectId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get field => $composableBuilder(
      column: $table.field, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get content => $composableBuilder(
      column: $table.content, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get atRevision => $composableBuilder(
      column: $table.atRevision, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get reason => $composableBuilder(
      column: $table.reason, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get serverRevision => $composableBuilder(
      column: $table.serverRevision,
      builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnOrderings(column));
}

class $$ContentVersionsTableAnnotationComposer
    extends Composer<_$AppDatabase, $ContentVersionsTable> {
  $$ContentVersionsTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get nodeId =>
      $composableBuilder(column: $table.nodeId, builder: (column) => column);

  GeneratedColumn<String> get projectId =>
      $composableBuilder(column: $table.projectId, builder: (column) => column);

  GeneratedColumn<String> get field =>
      $composableBuilder(column: $table.field, builder: (column) => column);

  GeneratedColumn<String> get content =>
      $composableBuilder(column: $table.content, builder: (column) => column);

  GeneratedColumn<int> get atRevision => $composableBuilder(
      column: $table.atRevision, builder: (column) => column);

  GeneratedColumn<String> get reason =>
      $composableBuilder(column: $table.reason, builder: (column) => column);

  GeneratedColumn<int> get serverRevision => $composableBuilder(
      column: $table.serverRevision, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);
}

class $$ContentVersionsTableTableManager extends RootTableManager<
    _$AppDatabase,
    $ContentVersionsTable,
    ContentVersion,
    $$ContentVersionsTableFilterComposer,
    $$ContentVersionsTableOrderingComposer,
    $$ContentVersionsTableAnnotationComposer,
    $$ContentVersionsTableCreateCompanionBuilder,
    $$ContentVersionsTableUpdateCompanionBuilder,
    (
      ContentVersion,
      BaseReferences<_$AppDatabase, $ContentVersionsTable, ContentVersion>
    ),
    ContentVersion,
    PrefetchHooks Function()> {
  $$ContentVersionsTableTableManager(
      _$AppDatabase db, $ContentVersionsTable table)
      : super(TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$ContentVersionsTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$ContentVersionsTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$ContentVersionsTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback: ({
            Value<String> id = const Value.absent(),
            Value<String> nodeId = const Value.absent(),
            Value<String> projectId = const Value.absent(),
            Value<String> field = const Value.absent(),
            Value<String> content = const Value.absent(),
            Value<int> atRevision = const Value.absent(),
            Value<String> reason = const Value.absent(),
            Value<int> serverRevision = const Value.absent(),
            Value<DateTime> createdAt = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              ContentVersionsCompanion(
            id: id,
            nodeId: nodeId,
            projectId: projectId,
            field: field,
            content: content,
            atRevision: atRevision,
            reason: reason,
            serverRevision: serverRevision,
            createdAt: createdAt,
            rowid: rowid,
          ),
          createCompanionCallback: ({
            required String id,
            required String nodeId,
            required String projectId,
            required String field,
            required String content,
            required int atRevision,
            required String reason,
            required int serverRevision,
            required DateTime createdAt,
            Value<int> rowid = const Value.absent(),
          }) =>
              ContentVersionsCompanion.insert(
            id: id,
            nodeId: nodeId,
            projectId: projectId,
            field: field,
            content: content,
            atRevision: atRevision,
            reason: reason,
            serverRevision: serverRevision,
            createdAt: createdAt,
            rowid: rowid,
          ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ));
}

typedef $$ContentVersionsTableProcessedTableManager = ProcessedTableManager<
    _$AppDatabase,
    $ContentVersionsTable,
    ContentVersion,
    $$ContentVersionsTableFilterComposer,
    $$ContentVersionsTableOrderingComposer,
    $$ContentVersionsTableAnnotationComposer,
    $$ContentVersionsTableCreateCompanionBuilder,
    $$ContentVersionsTableUpdateCompanionBuilder,
    (
      ContentVersion,
      BaseReferences<_$AppDatabase, $ContentVersionsTable, ContentVersion>
    ),
    ContentVersion,
    PrefetchHooks Function()>;
typedef $$OutboxTableCreateCompanionBuilder = OutboxCompanion Function({
  Value<int> seq,
  required String operationId,
  required String type,
  required String entityId,
  Value<int> baseRevision,
  required String payload,
  Value<bool> inFlight,
  required DateTime createdAt,
});
typedef $$OutboxTableUpdateCompanionBuilder = OutboxCompanion Function({
  Value<int> seq,
  Value<String> operationId,
  Value<String> type,
  Value<String> entityId,
  Value<int> baseRevision,
  Value<String> payload,
  Value<bool> inFlight,
  Value<DateTime> createdAt,
});

class $$OutboxTableFilterComposer
    extends Composer<_$AppDatabase, $OutboxTable> {
  $$OutboxTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<int> get seq => $composableBuilder(
      column: $table.seq, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get operationId => $composableBuilder(
      column: $table.operationId, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get type => $composableBuilder(
      column: $table.type, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get entityId => $composableBuilder(
      column: $table.entityId, builder: (column) => ColumnFilters(column));

  ColumnFilters<int> get baseRevision => $composableBuilder(
      column: $table.baseRevision, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get payload => $composableBuilder(
      column: $table.payload, builder: (column) => ColumnFilters(column));

  ColumnFilters<bool> get inFlight => $composableBuilder(
      column: $table.inFlight, builder: (column) => ColumnFilters(column));

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnFilters(column));
}

class $$OutboxTableOrderingComposer
    extends Composer<_$AppDatabase, $OutboxTable> {
  $$OutboxTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<int> get seq => $composableBuilder(
      column: $table.seq, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get operationId => $composableBuilder(
      column: $table.operationId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get type => $composableBuilder(
      column: $table.type, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get entityId => $composableBuilder(
      column: $table.entityId, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<int> get baseRevision => $composableBuilder(
      column: $table.baseRevision,
      builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get payload => $composableBuilder(
      column: $table.payload, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<bool> get inFlight => $composableBuilder(
      column: $table.inFlight, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
      column: $table.createdAt, builder: (column) => ColumnOrderings(column));
}

class $$OutboxTableAnnotationComposer
    extends Composer<_$AppDatabase, $OutboxTable> {
  $$OutboxTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<int> get seq =>
      $composableBuilder(column: $table.seq, builder: (column) => column);

  GeneratedColumn<String> get operationId => $composableBuilder(
      column: $table.operationId, builder: (column) => column);

  GeneratedColumn<String> get type =>
      $composableBuilder(column: $table.type, builder: (column) => column);

  GeneratedColumn<String> get entityId =>
      $composableBuilder(column: $table.entityId, builder: (column) => column);

  GeneratedColumn<int> get baseRevision => $composableBuilder(
      column: $table.baseRevision, builder: (column) => column);

  GeneratedColumn<String> get payload =>
      $composableBuilder(column: $table.payload, builder: (column) => column);

  GeneratedColumn<bool> get inFlight =>
      $composableBuilder(column: $table.inFlight, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);
}

class $$OutboxTableTableManager extends RootTableManager<
    _$AppDatabase,
    $OutboxTable,
    OutboxEntry,
    $$OutboxTableFilterComposer,
    $$OutboxTableOrderingComposer,
    $$OutboxTableAnnotationComposer,
    $$OutboxTableCreateCompanionBuilder,
    $$OutboxTableUpdateCompanionBuilder,
    (OutboxEntry, BaseReferences<_$AppDatabase, $OutboxTable, OutboxEntry>),
    OutboxEntry,
    PrefetchHooks Function()> {
  $$OutboxTableTableManager(_$AppDatabase db, $OutboxTable table)
      : super(TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$OutboxTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$OutboxTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$OutboxTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback: ({
            Value<int> seq = const Value.absent(),
            Value<String> operationId = const Value.absent(),
            Value<String> type = const Value.absent(),
            Value<String> entityId = const Value.absent(),
            Value<int> baseRevision = const Value.absent(),
            Value<String> payload = const Value.absent(),
            Value<bool> inFlight = const Value.absent(),
            Value<DateTime> createdAt = const Value.absent(),
          }) =>
              OutboxCompanion(
            seq: seq,
            operationId: operationId,
            type: type,
            entityId: entityId,
            baseRevision: baseRevision,
            payload: payload,
            inFlight: inFlight,
            createdAt: createdAt,
          ),
          createCompanionCallback: ({
            Value<int> seq = const Value.absent(),
            required String operationId,
            required String type,
            required String entityId,
            Value<int> baseRevision = const Value.absent(),
            required String payload,
            Value<bool> inFlight = const Value.absent(),
            required DateTime createdAt,
          }) =>
              OutboxCompanion.insert(
            seq: seq,
            operationId: operationId,
            type: type,
            entityId: entityId,
            baseRevision: baseRevision,
            payload: payload,
            inFlight: inFlight,
            createdAt: createdAt,
          ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ));
}

typedef $$OutboxTableProcessedTableManager = ProcessedTableManager<
    _$AppDatabase,
    $OutboxTable,
    OutboxEntry,
    $$OutboxTableFilterComposer,
    $$OutboxTableOrderingComposer,
    $$OutboxTableAnnotationComposer,
    $$OutboxTableCreateCompanionBuilder,
    $$OutboxTableUpdateCompanionBuilder,
    (OutboxEntry, BaseReferences<_$AppDatabase, $OutboxTable, OutboxEntry>),
    OutboxEntry,
    PrefetchHooks Function()>;
typedef $$KeyValuesTableCreateCompanionBuilder = KeyValuesCompanion Function({
  required String key,
  required String value,
  Value<int> rowid,
});
typedef $$KeyValuesTableUpdateCompanionBuilder = KeyValuesCompanion Function({
  Value<String> key,
  Value<String> value,
  Value<int> rowid,
});

class $$KeyValuesTableFilterComposer
    extends Composer<_$AppDatabase, $KeyValuesTable> {
  $$KeyValuesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get key => $composableBuilder(
      column: $table.key, builder: (column) => ColumnFilters(column));

  ColumnFilters<String> get value => $composableBuilder(
      column: $table.value, builder: (column) => ColumnFilters(column));
}

class $$KeyValuesTableOrderingComposer
    extends Composer<_$AppDatabase, $KeyValuesTable> {
  $$KeyValuesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get key => $composableBuilder(
      column: $table.key, builder: (column) => ColumnOrderings(column));

  ColumnOrderings<String> get value => $composableBuilder(
      column: $table.value, builder: (column) => ColumnOrderings(column));
}

class $$KeyValuesTableAnnotationComposer
    extends Composer<_$AppDatabase, $KeyValuesTable> {
  $$KeyValuesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get key =>
      $composableBuilder(column: $table.key, builder: (column) => column);

  GeneratedColumn<String> get value =>
      $composableBuilder(column: $table.value, builder: (column) => column);
}

class $$KeyValuesTableTableManager extends RootTableManager<
    _$AppDatabase,
    $KeyValuesTable,
    KeyValue,
    $$KeyValuesTableFilterComposer,
    $$KeyValuesTableOrderingComposer,
    $$KeyValuesTableAnnotationComposer,
    $$KeyValuesTableCreateCompanionBuilder,
    $$KeyValuesTableUpdateCompanionBuilder,
    (KeyValue, BaseReferences<_$AppDatabase, $KeyValuesTable, KeyValue>),
    KeyValue,
    PrefetchHooks Function()> {
  $$KeyValuesTableTableManager(_$AppDatabase db, $KeyValuesTable table)
      : super(TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$KeyValuesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$KeyValuesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$KeyValuesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback: ({
            Value<String> key = const Value.absent(),
            Value<String> value = const Value.absent(),
            Value<int> rowid = const Value.absent(),
          }) =>
              KeyValuesCompanion(
            key: key,
            value: value,
            rowid: rowid,
          ),
          createCompanionCallback: ({
            required String key,
            required String value,
            Value<int> rowid = const Value.absent(),
          }) =>
              KeyValuesCompanion.insert(
            key: key,
            value: value,
            rowid: rowid,
          ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ));
}

typedef $$KeyValuesTableProcessedTableManager = ProcessedTableManager<
    _$AppDatabase,
    $KeyValuesTable,
    KeyValue,
    $$KeyValuesTableFilterComposer,
    $$KeyValuesTableOrderingComposer,
    $$KeyValuesTableAnnotationComposer,
    $$KeyValuesTableCreateCompanionBuilder,
    $$KeyValuesTableUpdateCompanionBuilder,
    (KeyValue, BaseReferences<_$AppDatabase, $KeyValuesTable, KeyValue>),
    KeyValue,
    PrefetchHooks Function()>;

class $AppDatabaseManager {
  final _$AppDatabase _db;
  $AppDatabaseManager(this._db);
  $$ProjectsTableTableManager get projects =>
      $$ProjectsTableTableManager(_db, _db.projects);
  $$NodesTableTableManager get nodes =>
      $$NodesTableTableManager(_db, _db.nodes);
  $$NodeContentsTableTableManager get nodeContents =>
      $$NodeContentsTableTableManager(_db, _db.nodeContents);
  $$ContentVersionsTableTableManager get contentVersions =>
      $$ContentVersionsTableTableManager(_db, _db.contentVersions);
  $$OutboxTableTableManager get outbox =>
      $$OutboxTableTableManager(_db, _db.outbox);
  $$KeyValuesTableTableManager get keyValues =>
      $$KeyValuesTableTableManager(_db, _db.keyValues);
}
