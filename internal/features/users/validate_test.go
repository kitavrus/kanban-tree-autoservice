package users

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

func valid() Input {
	return Input{Login: "sidorov", Name: "Сидоров Пётр", Role: RoleExecutor, Phone: "+7 (900) 111-22-33", Password: "secret123", IsActive: true}
}

func fieldsOf(fs []errorspkg.Field) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.Name] = f.Message
	}
	return m
}

func TestInput_Normalize(t *testing.T) {
	in := Input{Login: "  Sidorov.P ", Name: "  Пётр  ", Role: " admin ", Phone: " 123 "}.normalize()
	assert.Equal(t, "sidorov.p", in.Login)
	assert.Equal(t, "Пётр", in.Name)
	assert.Equal(t, RoleAdmin, in.Role)
	assert.Equal(t, "123", in.Phone)
}

func TestInput_Validate_OK(t *testing.T) {
	assert.Empty(t, valid().validate(true))
	in := valid()
	in.Password = ""
	assert.Empty(t, in.validate(false), "при правке пустой пароль — не менять")
	in.Phone = ""
	assert.Empty(t, in.validate(false), "телефон необязателен")
}

func TestInput_Validate_Fields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Input)
		create bool
		field  string
		msg    string
	}{
		{"логин пуст", func(i *Input) { i.Login = "" }, true, "login", msgRequired},
		{"логин 2 символа", func(i *Input) { i.Login = "ab" }, true, "login", msgTooShort},
		{"логин 3 символа — граница", func(i *Input) { i.Login = "abc" }, true, "", ""},
		{"логин 50 — граница", func(i *Input) { i.Login = strings.Repeat("a", 50) }, true, "", ""},
		{"логин 51", func(i *Input) { i.Login = strings.Repeat("a", 51) }, true, "login", msgTooLong},
		{"логин кириллица", func(i *Input) { i.Login = "сидоров" }, true, "login", msgInvalidFormat},
		{"логин с пробелом", func(i *Input) { i.Login = "a b" }, true, "login", msgInvalidFormat},
		{"логин эмодзи", func(i *Input) { i.Login = "abc😀" }, true, "login", msgInvalidFormat},
		{"ФИО пусто", func(i *Input) { i.Name = "" }, true, "name", msgRequired},
		{"ФИО 200 — граница", func(i *Input) { i.Name = strings.Repeat("я", 200) }, true, "", ""},
		{"ФИО 201", func(i *Input) { i.Name = strings.Repeat("я", 201) }, true, "name", msgTooLong},
		{"ФИО эмодзи допустимы", func(i *Input) { i.Name = "Пётр 🔧" }, true, "", ""},
		{"роль мусор", func(i *Input) { i.Role = "root" }, true, "role", msgInvalid},
		{"роль пусто", func(i *Input) { i.Role = "" }, true, "role", msgInvalid},
		{"телефон буквы", func(i *Input) { i.Phone = "8-900-abc" }, true, "phone", msgInvalidFormat},
		{"телефон 51", func(i *Input) { i.Phone = strings.Repeat("1", 51) }, true, "phone", msgTooLong},
		{"пароль пуст при создании", func(i *Input) { i.Password = "" }, true, "password", msgRequired},
		{"пароль 7", func(i *Input) { i.Password = "1234567" }, true, "password", msgTooShort},
		{"пароль 8 — граница", func(i *Input) { i.Password = "12345678" }, true, "", ""},
		{"пароль 7 при правке", func(i *Input) { i.Password = "1234567" }, false, "password", msgTooShort},
		{"пароль 72 байта — граница", func(i *Input) { i.Password = strings.Repeat("x", 72) }, true, "", ""},
		{"пароль 73 байта", func(i *Input) { i.Password = strings.Repeat("x", 73) }, true, "password", msgTooLong},
		{"пароль 36 кириллических = 72 байта", func(i *Input) { i.Password = strings.Repeat("п", 36) }, true, "", ""},
		{"пароль 37 кириллических = 74 байта", func(i *Input) { i.Password = strings.Repeat("п", 37) }, true, "password", msgTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid()
			tc.mutate(&in)
			got := fieldsOf(in.normalize().validate(tc.create))
			if tc.field == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, map[string]string{tc.field: tc.msg}, got)
		})
	}
}

func TestInput_Validate_AllAtOnce(t *testing.T) {
	got := fieldsOf(Input{}.validate(true))
	assert.Equal(t, map[string]string{"login": msgRequired, "name": msgRequired, "role": msgInvalid, "password": msgRequired}, got)
}

func TestFieldText(t *testing.T) {
	assert.Equal(t, "Минимум 8 символов", fieldText("password", msgTooShort))
	assert.Equal(t, "Минимум 3 символа", fieldText("login", msgTooShort))
	assert.Contains(t, fieldText("login", msgInvalidFormat), "латинские")
	assert.Contains(t, fieldText("phone", msgInvalidFormat), "цифры")
	assert.Equal(t, "Недопустимое значение", fieldText("role", msgInvalid))
	assert.Contains(t, fieldText("password", msgTooLong), "72")
}

func TestListVM_FilterURL(t *testing.T) {
	var vm ListVM
	assert.Equal(t, "/users", vm.FilterURL("", "", ""))
	assert.Equal(t, "/users?q=%D0%BF%D1%91%D1%82%D1%80&role=admin&status=active", vm.FilterURL("пётр", "admin", "active"))
}
