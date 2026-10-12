#!/usr/bin/env bash
# Автодеплой: этот скрипт — единственное, что может запустить ключ GitHub Actions
# (command="…" в /root/.ssh/authorized_keys, см. allow-ci-deploy.sh). CI передаёт только sha коммита.
set -euo pipefail
DIR=/opt/prompttree

SHA="${SSH_ORIGINAL_COMMAND:-}"
[[ "$SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "ожидается полный sha коммита main" >&2; exit 2; }

cd "$DIR"
# Один деплой за раз: следующий пуш подождёт, пока закончится текущий.
exec 9>"$DIR/.deploy.lock"
flock 9

# Скрипт обновления берём из того же коммита, что и образы.
curl -fsSL "https://raw.githubusercontent.com/Lymoos/TreePromt/$SHA/backend/deploy/update.sh" -o update.sh.new
chmod +x update.sh.new
mv update.sh.new update.sh
echo "$(date -Is) deploy $SHA" >> "$DIR/deploy.log"
./update.sh "$SHA"
