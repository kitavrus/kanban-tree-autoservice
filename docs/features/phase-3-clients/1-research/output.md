---
stage: 1-research
feature: phase-3-clients
tier: M
---
# Research: Клиенты и оборудование (фаза 3)

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

## Module
- Модуль: `github.com/kitavrus/kanban-tree` (go.mod:1). Go 1.26.0 (go.mod:3).

## Что уже есть в коде

### Backend
Единственная реализованная предметная фича — `internal/features/users` (сотрудники, фаза 2). Плоский пакет, НЕ разбит на handler/mappers/validators (см. §Расхождение с /z-scaffold):
- `internal/features/users/dto.go:7-44` — wire JSON snake_case: `UserResponse`, `ListResponse{Items,Total,Limit,Offset}`, `CreateRequest`, `UpdateRequest`.
- `internal/features/users/handler.go:20-88` — `List`/`Get`/`Create`/`Update`; `actorOf(c)` (handler.go:76-80) читает `middleware.UserIDKey`/`SessionIDKey` из Locals.
- `internal/features/users/service.go:17-23` — sentinel `*errorspkg.AppError`; `sortColumns` whitelist (service.go:31-37, защита от SQL-инъекции в ORDER BY); `defaultLimit=50`/`maxLimit=200` (service.go:26-27).
- `internal/features/users/repo.go:59-166` — pgx-репозиторий: `List`(:59), `GetByID`(:109), `Create`(:113), `Update`(:129, транзакция + `FOR UPDATE`); `escapeLike()` (:104-106) экранирует `%`/`_`/`\`; `mapUnique()` (:168-174) — pg-код `23505`→`errLoginTaken`.
- `internal/features/users/validate.go:44-101` — ручная field-валидация (НЕ go-playground/validator); коды `msgRequired`/`msgTooShort`/`msgTooLong`/`msgInvalidFormat`/`msgInvalid` (validate.go:19-24); лимиты — именованные константы (validate.go:27-36).
- `internal/features/users/routes.go:11-24` — `Register(app, api, a, h, s)`: JSON `api.Group("/users", a.RequireAPI(), middleware.RequireAdmin(nil))` (:12); SSR `app.Group("/users", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))` (:18).
- `internal/features/users/ssr.go:14` — `const Root = "web/ssr/users/templates"`; PRG-хендлеры списка/форм; `ListVM`/`FormVM` несут `Shell ssr.Shell` (D38-рецепт); `formError()` (ssr.go:136-156) конвертирует `*errorspkg.AppError`→`vm.Errors`; `fieldText()` (ssr.go:169-190) — код→русский текст.

Optimistic-locking (`version`+409) прецедента НЕТ: `users.version` в БД есть (migrations/0001_init.up.sql:12), но API/сервис его не читает и не сравнивает (last-write-wins). Для клиентов/заказов D28 требует version-конфликт — это НОВЫЙ паттерн для фазы 3.

Sentinel-ошибки — образец (`internal/features/users/service.go:17-23`): `ErrUserNotFound`(404), `ErrLoginTaken`(409), `ErrSelfBlock`(409), `ErrLastAdmin`(409). Для новой фичи по аналогии: `CLIENT_NOT_FOUND`, `CLIENT_HAS_ORDERS`(409, D26 — заказов ещё нет, см. §Граничные случаи), `EQUIPMENT_TYPE_NOT_FOUND`, `EQUIPMENT_NOT_FOUND`, `EQUIPMENT_IN_ORDERS`(409, D26), `VERSION_CONFLICT`(409, D28 — новый код).

### Error flow (внутренний)
- `pkg/errorspkg/errors.go:19-24` — `AppError{Code,HTTP,Message,Fields}`; конструкторы `BadRequest/Unauthorized/Forbidden/NotFound/Conflict/Internal/Validation` (errors.go:32-44).
- `pkg/errorspkg/errors.go:58-76` — `FiberErrorHandler`, общий error handler (`fiber.Config{ErrorHandler: errorspkg.FiberErrorHandler}`, internal/server/router.go:34): `*AppError`→JSON `{code,message,details.fields[]}`; `*fiber.Error`→`{code:"HTTP_ERROR"}`; прочее→500 `{code:"INTERNAL"}` + `slog.Error`.
- Репозиторий→sentinel Go-error (`errLoginTaken`, `pgx.ErrNoRows`)→сервис матчит `errors.Is`→`*AppError` (users/service.go:97-102,118-123).
- SSR: `formError()` (users/ssr.go:136-156) через `errors.As` — 5xx пробрасывается, 4xx рендерится в форму под полем.

### Валидация
- Ручная (users/validate.go:63-101), НЕ go-playground/validator: `Input.validate(...)` возвращает `[]errorspkg.Field{Name,Message}`, вызывается в сервисе (users/service.go:110,141) — server-only (§FE-1.1).
- `errorspkg.Validation(fields...)` (pkg/errorspkg/errors.go:40-44) — 400 с `details.fields[]`.
- `internal/server/json.go:11-21` — `strictJSON`: `DisallowUnknownFields`+запрет мусора после объекта → 400, не 500; включён `fiber.Config{JSONDecoder: strictJSON}` (internal/server/router.go:35).

### Маршруты (образец)
```
GET/POST /api/v1/users, GET/PUT /api/v1/users/:id     (RequireAPI+RequireAdmin)
GET      /users, /users/new; POST /users, /users/:id  (RequireSSR+RequireAdmin)
POST/GET /api/v1/auth/login|logout|me; GET/POST /login; POST /logout
```
Регистрация — `internal/server/router.go:94-98` (`server.New`): `api := app.Group("/api/v1")`, затем `feature.Register(app, api, ...)`.

### Middleware
- `internal/server/middleware/auth.go:46-51` — `Auth{Store,Load,Secure,MaxAge}`; `RequireAPI()`(:93-104)→401; `RequireSSR()`(:107-118)→302 `/login?redirect=`; `RequireAdmin(forbidden)`(:122-132) — API передаёт nil→403 JSON, SSR передаёт `s.status.Forbidden`; `resolve()`(:55-80) читает права из БД на каждый запрос (D42).
- `internal/server/middleware/csrf.go:30-63` — собственный stateless double-submit (D40, НЕ Fiber middleware/csrf): cookie `kt_csrf` (64 hex), заголовок `X-Csrf-Token`/поле `_csrf`, проверка `Origin` (csrf.go:51-53); включён глобально до сессии `app.Use(middleware.CSRF(...))` (internal/server/router.go:67) — покрывает `/api/v1` и SSR.
- `internal/server/middleware/logging.go:12` — `RequestLogger(logger)`, включён `internal/server/router.go:42`.

### SSR-инфраструктура (фронтенд проекта, D30 — Vue нет)
- `internal/ssr/render.go:37-127` — `Renderer{pages,fragments}`; `NewRenderer(root, sharedRoots...)` (:44-85) собирает html/template из `{root}/layouts,partials` + `{sharedRoots}/layouts,partials` + `{root}/pages`; `Page()`(:102-104) рендерит layout `"base"`; `PageLayout()`(:107-114) — произвольный layout (`"bare"` — вход/ошибки); `Fragment()`(:117-127) — AJAX partial. Один Renderer НА ФИЧУ (не на проект) — html/template требует уникальных `{{define}}` внутри набора (render.go:5-11).
- `internal/ssr/render.go:35` — `const SharedRoot = "web/ssr/_layout/templates"`, вне корня статики `web/ssr/_shared`.
- `internal/ssr/render.go:13-19` — doc-комментарий, рецепт подключения фичи от z-scaffold к общему layout (D38, 3 шага): удалить сгенерированный `layouts/base.html`; добавить `Shell ssr.Shell` в VM и заполнять `h.Shell.Build(c,title)`; регистрировать `ssr.NewRenderer("web/ssr/{feature}/templates", ssr.SharedRoot)`.
- `internal/ssr/shell.go:29-38` — `Shell{Title,Menu,ActivePath,User,IsAdmin,CSRFToken}`; `ShellBuilder.Build()`(shell.go:56-68) собирает из Locals+`BuildMenu`. Каждая VM страницы ОБЯЗАНА нести `Shell ssr.Shell`.
- `internal/ssr/menu.go:19-31` — статический сайдбар УЖЕ содержит `{Title:"Клиенты",URL:"/clients",AdminOnly:true}` и `{Title:"Услуги и товары",URL:"/catalog",AdminOnly:true}` (menu.go:25-26) в группе «Справочники» — заготовлены под фазы 3/4, но НЕ в `enabled`-списке `ssr.NewShellBuilder("/", "/users")` (internal/server/router.go:74) — эту фазу нужно добавить `"/clients"` в `enabled` (`BuildMenu`, menu.go:35-50, скрывает не-enabled URL).
- `internal/ssr/status.go:20-37` — `StatusPages{R,Shell}`: `Forbidden()`(403), `NotFound()`(404) — шаблон `web/ssr/_layout/templates/pages/status.html`, `data-testid="status-page"` (уже покрыт E2E `e2e/tests/users.spec.ts:185-191` US-013).

### Wiring новой фичи в `internal/server/router.go` (шаблон)
```go
clientsR, err := ssr.NewRenderer(clients.Root, ssr.SharedRoot)   // образец: router.go:85
clientsSvc := clients.NewService(pool, ...)
clients.Register(app, api, guard, clients.NewHandler(clientsSvc), clients.NewSSR(clientsSvc, clientsR, shell, status))
```
`shell := ssr.NewShellBuilder("/", "/users")` (internal/server/router.go:74) — добавить `"/clients"`.

### Внешние интеграции в pkg/
Нет FCM/Telegram/S3. UUID: `github.com/google/uuid` напрямую в `internal/features/users/service.go:9,94,137` (`uuid.Validate(id)`) — пакета `pkg/ids` в РЕПО НЕТ (проверено `git ls-files`); генерация ID — только в БД `DEFAULT gen_random_uuid()` (migrations/0001_init.up.sql:7). Новые таблицы фазы 3 следуют этому же паттерну, НЕ заводят `pkg/ids`.

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

## Чего НЕТ (потребуется создать)
- Пакет(ы) фичи (`internal/features/clients`, возможно отдельно equipment-types/equipment — решение Design); миграция `0003_*` не существует.
- Optimistic locking (`version`+409) в API-слое — прецедента нет; решить механизм сравнения/инкремента в репозитории (WHERE id AND FOR UPDATE + `version=$N`, явный `VERSION_CONFLICT` 409).
- Динамический список вложенных полей в форме (телефоны 1..N, характеристики N по шаблону типа) — готового паттерна в SSR-стеке нет (прототип решает client-side JS, `prototype/index.html:1352-1361,1413-1421`).
- Sentinel-константы errorspkg: `CLIENT_NOT_FOUND`, `CLIENT_HAS_ORDERS`(409,D26), `EQUIPMENT_TYPE_NOT_FOUND`, `EQUIPMENT_NOT_FOUND`, `EQUIPMENT_IN_ORDERS`(409,D26), `VERSION_CONFLICT`(409,D28).
- Пункт `"/clients"` в `enabled`-списке `ssr.NewShellBuilder(...)` (`internal/server/router.go:74`, сейчас только `"/", "/users"`).
- SSR-шаблоны `web/ssr/clients/templates/` (список+карточка/форма) + экран «Типы оборудования» — открытый вопрос для Design: отдельный URL в фазе 3 или вкладка ещё не построенного `/catalog` (целиком фаза 4, но D24 явно переносит ТОЛЬКО типы оборудования в фазу 3, docs/plan.md:148).
- Решение по вложенным модалкам прототипа (`openEquipment` вызывается из `openClient`, `prototype/index.html:1362-1370`) — в проекте нет паттерна вложенных Bootstrap-модалок; альтернатива — полноценные страницы (как `/users/:id`).
- Тесты: `clients_http_live_test.go` (или по сущностям), E2E specs + POM (`ClientsPage.ts`, аналог для типов оборудования).

## Зависимости (что затронет изменение)
- `internal/server/router.go:74,85,94-98` — регистрация фичи, `ShellBuilder.enabled`.
- `internal/ssr/menu.go:25-26` — пункты `/clients`/`/catalog` уже заданы статически.
- `migrations/` — новая пара `0003_*.up.sql`/`.down.sql` (или несколько — решение Planner).
- `go.mod` — новых зависимостей вероятно не требуется (JSONB/`TEXT[]` — стандартные pgx/v5-типы).

## Граничные случаи из кода
- **D26 «клиент с заказами→409» / «оборудование было в заказах→409 при DELETE» — НЕЧЕМ ПРОВЕРИТЬ в этой фазе**: `tasks`/`task_equipment` — фаза 5/6 (docs/plan.md:115-116), в репозитории их нет (не в `git ls-files`, не в `migrations/`). Варианты для Design: (а) DELETE безусловно разрешён в фазе 3 (проверка — заглушка до фазы 5/6); (б) отложить DELETE до появления `tasks`. Выбор — за Design, не решается на research.
- Регистронезависимость логина (`lower(login)`, уникальный индекс, migrations/0002_users_roles.up.sql:10) — аналогичного требования к company_name/model в docs/plan.md НЕТ; уникальность company_name не требуется (в отличие от D16 для categories).
- `FOR UPDATE`-гонка (`internal/features/users/repo.go:129-166`) блокирует ВСЕ активные admin-строки ради инварианта "хотя бы один admin" — у client/equipment такого глобального инварианта нет, нужна только блокировка своей строки + сравнение `version`.
- Мусор в query-параметрах списка (limit/offset/sort/role) — тихий откат к дефолту, не 500 (`internal/features/users/users_http_live_test.go:148-154`) — обязателен тот же паттерн для новых списков.
- SQL-инъекция через sort/order предотвращена whitelist-мапой, не интерполяцией.

## PostgreSQL 18 специфика
- `pgcrypto` уже создан (migrations/0001_init.up.sql:4) — `gen_random_uuid()` доступен без повторной миграции.
- `TEXT[]` (`equipment_types.spec_keys`) и `JSONB` (`equipment.specs`) — стандартные pgx/v5-типы; примеров использования массивов/JSONB в репозитории пока НЕТ (`users` не использует ни то, ни другое) — новый паттерн для фичи.
- `version bigint NOT NULL DEFAULT 1` — колонка есть в `users` (migrations/0001_init.up.sql:12), но паттерн ПРОВЕРКИ версии (409 при несовпадении) в коде отсутствует — для `clients` это первое реальное использование по назначению.
