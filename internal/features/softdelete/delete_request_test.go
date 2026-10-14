package softdelete

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// DS-028: тело DELETE разбирается как JSON независимо от Content-Type; пустое, не-JSON и тело без
// reason дают «причины нет» (nil) — валидатор ответит required, а не 400 «некорректное тело».
func TestParseDeleteRequest(t *testing.T) {
	cases := []struct {
		name, body string
		want       *string
	}{
		{"пустое тело", "", nil},
		{"пробелы", " \n\t ", nil},
		{"не JSON", "{", nil},
		{"JSON не объект", `"строка"`, nil},
		{"нет поля", `{"note":"x"}`, nil},
		{"null", `{"reason":null}`, nil},
		{"reason не строка", `{"reason":5}`, nil},
		{"причина", `{"reason":"Создан по ошибке"}`, ptr("Создан по ошибке")},
		{"пробелы сохраняются до валидатора", ` {"reason":"  x  "} `, ptr("  x  ")},
		{"лишние поля игнорируются", `{"reason":"x","version":3}`, ptr("x")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseDeleteRequest([]byte(c.body)).Reason
			if c.want == nil {
				assert.Nil(t, got)
				return
			}
			if assert.NotNil(t, got) {
				assert.Equal(t, *c.want, *got)
			}
		})
	}
}

// DS-032: текст под полем «Причина» серверной формы удаления — по коду ошибки поля reason;
// отказ не по причине (другое поле, 409, не AppError) — пустая строка: форма покажет общий блок.
func TestReasonError(t *testing.T) {
	texts := map[string]string{
		"пусто":      "Укажите причину удаления",
		"длинная":    "Причина — не длиннее 500 символов",
		"NUL-символ": "Недопустимые символы в причине",
	}
	inputs := map[string]string{"пусто": "  ", "длинная": strings.Repeat("я", ReasonMax+1), "NUL-символ": "x\x00"}
	for name, in := range inputs {
		_, fields := ValidateReason(&in)
		assert.Equal(t, texts[name], ReasonError(errorspkg.Validation(fields...)), name)
	}
	assert.Equal(t, "Недопустимое значение", ReasonError(errorspkg.Validation(errorspkg.Field{Name: "reason", Message: "unknown_code"})))
	assert.Empty(t, ReasonError(errorspkg.Validation(errorspkg.Field{Name: "name", Message: "required"})), "ошибка другого поля")
	assert.Empty(t, ReasonError(ErrNameTaken), "отказ 409 — не по причине")
	assert.Empty(t, ReasonError(errors.New("сбой")), "не AppError")
	assert.Empty(t, ReasonError(nil))
}

// DS-017: NAME_TAKEN — 409 с текстом, который экран «Удалённые» показывает тостом как есть.
func TestErrNameTaken(t *testing.T) {
	assert.Equal(t, "NAME_TAKEN", ErrNameTaken.Code)
	assert.Equal(t, 409, ErrNameTaken.HTTP)
	assert.Equal(t, "Название занято — переименуйте действующую запись", ErrNameTaken.Message)
}
