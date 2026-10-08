# Окружение container (решение 7.1): Flutter + Claude Code CLI, без секретов внутри.
# Токен подписки передаётся только на время запуска (CLAUDE_CODE_OAUTH_TOKEN), worktree монтируется в /work.
#   docker build -t aicrew-agent-flutter:local aicrew/images/agent-flutter
FROM ghcr.io/cirruslabs/flutter:3.27.3

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg \
 && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/* \
 && npm install -g @anthropic-ai/claude-code@2.1.287 \
 && node --version && claude --version

# Агент работает не от root. Flutter пишет в свой кэш — отдаём SDK пользователю агента.
RUN useradd -m -u 1001 agent \
 && chown -R agent:agent "$FLUTTER_HOME" /home/agent \
 && git config --system --add safe.directory '*'
USER agent
ENV HOME=/home/agent \
    PUB_CACHE=/home/agent/.pub-cache \
    CI=true
RUN flutter --version && flutter precache --no-android --no-ios --linux --web >/dev/null
WORKDIR /work
