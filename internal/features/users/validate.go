package users

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// Роли (plan.md §2).
const (
	RoleAdmin    = "admin"
	RoleExecutor = "executor"
)

// Коды ошибок полей (details.fields[].message) — клиент переводит их в текст.
const (
	msgRequired      = "required"
	msgTooShort      = "too_short"
	msgTooLong       = "too_long"
	msgInvalidFormat = "invalid_format"
	msgInvalid       = "invalid"
)

// Границы полей.
const (
	loginMin    = 3
	loginMax    = 50
	nameMax     = 200
	phoneMax    = 50
	passwordMin = 8
	// passwordMaxBytes — предел bcrypt: длиннее 72 БАЙТ он отказывается хешировать
	// (кириллица — 2 байта на символ, т.е. ~36 символов).
	passwordMaxBytes = 72
)

var (
	loginRe = regexp.MustCompile(`^[a-z0-9._-]+$`)
	phoneRe = regexp.MustCompile(`^[0-9+()\- ]*$`)
)

// Input — нормализованные поля карточки сотрудника (общие для API и SSR-формы).
type Input struct {
	Login    string
	Name     string
	Role     string
	Phone    string
	Password string // пусто при правке — не менять
	IsActive bool
	// Timezone — имя IANA; nil — не передан: при создании пояс по умолчанию, при правке
	// колонка не трогается (DS-056).
	Timezone *string
}

// normalize — trim, логин в нижний регистр (вход регистронезависимый).
func (in Input) normalize() Input {
	in.Login = strings.ToLower(strings.TrimSpace(in.Login))
	in.Name = strings.TrimSpace(in.Name)
	in.Role = strings.TrimSpace(in.Role)
	in.Phone = strings.TrimSpace(in.Phone)
	return in
}

// validate — ошибки полей; passwordRequired — при создании пароль обязателен.
func (in Input) validate(passwordRequired bool) []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.Login); {
	case n == 0:
		add("login", msgRequired)
	case n < loginMin:
		add("login", msgTooShort)
	case n > loginMax:
		add("login", msgTooLong)
	case !loginRe.MatchString(in.Login):
		add("login", msgInvalidFormat)
	}
	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		add("name", msgRequired)
	case n > nameMax:
		add("name", msgTooLong)
	}
	if in.Role != RoleAdmin && in.Role != RoleExecutor {
		add("role", msgInvalid)
	}
	switch {
	case utf8.RuneCountInString(in.Phone) > phoneMax:
		add("phone", msgTooLong)
	case !phoneRe.MatchString(in.Phone):
		add("phone", msgInvalidFormat)
	}
	switch n := utf8.RuneCountInString(in.Password); {
	case n == 0 && passwordRequired:
		add("password", msgRequired)
	case n > 0 && n < passwordMin:
		add("password", msgTooShort)
	case len(in.Password) > passwordMaxBytes:
		add("password", msgTooLong)
	}
	if in.Timezone != nil && !timezone.Valid(*in.Timezone) {
		add("timezone", msgInvalid)
	}
	return f
}

// validateTimezone — свой пояс (PUT /users/me/timezone): пусто → required, не имя IANA → invalid.
func validateTimezone(tz string) []errorspkg.Field {
	switch {
	case tz == "":
		return []errorspkg.Field{{Name: "timezone", Message: msgRequired}}
	case !timezone.Valid(tz):
		return []errorspkg.Field{{Name: "timezone", Message: msgInvalid}}
	}
	return nil
}
