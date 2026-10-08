# PromptTree

Ассистент для сырых идей и задач: заметки на ходу структурируются в ТЗ для Claude Code. Позже — AiCrew, команда ИИ-агентов, выполняющих задачи из дерева.

| Папка | Что внутри |
|---|---|
| `docs/` | согласованные решения: [P0](docs/P0-decisions.md), [архитектура PromptTree](docs/architecture-prompttree.md) |
| `design/` | дизайн-концепт и [решения по дизайну](design/DECISIONS.md) |
| `backend/` | сервер на Go + PostgreSQL ([README](backend/README.md)) |
| `shared/` | общий код Flutter: локальная БД, синхронизация, API, общие экраны и виджеты |
| `app-mobile/` | приложение для Android и iOS ([README](app-mobile/README.md)) |
| `app-desktop/` | приложение для Windows, macOS, Linux ([README](app-desktop/README.md)) |
