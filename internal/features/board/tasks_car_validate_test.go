package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/internal/features/clients"
)

// Новый заказ по автомобилю (POST /api/v1/tasks с plate_number): prepare() — валидация формы
// диалога «Новый заказ» с путями полей new_equipment.* / new_client.*.

func carReq(mut func(*TaskCreateRequest)) TaskCreateRequest {
	in := TaskCreateRequest{
		TaskHeader:   TaskHeader{ColumnID: validUUID},
		ClientID:     validUUID,
		PlateNumber:  " о718мр  199 ",
		NewEquipment: &TaskNewEquipment{},
	}
	if mut != nil {
		mut(&in)
	}
	return in.normalize()
}

func fieldMap(t *testing.T, in TaskCreateRequest) (*carOrder, map[string]string) {
	t.Helper()
	car, fields := in.prepare()
	got := map[string]string{}
	for _, f := range fields {
		got[f.Name] = f.Message
	}
	return car, got
}

func TestTaskCreateCar_NewEquipmentExistingClient_Valid(t *testing.T) {
	car, got := fieldMap(t, carReq(nil))
	assert.Empty(t, got)
	require.NotNil(t, car)
	assert.Equal(t, "О718МР 199", car.plate, "госномер нормализован (верхний регистр, пробелы)")
	require.NotNil(t, car.newEquipment)
	assert.Equal(t, "О718МР 199", car.newEquipment.PlateNumber)
	assert.Nil(t, car.newClient)
}

func TestTaskCreateCar_TitleAndClientOptional(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) {
		in.ClientID = ""
		in.NewEquipment = nil
		in.EquipmentID = strings.ToUpper(validUUID)
	}))
	assert.Empty(t, got, "найденный автомобиль: title генерируется, клиент — владелец")
}

func TestTaskCreateCar_PlateRequired(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) { in.PlateNumber = "   " }))
	assert.Equal(t, msgRequired, got["plate_number"])
	assert.NotContains(t, got, "title", "title в режиме по автомобилю не обязателен")
}

func TestTaskCreateCar_PlateBadFormat(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) { in.PlateNumber = "А123*ВС" }))
	assert.Equal(t, "invalid_format", got["plate_number"])
	_, got = fieldMap(t, carReq(func(in *TaskCreateRequest) { in.PlateNumber = strings.Repeat("А", 21) }))
	assert.Equal(t, msgTooLong, got["plate_number"])
	assert.NotContains(t, got, "new_equipment.plate_number", "госномер — только поле верхнего уровня")
}

func TestTaskCreateCar_NewClientFieldPaths(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) {
		in.ClientID = ""
		in.NewClient = &TaskNewClient{CompanyName: " ", Phones: []clients.PhoneDTO{{Phone: "нет цифр"}}}
	}))
	assert.Equal(t, msgRequired, got["new_client.company_name"])
	assert.Equal(t, "invalid_format", got["new_client.phones[0].phone"])
	assert.NotContains(t, got, "client_id", "с new_client client_id не нужен")

	_, got = fieldMap(t, carReq(func(in *TaskCreateRequest) {
		in.ClientID = ""
		in.NewClient = &TaskNewClient{CompanyName: "Петрова Н. С."}
	}))
	assert.Equal(t, msgRequired, got["new_client.phones"])
}

func TestTaskCreateCar_ClientRequiredForNewEquipment(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) { in.ClientID = "" }))
	assert.Equal(t, msgRequired, got["client_id"])
}

func TestTaskCreateCar_Conflicts(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) { in.EquipmentID = validUUID }))
	assert.Equal(t, msgConflict, got["new_equipment"], "equipment_id и new_equipment вместе")

	_, got = fieldMap(t, carReq(func(in *TaskCreateRequest) { in.NewEquipment = nil }))
	assert.Equal(t, msgRequired, got["equipment_id"], "ни найденного, ни нового автомобиля")

	_, got = fieldMap(t, carReq(func(in *TaskCreateRequest) {
		in.NewClient = &TaskNewClient{CompanyName: "Х", Phones: []clients.PhoneDTO{{Phone: "1"}}}
	}))
	assert.Equal(t, msgConflict, got["new_client"], "new_client вместе с client_id")

	_, got = fieldMap(t, carReq(func(in *TaskCreateRequest) { in.NewEquipment = nil; in.EquipmentID = "nope" }))
	assert.Equal(t, msgInvalid, got["equipment_id"])
}

// TestTaskCreate_WithoutCar_Rejected — one-car-order: заказа без автомобиля нет. Прежний режим
// «title + client_id» больше не принимается: госномер и автомобиль (найденный/новый) обязательны.
func TestTaskCreate_WithoutCar_Rejected(t *testing.T) {
	_, got := fieldMap(t, TaskCreateRequest{
		TaskHeader: TaskHeader{Title: "Заказ без машины", ColumnID: validUUID},
		ClientID:   validUUID,
	}.normalize())
	assert.Equal(t, msgRequired, got["plate_number"])
	assert.Equal(t, msgRequired, got["equipment_id"])
	assert.NotContains(t, got, "title")
}

func TestTaskCreate_InvalidClientID(t *testing.T) {
	_, got := fieldMap(t, carReq(func(in *TaskCreateRequest) { in.ClientID = "not-a-uuid" }))
	assert.Equal(t, msgInvalid, got["client_id"])
}

func TestCarTitle(t *testing.T) {
	assert.Equal(t, "О718МР 199", carTitle("О718МР 199"), "название заказа — госномер")
}
