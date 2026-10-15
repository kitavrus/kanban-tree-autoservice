package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptrStr(v string) *string { return &v }
func ptrInt64(v int64) *int64 { return &v }

const validUUID = "11111111-1111-1111-1111-111111111111"

// withPlan — шапка с валидным планом работ (order-start-duration: start_date и duration_days
// обязательны), чтобы проверки остальных полей видели только свои ошибки.
func withPlan(h TaskHeader) TaskHeader {
	if h.StartDate == nil {
		h.StartDate = ptrStr("2026-10-15")
	}
	if h.DurationDays == nil {
		h.DurationDays = ptrInt(3)
	}
	return h
}

// fieldsOf — ошибки валидации шапки как name → message.
func fieldsOf(in TaskHeader) map[string]string {
	got := map[string]string{}
	for _, f := range in.normalize().validate() {
		got[f.Name] = f.Message
	}
	return got
}

// TestTaskCreateRequestValidate_EmptyTitle — UT-002.
func TestTaskCreateRequestValidate_EmptyTitle(t *testing.T) {
	in := withPlan(TaskHeader{Title: "   ", ColumnID: validUUID}).normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "title", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

// TestTaskCreateRequestValidate_TitleTooLong — UT-002.
func TestTaskCreateRequestValidate_TitleTooLong(t *testing.T) {
	in := withPlan(TaskHeader{
		Title: strings.Repeat("я", taskTitleMax+1), ColumnID: validUUID,
	}).normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "title", fields[0].Name)
	assert.Equal(t, msgTooLong, fields[0].Message)
}

// TestTaskHeaderValidate_MissingColumnID — UT-002. Клиента в шапке нет (one-car-order):
// клиент и автомобиль задаются только при создании (TaskCreateRequest.prepare).
func TestTaskHeaderValidate_MissingColumnID(t *testing.T) {
	in := withPlan(TaskHeader{Title: "Заказ"}).normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "column_id", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

func TestTaskHeaderValidate_InvalidColumnID(t *testing.T) {
	in := withPlan(TaskHeader{Title: "Заказ", ColumnID: "also-not-a-uuid"}).normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "column_id", fields[0].Name)
	assert.Equal(t, msgInvalid, fields[0].Message)
}

func TestTaskCreateRequestValidate_InvalidAssigneeID(t *testing.T) {
	in := withPlan(TaskHeader{
		Title: "Заказ", ColumnID: validUUID, AssigneeID: ptrStr("bad"),
	}).normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "assignee_id", fields[0].Name)
	assert.Equal(t, msgInvalid, fields[0].Message)
}

// TestTaskHeaderValidate_PlanRequired — order-start-duration: без start_date/duration_days
// (нет в теле или пустая дата) — required по каждому полю.
func TestTaskHeaderValidate_PlanRequired(t *testing.T) {
	got := fieldsOf(TaskHeader{Title: "Заказ", ColumnID: validUUID})
	assert.Equal(t, map[string]string{"start_date": msgRequired, "duration_days": msgRequired}, got)

	got = fieldsOf(TaskHeader{Title: "Заказ", ColumnID: validUUID, StartDate: ptrStr("   "), DurationDays: ptrInt(1)})
	assert.Equal(t, map[string]string{"start_date": msgRequired}, got, "пустая дата после trim")
}

// TestTaskHeaderValidate_StartDate — формат YYYY-MM-DD, существующая дата, окно 2000..2099.
func TestTaskHeaderValidate_StartDate(t *testing.T) {
	cases := []struct {
		start, want string
	}{
		{"2026-13-40", msgInvalidDate},
		{"2026-02-30", msgInvalidDate},
		{"15.10.2026", msgInvalidDate},
		{"2026-10-15T00:00:00Z", msgInvalidDate},
		{"0000-01-01", msgOutOfRange}, // год 0 — в PostgreSQL date нет (раньше — 500)
		{"1999-12-31", msgOutOfRange},
		{"2100-01-01", msgOutOfRange},
		{"2000-01-01", ""},
		{"2099-12-31", ""},
		{" 2026-10-15 ", ""},
	}
	for _, c := range cases {
		got := fieldsOf(withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID, StartDate: ptrStr(c.start)}))
		if c.want == "" {
			assert.Empty(t, got, "start_date=%q", c.start)
			continue
		}
		assert.Equal(t, map[string]string{"start_date": c.want}, got, "start_date=%q", c.start)
	}
}

// TestTaskHeaderValidate_DurationDays — дней на работу 1..60 (границы включительно).
func TestTaskHeaderValidate_DurationDays(t *testing.T) {
	for _, d := range []int{taskDurationMin - 1, -1, taskDurationMax + 1, 1000} {
		got := fieldsOf(withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID, DurationDays: ptrInt(d)}))
		assert.Equal(t, map[string]string{"duration_days": msgOutOfRange}, got, "duration_days=%d", d)
	}
	for _, d := range []int{taskDurationMin, 7, taskDurationMax} {
		assert.Empty(t, fieldsOf(withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID, DurationDays: ptrInt(d)})), "duration_days=%d", d)
	}
}

func TestTaskCreateRequestValidate_ValidInput(t *testing.T) {
	in := TaskHeader{
		Title: "  Заказ  ", ColumnID: validUUID,
		AssigneeID: ptrStr(validUUID), StartDate: ptrStr(" 2026-05-01 "), DurationDays: ptrInt(5),
	}.normalize()
	assert.Empty(t, in.validate())
	assert.Equal(t, "Заказ", in.Title, "trim")
	assert.Equal(t, "2026-05-01", *in.StartDate, "trim даты")
}

// TestTaskUpdateRequestValidate_MissingVersion — UT-002.
func TestTaskUpdateRequestValidate_MissingVersion(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID}),
	}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "version", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

func TestTaskUpdateRequestValidate_VersionZeroOrNegative(t *testing.T) {
	for _, v := range []int64{0, -1} {
		in := TaskUpdateRequest{
			TaskHeader: withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID}),
			Version:    ptrInt64(v),
		}.normalize()
		fields := in.validate()
		assert.Len(t, fields, 1, "version=%d", v)
		assert.Equal(t, "version", fields[0].Name)
	}
}

func TestTaskUpdateRequestValidate_ValidInput(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID}),
		Version:    ptrInt64(1),
	}.normalize()
	assert.Empty(t, in.validate())
}

// TestTaskUpdateRequestValidate_PlanRequired — PUT — полная замена шапки: план обязателен и здесь.
func TestTaskUpdateRequestValidate_PlanRequired(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: TaskHeader{Title: "Заказ", ColumnID: validUUID, DurationDays: ptrInt(61)},
		Version:    ptrInt64(1),
	}.normalize()
	got := map[string]string{}
	for _, f := range in.validate() {
		got[f.Name] = f.Message
	}
	assert.Equal(t, map[string]string{"start_date": msgRequired, "duration_days": msgOutOfRange}, got)
}

// TestTaskCreateRequestValidate_DescriptionTooLong — UT-002/AC-002: description >10000
// символов → 400 too_long; ровно 10000 — валидно (граница).
func TestTaskCreateRequestValidate_DescriptionTooLong(t *testing.T) {
	tooLong := ptrStr(strings.Repeat("я", taskDescriptionMax+1))
	in := withPlan(TaskHeader{
		Title: "Заказ", ColumnID: validUUID, Description: tooLong,
	}).normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "description", fields[0].Name)
	assert.Equal(t, msgTooLong, fields[0].Message)

	atLimit := withPlan(TaskHeader{
		Title: "Заказ", ColumnID: validUUID,
		Description: ptrStr(strings.Repeat("я", taskDescriptionMax)),
	}).normalize()
	assert.Empty(t, atLimit.validate(), "ровно 10000 символов — валидно")
}

// TestTaskUpdateRequestValidate_DescriptionNilKeepsValue — UT-002/AC-002: description=nil
// не порождает ошибку валидации формата (сервис трактует nil как «не менять», не «пусто»).
func TestTaskUpdateRequestValidate_DescriptionNilKeepsValue(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: withPlan(TaskHeader{Title: "Заказ", ColumnID: validUUID, Description: nil}),
		Version:    ptrInt64(1),
	}.normalize()
	assert.Empty(t, in.validate())
	assert.Nil(t, in.Description)
}
