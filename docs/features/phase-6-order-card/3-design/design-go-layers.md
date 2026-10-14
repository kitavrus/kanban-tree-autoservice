# Design Go Layers: phase-6-order-card

> Owner: DTO, enum, сервис/репозиторий, хендлеры, роуты, SSR печати, frontend-модули. Всё — в пакете `internal/features/board` (файлы-по-сущности, D43, код вручную по эталону фазы 5). Коды ошибок — только `design-errors.md`. JSON — snake_case (§API-1). NUMERIC — `pkg/numeric.Amount`; nullable-деньги — `*numeric.Amount` (nil → `null`; `Amount("")` сериализуется в `0` — для NULL-цены недопустимо).

## Package Layout
| Файл (новый / изменён) | Содержимое |
|---|---|
| `card_dto.go` (new) | §DTO OrderCard, §DTO Row Requests |
| `card_repo.go` (new) | `CardRepo` — Card Queries, Print Header Query |
| `card_service.go` (new) | `CardService` (struct), `Card`, `lockRowsScope`, `cardMutation` |
| `subtasks_{repo,service,validate}.go`, `subtask_products_{repo,service,validate}.go`, `subtask_logs_{repo,service,validate}.go`, `task_equipment_{repo,service,validate}.go` (new) | методы `CardService`/репозитории по сущностям |
| `card_handler.go` (new) | `CardHandler` — все row-эндпоинты + `Card` |
| `print_ssr.go` (new) | печать актов (§Handler Print) |
| `description.go` (new) | `descriptionToHTML(text) string` (ADR-006) |
| `tasks_dto.go`, `tasks_repo.go`, `tasks_service.go`, `tasks_validate.go` (mod) | `description`, `total_amount`, `last_log`, cleanup оборудования при смене клиента |
| `tx.go` (mod) | `withTx` ретраит также `errTaskMovedRetry` (ADR-100) |
| `errors.go` (mod) | Codes 16-22 (`design-errors.md`) |
| `routes.go`, `ssr.go` (mod) | §Routes; `NewSSR(r, shell, status *ssr.StatusPages, firm config.FirmInfo, card *CardService)` |
| `internal/config/config.go` (mod) | `Firm FirmInfo` из `FIRM_NAME`, `FIRM_ADDRESS`, `FIRM_PHONE` (пустые допустимы) |
| `migrations/0006_subtask_logs.{up,down}.sql` (new) | `design-sql.md §Subtask Logs Migration` |

## Enum SubtaskStatus
As-is `enums.go`: `SubtaskStatusNew="new"`, `SubtaskStatusInProgress="in_progress"`, `SubtaskStatusDone="done"`, `SubtaskStatuses`. Sync-тест — as-is `enums_test.go` (против CHECK `0005_board.up.sql`).

## Enum PrintKind
Новый, `enums.go`: `PrintKindIntake="intake"`, `PrintKindHandover="handover"`, `PrintKinds`. Источник — path `/tasks/:id/print/:kind`; иное → 404. Тест — `enums_test.go` (+кейс).

## DTO Task Changes
```go
// TaskCard (элемент GET /tasks, встроен в TaskResponse) — добавить:
TotalAmount numeric.Amount `json:"total_amount"` // design-sql.md §Totals Expression, AC-013
LastLog     *TaskLastLog   `json:"last_log"`     // null — записей нет, AC-012

type TaskLastLog struct {
    Text       string    `json:"text"`        // ≤120 символов + «…» при обрезке
    AuthorName string    `json:"author_name"`
    CreatedAt  time.Time `json:"created_at"`
}

// TaskResponse — добавить:
DescriptionHTML string `json:"description_html"` // хранимый HTML (ADR-006), клиент берёт текст через DOMParser

// TaskCreateRequest (и TaskUpdateRequest через embed) — добавить:
Description *string `json:"description"` // plain text ≤10000; POST nil→""; PUT nil→не менять
```
`TaskMutationResponse` / `TaskDeleteResponse` — as-is.

## DTO OrderCard
```go
// GET /api/v1/tasks/:id/card и card в ответах row-мутаций (ADR-001).
type OrderCardResponse struct {
    Task      TaskResponse    `json:"task"`
    Column    CardColumn      `json:"column"`
    Equipment []CardEquipment `json:"equipment"` // всегда массив
    Subtasks  []CardSubtask   `json:"subtasks"`
    Stock     []CardStock     `json:"stock"`     // по каждому товару карточки, для подсказки AC-009
}
// Итог заказа — только task.total_amount (одно имя в GET /tasks и в карточке; отдельного `total` нет).
type CardColumn struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
}
type CardEquipment struct {
    EquipmentID  string             `json:"equipment_id"`
    TypeName     string             `json:"type_name"`
    Brand        string             `json:"brand"`
    Model        string             `json:"model"`
    Serial       string             `json:"serial"`
    Completeness string             `json:"completeness"`
    Condition    string             `json:"condition"`
    History      []EquipmentHistory `json:"history"` // другие заказы с единицей, AC-006
}
type EquipmentHistory struct {
    TaskID    int64     `json:"task_id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"` // дата заказа для «Уже был в ремонте» (AC-006)
}
type CardSubtask struct {
    ID               string               `json:"id"`
    ServiceID        string               `json:"service_id"`
    ServiceName      string               `json:"service_name"`
    Status           string               `json:"status"`             // Enum SubtaskStatus
    AssigneeID       *string              `json:"assignee_id"`
    AssigneeName     *string              `json:"assignee_name"`
    AssigneeIsActive *bool                `json:"assignee_is_active"` // null без исполнителя; false → «неактивен» (SD-007)
    DueDate          *string              `json:"due_date"`           // YYYY-MM-DD
    Price            *numeric.Amount      `json:"price"`              // null = «по факту»
    EquipmentID      *string              `json:"equipment_id"`
    Position         int                  `json:"position"`
    Total            numeric.Amount       `json:"total"`              // subtask_total
    Products         []CardSubtaskProduct `json:"products"`
    Logs             []CardSubtaskLog     `json:"logs"`
}
type CardSubtaskProduct struct {
    ID        string          `json:"id"`
    ProductID string          `json:"product_id"`
    Name      string          `json:"name"`
    Unit      string          `json:"unit"`
    Position  int             `json:"position"` // ADR-103; массив уже отсортирован
    Qty       numeric.Amount  `json:"qty"`
    Price     *numeric.Amount `json:"price"`
    LineTotal numeric.Amount  `json:"line_total"`
}
type CardSubtaskLog struct {
    ID         string    `json:"id"`
    Text       string    `json:"text"`
    UserID     string    `json:"user_id"`
    AuthorName string    `json:"author_name"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
    CanEdit    bool      `json:"can_edit"` // автор == актор || актор admin (D27)
}
type CardStock struct {
    ProductID    string         `json:"product_id"`
    Stock        numeric.Amount `json:"stock"`
    MinStock     numeric.Amount `json:"min_stock"`
    TaskReserved numeric.Amount `json:"task_reserved"` // task_stock.qty этого заказа, 0 если нет
}
// Ответ всех row-мутаций (POST 201 / PUT 200 / DELETE 200).
type OrderCardMutationResponse struct {
    Card     OrderCardResponse `json:"card"`
    Warnings []StockWarning    `json:"warnings"` // as-is формат ADR-006 фазы 5; [] без Reconcile
}
```

## DTO Row Requests
Все — strictJSON (лишнее поле → 400 `BAD_REQUEST`), `normalize()/validate() []errorspkg.Field` на типе запроса (эталон `tasks_validate.go`). Правила — `design-errors.md §Validation Fields`.
```go
type SubtaskCreateRequest struct { ServiceID string `json:"service_id"` }
// PUT /subtasks/:id — полная замена изменяемых полей строки (LWW); клиент шлёт всю строку.
type SubtaskUpdateRequest struct {
    Status      string          `json:"status"`
    AssigneeID  *string         `json:"assignee_id"`
    DueDate     *string         `json:"due_date"`
    Price       *numeric.Amount `json:"price"`
    EquipmentID *string         `json:"equipment_id"`
}
type SubtaskProductCreateRequest struct {
    ProductID string          `json:"product_id"`
    Qty       *numeric.Amount `json:"qty"` // цена копируется из products.price (AC-008)
}
type SubtaskProductUpdateRequest struct {
    Qty   *numeric.Amount `json:"qty"`
    Price *numeric.Amount `json:"price"` // null = без цены (в итог 0)
}
type TaskEquipmentCreateRequest struct {
    EquipmentID  string `json:"equipment_id"`
    Completeness string `json:"completeness"`
    Condition    string `json:"condition"`
}
type TaskEquipmentUpdateRequest struct {
    Completeness string `json:"completeness"`
    Condition    string `json:"condition"`
}
type SubtaskLogRequest struct { Text string `json:"text"` }
```

## Service Interface
```go
type CardService struct { pool *pgxpool.Pool; tasks *TaskRepo; card *CardRepo; subtasks *SubtaskRepo;
    products *SubtaskProductRepo; logs *SubtaskLogRepo; equipment *TaskEquipmentRepo; engine *StockEngine }

func (s *CardService) Card(ctx context.Context, a Actor, taskID string) (*OrderCardResponse, error)
func (s *CardService) CreateSubtask(ctx context.Context, a Actor, taskID string, in SubtaskCreateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateSubtask(ctx context.Context, a Actor, id string, in SubtaskUpdateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) DeleteSubtask(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
func (s *CardService) CreateSubtaskProduct(ctx context.Context, a Actor, subtaskID string, in SubtaskProductCreateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateSubtaskProduct(ctx context.Context, a Actor, id string, in SubtaskProductUpdateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) DeleteSubtaskProduct(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
func (s *CardService) AddEquipment(ctx context.Context, a Actor, taskID string, in TaskEquipmentCreateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateEquipment(ctx context.Context, a Actor, taskID, equipmentID string, in TaskEquipmentUpdateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) RemoveEquipment(ctx context.Context, a Actor, taskID, equipmentID string) (*OrderCardMutationResponse, error)
func (s *CardService) CreateLog(ctx context.Context, a Actor, subtaskID string, in SubtaskLogRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateLog(ctx context.Context, a Actor, id string, in SubtaskLogRequest) (*OrderCardMutationResponse, error)
func (s *CardService) DeleteLog(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
func (s *CardService) PrintData(ctx context.Context, a Actor, taskID string) (*PrintVM, error)

// lockRowsScope — Lock Order (design-sql.md §Lock Order): TaskColumnID → LockTargetColumn (FOR SHARE)
// → LockTask (FOR UPDATE) → task.ColumnID != col.ID ? errTaskMovedRetry.
func (s *CardService) lockRowsScope(ctx context.Context, tx pgx.Tx, taskID int64) (*Task, *columnFlags, error)
// cardMutation — Re-fetch-after-write: Card() ПОСЛЕ commit + warnings Reconcile.
func (s *CardService) cardMutation(ctx context.Context, a Actor, taskID int64, r ReconcileResult) (*OrderCardMutationResponse, error)
```
Поток мутации товара/услуги (одна `withTx`): resolve parent (Subtask Parent Query) → `lockRowsScope` → Lock Subtask(/Product) (`task_id` сверяется в WHERE) → бизнес-правила (`SUBTASK_FINAL_COLUMN`) → ссылочные поля → запись → `engine.Reconcile(ctx, tx, taskID, col.ConsumesStock, a.ID)` для операций, меняющих товары (create/delete услуги, create/update/delete товара); `UpdateSubtask` — без Reconcile, но под теми же блокировками. `tasks.version`/`tasks.updated_at` не трогаются (ADR-002). Span: `otel.Tracer("board")` — `subtasks.mutate`, `subtask_products.mutate`, `task_equipment.mutate`, `subtask_logs.mutate`, `tasks.card`, `tasks.print`; атрибуты `task_id`, `subtask_id`/`product_id`, `op`.

`TaskService.Update` (mod): после paid-гарда, если `client_id` сменился — Foreign Equipment Query (409 или DELETE чужих `task_equipment`) до `repo.Update`; `description`: nil → текущее значение (`LockTask` дополнительно читает `description_html`), иначе `descriptionToHTML`.

## Repository Interface
```go
func (r *TaskRepo) TaskColumnID(ctx context.Context, tx pgx.Tx, id int64) (string, error) // errNotFound
// LockTargetColumn, LockTask — as-is (LockTask + description_html)
type CardRepo struct{ pool *pgxpool.Pool }
func (r *CardRepo) Column(ctx context.Context, id string) (*CardColumn, error)
func (r *CardRepo) Equipment(ctx context.Context, taskID int64) ([]CardEquipment, error)
func (r *CardRepo) Subtasks(ctx context.Context, taskID int64) ([]CardSubtask, error)
func (r *CardRepo) Products(ctx context.Context, taskID int64) (map[string][]CardSubtaskProduct, error) // по subtask_id
func (r *CardRepo) Logs(ctx context.Context, taskID int64) (map[string][]subtaskLogRow, error)
func (r *CardRepo) Stock(ctx context.Context, taskID int64) ([]CardStock, error)
func (r *CardRepo) PrintHeader(ctx context.Context, taskID int64) (*printHeader, error)
type SubtaskRepo struct{}
func (r *SubtaskRepo) TaskIDOf(ctx context.Context, tx pgx.Tx, id string) (int64, error)
func (r *SubtaskRepo) Lock(ctx context.Context, tx pgx.Tx, id string, taskID int64) (*subtaskRow, error)
func (r *SubtaskRepo) Insert(ctx context.Context, tx pgx.Tx, taskID int64, serviceID string) (string, error) // + копия комплекта
func (r *SubtaskRepo) ServiceState(ctx context.Context, tx pgx.Tx, serviceID string) (exists, active bool, err error)
func (r *SubtaskRepo) Update(ctx context.Context, tx pgx.Tx, id string, in subtaskWrite) error
func (r *SubtaskRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error
type SubtaskProductRepo struct{}
func (r *SubtaskProductRepo) ParentOf(ctx context.Context, tx pgx.Tx, id string) (taskID int64, subtaskID string, err error)
func (r *SubtaskProductRepo) Lock(ctx context.Context, tx pgx.Tx, id, subtaskID string) error
func (r *SubtaskProductRepo) ProductState(ctx context.Context, tx pgx.Tx, productID string) (exists, active bool, err error)
func (r *SubtaskProductRepo) Insert(ctx context.Context, tx pgx.Tx, subtaskID, productID string, qty numeric.Amount) (string, error)
func (r *SubtaskProductRepo) Update(ctx context.Context, tx pgx.Tx, id string, qty numeric.Amount, price *numeric.Amount) error
func (r *SubtaskProductRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error
type SubtaskLogRepo struct{ pool *pgxpool.Pool }
func (r *SubtaskLogRepo) Insert(ctx context.Context, subtaskID, userID, text string) (string, error) // 23503 → errNotFound
func (r *SubtaskLogRepo) Owner(ctx context.Context, id string) (userID string, taskID int64, err error)
func (r *SubtaskLogRepo) Update(ctx context.Context, id, text string) error
func (r *SubtaskLogRepo) Delete(ctx context.Context, id string) error
type TaskEquipmentRepo struct{}
func (r *TaskEquipmentRepo) EquipmentClient(ctx context.Context, tx pgx.Tx, equipmentID string) (string, error)
func (r *TaskEquipmentRepo) Insert(ctx context.Context, tx pgx.Tx, taskID int64, in TaskEquipmentCreateRequest) error
func (r *TaskEquipmentRepo) Update(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string, in TaskEquipmentUpdateRequest) error
func (r *TaskEquipmentRepo) UsedBy(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) ([]OpenSubtask, error)
func (r *TaskEquipmentRepo) Delete(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) error
func (r *TaskEquipmentRepo) Attached(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) (bool, error)
func (r *TaskEquipmentRepo) ForeignInUse(ctx context.Context, tx pgx.Tx, taskID int64, clientID string) ([]OpenSubtask, error)
func (r *TaskEquipmentRepo) DeleteForeign(ctx context.Context, tx pgx.Tx, taskID int64, clientID string) error
```
SQL — `design-sql.md` (одноимённые секции).

## Handler Card
`CardHandler{svc *CardService}` — один метод на эндпоинт (`card_handler.go`), `Actor` из `middleware.CurrentPrincipal` (as-is `TasksHandler`). Ответы: GET 200 `OrderCardResponse`; POST 201 / PUT 200 / DELETE 200 `OrderCardMutationResponse`. Ошибки — возврат `*errorspkg.AppError` в `FiberErrorHandler` (as-is).

## Handler Print
`print_ssr.go`, метод `(*SSR).print(c fiber.Ctx) error`: `kind` ∉ `PrintKinds` или TASK_NOT_FOUND → `StatusPages.NotFound`; иначе `r.Render(c.Response().BodyWriter(), "print-"+kind, "print", PrintVM)` + `Content-Type: text/html; charset=utf-8`. `no-store` — as-is `RequireSSR`.
```go
type PrintVM struct {
    Firm        config.FirmInfo // пустые поля → блок реквизитов опускается ({{ if }} по каждому)
    Kind        string          // intake | handover
    Card        OrderCardResponse
    ClientName  string; ContactName string; Phones string
    Description template.HTML   // task.description_html (ADR-006: хранится только серверно-экранированный HTML)
    PrintedAt   time.Time
}
```
Шаблоны: `web/ssr/board/templates/layouts/print.html` (`{{ define "print" }}`, без сайдбара/theme.js, `data-bs-theme="light"`, `/assets/ssr-shared/print.css` с `@media print`), `pages/print-intake.html`, `pages/print-handover.html`. Handover: все услуги со статусом, цена работы («по факту» — прочерк), товары qty×цена, итог `task.total_amount`, оплата (`is_paid`/`paid_at`).

## Routes
```go
// routes.go — добавить (все RequireAPI, CSRF на мутациях — каркас; admin-гардов нет, D19/AC-017)
t.Get("/:id/card", h.Card.Card)
t.Post("/:id/subtasks", h.Card.CreateSubtask)
t.Post("/:id/equipment", h.Card.AddEquipment)
t.Put("/:id/equipment/:equipment_id", h.Card.UpdateEquipment)
t.Delete("/:id/equipment/:equipment_id", h.Card.RemoveEquipment)
sub := api.Group("/subtasks", a.RequireAPI())
sub.Put("/:id", h.Card.UpdateSubtask); sub.Delete("/:id", h.Card.DeleteSubtask)
sub.Post("/:id/products", h.Card.CreateSubtaskProduct); sub.Post("/:id/logs", h.Card.CreateLog)
sp := api.Group("/subtask-products", a.RequireAPI())
sp.Put("/:id", h.Card.UpdateSubtaskProduct); sp.Delete("/:id", h.Card.DeleteSubtaskProduct)
sl := api.Group("/subtask-logs", a.RequireAPI())
sl.Put("/:id", h.Card.UpdateLog); sl.Delete("/:id", h.Card.DeleteLog)
// SSR (s != nil)
app.Get("/tasks/:id/print/:kind", a.RequireSSR(), s.print)
```
Новое оборудование из карточки — as-is `POST /api/v1/clients/:id/equipment` (без admin, D19) + `POST /api/v1/tasks/:id/equipment` (ADR-004). Справочники — as-is `GET /api/v1/services?q=`, `/products?q=`, `/equipment-types`, `/assignees`.

## Frontend Modules
SSR + vanilla JS + Bootstrap 5.3 (D30/D38). Element Plus/Vue — неприменимо.
- `web/ssr/_shared/js/task-card.js` (new, `KT.taskCard`): `open(taskId|null, defaultColumnId)`, `close()`; рендер разделов, row-сохранение, подсказка остатка, итоги. `board.js`: `openTaskForm` → `KT.taskCard.open`; `onChange` колбэк → `reloadAndRender()`; deep-link (ADR-104).
- `web/ssr/board/templates/pages/board.html` (mod): `#task-modal` → `modal-xl modal-fullscreen-lg-down modal-dialog-scrollable`; шапка — существующие поля + `#task-description` (textarea); контейнеры `#card-equipment`, `#card-subtasks`, `#card-total`; кнопки актов; `<script src="/assets/ssr-shared/js/task-card.js">` до `board.js`.
- Карточка доски (`cardHTML`): `total_amount` (если >0) + `last_log` (текст, автор); заголовок колонки — сумма `total_amount` видимых карточек в копейках (`Math.round(x*100)`), вывод `toFixed(2)` (ADR-005).
- Row-очередь (ADR-105): `rowQueue` по ключу строки (`subtask:<id>`, `product:<id>`, `equipment:<equipment_id>`, `log:<id>`); не более одного запроса строки в полёте; правки во время полёта копятся в `pending` строки и уходят ОДНИМ следующим запросом; тело PUT собирается в момент отправки = поля строки из последнего применённого `card` ⊕ `pending` (не из DOM и не из момента `change`). Ошибка → `pending` строки сбрасывается, значения — из последнего `card`.
- Row-сохранение (SD-008): `change` → `rowQueue.enqueue` → строка `aria-busy` → запрос → `applyCard(resp.card)` (patch: элементы, содержащие `document.activeElement`, не перезаписываются) → `showWarnings(resp.warnings)` (as-is формат фазы 5). Ошибка: 404 → `KT.uiAlert` «Строка уже удалена» + перечитать карточку; 400 → `is-invalid` + текст поля, откат значения; 409/403 → `KT.uiAlert(err.message)` + откат.
- Подсказка (AC-009): `avail = stock + task_reserved`; `need = Σ qty товара по всем услугам карточки`; `rest = avail − need`: `rest < 0` → «в наличии только {avail}»; `rest < min_stock` → «останется {rest} — минимум {min_stock}»; иначе «в наличии {avail}». Для товара в пикере до добавления — `stock/min_stock` из `GET /api/v1/products` (as-is).
- Новый заказ (SD-006): разделы оборудования/услуг `disabled` + подсказка «Сохраните заказ»; после `POST /tasks` — `open(id)` без закрытия модалки.
- Шапка: сохранение кнопкой (as-is PUT с `version`, `is_paid` отправляется всегда — board.js:590-593); смена клиента при наличии оборудования → `KT.uiConfirm` «Оборудование другого клиента будет убрано из заказа»; после 200 — `GET /card`.
- Удаление услуги — `KT.uiConfirm` «Услуга удалится вместе с товарами и журналом».
- Testid (контракт POM `e2e/pages/OrderCardPage.ts`): `card-description`, `card-equipment-add`, `card-equipment-new`, `card-equipment-row`, `card-equipment-completeness`, `card-equipment-condition`, `card-equipment-remove`, `card-equipment-history`, `card-subtask-add`, `card-subtask-row`, `card-subtask-status`, `card-subtask-assignee`, `card-subtask-due`, `card-subtask-price`, `card-subtask-equipment`, `card-subtask-delete`, `card-subtask-total`, `card-product-add`, `card-product-row`, `card-product-qty`, `card-product-price`, `card-product-hint`, `card-product-delete`, `card-log-text`, `card-log-add`, `card-log-row`, `card-log-edit`, `card-log-delete`, `card-total`, `card-print-intake`, `card-print-handover`, `card-warning`; на доске `task-card-total`, `task-card-last-log`, `column-total`. Admin-only (`task-delete`, `task-paid` enabled) — снятие testid атрибутом (as-is board.js:576-579); `card-log-edit/-delete` рендерятся только при `can_edit`.
- `web/ssr/_shared/print.css` (new): A4, `@media print` скрывает `.no-print` (кнопка «Печать»).
