---
name: prompttree-conventions
description: Product and code conventions of the PromptTree / AiCrew project itself — Local First sync, op-log, domain separation, Russian UI, design approval. Use for tasks in the PromptTree repository.
when: tasks in the PromptTree repository (backend, shared, app-*, web, aicrew)
roles: [tech_lead]
projects: [prompttree]
version: "1"
---

# PromptTree conventions

## Architecture (docs/architecture-prompttree.md)
- Local First: clients write to the local database (Drift) and an operation log; the server applies operations idempotently. Never make a client feature depend on being online.
- Conflicts are field-level; for text conflicts both versions are kept — never silently drop a user's text.
- Domains are separate: PromptTree (`tree`, `core`) does not know about AiCrew (`exec`); they are linked only by ids.
- AI output is validated by code against a JSON contract before it is stored (stage 6).

## UI
- Every visual decision is approved by the owner first. Do not invent new screens, icons, colours or layouts; implement exactly what the task describes and follow `design/DECISIONS.md`.
- Russian UI texts; short, calm wording.

## AiCrew
- Models never set task statuses; the server moves statuses after code checks.
- AiCrew never writes to `main`; results go to `aicrew/integration`.

## Process
- Changes come with tests. Keep commits focused; do not refactor unrelated code.
