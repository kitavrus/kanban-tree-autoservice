package warehouses

import (
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — SSR переводит их в текст.
const (
	msgRequired    = "required"
	msgTooLong     = "too_long"
	msgInvalidText = "invalid_text"
	// msgTaken — имя занято активным складом; msgArchived — архивным (DS-001: уникальность
	// включает архивные, ошибка — 400 по полю name, TS-002).
	msgTaken    = "taken"
	msgArchived = "archived"
)

const nameMax = 100

// Input — нормализованные поля склада (общие для API и SSR-формы).
type Input struct {
	Name     string
	IsActive bool
}

func (in Input) normalize() Input {
	in.Name = strings.TrimSpace(in.Name)
	return in
}

// validate — ошибки полей после normalize().
func (in Input) validate() []errorspkg.Field {
	var f []errorspkg.Field
	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		f = append(f, errorspkg.Field{Name: "name", Message: msgRequired})
	case n > nameMax:
		f = append(f, errorspkg.Field{Name: "name", Message: msgTooLong})
	}
	if sqlutil.HasInvalidText(in.Name) {
		f = append(f, errorspkg.Field{Name: "name", Message: msgInvalidText})
	}
	return f
}
