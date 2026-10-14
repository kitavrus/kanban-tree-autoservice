# Phase Bundle: phase-5-board / Phase 07 (frontend) — generated 2026-09-27 02:22

> Твой вход — ЭТОТ файл. Upstream-доки (research/prd/design-*) напрямую НЕ читать —
> нужные слайсы дизайна ниже. Код фичи и её тесты — читать/писать можно (твоя фаза).

## Фаза плана (6-plan/code-plan-phase-07-ui-stock.md)
# Phase 07: UI «Остатки» + E2E

**Layer:** frontend
**Commit**: `feat(phase-5): UI «Остатки» — KPI, таблица, правка остатка + E2E`

> Vertical-capability фаза (Stock API готов — фаза 04) — E2E прогоняется сразу.

## Files to CREATE
- `web/ssr/_shared/js/stock.js` — `KT.stock`: `summary` (KPI-плитки) + `list` (таблица, переключатель `risky`/`all`), модалка правки остатка (admin, `PUT /api/v1/stock/:product_id`) → после успеха `KT.refreshNavBadge()`
- `e2e/pages/StockPage.ts` — POM: локаторы `stock-kpi-low|out|will-low`, `stock-row-{productId}`, `stock-edit-{productId}`, `stock-scope`
- `e2e/tests/stock.spec.ts` — `BD-015` (KPI, таблица рисковых с прогнозом; admin правит остаток; executor — без кнопки правки), `BD-016` (бейдж «Остатков» виден executor и admin; после move с уходом в минимум — +1 без перезагрузки)

## Files to MODIFY
- `web/ssr/board/templates/pages/stock.html` — заполнить стаб (фаза 04): KPI-плитки, переключатель `risky`/`all`, таблица, модалка правки остатка (admin), `{{block "scripts"}}` → `stock.js`
- `web/ssr/_shared/app.css` — стили KPI-плиток/таблицы «Остатков» (только `--bs-*` токены)

## Run after
```bash
npx playwright test e2e/tests/stock.spec.ts --project=chromium --headed --workers=1
```

## Tests in this phase
- E2E (Playwright, реальный прогон): `BD-015`, `BD-016`

## Definition of Done
- [ ] `make lint` — 0 ошибок
- [ ] **Playwright E2E «Остатков» зелёный** — happy (BD-015 KPI/таблица/правка) + authz (BD-015 executor без кнопки правки) + live-update (BD-016 бейдж +1 без перезагрузки после move в общем E2E-стенде) — прогон `npx playwright test e2e/tests/stock.spec.ts --project=chromium --headed --workers=1`
- [ ] Тесты этой фазы зелёные до перехода (TD-1)
- [ ] testid-контракт (`stock-kpi-*`, `stock-row-{id}`, `stock-edit-{id}`, `stock-scope`) соблюдён дословно
- [ ] `KT.refreshNavBadge()` вызывается после `PUT /stock/:product_id` (ADR-004)
- [ ] console чистая, ожидаемые статусы через `allowHttpErrors` (TD-4)
- [ ] AC-015, AC-016 (UI) закрыты


## Слайс: 3-design/design-frontend-architecture.md [full]
MISSING

## Слайс: 3-design/design-go-layers.md [sections]
## Enum SubtaskStatus
`new`, `in_progress`, `done` — источник: `board/enums.go::SubtaskStatuses` = CHECK `subtasks.status` (0005); sync-тест `enums_test.go` (сверка с `pg_get_constraintdef` в live-сьюте).

## Enum PaidFilter
`all` (дефолт), `paid`, `unpaid` — `board/enums.go::PaidFilters`.

## Enum DueFilter
`all` (дефолт), `soon` (просрочено ИЛИ срок сегодня/завтра ИЛИ есть просроченная услуга), `overdue` (просрочено ИЛИ просроченная услуга); финальные колонки не входят — `board/enums.go::DueFilters` (ADR-003, по `prototype/index.html:589-596,784-785`).

## Enum AssigneeFilter
`me`, `all`, `<uuid пользователя>`; отсутствует → по роли: executor=`me`, admin=`all` (AC-014) — `board/enums.go::AssigneeMe/AssigneeAll`.

## Enum StockScope
`risky` (дефолт), `all` — `board/enums.go::StockScopes`.

## Enum StockState
`out`, `low`, `will_low`, `ok` — вычисляется SQL (design-sql §Stock Pending CTE); `board/enums.go::StockStates`.

## Enum WarningKind
`negative`, `below_min` — `board/enums.go::WarningKinds`.

## Enum StockMovementReason
`task_consume`, `task_return`, `manual` — as-is CHECK `0004_catalog.up.sql:59`; board пишет `task_consume`/`task_return`.

## DTO Column
```go
type ColumnResponse struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Color         string `json:"color"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
    Position      int    `json:"position"`
    TasksCount    int    `json:"tasks_count"`
}
type ColumnListResponse struct { Items []ColumnResponse `json:"items"`; Total, Limit, Offset int /* json total/limit/offset */ }
// Порядок: position, id. Дефолты: limit=100 (1..1000), offset=0.
type ColumnInput struct {           // POST и PUT /api/v1/columns[/:id]
    Name          string `json:"name"`
    Color         string `json:"color"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
    Position      *int   `json:"position"` // POST: nil → в конец; PUT: nil → без перемещения
}
type ColumnMutationResponse struct {
    Column   ColumnResponse `json:"column"`
    Warnings []StockWarning `json:"warnings"` // не nil: [] (TS без null)
}
```

## DTO Task
```go
type TaskCard struct {              // элемент списка доски
    ID            int64   `json:"id"`
    ColumnID      string  `json:"column_id"`
    ClientID      string  `json:"client_id"`
    ClientName    string  `json:"client_name"`
    Title         string  `json:"title"`
    AssigneeID    *string `json:"assignee_id"`
    AssigneeName  *string `json:"assignee_name"`
    DueDate       *string `json:"due_date"`       // YYYY-MM-DD
    IsPaid        bool    `json:"is_paid"`
    Position      string  `json:"position"`       // NUMERIC строкой — клиенту только для отладки/сортировки
    Version       int64   `json:"version"`
    SubtasksTotal int     `json:"subtasks_total"`
    SubtasksDone  int     `json:"subtasks_done"`
    SubtasksLate  int     `json:"subtasks_late"`
}
type TaskListResponse struct { Items []TaskCard `json:"items"`; Total, Limit, Offset int }
// Гарантия порядка items (контракт): columns.position, columns.id, tasks.position, tasks.id —
// т.е. по колонкам слева направо, внутри колонки сверху вниз; стабилен между запросами.
// Дефолты: limit=500 (1..1000), offset=0.
type TaskResponse struct {          // GET /tasks/:id и в mutation-ответах
    TaskCard
    PaidAt    *time.Time `json:"paid_at"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}
type TaskCreateRequest struct {
    Title      string  `json:"title"`
    ClientID   string  `json:"client_id"`
    ColumnID   string  `json:"column_id"`
    AssigneeID *string `json:"assignee_id"`
    DueDate    *string `json:"due_date"`
    IsPaid     *bool   `json:"is_paid"`   // executor: nil|false, иначе 403
}
type TaskUpdateRequest struct {
    TaskCreateRequest
    Version *int64 `json:"version"`       // обязателен (§FE-1.1)
}
type TaskMoveRequest struct {
    ColumnID string `json:"column_id"`
    PrevID   *int64 `json:"prev_id"`       // карточка СВЕРХУ после вставки (ADR-001)
    NextID   *int64 `json:"next_id"`       // карточка СНИЗУ
}
type TaskMutationResponse struct {
    Task     TaskResponse   `json:"task"`
    Warnings []StockWarning `json:"warnings"`
}
type TaskDeleteResponse struct {
    Warnings []StockWarning `json:"warnings"`
}
```
- `description_html` в фазе 5 не входит в DTO (D25, 5 полей формы) — PUT его не перезаписывает.

## DTO TaskFilter
```go
type TaskFilter struct {            // tasks_filter.go: ParseTaskFilter(c fiber.Ctx, a Actor) (TaskFilter, error)
    ClientID   *string  // client_id (uuid; невалидный → 400 field)
    Q          string   // q, trim, ≤100
    TaskNo     *int64   // из q ~ ^#?\d{1,18}$
    CategoryID *string  // category_id (uuid)
    Paid       *bool    // paid: all→nil
    AssigneeID *string  // assignee: me→a.ID, all→nil, uuid
    Due        string   // "" | soon | overdue
    Limit      int      // default 500, 1..1000
    Offset     int      // default 0
}
```

## DTO StockWarning
Q-006 (ADR-006):
```go
type StockWarning struct {
    ProductID string         `json:"product_id"`
    Name      string         `json:"name"`
    Stock     numeric.Amount `json:"stock"`
    MinStock  numeric.Amount `json:"min_stock"`
    Kind      string         `json:"kind"` // negative | below_min
}
```

## DTO Stock
```go
type StockRow struct {
    ProductID string         `json:"product_id"`
    Name      string         `json:"name"`
    Unit      string         `json:"unit"`
    Box       string         `json:"box"`
    Place     string         `json:"place"`
    Stock     numeric.Amount `json:"stock"`
    MinStock  numeric.Amount `json:"min_stock"`
    Pending   numeric.Amount `json:"pending"`    // потребность не взятых в работу заказов
    Forecast  numeric.Amount `json:"forecast"`
    State     string         `json:"state"`      // out|low|will_low|ok
}
type StockListResponse struct { Items []StockRow `json:"items"`; Total, Limit, Offset int }
type StockSummaryResponse struct {
    ProductsTotal int `json:"products_total"`
    LowCount      int `json:"low_count"`
    OutCount      int `json:"out_count"`
    WillLowCount  int `json:"will_low_count"`
    RiskCount     int `json:"risk_count"`
}
type StockAdjustRequest struct { Stock *numeric.Amount `json:"stock"` }   // обязателен
// ответ adjust — StockRow
```

## DTO Assignee
```go
type AssigneeResponse struct { ID string `json:"id"`; Name string `json:"name"`; Role string `json:"role"` }
type AssigneeListResponse struct { Items []AssigneeResponse `json:"items"`; Total, Limit, Offset int }
// Порядок: name, id. Дефолты: limit=200 (1..1000), offset=0; только is_active.
```

## Слайс: 3-design/output.md [sections]
## Контракт
Все под `/api/v1`, сессия + CSRF каркаса; JSON snake_case; списки `{items,total,limit,offset}` (CLAUDE.md §API-1). DTO — design-go-layers.
| Метод | Путь | Auth | Request | Response | Driver |
|---|---|---|---|---|---|
| GET | `/api/v1/columns` | любой | — | `ColumnListResponse` | AC-001 |
| POST | `/api/v1/columns` | admin | `ColumnInput` | 201 `ColumnMutationResponse` | AC-006 |
| PUT | `/api/v1/columns/:id` | admin | `ColumnInput` | 200 `ColumnMutationResponse` | AC-006, AC-007 |
| DELETE | `/api/v1/columns/:id` | admin | — | 204 | AC-006 |
| GET | `/api/v1/tasks` | любой | query `TaskFilter` | `TaskListResponse` | AC-001, AC-013, AC-014 |
| POST | `/api/v1/tasks` | любой (`is_paid=true` — admin, иначе 403 `PAID_ADMIN_ONLY`) | `TaskCreateRequest` | 201 `TaskMutationResponse` | AC-009 |
| GET | `/api/v1/tasks/:id` | любой | — | `TaskResponse` | AC-009 |
| PUT | `/api/v1/tasks/:id` | любой (смена `is_paid` — admin, иначе 403 `PAID_ADMIN_ONLY`) | `TaskUpdateRequest` (+`version`) | `TaskMutationResponse` | AC-009, AC-010, AC-011 |
| POST | `/api/v1/tasks/:id/move` | любой | `TaskMoveRequest` | `TaskMutationResponse` | AC-002…AC-005 |
| DELETE | `/api/v1/tasks/:id` | admin | — | 200 `TaskDeleteResponse` | AC-012 |
| GET | `/api/v1/stock` | любой | `scope,q,limit,offset` | `StockListResponse` | AC-015 |
| GET | `/api/v1/stock/summary` | любой | — | `StockSummaryResponse` | AC-015, AC-016 |
| PUT | `/api/v1/stock/:product_id` | admin | `StockAdjustRequest` | `StockRow` | AC-015 |
| GET | `/api/v1/assignees` | любой | `limit,offset` | `AssigneeListResponse` | AC-009 |
| GET | `/` , `/stock` | SSR любой | — | HTML (каркас; данные — JS) | AC-001, AC-015 |
- Порядок и пагинация списков (контракт): `GET /columns` — `position, id`, limit 100 (1..1000); `GET /tasks` — `columns.position, columns.id, tasks.position, tasks.id` (слева направо, сверху вниз, стабилен), limit 500 (1..1000); `GET /stock` — `stock−min_stock, name, id`, limit 200 (1..1000); `GET /assignees` — `name, id`, limit 200 (1..1000); offset по умолчанию 0.
- DELETE: `/columns/:id` → 204 без тела (склад не затрагивается); `/tasks/:id` → 200 `{warnings}` (возврат товаров, ADR-006) — различие намеренное.
- 403: `FORBIDDEN` — маршрут закрыт ролью; `PAID_ADMIN_ONLY` — запрещено только изменение `is_paid` (design-errors §Codes 01-15).

Изменённые as-is: `DELETE /api/v1/clients/:id`, `DELETE /api/v1/equipment/:id` (+SSR `POST .../delete`) → 409 `CLIENT_HAS_TASKS`/`EQUIPMENT_USED_IN_TASKS`; `DELETE /api/v1/services/:id` → 409 `SERVICE_IN_USE`.

### ADR-003: Контракт фильтров доски и загрузка одним запросом
- **Статус:** Принято
- **Driver:** AC-013 — «Доску можно отфильтровать по клиенту, тексту (номер или название), виду работ, оплате, исполнителю и сроку. Фильтры сочетаются между собой, и их можно сбросить (source: D19).»; AC-014 — «Исполнитель при открытии доски видит только свои заказы: те, где он ответственный или исполнитель хотя бы одной услуги. Фильтр снимается, и тогда видна вся доска. Администратор по умолчанию видит всё (source: D19).»
- **Отвечает на:** Q-003 — «Контракт фильтров доски»
- **Контекст:** прототип фильтрует на клиенте (`prototype/index.html:778-796`); у нас данные из API.
- **Решение:** `GET /api/v1/tasks?client_id&q&category_id&paid=all|paid|unpaid&assignee=me|all|<uuid>&due=all|soon|overdue&limit&offset` — серверная фильтрация, AND-комбинация. `q`: ILIKE по названию и клиенту + точный `#N`/`N`. `due` по прототипу: `soon` = срок ≤ завтра (включая просрочку) ИЛИ просроченная услуга; `overdue` = срок < сегодня ИЛИ просроченная услуга; финальные колонки исключены. `assignee` отсутствует → дефолт по роли на сервере (executor=`me`). Доска = `GET /columns` + ОДИН `GET /tasks` (limit по умолчанию 500, max 1000; при `total>limit` — баннер «показаны первые N»). Состояние — в `location.search` (reload сохраняет, «Сбросить» → дефолты роли).
- **Альтернативы:** загрузка по колонкам (N запросов, сложный DnD между загруженными страницами); клиентская фильтрация всего набора (рост объёма, расхождение с API).
- **Риски:** `current_date` — TZ БД; для одной мастерской совпадает с локальной (MVP).

## Слайс: 3-design/design-errors.md [full]
# Design Errors: phase-5-board

> Owner: коды ошибок, sentinel'ы, маппинг. Проект использует строковые коды `errorspkg.AppError.Code` (не числовые supportMessage) — заголовок `Codes` ниже нумерует строки таблицы 01–15. Wire — `{code, message, details?}` (`pkg/errorspkg/errors.go`).

## errorspkg Extension
ADR-102. Нужен `details.subtasks[]` для `SUBTASKS_NOT_DONE` — текущий `errDetails` несёт только `fields`.
```go
// pkg/errorspkg/errors.go
type AppError struct {
    Code    string         `json:"code"`
    HTTP    int            `json:"-"`
    Message string         `json:"message"`
    Fields  []Field        `json:"-"`
    Extra   map[string]any `json:"-"` // доп. ключи details (snake_case); sentinel не мутируется
}

// WithDetail — КОПИЯ ошибки с доп. ключом details (sentinel'ы — общие var, мутировать нельзя).
func (e *AppError) WithDetail(key string, v any) *AppError

// FiberErrorHandler: details = {"fields": Fields (если есть)} ∪ Extra; details опускается, если оба пусты.
```
- Wire VALIDATION не меняется (`details.fields[]`). Существующие live-тесты фаз 1–4 — регресс-гейт.

## Codes 01-15
| # | Code | HTTP | Sentinel (пакет) | Сообщение | Driver |
|---|---|---|---|---|---|
| 01 | `TASK_NOT_FOUND` | 404 | `ErrTaskNotFound` (board) | «Заказ не найден» | AC-009 |
| 02 | `COLUMN_NOT_FOUND` | 404 | `ErrColumnNotFound` (board) | «Колонка не найдена» | AC-006 |
| 03 | `SUBTASKS_NOT_DONE` | 409 | `ErrSubtasksNotDone` (board) + `details.subtasks[]` | «Есть незавершённые услуги — в финальную колонку нельзя» | AC-003 |
| 04 | `COLUMN_NOT_EMPTY` | 409 | `ErrColumnNotEmpty` (board) | «В колонке есть заказы» | AC-006 |
| 05 | `LAST_NON_FINAL_COLUMN` | 409 | `ErrLastNonFinalColumn` (board) | «Нужна хотя бы одна нефинальная колонка» | AC-006 |
| 06 | `TASK_VERSION_CONFLICT` | 409 | `ErrTaskVersionConflict` (board) | «Заказ уже изменён, обновите» | AC-010 |
| 07 | `CLIENT_HAS_TASKS` | 409 | `ErrClientHasTasks` (clients) | «У клиента есть заказы — удалить нельзя» | AC-017 |
| 08 | `EQUIPMENT_USED_IN_TASKS` | 409 | `ErrEquipmentUsedInTasks` (clients) | «Оборудование было в заказе — удалить нельзя» | AC-018 |
| 09 | `SERVICE_IN_USE` | 409 | `ErrServiceInUse` (catalog) | «Услуга используется в заказах — удалить нельзя, отправьте её в архив» | ADR-005 (новый FK `subtasks.service_id`) |
| 10 | `PRODUCT_NOT_FOUND` | 404 | `ErrStockProductNotFound` (board; код = catalog) | «Товар не найден» | AC-015 |
| 11 | `PAID_ADMIN_ONLY` | 403 | `ErrPaidAdminOnly` (board) | «Оплату меняет только администратор» | AC-011 |
| 12 | `FORBIDDEN` | 403 | `middleware.RequireAdmin(nil)` (как есть) — только роль-гард маршрута | «Недостаточно прав» | AC-006, AC-012, AC-015 |
| 13 | `VALIDATION` | 400 | `errorspkg.Validation(...)` | `details.fields[]` | AC-006, AC-009 |
| 14 | `BAD_REQUEST` | 400 | strictJSON / тип поля | мусорное тело (TD-4) | — |
| 15 | `PRODUCT_IN_USE` | 409 | `ErrProductInUse` (catalog, as-is) — текст расширить | «Товар используется в услугах, заказах или журнале склада — удалить нельзя, отправьте его в архив» | ADR-005 |

- Уточнение spec (§Обработка ошибок «404 клиент/ответственный — в форме ошибка поля»): ссылки ИЗ ТЕЛА (`client_id`, `column_id`, `assignee_id`) → 400 `VALIDATION` с `details.fields[{name, message:"не найден"}]` (эталон `catalog` `category_id`); 404 — только id в пути. Пользователь видит ошибку поля — поведение spec сохранено.
- 403 различимы по `code`: `FORBIDDEN` — маршрут закрыт ролью; `PAID_ADMIN_ONLY` — маршрут доступен, запрещено только изменение `is_paid` (клиент блокирует переключатель оплаты, форма остаётся рабочей).
- `tasks.id` в пути: `strconv.ParseInt` > 0, иначе `TASK_NOT_FOUND` (до SQL). uuid-пути (`columns/:id`, `stock/:product_id`) — `uuid.Validate`, иначе 404.

## Details Subtasks
```json
{"code":"SUBTASKS_NOT_DONE","message":"...","details":{"subtasks":[{"id":"<uuid>","service_name":"Замена картриджа","status":"in_progress"}]}}
```

## Validation Fields
| Поле | Правило | message |
|---|---|---|
| `title` | trim, 1..200 | «обязательно» / «не длиннее 200» |
| `client_id` | uuid, существует | «обязательно» / «не найден» |
| `column_id` | uuid, существует | «обязательно» / «не найдена» |
| `assignee_id` | NULL или uuid активного пользователя (проверка только при смене значения) | «не найден» |
| `due_date` | NULL или `YYYY-MM-DD` | «неверная дата» |
| `version` (PUT задачи) | обязателен, ≥1 | «обязательно» |
| `name` (колонка) | trim, 1..60 | «обязательно» / «не длиннее 60» |
| `color` | `^#[0-9A-Fa-f]{6}$` | «цвет в формате #RRGGBB» |
| `position` (колонка) | int ≥1 (clamp к N) | «неверная позиция» |
| `prev_id`/`next_id` | int64 или null (тип — strictJSON 400) | — |
| `stock` (Остатки) | число, NUMERIC(12,3) (валидатор `pkg/numeric`) | «слишком большое» / «не больше 3 знаков» |
| query `paid`/`due`/`assignee`/`scope` | enum (design-go-layers §Enum), иначе 400 field | «неверное значение» |
| query `limit`/`offset` | int, `limit` 1..1000, `offset` ≥0 | «неверное значение» |

## Sentinel Errors
```go
// internal/features/board/errors.go
var (
    ErrTaskNotFound         = errorspkg.New("TASK_NOT_FOUND", http.StatusNotFound, "Заказ не найден")
    ErrColumnNotFound       = errorspkg.New("COLUMN_NOT_FOUND", http.StatusNotFound, "Колонка не найдена")
    ErrSubtasksNotDone      = errorspkg.New("SUBTASKS_NOT_DONE", http.StatusConflict, "Есть незавершённые услуги — в финальную колонку нельзя")
    ErrColumnNotEmpty       = errorspkg.New("COLUMN_NOT_EMPTY", http.StatusConflict, "В колонке есть заказы")
    ErrLastNonFinalColumn   = errorspkg.New("LAST_NON_FINAL_COLUMN", http.StatusConflict, "Нужна хотя бы одна нефинальная колонка")
    ErrTaskVersionConflict  = errorspkg.New("TASK_VERSION_CONFLICT", http.StatusConflict, "Заказ уже изменён, обновите")
    ErrStockProductNotFound = errorspkg.New("PRODUCT_NOT_FOUND", http.StatusNotFound, "Товар не найден")
    ErrPaidAdminOnly        = errorspkg.New("PAID_ADMIN_ONLY", http.StatusForbidden, "Оплату меняет только администратор")
)
// репозиторные (внутренние): errNotFound, errVersionConflict, errColumnHasTasks (23503 columns FK)

// internal/features/clients/service.go — добавить
ErrClientHasTasks       = errorspkg.New("CLIENT_HAS_TASKS", http.StatusConflict, "У клиента есть заказы — удалить нельзя")
ErrEquipmentUsedInTasks = errorspkg.New("EQUIPMENT_USED_IN_TASKS", http.StatusConflict, "Оборудование было в заказе — удалить нельзя")

// internal/features/catalog/services_service.go — добавить
ErrServiceInUse = errorspkg.New("SERVICE_IN_USE", http.StatusConflict, "Услуга используется в заказах — удалить нельзя, отправьте её в архив")
```

## Error Mapping
| Источник | Условие | → |
|---|---|---|
| board repo | `pgx.ErrNoRows` на Lock Task | `ErrTaskNotFound` |
| board repo | UPDATE ... `version=$2` 0 строк, id есть | `ErrTaskVersionConflict` |
| board repo | 23503 на `tasks_column_id_fkey` при DELETE columns | `ErrColumnNotEmpty` |
| board repo | 23503 на `tasks_client_id_fkey`/`tasks_assignee_id_fkey` при INSERT/UPDATE (гонка с удалением) | 400 field `client_id`/`assignee_id` |
| board tx | 40P01 / 40001 | повтор ×3 (`withTx`), затем 500 INTERNAL |
| board tx | 22003 в Reconcile | 500 INTERNAL + `stock_reconcile_errors_total++` |
| clients repo | 23503 `tasks_client_id_fkey` (гонка EXISTS→DELETE) | `ErrClientHasTasks` |
| clients repo | 23503 `task_equipment_equipment_id_fkey` / `subtasks_equipment_id_fkey` | `ErrEquipmentUsedInTasks` |
| catalog services repo | 23503 `subtasks_service_id_fkey` | `ErrServiceInUse` |
| catalog products repo | 23503 (любой FK, as-is) | `ErrProductInUse` |
| SSR clients/equipment/services delete | AppError 409 | PRG-flash с `Message` (as-is механизм `ssr.go remove`) |
- Классификация по `pgErr.ConstraintName` (не по тексту). Неизвестная ошибка → 500 без текста (as-is `FiberErrorHandler`).

## Слайс: 3-design/design-e2e-strategy.md [head]
MISSING

## Слайс: 1-research/output.md [sections]
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

## Слайс: AC-строки
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

## Паспорт (выжимка)
# Passport: phase-5-board (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 20 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
