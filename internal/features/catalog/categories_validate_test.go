package catalog

import (
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

func TestCategoryInput_Normalize(t *testing.T) {
	in := CategoryInput{Kind: "  product  ", Name: "  Фильтры  "}.normalize()
	assert.Equal(t, "product", in.Kind)
	assert.Equal(t, "Фильтры", in.Name)
}

func TestCategoryInput_ValidateCreate(t *testing.T) {
	cases := []struct {
		name  string
		in    CategoryInput
		field string
		msg   string
	}{
		{"валидный product", CategoryInput{Kind: KindProduct, Name: "Расходники"}, "", ""},
		{"валидный service", CategoryInput{Kind: KindService, Name: "Ремонт"}, "", ""},
		{"kind пуст", CategoryInput{Kind: "", Name: "X"}, "kind", msgInvalid},
		{"kind мусор", CategoryInput{Kind: "junk", Name: "X"}, "kind", msgInvalid},
		{"имя пусто", CategoryInput{Kind: KindProduct, Name: ""}, "name", msgRequired},
		{"имя из пробелов", CategoryInput{Kind: KindProduct, Name: "   "}, "name", msgRequired},
		{"имя 100 — граница", CategoryInput{Kind: KindProduct, Name: strings.Repeat("я", 100)}, "", ""},
		{"имя 101", CategoryInput{Kind: KindProduct, Name: strings.Repeat("я", 101)}, "name", msgTooLong},
		{"NUL-байт", CategoryInput{Kind: KindProduct, Name: "X\u0000"}, "name", msgInvalidText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fieldsOf(tc.in.normalize().validateCreate())
			if tc.field == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.msg, got[tc.field])
		})
	}
}

func TestValidateCategoryName(t *testing.T) {
	assert.Empty(t, fieldsOf(validateCategoryName("Название")))
	assert.Equal(t, map[string]string{"name": msgRequired}, fieldsOf(validateCategoryName("")))
	assert.Equal(t, map[string]string{"name": msgTooLong}, fieldsOf(validateCategoryName(strings.Repeat("a", 101))))
}
