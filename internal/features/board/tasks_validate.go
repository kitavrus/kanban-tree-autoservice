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

// msgNotFound/msgInvalidDate — коды полей заказов; msgRequired/msgTooLong/msgInvalid/
// msgInvalidText — общие (columns_validate.go).
const (
	msgNotFound    = "not_found"
	msgInvalidDate = "invalid_date"
)

// taskTitleMax — design-errors.md §Validation Fields (`title`, trim 1..200).
const taskTitleMax = 200

// taskDescriptionMax — design-errors.md §Validation Fields (`description`, ≤10000
// символов; NULL — PUT «не менять», POST «пусто» — длина не проверяется).
const taskDescriptionMax = 10000

// dateLayout — YYYY-MM-DD (design-errors.md §Validation Fields `due_date`).
const dateLayout = "2006-01-02"

// normalize — trim текстовых полей; nil-указатели (assignee_id/due_date) остаются nil,
// пустая строка внутри указателя не трактуется как отсутствие значения здесь — это делает
// nilIfEmpty() в сервисном слое перед записью в БД.
func (in TaskHeader) normalize() TaskHeader {
	in.Title = strings.TrimSpace(in.Title)
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.ColumnID = strings.TrimSpace(in.ColumnID)
	if in.AssigneeID != nil {
		v := strings.TrimSpace(*in.AssigneeID)
		in.AssigneeID = &v
	}
	if in.DueDate != nil {
		v := strings.TrimSpace(*in.DueDate)
		in.DueDate = &v
	}
	return in
}

// validate — формат полей после normalize(); существование client_id/column_id/
// assignee_id проверяет сервис (design-errors.md: «404 клиент/ответственный — в форме
// ошибка поля»).
func (in TaskHeader) validate() []errorspkg.Field {
	return in.validateWith(true, true)
}

// validateWith — titleRequired=false: заказ по автомобилю, пустой title генерируется;
// clientRequired=false: клиент — владелец найденного автомобиля либо новый клиент.
func (in TaskHeader) validateWith(titleRequired, clientRequired bool) []errorspkg.Field {
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
	case in.ClientID == "":
		if clientRequired {
			add("client_id", msgRequired)
		}
	case uuid.Validate(in.ClientID) != nil:
		add("client_id", msgInvalid)
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

	if in.DueDate != nil && *in.DueDate != "" {
		if _, err := time.Parse(dateLayout, *in.DueDate); err != nil {
			add("due_date", msgInvalidDate)
		}
	}

	if in.Description != nil && utf8.RuneCountInString(*in.Description) > taskDescriptionMax {
		add("description", msgTooLong)
	}
	return f
}

// msgConflict — взаимоисключающие поля заказа по автомобилю (equipment_id и new_equipment;
// new_client вместе с client_id либо с найденным автомобилем).
const msgConflict = "conflict"

// carOrder — разобранный POST /tasks в режиме «по автомобилю» (prepare()). Ровно одно из
// equipmentID / newEquipment; newClient — только при newEquipment.
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
	in.PlateNumber = clients.NormalizePlateNumber(in.PlateNumber)
	in.EquipmentID = strings.ToLower(strings.TrimSpace(in.EquipmentID))
	return in
}

// byCar — запрос в режиме «по автомобилю».
func (in TaskCreateRequest) byCar() bool {
	return in.PlateNumber != "" || in.EquipmentID != "" || in.NewEquipment != nil || in.NewClient != nil
}

// prepare — валидация формы после normalize(). car == nil — заказ без автомобиля. Поля
// нового автомобиля/клиента валидируются правилами пакета clients (те же коды, что у
// POST /clients/:id/equipment и POST /clients) с префиксом new_equipment./new_client.
func (in TaskCreateRequest) prepare() (*carOrder, []errorspkg.Field) {
	if !in.byCar() {
		return nil, in.validate()
	}
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }
	car := &carOrder{plate: in.PlateNumber, equipmentID: in.EquipmentID}

	// клиент: найденный автомобиль — его владелец; новый автомобиль — client_id либо new_client.
	clientRequired := in.EquipmentID == "" && in.NewClient == nil
	f = append(f, in.validateWith(false, clientRequired)...)

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
		eq, ef := clients.PrepareEquipment(clients.EquipmentInput{
			TypeID: in.NewEquipment.TypeID, PlateNumber: in.PlateNumber,
			Brand: in.NewEquipment.Brand, Model: in.NewEquipment.Model, Serial: in.NewEquipment.Serial,
		})
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

// carTitle — название заказа по автомобилю: «{Марка} {Модель} {ГОСНОМЕР}» (пробелы
// схлопнуты, пустые части пропущены), не длиннее taskTitleMax символов.
func carTitle(brand, model, plate string) string {
	t := strings.Join(strings.Fields(brand+" "+model+" "+plate), " ")
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
