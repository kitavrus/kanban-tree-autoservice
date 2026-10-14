# >>> project-bootstrap >>>
# Генерится project_scaffold.py — правки блока руками = CONFLICT при регене.
APP_PORT ?= 8080
FRONTEND_PORT ?= 5173

# ── Первый запуск после генерации каркаса ───────────────────────────────────
# Одна команда вместо цепочки «зависимости → .env → поднять стенд с сидом».
# Идемпотентна: существующий .env НЕ перезатирается, зависимости доставляются.
fresh-start:  ## первый запуск: зависимости + .env + БД + миграции + seed-админ + стенд
	@if [ -f .env ]; then echo ".env уже есть — оставляю как есть"; \
		else cp .env.example .env && echo ".env создан из .env.example"; fi
	go mod tidy
	$(MAKE) local-up-seed
.PHONY: fresh-start

# ── Локальный стенд одной командой ──────────────────────────────────────────
# Поднимает postgres+redis (docker compose), миграции, backend (Go).
# Ctrl-C гасит backend; БД остаётся — полная остановка make local-down.
# "dev" в имени НЕТ намеренно — зарезервировано под удалённый контур /infra-bootstrap
# (setup-remote-dev/deploy-remote-dev/health-remote-dev); локальный стенд — local-*.
local-up:  ## поднять весь стенд локально (backend + БД)
	@bash scripts/dev/up.sh
.PHONY: local-up

local-up-seed:  ## то же, что local-up, но ещё создать seed-админа (admin/admin1234)
	@SEED=1 bash scripts/dev/up.sh
.PHONY: local-up-seed

local-reset:  ## пересоздать локальную БД с нуля (том сносится!) и поднять стенд заново
	@echo "СНОСИТСЯ локальный том БД (docker compose down -v) — dev-данные будут потеряны."
	$(MAKE) local-down
	docker compose down -v
	$(MAKE) local-up-seed
.PHONY: local-reset

local-down:  ## погасить контейнеры БД + оставшиеся процессы backend
	-@pids=$$(lsof -nP -iTCP:$(APP_PORT) -sTCP:LISTEN -t 2>/dev/null); \
		if [ -n "$$pids" ]; then echo "гашу backend на :$(APP_PORT) (PID $$pids)"; kill $$pids; fi

	docker compose down
.PHONY: local-down

build:  ## сборка бинаря
	go build -o bin/server ./cmd/server
.PHONY: build

run:  ## локальный запуск (env из .env, зависимости — make local-up)
	@test -f .env || (echo "нет .env — cp .env.example .env" && exit 1)
	. scripts/dev/env.sh && go run ./cmd/server
.PHONY: run

test:  ## unit-тесты (всегда полные, §EE-5)
	go test ./... -count=1
.PHONY: test

# trap-cleanup: зависшие тест-контейнеры удаляются при любом выходе (в т.ч. по ошибке) — ТОЛЬКО
# этого прогона (label kanban-tree.testutil.run=$TESTUTIL_RUN_ID, ставит pkg/testutil): параллельный
# прогон соседней сессии/ворктри не задевается.
test-integration:  ## полный integration-прогон (тег integration)
	@export TESTUTIL_RUN_ID="$$$$-$$(date +%s)"; \
	trap 'docker ps -aq --filter "label=kanban-tree.testutil.run=$$TESTUTIL_RUN_ID" | xargs -r docker rm -f -v >/dev/null 2>&1 || true' EXIT; \
	go test -tags=integration ./... -count=1 -race
.PHONY: test-integration

# Integration-тесты в скоупе задачи (§EE-5): затронутые пакеты резолвит go_affected_pkgs.py
# (git diff HEAD + untracked [+ BASE]); FULL-триггер (go.mod/go.sum, деструктивные миграции)
# расширяет скоуп до ./... сам. Пустой вывод — шаг пропускается (NO-GO-CHANGES).
test-integration-scoped:  ## integration по затронутым пакетам (§EE-5, дефолт гейтов)
	@export TESTUTIL_RUN_ID="$$$$-$$(date +%s)"; \
	trap 'docker ps -aq --filter "label=kanban-tree.testutil.run=$$TESTUTIL_RUN_ID" | xargs -r docker rm -f -v >/dev/null 2>&1 || true' EXIT; \
	PKGS=$$(python3 ~/.claude/tools/go_affected_pkgs.py --repo . $${BASE:+--base $$BASE}); \
	if [ -z "$$PKGS" ]; then echo "integration: NO-GO-CHANGES — skip"; \
	else go test -tags=integration $$PKGS -count=1 -race; fi
.PHONY: test-integration-scoped

lint: lint-go  ## все линтеры проекта
.PHONY: lint

lint-go:  ## golangci-lint (конфиг .golangci.yml)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "нет golangci-lint — поставь: make lint-install"; exit 1; }
	golangci-lint run ./...
.PHONY: lint-go

lint-install:  ## поставить golangci-lint в $(GOPATH)/bin
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
.PHONY: lint-install

fmt: fmt-go  ## форматирование всего кода
.PHONY: fmt

fmt-go:  ## gofmt по каталогам кода
	gofmt -w cmd internal pkg
.PHONY: fmt-go

swagger:  ## swag-аннотации → docs/ (US-1 swagger:yes)
	swag init -g cmd/server/main.go -o docs/swagger
.PHONY: swagger

migrate-up:  ## миграции вверх (go run ./cmd/migrate, без внешнего CLI)
	@test -f .env || (echo "нет .env — cp .env.example .env" && exit 1)
	. scripts/dev/env.sh && go run ./cmd/migrate up
.PHONY: migrate-up

migrate-down:  ## откат одной миграции
	@test -f .env || (echo "нет .env — cp .env.example .env" && exit 1)
	. scripts/dev/env.sh && go run ./cmd/migrate down
.PHONY: migrate-down
# <<< project-bootstrap <<<

# --- kanban-tree: seed-админ и демо-данные (вне блока project-bootstrap; D36, AC-019) ---
seed:  ## создать seed-админа admin/admin1234 (идемпотентно; БД и миграции — make local-up)
	@test -f .env || (echo "нет .env — cp .env.example .env" && exit 1)
	. scripts/dev/env.sh && go run ./cmd/seed
.PHONY: seed

seed-demo:  ## залить демо-данные доски одной командой (no-op, если заказы уже есть)
	@test -f .env || (echo "нет .env — cp .env.example .env" && exit 1)
	. scripts/dev/env.sh && go run ./cmd/seed-demo
.PHONY: seed-demo

# --- kanban-tree: E2E (вне блока project-bootstrap; CLAUDE.md §TD-3 — chromium, headed) ---
e2e-install:  ## зависимости Playwright + chromium
	npm --prefix e2e install
	npm --prefix e2e run install-browser
.PHONY: e2e-install

e2e:  ## E2E по стенду make local-up (SPECS=tests/x.spec.ts — затронутые специ)
	cd e2e && npx playwright test $(SPECS) --project=chromium --headed --workers=1 --reporter=list
.PHONY: e2e

e2e-full:  ## полный E2E-прогон (§EE-2: --retries=1)
	cd e2e && npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list
.PHONY: e2e-full

# Изолированный стенд: своя одноразовая БД (tmpfs) + redis + сервер (первый свободный порт от
# 8090) — dev-БД не раздувается, прогон не зависит от накопленных данных; параллельные прогоны
# разных сессий не пересекаются (свой compose-проект и порты на прогон). Основной способ гонять E2E;
# e2e/e2e-full выше — быстрая проверка против уже поднятого make local-up.
e2e-isolated:  ## E2E на чистом изолированном стенде (SPECS=tests/x.spec.ts — затронутые специ)
	@SPECS="$(SPECS)" PW_ARGS="--reporter=list $(PW_ARGS)" bash scripts/e2e/run.sh
.PHONY: e2e-isolated

e2e-isolated-full:  ## полный E2E на чистом изолированном стенде (§EE-2: --retries=1)
	@PW_ARGS="--retries=1 --reporter=list $(PW_ARGS)" bash scripts/e2e/run.sh
.PHONY: e2e-isolated-full
