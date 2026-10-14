package suppliers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

func check(in Input) map[string]string { return fieldsOf(in.normalize().validate()) }

// TS-020: name required / 200 символов после trim.
func TestTS020_Input_Name(t *testing.T) {
	in := Input{Name: "  ООО Картридж  ", ContactName: " Иван ", Phone: " +7 700 ", Address: " Алматы ", Note: " прим "}.normalize()
	assert.Equal(t, Input{Name: "ООО Картридж", ContactName: "Иван", Phone: "+7 700", Address: "Алматы", Note: "прим"}, in, "trim всех полей")
	assert.Empty(t, fieldsOf(in.validate()))

	assert.Equal(t, map[string]string{"name": msgRequired}, check(Input{}))
	assert.Equal(t, map[string]string{"name": msgRequired}, check(Input{Name: "   "}))
	assert.Empty(t, check(Input{Name: strings.Repeat("я", nameMax)}), "граница 200 — валидно")
	assert.Empty(t, check(Input{Name: " " + strings.Repeat("я", nameMax) + " "}), "длина считается после trim")
	assert.Equal(t, map[string]string{"name": msgTooLong}, check(Input{Name: strings.Repeat("я", nameMax+1)}))
	assert.Equal(t, map[string]string{"name": msgInvalidText}, check(Input{Name: "Поставщик\u0000"}))
}

// TS-020: phone — строка; без цифр → invalid, 51 символ → too_long, пустой — допустим.
func TestTS020_Input_Phone(t *testing.T) {
	for _, ok := range []string{"", "   ", "+7 (700) 123-45-67", "8", "доб. 5", strings.Repeat("1", phoneMax)} {
		assert.Empty(t, check(Input{Name: "X", Phone: ok}), "phone %q", ok)
	}
	for _, bad := range []string{"нет", "+-()", "abc"} {
		assert.Equal(t, map[string]string{"phone": msgInvalid}, check(Input{Name: "X", Phone: bad}), "phone %q", bad)
	}
	assert.Equal(t, map[string]string{"phone": msgTooLong}, check(Input{Name: "X", Phone: strings.Repeat("1", phoneMax+1)}))
	assert.Equal(t, map[string]string{"phone": msgTooLong}, check(Input{Name: "X", Phone: strings.Repeat("я", phoneMax+1)}),
		"длина важнее отсутствия цифр")
	assert.Equal(t, map[string]string{"phone": msgInvalidText}, check(Input{Name: "X", Phone: "1\u0000"}))
}

// TS-020: длины contact_name (200), address (500), note (1000) — граница валидна, +1 → too_long.
func TestTS020_Input_OptionalLengths(t *testing.T) {
	cases := []struct {
		field string
		limit int
		set   func(*Input, string)
	}{
		{"contact_name", contactNameMax, func(in *Input, v string) { in.ContactName = v }},
		{"address", addressMax, func(in *Input, v string) { in.Address = v }},
		{"note", noteMax, func(in *Input, v string) { in.Note = v }},
	}
	for _, c := range cases {
		in := Input{Name: "X"}
		c.set(&in, strings.Repeat("я", c.limit))
		assert.Empty(t, check(in), "%s: граница %d — валидно", c.field, c.limit)

		c.set(&in, strings.Repeat("я", c.limit+1))
		assert.Equal(t, map[string]string{c.field: msgTooLong}, check(in), "%s: %d символ", c.field, c.limit+1)

		c.set(&in, "текст\u0000")
		assert.Equal(t, map[string]string{c.field: msgInvalidText}, check(in), "%s: NUL", c.field)
	}

	// Все поля невалидны разом — по одной ошибке на поле.
	all := Input{
		ContactName: strings.Repeat("я", contactNameMax+1), Phone: "нет",
		Address: strings.Repeat("я", addressMax+1), Note: strings.Repeat("я", noteMax+1),
	}
	assert.Equal(t, map[string]string{
		"name": msgRequired, "contact_name": msgTooLong, "phone": msgInvalid, "address": msgTooLong, "note": msgTooLong,
	}, check(all))
}

// TS-020: дубль названия — 400 по полю name (duplicate), не 409; нет строки → SUPPLIER_NOT_FOUND.
func TestTS020_MapErr(t *testing.T) {
	var app *errorspkg.AppError
	assert.True(t, errors.As(mapErr(errNameDuplicate), &app))
	assert.Equal(t, 400, app.HTTP)
	assert.Equal(t, map[string]string{"name": msgDuplicate}, fieldsOf(app.Fields))

	assert.ErrorIs(t, mapErr(ErrNotFound), ErrSupplierNotFound)
	assert.Equal(t, 404, ErrSupplierNotFound.HTTP)
	assert.Equal(t, "SUPPLIER_NOT_FOUND", ErrSupplierNotFound.Code)

	assert.ErrorIs(t, mapNameConflict(&pgconn.PgError{Code: "23505"}), errNameDuplicate)
	other := &pgconn.PgError{Code: "23514"}
	assert.Equal(t, error(other), mapNameConflict(other), "прочие ошибки БД не маскируются")
}

// TS-020: фильтры списка валидируются до обращения к БД — is_active=maybe → 400
// is_active: invalid; q длиннее 200 → 400; не-uuid id в PUT → 404 (сервис с nil-репозиторием
// до БД не доходит).
func TestTS020_Service_RejectsBeforeDB(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	var app *errorspkg.AppError
	_, err := svc.List(ctx, ListQuery{IsActive: "maybe"})
	assert.True(t, errors.As(err, &app))
	assert.Equal(t, 400, app.HTTP)
	assert.Equal(t, map[string]string{"is_active": msgInvalid}, fieldsOf(app.Fields))

	for _, v := range []string{"1", "0", "TRUE", "yes"} {
		_, err = svc.List(ctx, ListQuery{IsActive: v})
		assert.True(t, errors.As(err, &app), "is_active=%q", v)
		assert.Equal(t, map[string]string{"is_active": msgInvalid}, fieldsOf(app.Fields), "is_active=%q", v)
	}

	_, err = svc.List(ctx, ListQuery{Q: strings.Repeat("я", queryMax+1)})
	assert.True(t, errors.As(err, &app))
	assert.Equal(t, map[string]string{"q": msgTooLong}, fieldsOf(app.Fields))

	_, err = svc.Update(ctx, "not-a-uuid", Input{Name: "X", IsActive: true})
	assert.ErrorIs(t, err, ErrSupplierNotFound)

	_, err = svc.Create(ctx, Input{Name: "   "})
	assert.True(t, errors.As(err, &app))
	assert.Equal(t, map[string]string{"name": msgRequired}, fieldsOf(app.Fields))
}
