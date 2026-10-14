#!/bin/bash
# Локальный стенд одной командой (make local-up / local-up-seed): поднимает
# postgres+redis (docker compose), гоняет миграции, стартует backend.
# Ctrl-C гасит backend; БД остаётся — полная остановка make local-down.
# SEED=1 — дополнительно создать seed-админа (login=admin/password=admin1234, идемпотентно).
set -euo pipefail
cd "$(dirname "$0")/../.."

docker compose up -d --wait
. scripts/dev/env.sh

# Освобождаем СВОИ порты перед стартом: забытый прошлый прогон не убивается сам
# (Ctrl-C гасит только процессы своей сессии), а два dev-сервера на одном порту
# сосуществуют — старый держит [::1] (куда резолвится localhost на macOS) и отдаёт
# устаревший код, новый берёт *:порт. Симптом: «Install @vitejs/plugin-vue to handle
# .vue files» при корректном конфиге. strictPort такой частичный конфликт НЕ ловит —
# bind на * при занятом ::1 успешен. Поэтому гасим по порту явно, как make local-down.
free_port() { # $1 — порт, $2 — что там живёт
	local pids
	pids=$(lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null || true)
	if [ -n "$pids" ]; then
		echo "порт $1 занят ($2, PID $(echo "$pids" | tr '\n' ' ')) — гашу прошлый прогон"
		kill $pids 2>/dev/null || true
		sleep 1
	fi
}
free_port "$PORT" "backend"

# Провал миграций почти всегда = том БД пережил смену схемы/кредов (Postgres
# инициализирует роль и базу ТОЛЬКО на пустом томе, golang-migrate не перечитывает
# применённые версии). Подсказываем лечение вместо голого стектрейса.
if ! go run ./cmd/migrate up; then
	echo
	echo "!! миграции не прошли. Если это dev-стенд и данные не нужны — том БД,"
	echo "!! вероятно, остался от прошлой схемы/кредов. Лечение: make local-reset"
	exit 1
fi

if [ "${SEED:-}" = "1" ]; then
	go run ./cmd/seed
fi

PIDS=()
cleanup() {
	echo "гашу backend..."
	kill "${PIDS[@]}" 2>/dev/null || true
	wait 2>/dev/null || true
}
trap cleanup INT TERM

go run ./cmd/server &
PIDS+=($!)


wait
