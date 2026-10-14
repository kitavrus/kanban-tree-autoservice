package board

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// equipmentFieldMax — design-errors.md §Validation Fields (`completeness`/`condition` — 1000).
const equipmentFieldMax = 1000

// msgForeignClient — оборудование принадлежит другому клиенту, не заказу (design-errors.md
// §Validation Fields: POST /tasks/:id/equipment, «оборудование другого клиента»).
const msgForeignClient = "foreign_client"

func (in TaskEquipmentCreateRequest) normalize() TaskEquipmentCreateRequest {
	in.EquipmentID = strings.ToLower(strings.TrimSpace(in.EquipmentID)) // uuid — нижний регистр
	in.Completeness = strings.TrimSpace(in.Completeness)
	in.Condition = strings.TrimSpace(in.Condition)
	return in
}

// validate — формат equipment_id (обязателен, uuid) + длина completeness/condition;
// принадлежность клиенту заказа — сервис (design-errors.md: существование/связи — не
// формат).
func (in TaskEquipmentCreateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch {
	case in.EquipmentID == "":
		add("equipment_id", msgRequired)
	case uuid.Validate(in.EquipmentID) != nil:
		add("equipment_id", msgInvalid)
	}
	if utf8.RuneCountInString(in.Completeness) > equipmentFieldMax {
		add("completeness", msgTooLong)
	}
	if utf8.RuneCountInString(in.Condition) > equipmentFieldMax {
		add("condition", msgTooLong)
	}
	return f
}

func (in TaskEquipmentUpdateRequest) normalize() TaskEquipmentUpdateRequest {
	in.Completeness = strings.TrimSpace(in.Completeness)
	in.Condition = strings.TrimSpace(in.Condition)
	return in
}

func (in TaskEquipmentUpdateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	if utf8.RuneCountInString(in.Completeness) > equipmentFieldMax {
		add("completeness", msgTooLong)
	}
	if utf8.RuneCountInString(in.Condition) > equipmentFieldMax {
		add("condition", msgTooLong)
	}
	return f
}
