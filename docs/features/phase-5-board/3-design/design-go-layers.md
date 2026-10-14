# Design Go Layers: phase-5-board

> Owner: пакеты, DTO, enum'ы, интерфейсы service/repo, хендлеры, роуты, SSR/JS-модули. Эталон — плоский многосущностный пакет `internal/features/catalog` (префиксы файлов по сущности, D43), код вручную без scaffold. Нейминг — CLAUDE.md §GO-1; URL/JSON — §API-1.

## Package Layout
`internal/features/board` (package `board`, один пакет — ADR-100):
| Файл | Содержимое |
|---|---|
| `errors.go` | sentinel'ы (design-errors §Sentinel Errors) |
| `enums.go` | константы §Enum, `ParseXxx` |
| `columns_dto.go` / `columns_validate.go` / `columns_repo.go` / `columns_service.go` / `columns_handler.go` | колонки |
| `tasks_dto.go` / `tasks_validate.go` / `tasks_filter.go` / `tasks_repo.go` / `tasks_service.go` / `tasks_handler.go` | заказы, фильтры, move |
| `position.go` | Position Query + ребалансировка (design-sql §Position Query) |
| `stock_engine.go` | `StockEngine.Reconcile` + warnings (ADR-104) — экспортируется для `internal/seeddemo` |
| `stock_dto.go` / `stock_repo.go` / `stock_service.go` / `stock_handler.go` | «Остатки», summary, adjust, `NavBadges` |
| `assignees_handler.go` | `GET /api/v1/assignees` |
| `tx.go` | `withTx(ctx, pool, fn)` — retry 40P01/40001 ×3 |
| `ssr.go` | `Root = "web/ssr/board/templates"`, страницы `/` и `/stock` |
| `routes.go` | `Register(app, api, a, h *Handlers, s *SSR)` |
| `*_validate_test.go`, `*_fuzz_test.go`, `board_http_live_test.go`, `stock_engine_integration_test.go` | тесты (design-tests) |

Прочие изменения:
| Путь | Изменение |
|---|---|
| `pkg/numeric/amount.go` (+`amount_validate.go`) | перенос `Amount` + валидатора NUMERIC из `catalog` (ADR-101) |
| `internal/features/catalog/amount.go` | `type Amount = numeric.Amount` (alias), валидатор — вызов `numeric` |
| `internal/features/catalog/services_repo.go` / `services_service.go` | 23503 → `ErrServiceInUse` |
| `internal/features/catalog/products_service.go` | текст `ErrProductInUse` |
| `internal/features/clients/repo.go` / `service.go` | D26 guard'ы (design-sql §Client/Equipment Delete Guard Query) |
| `pkg/errorspkg/errors.go` | `Extra` + `WithDetail` (design-errors §errorspkg Extension) |
| `internal/ssr/menu.go` | `NavItem.Badge int`; `BuildMenu(enabled, isAdmin, badges map[string]int)` |
| `internal/ssr/shell.go` | `BadgeSource` + `ShellBuilder.WithBadges` (ADR-004) |
| `internal/server/router.go` | `NewShellBuilder("/", "/stock", "/users", "/catalog", "/clients").WithBadges(...)`; `board.Register`; удалить `home` |
| `internal/features/home/`, `web/ssr/home/` | удалить (доска занимает `/`, ADR-100) |
| `internal/seeddemo/seeddemo.go` | `Run(ctx, pool) (Result, error)` — AC-019 (ADR-105) |
| `cmd/seed-demo/main.go` | тонкая обёртка: DATABASE_URL → `seeddemo.Run` |
| `Makefile` | цель `seed-demo:` ВНЕ блока `project-bootstrap` (`. scripts/dev/env.sh && go run ./cmd/seed-demo`) |
| `migrations/0005_board.{up,down}.sql` | design-sql |
| `e2e/package.json` | devDependency `pg`, `@types/pg` (ADR-103) |

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

## TS Mirror
§TD-6.2: `e2e/fixtures/board-types.ts` — интерфейсы `Column`, `TaskCard`, `Task`, `StockWarning`, `StockRow`, `StockSummary`, `ApiErrorSubtasks` 1:1 по json-тегам выше, с комментарием-источником `// source: internal/features/board/*_dto.go`. Wire snake_case, TS-поля — snake (без camelize).

## Service Interface
```go
// Actor — из middleware.CurrentPrincipal(c) (эталон catalog/products_handler.go:77-80); ssr.Viewer id не несёт.
type Actor struct { ID string; IsAdmin bool }
type ColumnService interface {
    List(ctx) (*ColumnListResponse, error)
    Create(ctx, userID string, in ColumnInput) (*ColumnMutationResponse, error)
    Update(ctx, id, userID string, in ColumnInput) (*ColumnMutationResponse, error) // смена consumes_stock → Recalc (AC-007)
    Delete(ctx, id string) error
}
type TaskService interface {
    List(ctx, f TaskFilter) (*TaskListResponse, error)
    Get(ctx, id string) (*TaskResponse, error)
    Create(ctx, a Actor, in TaskCreateRequest) (*TaskMutationResponse, error)
    Update(ctx, a Actor, id string, in TaskUpdateRequest) (*TaskMutationResponse, error)
    Move(ctx, a Actor, id string, in TaskMoveRequest) (*TaskMutationResponse, error)
    Delete(ctx, a Actor, id string) (*TaskDeleteResponse, error)
}
type StockService interface {
    List(ctx, scope, q string, limit, offset int) (*StockListResponse, error)
    Summary(ctx) (*StockSummaryResponse, error)
    Adjust(ctx, productID, userID string, in StockAdjustRequest) (*StockRow, error)
    NavBadges(ctx) map[string]int  // ssr.BadgeSource; ошибка → slog.Warn + nil
}
```
Алгоритм `TaskService.Move` (Driver AC-002/003/004):
1. `id` → int64; `withTx`:
2. Lock Target Column `FOR SHARE` (нет → 400 field `column_id`).
3. Lock Task `FOR UPDATE` (нет → `ErrTaskNotFound`).
4. Если `target.is_final` и `column_id` сменилась ИЛИ заказ уже не в final → Open Subtasks; ≥1 → `ErrSubtasksNotDone.WithDetail("subtasks", list)`. (Перестановка внутри той же финальной колонки — без проверки.)
5. Position Query (+ребалансировка) → Move Task Query (`version+1`).
6. `engine.Reconcile(tx, id, target.consumes_stock, a.ID)` → warnings.
7. span `tasks.move` (attrs: `task_id`, `from_column`, `to_column`, `movements_count`).
`Update`: те же шаги 2–6 при смене `column_id` (позиция = в конец); `is_paid` ≠ текущего у executor → `ErrPaidAdminOnly` (403 `PAID_ADMIN_ONLY`) (проверка ПОСЛЕ Lock Task, до UPDATE); UPDATE по `version`.
`Create`: Lock Target Column → проверка client/assignee → Insert (позиция в конец) → Reconcile (no-op без услуг; единообразие).
`Delete`: только admin (роут + повторная проверка роли в сервисе) → Lock Task → Reconcile(false) → Delete Task Query.

## Repository Interface
Структуры `*ColumnRepo`, `*TaskRepo`, `*StockRepo` над `*pgxpool.Pool`; методы, работающие в транзакции, принимают `pgx.Tx`:
```go
// TaskRepo
LockTargetColumn(ctx, tx pgx.Tx, columnID string) (columnFlags, error)
LockTask(ctx, tx pgx.Tx, id int64) (*Task, error)
OpenSubtasks(ctx, tx pgx.Tx, id int64) ([]OpenSubtask, error)
NextPosition(ctx, tx pgx.Tx, columnID string, taskID int64, prev, next *int64) (string, error) // position.go
Move(ctx, tx pgx.Tx, id int64, columnID, pos string) (*Task, error)
Update(ctx, tx pgx.Tx, id int64, version int64, in taskRow) (*Task, error)
Insert(ctx, tx pgx.Tx, in taskRow, createdBy string) (*Task, error)
Delete(ctx, tx pgx.Tx, id int64) error
List(ctx, f TaskFilter) ([]TaskCard, int, error)
Get(ctx, id int64) (*Task, error)
// ColumnRepo
LockAll(ctx, tx pgx.Tx) ([]Column, error)            // ORDER BY id FOR UPDATE
Insert/Update/Renumber/Delete(ctx, tx, ...)
TaskIDsForUpdate(ctx, tx pgx.Tx, columnID string) ([]int64, error)
// StockEngine
Reconcile(ctx, tx pgx.Tx, taskID int64, consumes bool, userID string) (ReconcileResult{Warnings []StockWarning; Movements int}, error)
// StockRepo
List/Summary/Adjust(...)
```

## Handler Columns
`columns_handler.go`: `List`, `Create`(201), `Update`(200), `Delete`(204 без тела — удаление колонки склад не трогает). Bind — `c.Bind().Body(&in)` (strictJSON), ошибка → `errorspkg.BadRequest`.

## Handler Tasks
`tasks_handler.go`: `List` (ParseTaskFilter с `Actor`), `Get`, `Create`(201), `Update`, `Move`, `Delete`(200 + `TaskDeleteResponse{warnings}` — удаление возвращает товары на склад, warnings нужны клиенту).

## Handler Stock
`stock_handler.go`: `List` (`scope`,`q`,`limit` default 200 max 1000,`offset`), `Summary`, `Adjust`.

## Routes
```go
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handlers, s *SSR) {
    admin := middleware.RequireAdmin(nil)
    col := api.Group("/columns", a.RequireAPI())
    col.Get("", h.Columns.List); col.Post("", admin, h.Columns.Create)
    col.Put("/:id", admin, h.Columns.Update); col.Delete("/:id", admin, h.Columns.Delete)

    t := api.Group("/tasks", a.RequireAPI())
    t.Get("", h.Tasks.List); t.Post("", h.Tasks.Create)
    t.Get("/:id", h.Tasks.Get); t.Put("/:id", h.Tasks.Update)
    t.Post("/:id/move", h.Tasks.Move); t.Delete("/:id", admin, h.Tasks.Delete)

    st := api.Group("/stock", a.RequireAPI())
    st.Get("", h.Stock.List); st.Get("/summary", h.Stock.Summary)
    st.Put("/:product_id", admin, h.Stock.Adjust)

    api.Get("/assignees", a.RequireAPI(), h.Assignees.List)

    if s == nil { return }
    app.Get("/", a.RequireSSR(), s.board)
    app.Get("/stock", a.RequireSSR(), s.stock)
}
```
- Path-параметр `:product_id` — допустим (§API-1 разрешает `:id`-параметры; snake внутри `:name` — не сегмент URL). Сегменты `columns`, `tasks`, `move`, `stock`, `summary`, `assignees` — kebab-валидны.

## Handler SSR
`ssr.go`: `board` → `Page(c, "board", BoardVM{Shell, ViewerID, IsAdmin})`, `stock` → `Page(c, "stock", StockVM{Shell})`. Данные грузит JS (rich-экран, D30); SSR отдаёт только каркас + `data-*` атрибуты (`data-viewer-id`, `data-is-admin`).

## Shell Badge
ADR-004:
```go
// internal/ssr/shell.go
type BadgeSource interface{ NavBadges(ctx context.Context) map[string]int }
func (b ShellBuilder) WithBadges(src BadgeSource) ShellBuilder // копия
// Build: badges := b.src.NavBadges(c.Context()) (если src != nil) → BuildMenu(enabled, isAdmin, badges)
// menu.go: NavItem{..., Badge int}
```
`base.html`: у пункта `{{.URL}}` = `/stock` рендерится `<span class="badge rounded-pill text-bg-danger ms-auto" data-nav-badge="{{.URL}}" data-testid="nav-badge-stock" {{if not .Badge}}hidden{{end}}>{{.Badge}}</span>` (span есть всегда — цель JS-обновления).

## Frontend Modules
Стек D30/D38: SSR html/template + vanilla JS + vendored Bootstrap 5.3; Element Plus/Vue — НЕ применимо.
| Файл | Роль |
|---|---|
| `web/ssr/board/templates/pages/board.html` | каркас доски: `{{block "actions"}}` (+Заказ; +Колонка — `{{if .Shell.IsAdmin}}`), фильтр-бар, `#board` (`.board-columns` — горизонтальный скролл контейнера), модалки `task-modal`, `column-modal`, `{{block "scripts"}}` → `board.js` |
| `web/ssr/board/templates/pages/stock.html` | KPI-плитки, переключатель risky/all, таблица, модалка правки остатка (admin), скрипт `stock.js` |
| `web/ssr/_shared/js/board.js` | `KT.board`: загрузка (`/api/v1/columns`, `/api/v1/tasks?<query>`, `/api/v1/assignees`, `/api/v1/catalog-categories?kind=service`), рендер колонок/карточек, HTML5 DnD (`dragstart/dragover/drop`, вычисление prev/next по DOM), оптимистичный перенос + откат на ошибке, меню карточки «Переместить в…» (ADR-106), фильтры ↔ `location.search` (`history.replaceState`), формы заказа/колонки (combobox клиента D6, блокировка «Сохранить» до ответа), toast warnings со ссылкой `/stock`, `ui-dialog` для 409 |
| `web/ssr/_shared/js/stock.js` | `KT.stock`: summary + list, правка остатка (admin) |
| `web/ssr/_shared/js/nav-badge.js` | `KT.refreshNavBadge()` → `GET /api/v1/stock/summary` → `[data-nav-badge="/stock"]`; подключается в `base.html` после `api.js` |
| `web/ssr/_shared/app.css` | `.board-columns` (flex, `overflow-x:auto`, `min-width:0`), `.board-col` (ширина 280px; ≤576px — 85vw), `.task-card` (`border-left: 4px solid var(--col-color)`), `.task-card.dragging/.drop-target`, overdue — `--bs-danger*`; только `--bs-*` токены (themes:both) |
Инвалидация бейджа: `KT.refreshNavBadge()` после успешных move / PUT со сменой колонки / POST / DELETE заказа / PUT колонки / PUT stock.

testid (E2E-контракт POM): `board-column-{id}`, `task-card-{id}`, `task-card-menu-{id}`, `task-move-to-{columnId}`, `filter-client|q|category|paid|assignee|due|reset`, `task-form`, `task-title`, `task-client`, `task-column`, `task-assignee`, `task-due`, `task-paid`, `task-save`, `task-delete`, `column-form`, `column-name|color|final|consumes|position|save|delete`, `toast-warnings`, `stock-kpi-low|out|will-low`, `stock-row-{productId}`, `stock-edit-{productId}`, `stock-scope`.
