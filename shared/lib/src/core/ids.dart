import 'package:uuid/uuid.dart';

const _uuid = Uuid();

/// UUIDv7: создаётся на клиенте, поэтому узел можно создать офлайн,
/// и его id не меняется после синхронизации (архитектура п. 3).
String newId() => _uuid.v7();
