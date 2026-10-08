import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_mobile/src/state/providers.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

void main() {
  testWidgets('login screen offers to work without a server', (tester) async {
    final store = memoryStore();
    await tester.pumpWidget(app(store, session: const Session(mode: SessionMode.none)));
    await tester.pumpAndSettle();
    expect(find.text('Войти'), findsOneWidget);
    await tester.tap(find.text('Работать без сервера'));
    await tester.pumpAndSettle();
    expect(find.text('Пока пусто'), findsOneWidget);
    expect(await store.getValue(sessionModeKey), 'local');
    await tearDownApp(tester, store);
  });

  testWidgets('login without server address shows a hint, not a crash', (tester) async {
    final store = memoryStore();
    await tester.pumpWidget(app(store, session: const Session(mode: SessionMode.none)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Войти'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Укажите адрес сервера'), findsOneWidget);
    await tearDownApp(tester, store);
  });

  testWidgets('create project and AI task, text autosaves, tree shows the branch', (tester) async {
    final store = memoryStore();
    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('create-first')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('name-field')), 'PromptTree');
    await tester.tap(find.text('Создать'));
    await tester.pumpAndSettle();
    expect(find.text('PromptTree'), findsWidgets);

    await tester.tap(find.byKey(const Key('create')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('AI Task'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('name-field')), 'Дерево-проводник');
    await tester.tap(find.text('Создать'));
    await tester.pumpAndSettle();

    // Открылся редактор.
    expect(find.byKey(const Key('editor-text')), findsOneWidget);
    await tester.enterText(find.byKey(const Key('editor-text')), 'левая панель как дерево 🌳');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    final nodes = await store.db.select(store.db.nodes).get();
    final task = nodes.single;
    expect(task.kind, NodeKind.aiTask);
    expect((await store.content(task.id))!.rawContent, 'левая панель как дерево 🌳');

    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.text('Дерево-проводник'), findsOneWidget);
    await tearDownApp(tester, store);
  });

  testWidgets('copy task puts text in clipboard and offers to delete it', (tester) async {
    final store = memoryStore();
    final tree = TreeService(store);
    final p = await tree.createProject('P');
    await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'Задача', text: 'сделать иконку');

    String? copied;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') copied = (call.arguments as Map)['text'] as String;
      return null;
    });

    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Задача'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('copy-task')));
    await tester.pumpAndSettle();

    expect(copied, 'сделать иконку');
    expect(find.text('Удалить задачу?'), findsOneWidget);
    await tester.tap(find.text('Удалить'));
    await tester.pumpAndSettle();

    final n = (await store.db.select(store.db.nodes).get()).single;
    expect(n.deletedAt, isNotNull, reason: 'soft delete');
    expect(find.text('Задача'), findsNothing);
    await tearDownApp(tester, store);
  });

  testWidgets('conflict: both versions are shown and one can be kept', (tester) async {
    final store = memoryStore();
    final tree = TreeService(store);
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'Спорная', text: 'с телефона');
    await store.patchNode(n, const NodesCompanion(hasConflict: Value(true)));
    for (final (id, reason, text) in [
      ('v1', VersionReason.conflictServer, 'с телефона'),
      ('v2', VersionReason.conflictLocal, 'с ПК'),
    ]) {
      await store.putVersion(ContentVersionsCompanion.insert(
        id: id,
        nodeId: n,
        projectId: p,
        field: 'raw',
        content: text,
        atRevision: 1,
        reason: reason,
        serverRevision: 5,
        createdAt: DateTime.utc(2026, 10, 8),
      ));
    }

    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P'));
    await tester.pumpAndSettle();
    expect(find.text('конфликт'), findsOneWidget);
    await tester.tap(find.text('Спорная'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('open-conflict')));
    await tester.pumpAndSettle();

    expect(find.text('с ПК'), findsOneWidget);
    await tester.tap(find.descendant(of: find.byKey(const Key('version-second')), matching: find.text('Оставить эту')));
    await tester.pumpAndSettle();

    expect((await store.content(n))!.rawContent, 'с ПК');
    expect((await store.node(n))!.hasConflict, isFalse);
    await tearDownApp(tester, store);
  });

  testWidgets('deleted note can be restored from the trash', (tester) async {
    final store = memoryStore();
    final tree = TreeService(store);
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'Мысль', text: 'x');
    await tree.delete(n);

    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Корзина'));
    await tester.pumpAndSettle();
    expect(find.text('Мысль'), findsOneWidget);
    await tester.tap(find.text('Вернуть'));
    await tester.pumpAndSettle();
    expect((await store.node(n))!.deletedAt, isNull);
    await tearDownApp(tester, store);
  });
}
