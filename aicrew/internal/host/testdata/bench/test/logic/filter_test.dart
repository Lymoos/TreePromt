import 'package:flutter_test/flutter_test.dart';
import 'package:todo_sandbox/logic/filter.dart';
import 'package:todo_sandbox/main.dart';

void main() {
  final todos = [Todo('Купить молоко'), Todo('Позвонить маме'), Todo('купить ХЛЕБ', done: true)];

  test('finds by substring ignoring case', () {
    expect(filterByText(todos, 'КУПИТЬ').map((t) => t.title), ['Купить молоко', 'купить ХЛЕБ']);
  }, skip: 'aicrew-contract');

  test('trims the query; empty query returns everything', () {
    expect(filterByText(todos, '  маме ').single.title, 'Позвонить маме');
    expect(filterByText(todos, '   ').length, 3);
  }, skip: 'aicrew-contract');

  test('does not change the source list', () {
    final copy = List.of(todos);
    filterByText(todos, 'молоко').clear();
    expect(todos, copy);
  }, skip: 'aicrew-contract');
}
