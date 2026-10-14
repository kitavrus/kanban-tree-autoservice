#!/bin/bash
# Экспортирует env для локального запуска (make run / scripts/dev/up.sh).
# Источник — .env (cp .env.example .env, gitignored); дефолты — на случай отсутствия файла.
set -a
[ -f .env ] && . ./.env
set +a
export DATABASE_URL="${DATABASE_URL:-postgres://admin:dev@127.0.0.1:5442/app?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6389}"
export PORT="${PORT:-8090}"
