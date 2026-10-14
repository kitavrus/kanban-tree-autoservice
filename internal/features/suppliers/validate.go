package suppliers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — экран переводит их в текст
// (web/ssr/_shared/js/suppliers.js).
const (
	msgRequired    = "required"
	msgTooLong     = "too_long"
	msgInvalid     = "invalid"
	msgInvalidText = "invalid_text"
	// msgDuplicate — название занято другим поставщиком, включая архивных (DS-007).
	msgDuplicate = "duplicate"
)

// Лимиты длины в символах (§4.1).
const (
	nameMax        = 200
	contactNameMax = 200
	phoneMax       = 50
	addressMax     = 500
	noteMax        = 1000
	queryMax       = 200
)

// Input — нормализованные поля поставщика.
type Input struct {
	Name        string
	ContactName string
	Phone       string
	Address     string
	Note        string
	IsActive    bool
}

func (in Input) normalize() Input {
	in.Name = strings.TrimSpace(in.Name)
	in.ContactName = strings.TrimSpace(in.ContactName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Address = strings.TrimSpace(in.Address)
	in.Note = strings.TrimSpace(in.Note)
	return in
}

// validate — ошибки полей после normalize(); на поле — не больше одной ошибки.
func (in Input) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.Name); {
	case sqlutil.HasInvalidText(in.Name):
		add("name", msgInvalidText)
	case n == 0:
		add("name", msgRequired)
	case n > nameMax:
		add("name", msgTooLong)
	}
	optional := func(name, v string, limit int) {
		switch {
		case sqlutil.HasInvalidText(v):
			add(name, msgInvalidText)
		case utf8.RuneCountInString(v) > limit:
			add(name, msgTooLong)
		}
	}
	optional("contact_name", in.ContactName, contactNameMax)
	switch {
	case sqlutil.HasInvalidText(in.Phone):
		add("phone", msgInvalidText)
	case utf8.RuneCountInString(in.Phone) > phoneMax:
		add("phone", msgTooLong)
	case in.Phone != "" && !hasDigit(in.Phone):
		add("phone", msgInvalid)
	}
	optional("address", in.Address, addressMax)
	optional("note", in.Note, noteMax)
	return f
}

func hasDigit(s string) bool {
	return strings.IndexFunc(s, unicode.IsDigit) >= 0
}
