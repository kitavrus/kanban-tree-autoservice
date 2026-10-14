---
stage: 1-research
feature: phase-5-board
tier: M
---
# Research: Фаза 5 — Доска заказов

## Triage
```yaml
scope: cross-feature
tier: M
touches:
  db: true
  fe: true
  infra: false
  external: false
risk: reversible
novelty: standard-crud
decisions: []
```
Обоснование: новая область (columns/tasks/task_stock/stock_movements) с транзакционным списанием (`SELECT … FOR UPDATE`) + SSR/JS доска с drag&drop — выше single-endpoint; без внешних интеграций/инфры. `docs/plan.md:115` фиксирует трек `/pipeline (M)`.

## Module
- Модуль `github.com/kitavrus/kanban-tree`(`go.mod:1`), `go 1.26.0`(`:3`).
- Зависимости (`go.mod:9-24`): fiber/v3, golang-migrate/v4, google/uuid, pgx/v5, dockertest/v3, client_golang, go-redis/v9, testify, fasthttp, otel-стек, x/crypto.
- Санитайзер `bluemonday` (`docs/plan.md:76`) в `go.mod` ОТСУТСТВУЕТ — нужен при `description_html` (см. §DB Schema, спорно для D25).

## Что уже есть в коде

### Backend
Фичи `internal/features/`: `auth`, `catalog`, `clients`, `equipmenttypes`, `home`, `users`. Пакета для доски/заказов/остатков НЕТ.

- `internal/features/home/handler.go:13,16-18,20-27,29-32` — `Root="web/ssr/home/templates"`, `PageVM{Shell}`, `Handlers{R,Shell}`, `Register`→`app.Get("/",guard.RequireSSR(),h.index)`, `index` уже рендерит заголовок **"Доска заказов"** — заглушка, подлежит замене.
- `web/ssr/home/templates/pages/index.html:2-4` — текст-заглушка `data-testid="home-placeholder"`.
- Паттерн плоской фичи (эталон `clients`, D43 `docs/plan.md:167`): один пакет, файлы `dto.go`, `handler.go`, `repo.go`, `routes.go`, `service.go`, `ssr.go`, `validate.go`, без вложенных каталогов.
- Паттерн многосущностной фичи (`internal/features/catalog/catalog.go:1`): файлы-префиксы сущности в одном пакете (`categories_*.go`,`products_*.go`,`services_*.go`)+общий `amount.go` — годится для `board` (`columns_*`/`tasks_*`/`stock_*`).
- `internal/features/clients/routes.go:14-32` — сигнатура `Register(app,api,a,h,s)`; `admin:=middleware.RequireAdmin(nil)` (`:15`) применяется per-route (`:18,20,21,27,28`), не групповым guard — паттерн D19; `:23` комментарий `// D19: без admin-guard` на `POST /:id/equipment` — эталон «executor создаёт/правит, не удаляет, не меняет is_paid»; `:30-32` при `s==nil` SSR-группа не регистрируется (тесты).
- `internal/server/router.go:35-45` — `New(cfg,logger,pool,rdb)`, `fiber.Config{ErrorHandler:errorspkg.FiberErrorHandler}` (`:37`), порядок middleware `recover→requestid→Trace→Metrics→RequestLogger`.
- `internal/server/router.go:66,70` — статика `/assets/ssr-shared` ДО CSRF; `middleware.CSRF(...)` на ВСЕ маршруты ниже, включая `/api/v1`/`/login`.
- `internal/server/router.go:77` — `ssr.NewShellBuilder("/","/users","/catalog","/clients")` — **`/stock` в enabled ОТСУТСТВУЕТ**.
- `internal/server/router.go:79-107,122` — каждая SSR-фича — свой `ssr.NewRenderer(feature.Root,ssr.SharedRoot)`; `:122` `(&home.Handlers{...}).Register(app,guard)` — точка замены на board.

**Error flow**: `pkg/errorspkg/errors.go:19-24` `AppError{Code,HTTP,Message,Fields}`; `:32-44` конструкторы `BadRequest/.../Validation(fields...)`→400; `:58-76` `FiberErrorHandler`: `*AppError`→JSON, `*fiber.Error`→`HTTP_ERROR`, прочее→500 `INTERNAL` (первопричина только в `slog.Error`, `:71-75`). Паттерн sentinel: `internal/features/clients/service.go:18-21,166-175` `ErrClientNotFound`/`ErrClientVersionConflict` через `errors.Is(err,ErrNotFound/errVersionConflict)`; `internal/features/clients/repo.go:39` `ErrNotFound=pgx.ErrNoRows`. Валидация — ручная в `service.go`/`validate.go`, НЕ handler/go-playground: `internal/features/clients/service.go:126-130,141-152,239-246,283-290` `in.normalize()`→`in.validate()`→`errorspkg.Validation(f...)`.

**NUMERIC-паттерн** (обязателен для `tasks.position`,`subtasks.price`,`subtask_products.qty/price`,`products.stock`): `internal/features/catalog/amount.go:5-9` `type Amount string` (не float64, точность денег); `:14-20` `MarshalJSON` пустая→`"0"`; `:22-33` `UnmarshalJSON` `null`→`""`, строка/объект/массив→ошибка; `:36-41` `String()` для SQL `::numeric`.

**Middleware** (`internal/server/middleware/auth.go`): `:32-40` `Principal{ID,Login,Name,Role,IsActive}`+`IsAdmin()`; `:30-31,55-80` `resolve(c)` читает БД на каждый запрос — роль/блокировка без перелогина (D42); `:93-104` `RequireAPI()`→401; `:107-118` `RequireSSR()`→редирект `/login`; `:122-132` `RequireAdmin(forbidden)` функция, не метод; `:135-138` `CurrentPrincipal(c)` — источник «мои» (D19). `internal/server/middleware/csrf.go:1` — `kt_csrf`↔`_csrf`/`X-Csrf-Token` (D40), подключён глобально (`router.go:70`), доска доп. кода не требует. `internal/session/store.go:1` — Redis session store.

**pkg/sqlutil**: `pkg/sqlutil/text.go:1`,`pkg/sqlutil/like.go:1` — `sqlutil.HasInvalidText` используется в `internal/features/clients/service.go:66` для отсечения NUL/невалидного UTF-8 в `q=` — тот же паттерн нужен фильтру доски.

**UUID**: пакета `pkg/ids` НЕТ. Все PK — `uuid DEFAULT gen_random_uuid()` в БД (`migrations/0001_init.up.sql:7`, `migrations/0003_clients_equipment.up.sql:3,14,23,34`, `migrations/0004_catalog.up.sql:6,18,35,46,55`); Go валидирует (`internal/features/clients/service.go:10,112`), не генерирует. Исключение — `tasks.id BIGSERIAL` (D37, `docs/plan.md:47`).

### Frontend (SSR html/template + vanilla JS, D30; `frontend/src` нет)
- Точка входа `/` — `internal/features/home/handler.go:25-27` + `web/ssr/home/templates/pages/index.html:1-6` (заменяется/расширяется доской).
- `web/ssr/_layout/templates/layouts/base.html:6,9,11-12,20-25,28-39,46-59` — единственный layout проекта: `<title>`, тема `data-bs-theme` inline до стилей, vendored Bootstrap+`app.css`, сайдбар `{{ range .Shell.Menu/.Items }}`+`NavActive`, блок пользователя+`POST /logout`, топбар `<h1>`+`{{block "actions"}}` (точка для «+Добавить заказ/колонку»), `{{template "content"}}`, общие скрипты (`bootstrap.bundle.min.js`,`theme.js`,`api.js`,`ui-dialog.js`,`combobox.js`)+`{{block "scripts"}}` для board-JS.
- `internal/ssr/shell.go:29-38,41-53,56-68,73-78,81-91` — `Shell{Title,Menu,ActivePath,User,IsAdmin,CSRFToken}`, `ShellBuilder{enabled}`+`NewShellBuilder(...)`, `Build(c,title)`, `NavActive(url)` (ADR-006: `"/"` точное, иначе точное/префикс), `Initials(name)`.
- `internal/ssr/menu.go:19-31` — сайдбар уже описан целиком: «Работа»→`{Доска,"/"}`(`:21`) и **`{Остатки,"/stock"}`(`:22`)**; «Справочники»(`Клиенты /clients`,`Каталог /catalog`,`AdminOnly:true`,`:25-26`); «Администрирование»(`Сотрудники /users`,`AdminOnly:true`,`:29`).
- `internal/ssr/menu.go:35-50` — `BuildMenu(enabled,isAdmin)` фильтрует `enabled[URL]`+`AdminOnly`, пустые группы отбрасываются. **Вывод**: `/stock` уже в `menu.go:22`, но не в `enabled` (`router.go:77`) → скрыт.
- `internal/ssr/menu.go:4-9` — поля для бейджа-счётчика в `NavItem` НЕТ — расширение структуры либо DOM-инъекция как в прототипе.
- `internal/ssr/render.go:1-21,44-85,102-127` — doc-comment рецепта подключения от `/z-scaffold` за 3 шага (D38, board — вручную D30); `NewRenderer(root,sharedRoots...)` собирает `layouts/partials` из root+shared, `pages` только из root; `Page`→layout `"base"`, `PageLayout`→`"bare"` (логин/ошибки), `Fragment`→AJAX partial.
- Общий JS `web/ssr/_shared/js/`: `api.js:1`, `combobox.js:1` (поиск в `select.form-select`, D6), `row-list.js:1-12` (динамические строки 1..N, для формы фазы 6, не нужен простой форме D25), `theme.js:1`, `ui-dialog.js:1` (замена `alert/confirm/prompt`, testid `ui-dialog*`).
- `e2e/pages/ShellPage.ts:5-75` — POM layout: `sidebar/title/themeToggle/menuToggle`(testid), `dialog*`(`ui-dialog*`), `dropdown`(`combobox-dropdown`), `navLink`, `openDialog` через `window.KT.uiConfirm/uiPrompt/uiAlert`, `hasHorizontalOverflow()` (`:72-74`, US-1 `adaptive:min`).
- `web/ssr/_shared/vendor/bootstrap/bootstrap.min.css:1` — vendored Bootstrap 5.3 (D38, `docs/plan.md:162`); вид прототипа — токены `--bs-*`+`app.css` (primary `#2563EB`).

### Прототип — эталон поведения (`prototype/index.html`)
Демо-данные: `:530-535` `state.columns` (Новые/В работе/Ждёт запчасти/Готово с color/final/consume — сид D32), `:347` `COLORS` палитра(8), `:507-519` 11 демо-товаров, `:520-528` 8 услуг с `goods:[{productId,qty}]`, `:608-628` 5 демо-заказов (`newSubtask` `:573-581` копирует `svc.price`+`goods` — паттерн `POST /tasks/:id/subtasks`), `:675-683` демо принятого оборудования (аналог `task_equipment`).

Ключевая механика (правило→строка, все `prototype/index.html`):
- `:599-605` `finalBlock` блокирует перенос в финальную при незавершённых услугах (текст их перечисляет) — эквивалент 409 `SUBTASKS_NOT_DONE`(D11/D22). `:590-595` `dueState`: `overdue/soon/ok`, финальная не подсвечивает просрочку.
- `:636-640` `taskDemand` суммирует `qty` по `productId` из подзадач; `:635` `consumes(t)`=`columnById(t.columnId)?.consume`.
- `:648-662` `stockSync(t,removed)`: цель=`taskDemand(t)` если `consumes`и не`removed`, иначе `{}`; дельта=`target-prev(t.writtenOff)`→`p.stock` — ПОЛНАЯ замена состояния, не инкремент, сервер обязан повторить идемпотентность. `:664-670` `stockAfter` вызывает `stockSync` по задачам, собирает пересечения минимума→`uiDialog`.
- `:796-825` `renderBoardView` — панель фильтров (клиент/поиск/категория/оплата/исполнитель/срок). `:778-794` `taskMatches`: `client`,`paid`,`category`(услуги),`assignee`(ответственный ИЛИ исполнитель услуги),`due`,`q`(полнотекст).
- `:828-858` `cardHTML` — title/клиент+оплата/чипы услуг(3+"+N")/оборудование/`#id`/прогресс/просрочка/фото-видео/`lastLog`(`:827`)/футер срок-сумма-аватар.
- `:861-906` `renderBoard` — колонки по `state.columns`; drag&drop `dragstart`→`taskId`,`dragover`→placeholder,`drop`→`moveTask(dragId,colId,beforeId)`.
- `:908-920` `moveTask` — `finalBlock` блокирует смену колонки (toast-ошибка); успех→toast+`stockAfter([t],...)` (локальный перенос→пересчёт склада; сервер — одна транзакция `POST /tasks/:id/move`).
- `:930-962` `openColumn` — модалка(name/color/final/consume); «Удалить» задизейблена при заказах в колонке(`:933`, аналог 409 D23); смена `consume`→`stockAfter` по заказам колонки(`:954-958`, D23/D34 — одна транзакция на сервере).
- `:1635-1669` `renderStock` — `pending`=`taskDemand` по `!consumes&&!isFinal`(`:1637-1638`); `fc=stock-pending`(`:1639`); `risky=isLow||fc<=minStock`(`:1640`); KPI(`:1648-1651`); сортировка `(stock-minStock)`(`:1642`); клик(admin)→`openProduct`(`:1668`).
- `:642-647` `refreshLowBadge` — DOM-инъекция счётчика у «Остатки» (не поле меню). `:702-746,705` `render` — non-admin видит только `board`/`stock`, частично воспроизведено `AdminOnly` в `internal/ssr/menu.go:24-30`.

## DB Schema (baseline)
Таблицы `columns/tasks/subtasks/subtask_products/task_stock` из `docs/plan.md:46-57` — ЕЩЁ НЕ в миграциях (создаются этой фазой). `clients/equipment/products/services/service_products/stock_movements` — уже существуют, ниже сводка.

- `users` (`migrations/0001_init.up.sql:6-15`): `id uuid PK`,`login UNIQUE`,`password_hash`,`name`,`is_active`,`version bigint default 1`,`created_at/updated_at`; + `role CHECK IN(admin,executor)`,`phone` (`migrations/0002_users_roles.up.sql:4-5`), уникальный индекс `lower(login)` (`:10`).
- `clients` (`migrations/0003_clients_equipment.up.sql:2-11`): `id uuid PK`,`company_name CHECK btrim<>''`,`address/contact_name/contact_position`,`version bigint`,`created_at/updated_at`.
- `client_phones` (`:13-20`): `id`,`client_id→clients CASCADE`,`position smallint`,`phone CHECK~'[0-9]'`,`note`, `UNIQUE(client_id,position)`(`:19`).
- `equipment_types` (`:22-29`): `id`,`name CHECK btrim<>''`,`spec_keys text[]`,`is_active`; уникальный индекс `lower(btrim(name))` (`:31`).
- `equipment` (`:33-45`): `id`,`client_id→clients CASCADE`,`type_id→equipment_types RESTRICT`,`brand/model(CHECK btrim<>'')/sku/serial`,`specs jsonb CHECK jsonb_typeof=array`,`note`; индексы `:46-47`.
- `catalog_categories` (`migrations/0004_catalog.up.sql:5-13`): `id`,`kind CHECK IN(service,product)`,`name CHECK btrim<>''`,`position int`,`is_active`; уникальный индекс `(kind,lower(btrim(name)))` (`:15`).
- `products` (`:17-31`): `id`,`category_id→catalog_categories RESTRICT`,`name CHECK btrim<>''`,`sku/unit`,`price numeric(12,2) CHECK>=0`,`is_active`,`stock numeric(12,3)`,`min_stock numeric(12,3) CHECK>=0`,`box/place`; индекс `:32`.
- `services` (`:34-43`): `id`,`category_id→catalog_categories RESTRICT`,`name CHECK btrim<>''`,`price numeric(12,2) NULL CHECK(NULL OR>=0)`(NULL="по факту"),`is_active`; индекс `:43`.
- `service_products` (`:45-52`): `service_id→services CASCADE`+`product_id→products RESTRICT` composite PK, `qty numeric(12,3) CHECK>0`,`position int`; индекс на `product_id`(`:52`).
- `stock_movements` (`:54-63`): `id`,`product_id→products RESTRICT`,**`task_id uuid NULL` БЕЗ FK** (комментарий `:57` «FK появится в фазе 5»),`qty_delta numeric(12,3) CHECK<>0`,`reason CHECK IN(task_consume,task_return,manual)`,`user_id→users RESTRICT`; индекс `:63`.

**Расхождение plan vs факт**: `docs/plan.md:47` — `tasks(id BIGSERIAL...)` (D37, сквозной номер), но `stock_movements.task_id` (`migrations/0004_catalog.up.sql:57`) уже `uuid`. Если `tasks.id` станет `bigserial`, `stock_movements.task_id` нужно альтерить типом (данных с `task_id` в проде пока нет — все существующие записи `reason=manual`, `task_id IS NULL`, D44). Design обязан явно зафиксировать тип PK `tasks` до генерации миграции; в любом случае FK `stock_movements.task_id → tasks(id)` нужно добавить новой миграцией.

**Модель фазы 5** (дословно `docs/plan.md:46-57`, ещё не в БД): `columns(id,name,color,is_final,consumes_stock,position)`; `tasks(id BIGSERIAL,column_id,client_id NOT NULL,title,description_html,assignee_id→users NULL,due_date NULL,is_paid,paid_at NULL,position NUMERIC,created_by,created_at,updated_at,version)`; `subtasks(id,task_id,service_id→services,status ENUM(new,in_progress,done),assignee_id NULL,due_date NULL,price NUMERIC(12,2) NULL,equipment_id→equipment NULL,position)`; `subtask_products(id,subtask_id,product_id,qty NUMERIC(12,3),price NUMERIC(12,2))`; `task_stock(task_id,product_id,qty NUMERIC(12,3))`; `task_equipment(task_id,equipment_id,completeness,condition)` (`docs/plan.md:35`, формально фаза 6, но частично нужна фазе 5 для приёмки).

**Открытый вопрос (не решается в research)**: D25/декомпозиция относят `subtasks`/`subtask_products` к фазе 6, но ТЗ требует E2E drag&drop «заказы с услугами — из сида» + проверку D11/D22 (финальная колонка блокируется незавершёнными услугами) уже в фазе 5 — это подразумевает, что миграция `subtasks`/`subtask_products` физически нужна уже здесь. Решение — за design/spec-interview.

## Contracts
- Публичные маршруты: `/health`, `/ready`, `/metrics` (`internal/server/router.go:47-61`), `/assets/ssr-shared/*`(`:66`), `/login` SSR, `POST /api/v1/auth/login`.
- Защищённые (`RequireAPI`/`RequireSSR`): весь остальной `/api/v1/*` и SSR.
- `RequireAdmin` — доп. слой per-route (`internal/features/clients/routes.go:15-46`).
- API доски (`docs/plan.md:83,86-95`, ЕЩЁ НЕ реализовано): `GET /api/v1/board?client_id=&q=&category_id=&is_paid=&assignee_id=&due=`; `POST/PUT/DELETE /api/v1/columns`+`.../:id/move`; `.../tasks/:id`(CRUD)+`.../move{column_id,before_id,after_id}`; `.../tasks/:id/subtasks`+`.../subtasks/:id`(фаза 6, см. открытый вопрос); `GET .../stock`, `PUT .../products/:id{stock}`(admin); `PUT .../tasks/:id/payment{is_paid}`(admin); `GET /tasks/:id/print` (SSR, фаза 6).
- URL/JSON — CLAUDE.md §API-1 (kebab URL, snake_case JSON, списки `{items,total,limit,offset}`), уже применяется в `catalog`/`clients` (пример: `e2e/fixtures/auth.ts:78` `Category{item_count}`).
- Enum CHECK в БД: `users.role IN(admin,executor)` (`migrations/0002_users_roles.up.sql:4`), `catalog_categories.kind IN(service,product)` (`migrations/0004_catalog.up.sql:7`), `stock_movements.reason IN(...)` (`:59`); будущий `subtasks.status` — CHECK не создан (таблицы нет). US-1=`swagger:no` (`docs/plan.md:11`) делает `enums:` swag-тег неприменимым в этой фазе.
- Error responses (`pkg/errorspkg/errors.go:40-44,74-76`): 400 `VALIDATION`+`details.fields[]`; 401 `UNAUTHORIZED`(`internal/server/middleware/auth.go:100`); 403 `FORBIDDEN`(`:130`); 404/409 — коды по образцу `CLIENT_NOT_FOUND`/`CLIENT_VERSION_CONFLICT` (`internal/features/clients/service.go:18-21`); будущие: `SUBTASKS_NOT_DONE`(D22),`COLUMN_HAS_TASKS`(D23),`CLIENT_HAS_TASKS`/`EQUIPMENT_IN_TASK`(D26); 500 `INTERNAL`.

## Infrastructure
`touches.infra=false` — фаза не меняет инфру; ниже факты по прямому запросу оркестратора.
- `Makefile:6-99` (project-bootstrap блок): `fresh-start`,`local-up`/`local-up-seed`/`local-down`/`local-reset`,`build`,`run`,`test`,`test-integration`,`test-integration-scoped`(скоуп `~/.claude/tools/go_affected_pkgs.py`),`lint*`,`swagger`(не исп. — US-1 `swagger:no`),`migrate-up`/`migrate-down`.
- `Makefile:107-118` (доп. блок kanban-tree): `e2e-install`,`e2e`,`e2e-full`(`--retries=1`).
- **`make seed`/`make seed-demo` (D36, `docs/plan.md:160`) ОТСУТСТВУЮТ.** Есть `local-up-seed` (`Makefile:20-22`, `SEED=1`→`scripts/dev/up.sh`)→`cmd/seed/main.go:19-47` — вставляет ТОЛЬКО админа (`:14-17,38-41`, `ON CONFLICT DO NOTHING`). Фаза 5 добавляет цель `seed`+новую `seed-demo`.
- `cmd/server/main.go:1`, `cmd/migrate/main.go:1` (`Makefile:93,97`) — точки входа приложения/миграций.
- CI/CD-файлов (`.gitlab-ci.yml`/`.github/workflows/*`) не обнаружено — отсутствует.

## Tests
- `pkg/testutil/base_suite.go:1,22-32,74-76` — билд-тег `integration`; `BaseSuite{Pool,DSN}`, dockertest `postgres:18`+`redis:8`, миграции `golang-migrate` из `file://<moduleRoot>/migrations`.
- `pkg/testutil/live_suite.go:19-31,56-65,72-85,87-103` — `LiveSuite` расширяет `BaseSuite`: `go build`, subprocess с `DATABASE_URL/REDIS_ADDR/PORT`, `http.Client`+cookie jar, ждёт `/health` 200 (30s). Комментарий `:19` «auth-стабы запрещены» (§TD-8).
- `pkg/testutil/login.go:1` — `defaultExtraEnv()` (исп. `live_suite.go:79`).
- Unit без билд-тега (эталон): `internal/features/clients/validate_test.go:1`, `internal/features/catalog/categories_validate_test.go:1` — тестируют `validate()`/`normalize()` без БД.
- Live-сьюты (эталон): `internal/features/clients/clients_http_live_test.go:1`, `internal/features/catalog/catalog_http_live_test.go:1`, `internal/server/shell_http_live_test.go:1`, `internal/session/store_integration_test.go:1`.
- `e2e/playwright.config.ts:1` — прогон `Makefile:112` (`--project=chromium --headed --workers=1`).
- `e2e/pages/ShellPage.ts:1` (+`CatalogPage`,`ClientsPage`,`EquipmentPage`,`EquipmentTypesPage`,`LoginPage`,`UsersPage`) — POM на фичу; board нужен `BoardPage.ts`/`StockPage.ts`.
- `e2e/fixtures/auth.ts:137-144,147-241` — `AdminApi` (create/update-методы по сущности) + `loginContext`; TS-типы дублируют Go DTO (§TD-6.2, напр. `:29,49,74,105`) — board нужны `createColumn`/`createTask`/`moveTask`.
- `e2e/tests/auth.spec.ts:1` (+catalog/clients/equipment*/shell/users) — один spec на фичу; board — `board.spec.ts` (+`stock.spec.ts`), ТЗ требует E2E drag&drop.
- `e2e/pages/ShellPage.ts:72-74` — `hasHorizontalOverflow()` для US-1 `adaptive:min`. Coverage — не настроено.

## Паттерны в коде
- Плоская фича без вложенных каталогов — эталон `clients` (`internal/features/clients/*`, D43 `docs/plan.md:167`); многосущностная — префиксы файлов по сущности в одном пакете (`internal/features/catalog/*`).
- `Register(app,api,a,h,s)`, `s==nil`→SSR не регистрируется (`internal/features/clients/routes.go:14,30-32`); `admin:=middleware.RequireAdmin(nil)` per-route, не групповой (`:15-28`).
- Ошибки — только `pkg/errorspkg`, sentinel-`*AppError` в `var(...)` блоке `service.go` (`internal/features/clients/service.go:17-22`); валидация ручная в `service.go`/`validate.go`: `in.normalize()`→`in.validate()`→`errorspkg.Validation(f...)`.
- NUMERIC-поля — `type Amount string` с кастомным `Marshal/UnmarshalJSON` (`internal/features/catalog/amount.go:9,14-33`), не `float64`.
- Оптимистичная блокировка `version` — `ErrXVersionConflict`+`errors.Is(err,errVersionConflict)`, поле `version *int64` в теле, не If-Match (§FE-1.1) (`internal/features/clients/service.go:141-164`).
- `uuid.Validate(id)` до любого SQL-запроса (`internal/features/clients/service.go:112,142,167,193,222,240,284,323`); все id-PK — `uuid DEFAULT gen_random_uuid()` в БД, кроме `tasks.id BIGSERIAL`(D37) — Go не генерирует UUID.
- `pool.QueryRow/Exec` напрямую из `*pgxpool.Pool`, без ORM (`internal/features/clients/repo.go:1`).
- SSR — один `ssr.Renderer` на фичу, единственный layout — `web/ssr/_layout/templates/layouts/base.html`; сайдбар растёт через `enabled`-список `ssr.NewShellBuilder(...)` (`router.go:77`) — путь фичи нужно явно добавить, иначе пункт скрыт даже если уже в `menu.go`.
- Тесты — `BaseSuite`(dockertest)→`LiveSuite`(+реальный бинарь+HTTP), auth только через реальный login (§TD-8); E2E — POM-класс на фичу+`AdminApi` fixture, TS-типы дублируют Go DTO с комментарием источника (§TD-6.2).

## Чего НЕТ (потребуется создать)
- Пакет(ы) `board`/`stock`/`tasks`/`columns` в `internal/features/` — нет.
- Миграции `columns`,`tasks`,`task_stock`; FK `stock_movements.task_id→tasks(id)` (`migrations/0004_catalog.up.sql:57` — сейчас `uuid NULL` без FK); `subtasks`/`subtask_products` — если design решит, что нужны фазе 5 (открытый вопрос §DB Schema).
- `web/ssr/board/templates/`+`web/ssr/stock/templates/` — сейчас только заглушка (`web/ssr/home/templates/pages/index.html:2-4`); board-JS (drag&drop, фильтры, модалки) — общие `api.js`/`ui-dialog.js`/`combobox.js` есть, board-логика нет.
- `/stock` не в `enabled` `internal/server/router.go:77` (пункт уже в `internal/ssr/menu.go:22`); поле бейджа-счётчика в `NavItem` (`:4-9`) нет.
- `bluemonday` (`docs/plan.md:76`) не в `go.mod` — может не понадобиться при D25-минимальной форме без описания, уточнить на design.
- `make seed`/`make seed-demo` (D36) — только `local-up-seed`+`cmd/seed` (только админ, `cmd/seed/main.go:14-17`).
- D26-проверки: `internal/features/clients/repo.go:327-336` `DeleteClient` (каскад БЕЗ проверки заказов), `:473-480` `DeleteEquipment` (безусловное удаление) — добавить в этой фазе (перенесено из фазы 3, `docs/plan.md:176`).
- `AdminApi`-методы колонок/задач в `e2e/fixtures/auth.ts:147`, POM `BoardPage.ts`/`StockPage.ts`, spec `board.spec.ts` — нет. Печать актов (D20) — фаза 6, не эта. CHECK `subtasks.status` — таблицы нет.

## Зависимости (что затронет изменение)
- `internal/server/router.go:77,92-95,122` — enabled-список ShellBuilder, регистрация board вместо/вместе с `home` (`internal/features/home/*`).
- `internal/ssr/menu.go:4-9` — возможное добавление поля Badge в `NavItem`.
- `migrations/0004_catalog.up.sql:57` — ALTER `stock_movements.task_id` (FK, возможно тип), совместимость с существующими `reason=manual` строками (`task_id IS NULL`, D44).
- `internal/features/clients/repo.go:327-336,473-480`, `service.go:166-175,322-331` — D26-проверки использования в `tasks`/`subtasks`/`task_equipment`.
- `e2e/fixtures/auth.ts:147-241`, `Makefile:20-22` (цели `seed`/`seed-demo`, D36), `go.mod:9-24` (вероятно `bluemonday`, зависит от решения design про `description_html`).

## Граничные случаи из кода
- `internal/features/clients/service.go:66-70` — NUL/невалидный UTF-8 в `q=`→молчаливый сброс фильтра, не 500 (эталон для `?q=` доски). `:86-109` `searchDigits` — нормализация телефонного поиска (ведущая 7/8).
- `internal/features/clients/repo.go:60-65` — `errClientGone`/`errTypeGone`: гонка «удалено параллельно между валидацией и вставкой»→sentinel, не 500 — применимо к «колонка удалена, пока двигали карточку».
- `internal/server/middleware/auth.go:71-74` — заблокированный пользователь с живой сессией убивается при следующем запросе, не мгновенно — учитывать в тестах `assignee_id`/`created_by`.
- `prototype/index.html:599-605` — `finalBlock` блокирует «в» финальную, но не проверяет «из» — иная механика (D22: «нельзя вернуть услугу из done»). `:648-662` `stockSync` — ПОЛНОЕ целевое состояние, не инкремент — сервер обязан повторить, иначе повторная правка рассинхронизирует `task_stock`.
- `internal/features/catalog/amount.go:22-33` — `UnmarshalJSON` строго отвергает строку/объект/массив — так же будет вести себя `qty`/`price` задачи/подзадачи.

## PostgreSQL 18 специфика
- `migrations/0001_init.up.sql:1-4` — `pgcrypto` ДО первой таблицы (иначе `gen_random_uuid()` не резолвится); board extension не пересоздаёт.
- `migrations/0004_catalog.up.sql:25-26` — `NUMERIC(12,3)` для дробных количеств (D33) — паттерн для `subtask_products.qty`.
- `migrations/0003_clients_equipment.up.sql:41` — `jsonb CHECK jsonb_typeof(specs)='array'`; `task_equipment.completeness/condition` по плану — `text` (`docs/plan.md:35`).
- `btrim(...)<>''` CHECK для обязательных текстовых полей (`migrations/0003_clients_equipment.up.sql:4,24,38`, `migrations/0004_catalog.up.sql:8,20,37`) — повторить для `tasks.title` (D25).
- Уникальные индексы `lower(btrim(name))` (`migrations/0002_users_roles.up.sql:10`, `migrations/0003_clients_equipment.up.sql:31`, `migrations/0004_catalog.up.sql:15`) — для `columns.name` не упомянуто в `docs/plan.md` §3 — уточнить на design.
