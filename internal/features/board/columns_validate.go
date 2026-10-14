package board

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — эталон internal/features/clients/validate.go.
const (
	msgRequired      = "required"
	msgTooLong       = "too_long"
	msgInvalid       = "invalid"
	msgInvalidFormat = "invalid_format"
	msgInvalidText   = "invalid_text"
)

// columnNameMax — design-errors.md §Validation Fields (`name` колонки, trim 1..60).
const columnNameMax = 60

// columnColorRe — design-errors.md §Validation Fields / migrations/0005_board CHECK.
var columnColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// normalize — trim name/color; is_final/consumes_stock/position — без нормализации.
func (in ColumnInput) normalize() ColumnInput {
	in.Name = strings.TrimSpace(in.Name)
	in.Color = strings.TrimSpace(in.Color)
	return in
}

// validate — ошибки полей после normalize(). Position == nil допустим (POST → в конец,
// PUT → без перемещения); клэмп явного значения к 1..N — сервис (там известно текущее N
// колонок под FOR UPDATE, design-sql.md §Column Renumber Query).
func (in ColumnInput) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		add("name", msgRequired)
	case n > columnNameMax:
		add("name", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Name) {
		add("name", msgInvalidText)
	}

	if !columnColorRe.MatchString(in.Color) {
		add("color", msgInvalidFormat)
	}

	if in.Position != nil && *in.Position < 1 {
		add("position", msgInvalid)
	}
	return f
}
