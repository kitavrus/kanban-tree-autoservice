# Input Bundle: phase-3-clients / стадия 2-spec-interview — generated 2026-09-25 14:19

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 1-research/output.md [sections]
## Triage
```yaml
scope: multi-endpoint        # clients CRUD + client_phones + equipment_types CRUD + equipment CRUD
tier: M
touches:
  db: true
  fe: true                   # SSR (html/template + vanilla JS) — фронтенд проекта, D30
  infra: false
  external: false
risk: reversible
novelty: standard-crud        # + optimistic locking version+409 — прецедента в коде нет
decisions: []                 # draft-spec отсутствует; решения — D1-D42 docs/plan.md, не DS-NNN
```

## DB Schema (baseline)

### `users` (образец: version-колонка есть, НЕ используется в API — D28 last-write-wins)
```sql
-- migrations/0001_init.up.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login text NOT NULL UNIQUE, password_hash text NOT NULL, name text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
-- migrations/0002_users_roles.up.sql
ALTER TABLE users ADD COLUMN role text NOT NULL DEFAULT 'executor' CHECK (role IN ('admin','executor')), ADD COLUMN phone text NOT NULL DEFAULT '';
UPDATE users SET role = 'admin', login = lower(login);
CREATE UNIQUE INDEX users_login_lower_uq ON users (lower(login));
```
Следующая свободная миграция: `0003_*` (0001/0002 заняты, `migrations/` имеет ровно 2 пары up/down).

### Целевая схема фазы 3 (docs/plan.md:29-34 — ЕЩЁ НЕ РЕАЛИЗОВАНА, факт для Design, не код)
```sql
clients(id, company_name, address, contact_name, contact_position, created_at, updated_at, version)
client_phones(id, client_id → clients, phone, note)                 -- 1..N телефонов
equipment_types(id, name, spec_keys TEXT[], is_active)
equipment(id, client_id, type_id, brand, model, sku, serial, specs JSONB [{k,v}], note)  -- DELETE только если не было в заказе (D26)
```
`task_equipment(task_id, equipment_id, completeness, condition)` — принадлежит фазе 5/6 (`tasks` ещё не существует) — связь «оборудование было в заказе» физически нечем проверить сейчас (см. §Граничные случаи).

`pgcrypto` уже создан миграцией 0001 — `gen_random_uuid()` доступен без повторной установки.

## Frontend (SSR-слой, D30 — Vue отсутствует, это фронтенд проекта)
- Существующие SSR-фичи: `web/ssr/users/templates/pages/list.html`, `web/ssr/users/templates/pages/form.html` (список+PRG-форма), `web/ssr/auth/templates/pages/login.html`, `web/ssr/home/templates/pages/index.html`, общие `web/ssr/_layout/templates/layouts/base.html`, `web/ssr/_layout/templates/layouts/bare.html`, `web/ssr/_layout/templates/pages/status.html`.
- `web/ssr/_layout/templates/layouts/base.html:54-58` — порядок подключения JS: `bootstrap.bundle.min.js`→`theme.js`→`api.js`→`ui-dialog.js`→`combobox.js`.
- `web/ssr/_shared/js/ui-dialog.js:81-84` — `KT.uiDialog/uiPrompt/uiConfirm/uiAlert`, Bootstrap Modal, `data-testid="ui-dialog"`/`ui-dialog-ok`/`ui-dialog-cancel`/`ui-dialog-input` — замена нативных prompt/confirm/alert прототипа (`prototype/index.html:1381-1383`).
- `web/ssr/_shared/js/combobox.js:26-157` — автообогащение ЛЮБОГО `select.form-select` (кроме `[multiple]`/`[data-native]`) в поисковый комбобокс (D6), `data-testid="combobox-dropdown"`, работает внутри открытой Bootstrap-модалки (combobox.js:69-70) — аналог `enhanceSelect` прототипа.
- Нет генерического JS для динамических списков (телефоны 1..N, характеристики N) — прототип использует ручной JS `drawPhones()`/`drawSpecs()` (`prototype/index.html:1352-1361`, `prototype/index.html:1413-1421`); в SSR-стеке проекта такого паттерна ещё нет — писать заново (chistый JS по образцу прототипа или AJAX-фрагмент `r.Fragment`).
- Переиспользуемый образец карточки/PRG-формы — `internal/features/users/ssr.go:1` целиком; общая 403/404-страница — `web/ssr/_layout/templates/pages/status.html:1`.
- Нормативных Vue-компонентов (`components/ui/`, StatusBadge и т.п.) в проекте НЕТ — не тот стек (SSR/Bootstrap, не Vue+Element Plus).
- Стек: vendored Bootstrap 5.3 (`web/ssr/_shared/vendor/bootstrap/bootstrap.min.css:1`, `web/ssr/_shared/vendor/bootstrap/bootstrap.bundle.min.js:1`), без CDN/Vue/React (D38); токены — `web/ssr/_shared/app.css:1` (primary `#2563EB`); тёмная тема — `data-bs-theme` через `web/ssr/_shared/js/theme.js:1` (US-1 `themes:both`).

### Расхождение с `/z-scaffold` (~/.claude/skills/z-scaffold/SKILL.md — вход для решения scaffold vs вручную)
1. z-scaffold штампует СВОЙ `layouts/base.html` на фичу → конфликт с общим `web/ssr/_layout/templates/layouts/base.html` (два `{{define "base"}}`) — обязателен к удалению (рецепт `internal/ssr/render.go:13-19`).
2. Сгенерированный `BaseVM` — БЕЗ поля `Shell` — добавляется руками в `models/dto/view_model.go:1` (z-scaffold-путь), иначе общий layout не отрендерит сайдбар/тему/CSRF.
3. z-scaffold разбивает на пакеты `handler/`(по файлу на действие)+`mappers/helpers.go:1`+`validators/form.go:1` — РАСХОДИТСЯ с фактическим паттерном: `internal/features/users` — один плоский пакет (`dto.go`/`handler.go`/`service.go`/`repo.go`/`ssr.go`/`validate.go`/`routes.go`), без под-пакетов.
4. Путь `web/ssr/{feature}/templates` — СОВПАДАЕТ (`internal/features/users/ssr.go:14`) — единственный несовпадающий с расхождением пункт.
5. z-scaffold заворачивает ошибки сервиса (кроме валидации формы) в `fiber.ErrInternalServerError`/`ErrNotFound` — расходится с `formError()` (users/ssr.go:136-156), которая через `errors.As` вытаскивает произвольные бизнес-коды (409 и т.п.) — доработка руками, если используется scaffold.
6. Пагинация z-scaffold — классическая нумерованная; фактический паттерн — простой счётчик `Total`/`Items` без номеров страниц (`internal/features/users/ssr.go:29-37`, `web/ssr/users/templates/pages/list.html:41`).
7. Динамические JSONB-характеристики оборудования по внешнему шаблону (`equipment_types.spec_keys`) — вне штатных типов полей манифеста z-scaffold — ручная доработка формы в любом случае.

**Для Design:** зафиксировать явный выбор — плоский пакет (как users) или разбитые z-scaffold-пакеты; в обоих случаях 3 шага D38-рецепта обязательны, а JSONB-характеристики дорабатываются руками.

### Contracts
- Защищённые: весь `/api/v1/users*` (RequireAPI+RequireAdmin), SSR `/users*` (RequireSSR+RequireAdmin) — по аналогии `/clients`/`/catalog` тоже `AdminOnly:true` в `internal/ssr/menu.go:25-26`.
- Публичные: `/health`, `/ready`, `/metrics`, `/login`, `/assets/ssr-shared/*`.
- CSRF — на все мутации всех маршрутов включая `/api/v1` (`internal/server/middleware/csrf.go:30-63` — глобально).
- `.env` — новых переменных фаза 3 не требует (DATABASE_URL/REDIS_ADDR/PORT уже покрывают).
- Rate limiting — только login (D41), вне зоны фичи.
- Не-REST протоколы — отсутствуют.

**Enum coverage:** US-1 `swagger:no` → §API-1/§8.1 n/a, swag-аннотации не пишутся. DB CHECK-образец: `users.role CHECK (role IN ('admin','executor'))` (migrations/0002_users_roles.up.sql:4). Целевая схема фазы 3 (clients/client_phones/equipment_types/equipment) enum-полей НЕ содержит — только text/boolean `is_active`/`TEXT[]`/`JSONB`.

### Error responses (образец формата)
```json
{"code":"VALIDATION","message":"validation failed","details":{"fields":[{"name":"login","message":"required"}]}}
{"code":"LOGIN_TAKEN","message":"Логин уже занят"}
```
Источник: `pkg/errorspkg/errors.go:46-76`. 401/403/404/500 — тот же `{code,message}` без `details`.

### Infrastructure
- Docker Compose — `postgres:18`+`redis:8` уже подняты фазой 1 (не читалось построчно — `touches.infra=false`); фаза 3 новых сервисов не добавляет.
- CI/CD — не обнаружено ни одного файла CI в `git ls-files` (нет `.gitlab-ci.yml`/`.github/workflows`).
- Dockerfile — не обнаружен в `git ls-files`.
- Внешние сервисы — Redis (сессии, `internal/session/store.go:9`), Postgres 18; новых не вводит.
- Secrets — `.env.example`, локальная разработка (CM-1=MVP, прода нет).
- Мониторинг/логирование — `internal/observability/logger.go:9` (slog `NewLogger`), `internal/observability/metrics.go:25` (Prometheus `/metrics`, `internal/server/router.go:58`), `internal/observability/otel.go:18` (`InitTracer`, `cmd/server/main.go:38`) — фаза 3 новых метрик не требует.

### Tests
- Unit: `internal/features/users/validate_test.go:1`, `internal/server/middleware/csrf_test.go:1`, `internal/ssr/render_test.go:1` — обычный `go test`.
- Integration (§TD-8, тег `integration`): `pkg/testutil/base_suite.go:22` — `BaseSuite` (dockertest PG18+Redis, миграции применены, `SeedUser`); `pkg/testutil/live_suite.go:20` — `LiveSuite` (боевой бинарь `go build ./cmd/server`, реальный TCP, `/health`-опрос); `pkg/testutil/login.go:30` — `Browser` (свой cookie jar, редиректы не следуются, `CSRFToken()`/`Request()`/`Do()`/`Login()` через реальный `/api/v1/auth/login`, auth-стабы запрещены).
  - Образец: `internal/features/users/users_http_live_test.go:20-27` — `UsersLiveSuite{LiveSuite{BinaryPkg:"./cmd/server"}}`, покрывает happy-path/authz/валидацию-мусор→400/дубликат→409/конкурентность(`LastAdminSuite`, гонка на `FOR UPDATE`)/SSR PRG.
  - `internal/features/auth/auth_http_live_test.go:1`, `internal/server/shell_http_live_test.go:1` — тот же паттерн.
  - `app.Test` НЕ используется нигде в существующих тестах — соответствует запрету §TD-8 на новые `app.Test`-сьюты.
- Vue-тестов нет (Vue отсутствует). E2E — Playwright:
  - `e2e/playwright.config.ts:7-22` — `projects:[chromium, devices['Desktop Chrome']]`, `workers:1`, `fullyParallel:false`, `PWSLOWMO` env (§TD-3/§TD-5 соблюдены).
  - `e2e/fixtures/test.ts:24-92` — `allowHttpErrors` (auto, ловит console.error/pageerror/HTTP≥400, §TD-4), `uniq`, `makeUser`, `loginAs`, `adminPage`, `runAdmin`/`adminApi` (worker-scope).
  - `e2e/fixtures/auth.ts:32-82` — `loginContext()` (реальный вход, csrf из cookie `kt_csrf`), `AdminApi` (сидирование данных), `SEED_ADMIN={login:'admin',password:'admin1234'}`, `RUN` — метка прогона (не литерал в ассертах, §TD-6.2).
  - `e2e/pages/ShellPage.ts:5-75` — POM общего layout (sidebar/theme/dialog/combobox, §TD-6.1); `e2e/pages/UsersPage.ts:4-77` — образец экрана: полноценные страницы `/users/new`,`/users/:id` (PRG), НЕ модалки — расходится с прототипом, где карточка клиента/оборудования — Bootstrap-модалка.
  - `e2e/tests/users.spec.ts:8-233` — образец глубины TD-6: happy path (US-001), exhaustive-валидация (US-002, TD-6.5), занятый логин (US-003), правка (US-004/005), блокировка+фильтры-в-URL (US-010..013), конкурентность last-write-wins (US-020, D28-для-users), адаптив 360/1280px без overflow (US-021, `adaptive:min`).
- Coverage — `coverage.out`/threshold не обнаружены.

## Паттерны в коде
- Один плоский Go-пакет на фичу (`internal/features/{feature}`), НЕ разбит на handler/mappers/validators (расходится с z-scaffold SSR).
- Sentinel `*errorspkg.AppError` пакетными переменными, маппинг pg-ошибок→доменные в репозитории, матчинг в сервисе `errors.Is`.
- Ручная field-валидация кодами сообщений (`msgRequired` и т.п.), не текстом — SSR транслирует код в текст.
- SSR VM обязательно несёт `Shell ssr.Shell`, заполняется `shell.Build(c,title)` перед рендером (D38).
- PRG для форм: `POST /{feature}` → `303 See Other` → `?flash=created`; ошибка формы — тот же метод, соответствующий HTTP-код, форма перерисовывается с `vm.Errors`.
- Whitelist для пользовательского ввода, идущего в SQL как идентификатор (сортировка/фильтр-роль) — тихий откат к дефолту, не 500.
- `escapeLike()` — экранирование ILIKE-паттернов при поиске по подстроке.
- Права читаются из БД на каждый запрос, не кэшируются в сессии.
- Все HTTP-интеграционные тесты — через `testutil.LiveSuite`, НЕ `app.Test` (§TD-8).
- E2E — POM (`e2e/pages/*Page.ts`) + `data-testid` на все интерактивные элементы; `ShellPage` переиспользуется каждой фичей.
- UUID/ID: единой Go-точки генерации (`pkg/ids`) НЕТ — генерация только в БД `DEFAULT gen_random_uuid()`; Go лишь валидирует уже присвоенный uuid (`uuid.Validate`, users/service.go:94,137). Новые таблицы следуют тому же паттерну.

## Источник: draft-spec.md [full]
MISSING

## Источник: draft-spec-decisions.md [full]
MISSING

## Паспорт (выжимка)
# Passport: phase-3-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 0 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
