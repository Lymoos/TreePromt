# PromptTree backend

Go-сервер PromptTree: авторизация, журнал операций синхронизации, pull по курсору, уведомления по WebSocket.
Архитектура: `../docs/architecture-prompttree.md`.

## Структура

| Путь | Что внутри |
|---|---|
| `cmd/server` | точка входа, graceful shutdown |
| `internal/core` | пользователи, устройства, сессии, Argon2id, JWT, ротация refresh-токенов |
| `internal/tree` | операции (`apply.go`, `handlers.go`), правила конфликтов, `Pull` |
| `internal/ai` | AI Structuring Engine: промпт `prompts/structure_v1.md`, клиент Gemini, валидатор, очередь ([этап 6](../docs/stage6-ai-structuring.md)) |
| `internal/realtime` | WebSocket-хаб: только «есть ревизия N», данные идут через pull |
| `internal/httpapi` | REST + WebSocket, CORS, лимит попыток входа |
| `migrations` | SQL-миграции (goose), применяются при старте |
| `deploy` | Docker Compose для VPS: backend, PostgreSQL, Caddy, бэкапы |

## API (`/api/v1`)

| Метод | Путь | Назначение |
|---|---|---|
| POST | `/auth/register` | регистрация; закрыта, когда уже есть пользователь (если `ALLOW_REGISTRATION` не `true`) |
| POST | `/auth/login` | `{email, password, device: {id, name, platform}}` → access + refresh |
| POST | `/auth/refresh` | ротация refresh-токена; повтор старого токена после 30 с отзывает сессию |
| POST | `/auth/logout` | отзыв сессии |
| GET | `/me` | текущий пользователь |
| POST | `/sync/push` | `{operations: [...]}`, до 500 штук; ответ на каждую: `applied` / `conflict` / `rejected` + состояние объекта |
| GET | `/sync/pull?cursor=N&limit=L` | изменения с ревизией > N; `has_more` — есть ещё страницы |
| POST | `/ai/structure` | `{node_id, source_revision}` → 202 `{request_id}`; результат приходит через pull. 409 `stale_source` — сначала синхронизироваться; 503 `ai_disabled` — нет ключа |
| GET | `/ai/structure/{id}` | состояние запроса |
| GET | `/ws` | первое сообщение `{"type":"auth","token":"<access>"}`, затем `{"type":"revision","revision":N}` |

Операции: `create_project`, `update_project`, `delete_project`, `restore_project`, `create_node`, `rename_node`,
`move_node`, `change_kind`, `delete_node`, `restore_node`, `set_raw_content`, `restore_version`, `resolve_conflict`,
`set_structured_text`, `apply_proposal`, `dismiss_proposal`.

## Локальный запуск и тесты

Тесты с базой создают и удаляют отдельную БД на каждый тест. Нужен PostgreSQL, где можно `CREATE DATABASE`:

```bash
docker run -d --name pt-test-pg -e POSTGRES_PASSWORD=pt_test_only -p 127.0.0.1:55432:5432 postgres:17-alpine
```

```bash
TEST_DATABASE_URL="postgres://postgres:pt_test_only@127.0.0.1:55432/postgres?sslmode=disable" go test ./...
```

Без `TEST_DATABASE_URL` тесты с базой пропускаются, остальные выполняются.

Eval-тесты промпта на настоящем Gemini (нужен ключ):

```bash
GEMINI_API_KEY=... go test ./internal/ai -run Eval -v
```

## Развёртывание на VPS

Образы backend и веб-версии (Caddy + Flutter Web) собирает GitHub Actions при каждом push в `main` и кладёт в ghcr.io; на сервере ничего не компилируется.

На сервере (Ubuntu/Debian, от root):

```bash
curl -fsSL https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy/install.sh -o install.sh
```

```bash
bash install.sh
```

Скрипт поставит Docker, спросит домен и ключ Gemini (скрытый ввод), сам сгенерирует пароли, проверит порты и DNS и запустит всё в `/opt/prompttree`. Если порт 443 занят (например, VPN), предложит другой порт для HTTPS. Обновление — `/opt/prompttree/update.sh`.

После первого запуска зарегистрировать владельца («Первый запуск: создать владельца»); дальше регистрация закрывается сама. Бэкапы — в `/opt/prompttree/backups`, раз в сутки, хранятся 14 дней.
