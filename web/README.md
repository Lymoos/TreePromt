# PromptTree — веб-версия

Flutter Web на том же общем коде, что телефон и ПК ([`../shared`](../shared)). Раскладка подстраивается под ширину окна: от 840 px — две панели, как на ПК; уже — как на телефоне.

## Как устроено

- **Данные** — SQLite в WebAssembly (Drift): в Chrome хранится в OPFS, иначе в IndexedDB. Как и в приложениях, всё сначала пишется локально, потом уходит на сервер.
- **Вход** — refresh-токен только в httpOnly-cookie (заголовок `X-PT-Client: web`), access-токен — только в памяти вкладки. После перезагрузки страницы новый access-токен берётся через cookie.
- **Адрес сервера** — тот же, откуда открыта страница: Caddy отдаёт веб и проксирует `/api` в backend.
- **Создать** — Alt N (AI Task) и Alt Shift N (черновик): Ctrl N в браузере занят самим браузером.

## Файлы для SQLite в браузере

`web/sqlite3.wasm` — из релиза пакета sqlite3, версия должна совпадать с `sqlite3` в `pubspec.lock`:

```bash
curl -L -o web/sqlite3.wasm https://github.com/simolus3/sqlite3.dart/releases/download/sqlite3-2.9.4/sqlite3.wasm
```

`web/drift_worker.js` — собирается из `worker/drift_worker.dart` той же версией Drift, что и приложение:

```bash
dart compile js -O4 worker/drift_worker.dart -o web/drift_worker.js
```

После обновления drift или sqlite3 оба файла нужно обновить.

## Сборка и выкладка

```bash
flutter build web --release --no-web-resources-cdn
```

`--no-web-resources-cdn` — CanvasKit отдаётся с нашего сервера, без запросов к CDN Google. Содержимое `build/web` копируется в `backend/deploy/web`, его раздаёт Caddy с заголовками COOP/COEP (нужны для OPFS).

## Локальная проверка полной связки

Из `backend/deploy` (нужен файл `.env.local` с `POSTGRES_PASSWORD`, `JWT_SECRET`, `DOMAIN=localhost`):

```bash
docker compose -p prompttree-local --env-file .env.local -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

Открыть `http://localhost:8088` в обычном браузере. Встроенные панели некоторых IDE не поддерживают SharedWorker и service worker — там хранилище не откроется, это ограничение панели.

## Тесты

```bash
flutter test
```
