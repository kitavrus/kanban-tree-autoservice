# Идиомы проекта — слепок с фичи `board`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/board`

## Образцы слоёв (по одному файлу на слой; ещё 54 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/board/order_card_http_live_test.go`

```go
//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// OrderCardLiveSuite (§TD-8): карточка заказа (GET) + оборудование заказа. Реальный
// бинарь + TCP + реальный логин; auth-стабы и app.Test запрещены. Услуги/товары/журнал —
// фикстуры напрямую SQL (репозитории записи появятся в фазе 03); оборудование — через API
// этой фазы.
type OrderCardLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestOrderCardLiveSuite(t *testing.T)

func (s *OrderCardLiveSuite) SetupSuite()

// ---- DTO wire-типы (1:1 с internal/features/board/card_dto.go, §TD-6.2) ----

type cardColumnDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
}

type equipmentHistoryDTO struct {
	TaskID    int64  `json:"task_id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type cardEquipmentDTO struct {
	EquipmentID  string                `json:"equipment_id"`
	TypeName     string                `json:"type_name"`
	Brand        string                `json:"brand"`
	Model        string                `json:"model"`
	Serial       string                `json:"serial"`
	Completeness string                `json:"completeness"`
	Condition    string                `json:"condition"`
	History      []equipmentHistoryDTO `json:"history"`
}

type cardSubtaskProductDTO struct {
	ID        string       `json:"id"`
	ProductID string       `json:"product_id"`
	Name      string       `json:"name"`
	Unit      string       `json:"unit"`
	Position  int          `json:"position"`
	Qty       json.Number  `json:"qty"`
	Price     *json.Number `json:"price"`
	LineTotal json.Number  `json:"line_total"`
}

type cardSubtaskLogDTO struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	UserID     string `json:"user_id"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	CanEdit    bool   `json:"can_edit"`
}

type cardSubtaskDTO struct {
	ID               string                  `json:"id"`
	ServiceID        string                  `json:"service_id"`
	ServiceName      string                  `json:"service_name"`
	Status           string                  `json:"status"`
	AssigneeID       *string                 `json:"assignee_id"`
	AssigneeName     *string                 `json:"assignee_name"`
	AssigneeIsActive *bool                   `json:"assignee_is_active"`
	DueDate          *string                 `json:"due_date"`
	Price            *json.Number            `json:"price"`
	EquipmentID      *string                 `json:"equipment_id"`
	Position         int                     `json:"position"`
	Total            json.Number             `json:"total"`
	Products         []cardSubtaskProductDTO `json:"products"`
	Logs             []cardSubtaskLogDTO     `json:"logs"`
}

type cardStockDTO struct {
	ProductID    string      `json:"product_id"`
	Stock        json.Number `json:"stock"`
	MinStock     json.Number `json:"min_stock"`
	TaskReserved json.Number `json:"task_reserved"`
}

// cardTaskDTO — только поля, нужные этому сьюту (не весь TaskResponse — §TD-6.2 допускает
// частичное декодирование).
type cardTaskDTO struct {
	ID              int64       `json:"id"`
	Title           string      `json:"title"`
	ClientID        string      `json:"client_id"`
	ColumnID        string      `json:"column_id"`
	Version         int64       `json:"version"`
	IsPaid          bool        `json:"is_paid"`
	DescriptionHTML string      `json:"description_html"`
	TotalAmount     json.Number `json:"total_amount"`
}

type orderCardDTO struct {
	Task      cardTaskDTO        `json:"task"`
	Column    cardColumnDTO      `json:"column"`
	Equipment []cardEquipmentDTO `json:"equipment"`
	Subtasks  []cardSubtaskDTO   `json:"subtasks"`
	Stock     []cardStockDTO     `json:"stock"`
}

type orderCardMutationDTO struct {
	Card     orderCardDTO      `json:"card"`
	Warnings []stockWarningDTO `json:"warnings"`
}

type equipmentCreateDTO struct {
	ID string `json:"id"`
}

func (s *OrderCardLiveSuite) errOf(body []byte) apiErrDTO

func (s *OrderCardLiveSuite) fields(body []byte) map[string]string

// ---- фикстуры ----

func (s *OrderCardLiveSuite) newClient(company string) string

func (s *OrderCardLiveSuite) columnID(name string) string

func (s *OrderCardLiveSuite) newTask(clientID, columnID string) (int64, int64)

func (s *OrderCardLiveSuite) newServiceCategory() string

func (s *OrderCardLiveSuite) newService(price string) string

// newServiceNoPrice — услуга «по факту» (price=NULL, AC-011/design-sql.md §Totals
// Expression: работа не входит в subtask_total, только товары).
func (s *OrderCardLiveSuite) newServiceNoPrice() string

func (s *OrderCardLiveSuite) addSubtask(taskID int64, serviceID, price string) string

func (s *OrderCardLiveSuite) attachSubtaskEquipment(subtaskID, equipmentID string)

func (s *OrderCardLiveSuite) addLog(subtaskID, text string)

func (s *OrderCardLiveSuite) newEquipmentType() string

// newEquipment — единица напрямую SQL (клиентская фича — не в API этой фазы).
func (s *OrderCardLiveSuite) newEquipment(clientID string) string

func (s *OrderCardLiveSuite) getCard(b *testutil.Browser, id int64) (int, orderCardDTO, []byte)

func (s *OrderCardLiveSuite) doEquipment(b *testutil.Browser, method, path string, body map[string]any) (int, orderCardMutationDTO, []byte)

// doCard — тот же конверт card-мутации (услуги/товары/журнал этой фазы, не только
// оборудование) — doEquipment реализует ровно тот же паттерн, алиас для читаемости тестов.
func (s *OrderCardLiveSuite) doCard(b *testutil.Browser, method, path string, body map[string]any) (int, orderCardMutationDTO, []byte)

// ---- фикстуры фазы 03: товары/комплекты услуг ----

func (s *OrderCardLiveSuite) newProductCategory() string

func (s *OrderCardLiveSuite) newProduct(price, stock, minStock string) string

// addServiceKit — комплект товаров услуги (service_products), копируется CreateSubtask
// (AC-008).
func (s *OrderCardLiveSuite) addServiceKit(serviceID, productID, qty string, position int)

func (s *OrderCardLiveSuite) setServiceActive(id string, active bool)

func (s *OrderCardLiveSuite) setProductActive(id string, active bool)

// ---- Happy Path ----

// LV-101/AC-001: GET /tasks/:id/card — одним запросом полное дерево карточки.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_GetCard_FullTree()

// LV-106/AC-001: заказ без услуг/оборудования — массивы пустые ([]), не null.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_GetCard_EmptyArraysNotNull()

// LV-102/AC-002: description экранируется на сервере, PUT с description меняет, PUT с
// description=null — не трогает сохранённое значение.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Description_EscapeAndUpdate()

// LV-111: deep-link на доску с ?task=<id> — SSR-страница не падает (модалка — фронтенд,
// фаза 05).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_DeepLink_SSR()

// LV-108/AC-004: добавить/поправить/удалить оборудование заказа.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_AddUpdateRemove()

// LV-109/AC-005: новое оборудование прямо из карточки — ADR-004, два запроса
// (POST /clients/:id/equipment, затем POST /tasks/:id/equipment).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_NewFromCard()

// LV-110/AC-006: история других заказов с той же единицей оборудования.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_History()

// ---- Business Rules / Conflicts ----

// LV-142/AC-004: единица выбрана в открытой услуге заказа — DELETE → 409
// TASK_EQUIPMENT_IN_USE + details.subtasks.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_InUseConflict()

// LV-143/AC-004: смена клиента заказа — оборудование ПРЕЖНЕГО клиента, выбранное в
// открытой услуге, блокирует смену; свободное (не выбранное) — тихо убирается.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_ClientChange()

// LV-144/AC-004: оборудование другого клиента — 400 VALIDATION field equipment_id.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_ForeignClient400()

// ---- Invalid Path IDs (TD-4) ----

// LV-170 (часть — card/equipment пути): невалидные/несуществующие id в пути → 404.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_InvalidPathIDs404()

// ---- Subtasks/Products/Logs (фаза 03) ----

// LV-103/AC-007/AC-008: POST /tasks/:id/subtasks в НЕ списывающей колонке — цена из
// каталога, комплект товаров скопирован (qty/цена/порядок), позиция в конец, tasks.version
// не тронут.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_CreateWithKit()

// LV-104/AC-007/AC-011: PUT /subtasks/:id — status/assignee/due/price(null)/equipment_id.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_UpdateFields()

// LV-105/AC-008: POST/PUT/DELETE /subtask-products — line_total корректен на каждом шаге.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_SubtaskProduct_CRUD()

// LV-106/AC-013: итоги — subtask_total аддитивен из работы+товаров, task_total = Σ,
// GET /tasks total_amount — то же значение.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Totals()

// LV-107/AC-012: журнал работ — CRUD + GET /tasks last_log (обрезка 120 + «…»).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Logs_CRUD()

// LV-130/AC-002/AC-014: row-мутации НЕ трогают tasks.version.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Version_UnaffectedByRowMutations()

// LV-131/AC-002: два PUT /tasks/:id с одним version — второй 409, данные первого сохранены.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Version_ConflictOnHeader()

// LV-140/AC-011: финальная колонка — POST /tasks/:id/subtasks → 409, услуг не прибавилось.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_FinalColumn_BlocksCreate()

// LV-141/AC-011: финальная — done→in_progress 409, done→done с правкой цены — 200.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_FinalColumn_BlocksUndone()

// LV-145/AC-007: equipment_id не из оборудования ЭТОГО заказа — 400.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_EquipmentNotInOrder400()

// LV-146/AC-007: assignee_id заблокированного — 400 только при СМЕНЕ значения (SD-007).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_InactiveAssignee400()

// LV-147/AC-007/AC-008: услуга/товар в архиве — 400.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_ArchivedServiceOrProduct400()

// LV-148/AC-007/AC-012: DELETE /subtasks/:id — CASCADE товары/журнал.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_DeleteSubtask_CascadesLogsProducts()

// LV-160/AC-017/AC-008: executor — все row-эндпоинты, видит цену товара.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Access_ExecutorRowEndpoints()

// LV-163/AC-012: чужая запись журнала — 403 SUBTASK_LOG_FORBIDDEN, admin — 200.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Logs_ForbiddenForOthers()

// LV-164/AC-017: без сессии — все новые эндпоинты фазы 03 → 401.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_NoSession401()

// LV-171/AC-008/AC-012: границы qty/price/text/completeness/description/status/due_date.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_InvalidInput400()

// LV-172 (TD-4): мусорное тело — 400 BAD_REQUEST, не 500; лишнее поле в PUT /subtasks.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_GarbageBody400()

// LV-173/AC-014: повторный DELETE /subtask-products/:id — 404, идемпотентность.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_IdempotentDelete404()

// LV-174/AC-014: POST /subtasks/:id/logs удалённой услуги — 404 SUBTASK_NOT_FOUND.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_LogOnDeletedSubtask404()

// ---- Access Control ----

// D19: оборудование заказа доступно и executor (не только admin).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Executor_EquipmentAllowed()
```

### `internal/features/board/tasks_repo.go`

```go
package board

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Task — repo-модель заказа. LockTask/Move/Update/Insert заполняют базовые поля;
// ClientName/AssigneeName/Subtasks* заполняет только Get/List (JOIN'ы, design-sql.md
// §Board List Query) — «Re-fetch-after-write»: ответ клиенту всегда собирается из Get()
// ПОСЛЕ коммита транзакции, не из RETURNING строки записи.
type Task struct {
	ID            int64
	ColumnID      string
	ClientID      string
	ClientName    string
	Title         string
	AssigneeID    *string
	AssigneeName  *string
	DueDate       *string // YYYY-MM-DD
	IsPaid        bool
	PaidAt        *time.Time
	Position      string
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SubtasksTotal int
	SubtasksDone  int
	SubtasksLate  int
	// DescriptionHTML/TotalAmount/LastLog — фаза 6 (design-sql.md §Totals Expression/
	// §Last Log Expression); DescriptionHTML заполняют также LockTask (текущее значение
	// нужно TaskService.Update, когда description в теле — nil, «не менять»).
	DescriptionHTML string
	TotalAmount     numeric.Amount
	LastLog         *TaskLastLog
}

// columnFlags — колонка под FOR SHARE (design-sql.md §Lock Target Column Query).
type columnFlags struct {
	ID            string
	IsFinal       bool
	ConsumesStock bool
}

// taskRow — параметры записи Insert/Update (design-go-layers.md §Repository Interface:
// `in taskRow`).
type taskRow struct {
	Title           string
	ClientID        string
	ColumnID        string
	AssigneeID      *string
	DueDate         *string
	IsPaid          bool
	Position        string
	DescriptionHTML string
}

type TaskRepo struct {
	pool *pgxpool.Pool
}

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo { return &TaskRepo{pool: pool} }

// taskWriteReturning — общий список колонок RETURNING для Insert/Update/Move (без JOIN —
// денормализованные поля добавляет только Get/List).
const taskWriteReturning = `id, column_id, client_id, title, assignee_id, due_date::text, is_paid, paid_at, position::text, version, created_at, updated_at`

func scanTaskRow(row pgx.Row, t *Task) error

// TaskColumnID — design-sql.md §Task Column Id Query: первый шаг lockRowsScope (card_
// service.go), читается ДО блокировки колонки/заказа. 0 строк → errNotFound.
func (r *TaskRepo) TaskColumnID(ctx context.Context, tx pgx.Tx, id int64) (string, error)

// lastLogJoinSQL — design-sql.md §Last Log Expression, LEFT JOIN LATERAL коррелированный
// с t.id; алиасы ls/lu — во избежание конфликта с внешними s/u (Board List/Get). Go:
// `cut` → добавить «…» (buildLastLog).
const lastLogJoinSQL = `

// totalAmountSelectSQL — design-sql.md §Totals Expression, скаляр-фрагмент коррелирован с
// t.id (алиас s2 — во избежание конфликта с внешними алиасами Get/List).
const totalAmountSelectSQL = `

// buildLastLog — nil, если записей журнала у заказа нет вовсе (ll.text IS NULL).
func buildLastLog(text, author sql.NullString, cut sql.NullBool, createdAt sql.NullTime) *TaskLastLog

// LockTargetColumn — design-sql.md §Lock Target Column Query. 0 строк → errNotFound
// (сервис мапит в 400 field column_id, «404 — только id в пути»).
func (r *TaskRepo) LockTargetColumn(ctx context.Context, tx pgx.Tx, columnID string) (*columnFlags, error)

// LockTask — design-sql.md §Lock Task Query. 0 строк → errNotFound (ErrTaskNotFound).
func (r *TaskRepo) LockTask(ctx context.Context, tx pgx.Tx, id int64) (*Task, error)

// OpenSubtasks — design-sql.md §Open Subtasks Query (D11/D22): незавершённые услуги
// заказа, проверка перед переносом в is_final.
func (r *TaskRepo) OpenSubtasks(ctx context.Context, tx pgx.Tx, taskID int64) ([]OpenSubtask, error)

// NextPosition — делегирует position.go (design-sql.md §Position Query). taskID=0 —
// сентинел нового (ещё не вставленного) заказа, используется Create.
func (r *TaskRepo) NextPosition(ctx context.Context, tx pgx.Tx, columnID string, taskID int64, prevID, nextID *int64) (string, error)

// Move — design-sql.md §Move Task Query.
func (r *TaskRepo) Move(ctx context.Context, tx pgx.Tx, id int64, columnID, pos string) (*Task, error)

// Update — design-sql.md §Update Task Query. 0 строк при уже подтверждённом (LockTask)
// существовании заказа → errVersionConflict.
func (r *TaskRepo) Update(ctx context.Context, tx pgx.Tx, id, version int64, in taskRow) (*Task, error)

// Insert — design-sql.md §Insert Task Query.
func (r *TaskRepo) Insert(ctx context.Context, tx pgx.Tx, in taskRow, createdBy string) (*Task, error)

// Delete — design-sql.md §Delete Task Query. Вызывающая сторона уже отработала
// StockEngine.Reconcile(target=false) под тем же FOR UPDATE заказа.
func (r *TaskRepo) Delete(ctx context.Context, tx pgx.Tx, id int64) error

// ClientExists — проверка client_id (design-go-layers.md §Service Interface: «проверка
// client/assignee» перед Insert/Update). Запрос — в переданной tx (тот же снапшот
// READ COMMITTED, что и остальные шаги Create/Update).
func (r *TaskRepo) ClientExists(ctx context.Context, tx pgx.Tx, id string) (bool, error)

// ActiveAssigneeExists — design-errors.md §Validation Fields: assignee_id «NULL или uuid
// активного пользователя».
func (r *TaskRepo) ActiveAssigneeExists(ctx context.Context, tx pgx.Tx, id string) (bool, error)

// Get — design-sql.md §Board List Query (без пагинации/фильтров, одна строка): полная
// карточка заказа с денормализованными именами и счётчиками подзадач — источник ответа
// GET /tasks/:id и всех mutation-ответов (Re-fetch-after-write).
func (r *TaskRepo) Get(ctx context.Context, id int64) (*Task, error)

// boardListSQL — design-sql.md §Board List Query, `due_date`/`position` приведены к text
// (TaskCard.DueDate/Position — *string/string, не time.Time/Amount).
const boardListSQL = `

// List — design-sql.md §Board List Query. Q-3 передаётся вызывающей стороной уже
// экранированным (pkg/sqlutil.EscapeLike, tasks_filter.go).
func (r *TaskRepo) List(ctx context.Context, f TaskFilter) ([]TaskCard, int, error)
```

---

Исходники образца: 387,598 символов · слепок: 18,135 · сжатие ×21.4
