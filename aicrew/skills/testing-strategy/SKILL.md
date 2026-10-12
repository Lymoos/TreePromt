---
name: testing-strategy
description: What and how to test so the tests catch real bugs and never flake — regression tests for fixes, deterministic time and data, golden screenshots, integration tests with real databases. Use for any task that adds behaviour or fixes a bug, in any language.
when: every task that changes behaviour; always for bug fixes
roles: [tech_lead, junior]
projects: [flutter, go, web, roblox, prompttree]
version: "1"
---

# Testing strategy

## What to test
- Test behaviour a user or caller relies on, through the public surface (widget, HTTP handler, service method) — not private helpers.
- **Every bug fix comes with a test that fails without the fix.** Name it after the situation: `rejected plan with a comment goes back to the Manager`.
- Cover the edges the change touches: empty input, the last item, permission denied, network down, the second call (idempotency), two actors at once (races).
- Prefer a few precise assertions over many vague ones; a test that cannot fail is noise.

## Deterministic tests (no flakes)
- **Never depend on the wall clock.** Inject a clock (`TreeService(clock: …)`, `svc.now = …`) and use fixed dates; anything that renders a time must use the fixed one.
- No real sleeps or timing races: drive time with fakes (`fakeAsync`, test clocks), wait for conditions, not for durations.
- No random data without a fixed seed; no dependency on test order or leftovers — each test builds its own state.
- Network and models are faked at the boundary (`MockClient`, fake reviewer/agent); the real thing gets a separate end-to-end test.

## Levels
- Unit — pure logic, fast, many.
- Integration — real database (fresh per test), real git in a temp folder, real file system.
- Widget / UI — Flutter `testWidgets` with the real theme; golden screenshots only for screens whose look was approved, with real fonts loaded and fixed data.
- End-to-end — one happy path plus the most important failure, against a real server.

## Golden screenshots
- Regenerate with `--update-goldens` only when the change in look is intended, and review the new image.
- A diff that is only anti-aliasing (same layout, edges of glyphs) means another machine rendered them; regenerate on the project's reference machine.

## Before finishing
Run exactly the checks the orchestrator will run (they are listed in your task). Red tests are never "flaky" until you have proven why.
