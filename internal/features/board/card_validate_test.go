package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// TestTaskEquipmentCreateRequestValidate_MissingEquipmentID — UT-002/AC-004.
func TestTaskEquipmentCreateRequestValidate_MissingEquipmentID(t *testing.T) {
	in := TaskEquipmentCreateRequest{}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "equipment_id", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)

	in = TaskEquipmentCreateRequest{EquipmentID: "not-a-uuid"}.normalize()
	fields = in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "equipment_id", fields[0].Name)
	assert.Equal(t, msgInvalid, fields[0].Message)
}

// TestTaskEquipmentCreateRequestValidate_TooLongFields — UT-002/AC-004: completeness/
// condition >1000 символов → too_long; ровно 1000 — валидно (граница).
func TestTaskEquipmentCreateRequestValidate_TooLongFields(t *testing.T) {
	in := TaskEquipmentCreateRequest{
		EquipmentID: validUUID, Completeness: strings.Repeat("a", equipmentFieldMax+1),
		Condition: strings.Repeat("b", equipmentFieldMax+1),
	}.normalize()
	got := map[string]string{}
	for _, f := range in.validate() {
		got[f.Name] = f.Message
	}
	assert.Equal(t, msgTooLong, got["completeness"])
	assert.Equal(t, msgTooLong, got["condition"])

	atLimit := TaskEquipmentCreateRequest{
		EquipmentID: validUUID, Completeness: strings.Repeat("a", equipmentFieldMax),
		Condition: strings.Repeat("b", equipmentFieldMax),
	}.normalize()
	assert.Empty(t, atLimit.validate())
}

func TestTaskEquipmentUpdateRequestValidate_TooLongFields(t *testing.T) {
	in := TaskEquipmentUpdateRequest{
		Completeness: strings.Repeat("a", equipmentFieldMax+1),
		Condition:    strings.Repeat("b", equipmentFieldMax+1),
	}.normalize()
	got := map[string]string{}
	for _, f := range in.validate() {
		got[f.Name] = f.Message
	}
	assert.Equal(t, msgTooLong, got["completeness"])
	assert.Equal(t, msgTooLong, got["condition"])
}

// ---- UT-002: SubtaskCreateRequest ----

func TestSubtaskCreateRequestValidate(t *testing.T) {
	fields := func(in SubtaskCreateRequest) map[string]string {
		got := map[string]string{}
		for _, f := range in.normalize().validate() {
			got[f.Name] = f.Message
		}
		return got
	}

	assert.Equal(t, map[string]string{"service_id": msgRequired}, fields(SubtaskCreateRequest{}))
	assert.Equal(t, map[string]string{"service_id": msgInvalid}, fields(SubtaskCreateRequest{ServiceID: "not-a-uuid"}))
	assert.Empty(t, fields(SubtaskCreateRequest{ServiceID: validUUID}))
	assert.Empty(t, fields(SubtaskCreateRequest{ServiceID: "  " + validUUID + "  "}), "normalize — trim")
}

// ---- UT-002: SubtaskUpdateRequest ----

func amt(s string) *numeric.Amount { v := numeric.Amount(s); return &v }
func str(s string) *string         { return &s }

func TestSubtaskUpdateRequestValidate_StatusEnum(t *testing.T) {
	fields := func(status string) map[string]string {
		in := SubtaskUpdateRequest{Status: status}.normalize()
		got := map[string]string{}
		for _, f := range in.validate() {
			got[f.Name] = f.Message
		}
		return got
	}

	assert.Equal(t, msgRequired, fields("")["status"])
	assert.Equal(t, msgInvalid, fields("closed")["status"])
	for _, v := range SubtaskStatuses {
		assert.NotContains(t, fields(v), "status", "status=%q — валидно", v)
	}
}

func TestSubtaskUpdateRequestValidate_AssigneeDueEquipment(t *testing.T) {
	base := SubtaskUpdateRequest{Status: SubtaskStatusNew}

	f := func(in SubtaskUpdateRequest) map[string]string {
		got := map[string]string{}
		for _, fl := range in.normalize().validate() {
			got[fl.Name] = fl.Message
		}
		return got
	}

	// nil — допустимо (нет ни ответственного, ни срока, ни оборудования).
	assert.Empty(t, f(base))

	bad := base
	bad.AssigneeID = str("not-a-uuid")
	assert.Equal(t, msgInvalid, f(bad)["assignee_id"])

	bad = base
	bad.DueDate = str("2026-02-30")
	assert.Equal(t, msgInvalidDate, f(bad)["due_date"])

	ok := base
	ok.DueDate = str("2026-02-15")
	assert.Empty(t, f(ok))

	bad = base
	bad.EquipmentID = str("not-a-uuid")
	assert.Equal(t, msgInvalid, f(bad)["equipment_id"])

	// пустая строка внутри указателя — «нет значения», не ошибка формата.
	empty := base
	empty.AssigneeID, empty.DueDate, empty.EquipmentID = str(""), str(""), str("")
	assert.Empty(t, f(empty))
}

func TestSubtaskUpdateRequestValidate_PriceScale(t *testing.T) {
	f := func(price *numeric.Amount) map[string]string {
		in := SubtaskUpdateRequest{Status: SubtaskStatusNew, Price: price}.normalize()
		got := map[string]string{}
		for _, fl := range in.validate() {
			got[fl.Name] = fl.Message
		}
		return got
	}

	assert.Empty(t, f(nil), "nil — «по факту», валидно")
	assert.Empty(t, f(amt("0")))
	assert.Empty(t, f(amt("1234567890.12")), "10 цифр целой части — граница")
	assert.Equal(t, numeric.MsgTooLarge, f(amt("12345678901"))["price"])
	assert.Equal(t, numeric.MsgTooPrecise, f(amt("1.239"))["price"])
	assert.Equal(t, numeric.MsgInvalid, f(amt("-1"))["price"], "отрицательная цена не допускается")
	assert.Equal(t, numeric.MsgInvalidFormat, f(amt("NaN"))["price"])
}

// ---- UT-002: SubtaskProductCreateRequest/UpdateRequest ----

func TestSubtaskProductCreateRequestValidate_Qty(t *testing.T) {
	f := func(in SubtaskProductCreateRequest) map[string]string {
		got := map[string]string{}
		for _, fl := range in.normalize().validate() {
			got[fl.Name] = fl.Message
		}
		return got
	}

	assert.Equal(t, msgRequired, f(SubtaskProductCreateRequest{ProductID: validUUID})["qty"])
	assert.Equal(t, msgNotPositive, f(SubtaskProductCreateRequest{ProductID: validUUID, Qty: amt("0")})["qty"])
	assert.Equal(t, numeric.MsgInvalid, f(SubtaskProductCreateRequest{ProductID: validUUID, Qty: amt("-1")})["qty"])
	assert.Equal(t, numeric.MsgTooPrecise, f(SubtaskProductCreateRequest{ProductID: validUUID, Qty: amt("1.2345")})["qty"])
	assert.Empty(t, f(SubtaskProductCreateRequest{ProductID: validUUID, Qty: amt("0.5")}))

	assert.Equal(t, msgRequired, f(SubtaskProductCreateRequest{Qty: amt("1")})["product_id"])
	assert.Equal(t, msgInvalid, f(SubtaskProductCreateRequest{ProductID: "x", Qty: amt("1")})["product_id"])
}

func TestSubtaskProductUpdateRequestValidate(t *testing.T) {
	f := func(in SubtaskProductUpdateRequest) map[string]string {
		got := map[string]string{}
		for _, fl := range in.normalize().validate() {
			got[fl.Name] = fl.Message
		}
		return got
	}

	assert.Equal(t, msgRequired, f(SubtaskProductUpdateRequest{})["qty"])
	assert.Equal(t, msgNotPositive, f(SubtaskProductUpdateRequest{Qty: amt("0")})["qty"])
	assert.Empty(t, f(SubtaskProductUpdateRequest{Qty: amt("2")}), "price=nil — «без цены», валидно")
	assert.Equal(t, numeric.MsgInvalid, f(SubtaskProductUpdateRequest{Qty: amt("1"), Price: amt("-1")})["price"])
	assert.Empty(t, f(SubtaskProductUpdateRequest{Qty: amt("1"), Price: amt("0")}))
}

// ---- UT-002: SubtaskLogRequest ----

func TestSubtaskLogRequestValidate_TrimAndLength(t *testing.T) {
	f := func(text string) map[string]string {
		in := SubtaskLogRequest{Text: text}.normalize()
		got := map[string]string{}
		for _, fl := range in.validate() {
			got[fl.Name] = fl.Message
		}
		return got
	}

	assert.Equal(t, msgRequired, f("")["text"])
	assert.Equal(t, msgRequired, f("   ")["text"], "пробелы — trim до пусто")
	assert.Empty(t, f(strings.Repeat("я", subtaskLogTextMax)), "граница — валидно")
	assert.Equal(t, msgTooLong, f(strings.Repeat("я", subtaskLogTextMax+1))["text"])
}
