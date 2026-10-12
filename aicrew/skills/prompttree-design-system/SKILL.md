---
name: prompttree-design-system
description: The approved visual system of PromptTree — black and white palette, Onest and JetBrains Mono, shape markers instead of colour, soft shadows, radii and motion tokens — and the rule that every new visual decision needs the owner's approval. Use for any change to PromptTree screens (shared/, app-*, web/).
when: UI tasks in the PromptTree repository
roles: [tech_lead]
projects: [prompttree, flutter]
version: "1"
---

# PromptTree design system

Source of truth: `design/DECISIONS.md` (decisions with dates) and the code tokens in `shared/lib/src/ui/` (`theme.dart`, `motion.dart`, `glyphs.dart`). Read them before changing a screen.

## The rule
**Every new visual decision is approved by the owner before it is built.** Implement exactly what the task and `DECISIONS.md` describe. If the task needs a new screen, layout or pattern that is not decided yet, do not invent it — build the closest existing pattern and say in your summary what needs a decision. (The `frontend-design` skill is for new products, not for PromptTree screens.)

## Tokens (use them, never literal values)
- Colour: only black, white and greys — `context.pt.bg / fg / muted / faint / line / lineStrong / hover / selected / panel / shadow`, light and dark themes. No accent colour. Depth comes from surfaces, soft shadows and motion.
- Type: `ui(size, weight:)` — Onest for interface text; `mono(size)` — JetBrains Mono for service labels (ids, counters, times). Russian UI text.
- Radii: `PtRadius` 8 / 12 / 16 / 24. Filled inputs, 12 px buttons, cards and menus with a thin border and soft shadow.
- Motion: `PtMotion` 160 / 260 / 420 ms, curve `Cubic(0.2, 0, 0, 1)`; lists and menus appear in a stagger; respect the system "reduce motion".

## Meaning through shape
- Tree markers: project — filled square; folder — no marker (weight 500, counter, chevron); Raw Note — circle; AI Task — diamond: empty = not structured, filled = structured, half = edited after structuring.
- Left panel is a tree by indentation (16 px per level); lines are drawn only on the path to the open file. No folder icons.
- AiCrew kanban: circle — queue, half circle — in progress, outlined diamond — checks, filled diamond — waits for decision, square — merged, crossed square — failed. A pause is an inverted bar.

## Layout
- Desktop (≥ 900 px): side panel + content; AiCrew — board of six columns. Phone: one column, sheets from the bottom; AiCrew — list with «ждёт решения» first.
- Empty states: icon, title, one line of explanation and the action.

## Copy
- Name things the way the user sees them; buttons say what happens («Слить», «Вернуть с комментарием…»), and the result message uses the same word.
- Errors say what happened and what to do, without apologies.
