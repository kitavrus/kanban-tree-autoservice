package board

// Enum'ы доски (design-go-layers.md §Enum*). Источник правды для валидации query-параметров
// и sync-тестов (enums_test.go — статическая сверка; сверка с pg_get_constraintdef CHECK'ов
// БД — live-сьюта фазы 03, board_http_live_test.go).

// SubtaskStatus — CHECK subtasks.status (migrations/0005_board.up.sql).
const (
	SubtaskStatusNew        = "new"
	SubtaskStatusInProgress = "in_progress"
	SubtaskStatusDone       = "done"
)

var SubtaskStatuses = []string{SubtaskStatusNew, SubtaskStatusInProgress, SubtaskStatusDone}

// PrintKind — вид печатного акта, path-параметр `/tasks/:id/print/:kind` (ADR-102).
// Значение ∉ PrintKinds → 404 (print_ssr.go), не VALIDATION — это SSR-страница, не API.
const (
	PrintKindIntake   = "intake"
	PrintKindHandover = "handover"
)

var PrintKinds = []string{PrintKindIntake, PrintKindHandover}

// PaidFilter — query `paid` списка заказов (AC-013).
const (
	PaidFilterAll    = "all"
	PaidFilterPaid   = "paid"
	PaidFilterUnpaid = "unpaid"
)

var PaidFilters = []string{PaidFilterAll, PaidFilterPaid, PaidFilterUnpaid}

// DueFilter — query `due` списка заказов (AC-013, ADR-003); финальные колонки не входят.
const (
	DueFilterAll     = "all"
	DueFilterSoon    = "soon"
	DueFilterOverdue = "overdue"
)

var DueFilters = []string{DueFilterAll, DueFilterSoon, DueFilterOverdue}

// AssigneeFilter — query `assignee`: "me"/"all"/<uuid>; отсутствует → по роли (AC-014).
const (
	AssigneeMe  = "me"
	AssigneeAll = "all"
)

// StockScope — query `scope` экрана «Остатки» (AC-015).
const (
	StockScopeRisky = "risky"
	StockScopeAll   = "all"
)

var StockScopes = []string{StockScopeRisky, StockScopeAll}

// StockState — вычисляется SQL (design-sql.md §Stock Pending CTE), не CHECK в БД.
const (
	StockStateOut     = "out"
	StockStateLow     = "low"
	StockStateWillLow = "will_low"
	StockStateOK      = "ok"
)

var StockStates = []string{StockStateOut, StockStateLow, StockStateWillLow, StockStateOK}

// WarningKind — StockWarning.Kind (Q-006, ADR-006).
const (
	WarningKindNegative = "negative"
	WarningKindBelowMin = "below_min"
)

var WarningKinds = []string{WarningKindNegative, WarningKindBelowMin}

// StockMovementReason — as-is CHECK stock_movements.reason (migrations/0004_catalog.up.sql).
// board пишет task_consume/task_return; manual — существующий ручной ввод (catalog).
const (
	StockMovementReasonTaskConsume = "task_consume"
	StockMovementReasonTaskReturn  = "task_return"
	StockMovementReasonManual      = "manual"
)

// ParseXxx для TaskFilter (client_id/q/category_id/paid/assignee/due) и Stock scope —
// tasks_filter.go/stock_handler.go (фазы 03/04, design-go-layers.md §Package Layout):
// конкретные сигнатуры парсинга query-параметров не входят в паспорт этой фазы (design-sql
// §Board List Query/Stock List Query используют их) — здесь заведён только источник правды
// enum-значений, которым эти будущие Parse-функции будут валидировать вход.

// EquipmentLocation — CHECK equipment.location_status (migrations/0014, transfers-core
// DS-008): местоположение единицы; меняют переходы документа «Перемещение» и (временно, до
// transfers-partners) добавление единицы «у клиента» в заказ со складом приёма.
const (
	EquipmentLocationAtClient    = "at_client"
	EquipmentLocationAtWarehouse = "at_warehouse"
	EquipmentLocationInTransit   = "in_transit"
)

var EquipmentLocations = []string{EquipmentLocationAtClient, EquipmentLocationAtWarehouse, EquipmentLocationInTransit}
