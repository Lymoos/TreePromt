#!/bin/sh
# screenshot <каталог веб-сборки> <каталог для снимков>: телефон и десктоп.
set -eu
web="$1"; out="$2"
cd "$web"
python3 -m http.server 8080 --bind 127.0.0.1 >/dev/null 2>&1 &
server=$!
sleep 1
for size in 412,915 1280,800; do
  chromium --headless=new --no-sandbox --disable-gpu --hide-scrollbars --force-device-scale-factor=1 \
    --window-size="$size" --virtual-time-budget=20000 \
    --screenshot="$out/shot-${size%%,*}.png" http://127.0.0.1:8080/ >/dev/null 2>&1
done
kill "$server"
ls "$out"
