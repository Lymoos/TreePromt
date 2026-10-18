# Окружение для монорепозитория PromptTree (этап 7.9): образ агента Flutter + Go для backend и aicrew.
#   docker build -t aicrew-agent-monorepo:local aicrew/images/agent-monorepo
# Тесты backend, которым нужна база (TEST_DATABASE_URL), здесь пропускаются: их гоняет CI.
FROM aicrew-agent-flutter:local

ARG GO_VERSION=1.23.4
USER root
RUN curl -fsSLo /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
 && tar -C /usr/local -xzf /tmp/go.tgz \
 && rm /tmp/go.tgz \
 && /usr/local/go/bin/go version
USER agent
ENV PATH=/usr/local/go/bin:/home/agent/go/bin:$PATH \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=mod
WORKDIR /work
