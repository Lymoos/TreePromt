# Окружение container (решение 7.1): Flutter + Claude Code CLI, без секретов внутри.
# Токен подписки передаётся только на время запуска (CLAUDE_CODE_OAUTH_TOKEN), worktree монтируется в /work.
#   docker build -t aicrew-agent-flutter:local aicrew/images/agent-flutter
#
# Лёгкая сборка: официальный Node.js 22 (npm уже есть, без сторонних установочных скриптов)
# + Flutter только для Linux/веба, без Android SDK. Проверки полигона — analyze, test, build web.
FROM node:22-bookworm-slim

ARG FLUTTER_VERSION=3.27.3
ARG CLAUDE_CODE_VERSION=2.1.287

# Каждый шаг падает сам по себе: никаких «curl | bash», где ошибка теряется в конвейере.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates git curl unzip xz-utils zip \
 && rm -rf /var/lib/apt/lists/*

RUN npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
 && npm cache clean --force \
 && claude --version

# Агент работает не от root; Flutter пишет в свой кэш — SDK принадлежит пользователю агента.
RUN useradd -m -u 1001 agent \
 && mkdir -p /opt/flutter && chown agent:agent /opt/flutter \
 && git config --system --add safe.directory '*'
USER agent
ENV HOME=/home/agent \
    PUB_CACHE=/home/agent/.pub-cache \
    PATH=/opt/flutter/bin:/home/agent/.pub-cache/bin:$PATH \
    CI=true \
    FLUTTER_SUPPRESS_ANALYTICS=true

RUN git clone --depth 1 --branch "${FLUTTER_VERSION}" https://github.com/flutter/flutter.git /opt/flutter \
 && flutter config --no-analytics --no-cli-animations --no-enable-android --no-enable-ios \
 && flutter precache --no-android --no-ios --linux --web \
 && flutter --version

WORKDIR /work
