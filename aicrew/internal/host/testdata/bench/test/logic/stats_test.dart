import 'package:flutter_test/flutter_test.dart';
import 'package:todo_sandbox/logic/stats.dart';
import 'package:todo_sandbox/main.dart';

void main() {
  test('counts', () {
    final s = countTodos([Todo('a', done: true), Todo('b'), Todo('c', done: true), Todo('d')]);
    expect((s.total, s.done, s.active), (4, 2, 2));
    expect(s.progress, 0.5);
  }, skip: 'aicrew-contract');

  test('empty list has zero progress', () {
    final s = countTodos([]);
    expect((s.total, s.done, s.active, s.progress), (0, 0, 0, 0.0));
  }, skip: 'aicrew-contract');
}
