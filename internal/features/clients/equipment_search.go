package clients

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Поиск единиц оборудования по всем клиентам — GET /api/v1/equipment (transfers-core
// DS-057, §6.8): строка «+ Оборудование» документа «Перемещение». Чтение — любому
// авторизованному, как GET /api/v1/equipment/:id.

// Местоположение единицы (equipment.location_status, CHECK миграции 0014, DS-008).
const (
	LocationAtClient    = "at_client"
	LocationAtWarehouse = "at_warehouse"
	LocationInTransit   = "in_transit"
)

// LocationStatuses — допустимые значения фильтра location_status.
var LocationStatuses = []string{LocationAtClient, LocationAtWarehouse, LocationInTransit}

const (
	equipmentSearchDefaultLimit = 20
	equipmentSearchMaxLimit     = 100
	equipmentSearchQMax         = 100
)

// EquipmentSearchItem — единица в выдаче поиска.
type EquipmentSearchItem struct {
	ID               string  `json:"id"`
	ClientID         string  `json:"client_id"`
	ClientName       string  `json:"client_name"`
	TypeName         string  `json:"type_name"`
	Brand            string  `json:"brand"`
	Model            string  `json:"model"`
	Serial           string  `json:"serial"`
	LocationStatus   string  `json:"location_status"`
	WarehouseID      *string `json:"warehouse_id"`
	WarehouseName    *string `json:"warehouse_name"`
	ActiveTransferID *int64  `json:"active_transfer_id"`
}

type EquipmentSearchResponse struct {
	Items  []EquipmentSearchItem `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// EquipmentSearchQuery — сырые query-параметры.
type EquipmentSearchQuery struct {
	WarehouseID, LocationStatus, Q string
	Limit, Offset                  int
}

func isLocationStatus(v string) bool {
	for _, s := range LocationStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// validate — ошибки полей после trim; значения — коды (текст подбирает клиент).
func (q EquipmentSearchQuery) validate() []errorspkg.Field {
	var f []errorspkg.Field
	if q.WarehouseID != "" && (len(q.WarehouseID) != 36 || uuid.Validate(q.WarehouseID) != nil) {
		f = append(f, errorspkg.Field{Name: "warehouse_id", Message: "invalid"})
	}
	if q.LocationStatus != "" && !isLocationStatus(q.LocationStatus) {
		f = append(f, errorspkg.Field{Name: "location_status", Message: "invalid"})
	}
	switch {
	case utf8.RuneCountInString(q.Q) > equipmentSearchQMax:
		f = append(f, errorspkg.Field{Name: "q", Message: "too_long"})
	case sqlutil.HasInvalidText(q.Q):
		f = append(f, errorspkg.Field{Name: "q", Message: "invalid_text"})
	}
	return f
}

const equipmentSearchWhere = `
FROM equipment e
JOIN clients c         ON c.id = e.client_id
JOIN equipment_types t ON t.id = e.type_id
LEFT JOIN warehouses w ON w.id = e.warehouse_id
WHERE e.deleted_at IS NULL
  AND ($1::uuid IS NULL OR e.warehouse_id = $1::uuid)
  AND ($2::text IS NULL OR e.location_status = $2)
  AND ($3::text IS NULL OR e.model ILIKE $3 ESCAPE '\' OR e.brand ILIKE $3 ESCAPE '\'
       OR e.serial ILIKE $3 ESCAPE '\' OR c.company_name ILIKE $3 ESCAPE '\')`

// SearchEquipment — единицы по всем клиентам; фильтры по «И»; сортировка — клиент, модель.
func (r *Repo) SearchEquipment(ctx context.Context, q EquipmentSearchQuery) ([]EquipmentSearchItem, int, error) {
	var warehouse, location, like any
	if q.WarehouseID != "" {
		warehouse = q.WarehouseID
	}
	if q.LocationStatus != "" {
		location = q.LocationStatus
	}
	if q.Q != "" {
		like = "%" + sqlutil.EscapeLike(q.Q) + "%"
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)`+equipmentSearchWhere, warehouse, location, like).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT e.id::text, e.client_id::text, c.company_name, t.name, e.brand, e.model, e.serial,
       e.location_status, e.warehouse_id::text, w.name, e.active_transfer_id`+equipmentSearchWhere+`
ORDER BY c.company_name, e.model, e.id
LIMIT $4 OFFSET $5`, warehouse, location, like, q.Limit, q.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []EquipmentSearchItem{}
	for rows.Next() {
		var it EquipmentSearchItem
		if err := rows.Scan(&it.ID, &it.ClientID, &it.ClientName, &it.TypeName, &it.Brand, &it.Model, &it.Serial,
			&it.LocationStatus, &it.WarehouseID, &it.WarehouseName, &it.ActiveTransferID); err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// SearchEquipment — validate → лимиты (1–100, по умолчанию 20) → repo.
func (s *Service) SearchEquipment(ctx context.Context, q EquipmentSearchQuery) (*EquipmentSearchResponse, error) {
	q.WarehouseID = strings.ToLower(strings.TrimSpace(q.WarehouseID))
	q.LocationStatus = strings.TrimSpace(q.LocationStatus)
	q.Q = strings.TrimSpace(q.Q)
	if f := q.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	if q.Limit <= 0 {
		q.Limit = equipmentSearchDefaultLimit
	}
	q.Limit = min(q.Limit, equipmentSearchMaxLimit)
	q.Offset = max(q.Offset, 0)
	items, total, err := s.repo.SearchEquipment(ctx, q)
	if err != nil {
		return nil, err
	}
	return &EquipmentSearchResponse{Items: items, Total: total, Limit: q.Limit, Offset: q.Offset}, nil
}

// SearchEquipment — GET /api/v1/equipment?warehouse_id=&location_status=&q=&limit=&offset=.
func (h *Handler) SearchEquipment(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	resp, err := h.svc.SearchEquipment(c.Context(), EquipmentSearchQuery{
		WarehouseID: c.Query("warehouse_id"), LocationStatus: c.Query("location_status"), Q: c.Query("q"),
		Limit: limit, Offset: offset,
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
