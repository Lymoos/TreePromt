# Скиллы AiCrew

Каталог, из которого Manager выбирает `required_skills` задачи (docs/stage7-5-manager-skills-mcp.md, раздел 2).
Скилл — папка `<имя>/SKILL.md` с заголовком (name, description, when, roles, projects, version, license);
версия при подключении фиксируется хэшем содержимого. Новые скиллы и правки — только через коммит, который
одобряет владелец: скилл — недоверенный контент уровня кода.

| Скилл | Для чего | Источник |
|---|---|---|
| `dart-clean-code` | код на Dart/Flutter | свой |
| `flutter-widget-tests` | виджет-тесты Flutter | свой |
| `go-backend` | бэкенд на Go | свой |
| `prompttree-conventions` | архитектура и процесс PromptTree | свой |
| `prompttree-design-system` | утверждённый дизайн PromptTree | свой |
| `testing-strategy` | что и как тестировать без «мигающих» тестов | свой |
| `db-migrations` | миграции PostgreSQL (goose) и Drift | свой |
| `web-apps` | веб: Flutter Web и обычные веб-проекты | свой |
| `roblox-luau` | игры Roblox на Luau, проверка в Studio через MCP | свой |
| `frontend-design` | дизайн новых интерфейсов | Anthropic |
| `webapp-testing` | проверка веб-приложений в браузере (Playwright) | Anthropic |
| `mcp-builder` | создание MCP-серверов | Anthropic |

Скиллы Anthropic взяты из https://github.com/anthropics/skills (коммит dbd4588, 12.10.2026) без изменений,
лицензия Apache 2.0 — файл `LICENSE.txt` в папке каждого из них.
