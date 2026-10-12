#!/usr/bin/env bash
# Обновление PromptTree до образов из GitHub Actions. Данные и .env не трогаются.
#   ./update.sh         — последние образы (latest)
#   ./update.sh <sha>   — образы конкретного коммита main (так вызывает автодеплой из CI)
set -euo pipefail
cd "$(dirname "$0")"

REF="${1:-main}"
export PT_VERSION="${1:-latest}"

curl -fsSL "https://raw.githubusercontent.com/Lymoos/TreePromt/$REF/backend/deploy/docker-compose.yml" -o docker-compose.yml.new
mv docker-compose.yml.new docker-compose.yml
docker compose pull
docker compose up -d --remove-orphans

# Сервер считается обновлённым, только когда новый backend отвечает.
for _ in $(seq 1 30); do
  if docker compose exec -T caddy wget -qO- http://backend:8080/api/v1/healthz >/dev/null 2>&1; then
    docker image prune -f >/dev/null
    docker compose ps
    echo "Готово: PromptTree $PT_VERSION работает."
    exit 0
  fi
  sleep 2
done
echo "Backend не ответил за минуту. Последние строки журнала:" >&2
docker compose logs --tail 50 backend >&2
exit 1
