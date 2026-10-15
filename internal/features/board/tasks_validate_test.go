package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptrStr(v string) *string { return &v }
func ptrInt64(v int64) *int64 { return &v }

const validUUID = "11111111-1111-1111-1111-111111111111"

// TestTaskCreateRequestValidate_EmptyTitle — UT-002.
func TestTaskCreateRequestValidate_EmptyTitle(t *testing.T) {
	in := TaskHeader{Title: "   ", ColumnID: validUUID}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "title", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

// TestTaskCreateRequestValidate_TitleTooLong — UT-002.
func TestTaskCreateRequestValidate_TitleTooLong(t *testing.T) {
	in := TaskHeader{
		Title: strings.Repeat("я", taskTitleMax+1), ColumnID: validUUID,
	}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "title", fields[0].Name)
	assert.Equal(t, msgTooLong, fields[0].Message)
}

// TestTaskHeaderValidate_MissingColumnID — UT-002. Клиента в шапке нет (one-car-order):
// клиент и автомобиль задаются только при создании (TaskCreateRequest.prepare).
func TestTaskHeaderValidate_MissingColumnID(t *testing.T) {
	in := TaskHeader{Title: "Заказ"}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "column_id", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

func TestTaskHeaderValidate_InvalidColumnID(t *testing.T) {
	in := TaskHeader{Title: "Заказ", ColumnID: "also-not-a-uuid"}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "column_id", fields[0].Name)
	assert.Equal(t, msgInvalid, fields[0].Message)
}

func TestTaskCreateRequestValidate_InvalidAssigneeID(t *testing.T) {
	in := TaskHeader{
		Title: "Заказ", ColumnID: validUUID, AssigneeID: ptrStr("bad"),
	}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "assignee_id", fields[0].Name)
	assert.Equal(t, msgInvalid, fields[0].Message)
}

func TestTaskCreateRequestValidate_InvalidDueDate(t *testing.T) {
	in := TaskHeader{
		Title: "Заказ", ColumnID: validUUID, DueDate: ptrStr("2026-13-40"),
	}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "due_date", fields[0].Name)
	assert.Equal(t, msgInvalidDate, fields[0].Message)
}

func TestTaskCreateRequestValidate_ValidInput(t *testing.T) {
	in := TaskHeader{
		Title: "  Заказ  ", ColumnID: validUUID,
		AssigneeID: ptrStr(validUUID), DueDate: ptrStr("2026-05-01"),
	}.normalize()
	assert.Empty(t, in.validate())
	assert.Equal(t, "Заказ", in.Title, "trim")
}

// TestTaskUpdateRequestValidate_MissingVersion — UT-002.
func TestTaskUpdateRequestValidate_MissingVersion(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: TaskHeader{Title: "Заказ", ColumnID: validUUID},
	}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "version", fields[0].Name)
	assert.Equal(t, msgRequired, fields[0].Message)
}

func TestTaskUpdateRequestValidate_VersionZeroOrNegative(t *testing.T) {
	for _, v := range []int64{0, -1} {
		in := TaskUpdateRequest{
			TaskHeader: TaskHeader{Title: "Заказ", ColumnID: validUUID},
			Version:    ptrInt64(v),
		}.normalize()
		fields := in.validate()
		assert.Len(t, fields, 1, "version=%d", v)
		assert.Equal(t, "version", fields[0].Name)
	}
}

func TestTaskUpdateRequestValidate_ValidInput(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: TaskHeader{Title: "Заказ", ColumnID: validUUID},
		Version:    ptrInt64(1),
	}.normalize()
	assert.Empty(t, in.validate())
}

// TestTaskCreateRequestValidate_DescriptionTooLong — UT-002/AC-002: description >10000
// символов → 400 too_long; ровно 10000 — валидно (граница).
func TestTaskCreateRequestValidate_DescriptionTooLong(t *testing.T) {
	tooLong := ptrStr(strings.Repeat("я", taskDescriptionMax+1))
	in := TaskHeader{
		Title: "Заказ", ColumnID: validUUID, Description: tooLong,
	}.normalize()
	fields := in.validate()
	assert.Len(t, fields, 1)
	assert.Equal(t, "description", fields[0].Name)
	assert.Equal(t, msgTooLong, fields[0].Message)

	atLimit := TaskHeader{
		Title: "Заказ", ColumnID: validUUID,
		Description: ptrStr(strings.Repeat("я", taskDescriptionMax)),
	}.normalize()
	assert.Empty(t, atLimit.validate(), "ровно 10000 символов — валидно")
}

// TestTaskUpdateRequestValidate_DescriptionNilKeepsValue — UT-002/AC-002: description=nil
// не порождает ошибку валидации формата (сервис трактует nil как «не менять», не «пусто»).
func TestTaskUpdateRequestValidate_DescriptionNilKeepsValue(t *testing.T) {
	in := TaskUpdateRequest{
		TaskHeader: TaskHeader{Title: "Заказ", ColumnID: validUUID, Description: nil},
		Version:    ptrInt64(1),
	}.normalize()
	assert.Empty(t, in.validate())
	assert.Nil(t, in.Description)
}
