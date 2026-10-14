package softdelete

import (
	"context"
	"strings"
	"time"
)

// Kind — вид записей раздела «Удалённые» (значение query-параметра kind, §6.3).
type Kind string

const (
	KindTasks             Kind = "tasks"
	KindTransfers         Kind = "transfers"
	KindColumns           Kind = "columns"
	KindClients           Kind = "clients"
	KindEquipment         Kind = "equipment"
	KindProducts          Kind = "products"
	KindServices          Kind = "services"
	KindCatalogCategories Kind = "catalog-categories"
	KindEquipmentTypes    Kind = "equipment-types"
	KindWarehouses        Kind = "warehouses"
	KindSuppliers         Kind = "suppliers"
)

// Kinds — реестр видов (DS-001), порядок — порядок вкладок раздела. Вид, чей источник ещё не
// подключён (его часть мастер-спеки не смержена), отдаёт пустой список и счётчик 0.
var Kinds = []Kind{
	KindTasks, KindTransfers, KindColumns, KindClients, KindEquipment, KindProducts,
	KindServices, KindCatalogCategories, KindEquipmentTypes, KindWarehouses, KindSuppliers,
}

// IsKind — v входит в реестр.
func IsKind(v string) bool {
	for _, k := range Kinds {
		if string(k) == v {
			return true
		}
	}
	return false
}

// CountKey — ключ вида в counts ответа summary (JSON snake_case, §API-1).
func (k Kind) CountKey() string { return strings.ReplaceAll(string(k), "-", "_") }

// Filter — фильтры списка удалённого, сочетаются по «И». Q — уже экранирован для ILIKE
// (пусто — без поиска); QNumber — номер из строки поиска («128» / «#128») для видов с
// номером; DeletedFrom/DeletedTo — границы периода в UTC: [DeletedFrom; DeletedTo).
type Filter struct {
	Q           string
	QNumber     *int64
	DeletedFrom *time.Time
	DeletedTo   *time.Time
	DeletedBy   string
	Limit       int
	Offset      int
}

// Source — источник удалённых записей одного вида. Части 2–6 подключают свои источники через
// Service.Register; порядок списка — deleted_at по убыванию, затем id.
type Source interface {
	List(ctx context.Context, f Filter) ([]Item, int, error)
	Count(ctx context.Context) (int, error)
}
