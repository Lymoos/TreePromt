import '../main.dart';

/// {"title": ..., "done": ...}
Map<String, dynamic> todoToJson(Todo todo) {
  throw UnimplementedError();
}

/// Обратное преобразование. Нет "done" — задача не выполнена. Нет "title" или он не строка — FormatException.
Todo todoFromJson(Map<String, dynamic> json) {
  throw UnimplementedError();
}
