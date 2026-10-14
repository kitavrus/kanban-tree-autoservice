package catalog

import (
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — SSR переводит их в текст.
const (
	msgRequired = "required"
	msgTooLong  = "too_long"
	msgInvalid  = "invalid"
	// msgNotFound — выбранная запись каталога мягко удалена (DS-014).
	msgNotFound    = "not_found"
	msgInvalidText = "invalid_text"
)

// KindService — вид категорий каталога (D16): категории есть только у услуг.
const KindService = "service"

const categoryNameMax = 100

// CategoryInput — нормализованные поля категории.
type CategoryInput struct {
	Kind string
	Name string
}

func (in CategoryInput) normalize() CategoryInput {
	in.Kind = strings.TrimSpace(in.Kind)
	in.Name = strings.TrimSpace(in.Name)
	return in
}

// validateCreate — kind + name.
func (in CategoryInput) validateCreate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	if in.Kind != KindService {
		add("kind", msgInvalid)
	}
	f = append(f, validateCategoryName(in.Name)...)
	return f
}

// validateRename — только name (PUT/rename).
func validateCategoryName(name string) []errorspkg.Field {
	var f []errorspkg.Field
	add := func(fld, msg string) { f = append(f, errorspkg.Field{Name: fld, Message: msg}) }
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		add("name", msgRequired)
	case n > categoryNameMax:
		add("name", msgTooLong)
	}
	if sqlutil.HasInvalidText(name) {
		add("name", msgInvalidText)
	}
	return f
}
