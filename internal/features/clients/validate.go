package clients

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — SSR переводит их в текст.
const (
	msgRequired      = "required"
	msgTooLong       = "too_long"
	msgTooMany       = "too_many"
	msgInvalidFormat = "invalid_format"
	msgInvalid       = "invalid"
	// msgInvalidText — NUL-байт / невалидный UTF-8 (code review §TD-4).
	msgInvalidText = "invalid_text"
)

// Границы полей (Tech Spec §Лимиты).
const (
	companyNameMax = 200
	phonesMax      = 20
	phoneMax       = 32
	phoneNoteMax   = 200

	equipmentPlateMax = 20
)

// phoneDigitRe — телефон обязан содержать хотя бы одну цифру.
var phoneDigitRe = regexp.MustCompile(`[0-9]`)

// plateNumberRe — допустимые символы госномера: буквы (кириллица/латиница), цифры,
// пробел, дефис. Проверяется ПОСЛЕ NormalizePlateNumber.
var plateNumberRe = regexp.MustCompile(`^[\p{L}0-9 -]*$`)

// plateSpacesRe — серия пробельных символов, включая неразрывный пробел (схлопывается в
// один пробел).
var plateSpacesRe = regexp.MustCompile(`[\s\p{Zs}]+`)

// NormalizePlateNumber — trim, схлопывание пробелов, верхний регистр («а123вс  777» →
// «А123ВС 777»). Используется и при сохранении, и в тестах.
func NormalizePlateNumber(s string) string {
	s = strings.TrimSpace(s)
	if sqlutil.HasInvalidText(s) {
		// битый UTF-8 / NUL не трогаем: ToUpper заменил бы байты на U+FFFD и validate()
		// не увидел бы invalid_text.
		return s
	}
	s = plateSpacesRe.ReplaceAllString(s, " ")
	return strings.ToUpper(s)
}

// PlateNumberProblem — код ошибки поля госномера после NormalizePlateNumber ("" — госномер
// допустим; пустой тоже допустим — обязательность решает вызывающий). Общий для единицы
// оборудования и нового заказа по госномеру (board).
func PlateNumberProblem(p string) string {
	switch {
	case sqlutil.HasInvalidText(p):
		return msgInvalidText
	case utf8.RuneCountInString(p) > equipmentPlateMax:
		return msgTooLong
	case !plateNumberRe.MatchString(p):
		return msgInvalidFormat
	}
	return ""
}

// ClientInput — нормализованные поля карточки клиента + телефонов (общие для API и
// SSR-формы).
type ClientInput struct {
	CompanyName string
	Phones      []PhoneDTO
}

// normalize — trim всех строк; строки телефонов с пустыми phone И note отбрасываются
// (пустая строка формы). Индексы валидации/ошибок считаются ПОСЛЕ этого отброса.
func (in ClientInput) normalize() ClientInput {
	in.CompanyName = strings.TrimSpace(in.CompanyName)

	phones := make([]PhoneDTO, 0, len(in.Phones))
	for _, p := range in.Phones {
		p.Phone = strings.TrimSpace(p.Phone)
		p.Note = strings.TrimSpace(p.Note)
		if p.Phone == "" && p.Note == "" {
			continue
		}
		phones = append(phones, p)
	}
	in.Phones = phones
	return in
}

// validate — ошибки полей после normalize().
func (in ClientInput) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.CompanyName); {
	case n == 0:
		add("company_name", msgRequired)
	case n > companyNameMax:
		add("company_name", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.CompanyName) {
		add("company_name", msgInvalidText)
	}

	switch {
	case len(in.Phones) == 0:
		add("phones", msgRequired)
	case len(in.Phones) > phonesMax:
		add("phones", msgTooMany)
	}
	for i, p := range in.Phones {
		switch n := utf8.RuneCountInString(p.Phone); {
		case n == 0:
			add(fmt.Sprintf("phones[%d].phone", i), msgRequired)
		case n > phoneMax:
			add(fmt.Sprintf("phones[%d].phone", i), msgTooLong)
		case !phoneDigitRe.MatchString(p.Phone):
			add(fmt.Sprintf("phones[%d].phone", i), msgInvalidFormat)
		}
		if sqlutil.HasInvalidText(p.Phone) {
			add(fmt.Sprintf("phones[%d].phone", i), msgInvalidText)
		}
		if utf8.RuneCountInString(p.Note) > phoneNoteMax {
			add(fmt.Sprintf("phones[%d].note", i), msgTooLong)
		}
		if sqlutil.HasInvalidText(p.Note) {
			add(fmt.Sprintf("phones[%d].note", i), msgInvalidText)
		}
	}
	return f
}

// EquipmentInput — автомобиль: только госномер (общие для API и SSR-формы).
type EquipmentInput struct {
	PlateNumber string
}

// normalize — госномер через NormalizePlateNumber.
func (in EquipmentInput) normalize() EquipmentInput {
	in.PlateNumber = NormalizePlateNumber(in.PlateNumber)
	return in
}

// validate — ошибки полей после normalize(): госномер обязателен.
func (in EquipmentInput) validate() []errorspkg.Field {
	if in.PlateNumber == "" {
		return []errorspkg.Field{{Name: "plate_number", Message: msgRequired}}
	}
	if msg := PlateNumberProblem(in.PlateNumber); msg != "" {
		return []errorspkg.Field{{Name: "plate_number", Message: msg}}
	}
	return nil
}
