package clients

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

func onePhone(phone, note string) []PhoneDTO { return []PhoneDTO{{Phone: phone, Note: note}} }

// UT-001: ClientInput: trim; company_name пусто → required, 200/201 rune (кириллица) →
// ok/too_long.
func TestClientInput_Validate_StringLimits(t *testing.T) {
	cases := []struct {
		name  string
		in    ClientInput
		field string
		msg   string
	}{
		{"валидный", ClientInput{CompanyName: "ООО Ромашка", Phones: onePhone("+7 999 1234567", "")}, "", ""},
		{"company_name пусто", ClientInput{Phones: onePhone("123", "")}, "company_name", msgRequired},
		{"company_name из пробелов — required после trim", ClientInput{CompanyName: "   ", Phones: onePhone("123", "")}, "company_name", msgRequired},
		{"company_name 200 — граница", ClientInput{CompanyName: strings.Repeat("я", 200), Phones: onePhone("123", "")}, "", ""},
		{"company_name 201", ClientInput{CompanyName: strings.Repeat("я", 201), Phones: onePhone("123", "")}, "company_name", msgTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fieldsOf(tc.in.normalize().validate())
			if tc.field == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.msg, got[tc.field])
		})
	}
	// trim компании — пробелы вокруг не считаются частью значения.
	in := ClientInput{CompanyName: "  ООО Ромашка  ", Phones: onePhone("123", "")}.normalize()
	assert.Equal(t, "ООО Ромашка", in.CompanyName)
}

// UT-002: phones: 0 после отброса → phones required; 20 ok / 21 too_many.
func TestClientInput_Validate_PhonesCount(t *testing.T) {
	in := ClientInput{CompanyName: "ООО"}.normalize()
	assert.Equal(t, map[string]string{"phones": msgRequired}, fieldsOf(in.validate()))

	// все строки пустые (phone+note) — отбрасываются normalize(), остаётся 0 → required.
	in = ClientInput{CompanyName: "ООО", Phones: []PhoneDTO{{}, {Phone: "  ", Note: " "}}}.normalize()
	assert.Empty(t, in.Phones)
	assert.Equal(t, map[string]string{"phones": msgRequired}, fieldsOf(in.validate()))

	phones20 := make([]PhoneDTO, 20)
	for i := range phones20 {
		phones20[i] = PhoneDTO{Phone: "1234567"}
	}
	in = ClientInput{CompanyName: "ООО", Phones: phones20}.normalize()
	assert.Len(t, in.Phones, 20)
	assert.Empty(t, fieldsOf(in.validate()), "20 телефонов — граница, ok")

	phones21 := append(append([]PhoneDTO{}, phones20...), PhoneDTO{Phone: "7654321"})
	in = ClientInput{CompanyName: "ООО", Phones: phones21}.normalize()
	assert.Equal(t, map[string]string{"phones": msgTooMany}, fieldsOf(in.validate()))
}

// UT-003: строки телефонов: пустые (phone+note) отбрасываются; note без phone →
// phones[i].phone required; индекс после нормализации.
func TestClientInput_Normalize_PhoneRows(t *testing.T) {
	in := ClientInput{CompanyName: "ООО", Phones: []PhoneDTO{
		{Phone: "", Note: ""},                // отброшена
		{Phone: "  ", Note: "  "},            // отброшена (пусто после trim)
		{Phone: "", Note: "звонить вечером"}, // note без phone — остаётся, индекс 0 после отброса
		{Phone: " 123 ", Note: " доб. 1 "},   // остаётся, индекс 1
	}}.normalize()

	assert.Equal(t, []PhoneDTO{
		{Phone: "", Note: "звонить вечером"},
		{Phone: "123", Note: "доб. 1"},
	}, in.Phones, "пустые строки отброшены, индексы пересчитаны")

	got := fieldsOf(in.validate())
	assert.Equal(t, msgRequired, got["phones[0].phone"], "note без phone → phone required по нормализованному индексу")
	assert.NotContains(t, got, "phones[1].phone")
}

// UT-004: phone: 32/33 → too_long; «abc» / «()-» → invalid_format; «+7 (999) 1» ok;
// note 200/201.
func TestClientInput_Validate_PhoneField(t *testing.T) {
	cases := []struct {
		name  string
		phone string
		note  string
		field string
		msg   string
	}{
		{"phone 32 — граница", strings.Repeat("1", 32), "", "", ""},
		{"phone 33", strings.Repeat("1", 33), "", "phones[0].phone", msgTooLong},
		{"без цифр — буквы", "abc", "", "phones[0].phone", msgInvalidFormat},
		{"без цифр — только спецсимволы", "()-", "", "phones[0].phone", msgInvalidFormat},
		{"с цифрой и форматированием — ok", "+7 (999) 1", "", "", ""},
		{"note 200 — граница", "123", strings.Repeat("я", 200), "", ""},
		{"note 201", "123", strings.Repeat("я", 201), "phones[0].note", msgTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := ClientInput{CompanyName: "ООО", Phones: onePhone(tc.phone, tc.note)}.normalize()
			got := fieldsOf(in.validate())
			if tc.field == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.msg, got[tc.field])
		})
	}
}

// UT-010: digits-нормализация поиска (ADR-003): «8 (999) 12» → digits «899912» + digitsAlt
// «99912» (ведущая 7/8 отбрасывается НЕЗАВИСИМО от длины ввода — фикс регрессии итерации 2:
// раньше digitsAlt считался только при ≥11 цифрах, из-за чего частичный ввод с кодом страны
// не находил номер); «ab1» → «»/«» (не похоже на телефон); «12» (<3 цифр) → «»/«»; digitsAlt
// пуст, если строка не начинается с 7/8.
func TestSearchDigits(t *testing.T) {
	cases := []struct {
		name          string
		q             string
		wantDigits    string
		wantDigitsAlt string
	}{
		{"короткий ввод с ведущей 8 — digitsAlt без длины-порога", "8 (999) 12", "899912", "99912"},
		{"короткий ввод без ведущей 7/8 — digitsAlt пуст", "9 (999) 12", "999912", ""},
		{"буквы — не похоже на телефон", "ab1", "", ""},
		{"меньше 3 цифр", "12", "", ""},
		{"пусто", "", "", ""},
		{"полный номер с ведущей 8", "89991234567", "89991234567", "9991234567"},
		{"полный номер с ведущей 7", "+7 (999) 123-45-67", "79991234567", "9991234567"},
		{"10 цифр без кода страны — digitsAlt пуст", "9991234567", "9991234567", ""},
		{"частичный ввод с ведущей 7 (регрессия итерации 2)", "+7 999 555", "7999555", "999555"},
		{"частичный ввод с ведущей 8 (регрессия итерации 2)", "8 (999) 555", "8999555", "999555"},
		{"3 цифры с ведущей 7 — alt не короче минимума (review LOW)", "755", "755", ""},
		{"4 цифры с ведущей 8 — alt из 3 цифр допустим", "8555", "8555", "555"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			digits, digitsAlt := searchDigits(tc.q)
			assert.Equal(t, tc.wantDigits, digits, "digits")
			assert.Equal(t, tc.wantDigitsAlt, digitsAlt, "digitsAlt")
		})
	}
}

// Госномер: нормализация — trim, схлопывание пробельных серий в один пробел, верхний
// регистр (кириллица и латиница).
func TestNormalizePlateNumber(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"   ":             "",
		"а123вс777":       "А123ВС777",
		"  а123вс   777 ": "А123ВС 777",
		"a\t123 bc":       "A 123 BC",
		"х-987-км":        "Х-987-КМ",
		"Ё001ЁЁ 77":       "Ё001ЁЁ 77",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizePlateNumber(in), "in=%q", in)
	}
	in := EquipmentInput{PlateNumber: " в 777 хх "}.normalize()
	assert.Equal(t, "В 777 ХХ", in.PlateNumber)
}

// Госномер (автомобиль = госномер): обязателен; ≤20 символов (руны, после нормализации); только буквы, цифры,
// пробел, дефис; NUL/битый UTF-8 → invalid_text.
func TestEquipmentInput_Validate_PlateNumber(t *testing.T) {
	cases := []struct {
		name, plate, msg string
	}{
		{"пусто — required", "", msgRequired},
		{"из пробелов — required после нормализации", "   ", msgRequired},
		{"кириллица с пробелом", "а123вс 777", ""},
		{"латиница с дефисом", "ab-1234", ""},
		{"20 рун — граница", strings.Repeat("Я", 20), ""},
		{"20 после схлопывания пробелов", "АААААААААА     ББББББББ1", ""},
		{"21 руна", strings.Repeat("Я", 21), msgTooLong},
		{"спецсимвол", "А123#ВС", msgInvalidFormat},
		{"подчёркивание", "А_1", msgInvalidFormat},
		{"точка", "A.1", msgInvalidFormat},
		{"NUL-байт", "А1\x00", msgInvalidText},
		{"битый UTF-8", "А1\xff", msgInvalidText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fieldsOf(EquipmentInput{PlateNumber: tc.plate}.normalize().validate())
			if tc.msg == "" {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.msg, got["plate_number"])
		})
	}
}
