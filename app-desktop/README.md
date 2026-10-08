# PromptTree — приложение для ПК

Flutter, Windows (основная цель), macOS и Linux. Окно по согласованному концепту: слева панель-дерево (вариант C), справа редактор. Экраны и логика — общие с телефоном, из [`../shared`](../shared).

| Действие | Как |
|---|---|
| Открыть заметку | клик по файлу в дереве |
| Действия над узлом | правый клик: открыть, переименовать, переместить, сменить тип, удалить |
| Новая AI Task / черновик | Ctrl N / Ctrl Shift N (Cmd на macOS) — работает при любом фокусе; или «Создать» |
| Корзина, вход, выход | внизу панели |

Данные хранятся в `%APPDATA%\app.prompttree\PromptTree\prompttree.sqlite`, токены — в диспетчере учётных данных Windows.

## Сборка под Windows

Нужен включённый режим разработчика Windows (плагинам нужны символические ссылки).

Flutter 3.27 не распознаёт Visual Studio 2026 и пытается использовать генератор VS 2019. Пока Flutter не обновлён, собираем через CMake из комплекта VS:

```bash
flutter build windows --release
```

Первая команда упадёт на генерации проекта, но успеет создать `windows/flutter/ephemeral`. Затем:

```bash
"/d/Program Files/VS/Common7/IDE/CommonExtensions/Microsoft/CMake/CMake/bin/cmake.exe" -S windows -B build/windows/vs2026 -G "Visual Studio 18 2026" -A x64
```

```bash
"/d/Program Files/VS/Common7/IDE/CommonExtensions/Microsoft/CMake/CMake/bin/cmake.exe" --build build/windows/vs2026 --config Release --target INSTALL --parallel
```

Готовое приложение: `build/windows/vs2026/runner/Release/prompttree_desktop.exe` (папку `Release` можно копировать целиком).

## Тесты

```bash
flutter test
```

Скриншоты окна в обеих темах — `test/goldens/`. Иконка Windows генерируется из `AppIcon`:

```bash
flutter test test/tool/app_icon_test.dart
```
