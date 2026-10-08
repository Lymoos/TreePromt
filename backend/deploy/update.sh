#!/usr/bin/env bash
# Обновление PromptTree до последних образов из GitHub Actions. Данные и .env не трогаются.
set -euo pipefail
cd "$(dirname "$0")"
curl -fsSL https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy/docker-compose.yml -o docker-compose.yml
docker compose pull
docker compose up -d --remove-orphans
docker image prune -f >/dev/null
docker compose ps
