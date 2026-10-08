import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:prompttree_shared/app.dart';
import 'package:prompttree_shared/prompttree_shared.dart';

import 'support.dart';

Future<void> pumpAt(WidgetTester tester, Size size, DriftLocalStore store) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(app(store));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('wide browser window shows two panes', (tester) async {
    final store = memoryStore();
    await TreeService(store).createProject('P');
    await pumpAt(tester, const Size(1280, 800), store);
    expect(find.byType(DesktopShell), findsOneWidget);
    expect(find.text('Откройте заметку слева'), findsOneWidget);
    await tearDownApp(tester, store);
  });

  testWidgets('narrow browser window (phone) shows the mobile layout', (tester) async {
    final store = memoryStore();
    await TreeService(store).createProject('P');
    await pumpAt(tester, const Size(390, 844), store);
    expect(find.byType(MobileShell), findsOneWidget);
    expect(find.byKey(const Key('create')), findsOneWidget);
    await tearDownApp(tester, store);
  });
}
