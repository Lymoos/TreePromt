#!/usr/bin/env bash
# Записывает ключ Google AI Studio в /opt/prompttree/.env и перезапускает backend.
# Ключ читается из stdin, а не из аргументов, чтобы не попасть в историю и список процессов:
#   bash set-gemini-key.sh      — вставить ключ, Enter
# С ПК владельца это делает .aicrew-local/set-gemini.ps1 (скрытый ввод, ключ идёт по ssh).
set -euo pipefail
DIR=/opt/prompttree
ENV="$DIR/.env"
[ -f "$ENV" ] || { echo "Нет $ENV — сначала install.sh" >&2; exit 1; }

IFS= read -r KEY || true
KEY=$(printf '%s' "$KEY" | tr -d '\r\n\t ')
[[ "$KEY" =~ ^[A-Za-z0-9._-]{20,200}$ ]] || { echo "Это не похоже на ключ Google AI Studio." >&2; exit 2; }

# Проверка у Google до сохранения: ключ в заголовке, не в адресе.
code=$(curl -s -o /tmp/gemini-check.json -w '%{http_code}' -m 20 -H "x-goog-api-key: $KEY" \
  https://generativelanguage.googleapis.com/v1beta/models || true)
if [ "$code" != 200 ]; then
  echo "Google не принял ключ (HTTP $code):" >&2
  grep -o '"message": *"[^"]*"' /tmp/gemini-check.json >&2 || true
  rm -f /tmp/gemini-check.json
  exit 3
fi
rm -f /tmp/gemini-check.json

tmp=$(mktemp "$ENV.XXXXXX")
chmod 600 "$tmp"
grep -v '^GEMINI_API_KEY=' "$ENV" > "$tmp" || true
echo "GEMINI_API_KEY=$KEY" >> "$tmp"
mv "$tmp" "$ENV"

cd "$DIR"
docker compose up -d backend
sleep 3
if docker compose logs --since 20s backend 2>&1 | grep -q 'GEMINI_API_KEY is not set'; then
  echo "Backend не увидел ключ — проверьте $ENV" >&2
  exit 4
fi
echo "Ключ …${KEY: -4} сохранён и проверен, backend перезапущен: «Структурировать» и Logic QA на сервере включены."
