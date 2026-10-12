---
name: dart-clean-code
description: Dart and Flutter code conventions for this owner's projects — small widgets, immutable models, null safety, formatting, no new dependencies without approval. Use for any Dart/Flutter code change.
when: any task that edits .dart files
roles: [tech_lead, junior]
projects: [flutter]
version: "1"
---

# Dart / Flutter code

## Must
- Code passes `dart format`, `flutter analyze` (no warnings) and `flutter test` — the orchestrator runs exactly these.
- Sound null safety: no `!` on values that can really be null; handle the null case.
- Prefer `final` and `const`; models are immutable (`copyWith`) unless the task's existing code already uses mutable models — then follow the existing style.
- Dispose controllers (`TextEditingController`, `AnimationController`, `ScrollController`) in `dispose`.
- After `await` in a `State`, check `mounted` before `setState` or using `context`.

## Structure
- Split a widget when `build` grows beyond ~60 lines or a part has its own state.
- Keep business logic out of widgets when it grows: a small class or function that can be unit-tested.
- Follow the existing folder layout and the architecture contract (`architecture.yaml`) if the repository has one.

## Do not
- Add packages to `pubspec.yaml` unless the task explicitly asks — such changes wait for the owner's approval anyway.
- Leave `print`, commented-out code or TODOs without a reason.
- Rename or move unrelated code "while you're there".

## User-facing text
- UI strings are Russian unless the project already uses something else; keep them short and consistent with existing wording.
