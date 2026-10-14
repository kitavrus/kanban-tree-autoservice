# Input Bundle: phase-5-board / стадия 3-design — generated 2026-09-26 22:48

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 2-spec-interview/output.md [full]
---
stage: 2-spec-interview
feature: phase-5-board
tier: M
---
# Spec: Доска заказов и «Остатки» (фаза 5)
**Дата интервью:** 2026-09-26
**На основе:** PRD `2-spec-interview/prd.md` (approved) + input-bundle (research-экстракты) + docs/plan.md D1–D44 + `spec-decisions.md` SD-001…SD-008
**Tier:** M
**Совместимость:** MVP (CM-1)
**UI-scope (US-1):** swagger:no | adaptive:min | themes:both

## Применённые правила (zap-list)
- Ошибки — только `pkg/errorspkg`, единый wire-формат; 400 → `details.fields[]` (CLAUDE.md проекта).
- 404 / 403 / 409 — стандартные статусы; auth — Redis-сессия + CSRF каркаса (D35), гарды `middleware.RequireAdmin(nil)` per-route.
- Синхронная обработка, без очередей; аудит-лог не вводится (slog + `trace_id`); журнал склада — `stock_movements` (D34, D44).
- Миграции — только `make migrate-up`, без auto-apply.
- URL kebab-case + `/api/v1`, JSON snake_case, list-эндпоинты — конверт `{items,total,limit,offset}` (CLAUDE.md §API-1).
- NUMERIC — `type Amount string`-паттерн (`internal/features/catalog/amount.go`), не `float64`.
- `version` в теле (`version *int64`), не If-Match (§FE-1.1); только у clients и tasks (D28).
- Tracer — `otel.Tracer("kanban-tree/{component}")` по паттерну каркаса.
- Тесты — `pkg/testutil.LiveSuite` (§TD-8), auth-стабы запрещены; E2E — POM на экран + fixture API.
- Каталоги Go — §GO-1: новая фича `internal/features/board` (или `board`+`stock` — решает design), файлы snake_case.

## Технические ограничения

### Схема (source: SD-001, SD-002, SD-007)
Одна новая миграция `0005_*` создаёт:
- `columns(id uuid, name, color, is_final, consumes_stock, position)` + сид D32 в самой миграции (driver: AC-008).
- `tasks(id BIGSERIAL, column_id→columns RESTRICT, client_id→clients RESTRICT NOT NULL, title, description_html, assignee_id→users NULL, due_date NULL, is_paid, paid_at NULL, position NUMERIC, created_by→users, created_at, updated_at, version)` (source: D37; driver: AC-009).
- `subtasks`, `subtask_products`, `task_stock(task_id, product_id, qty)`, `task_equipment` — полная схема по docs/plan.md; UI правки нет (D25), наполнение только seed-demo (driver: AC-003, AC-004, AC-013, AC-015, AC-018).
- `ALTER stock_movements.task_id TYPE bigint` (данных с `task_id` нет — все строки `manual`) + FK `→ tasks(id) ON DELETE SET NULL` (source: SD-002, SD-007).
- FK `clients`→`tasks` RESTRICT и `equipment`→`task_equipment` RESTRICT — физическая опора D26-проверок; сервис отвечает 409 до ошибки FK (driver: AC-017, AC-018).
- `ON DELETE` для `subtasks/subtask_products/task_stock/task_equipment` от `tasks` — CASCADE (удаление заказа, AC-012).

### Списание (source: D17, D34)
- Потребность заказа = Σ `subtask_products.qty` по product_id. `task_stock` = фактически списанное по заказу.
- Функция пересчёта `reconcile(task)`: целевое = потребность, если колонка `consumes_stock`, иначе 0; дельта = целевое − `task_stock`; для каждой ненулевой дельты: `products.stock -= дельта`, upsert `task_stock`, строка `stock_movements` (`task_consume` при дельте>0 → `qty_delta<0`; `task_return` → `qty_delta>0`) с `task_id`, `user_id`.
- Единая транзакция: `SELECT … FOR UPDATE` строки(строк) `tasks`, затем `products` в порядке id (D34). Вызывается из: move, PUT со сменой колонки, DELETE заказа (целевое=0 до удаления), смена `consumes_stock` колонки (по всем её заказам в одной транзакции, D23).
- Минус допустим; после коммита ответ несёт `warnings` (товары с `stock < 0` и `stock ≤ min_stock`) (driver: AC-005).
- Инвариант метрики PRD §3: `products.stock` = начальный + Σ `stock_movements.qty_delta` — проверяется integration-тестом.

### Перенос (source: SD-006)
- `POST /api/v1/tasks/:id/move` `{column_id, position}` (или соседи — Q-001), без `version`; last-write-wins; инкремент `version`.
- В `is_final`-колонку при наличии `subtasks.status<>'done'` → 409 `SUBTASKS_NOT_DONE` + список (D11, D22). Та же проверка в `PUT` при смене `column_id`. Проверки D22 «вернуть услугу из done / добавить услугу в финальной» — эндпоинтов услуг в фазе 5 нет, реализуются в фазе 6.
- Бюджет: p95 < 500 мс (driver: PRD §3).

### Внешние зависимости и инфра
- Новых сервисов/env нет; Redis — только сессии каркаса. CI без изменений.
- `make seed-demo` → `cmd/seed-demo` (или флаг `cmd/seed`): грузит демо прототипа только при `SELECT count(*) FROM tasks = 0`, иначе сообщение и exit 0; админа не трогает; списания в сиде — через тот же `reconcile`, чтобы журнал и остаток сходились (source: SD-003, D36; driver: AC-019).

### UI (D30, D43)
- Доска `/` и «Остатки» `/stock` — rich-экраны на vanilla JS поверх JSON API; каркас страницы SSR-layout `base.html`; `/stock` добавить в `enabled` ShellBuilder (`router.go`).
- Формы колонки и заказа — модалки Bootstrap по эталону prototype/index.html; клиент — `combobox.js` (D6); подтверждения — `ui-dialog.js`.

## Enum-поля фичи (CLAUDE.md §8.1)
| Поле (DTO.json) | Допустимые значения | Источник правды |
|---|---|---|
| `subtasks.status` (в фильтрах/ответе доски) | `new, in_progress, done` | миграция 0005 CHECK + Go-константы фичи |
| `stock_movements.reason` | `task_consume, task_return, manual` | `migrations/0004_catalog.up.sql` (без изменений) |
| фильтр `paid` | `all, paid, unpaid` | Go-константы фичи (Q-003) |
| фильтр `due` | см. Q-003 | Go-константы фичи |

## Безопасность и доступ
| Действие | admin | executor | Driver |
|---|---|---|---|
| Просмотр доски, фильтры, «Остатки», счётчик | да | да | AC-001, AC-015, AC-016 |
| Создание/правка заказа, move | да | да | AC-009, AC-002 (D19) |
| Изменение `is_paid` | да | 403, если значение отличается от текущего | AC-011 (SD-008) |
| Удаление заказа | да | 403 | AC-012 (D19) |
| CRUD колонок | да | 403, UI скрыт | AC-006 (SD-005) |
| Правка остатка в «Остатках» | да (manual, D44) | 403, UI только просмотр | AC-015 |
- Все write-эндпоинты — CSRF каркаса. Id: `uuid.Validate` для uuid-сущностей; `tasks.id` — парсинг int64 до SQL (невалидный → 404).
- Чувствительных данных нет; 403 vs 404 — ресурс не скрывается (одна мастерская).

## UI/UX требования
- Drag&drop карточки: оптимистичный перенос; при 409/ошибке — откат карточки + сообщение `ui-dialog`/toast со списком незавершённых услуг (AC-003).
- `warnings` из ответа move/PUT/смены колонки → toast с перечнем и ссылкой на `/stock` (AC-005).
- Фильтр «исполнитель» у executor по умолчанию «мои» (ответственный ИЛИ исполнитель услуги), admin — «все» (AC-014); состояние фильтров — в URL query (Q-003).
- 360 px: горизонтальный скролл только у контейнера колонок, `hasHorizontalOverflow()` страницы = false; обе темы через `--bs-*` токены (AC-020).
- Повторный клик «Сохранить» — кнопка блокируется до ответа.

## Обработка ошибок
| Ошибка | Что видит пользователь | Действие системы | Driver |
|---|---|---|---|
| 409 `SUBTASKS_NOT_DONE` (`details.subtasks[]`) | карточка вернулась, список незавершённых услуг | транзакция откатывается | AC-003 |
| 409 `COLUMN_NOT_EMPTY` | «В колонке есть заказы» | колонка не удаляется | AC-006 |
| 409 `LAST_NON_FINAL_COLUMN` | «Нужна хотя бы одна нефинальная колонка» | удаление/смена `is_final` отклонены | AC-006 |
| 409 `TASK_VERSION_CONFLICT` | «Заказ уже изменён, обновите» | правка не применяется | AC-010 |
| 409 `CLIENT_HAS_TASKS` | «У клиента есть заказы» | клиент не удаляется | AC-017 |
| 409 `EQUIPMENT_USED_IN_TASKS` | «Оборудование было в заказе» | не удаляется | AC-018 |
| 403 | «Недостаточно прав» | — | AC-006, AC-011, AC-012 |
| 404 заказ/колонка/клиент/ответственный | «Не найдено» (в форме — ошибка поля) | — | AC-009 |
| 400 валидация (`title`, `client_id` обязательны; `color` hex; `name` не пустой) | ошибки полей `details.fields[]` | — | AC-009, AC-006 |
| 200 + `warnings[]` (минус / ≤ минимума) | toast-предупреждение | списание выполнено | AC-005 |
- Всё-или-ничего: каждая операция склада — одна транзакция; retry ручной (повтор действия пользователем).

## Операции и наблюдаемость (M)
### Spans
| Имя | Tracer | Log fields | Когда |
|---|---|---|---|
| `tasks.move` | `kanban-tree/board` | `trace_id`, `task_id`, `from_column`, `to_column`, `movements_count` | каждый move |
| `stock.reconcile` | `kanban-tree/board` | `trace_id`, `task_id`, `products_count`, `negative_count` | каждый пересчёт |
| `columns.recalc` | `kanban-tree/board` | `trace_id`, `column_id`, `tasks_count` | смена `consumes_stock` |

### Метрики и алерты
| Метрика | Порог алерта | Действие on-call |
|---|---|---|
| `http_request_duration_seconds{route="/api/v1/tasks/:id/move"}` p95 | > 0.5 с за 10 мин (driver: PRD §3) | Jaeger по `trace_id`, проверить блокировки `FOR UPDATE` |
| `stock_reconcile_errors_total` | > 0 за 5 мин | логи `stock.reconcile`, сверить остаток vs журнал |
- Прод-алертинг проекта ещё не развёрнут (`/infra-bootstrap` не проводился): правила фиксируются в спеке, подключение — при `/infra-bootstrap` (deferred, трекинг — docs/plan.md, инфра-фаза).
- Runbook читает Dev (владелец). После деплоя: `make seed-demo` локально, smoke drag&drop «Согласование → В работе → Готово», сверка остатка с журналом.

## Принятые компромиссы
- Услуги/товары заказа в фазе 5 только из сида (SD-001) — редактирование и D22-запреты на уровне услуг в фазе 6.
- move без `version` (SD-006) — одновременный перенос одного заказа: побеждает последний.
- Физическое удаление заказа, журнал теряет связь с заказом (SD-007).
- Номер `#id` bigserial не переиспользуется, пропуски после удаления допустимы (D37).
- Счётчик «Остатков» считается на каждый рендер layout (объём каталога мастерской мал) — Q-004.

## Отклонённые решения
- uuid PK + отдельный номер у `tasks` — против D37 (SD-002).
- Перенос схемы услуг в фазу 6 — ломает D11/D17/E2E фазы 5 (SD-001).
- Мягкое удаление заказов — фильтрация во всех выборках ради одной связи в журнале (SD-007).
- `version` на move — частые отказы drag&drop при устаревшей доске (SD-006).
- drag&drop колонок — порядок полем «позиция» (SD-005).
- Блок «услуги» в простой форме — расширяет D25 (SD-001).

## Открытые вопросы

### Q-001: Алгоритм дробной позиции карточки
Контракт move: клиент присылает `position` или id соседей (`before_id/after_id`), сервер считает середину. Нужны правило вставки в пустую колонку/край и ребалансировка при исчерпании точности NUMERIC. Design фиксирует контракт и правило.

### Q-002: Формула прогноза «Остатков» и KPI
«Ещё не взятые в работу заказы» = заказы в колонках без `consumes_stock` и не `is_final`; прогноз = `stock − Σ потребности таких заказов`. KPI: на минимуме (`0 < stock ≤ min_stock`), нет в наличии (`stock ≤ 0`), уйдут в минимум (прогноз ≤ `min_stock` при текущем > `min_stock`). Рисковые = объединение трёх множеств; счётчик меню = размер этого множества. Design подтверждает по прототипу и фиксирует SQL.

### Q-003: Контракт фильтров доски
Параметры query (`client_id`, `q`, `category_id` — «вид работ» по категории услуг subtasks, `paid`, `assignee` = `me|all|<user_id>`, `due` — набор значений по прототипу: просрочен/сегодня/неделя/без срока); хранение в URL; загрузка доски — один list-запрос со всеми карточками (limit по умолчанию достаточно большой) vs по колонкам. Design фиксирует с опорой на prototype/index.html.

### Q-004: Доставка счётчика «Остатков» в меню
Расширение `ssr.NavItem` полем бейджа + вычисление в `ShellBuilder.Build` (один SQL на рендер) vs DOM-инъекция JS после `GET /api/v1/stock/summary`. Design выбирает и описывает инвалидацию после move/правки остатка.

### Q-005: Размещение D26-проверок фазы 3
409 `CLIENT_HAS_TASKS` / `EQUIPMENT_USED_IN_TASKS` — в сервисах `clients` (зависимость от таблиц `tasks`/`task_equipment` через SQL EXISTS) без импорта пакета доски. Design подтверждает границу пакетов и маппинг ошибки FK → 409 как страховку.

### Q-006: Формат `warnings` в ответах
Единая структура `warnings: [{product_id, name, stock, min_stock, kind: negative|below_min}]` в ответах move, PUT задачи, PUT колонки, DELETE задачи. Design фиксирует DTO и TS-зеркало (§TD-6.2).

## Источник: AC-строки (2-spec-interview/prd.md) [ac]
#### AC-001: Доска с колонками и карточками
#### AC-002: Статус заказа — это колонка
#### AC-003: Финальная колонка только для завершённых заказов
#### AC-004: Списание товаров при запуске в работу
#### AC-005: Минус на складе не блокирует, но предупреждает
#### AC-006: Управление колонками
#### AC-007: Смена «списывать товары» у колонки
#### AC-008: Колонки по умолчанию
#### AC-009: Создание и правка заказа простой формой
#### AC-010: Одновременная правка заказа
#### AC-011: Оплата меняется только администратором
#### AC-012: Удаление заказа
#### AC-013: Фильтры доски
#### AC-014: «Мои» по умолчанию для исполнителя
#### AC-015: Экран «Остатки»
#### AC-016: Счётчик рисковых товаров в меню
#### AC-017: Нельзя удалить клиента с заказами
#### AC-018: Нельзя удалить оборудование, бывшее в заказе
#### AC-019: Демо-данные одной командой
#### AC-020: Работа на телефоне и в обеих темах

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

## Паспорт (выжимка)
# Passport: phase-5-board (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 20 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
