---
name: flutter-widget-tests
description: How to write reliable Flutter widget tests — finders, pumping, async, text input, dialogs. Use when a task adds or changes Flutter UI or asks for widget tests.
when: Flutter UI changes; any task that says "add tests" in a Flutter project
roles: [tech_lead, junior]
projects: [flutter]
version: "1"
---

# Flutter widget tests

Write tests that check behaviour the user sees, not implementation details.

## Structure
- One `testWidgets` per behaviour; the name says what the user does and sees: `'marks a todo as done'`.
- Start every test with `await tester.pumpWidget(const MyApp());` — never share state between tests.
- Put repeated steps into small helpers (`Future<void> addTodo(WidgetTester t, String text)`).

## Finders
- Prefer `find.text`, `find.byType`, `find.byIcon`, `find.byTooltip`; use `find.byKey(ValueKey(...))` only when text is ambiguous.
- Assert counts precisely: `findsOneWidget`, `findsNothing`, `findsNWidgets(2)`.
- When several widgets match, scope the search: `find.descendant(of: find.byType(ListTile), matching: find.byType(Checkbox))`.

## Pumping
- After `tap`/`enterText`, call `await tester.pump();`.
- Use `await tester.pumpAndSettle();` for dialogs, routes and animations. Never use real `Future.delayed` waits.
- Dialogs: tap the trigger, `pumpAndSettle`, interact, tap the button, `pumpAndSettle` again, then assert the dialog is gone.

## Text input
- `await tester.enterText(find.byType(TextField), 'Купить молоко');` — Cyrillic, emoji and long text must work; test at least one of them when input matters.
- `await tester.testTextInput.receiveAction(TextInputAction.done);` to submit from the keyboard.

## Edge cases worth a test
- Empty and whitespace-only input is ignored.
- Deleting the last item; filters with no matching items (empty state).
- State survives the action that should not reset it (e.g. switching filters keeps items).

## Avoid
- Testing private methods or `State` fields directly.
- Golden tests unless the task asks for them.
- `expect(true, isTrue)`-style tests that cannot fail.
