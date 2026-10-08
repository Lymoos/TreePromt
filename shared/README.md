# prompttree_shared

Общий код PromptTree для `app-mobile`, `app-desktop` и `web`. Слои — по [архитектуре](../docs/architecture-prompttree.md):

| Папка | Что внутри |
|---|---|
| `lib/src/core` | UUIDv7, дробные ключи порядка |
| `lib/src/models` | типы операций, видов узлов, статусов |
| `lib/src/domain` | `LocalStore` (абстракция хранилища), `TreeService` (действия пользователя), `buildTree` (панель C) |
| `lib/src/data` | Drift/SQLite: таблицы, реализация `LocalStore`, открытие БД на всех платформах |
| `lib/src/sync` | очередь, push/pull, перебазирование неотправленных правок, WebSocket |
| `lib/src/api` | HTTP-клиент, токены, обновление сессии |
| `lib/src/ui` | тема (ч/б токены), иконка C, маркеры, панель-дерево |

После изменения таблиц:

```bash
dart run build_runner build --delete-conflicting-outputs
```

Тесты: `flutter test`. Сквозной тест с настоящим сервером — см. `test/server_integration_test.dart`.
