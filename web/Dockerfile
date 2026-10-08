# Образ фронта для VPS: Caddy (HTTPS, прокси /api) + собранный Flutter Web.
# Контекст сборки — корень репозитория (нужны shared/ и web/).
FROM ghcr.io/cirruslabs/flutter:3.27.3 AS build
WORKDIR /src
COPY shared ./shared
COPY web ./web
WORKDIR /src/web
RUN flutter pub get && flutter build web --release --no-web-resources-cdn

FROM caddy:2.8-alpine
COPY backend/deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /src/web/build/web /srv/web
