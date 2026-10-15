# Идиомы проекта — слепок с фичи `board`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `.claude/worktrees/agitated-fermat-76a73a/internal/features/board`
- `.claude/worktrees/charming-brahmagupta-e30492/internal/features/board`
- `.claude/worktrees/determined-mestorf-7fadc2/internal/features/board`
- `.claude/worktrees/frosty-lumiere-aba02a/internal/features/board`
- `.claude/worktrees/sleepy-leakey-04a583/internal/features/board`
- `.claude/worktrees/sleepy-rhodes-c4ce4b/internal/features/board`
- `.claude/worktrees/sweet-satoshi-832a66/internal/features/board`
- `.claude/worktrees/upbeat-margulis-bae97c/internal/features/board`
- `internal/features/board`

## Образцы слоёв (по одному файлу на слой; ещё 742 однотипных файлов не включены — идиома у слоя одна)

### `.claude/worktrees/charming-brahmagupta-e30492/internal/features/board/order_card_http_live_test.go`

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
	// Местоположение единицы (transfers-core DS-008, §6.9).
	LocationStatus          string  `json:"location_status"`
	WarehouseID             *string `json:"warehouse_id"`
	WarehouseName           *string `json:"warehouse_name"`
	ActiveTransferID        *int64  `json:"active_transfer_id"`
	TransitToName           *string `json:"transit_to_name"`
	ActiveTransferDirection *string `json:"active_transfer_direction"`
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
	// warehouses DS-005: склад строки товара.
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
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
	ID               string       `json:"id"`
	ServiceID        string       `json:"service_id"`
	ServiceName      string       `json:"service_name"`
	Status           string       `json:"status"`
	AssigneeID       *string      `json:"assignee_id"`
	AssigneeName     *string      `json:"assignee_name"`
	AssigneeIsActive *bool        `json:"assignee_is_active"`
	DueDate          *string      `json:"due_date"`
	Price            *json.Number `json:"price"`
	EquipmentID      *string      `json:"equipment_id"`
	Position         int          `json:"position"`
	Total            json.Number  `json:"total"`
	// executor-payouts (D62–D65): выплата исполнителю.
	IsExecutorPaid bool                    `json:"is_executor_paid"`
	ExecutorPaidAt *string                 `json:"executor_paid_at"`
	ExecutorPayout *json.Number            `json:"executor_payout"`
	Products       []cardSubtaskProductDTO `json:"products"`
	Logs           []cardSubtaskLogDTO     `json:"logs"`
}

type cardStockDTO struct {
	ProductID    string      `json:"product_id"`
	WarehouseID  string      `json:"warehouse_id"`
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
	HasInTransit    bool        `json:"has_in_transit"`
	// TransfersAmount — «Продано по накладным» (transfers-sales DS-014).
	TransfersAmount json.Number `json:"transfers_amount"`
}

type orderCardDTO struct {
	Task       cardTaskDTO        `json:"task"`
	Column     cardColumnDTO      `json:"column"`
	Equipment  []cardEquipmentDTO `json:"equipment"`
	Subtasks   []cardSubtaskDTO   `json:"subtasks"`
	Stock      []cardStockDTO     `json:"stock"`
	Warehouses []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		IsActive bool   `json:"is_active"`
	} `json:"warehouses"`
	// Transfers — документы «Перемещение» заказа (transfers-partners §6.8), новые сверху.
	Transfers []cardTransferDTO `json:"transfers"`
}

// cardTransferDTO — 1:1 с board.CardTransfer.
type cardTransferDTO struct {
	ID            int64       `json:"id"`
	Direction     string      `json:"direction"`
	Status        string      `json:"status"`
	WarehouseName string      `json:"warehouse_name"`
	ItemsCount    int         `json:"items_count"`
	CreatedAt     string      `json:"created_at"`
	Amount        json.Number `json:"amount"`
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

### `.claude/worktrees/sweet-satoshi-832a66/internal/features/board/card_dto.go`

```go
package board

import (
	"encoding/json"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Wire-типы карточки заказа — JSON только snake_case (§API-1). GET /api/v1/tasks/:id/card
// и card в ответах row-мутаций (ADR-001, design-go-layers.md §DTO OrderCard).

// OrderCardResponse — итог заказа только в task.total_amount (одно имя в GET /tasks и в
// карточке; отдельного поля total нет). Equipment/Subtasks/Stock — всегда массивы, не null
// (AC-001).
type OrderCardResponse struct {
	Task      TaskResponse    `json:"task"`
	Column    CardColumn      `json:"column"`
	Equipment []CardEquipment `json:"equipment"`
	Subtasks  []CardSubtask   `json:"subtasks"`
	Stock     []CardStock     `json:"stock"`
	// Warehouses — склады для select'а строки товара (D45/DS-005): активные по position +
	// архивные и мягко удалённые (is_deleted), на которые ещё ссылаются строки этого заказа.
	Warehouses []CardWarehouse `json:"warehouses"`
	// Attachments — вложения заказа (фаза 7, D21), всегда массив, порядок загрузки.
	Attachments []CardAttachment `json:"attachments"`
	// Transfers — документы «Перемещение» заказа (transfers-partners DS-012), новые сверху;
	// всегда массив.
	Transfers []CardTransfer `json:"transfers"`
}

// CardTransfer — документ заказа в блоке «Перемещения заказа». WarehouseName — склад
// назначения (приём от клиента) либо отправления (возврат клиенту). Amount — сумма документа
// (transfers-sales DS-014): Σ price × qty_sent товарных строк; 0.00 для приёма от клиента и
// накладной без товаров — в любом статусе (в итог заказа идут только «В пути» и «Получено»).
type CardTransfer struct {
	ID            int64          `json:"id"`
	Direction     string         `json:"direction"` // client_to_warehouse | warehouse_to_client
	Status        string         `json:"status"`
	WarehouseName string         `json:"warehouse_name"`
	ItemsCount    int            `json:"items_count"`
	CreatedAt     time.Time      `json:"created_at"`
	Amount        numeric.Amount `json:"amount"`
}

// CardAttachment — вложение заказа. URL — отдача файла с проверкой сессии (inline);
// CanDelete — автор == актор либо актор admin.
type CardAttachment struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"` // photo | video | pdf
	OriginalName string    `json:"original_name"`
	ContentType  string    `json:"content_type"`
	Size         int64     `json:"size"`
	UserID       string    `json:"user_id"`
	AuthorName   string    `json:"author_name"`
	CreatedAt    time.Time `json:"created_at"`
	URL          string    `json:"url"`
	CanDelete    bool      `json:"can_delete"`
}

type CardColumn struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
}

// CardEquipment — единица оборудования клиента, принятая в заказ (AC-004).
type CardEquipment struct {
	EquipmentID string `json:"equipment_id"`
	// IsDeleted — единица мягко удалена (soft-delete-clients DS-014): в заказе остаётся с
	// бейджем «удалён».
	IsDeleted    bool               `json:"is_deleted"`
	TypeName     string             `json:"type_name"`
	Brand        string             `json:"brand"`
	Model        string             `json:"model"`
	Serial       string             `json:"serial"`
	Completeness string             `json:"completeness"`
	Condition    string             `json:"condition"`
	History      []EquipmentHistory `json:"history"`
	// Местоположение единицы (transfers-core DS-008, читается с equipment): LocationStatus —
	// at_client|at_warehouse|in_transit; WarehouseID/Name — склад (только at_warehouse);
	// ActiveTransferID — незавершённый документ «Перемещение» единицы (ссылка «перемещение
	// #N»); TransitToName — пока единица в пути: название склада назначения этого документа
	// либо «клиент» (документ «склад → клиент», transfers-partners DS-013).
	LocationStatus   string  `json:"location_status"`
	WarehouseID      *string `json:"warehouse_id"`
	WarehouseName    *string `json:"warehouse_name"`
	ActiveTransferID *int64  `json:"active_transfer_id"`
	TransitToName    *string `json:"transit_to_name"`
	// ActiveTransferDirection — направление незавершённого документа единицы (null — документа
	// нет): исполнителю «перемещение #N» — ссылка только у документов с клиентом, документ
	// «склад → склад» ему недоступен (transfers-partners DS-018).
	ActiveTransferDirection *string `json:"active_transfer_direction"`
}

// EquipmentHistory — другие заказы с этой же единицей оборудования (AC-006).
type EquipmentHistory struct {
	TaskID    int64     `json:"task_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// CardSubtask — услуга заказа + её товары/журнал.
type CardSubtask struct {
	ID          string `json:"id"`
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	// ServiceIsDeleted — услуга мягко удалена из каталога: бейдж «удалён» (DS-014).
	ServiceIsDeleted bool            `json:"service_is_deleted"`
	Status           string          `json:"status"` // Enum SubtaskStatus
	AssigneeID       *string         `json:"assignee_id"`
	AssigneeName     *string         `json:"assignee_name"`
	AssigneeIsActive *bool           `json:"assignee_is_active"` // null без исполнителя (SD-007)
	DueDate          *string         `json:"due_date"`           // YYYY-MM-DD
	Price            *numeric.Amount `json:"price"`              // null = «по факту»
	EquipmentID      *string         `json:"equipment_id"`
	Position         int             `json:"position"`
	Total            numeric.Amount  `json:"total"` // subtask_total
	// Выплата исполнителю (D62–D64): флаг, дата отметки (null, пока не выплачено) и сумма
	// выплаты — отдельная от price (null = не задана).
	IsExecutorPaid bool                 `json:"is_executor_paid"`
	ExecutorPaidAt *time.Time           `json:"executor_paid_at"`
	ExecutorPayout *numeric.Amount      `json:"executor_payout"`
	Products       []CardSubtaskProduct `json:"products"`
	Logs           []CardSubtaskLog     `json:"logs"`
}

type CardSubtaskProduct struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	// ProductIsDeleted — товар мягко удалён из каталога: бейдж «удалён» (DS-014).
	ProductIsDeleted bool            `json:"product_is_deleted"`
	Name             string          `json:"name"`
	Unit             string          `json:"unit"`
	Position         int             `json:"position"` // ADR-103; массив уже отсортирован
	Qty              numeric.Amount  `json:"qty"`
	Price            *numeric.Amount `json:"price"`
	LineTotal        numeric.Amount  `json:"line_total"`
	// WarehouseID/WarehouseName — склад списания строки (DS-005).
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
}

// CardSubtaskLog — запись журнала работ услуги (AC-012). CanEdit — автор == актор либо
// актор admin (D27), вычисляется в сервисе (Actor неизвестен репозиторию).
type CardSubtaskLog struct {
	ID         string    `json:"id"`
	Text       string    `json:"text"`
	UserID     string    `json:"user_id"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CanEdit    bool      `json:"can_edit"`
}

// CardWarehouse — склад в select'е строки товара карточки.
type CardWarehouse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	// IsDeleted — склад мягко удалён (soft-delete-warehouses-suppliers DS-014): присутствует,
	// только если на него ссылаются строки заказа; в выбор склада новой строки не попадает.
	IsDeleted bool `json:"is_deleted"`
}

// CardStock — остаток пары товар×склад, задействованной в карточке (AC-009, DS-005).
type CardStock struct {
	ProductID    string         `json:"product_id"`
	WarehouseID  string         `json:"warehouse_id"`
	Stock        numeric.Amount `json:"stock"`
	MinStock     numeric.Amount `json:"min_stock"`
	TaskReserved numeric.Amount `json:"task_reserved"` // task_stock.qty этого заказа по паре, 0 если нет
}

// OrderCardMutationResponse — ответ всех row-мутаций карточки (POST 201 / PUT 200 /
// DELETE 200): re-fetch-after-write, Warnings — as-is формат ADR-006 фазы 5 (пустой массив,
// не null, если Reconcile не запускался).
type OrderCardMutationResponse struct {
	Card     OrderCardResponse `json:"card"`
	Warnings []StockWarning    `json:"warnings"`
}

// --- DTO Row Requests (design-go-layers.md §DTO Row Requests) ---
// Все — strictJSON (лишнее поле → 400 BAD_REQUEST), normalize()/validate() на типе
// запроса (эталон tasks_validate.go). Правила — design-errors.md §Validation Fields.
// Поля объявлены здесь одним файлом (Package Layout); normalize/validate услуг/товаров/
// журнала реализуются по мере готовности сервисов (фаза 03) — в этой фазе реализованы
// только TaskEquipmentCreateRequest/TaskEquipmentUpdateRequest (task_equipment_validate.go).

type SubtaskCreateRequest struct {
	ServiceID string `json:"service_id"`
}

// SubtaskUpdateRequest — PUT /subtasks/:id: полная замена изменяемых полей строки (LWW);
// клиент шлёт всю строку.
//
// Выплата исполнителю (D62–D66) — НЕ часть полной замены: у этих полей есть второй
// писатель (отчёт «Выплаты», POST /executor-payouts/mark), поэтому поле, которого нет в
// теле, не меняется, и карточка шлёт их только когда правит именно их. IsExecutorPaid —
// nil → флаг прежний; перевод false→true допустим только при назначенном исполнителе
// (400 is_executor_paid: no_assignee). ExecutorPayout — нет поля → сумма прежняя, null →
// очистить, число → установить; правится независимо от флага. Вправе и admin, и executor.
type SubtaskUpdateRequest struct {
	Status         string          `json:"status"`
	AssigneeID     *string         `json:"assignee_id"`
	DueDate        *string         `json:"due_date"`
	Price          *numeric.Amount `json:"price"`
	EquipmentID    *string         `json:"equipment_id"`
	IsExecutorPaid *bool           `json:"is_executor_paid"`
	ExecutorPayout OptionalAmount  `json:"executor_payout"`
}

// SubtaskProductCreateRequest — WarehouseID пуст → склад по умолчанию (DS-005, D49).
type SubtaskProductCreateRequest struct {
	ProductID   string          `json:"product_id"`
	Qty         *numeric.Amount `json:"qty"` // цена копируется из products.price (AC-008)
	WarehouseID string          `json:"warehouse_id"`
}

// SubtaskProductUpdateRequest — WarehouseID пуст → склад строки не меняется; смена склада
// у списанной строки пересчитывает списание (возврат на старый, списание с нового, DS-005).
type SubtaskProductUpdateRequest struct {
	Qty         *numeric.Amount `json:"qty"`
	Price       *numeric.Amount `json:"price"` // null = без цены (в итог 0)
	WarehouseID string          `json:"warehouse_id"`
}

// TaskEquipmentCreateRequest — POST /tasks/:id/equipment (AC-004/AC-005). WarehouseID —
// упразднённый «склад приёма» (transfers-partners DS-045): поле принимается строгим разбором
// тела, но игнорируется при любом значении — единица остаётся там, где числится; на склад её
// приводит документ «клиент → склад».
type TaskEquipmentCreateRequest struct {
	EquipmentID  string          `json:"equipment_id"`
	Completeness string          `json:"completeness"`
	Condition    string          `json:"condition"`
	WarehouseID  json.RawMessage `json:"warehouse_id"`
}

// TaskEquipmentUpdateRequest — PUT /tasks/:id/equipment/:equipment_id.
type TaskEquipmentUpdateRequest struct {
	Completeness string `json:"completeness"`
	Condition    string `json:"condition"`
}

type SubtaskLogRequest struct {
	Text string `json:"text"`
}
```

---

Исходники образца: 7,307,366 символов · слепок: 25,269 · сжатие ×289.2
