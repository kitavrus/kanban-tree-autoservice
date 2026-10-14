package equipmenttypes

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

func fieldsOf(fs []errorspkg.Field) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.Name] = f.Message
	}
	return m
}

func TestInput_Normalize(t *testing.T) {
	in := Input{
		Name:     "  Холодильник  ",
		SpecKeys: []string{" Объём ", "объём", "", "Тип хладагента", "ТИП ХЛАДАГЕНТА", "  "},
	}.normalize()
	assert.Equal(t, "Холодильник", in.Name)
	assert.Equal(t, []string{"Объём", "Тип хладагента"}, in.SpecKeys, "trim + дедуп case-insensitive, порядок первого вхождения")
}

// UT-008: name required/100/101.
func TestInput_Validate(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
		msg   string
	}{
		{"валидный", Input{Name: "Дрель"}, "", ""},
		{"имя пусто", Input{Name: ""}, "name", msgRequired},
		{"имя из пробелов — required после trim", Input{Name: "   "}, "name", msgRequired},
		{"имя 100 — граница", Input{Name: strings.Repeat("я", 100)}, "", ""},
		{"имя 101", Input{Name: strings.Repeat("я", 101)}, "name", msgTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fieldsOf(tc.in.normalize().validate())
			if tc.field == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, map[string]string{tc.field: tc.msg}, got)
		})
	}
}

// UT-008: spec_keys trim/дедуп case-insensitive/порядок первого вхождения; 50/51 после дедупа.
func TestInput_Validate_SpecKeys(t *testing.T) {
	keys50 := make([]string, 50)
	for i := range keys50 {
		keys50[i] = "k" + strconv.Itoa(i)
	}
	in := Input{Name: "Дрель", SpecKeys: keys50}.normalize()
	assert.Len(t, in.SpecKeys, 50)
	assert.Empty(t, fieldsOf(in.validate()), "50 уникальных ключей — граница, ok")

	keys51 := append(append([]string{}, keys50...), "extra")
	in = Input{Name: "Дрель", SpecKeys: keys51}.normalize()
	assert.Equal(t, map[string]string{"spec_keys": msgTooMany}, fieldsOf(in.validate()), "51 уникальный после дедупа — too_many")

	// дубли (в т.ч. case-insensitive) не считаются за лишние — 50 уникальных + 20 дублей всё ещё 50.
	dupHeavy := append(append([]string{}, keys50...), keys50...)
	in = Input{Name: "Дрель", SpecKeys: dupHeavy}.normalize()
	assert.Len(t, in.SpecKeys, 50)
	assert.Empty(t, fieldsOf(in.validate()))

	in = Input{Name: "Дрель", SpecKeys: []string{strings.Repeat("k", 100)}}.normalize()
	assert.Empty(t, fieldsOf(in.validate()), "ключ 100 рун — граница")

	in = Input{Name: "Дрель", SpecKeys: []string{strings.Repeat("k", 101)}}.normalize()
	assert.Equal(t, map[string]string{"spec_keys[0]": msgTooLong}, fieldsOf(in.validate()), "ключ 101 руна — too_long")
}

// UT-009: SSR-разбор spec_keys строки через запятую → normalize().
func TestParseSpecKeys(t *testing.T) {
	got := Input{SpecKeys: ParseSpecKeys("a, b,,A ")}.normalize().SpecKeys
	assert.Equal(t, []string{"a", "b"}, got)

	assert.Empty(t, Input{SpecKeys: ParseSpecKeys("")}.normalize().SpecKeys, "пустая строка формы — пустой список")
}
