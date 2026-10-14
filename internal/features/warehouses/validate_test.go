package warehouses

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

func fieldsOf(f []errorspkg.Field) map[string]string {
	out := map[string]string{}
	for _, x := range f {
		out[x.Name] = x.Message
	}
	return out
}

func TestInput_NormalizeValidate(t *testing.T) {
	in := Input{Name: "  Москва  "}.normalize()
	assert.Equal(t, "Москва", in.Name)
	assert.Empty(t, fieldsOf(in.validate()))

	assert.Equal(t, map[string]string{"name": msgRequired}, fieldsOf(Input{Name: "   "}.normalize().validate()))
	assert.Empty(t, fieldsOf(Input{Name: strings.Repeat("я", nameMax)}.normalize().validate()), "граница — валидно")
	assert.Equal(t, map[string]string{"name": msgTooLong}, fieldsOf(Input{Name: strings.Repeat("я", nameMax+1)}.normalize().validate()))
	assert.Equal(t, map[string]string{"name": msgInvalidText}, fieldsOf(Input{Name: "Склад\u0000"}.normalize().validate()))
}

// TS-002: дубль имени — 400 по полю name (taken/archived), не 409.
func TestMapErr_NameConflictsAreFieldValidation(t *testing.T) {
	for err, code := range map[error]string{errNameTaken: msgTaken, errNameArchived: msgArchived} {
		var app *errorspkg.AppError
		assert.True(t, errors.As(mapErr(err), &app))
		assert.Equal(t, 400, app.HTTP)
		assert.Equal(t, map[string]string{"name": code}, fieldsOf(app.Fields))
	}
	assert.ErrorIs(t, mapErr(ErrNotFound), ErrWarehouseNotFound)
}

func TestFieldText(t *testing.T) {
	assert.Equal(t, "Такой склад уже есть", fieldText(msgTaken))
	assert.Contains(t, fieldText(msgArchived), "архив")
	assert.Equal(t, "Обязательное поле", fieldText(msgRequired))
	assert.Equal(t, "Недопустимое значение", fieldText("whatever"))
}

// soft-delete-warehouses-suppliers §6.1, §8.1 (TS-031): три отказа WAREHOUSE_IN_USE при удалении
// различаются сообщением (что именно мешает), код и статус у них общие; у «последнего активного»
// свой код. Ключ ?error= серверной формы ведёт к тому же тексту, что отдаёт API.
func TestDeleteRefusals_DistinctMessages(t *testing.T) {
	inUse := []*errorspkg.AppError{ErrDeleteHasStock, ErrDeleteHasEquipment, ErrDeleteHasTransfers}
	seen := map[string]bool{ErrInUse.Message: true} // сообщение архивирования — отдельное
	for _, e := range inUse {
		assert.Equal(t, "WAREHOUSE_IN_USE", e.Code)
		assert.Equal(t, 409, e.HTTP)
		assert.False(t, seen[e.Message], "сообщение %q повторяется", e.Message)
		seen[e.Message] = true
	}
	assert.Equal(t, "WAREHOUSE_LAST_ACTIVE", ErrDeleteLastActive.Code)
	assert.NotEqual(t, ErrLastActive.Message, ErrDeleteLastActive.Message, "удаление и архив — разные тексты")

	for key, want := range map[string]*errorspkg.AppError{
		"last_active": ErrLastActive, "in_use": ErrInUse, "delete_stock": ErrDeleteHasStock,
		"delete_equipment": ErrDeleteHasEquipment, "delete_transfers": ErrDeleteHasTransfers, "delete_last_active": ErrDeleteLastActive,
	} {
		assert.Equal(t, want.Message, listErrors[key], key)
	}
	assert.Empty(t, listErrors["unknown"], "неизвестный ключ — без блока ошибки")
}
