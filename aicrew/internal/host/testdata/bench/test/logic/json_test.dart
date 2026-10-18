import 'package:flutter_test/flutter_test.dart';
import 'package:todo_sandbox/logic/json.dart';
import 'package:todo_sandbox/main.dart';

void main() {
  test('to json', () {
    expect(todoToJson(Todo('Чай', done: true)), {'title': 'Чай', 'done': true});
  }, skip: 'aicrew-contract');

  test('round trip', () {
    final t = todoFromJson(todoToJson(Todo('Кофе')));
    expect((t.title, t.done), ('Кофе', false));
  }, skip: 'aicrew-contract');

  test('missing done means not done; bad title is an error', () {
    expect(todoFromJson({'title': 'x'}).done, isFalse);
    expect(() => todoFromJson({'done': true}), throwsFormatException);
    expect(() => todoFromJson({'title': 5}), throwsFormatException);
  }, skip: 'aicrew-contract');
}
