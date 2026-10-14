#!/bin/bash
# Изолированный E2E-прогон (make e2e-isolated / e2e-isolated-full): одноразовый стенд
# со своей БД и своим инстансом сервера, не пересекающийся с dev-стендом (make local-up).
#
#   1. docker-compose.e2e.yml: postgres (tmpfs) + redis на своих портах — всегда пустые;
#   2. миграции + seed-админ (admin/admin1234 — от него E2E-фикстуры заводят админа прогона);
#   3. сервер собирается в bin/server-e2e и стартует на E2E_APP_PORT с временным UPLOADS_DIR;
#   4. playwright (chromium, headed, 1 воркер) против E2E_BASE_URL этого сервера;
#   5. trap: сервер гасится, контейнеры и tmp-вложения удаляются (E2E_KEEP=1 — оставить
#      стенд поднятым для разбора; команда сноса печатается в конце прогона).
#
# Параллельные прогоны (несколько сессий/ворктри на одной машине) не мешают друг другу:
#   - у каждого прогона свой compose-проект kanban-tree-e2e-<pid>-<время> (контейнеры, сеть,
#     тома — по нему), чужие стенды скрипт НЕ трогает;
#   - порты по умолчанию — свободные: PG/Redis получают случайный host-порт от docker,
#     приложение — первый свободный порт от 8090, забронированный за прогоном (каталог
#     /tmp/kanban-tree-e2e-ports-<uid>/<порт>, см. reserve_port); явные E2E_PG_PORT /
#     E2E_REDIS_PORT / E2E_APP_PORT по-прежнему работают (занятый или забронированный
#     явный порт приложения — ошибка);
#   - бинарь и лог сервера — на прогон: bin/server-e2e-<id>, e2e/.stand/server-<id>.log;
#   - на старте сносятся только «брошенные» стенды: проекты kanban-tree-e2e-<pid>-*, чей
#     процесс-владелец уже мёртв (прогон упал до trap, kill -9).
# Два прогона в ОДНОМ ворктри делят e2e/test-results — гоняй их последовательно.
#
# env .env НЕ читается намеренно: стенд детерминирован и не зависит от dev-настроек.
# SPECS — подмножество спек (tests/x.spec.ts ...), PW_ARGS — доп. флаги playwright,
# E2E_HEADLESS=1 — без окна браузера.
set -euo pipefail
cd "$(dirname "$0")/../.."
ROOT="$PWD"

RUN_ID="$$-$(date +%s)"
PROJECT="kanban-tree-e2e-$RUN_ID"
# Пустое значение → случайный свободный host-порт (docker-compose.e2e.yml).
export E2E_PG_PORT="${E2E_PG_PORT:-}" E2E_REDIS_PORT="${E2E_REDIS_PORT:-}"

# Абсолютный путь: cleanup срабатывает уже после `cd e2e` (запуск playwright).
COMPOSE=(docker compose -p "$PROJECT" -f "$ROOT/docker-compose.e2e.yml")
STAND_DIR="$ROOT/e2e/.stand"
SERVER_LOG="$STAND_DIR/server-$RUN_ID.log"
SERVER_BIN="$ROOT/bin/server-e2e-$RUN_ID"

listener_pids() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | sort -u || true; }
port_busy() { [ -n "$(listener_pids "$1")" ]; }

# Бронь порта приложения. Между выбором порта и стартом сервера — десятки секунд
# (compose up, миграции, сборка), и одной проверки «порт свободен» мало: два прогона,
# стартовавшие почти одновременно, оба видят 8090 свободным, оба поднимают на нём сервер
# (bind не падает), и playwright одного ходит в сервер и БД другого. Поэтому порт
# бронируется атомарно — mkdir каталога <порт> в общем для всех ворктри месте (не
# $TMPDIR: у сессий в песочнице он свой); внутри pid владельца. Бронь снимает cleanup;
# брошенную (владелец мёртв) забирает следующий прогон.
PORT_LOCK_DIR="${E2E_PORT_LOCK_DIR:-/tmp/kanban-tree-e2e-ports-$(id -u)}"
PORT_LOCK=""

reserve_port() { # $1 — порт; 0 — бронь наша, 1 — порт занят или забронирован живым прогоном
	local lock="$PORT_LOCK_DIR/$1" owner
	mkdir -p "$PORT_LOCK_DIR"
	if ! mkdir "$lock" 2>/dev/null; then
		owner="$(cat "$lock/pid" 2>/dev/null || true)"
		# Нет pid — владелец между mkdir и записью: бронь живая.
		if [ -z "$owner" ] || kill -0 "$owner" 2>/dev/null; then return 1; fi
		# Брошенная: забирает тот, чей mv прошёл (rename атомарен), остальные — мимо.
		mv "$lock" "$lock.stale.$$" 2>/dev/null || return 1
		rm -rf "$lock.stale.$$"
		mkdir "$lock" 2>/dev/null || return 1
	fi
	echo "$$" >"$lock/pid"
	PORT_LOCK="$lock"
	if port_busy "$1"; then
		release_port
		return 1
	fi
}

release_port() {
	if [ -n "$PORT_LOCK" ]; then rm -rf "$PORT_LOCK"; fi
	PORT_LOCK=""
}

# Брошенные стенды прошлых прогонов (владелец умер до trap): снести, чтобы не копились.
# Живые стенды соседних сессий не трогаем — их pid жив.
for stale in $(docker ps -a --filter "label=com.docker.compose.project" \
	--format '{{.Label "com.docker.compose.project"}}' | sort -u | grep -E '^kanban-tree-e2e-[0-9]+-[0-9]+$' || true); do
	owner="${stale#kanban-tree-e2e-}"
	owner="${owner%%-*}"
	if ! kill -0 "$owner" 2>/dev/null; then
		docker compose -p "$stale" -f "$ROOT/docker-compose.e2e.yml" down -v --remove-orphans >/dev/null 2>&1 || true
	fi
done

mkdir -p "$STAND_DIR"
UPLOADS_DIR="$(mktemp -d "${TMPDIR:-/tmp}/kanban-tree-e2e-uploads.XXXXXX")"
SERVER_PID=""

cleanup() {
	local code=$?
	if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
		kill "$SERVER_PID" 2>/dev/null || true
		wait "$SERVER_PID" 2>/dev/null || true
	fi
	rm -f "$SERVER_BIN"
	release_port
	if [ "${E2E_KEEP:-}" = "1" ]; then
		echo "E2E_KEEP=1: сервер погашен, БД оставлена (DATABASE_URL=${DATABASE_URL:-}), вложения — $UPLOADS_DIR"
		echo "   снести: docker compose -p $PROJECT -f docker-compose.e2e.yml down -v"
	else
		"${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 ||
			echo "!! не удалось снести e2e-контейнеры: docker compose -p $PROJECT -f docker-compose.e2e.yml down -v" >&2
		rm -rf "$UPLOADS_DIR"
	fi
	exit "$code"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# Порт приложения: явный — бронируем и проверяем, чужой процесс не гасим (в отличие от
# scripts/dev/up.sh: это может быть что угодно, например стенд соседней сессии);
# не задан — первый свободный и никем не забронированный от 8090.
if [ -n "${E2E_APP_PORT:-}" ]; then
	if ! reserve_port "$E2E_APP_PORT"; then
		echo "!! порт $E2E_APP_PORT занят или забронирован другим e2e-прогоном ($PORT_LOCK_DIR/$E2E_APP_PORT) — освободи его или задай другой: E2E_APP_PORT=NNNN make e2e-isolated" >&2
		exit 1
	fi
else
	E2E_APP_PORT=8090
	until reserve_port "$E2E_APP_PORT"; do
		E2E_APP_PORT=$((E2E_APP_PORT + 1))
		if [ "$E2E_APP_PORT" -gt 8190 ]; then
			echo "!! нет свободного порта приложения в 8090–8190 — задай E2E_APP_PORT (брони: $PORT_LOCK_DIR)" >&2
			exit 1
		fi
	done
fi

"${COMPOSE[@]}" up -d --wait

# Фактические host-порты (случайные, если не заданы явно): «127.0.0.1:NNNNN».
E2E_PG_PORT="$("${COMPOSE[@]}" port postgres 5432 | awk -F: '{print $NF}')"
E2E_REDIS_PORT="$("${COMPOSE[@]}" port redis 6379 | awk -F: '{print $NF}')"

export DATABASE_URL="postgres://admin:e2e@127.0.0.1:${E2E_PG_PORT}/app?sslmode=disable"
export REDIS_ADDR="127.0.0.1:${E2E_REDIS_PORT}"
export PORT="$E2E_APP_PORT"
export LOG_LEVEL="${E2E_LOG_LEVEL:-info}"
export OTLP_ENDPOINT=""
export SESSION_TTL="12h"
export SESSION_ABSOLUTE_MAX="168h"
export SESSION_COOKIE_SECURE="false"
export LOGIN_MAX_FAILURES="10"
export LOGIN_FAILURE_WINDOW="15m"
export FIRM_NAME="" FIRM_ADDRESS="" FIRM_PHONE=""
export UPLOADS_DIR
export UPLOADS_GC_INTERVAL="6h" UPLOADS_GC_MIN_AGE="1h"
# Служебная физическая уборка тестовых данных (DELETE /api/v1/test/purge/:kind/:id,
# soft-delete-core DS-027): фикстуры E2E убирают заказы через неё. Только тестовый стенд.
export TEST_PURGE_ENABLED="true"

go run ./cmd/migrate up
go run ./cmd/seed

go build -o "$SERVER_BIN" ./cmd/server
"$SERVER_BIN" >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 60); do
	if curl -fsS "http://127.0.0.1:${E2E_APP_PORT}/health" >/dev/null 2>&1; then break; fi
	if ! kill -0 "$SERVER_PID" 2>/dev/null; then
		echo "!! сервер упал при старте — лог: $SERVER_LOG" >&2
		tail -n 30 "$SERVER_LOG" >&2 || true
		exit 1
	fi
	sleep 0.5
done
curl -fsS "http://127.0.0.1:${E2E_APP_PORT}/health" >/dev/null || {
	echo "!! сервер не ответил на /health за 30 с — лог: $SERVER_LOG" >&2
	exit 1
}

# Страховка поверх брони (прогон со старым скриптом, чужой процесс, занявший порт уже
# после брони): /health ответил, но порт должен слушать ТОЛЬКО наш сервер. Второй
# слушатель (например, на [::1] при нашем 0.0.0.0) — и часть запросов уйдёт не в наш
# стенд, а тесты пойдут против чужого кода и чужой БД.
LISTENERS="$(listener_pids "$E2E_APP_PORT" | tr '\n' ' ' | sed 's/ $//')"
if [ "$LISTENERS" != "$SERVER_PID" ]; then
	echo "!! порт $E2E_APP_PORT слушает не только наш сервер (наш pid $SERVER_PID, слушатели: ${LISTENERS:-нет}) — прогон остановлен, чтобы тесты не ушли в чужой стенд:" >&2
	lsof -nP -iTCP:"$E2E_APP_PORT" -sTCP:LISTEN >&2 || true
	exit 1
fi
# 127.0.0.1, а не localhost: сервер слушает только IPv4 (Fiber, tcp4), а localhost на
# macOS сначала резолвится в ::1 — там может жить чужой процесс.
echo "e2e-стенд $PROJECT: http://127.0.0.1:${E2E_APP_PORT} (pg :${E2E_PG_PORT}, redis :${E2E_REDIS_PORT}; лог сервера: $SERVER_LOG)"

cd e2e
HEADED="--headed"
if [ "${E2E_HEADLESS:-}" = "1" ]; then HEADED=""; fi
# shellcheck disable=SC2086 # SPECS/PW_ARGS/HEADED — списки аргументов, word splitting нужен
E2E_BASE_URL="http://127.0.0.1:${E2E_APP_PORT}" \
	npx playwright test ${SPECS:-} --project=chromium $HEADED --workers=1 ${PW_ARGS:-}
