import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

// Окно проекта и скиллы задачи (этап 7.9, design/project-v1.html).

class _Server {
  final calls = <(String, String, Object?)>[];
  List<String> nodeSkills = ['roblox-ui-ux'];

  http.Response _json(Object body, [int code = 200]) => http.Response.bytes(utf8.encode(jsonEncode(body)), code,
      headers: {'content-type': 'application/json'});

  ApiClient client(TokenStore tokens) => ApiClient(
        baseUrl: Uri.parse('http://test'),
        tokens: tokens,
        client: MockClient((req) async {
          final body = req.body.isEmpty ? null : jsonDecode(req.body);
          final path = req.url.path.replaceFirst('/api/v1', '');
          calls.add((req.method, path, body));
          switch ((req.method, path)) {
            case ('GET', '/projects/p1/context'):
              return _json({
                'project_id': 'p1',
                'description': 'Игра в Roblox для детей.',
                'files': [
                  {'id': 'f1', 'name': 'ТЗ_пасхалки.md', 'size': 18432, 'content_type': 'text/markdown'},
                ],
              });
            case ('GET', '/exec/repositories'):
              return _json({
                'repositories': [
                  {
                    'id': 'r1', 'project_id': 'p1', 'host_id': 'h1', 'name': 'easter-hunt', 'local_path': r'D:\aicrew-work\easter-hunt',
                    'git_url': 'https://github.com/Lymoos/easter-hunt.git', 'default_branch': 'main', 'status': 'ready',
                    'verification_profile': {'template': 'roblox'},
                    'skill_policy': {'required': ['roblox-luau'], 'forbidden': ['frontend-design']},
                  },
                ],
              });
            case ('GET', '/exec/workers'):
              return _json({
                'workers': [
                  {'host_id': 'h1', 'host_name': 'DESKTOP-E2RTGAK', 'kind': 'claude_code', 'model': 'sonnet', 'status': 'idle', 'host_status': 'online'},
                  {'host_id': 'h1', 'host_name': 'DESKTOP-E2RTGAK', 'kind': 'codex', 'model': '', 'status': 'idle', 'host_status': 'online'},
                ],
              });
            case ('GET', '/exec/skills'):
              return _json({
                'skills': [
                  {'name': 'db-migrations', 'description': 'миграции PostgreSQL и Drift'},
                  {'name': 'frontend-design', 'description': 'дизайн новых интерфейсов'},
                  {'name': 'roblox-luau', 'description': 'игры Roblox на Luau'},
                  {'name': 'roblox-ui-ux', 'description': 'интерфейсы Roblox'},
                ],
              });
            case ('GET', '/nodes/n1/skills'):
              return _json({'skills': nodeSkills});
            case ('PUT', '/nodes/n1/skills'):
              nodeSkills = ((body as Map)['skills'] as List).cast<String>();
              return _json({'skills': nodeSkills});
            case ('POST', '/exec/repositories/bind'):
              return _json({'id': 'r1', 'project_id': 'p1', 'host_id': 'h1', 'name': 'easter-hunt', 'status': 'pending'});
          }
          return http.Response('', 204);
        }),
      );
}

Future<_Server> _pump(WidgetTester tester, Widget child) async {
  final server = _Server();
  final tokens = MemoryTokenStore();
  await tokens.write(const AuthTokens(accessToken: 'a', refreshToken: 'r', userId: 'u', deviceId: 'd'));
  tester.view.physicalSize = const Size(1280, 1100);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(ProviderScope(
    overrides: [
      apiProvider.overrideWithValue(server.client(tokens)),
      projectsProvider.overrideWith((ref) => Stream.value(const <Project>[])),
    ],
    child: MaterialApp(theme: buildTheme(Brightness.light), home: Scaffold(body: child)),
  ));
  await tester.pumpAndSettle();
  return server;
}

void main() {
  testWidgets('project window: description, files, saving', (tester) async {
    final server = await _pump(tester, const ProjectWindow(projectId: 'p1'));
    expect(find.text('Игра в Roblox для детей.'), findsOneWidget);
    expect(find.text('ТЗ_пасхалки.md'), findsOneWidget);
    expect(find.text('18 КБ'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('project-description')), 'Игра для детей 8–12 лет, крупные кнопки.');
    await tester.pump();
    await tester.tap(find.byKey(const Key('save-description')));
    await tester.pumpAndSettle();
    final put = server.calls.firstWhere((c) => c.$1 == 'PUT');
    expect(put.$2, '/projects/p1/context');
    expect(put.$3, {'description': 'Игра для детей 8–12 лет, крупные кнопки.'});
    expect(find.text('Описание сохранено'), findsOneWidget);
  });

  testWidgets('project window: code — folder and git together, bind status', (tester) async {
    final server = await _pump(tester, const ProjectWindow(projectId: 'p1'));
    await tester.tap(find.byKey(const Key('project-tab-code')));
    await tester.pumpAndSettle();
    expect(find.text(r'D:\aicrew-work\easter-hunt'), findsOneWidget);
    expect(find.text('https://github.com/Lymoos/easter-hunt.git'), findsOneWidget);
    expect(find.textContaining('Привязан', findRichText: true), findsOneWidget);
    expect(find.textContaining('DESKTOP-E2RTGAK', findRichText: true), findsOneWidget);

    await tester.enterText(find.byKey(const Key('code-path')), r'D:\games\easter');
    await tester.tap(find.byKey(const Key('bind-code')));
    await tester.pumpAndSettle();
    final bind = server.calls.firstWhere((c) => c.$1 == 'POST');
    expect(bind.$2, '/exec/repositories/bind');
    expect(bind.$3, {
      'project_id': 'p1',
      'host_id': 'h1',
      'local_path': r'D:\games\easter',
      'git_url': 'https://github.com/Lymoos/easter-hunt.git',
      'default_branch': 'main',
      'verification_profile': {'template': 'roblox'},
    });
  });

  testWidgets('project window: project skills — required in a frame, forbidden struck through', (tester) async {
    await _pump(tester, const ProjectWindow(projectId: 'p1'));
    await tester.tap(find.byKey(const Key('project-tab-skills')));
    await tester.pumpAndSettle();
    expect(find.text('roblox-luau'), findsOneWidget);
    final forbidden = tester.widget<Text>(find.text('frontend-design'));
    expect(forbidden.style?.decoration, TextDecoration.lineThrough);
  });

  testWidgets('task skills: project ones are locked, picking saves to the task', (tester) async {
    final server = await _pump(tester, const TaskSkillBar(nodeId: 'n1', projectId: 'p1'));
    expect(find.text('roblox-luau'), findsOneWidget, reason: 'required by the project');
    expect(find.text('roblox-ui-ux'), findsOneWidget, reason: 'chosen in the task');

    await tester.tap(find.byKey(const Key('add-skill')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('skill-frontend-design')), findsNothing, reason: 'forbidden skills are not offered');
    await tester.tap(find.byKey(const Key('skill-db-migrations')));
    await tester.tap(find.byKey(const Key('skills-done')));
    await tester.pumpAndSettle();
    final put = server.calls.lastWhere((c) => c.$1 == 'PUT');
    expect(put.$2, '/nodes/n1/skills');
    expect(put.$3, {'skills': ['db-migrations', 'roblox-ui-ux']});
    expect(find.text('db-migrations'), findsOneWidget);
  });
}
