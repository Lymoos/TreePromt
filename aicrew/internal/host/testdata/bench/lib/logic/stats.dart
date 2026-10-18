import '../main.dart';

class TodoStats {
  const TodoStats({required this.total, required this.done});

  final int total;
  final int done;

  /// Сколько осталось сделать.
  int get active => throw UnimplementedError();

  /// Доля выполненных от 0 до 1; для пустого списка — 0.
  double get progress => throw UnimplementedError();
}

TodoStats countTodos(List<Todo> todos) {
  throw UnimplementedError();
}
