// Снимки окна проекта (этап 7.9, design/project-v1.html, DECISIONS.md 19.10.2026): обе темы, вкладки «О проекте» и «Код».
// Обновить: flutter test test/project_screenshots_test.dart --update-goldens
@Tags(['golden'])
library;

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

http.Response _json(Object body) =>
    http.Response.bytes(utf8.encode(jsonEncode(body)), 200, headers: {'content-type': 'application/json'});

ApiClient _client(TokenStore tokens) => ApiClient(
      baseUrl: Uri.parse('http://test'),
      tokens: tokens,
      client: MockClient((req) async => switch (req.url.path) {
            '/api/v1/projects/p1/context' => _json({
                'description': 'Игра в Roblox для детей 8–12 лет: игрок ищет пасхалки на карте парка, за находки получает монеты '
                    'и скины.\nСтиль — мультяшный, светлый, крупные кнопки (играют в основном с телефона).\n'
                    'Код синхронизируется через Rojo, сервер — источник истины для монет.\n'
                    'Нельзя: платные механики с таймерами, чат между игроками.',
                'files': [
                  {'id': 'f1', 'name': 'ТЗ_пасхалки_v2.md', 'size': 18432},
                  {'id': 'f2', 'name': 'макет_магазина.png', 'size': 3040870},
                  {'id': 'f3', 'name': 'гайд_по_стилю.pdf', 'size': 1258291},
                ],
              }),
            '/api/v1/exec/repositories' => _json({
                'repositories': [
                  {
                    'id': 'r1', 'project_id': 'p1', 'host_id': 'h1', 'name': 'easter-hunt',
                    'local_path': r'D:\aicrew-work\easter-hunt', 'git_url': 'https://github.com/Lymoos/easter-hunt.git',
                    'default_branch': 'main', 'status': 'ready', 'verification_profile': {'template': 'roblox'},
                  },
                ],
              }),
            '/api/v1/exec/workers' => _json({
                'workers': [
                  {'host_id': 'h1', 'host_name': 'DESKTOP-E2RTGAK', 'kind': 'claude_code', 'model': 'sonnet', 'status': 'idle', 'host_status': 'online'},
                ],
              }),
            _ => http.Response('', 204),
          }),
    );

Future<void> _pump(WidgetTester tester, bool dark) async {
  final tokens = MemoryTokenStore();
  await tokens.write(const AuthTokens(accessToken: 'a', refreshToken: 'r', userId: 'u', deviceId: 'd'));
  tester.view.physicalSize = const Size(2200, 1600);
  tester.view.devicePixelRatio = 2;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(ProviderScope(
    overrides: [
      apiProvider.overrideWithValue(_client(tokens)),
      projectsProvider.overrideWith((ref) => Stream.value(const <Project>[])),
    ],
    child: MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: buildTheme(dark ? Brightness.dark : Brightness.light),
      home: Scaffold(body: ProjectWindow(projectId: 'p1', onClose: () {})),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(loadFonts);

  for (final dark in [false, true]) {
    final theme = dark ? 'dark' : 'light';
    testWidgets('project window: about ($theme)', (tester) async {
      await _pump(tester, dark);
      await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/project_about_$theme.png'));
    });
  }
  testWidgets('project window: code (light)', (tester) async {
    await _pump(tester, false);
    await tester.tap(find.byKey(const Key('project-tab-code')));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/project_code_light.png'));
  });
}
