# aicrew-host

Служба Execution Host AiCrew на домашнем ПК ([7.1](../docs/stage7-1-aicrew-architecture.md), [7.2](../docs/stage7-2-state-machine.md)). Сама подключается к серверу PromptTree, берёт задачи с lease, создаёт worktree, запускает Claude Code в одноразовом контейнере, автокоммитит, гоняет проверку и докладывает факты. Статусы меняет только сервер.

## Установка (один раз)

1. Docker Desktop должен быть запущен. Образ исполнителя:

```bash
docker build -t aicrew-agent-flutter:local images/agent-flutter
```

2. Собрать службу:

```bash
go build -o aicrew-host.exe ./cmd/aicrew-host
```

3. Войти на сервер (email и пароль владельца PromptTree):

```bash
./aicrew-host.exe login
```

4. Токен подписки Claude Code: получить его командой `claude setup-token` и сохранить (ввод скрыт, хранится зашифрованным DPAPI):

```bash
./aicrew-host.exe set-claude-token
```

5. Привязать репозиторий к проекту PromptTree (id проекта — из приложения) с профилем проверки:

```bash
./aicrew-host.exe add-repo -project <id проекта> -path C:/MyProfile/My_project/aicrew-sandbox -profile C:/MyProfile/My_project/aicrew-sandbox/aicrew-profile.json
```

## Работа

```bash
./aicrew-host.exe run
```

Одна задача за раз (решение 7.1). Настройки — `%APPDATA%\AiCrew\config.json`, секреты — `%APPDATA%\AiCrew\secrets\` (DPAPI, только для вашей учётной записи Windows).

## Что гарантируется

- Команды хоста — только `git` и `docker`, только argv; оболочки и интерпретаторы запрещены всегда.
- В контейнер монтируется только worktree задачи; токен Claude передаётся через окружение процесса docker, не через командную строку; секреты хоста дочерним процессам не наследуются.
- Факты (base/result commit, изменённые файлы, diff --stat, exit-коды проверок) собирает код; итог агента — только дополнительный контекст.
- Провал или отмена до слияния — worktree удаляется; при старте удаляются осиротевшие worktree.

Ещё не сделано (этапы 7.3–7.6): слияние и integration-ветка, полноценные профили и LLM-ревью, менеджер и окружение `windows_user`, сетевой allowlist-прокси для контейнера, секрет-сканер, Kill Switch.

## Тесты

```bash
go test ./...
```

Сквозной тест с настоящим сервером (запущенным с `ALLOW_REGISTRATION=true`):

```bash
AICREW_E2E_SERVER=http://127.0.0.1:58081 go test ./internal/host -run E2E -v
```
