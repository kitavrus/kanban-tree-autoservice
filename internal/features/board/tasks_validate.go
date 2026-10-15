package board

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/internal/features/clients"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// msgNotFound/msgInvalidDate/msgOutOfRange — коды полей заказов; msgRequired/msgTooLong/
// msgInvalid/msgInvalidText — общие (columns_validate.go).
const (
	msgNotFound    = "not_found"
	msgInvalidDate = "invalid_date"
	msgOutOfRange  = "out_of_range"
)

// Границы плана работ заказа (order-start-duration): дней на работу — те же, что CHECK
// tasks.duration_days; дата начала — разумное окно (опечатка в годе — ошибка поля, не 500 БД).
const (
	taskDurationMin = 1
	taskDurationMax = 60
)

var (
	taskStartMin = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	taskStartMax = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
)

// taskTitleMax — design-errors.md §Validation Fields (`title`, trim 1..200).
const taskTitleMax = 200

// taskDescriptionMax — design-errors.md §Validation Fields (`description`, ≤10000
// символов; NULL — PUT «не менять», POST «пусто» — длина не проверяется).
const taskDescriptionMax = 10000

// dateLayout — YYYY-MM-DD (design-errors.md §Validation Fields `due_date`; start_date заказа).
const dateLayout = "2006-01-02"

// normalize — trim текстовых полей; nil-указатели (assignee_id/start_date) остаются nil.
// Пустая строка в assignee_id — «не назначен» (nilIfEmpty() в сервисном слое перед записью в
// БД); пустая start_date — ошибка required в validate().
func (in TaskHeader) normalize() TaskHeader {
	in.Title = strings.TrimSpace(in.Title)
	in.ColumnID = strings.TrimSpace(in.ColumnID)
	if in.AssigneeID != nil {
		v := strings.TrimSpace(*in.AssigneeID)
		in.AssigneeID = &v
	}
	if in.StartDate != nil {
		v := strings.TrimSpace(*in.StartDate)
		in.StartDate = &v
	}
	return in
}

// validate — формат полей после normalize(); существование column_id/assignee_id
// проверяет сервис (design-errors.md: «404 ответственный — в форме ошибка поля»).
func (in TaskHeader) validate() []errorspkg.Field {
	return in.validateWith(true)
}

// validateWith — titleRequired=false: новый заказ, пустой title генерируется из госномера.
func (in TaskHeader) validateWith(titleRequired bool) []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.Title); {
	case n == 0 && titleRequired:
		add("title", msgRequired)
	case n > taskTitleMax:
		add("title", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Title) {
		add("title", msgInvalidText)
	}

	switch {
	case in.ColumnID == "":
		add("column_id", msgRequired)
	case uuid.Validate(in.ColumnID) != nil:
		add("column_id", msgInvalid)
	}

	if in.AssigneeID != nil && *in.AssigneeID != "" && uuid.Validate(*in.AssigneeID) != nil {
		add("assignee_id", msgInvalid)
	}

	// План работ (order-start-duration): начало и дни обязательны; плановое окончание
	// (due_date) вычисляет БД.
	switch {
	case in.StartDate == nil || *in.StartDate == "":
		add("start_date", msgRequired)
	default:
		d, err := time.Parse(dateLayout, *in.StartDate)
		switch {
		case err != nil:
			add("start_date", msgInvalidDate)
		case d.Before(taskStartMin) || d.After(taskStartMax):
			add("start_date", msgOutOfRange)
		}
	}
	switch {
	case in.DurationDays == nil:
		add("duration_days", msgRequired)
	case *in.DurationDays < taskDurationMin || *in.DurationDays > taskDurationMax:
		add("duration_days", msgOutOfRange)
	}

	if in.Description != nil && utf8.RuneCountInString(*in.Description) > taskDescriptionMax {
		add("description", msgTooLong)
	}
	return f
}

// msgConflict — взаимоисключающие поля заказа по автомобилю (equipment_id и new_equipment;
// new_client вместе с client_id либо с найденным автомобилем).
const msgConflict = "conflict"

// msgForeignClient — найденный автомобиль (equipment_id) принадлежит не клиенту client_id.
const msgForeignClient = "foreign_client"

// msgPlateMismatch — найденный автомобиль (equipment_id) с другим госномером, чем plate_number
// нового заказа.
const msgPlateMismatch = "plate_mismatch"

// carOrder — разобранный POST /tasks (prepare()). Ровно одно из equipmentID / newEquipment;
// newClient — только при newEquipment.
type carOrder struct {
	plate        string // NormalizePlateNumber
	equipmentID  string
	newEquipment *clients.EquipmentInput // после clients.PrepareEquipment
	newClient    *clients.ClientInput    // после clients.PrepareClient
}

// normalize — шапка + госномер (clients.NormalizePlateNumber) и equipment_id; поля
// new_equipment/new_client нормализует prepare() правилами пакета clients.
func (in TaskCreateRequest) normalize() TaskCreateRequest {
	in.TaskHeader = in.TaskHeader.normalize()
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.PlateNumber = clients.NormalizePlateNumber(in.PlateNumber)
	in.EquipmentID = strings.ToLower(strings.TrimSpace(in.EquipmentID))
	return in
}

// prepare — валидация формы после normalize(). Заказа без автомобиля нет (one-car-order):
// plate_number и одно из equipment_id/new_equipment обязательны всегда. Поля нового
// автомобиля/клиента валидируются правилами пакета clients (те же коды, что у
// POST /clients/:id/equipment и POST /clients) с префиксом new_equipment./new_client.
func (in TaskCreateRequest) prepare() (*carOrder, []errorspkg.Field) {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }
	car := &carOrder{plate: in.PlateNumber, equipmentID: in.EquipmentID}

	f = append(f, in.validateWith(false)...)

	// клиент: найденный автомобиль — его владелец; новый автомобиль — client_id либо new_client.
	switch {
	case in.ClientID == "":
		if in.EquipmentID == "" && in.NewClient == nil {
			add("client_id", msgRequired)
		}
	case uuid.Validate(in.ClientID) != nil:
		add("client_id", msgInvalid)
	}

	if in.PlateNumber == "" {
		add("plate_number", msgRequired)
	} else if msg := clients.PlateNumberProblem(in.PlateNumber); msg != "" {
		add("plate_number", msg)
	}

	switch {
	case in.EquipmentID != "" && in.NewEquipment != nil:
		add("new_equipment", msgConflict)
	case in.EquipmentID == "" && in.NewEquipment == nil:
		add("equipment_id", msgRequired)
	case in.EquipmentID != "" && uuid.Validate(in.EquipmentID) != nil:
		add("equipment_id", msgInvalid)
	}

	if in.NewEquipment != nil {
		eq, ef := clients.PrepareEquipment(clients.EquipmentInput{PlateNumber: in.PlateNumber})
		for _, e := range ef {
			if e.Name == "plate_number" {
				continue // госномер — поле верхнего уровня, проверен выше
			}
			add("new_equipment."+e.Name, e.Message)
		}
		car.newEquipment = &eq
	}

	if in.NewClient != nil {
		switch {
		case in.EquipmentID != "":
			add("new_client", msgConflict) // у найденного автомобиля владелец уже есть
		case in.ClientID != "":
			add("new_client", msgConflict)
		}
		cl, cf := clients.PrepareClient(clients.ClientInput{CompanyName: in.NewClient.CompanyName, Phones: in.NewClient.Phones})
		for _, e := range cf {
			add("new_client."+e.Name, e.Message)
		}
		car.newClient = &cl
	}
	return car, f
}

// carTitle — название заказа по автомобилю: госномер (пробелы схлопнуты), не длиннее
// taskTitleMax символов.
func carTitle(plate string) string {
	t := strings.Join(strings.Fields(plate), " ")
	if r := []rune(t); len(r) > taskTitleMax {
		t = strings.TrimSpace(string(r[:taskTitleMax]))
	}
	return t
}

// normalize/validate TaskUpdateRequest — та же форма плюс обязательный version (≥1).
func (in TaskUpdateRequest) normalize() TaskUpdateRequest {
	in.TaskHeader = in.TaskHeader.normalize()
	return in
}

func (in TaskUpdateRequest) validate() []errorspkg.Field {
	f := in.TaskHeader.validate()
	if in.Version == nil || *in.Version < 1 {
		f = append(f, errorspkg.Field{Name: "version", Message: msgRequired})
	}
	return f
}
