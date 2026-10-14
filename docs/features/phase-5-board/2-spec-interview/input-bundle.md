# Input Bundle: phase-5-board / стадия 2-spec-interview — generated 2026-09-26 09:31

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 1-research/output.md [sections]
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

## Источник: draft-spec.md [full]
MISSING

## Источник: draft-spec-decisions.md [full]
MISSING

## Паспорт (выжимка)
# Passport: phase-5-board (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 0 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
