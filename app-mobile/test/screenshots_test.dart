// Скриншоты экранов для визуальной проверки и согласования.
// Обновить: flutter test --update-goldens test/screenshots_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'package:prompttree_mobile/src/state/providers.dart';

import 'support.dart';

Future<String> seed(DriftLocalStore store) async {
  final t = TreeService(store);
  final p = await t.createProject('PromptTree');
  final sync = await t.createNode(projectId: p, kind: NodeKind.folder, name: 'Синхронизация');
  await t.createNode(projectId: p, parentId: sync, kind: NodeKind.aiTask, name: 'Журнал операций');
  await t.createNode(projectId: p, parentId: sync, kind: NodeKind.rawNote, name: 'Мысли про конфликты');
  final uiF = await t.createNode(projectId: p, kind: NodeKind.folder, name: 'UI');
  final tree = await t.createNode(
    projectId: p,
    parentId: uiF,
    kind: NodeKind.aiTask,
    name: 'Дерево-проводник',
    text: 'хочу левую панель как дерево, но не прям папки как в проводнике. чб, минимализм как notion.\n\n'
        'проекты → папки → файлы. сразу видно, где черновик, а где задача для claude. на телефоне тоже должно работать.',
  );
  await t.createNode(projectId: p, parentId: uiF, kind: NodeKind.rawNote, name: 'Референсы Notion');
  await t.createNode(projectId: p, kind: NodeKind.aiTask, name: 'Иконка приложения');
  final crew = await t.createProject('AiCrew');
  final orch = await t.createNode(projectId: crew, kind: NodeKind.folder, name: 'Оркестратор');
  await t.createNode(projectId: crew, parentId: orch, kind: NodeKind.aiTask, name: 'Lease и heartbeat');
  await t.createNode(projectId: crew, kind: NodeKind.rawNote, name: 'Список MCP-серверов');
  await store.patchNode(
      (await store.db.select(store.db.nodes).get()).firstWhere((n) => n.name == 'Журнал операций').id,
      const NodesCompanion(structureStatus: Value(StructureStatus.done)));
  await store.patchNode(tree, const NodesCompanion(structureStatus: Value(StructureStatus.stale)));
  return tree;
}

Future<void> openTree(WidgetTester tester) async {
  for (final name in ['PromptTree', 'UI', 'Синхронизация', 'AiCrew']) {
    await tester.tap(find.text(name).first);
    await tester.pumpAndSettle();
  }
}

void main() {
  setUpAll(loadFonts);

  for (final dark in [false, true]) {
    final theme = dark ? 'dark' : 'light';

    testWidgets('tree screen ($theme)', (tester) async {
      tester.view.physicalSize = const Size(1170, 2532);
      tester.view.devicePixelRatio = 3;
      tester.platformDispatcher.platformBrightnessTestValue = dark ? Brightness.dark : Brightness.light;
      addTearDown(tester.view.reset);
      addTearDown(tester.platformDispatcher.clearPlatformBrightnessTestValue);

      final store = memoryStore();
      await seed(store);
      await tester.pumpWidget(app(store));
      await tester.pumpAndSettle();
      await openTree(tester);
      await tester.tap(find.text('Дерево-проводник'));
      await tester.pumpAndSettle();
      await tester.pageBack();
      await tester.pumpAndSettle();

      await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/tree_$theme.png'));

      await tester.tap(find.text('Дерево-проводник'));
      await tester.pumpAndSettle();
      await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/editor_$theme.png'));
      await tearDownApp(tester, store);
    });
  }

  testWidgets('login screen', (tester) async {
    tester.view.physicalSize = const Size(1170, 2532);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.reset);
    final store = memoryStore();
    await tester.pumpWidget(app(store, session: const Session(mode: SessionMode.none)));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/login_light.png'));
    await tearDownApp(tester, store);
  });
}
