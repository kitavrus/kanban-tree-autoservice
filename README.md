# kanban-tree

Go-сервис (Fiber v3 + pgx/v5 + Redis), auth: session.
Каркас сгенерирован /project-bootstrap; фичи добавляются /scaffold, прод-инфра — /infra-bootstrap.

## Quickstart

```bash
make fresh-start     # первый запуск: зависимости + .env + БД + миграции + seed + стенд
```

Вход: `admin` / `admin1234`. Ctrl-C гасит backend; БД остаётся.

Дальше в обычной работе:

```bash
make local-up        # поднять стенд (БД + миграции + backend)
make local-up-seed   # то же + seed-админ (идемпотентно)
make seed            # только seed-админ в уже поднятую БД (идемпотентно)
make local-down      # погасить всё, включая контейнеры БД
```

**Правил уже применённую миграцию?** golang-migrate не перечитывает применённые версии —
схема в томе останется старой, а код будет ждать новых колонок (симптом: 500
`column ... does not exist`). Для dev это лечится пересозданием тома:

```bash
make local-reset     # СНОСИТ том БД и поднимает стенд заново с сидом
```

Приложение: http://127.0.0.1:8080/health

## Структура

- `cmd/server` — бинарь (`--check-config` обязателен, контракт infra-bootstrap)
- `cmd/migrate` — `go run ./cmd/migrate up|down` (без внешнего migrate-CLI)
- `cmd/seed` — идемпотентный локальный admin (`make seed` или `make local-up-seed`)
- `scripts/dev/{up.sh,env.sh}` — оркестрация local-up и env для `make run`
- `internal/config` — env-контракт (DATABASE_URL, REDIS_ADDR, PORT)
- `internal/server` — Fiber-роутер, middleware; `/health` `/ready` `/metrics`
- `internal/features/auth` — login/logout/me
- `internal/features/users` — управление пользователями (список/создать/статус активен-неактивен/удалить, self-guard); новые фичи — `internal/features/{name}` (§GO-1)
- `internal/observability` — slog, OTel (OTLP_ENDPOINT, пусто = no-op), Prometheus
- `pkg/errorspkg` — wire-формат ошибок (§API-1: snake_case, 400 → `details.fields[]`)
- `pkg/testutil` — BaseSuite (dockertest PG+Redis) + LiveSuite (§TD-8, реальный бинарь+TCP)
- `migrations/` — golang-migrate (`NNNN_name.up/down.sql`)

## Тесты

```bash
make test                      # unit, всегда полные
make test-integration-scoped   # integration по затронутым пакетам (§EE-5)
make test-integration          # полный тег integration
```

E2E (Playwright, `e2e/`; разово — `make e2e-install`):

```bash
make e2e-isolated-full                              # полный прогон на чистом стенде (--retries=1)
make e2e-isolated SPECS=tests/board.spec.ts         # затронутые специ на чистом стенде
```

`e2e-isolated*` поднимает одноразовый стенд (`docker-compose.e2e.yml`: postgres в tmpfs +
redis, миграции, seed-админ, сервер) и сносит его после прогона — dev-БД не трогается,
каждый прогон стартует с пустой базы. Параллельные прогоны из разных сессий/ворктри не
мешают друг другу: у каждого свой compose-проект `kanban-tree-e2e-<pid>-<время>`, postgres и
redis получают случайные свободные порты, сервер — первый свободный от :8090, порт
бронируется за прогоном в `/tmp/kanban-tree-e2e-ports-<uid>/` (явно — `E2E_PG_PORT`,
`E2E_REDIS_PORT`, `E2E_APP_PORT`). На старте сносятся только брошенные
стенды упавших прогонов. `E2E_KEEP=1` оставляет БД для разбора (команда сноса печатается);
лог сервера — `e2e/.stand/server-<id>.log`. Два прогона в одном ворктри делят
`e2e/test-results` — там гоняй последовательно. `make e2e` / `make e2e-full` — прогон против
уже поднятого `make local-up` (данные тестов копятся в dev-БД).

## Линтеры

```bash
make lint            # все линтеры проекта (lint-go)
make lint-install    # поставить golangci-lint (разово)
make fmt             # форматирование (fmt-go)
```

Go — `.golangci.yml` (govet, staticcheck, errcheck, ineffassign, unused, misspell).

## Прод за nginx

Загрузка вложений (`POST /api/v1/tasks/:id/attachments`) принимает видео до 500 МБ потоком,
поэтому на этом роуте nginx нужен `client_max_body_size 501m` + `proxy_request_buffering off`
и увеличенные таймауты; остальные роуты — дефолтный лимит (приложение само режет тело на 4 МБ).
Образец server-блока: [`deploy/nginx/kanban-tree.conf.example`](deploy/nginx/kanban-tree.conf.example).
Если прод-инфра будет сгенерирована `/infra-bootstrap` — перенести эти `location` в её nginx-шаблон.
