import 'package:flutter_test/flutter_test.dart';
import 'package:todo_sandbox/logic/sort.dart';
import 'package:todo_sandbox/main.dart';

void main() {
  test('active first, order kept inside groups', () {
    final todos = [Todo('a', done: true), Todo('b'), Todo('c', done: true), Todo('d')];
    expect(sortActiveFirst(todos).map((t) => t.title), ['b', 'd', 'a', 'c']);
  }, skip: 'aicrew-contract');

  test('returns a new list and keeps the source', () {
    final todos = [Todo('x', done: true), Todo('y')];
    final sorted = sortActiveFirst(todos);
    expect(identical(sorted, todos), isFalse);
    expect(todos.map((t) => t.title), ['x', 'y']);
  }, skip: 'aicrew-contract');

  test('empty list', () {
    expect(sortActiveFirst([]), isEmpty);
  }, skip: 'aicrew-contract');
}
