package board

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// msgArchived/msgNotInOrder/msgNotPositive — коды полей фазы 6 (design-errors.md
// §Validation Fields: «в архиве» / «нет в оборудовании заказа» / «должно быть больше 0»).
const (
	msgArchived    = "archived"
	msgNotInOrder  = "not_in_order"
	msgNotPositive = "not_positive"
	// msgNoAssignee — is_executor_paid=true у услуги без исполнителя (D65).
	msgNoAssignee = "no_assignee"
)

// priceScale/qtyScale — NUMERIC(12,2)/(12,3): subtasks.price, subtask_products.price
// (Scale 10/2) и subtask_products.qty (Scale 9/3) (design-errors.md §Validation Fields).
// Отдельная декларация в board (не импорт из catalog, ADR-101 — фичи друг от друга не
// зависят).
var (
	priceScale = numeric.Scale{IntDigits: 10, FracDigits: 2}
	qtyScale   = numeric.Scale{IntDigits: 9, FracDigits: 3}
)

func (in SubtaskCreateRequest) normalize() SubtaskCreateRequest {
	in.ServiceID = strings.TrimSpace(in.ServiceID)
	return in
}

// validate — формат service_id; существование/is_active — сервис (design-errors.md:
// «404 — только id в пути», ссылки из тела — VALIDATION).
func (in SubtaskCreateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	switch {
	case in.ServiceID == "":
		f = append(f, errorspkg.Field{Name: "service_id", Message: msgRequired})
	case uuid.Validate(in.ServiceID) != nil:
		f = append(f, errorspkg.Field{Name: "service_id", Message: msgInvalid})
	}
	return f
}

// normalize — trim строковых указателей; пустая строка внутри указателя — «нет значения»
// для сервиса (nilIfEmpty, эталон tasks_validate.go).
func (in SubtaskUpdateRequest) normalize() SubtaskUpdateRequest {
	in.Status = strings.TrimSpace(in.Status)
	if in.AssigneeID != nil {
		v := strings.TrimSpace(*in.AssigneeID)
		in.AssigneeID = &v
	}
	if in.DueDate != nil {
		v := strings.TrimSpace(*in.DueDate)
		in.DueDate = &v
	}
	if in.EquipmentID != nil {
		v := strings.TrimSpace(*in.EquipmentID)
		in.EquipmentID = &v
	}
	return in
}

// validate — формат полей после normalize(); существование assignee_id/принадлежность
// equipment_id заказу — сервис (design-errors.md §Validation Fields).
func (in SubtaskUpdateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch {
	case in.Status == "":
		add("status", msgRequired)
	case !isSubtaskStatus(in.Status):
		add("status", msgInvalid)
	}

	if in.AssigneeID != nil && *in.AssigneeID != "" && uuid.Validate(*in.AssigneeID) != nil {
		add("assignee_id", msgInvalid)
	}

	if in.DueDate != nil && *in.DueDate != "" {
		if _, err := time.Parse(dateLayout, *in.DueDate); err != nil {
			add("due_date", msgInvalidDate)
		}
	}

	if in.Price != nil {
		if code := in.Price.Validate(priceScale, false); code != "" {
			add("price", code)
		}
	}

	if in.EquipmentID != nil && *in.EquipmentID != "" && uuid.Validate(*in.EquipmentID) != nil {
		add("equipment_id", msgInvalid)
	}

	// executor_payout — NUMERIC(12,2) ≥ 0, как price (D63).
	if in.ExecutorPayout.Value != nil {
		if code := in.ExecutorPayout.Value.Validate(priceScale, false); code != "" {
			add("executor_payout", code)
		}
	}
	return f
}

func isSubtaskStatus(v string) bool {
	for _, s := range SubtaskStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// isZeroAmount — decimalRe (pkg/numeric) уже гарантирует чистый десятичный формат без
// знака (для qty allowNegative=false отрицательное отсекается раньше в Validate) — «0»
// проверяется как отсутствие значащих цифр, кроме '0'/'.' (design-errors.md: qty «>0»,
// сам numeric.Amount.Validate допускает 0 как валидное неотрицательное значение).
func isZeroAmount(a numeric.Amount) bool {
	for _, ch := range string(a) {
		if ch != '0' && ch != '.' {
			return false
		}
	}
	return true
}
