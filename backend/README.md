# PromptTree backend

Go-сервер PromptTree: авторизация, журнал операций синхронизации, pull по курсору, уведомления по WebSocket.
Архитектура: `../docs/architecture-prompttree.md`.

## Структура

| Путь | Что внутри |
|---|---|
| `cmd/server` | точка входа, graceful shutdown |
| `internal/core` | пользователи, устройства, сессии, Argon2id, JWT, ротация refresh-токенов |
| `internal/tree` | операции (`apply.go`, `handlers.go`), правила конфликтов, `Pull` |
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
| GET | `/ws` | первое сообщение `{"type":"auth","token":"<access>"}`, затем `{"type":"revision","revision":N}` |

Операции: `create_project`, `update_project`, `delete_project`, `restore_project`, `create_node`, `rename_node`,
`move_node`, `change_kind`, `delete_node`, `restore_node`, `set_raw_content`, `restore_version`, `resolve_conflict`.

## Локальный запуск и тесты

Тесты с базой создают и удаляют отдельную БД на каждый тест. Нужен PostgreSQL, где можно `CREATE DATABASE`:

```bash
docker run -d --name pt-test-pg -e POSTGRES_PASSWORD=pt_test_only -p 127.0.0.1:55432:5432 postgres:17-alpine
```

```bash
TEST_DATABASE_URL="postgres://postgres:pt_test_only@127.0.0.1:55432/postgres?sslmode=disable" go test ./...
```

Без `TEST_DATABASE_URL` тесты с базой пропускаются, остальные выполняются.

## Развёртывание на VPS

```bash
cd deploy && cp .env.example .env
```

Заполнить `.env` (домен и два секрета), затем:

```bash
docker compose up -d --build
```

После первого запуска зарегистрировать владельца через `POST /api/v1/auth/register`; дальше регистрация закрывается сама.
Бэкапы — в `deploy/backups/`, раз в сутки, хранятся 14 дней.
