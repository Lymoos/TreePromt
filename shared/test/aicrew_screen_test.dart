import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

// Канбан AiCrew (7.6): доска на ПК, список на телефоне, решения уходят на сервер.

final _now = DateTime.now();

ExecTask _task(String id, String title, String status, {String mode = 'manual', List<String> skills = const []}) =>
    ExecTask(id: id, title: title, status: status, attempt: 1, maxAttempts: 3, mode: mode, updatedAt: _now, skills: skills);

AiCrewBoard _board({List<Incident> incidents = const []}) => AiCrewBoard(
      tasks: [
        _task('t1', 'Заголовок «Дела на сегодня»', 'MERGEABLE', skills: ['dart-clean-code']),
        _task('t2', 'Поиск по задачам', 'IN_PROGRESS'),
        _task('t3', 'Цветная метка', 'VERIFYING'),
        _task('t4', 'Сортировка по дате', 'QUEUED'),
        _task('t5', 'Тесты на приоритеты', 'DONE'),
        _task('t6', 'Счётчик в заголовке', 'AWAITING_HUMAN'),
        _task('t7', 'Отменённая', 'CANCELLED'),
      ],
      plans: [
        ExecPlan(id: 'p1', request: 'Экспорт списка в файл', status: 'AWAITING_APPROVAL', updatedAt: _now, tasks: const [
          PlanTaskView(title: 'Сериализация в JSON', dependsOn: []),
          PlanTaskView(title: 'Кнопка «Экспорт»', dependsOn: [0]),
          PlanTaskView(title: 'Тесты экспорта', dependsOn: [0, 1]),
        ]),
      ],
      approvals: const [
        Approval(id: 'a1', type: 'merge', status: 'pending', taskId: 't1'),
        Approval(id: 'a2', type: 'plan', status: 'pending', planId: 'p1'),
      ],
      incidents: incidents,
      workers: const [WorkerView(hostName: 'home-pc', kind: 'claude_code', model: 'sonnet', busy: true, online: true)],
    );

class _Recorder {
  final calls = <(String, String, Map<String, dynamic>)>[];

  ApiClient client(TokenStore tokens) => ApiClient(
        baseUrl: Uri.parse('http://test'),
        tokens: tokens,
        client: MockClient((req) async {
          final body = req.body.isEmpty ? <String, dynamic>{} : jsonDecode(req.body) as Map<String, dynamic>;
          calls.add((req.method, req.url.path, body));
          if (req.method == 'GET') {
            final key = req.url.pathSegments.last;
            return http.Response(jsonEncode({key: []}), 200);
          }
          return http.Response('', 204);
        }),
      );
}

Future<_Recorder> _pump(WidgetTester tester, {required double width, AiCrewBoard? board, double height = 1400}) async {
  final rec = _Recorder();
  final tokens = MemoryTokenStore();
  await tokens.write(const AuthTokens(accessToken: 'a', refreshToken: 'r', userId: 'u', deviceId: 'd'));
  tester.view.physicalSize = Size(width, height);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(ProviderScope(
    overrides: [apiProvider.overrideWithValue(rec.client(tokens))],
    child: MaterialApp(
      theme: buildTheme(Brightness.light),
      home: Scaffold(body: KanbanView(board: board ?? _board(), wide: width >= kanbanBoardMinWidth)),
    ),
  ));
  return rec;
}

void main() {
  sendDialogTests();
  testWidgets('desktop: six columns, cancelled tasks are hidden', (tester) async {
    await _pump(tester, width: 1600);
    for (final title in ['ОЧЕРЕДЬ', 'В РАБОТЕ', 'ПРОВЕРКА', 'ЖДЁТ РЕШЕНИЯ', 'СЛИТО', 'ОШИБКИ']) {
      expect(find.text(title), findsOneWidget, reason: title);
    }
    expect(find.text('Отменённая'), findsNothing);
    expect(find.text('home-pc · sonnet'), findsOneWidget);
  });

  testWidgets('phone: list starts with what waits for the owner', (tester) async {
    await _pump(tester, width: 400, height: 4000);
    expect(find.text('ОЧЕРЕДЬ'), findsNothing); // доски нет
    final wait = tester.getTopLeft(find.text('ЖДЁТ РЕШЕНИЯ · 2')).dy;
    final fail = tester.getTopLeft(find.text('ОШИБКИ · 1')).dy;
    final done = tester.getTopLeft(find.text('СЛИТО · 1')).dy;
    expect(wait < fail && fail < done, isTrue);
  });

  testWidgets('merge approval and rejection with a comment', (tester) async {
    final rec = await _pump(tester, width: 1600);
    await tester.tap(find.text('Слить'));
    await tester.pumpAndSettle();
    expect(rec.calls.any((c) => c.$2 == '/api/v1/exec/approvals/a1' && c.$3['decision'] == 'approve'), isTrue);

    await tester.tap(find.text('Вернуть с комментарием…'));
    await tester.pump();
    await tester.tap(find.text('Отправить на доработку'));
    await tester.pump();
    expect(rec.calls.where((c) => c.$3['decision'] == 'reject'), isEmpty, reason: 'empty comment is not sent');
    await tester.enterText(find.byType(TextField), 'выбор приоритета — рядом с полем ввода');
    await tester.tap(find.text('Отправить на доработку'));
    await tester.pumpAndSettle();
    final reject = rec.calls.firstWhere((c) => c.$3['decision'] == 'reject');
    expect(reject.$3['comment'], 'выбор приоритета — рядом с полем ввода');
  });

  testWidgets('plan: partial approval sends chosen tasks', (tester) async {
    final rec = await _pump(tester, width: 1600);
    expect(find.text('Утвердить план'), findsOneWidget);
    await tester.tap(find.textContaining('Тесты экспорта', findRichText: true));
    await tester.pump();
    await tester.tap(find.text('Утвердить выбранные'));
    await tester.pumpAndSettle();
    final call = rec.calls.firstWhere((c) => c.$2 == '/api/v1/exec/approvals/a2');
    expect(call.$3['decision'], 'partial');
    expect(call.$3['tasks'], [0, 1]);
  });

  testWidgets('incident: inverted pause bar, no «stop all», resume', (tester) async {
    final rec = await _pump(tester, width: 1600,
        board: _board(incidents: const [Incident(id: 'i1', kind: 'secret_leak', reason: 'lib/config.dart:1 (google_api_key)')]));
    expect(find.textContaining('AiCrew на паузе'), findsOneWidget);
    expect(find.text('Остановить всё'), findsNothing);
    await tester.tap(find.text('Возобновить'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, 'ключ отозван');
    await tester.tap(find.text('Готово'));
    await tester.pumpAndSettle();
    expect(rec.calls.any((c) => c.$2 == '/api/v1/exec/incidents/i1/resolve' && c.$3['comment'] == 'ключ отозван'), isTrue);
  });

  test('status → column', () {
    expect(taskColumn('MERGE_QUEUED'), KanbanCol.check);
    expect(taskColumn('AWAITING_HUMAN'), KanbanCol.fail);
    expect(taskColumn('CANCELLED'), isNull);
    expect(planColumn('AWAITING_APPROVAL'), KanbanCol.wait);
  });
}

class _SendRecorder {
  final calls = <(String, String, Map<String, dynamic>)>[];
  ApiClient client(TokenStore tokens) => ApiClient(
        baseUrl: Uri.parse('http://test'),
        tokens: tokens,
        client: MockClient((req) async {
          final body = req.body.isEmpty ? <String, dynamic>{} : jsonDecode(req.body) as Map<String, dynamic>;
          calls.add((req.method, req.url.path, body));
          if (req.url.path.endsWith('/exec/repositories')) {
            return http.Response.bytes(
                utf8.encode(jsonEncode({
                  'repositories': [
                    {'id': 'r1', 'project_id': 'proj', 'name': 'todo-sandbox', 'local_path': 'C:/x'},
                    {'id': 'r2', 'project_id': 'other', 'name': 'чужой', 'local_path': 'C:/y'},
                  ]
                })),
                200);
          }
          return http.Response(jsonEncode({'id': 'new'}), 201);
        }),
      );
}

Future<_SendRecorder> _pumpSend(WidgetTester tester) async {
  final rec = _SendRecorder();
  final tokens = MemoryTokenStore();
  await tokens.write(const AuthTokens(accessToken: 'a', refreshToken: 'r', userId: 'u', deviceId: 'd'));
  tester.view.physicalSize = const Size(1200, 1600);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(ProviderScope(
    overrides: [apiProvider.overrideWithValue(rec.client(tokens))],
    child: MaterialApp(
      theme: buildTheme(Brightness.light),
      home: Scaffold(body: Consumer(builder: (context, ref, _) => Center(
        child: FilledButton(
          onPressed: () => sendToAiCrew(context, ref, nodeId: 'n1', projectId: 'proj', name: 'Поиск по задачам'),
          child: const Text('open'),
        ),
      ))),
    ),
  ));
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
  return rec;
}


void sendDialogTests() {
  testWidgets('send dialog: one task with chosen settings', (tester) async {
    final rec = await _pumpSend(tester);
    expect(find.text('todo-sandbox'), findsOneWidget);
    expect(find.text('чужой'), findsNothing, reason: 'only repositories of this project');
    expect(find.textContaining('Junior', findRichText: true), findsNothing,
        reason: 'Junior is never chosen directly: only under a contract inside a Manager plan');
    await tester.tap(find.textContaining('Меняет интерфейс', findRichText: true));
    await tester.tap(find.text('Отправить'));
    await tester.pumpAndSettle();
    final post = rec.calls.firstWhere((c) => c.$1 == 'POST');
    expect(post.$2, '/api/v1/exec/tasks');
    expect(post.$3, containsPair('node_id', 'n1'));
    expect(post.$3, containsPair('repository_id', 'r1'));
    expect(post.$3.containsKey('executor'), isFalse, reason: 'the server always uses Claude for a single task');
    expect(post.$3, containsPair('touches_ui', true));
    expect(find.text('Задача отправлена в AiCrew'), findsOneWidget);
  });

  testWidgets('send dialog: via Manager creates a plan', (tester) async {
    final rec = await _pumpSend(tester);
    await tester.tap(find.textContaining('Через Manager', findRichText: true));
    await tester.pump();
    expect(find.textContaining('Qwen', findRichText: true), findsOneWidget, reason: 'the Manager option explains who does what');
    await tester.tap(find.text('Отправить'));
    await tester.pumpAndSettle();
    expect(rec.calls.firstWhere((c) => c.$1 == 'POST').$2, '/api/v1/exec/plans');
  });
}
