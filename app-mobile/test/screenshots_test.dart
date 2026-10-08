// Скриншоты экранов для визуальной проверки и согласования.
// Обновить: flutter test --update-goldens test/screenshots_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'package:prompttree_shared/app.dart';

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
    await tester.tap(find.text(name).last);
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

  Future<void> phone(WidgetTester tester, {bool dark = false}) async {
    tester.view.physicalSize = const Size(1170, 2532);
    tester.view.devicePixelRatio = 3;
    tester.platformDispatcher.platformBrightnessTestValue = dark ? Brightness.dark : Brightness.light;
    addTearDown(tester.view.reset);
    addTearDown(tester.platformDispatcher.clearPlatformBrightnessTestValue);
  }

  for (final dark in [false, true]) {
    final theme = dark ? 'dark' : 'light';

    testWidgets('login screen ($theme)', (tester) async {
      await phone(tester, dark: dark);
      final store = memoryStore();
      await tester.pumpWidget(app(store, session: const Session(mode: SessionMode.none)));
      await tester.pumpAndSettle();
      await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/login_$theme.png'));
      await tearDownApp(tester, store);
    });
  }

  testWidgets('empty tree', (tester) async {
    await phone(tester);
    final store = memoryStore();
    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/empty_light.png'));
    await tearDownApp(tester, store);
  });

  testWidgets('create menu and row menu', (tester) async {
    await phone(tester);
    final store = memoryStore();
    await seed(store);
    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await openTree(tester);
    // Выбрать папку UI, оставив её раскрытой.
    for (var i = 0; i < 2; i++) {
      await tester.tap(find.text('UI'));
      await tester.pumpAndSettle();
    }
    await tester.tap(find.byKey(const Key('create')));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/create_sheet_light.png'));
    await tester.tapAt(const Offset(20, 40));
    await tester.pumpAndSettle();
    await tester.longPress(find.text('Журнал операций'));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/row_sheet_light.png'));
    await tearDownApp(tester, store);
  });

  testWidgets('trash', (tester) async {
    await phone(tester, dark: true);
    final store = memoryStore();
    final t = TreeService(store);
    final p = await t.createProject('PromptTree');
    for (final name in ['Старая идея', 'Черновик про иконку']) {
      await t.delete(await t.createNode(projectId: p, kind: NodeKind.rawNote, name: name));
    }
    await t.delete(await t.createNode(projectId: p, kind: NodeKind.folder, name: 'Архив'));
    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('menu')));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/menu_dark.png'));
    await tester.tap(find.text('Корзина'));
    await tester.pumpAndSettle();
    await expectLater(find.byType(MaterialApp), matchesGoldenFile('goldens/trash_dark.png'));
    await tearDownApp(tester, store);
  });
}
