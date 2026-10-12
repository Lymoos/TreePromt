import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

// Снимки канбана AiCrew (design/DECISIONS.md): доска на ПК, список на телефоне, обе темы.
// Обновить: flutter test test/kanban_screenshots_test.dart --update-goldens

final _t = DateTime(2026, 10, 12, 5, 13);

ExecTask _task(String id, String title, String status,
        {int attempt = 1, String executor = 'claude', List<String> skills = const [], bool ui = false,
        List<String> approval = const [], String? review, String? failure}) =>
    ExecTask(
      id: id,
      title: title,
      status: status,
      attempt: attempt,
      maxAttempts: 3,
      mode: 'manual',
      updatedAt: _t,
      executor: executor,
      skills: skills,
      touchesUI: ui,
      approvalRequired: approval,
      reviewSummary: review,
      failureClass: failure,
      failureCode: failure == null ? null : 'verification_failed',
    );

final sample = AiCrewBoard(
  tasks: [
    _task('t1', 'Заголовок «Дела на сегодня»', 'MERGEABLE', attempt: 2, skills: ['dart-clean-code'],
        approval: ['architecture.yaml'], review: 'Заголовок изменён в коде и тестах, лишних правок нет.'),
    _task('t2', 'Поиск по задачам', 'IN_PROGRESS', skills: ['flutter-widget-tests']),
    _task('t3', 'Цветная метка приоритета', 'VERIFYING', attempt: 4, ui: true),
    _task('t4', 'Сортировка по дате', 'QUEUED', attempt: 0, executor: 'junior'),
    _task('t5', 'Тесты на приоритеты', 'DONE'),
    _task('t6', 'Выбор приоритета в диалоге', 'DONE'),
    _task('t7', 'Счётчик в заголовке', 'AWAITING_HUMAN', attempt: 3, failure: 'TEST_FAILURE'),
  ],
  plans: [
    ExecPlan(id: 'p1', request: 'Экспорт списка дел в файл', status: 'AWAITING_APPROVAL', updatedAt: _t, tasks: const [
      PlanTaskView(title: 'Сериализация списка в JSON', dependsOn: []),
      PlanTaskView(title: 'Кнопка «Экспорт» в меню', dependsOn: [0]),
      PlanTaskView(title: 'Тесты экспорта', dependsOn: [0, 1]),
    ]),
  ],
  approvals: const [
    Approval(id: 'a1', type: 'merge', status: 'pending', taskId: 't1'),
    Approval(id: 'a2', type: 'plan', status: 'pending', planId: 'p1'),
  ],
  workers: const [
    WorkerView(hostName: 'home-pc', kind: 'claude_code', model: 'sonnet', busy: true, online: true),
  ],
);

Widget _app(bool dark, bool wide) => ProviderScope(
      child: MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: buildTheme(dark ? Brightness.dark : Brightness.light),
        home: Scaffold(appBar: AppBar(title: const Text('AiCrew')), body: KanbanView(board: sample, wide: wide)),
      ),
    );

void main() {
  setUpAll(loadFonts);

  for (final dark in [false, true]) {
    final theme = dark ? 'dark' : 'light';
    testWidgets('kanban board on desktop ($theme)', (tester) async {
      tester.view.physicalSize = const Size(2880, 1700);
      tester.view.devicePixelRatio = 2;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_app(dark, true));
      await tester.pumpAndSettle();
      await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/kanban_desktop_$theme.png'));
    });
    testWidgets('kanban list on phone ($theme)', (tester) async {
      tester.view.physicalSize = const Size(800, 2400);
      tester.view.devicePixelRatio = 2;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(_app(dark, false));
      await tester.pumpAndSettle();
      await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/kanban_phone_$theme.png'));
    });
  }
}
