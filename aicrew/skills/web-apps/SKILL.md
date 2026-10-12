---
name: web-apps
description: Building web front-ends that work for real users — Flutter Web (PromptTree web) and plain web projects (TypeScript, Vite) — responsive layout, accessibility, auth in the browser, offline-friendly behaviour, and testing in a real browser. Use for tasks in web projects or the web build of a Flutter app.
when: web projects; changes to the PromptTree web app (web/)
roles: [tech_lead, junior]
projects: [web, flutter, prompttree]
version: "1"
---

# Web apps

## Flutter Web (PromptTree `web/`)
- Same screens as desktop/mobile come from `shared/`; the web shell only wires platform details. Layout switches by window width (≥ 900 px desktop layout, narrower — phone layout).
- Auth: the refresh token lives in an httpOnly cookie (`X-PT-Client: web`), never in localStorage. Don't add code that reads tokens in JS.
- Keyboard: `Ctrl+N` belongs to the browser — the web app uses `Alt+N`. Check shortcuts against browser defaults.
- Builds: `flutter build web` (release). Do not rely on service workers or SharedWorker in tests: some embedded browsers block them; test in headless Chrome.
- Text and input must work with Cyrillic, emoji and long strings; check that selection and IME input work.

## Plain web projects (TypeScript + Vite or similar)
- TypeScript strict; no `any` in public code. Small components, state close to where it is used.
- Semantic HTML first: buttons are `<button>`, links are `<a>`, every input has a label. Visible focus, keyboard navigation, `prefers-reduced-motion` respected.
- Responsive from 360 px wide; no horizontal page scroll; images have sizes and `alt`.
- Network: show loading and error states, retry where it makes sense; never lose what the user typed on a failed request.
- Secrets never go into front-end code or the bundle.

## Testing
- Unit tests for logic; component tests for UI states; browser tests (Playwright, see the `webapp-testing` skill) for the main flow: open, sign in, do the task, see the result.
- Before finishing run the project's `lint`, `test` and `build` scripts — the orchestrator runs the same.

## Do not
- Add a UI library, CSS framework or analytics script without the task asking for it (dependency changes need approval).
- Load code or fonts from new external hosts without a reason in the task.
