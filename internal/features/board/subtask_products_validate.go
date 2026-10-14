package board

import (
	"strings"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

func (in SubtaskProductCreateRequest) normalize() SubtaskProductCreateRequest {
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.WarehouseID = strings.ToLower(strings.TrimSpace(in.WarehouseID)) // uuid — нижний регистр: сравнения «тот же склад» идут по строке
	return in
}

// validateWarehouseID — склад строки (DS-005): необязателен, но если передан — uuid.
func validateWarehouseID(id string, add func(name, msg string)) {
	if id != "" && uuid.Validate(id) != nil {
		add("warehouse_id", msgInvalid)
	}
}

// validate — формат product_id + qty (обязателен, >0, Scale 9/3); существование/is_active
// product_id — сервис (design-errors.md §Validation Fields).
func (in SubtaskProductCreateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch {
	case in.ProductID == "":
		add("product_id", msgRequired)
	case uuid.Validate(in.ProductID) != nil:
		add("product_id", msgInvalid)
	}

	if in.Qty == nil {
		add("qty", msgRequired)
	} else if code := in.Qty.Validate(qtyScale, false); code != "" {
		add("qty", code)
	} else if isZeroAmount(*in.Qty) {
		add("qty", msgNotPositive)
	}
	validateWarehouseID(in.WarehouseID, add)
	return f
}

// normalize — числовые поля не нуждаются в trim; метод — для единообразия контракта
// Row Requests (normalize()/validate() на каждом типе, design-go-layers.md §DTO Row
// Requests).
func (in SubtaskProductUpdateRequest) normalize() SubtaskProductUpdateRequest {
	in.WarehouseID = strings.ToLower(strings.TrimSpace(in.WarehouseID)) // uuid — нижний регистр: сравнения «тот же склад» идут по строке
	return in
}

// validate — PUT: полная замена (LWW) — qty обязателен и >0 (как при создании), price —
// NULL допустим («без цены», итог 0).
func (in SubtaskProductUpdateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	if in.Qty == nil {
		add("qty", msgRequired)
	} else if code := in.Qty.Validate(qtyScale, false); code != "" {
		add("qty", code)
	} else if isZeroAmount(*in.Qty) {
		add("qty", msgNotPositive)
	}

	if in.Price != nil {
		if code := in.Price.Validate(priceScale, false); code != "" {
			add("price", code)
		}
	}
	validateWarehouseID(in.WarehouseID, add)
	return f
}
