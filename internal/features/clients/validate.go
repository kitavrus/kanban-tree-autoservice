package clients

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

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
	msgKeyRequired   = "key_required"
	// msgInvalidText — NUL-байт / невалидный UTF-8 (code review §TD-4): отдельный от
	// msgInvalid код, т.к. msgInvalid для type_id в equipmentFieldText жёстко маппится на
	// текст «Тип не найден» безотносительно поля — переиспользование сломало бы текст
	// ошибки для остальных полей.
	msgInvalidText = "invalid_text"
)

// Границы полей (Tech Spec §Лимиты).
const (
	companyNameMax = 200
	addressMax     = 500
	contactMax     = 200
	phonesMax      = 20
	phoneMax       = 32
	phoneNoteMax   = 200

	equipmentModelMax  = 200
	equipmentBrandMax  = 200
	equipmentSKUMax    = 200
	equipmentSerialMax = 200
	equipmentNoteMax   = 1000
	equipmentPlateMax  = 20
	specsMax           = 50
	specKMax           = 100
	specVMax           = 500
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
	CompanyName     string
	Address         string
	ContactName     string
	ContactPosition string
	Phones          []PhoneDTO
}

// normalize — trim всех строк; строки телефонов с пустыми phone И note отбрасываются
// (пустая строка формы). Индексы валидации/ошибок считаются ПОСЛЕ этого отброса.
func (in ClientInput) normalize() ClientInput {
	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.Address = strings.TrimSpace(in.Address)
	in.ContactName = strings.TrimSpace(in.ContactName)
	in.ContactPosition = strings.TrimSpace(in.ContactPosition)

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
	if utf8.RuneCountInString(in.Address) > addressMax {
		add("address", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Address) {
		add("address", msgInvalidText)
	}
	if utf8.RuneCountInString(in.ContactName) > contactMax {
		add("contact_name", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.ContactName) {
		add("contact_name", msgInvalidText)
	}
	if utf8.RuneCountInString(in.ContactPosition) > contactMax {
		add("contact_position", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.ContactPosition) {
		add("contact_position", msgInvalidText)
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

// EquipmentInput — нормализованные поля единицы оборудования (общие для API и SSR-формы).
type EquipmentInput struct {
	TypeID      string
	PlateNumber string
	Brand       string
	Model       string
	SKU         string
	Serial      string
	Note        string
	Specs       []SpecDTO
}

// normalize — trim; пары характеристик с пустыми k И v отбрасываются; nil specs → [].
func (in EquipmentInput) normalize() EquipmentInput {
	in.TypeID = strings.TrimSpace(in.TypeID)
	in.PlateNumber = NormalizePlateNumber(in.PlateNumber)
	in.Brand = strings.TrimSpace(in.Brand)
	in.Model = strings.TrimSpace(in.Model)
	in.SKU = strings.TrimSpace(in.SKU)
	in.Serial = strings.TrimSpace(in.Serial)
	in.Note = strings.TrimSpace(in.Note)

	specs := make([]SpecDTO, 0, len(in.Specs))
	for _, sp := range in.Specs {
		sp.K = strings.TrimSpace(sp.K)
		sp.V = strings.TrimSpace(sp.V)
		if sp.K == "" && sp.V == "" {
			continue
		}
		specs = append(specs, sp)
	}
	in.Specs = specs
	return in
}

// validate — ошибки полей после normalize(). Активность/существование type_id — сервис
// (запрос к БД), не здесь. Дубли k разрешены.
func (in EquipmentInput) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch {
	case in.TypeID == "":
		add("type_id", msgRequired)
	case uuid.Validate(in.TypeID) != nil:
		add("type_id", msgInvalid)
	}

	if msg := PlateNumberProblem(in.PlateNumber); msg != "" {
		add("plate_number", msg)
	}

	switch n := utf8.RuneCountInString(in.Model); {
	case n == 0:
		add("model", msgRequired)
	case n > equipmentModelMax:
		add("model", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Model) {
		add("model", msgInvalidText)
	}
	if utf8.RuneCountInString(in.Brand) > equipmentBrandMax {
		add("brand", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Brand) {
		add("brand", msgInvalidText)
	}
	if utf8.RuneCountInString(in.SKU) > equipmentSKUMax {
		add("sku", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.SKU) {
		add("sku", msgInvalidText)
	}
	if utf8.RuneCountInString(in.Serial) > equipmentSerialMax {
		add("serial", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Serial) {
		add("serial", msgInvalidText)
	}
	if utf8.RuneCountInString(in.Note) > equipmentNoteMax {
		add("note", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Note) {
		add("note", msgInvalidText)
	}

	if len(in.Specs) > specsMax {
		add("specs", msgTooMany)
	}
	for i, sp := range in.Specs {
		switch {
		case sp.K == "":
			add(fmt.Sprintf("specs[%d].k", i), msgKeyRequired)
		case utf8.RuneCountInString(sp.K) > specKMax:
			add(fmt.Sprintf("specs[%d].k", i), msgTooLong)
		}
		if sqlutil.HasInvalidText(sp.K) {
			add(fmt.Sprintf("specs[%d].k", i), msgInvalidText)
		}
		if utf8.RuneCountInString(sp.V) > specVMax {
			add(fmt.Sprintf("specs[%d].v", i), msgTooLong)
		}
		if sqlutil.HasInvalidText(sp.V) {
			add(fmt.Sprintf("specs[%d].v", i), msgInvalidText)
		}
	}
	return f
}
