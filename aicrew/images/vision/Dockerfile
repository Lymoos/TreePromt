# Vision QA (этап 7.5b): снимки веб-сборки приложения в безголовом Chromium.
# Модель (Qwen-VL) получает только снимки; управления вводом здесь нет (ТЗ п. 12).
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends chromium python3 fonts-noto-core fonts-noto-color-emoji ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && useradd -m -u 1001 vision
COPY screenshot.sh /usr/local/bin/screenshot
RUN chmod 755 /usr/local/bin/screenshot
USER vision
ENTRYPOINT ["/usr/local/bin/screenshot"]
