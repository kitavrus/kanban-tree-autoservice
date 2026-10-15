package board

import (
	"time"

	"github.com/kitavrus/kanban-tree/internal/features/clients"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Wire-типы заказов — JSON только snake_case (§API-1); конверт списка {items,total,limit,
// offset} (design-go-layers.md §DTO Task).

// TaskCard — элемент списка доски. Position — NUMERIC строкой, клиенту только для
// отладки/сортировки (не Amount — это не денежная величина).
type TaskCard struct {
	ID         int64  `json:"id"`
	ColumnID   string `json:"column_id"`
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	// ClientIsDeleted — клиент заказа мягко удалён (soft-delete-clients DS-014): заказ остаётся
	// действующим, у имени клиента — бейдж «удалён».
	ClientIsDeleted bool    `json:"client_is_deleted"`
	Title           string  `json:"title"`
	AssigneeID      *string `json:"assignee_id"`
	AssigneeName    *string `json:"assignee_name"`
	DueDate         *string `json:"due_date"` // YYYY-MM-DD
	IsPaid          bool    `json:"is_paid"`
	Position        string  `json:"position"`
	Version         int64   `json:"version"`
	SubtasksTotal   int     `json:"subtasks_total"`
	SubtasksDone    int     `json:"subtasks_done"`
	SubtasksLate    int     `json:"subtasks_late"`
	// TotalAmount/LastLog — фаза 6 (design-sql.md §Totals Expression/§Last Log Expression).
	// Одно имя итога и в GET /tasks, и в карточке (design-go-layers.md §DTO OrderCard).
	TotalAmount numeric.Amount `json:"total_amount"`
	LastLog     *TaskLastLog   `json:"last_log"` // null — записей журнала нет (AC-012)
	// ColumnTotal — pagination DS-002: число заказов ЭТОЙ колонки, подходящих под фильтры
	// (без учёта окна per_column и limit/offset). Всегда присутствует в GET /tasks; в
	// GET /tasks/:id и mutation-ответах — 0 (не вычисляется, клиент его там не читает).
	ColumnTotal int `json:"column_total"`
	// ExecutorUnpaidDone — число услуг заказа со статусом done и назначенным исполнителем,
	// за которые исполнителю ещё не выплачено (D68, метка на карточке доски; 0 — метки нет).
	// Вычисляется в GET /tasks и GET /tasks/:id.
	ExecutorUnpaidDone int `json:"executor_unpaid_done"`
}

// TaskLastLog — последняя запись журнала работ ЛЮБОЙ услуги заказа (карточка доски, AC-012).
type TaskLastLog struct {
	Text       string    `json:"text"` // ≤120 символов + «…» при обрезке
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

// TaskListResponse — гарантия порядка items (контракт): columns.position, columns.id,
// tasks.position, tasks.id — по колонкам слева направо, внутри колонки сверху вниз;
// стабилен между запросами. Дефолты: limit=500 (1..1000), offset=0.
//
// pagination DS-002 (аддитивно): query column_id (uuid) — только эта колонка; per_column
// (1..1000) — не более N первых (tasks.position, tasks.id) карточек каждой колонки, limit/
// offset применяются ПОСЛЕ окна. Total — число заказов под фильтрами (с column_id, без окна
// per_column); на странице за пределами выдачи (offset ≥ числа строк) items пуст и total=0
// (count(*) OVER () — прежнее поведение). Счётчик по колонке — TaskCard.ColumnTotal.
type TaskListResponse struct {
	Items  []TaskCard `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// TaskResponse — GET /tasks/:id и в mutation-ответах.
type TaskResponse struct {
	TaskCard
	PaidAt          *time.Time `json:"paid_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DescriptionHTML string     `json:"description_html"` // хранимый HTML (ADR-006); клиент берёт текст через DOMParser
}

// TaskHeader — поля шапки заказа, общие для POST и PUT. IsPaid: executor — nil|false, иначе
// 403 PAID_ADMIN_ONLY (AC-011). Клиента и автомобиля здесь нет (one-car-order): они задаются
// только при создании (TaskCreateRequest) и потом не меняются — PUT с client_id/equipment_id
// отклоняется как тело с неизвестным полем (strictJSON → 400 BAD_REQUEST).
type TaskHeader struct {
	Title      string  `json:"title"`
	ColumnID   string  `json:"column_id"`
	AssigneeID *string `json:"assignee_id"`
	DueDate    *string `json:"due_date"`
	IsPaid     *bool   `json:"is_paid"`
	// Description — plain text ≤10000 символов (design-errors.md §Validation Fields).
	// POST: nil → "" (пусто); PUT (через embed в TaskUpdateRequest): nil → не менять.
	Description *string `json:"description"`
}

// TaskCreateRequest — POST /api/v1/tasks (одна транзакция). Заказ = ровно один автомобиль
// (one-car-order): plate_number обязателен и ровно одно из equipment_id (найденный
// автомобиль — клиент заказа его владелец, client_id можно не передавать) либо new_equipment
// (новый автомобиль клиента client_id либо нового клиента new_client). title пуст →
// госномер. Автомобиль пишется в tasks.car_id; заказа без автомобиля не бывает.
//
// Ошибки полей — details.fields[] с путями new_client.phones[0].phone и т.п.
type TaskCreateRequest struct {
	TaskHeader
	ClientID     string            `json:"client_id"`
	PlateNumber  string            `json:"plate_number"`
	EquipmentID  string            `json:"equipment_id"`
	NewEquipment *TaskNewEquipment `json:"new_equipment"`
	NewClient    *TaskNewClient    `json:"new_client"`
}

// TaskNewEquipment — признак «новый автомобиль» (`new_equipment: {}`): автомобиль — только
// госномер, он берётся из plate_number запроса.
type TaskNewEquipment struct{}

// TaskNewClient — новый клиент-владелец нового автомобиля. Правила — как у POST
// /api/v1/clients: company_name (ФИО) и хотя бы один телефон обязательны.
type TaskNewClient struct {
	CompanyName string             `json:"company_name"`
	Phones      []clients.PhoneDTO `json:"phones"`
}

// TaskUpdateRequest — PUT /api/v1/tasks/:id, полная замена шапки. Version обязателен
// (§FE-1.1, оптимистичная блокировка).
type TaskUpdateRequest struct {
	TaskHeader
	Version *int64 `json:"version"`
}

// TaskMoveRequest — POST /api/v1/tasks/:id/move. PrevID/NextID — карточки сверху/снизу
// после вставки (ADR-001); «чужой»/несуществующий сосед игнорируется (position.go).
type TaskMoveRequest struct {
	ColumnID string `json:"column_id"`
	PrevID   *int64 `json:"prev_id"`
	NextID   *int64 `json:"next_id"`
}

// TaskMutationResponse — ответ Create/Update/Move.
type TaskMutationResponse struct {
	Task     TaskResponse   `json:"task"`
	Warnings []StockWarning `json:"warnings"`
}

// TaskDeleteResponse — ответ Delete: удаление возвращает товары на склад, клиенту нужны
// предупреждения (напр. below_min после возврата демонстративно маловероятно, но контракт
// единообразен с Create/Update/Move).
//
// soft-delete-core §6.1: удаление мягкое — ответ несёт номер заказа и момент удаления (UTC).
type TaskDeleteResponse struct {
	ID        int64          `json:"id"`
	DeletedAt time.Time      `json:"deleted_at"`
	Warnings  []StockWarning `json:"warnings"`
}

// RestoreNotice — что изменилось при восстановлении (soft-delete-core §6.2): code —
// column_replaced (DS-039), client_deleted / equipment_deleted (soft-delete-clients DS-016:
// клиент либо оборудование заказа в удалённых); message — готовый текст для пользователя.
type RestoreNotice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// TaskRestoreResponse — ответ POST /api/v1/tasks/:id/restore: предупреждения по остатку после
// повторного списания (формат карточки заказа) и notices; оба — всегда массивы.
type TaskRestoreResponse struct {
	ID       int64           `json:"id"`
	Warnings []StockWarning  `json:"warnings"`
	Notices  []RestoreNotice `json:"notices"`
}

// PurgeResponse — ответ служебной уборки DELETE /api/v1/test/purge/:kind/:id (§6.5).
type PurgeResponse struct {
	ID string `json:"id"`
}

// OpenSubtask — незавершённая услуга заказа (design-sql.md §Open Subtasks Query),
// сериализуется в details.subtasks[] ошибки SUBTASKS_NOT_DONE (ADR-102).
type OpenSubtask struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
}
