package equipmenttypes

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — SSR переводит их в текст.
const (
	msgRequired = "required"
	msgTooLong  = "too_long"
	msgTooMany  = "too_many"
	// msgInvalidText — NUL-байт / невалидный UTF-8 (code review §TD-4, «мусор → 400, не 500»).
	msgInvalidText = "invalid_text"
)

// Границы полей (Tech Spec §Лимиты).
const (
	nameMax     = 100
	specKeyMax  = 100
	specKeysMax = 50
)

// Input — нормализованные поля типа оборудования (общие для API и SSR-формы).
type Input struct {
	Name     string
	SpecKeys []string
	IsActive bool
}

// normalize — trim name; spec_keys: trim, пустые и дубли (регистронезависимо)
// отбрасываются молча, порядок — первого вхождения.
func (in Input) normalize() Input {
	in.Name = strings.TrimSpace(in.Name)
	seen := make(map[string]bool, len(in.SpecKeys))
	keys := make([]string, 0, len(in.SpecKeys))
	for _, k := range in.SpecKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		low := strings.ToLower(k)
		if seen[low] {
			continue
		}
		seen[low] = true
		keys = append(keys, k)
	}
	in.SpecKeys = keys
	return in
}

// validate — ошибки полей после normalize().
func (in Input) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		add("name", msgRequired)
	case n > nameMax:
		add("name", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Name) {
		add("name", msgInvalidText)
	}
	if len(in.SpecKeys) > specKeysMax {
		add("spec_keys", msgTooMany)
	}
	for i, k := range in.SpecKeys {
		if utf8.RuneCountInString(k) > specKeyMax {
			add(fmt.Sprintf("spec_keys[%d]", i), msgTooLong)
		}
		if sqlutil.HasInvalidText(k) {
			add(fmt.Sprintf("spec_keys[%d]", i), msgInvalidText)
		}
	}
	return f
}

// ParseSpecKeys — SSR-форма типа: одна строка через запятую → сырой список
// (дедуп/trim делает Input.normalize()). Эталон прототипа.
func ParseSpecKeys(raw string) []string {
	return strings.Split(raw, ",")
}
