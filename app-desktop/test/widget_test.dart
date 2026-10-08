import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

Future<void> desktopSize(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1280, 800);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

void main() {
  testWidgets('clicking a file opens it in the right pane; switching files keeps text', (tester) async {
    await desktopSize(tester);
    final store = memoryStore();
    final tree = TreeService(store);
    final p = await tree.createProject('P');
    final a = await tree.createNode(projectId: p, kind: NodeKind.aiTask, name: 'Первая', text: 'a');
    await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'Вторая', text: 'b');

    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    expect(find.text('Откройте заметку слева'), findsOneWidget);

    await tester.tap(find.text('P'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Первая'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('editor-text')), findsOneWidget);

    // Печатаем и сразу переключаемся, не дожидаясь паузы: текст не должен потеряться.
    await tester.enterText(find.byKey(const Key('editor-text')), 'a — дописано');
    await tester.tap(find.text('Вторая'));
    await tester.pumpAndSettle();
    expect((await store.content(a))!.rawContent, 'a — дописано');
    expect(find.text('b'), findsOneWidget);
    await tearDownApp(tester, store);
  });

  testWidgets('right click opens context menu; delete moves to trash', (tester) async {
    await desktopSize(tester);
    final store = memoryStore();
    final tree = TreeService(store);
    final p = await tree.createProject('P');
    final n = await tree.createNode(projectId: p, kind: NodeKind.rawNote, name: 'Мысль');

    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Мысль'), buttons: kSecondaryButton);
    await tester.pumpAndSettle();
    expect(find.text('Переместить'), findsOneWidget);
    await tester.tap(find.text('Удалить'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Удалить'));
    await tester.pumpAndSettle();
    expect((await store.node(n))!.deletedAt, isNotNull);
    await tearDownApp(tester, store);
  });

  testWidgets('Ctrl+N creates an AI task in the selected project and opens it', (tester) async {
    await desktopSize(tester);
    final store = memoryStore();
    final p = await TreeService(store).createProject('P');

    await tester.pumpWidget(app(store));
    await tester.pumpAndSettle();
    await tester.tap(find.text('P'));
    await tester.pumpAndSettle();

    await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.keyN);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('name-field')), 'Задача с клавиатуры');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();

    final node = (await store.db.select(store.db.nodes).get()).single;
    expect(node.kind, NodeKind.aiTask);
    expect(node.projectId, p);
    expect(find.byKey(const Key('editor-text')), findsOneWidget);
    await tearDownApp(tester, store);
  });
}
