#!/usr/bin/env bash
# Установка PromptTree на VPS (Ubuntu/Debian), запускать от root:
#   curl -fsSL https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy/install.sh -o install.sh
#   less install.sh      # посмотреть, что он делает
#   bash install.sh
# Повторный запуск безопасен: существующий .env не перезаписывается.
set -euo pipefail

DIR=/opt/prompttree
RAW=https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy

say()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
fail() { printf '\nОшибка: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "запустите от root (sudo bash install.sh)"
command -v apt-get >/dev/null || fail "скрипт рассчитан на Ubuntu/Debian"

say "1/6 Docker"
if ! command -v docker >/dev/null || ! docker compose version >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl openssl iproute2 dnsutils >/dev/null
  # Официальный репозиторий Docker: свежий Docker Engine и плагин compose.
  install -m 0755 -d /etc/apt/keyrings
  # shellcheck disable=SC1091
  . /etc/os-release
  curl -fsSL "https://download.docker.com/linux/${ID}/gpg" -o /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${ID} ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin >/dev/null
fi
docker compose version

mkdir -p "$DIR/backups"
cd "$DIR"

if [ -f .env ]; then
  say "2/6 Настройки: .env уже есть, оставляю как есть"
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
else
  say "2/6 Настройки"
  read -rp "Домен (DNS должен указывать на этот сервер): " DOMAIN
  [ -n "$DOMAIN" ] || fail "домен обязателен: без него не получить HTTPS-сертификат"

  HTTPS_PORT=443
  if ss -tlnH "sport = :443" | grep -q .; then
    echo "Порт 443 занят:"; ss -tlnp "sport = :443" | tail -n +2
    read -rp "Какой порт использовать для HTTPS вместо 443 (например 8443): " HTTPS_PORT
    [[ "$HTTPS_PORT" =~ ^[0-9]+$ ]] || fail "нужен номер порта"
  fi
  PUBLIC_ORIGIN="https://$DOMAIN"
  [ "$HTTPS_PORT" = 443 ] || PUBLIC_ORIGIN="https://$DOMAIN:$HTTPS_PORT"

  echo "Ключ Google AI Studio для «Структурировать» (ввод скрыт; Enter — пропустить, добавить позже в $DIR/.env):"
  read -rsp "GEMINI_API_KEY: " GEMINI_API_KEY; echo

  umask 077
  cat > .env <<EOF
DOMAIN=$DOMAIN
PUBLIC_ORIGIN=$PUBLIC_ORIGIN
HTTPS_PORT=$HTTPS_PORT
POSTGRES_PASSWORD=$(openssl rand -hex 24)
JWT_SECRET=$(openssl rand -hex 32)
GEMINI_API_KEY=$GEMINI_API_KEY
EOF
  chmod 600 .env
  echo "Сохранено в $DIR/.env (доступ только root)."
fi

say "3/6 Проверка портов и DNS"
if ss -tlnH "sport = :80" | grep -q . && ! docker ps --format '{{.Names}}' | grep -q caddy; then
  echo "Порт 80 занят:"; ss -tlnp "sport = :80" | tail -n +2
  fail "порт 80 нужен Let's Encrypt для выпуска сертификата — освободите его"
fi
if [ "${HTTPS_PORT:-443}" != 443 ] && ss -tlnH "sport = :${HTTPS_PORT}" | grep -q . && ! docker ps --format '{{.Names}}' | grep -q caddy; then
  fail "порт ${HTTPS_PORT} тоже занят"
fi
MY_IP=$(curl -fsS4 https://api.ipify.org || true)
DNS_IP=$(dig +short A "$DOMAIN" | tail -n1 || true)
if [ -n "$MY_IP" ] && [ "$DNS_IP" != "$MY_IP" ]; then
  echo "Внимание: $DOMAIN указывает на '${DNS_IP:-ничего}', а этот сервер — $MY_IP."
  echo "Сертификат не выпустится, пока DNS не обновится. Продолжаю; Caddy будет пытаться сам."
fi

say "4/6 Файлы"
curl -fsSL "$RAW/docker-compose.yml" -o docker-compose.yml
curl -fsSL "$RAW/update.sh" -o update.sh && chmod +x update.sh

say "5/6 Запуск"
docker compose pull
docker compose up -d
docker compose ps

say "6/6 Готово"
echo "Откройте ${PUBLIC_ORIGIN:-https://$DOMAIN} — первый сертификат может выпускаться до минуты."
echo "Первый вход: «Первый запуск: создать владельца». После этого регистрация закроется сама."
echo "Обновление: $DIR/update.sh   Логи: cd $DIR && docker compose logs -f backend"
echo "Бэкапы базы: $DIR/backups (раз в сутки, 14 дней)."
