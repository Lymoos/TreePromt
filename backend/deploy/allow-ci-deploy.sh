#!/usr/bin/env bash
# Разрешает GitHub Actions обновлять PromptTree на этом сервере. Запускать от root после install.sh:
#   bash allow-ci-deploy.sh 'ssh-ed25519 AAAA… prompttree-ci'
# Ключ получает доступ только к ci-deploy.sh: без shell, без проброса портов, без других команд.
set -euo pipefail
DIR=/opt/prompttree
KEY="${1:-}"

fail() { printf 'Ошибка: %s\n' "$*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || fail "запустите от root"
[ -f "$DIR/.env" ] || fail "сначала установите PromptTree (install.sh)"
[[ "$KEY" =~ ^ssh-ed25519\ [A-Za-z0-9+/=]+(\ .*)?$ ]] || fail "передайте открытый ключ ssh-ed25519 одной строкой"
command -v flock >/dev/null || apt-get install -y -qq util-linux >/dev/null

curl -fsSL https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy/ci-deploy.sh -o "$DIR/ci-deploy.sh"
chmod 700 "$DIR/ci-deploy.sh"

install -d -m 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
# Старый ключ CI (если был) заменяется новым.
sed -i '/ prompttree-ci$/d' /root/.ssh/authorized_keys
body=$(echo "$KEY" | awk '{print $1" "$2}')
echo "command=\"$DIR/ci-deploy.sh\",restrict $body prompttree-ci" >> /root/.ssh/authorized_keys
echo "Готово: GitHub Actions может обновлять PromptTree (и ничего больше)."
