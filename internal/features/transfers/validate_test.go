package transfers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

const (
	whA   = "11111111-1111-1111-1111-111111111111"
	whB   = "22222222-2222-2222-2222-222222222222"
	prodX = "33333333-3333-3333-3333-333333333333"
	prodY = "44444444-4444-4444-4444-444444444444"
	eqX   = "55555555-5555-5555-5555-555555555555"
	eqY   = "66666666-6666-6666-6666-666666666666"
)

func fieldMap(f []errorspkg.Field) map[string]string {
	m := map[string]string{}
	for _, x := range f {
		if _, ok := m[x.Name]; !ok {
			m[x.Name] = x.Message
		}
	}
	return m
}

func product(id, qty string) ItemInput {
	return ItemInput{Kind: KindProduct, ProductID: id, QtySent: numeric.Amount(qty)}
}

func unit(id string) ItemInput { return ItemInput{Kind: KindEquipment, EquipmentID: id} }

// input — документ «склад → склад» (направление ядра) со строками items.
func input(items ...ItemInput) Input {
	return Input{Direction: DirWarehouseToWarehouse, FromWarehouseID: whA, ToWarehouseID: whB, Items: items}
}

func TestInputValidate_OK(t *testing.T) {
	in := Input{
		Direction:       " " + DirWarehouseToWarehouse + " ",
		FromWarehouseID: " " + strings.ToUpper(whA) + " ", ToWarehouseID: whB, Note: "  срочно ",
		Items: []ItemInput{product(prodX, "1.5"), product(prodY, "2"), unit(" " + strings.ToUpper(eqX) + " "), unit(eqY)},
	}.normalize()
	assert.Empty(t, in.validate())
	assert.Equal(t, whA, in.FromWarehouseID, "trim + lower")
	assert.Equal(t, eqX, in.Items[2].EquipmentID, "trim + lower единицы")
	assert.Equal(t, "срочно", in.Note)
}

// from = to → 400 to_warehouse_id: same_warehouse (регистр uuid не важен).
func TestInputValidate_SameWarehouse(t *testing.T) {
	in := Input{Direction: DirWarehouseToWarehouse, FromWarehouseID: whA, ToWarehouseID: strings.ToUpper(whA), Items: []ItemInput{product(prodX, "1")}}.normalize()
	assert.Equal(t, map[string]string{"to_warehouse_id": msgSameWarehouse}, fieldMap(in.validate()))
}

func TestInputValidate_Fields(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want map[string]string
	}{
		{"пусто", Input{Direction: DirWarehouseToWarehouse}, map[string]string{"from_warehouse_id": msgRequired, "to_warehouse_id": msgRequired, "items": msgRequired}},
		{"битые uuid", Input{Direction: DirWarehouseToWarehouse, FromWarehouseID: "x", ToWarehouseID: "y", Items: []ItemInput{product("z", "1"), unit("q")}},
			map[string]string{"from_warehouse_id": msgInvalid, "to_warehouse_id": msgInvalid,
				"items[0].product_id": msgInvalid, "items[1].equipment_id": msgInvalid}},
		{"urn-форма uuid — не каноническая", input(product("urn:uuid:"+prodX, "1")), map[string]string{"items[0].product_id": msgInvalid}},
		{"qty 0", input(product(prodX, "0")), map[string]string{"items[0].qty_sent": msgNotPositive}},
		{"qty отрицательное", input(product(prodX, "-1")), map[string]string{"items[0].qty_sent": msgNotPositive}},
		{"qty нет", input(ItemInput{Kind: KindProduct, ProductID: prodX}), map[string]string{"items[0].qty_sent": msgRequired}},
		{"qty точность", input(product(prodX, "1.0001")), map[string]string{"items[0].qty_sent": "too_precise"}},
		{"qty велико", input(product(prodX, "1000000000")), map[string]string{"items[0].qty_sent": "too_large"}},
		{"дубль товара", input(product(prodX, "1"), product(strings.ToUpper(prodX), "2")), map[string]string{"items[1].product_id": msgDuplicate}},
		{"дубль единицы", input(unit(eqX), unit(strings.ToUpper(eqX))), map[string]string{"items[1].equipment_id": msgDuplicate}},
		{"товар и единица с одним uuid — не дубль", input(product(prodX, "1"), unit(prodX)), map[string]string{}},
		{"длинное примечание", Input{Direction: DirWarehouseToWarehouse, FromWarehouseID: whA, ToWarehouseID: whB, Note: strings.Repeat("я", noteMax+1), Items: []ItemInput{product(prodX, "1")}},
			map[string]string{"note": msgTooLong}},
		{"NUL в примечании", Input{Direction: DirWarehouseToWarehouse, FromWarehouseID: whA, ToWarehouseID: whB, Note: "a\x00b", Items: []ItemInput{product(prodX, "1")}},
			map[string]string{"note": msgInvalidText}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, fieldMap(tc.in.normalize().validate()))
		})
	}
}

// §6.2: валидатор строк kind × поля — «чужое» поле типа строки → not_allowed, своё
// отсутствует → required, kind не из списка → invalid / required.
func TestInputValidate_KindMatrix(t *testing.T) {
	cases := []struct {
		name string
		item ItemInput
		want map[string]string
	}{
		{"product: всё своё", product(prodX, "1"), map[string]string{}},
		{"equipment: всё своё", unit(eqX), map[string]string{}},
		{"product + equipment_id", ItemInput{Kind: KindProduct, ProductID: prodX, QtySent: "1", EquipmentID: eqX},
			map[string]string{"items[0].equipment_id": msgNotAllowed}},
		{"equipment + qty_sent", ItemInput{Kind: KindEquipment, EquipmentID: eqX, QtySent: "1"},
			map[string]string{"items[0].qty_sent": msgNotAllowed}},
		{"equipment + product_id", ItemInput{Kind: KindEquipment, EquipmentID: eqX, ProductID: prodX},
			map[string]string{"items[0].product_id": msgNotAllowed}},
		{"equipment + оба чужих", ItemInput{Kind: KindEquipment, EquipmentID: eqX, ProductID: prodX, QtySent: "0"},
			map[string]string{"items[0].product_id": msgNotAllowed, "items[0].qty_sent": msgNotAllowed}},
		{"product без product_id", ItemInput{Kind: KindProduct, QtySent: "1"}, map[string]string{"items[0].product_id": msgRequired}},
		{"equipment без equipment_id", ItemInput{Kind: KindEquipment}, map[string]string{"items[0].equipment_id": msgRequired}},
		{"kind пуст", ItemInput{ProductID: prodX, QtySent: "1"}, map[string]string{"items[0].kind": msgRequired}},
		{"kind неизвестен", ItemInput{Kind: "service", ProductID: prodX, QtySent: "1"}, map[string]string{"items[0].kind": msgInvalid}},
		{"kind в другом регистре", ItemInput{Kind: "Product", ProductID: prodX, QtySent: "1"}, map[string]string{"items[0].kind": msgInvalid}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, fieldMap(input(tc.item).normalize().validate()))
		})
	}
}

func TestInputValidate_TooManyItems(t *testing.T) {
	items := make([]ItemInput, itemsMax+1)
	for i := range items {
		items[i] = product(prodX, "1")
	}
	got := fieldMap(input(items...).normalize().validate())
	assert.Equal(t, msgTooMany, got["items"])
}

// DS-037: факт по умолчанию = qty_sent только у товарных строк; 0 ≤ qty_received ≤ qty_sent;
// строка-единица в запросе приёмки → not_allowed; чужая строка → not_found.
func TestResolveReceive(t *testing.T) {
	items := []sentItem{
		{ID: "aaaaaaaa-0000-0000-0000-000000000001", Kind: KindProduct, QtySent: "5.000"},
		{ID: "aaaaaaaa-0000-0000-0000-000000000002", Kind: KindProduct, QtySent: "2.500"},
		{ID: "aaaaaaaa-0000-0000-0000-000000000003", Kind: KindEquipment},
	}

	facts, f := resolveReceive(DirWarehouseToWarehouse, items, nil)
	require.Empty(t, f)
	assert.Equal(t, map[string]string{items[0].ID: "5.000", items[1].ID: "2.500"}, facts, "у единицы факта нет")

	facts, f = resolveReceive(DirWarehouseToWarehouse, items, []ReceiveItemInput{{ID: strings.ToUpper(items[0].ID), QtyReceived: "3.5"}, {ID: items[1].ID, QtyReceived: "0"}})
	require.Empty(t, f)
	assert.Equal(t, "3.5", facts[items[0].ID])
	assert.Equal(t, "0", facts[items[1].ID])

	_, f = resolveReceive(DirWarehouseToWarehouse, items, []ReceiveItemInput{
		{ID: items[0].ID, QtyReceived: "5.001"},
		{ID: "bbbbbbbb-0000-0000-0000-000000000009", QtyReceived: "1"},
		{ID: items[1].ID, QtyReceived: "-1"},
		{ID: items[1].ID, QtyReceived: "1"},
		{ID: "", QtyReceived: "1"},
		{ID: items[2].ID, QtyReceived: "1"},
	})
	assert.Equal(t, map[string]string{
		"items[0].qty_received": msgExceedsSent,
		"items[1].id":           msgNotFound,
		"items[2].qty_received": msgNegative,
		"items[3].id":           msgDuplicate,
		"items[4].id":           msgRequired,
		"items[5].id":           msgNotAllowed,
	}, fieldMap(f))

	_, f = resolveReceive(DirWarehouseToWarehouse, items, []ReceiveItemInput{{ID: items[0].ID}})
	assert.Equal(t, map[string]string{"items[0].qty_received": msgRequired}, fieldMap(f))
	_, f = resolveReceive(DirWarehouseToWarehouse, items, []ReceiveItemInput{{ID: items[0].ID, QtyReceived: "1.0001"}})
	assert.Equal(t, map[string]string{"items[0].qty_received": "too_precise"}, fieldMap(f))
}

func TestIsStatusAndParseID(t *testing.T) {
	for _, s := range Statuses {
		assert.True(t, isStatus(s))
	}
	assert.False(t, isStatus("done"))
	_, err := parseID("abc")
	assert.ErrorIs(t, err, ErrTransferNotFound)
	_, err = parseID("0")
	assert.ErrorIs(t, err, ErrTransferNotFound)
	id, err := parseID("42")
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
}

// TS-009 (unit): у каждого недопустимого перехода §5 — своё сообщение; код один.
func TestTS009_StatusConflictMessages(t *testing.T) {
	cases := []struct{ action, status, want string }{
		{actionUpdate, StatusInTransit, "Перемещение уже отправлено — править нельзя"},
		{actionUpdate, StatusReceived, "Перемещение уже получено — править нельзя"},
		{actionUpdate, StatusCancelled, "Перемещение отменено — править нельзя"},
		{actionSend, StatusInTransit, "Перемещение уже отправлено"},
		{actionSend, StatusReceived, "Перемещение уже получено"},
		{actionSend, StatusCancelled, "Перемещение отменено"},
		{actionReceive, StatusRequested, "Перемещение ещё не отправлено"},
		{actionReceive, StatusReceived, "Перемещение уже получено"},
		{actionReceive, StatusCancelled, "Перемещение отменено"},
		{actionCancel, StatusReceived, "Перемещение получено — сначала отмените проведение"},
		{actionCancel, StatusCancelled, "Перемещение уже отменено"},
	}
	for _, tc := range cases {
		t.Run(tc.action+"/"+tc.status, func(t *testing.T) {
			var app *errorspkg.AppError
			require.ErrorAs(t, mapErr(&statusError{action: tc.action, status: tc.status}), &app)
			assert.Equal(t, "TRANSFER_STATUS_CONFLICT", app.Code)
			assert.Equal(t, 409, app.HTTP)
			assert.Equal(t, tc.want, app.Message)
		})
	}
	// допустимый переход в таблице сообщений отсутствует — общий текст, не паника.
	var app *errorspkg.AppError
	require.ErrorAs(t, mapErr(&statusError{action: actionSend, status: StatusRequested}), &app)
	assert.Equal(t, ErrStatusConflict.Message, app.Message)
}

// TS-005 / TS-006 (unit): ошибки занятия и местоположения единицы несут details.
func TestMapErr_EquipmentConflicts(t *testing.T) {
	var app *errorspkg.AppError
	require.ErrorAs(t, mapErr(&equipmentBusyError{equipmentID: eqX, transferID: 17}), &app)
	assert.Equal(t, "EQUIPMENT_IN_TRANSFER", app.Code)
	assert.Equal(t, 409, app.HTTP)
	assert.Equal(t, "Оборудование уже в перемещении #17", app.Message)
	assert.Equal(t, map[string]any{"transfer_id": int64(17), "equipment_id": eqX}, app.Extra)

	// страховочная ветка guard-UPDATE'а: документ неизвестен — текст без «#0», без details.
	require.ErrorAs(t, mapErr(&equipmentBusyError{}), &app)
	assert.Equal(t, "EQUIPMENT_IN_TRANSFER", app.Code)
	assert.Equal(t, "Оборудование уже в перемещении", app.Message)
	assert.Nil(t, app.Extra)

	require.ErrorAs(t, mapErr(&notAtSourceError{equipmentID: eqY}), &app)
	assert.Equal(t, "TRANSFER_EQUIPMENT_NOT_AT_SOURCE", app.Code)
	assert.Equal(t, 409, app.HTTP)
	assert.Equal(t, map[string]any{"equipment_id": eqY}, app.Extra)
	assert.Nil(t, ErrEquipmentNotAtSource.Extra, "sentinel не мутируется")

	assert.Equal(t, ErrVersionConflict, mapErr(errVersion))
	assert.Equal(t, ErrTransferNotFound, mapErr(errNotFound))
	assert.Equal(t, ErrStockOverflow, mapErr(errOverflow))
}

// FuzzTS028_CreateBody — TS-028 (TD-4/TD-7): произвольный JSON тела POST /api/v1/transfers —
// разбор + normalize() + validate() + createFlags() без паники; имена полей ошибок — только из
// контракта §6.2 (с направлением и точками маршрута transfers-partners).
func FuzzTS028_CreateBody(f *testing.F) {
	for _, seed := range []string{
		`{}`, `null`, `[]`, `{"items":null}`, `{"items":[{}]}`, `{"items":[null]}`,
		`{"from_warehouse_id":"` + whA + `","to_warehouse_id":"` + whB + `","items":[{"kind":"product","product_id":"` + prodX + `","qty_sent":1}]}`,
		`{"from_warehouse_id":"` + whA + `","to_warehouse_id":"` + whA + `","items":[{"kind":"equipment","equipment_id":"` + eqX + `","qty_sent":1e3}]}`,
		`{"items":[{"kind":"equipment","product_id":"x","qty_sent":-0}],"send":true,"note":"\u0000"}`,
		`{"items":[{"kind":7}]}`, `{"send":"yes"}`, `{"items":[{"kind":"product","qty_sent":"1"}]}`,
		`{"items":[{"kind":"product","qty_sent":99999999999999999999999999}]}`,
		`{"direction":"client_to_warehouse","to_warehouse_id":"` + whA + `","task_id":7,"client_id":"` + whB + `","items":[{"kind":"equipment","equipment_id":"` + eqX + `"}],"receive":true}`,
		`{"direction":"supplier_to_warehouse","to_warehouse_id":"` + whA + `","supplier_id":"x","task_id":-1,"items":[{"kind":"equipment","equipment_id":"` + eqX + `"}],"receive":true}`,
		`{"direction":"warehouse_to_client","from_warehouse_id":"` + whA + `","to_warehouse_id":"` + whB + `","task_id":9223372036854775807,"items":[{"kind":"product","product_id":"` + prodX + `","qty_sent":1}]}`,
		`{"direction":"sideways","task_id":0}`, `{"direction":7}`, `{"task_id":"7"}`,
	} {
		f.Add(seed)
	}
	allowed := func(name string) bool {
		switch name {
		case "direction", "from_warehouse_id", "to_warehouse_id", "task_id", "supplier_id", "receive", "note", "items":
			return true
		}
		if !strings.HasPrefix(name, "items[") {
			return false
		}
		_, field, ok := strings.Cut(name, "].")
		return ok && (field == "kind" || field == "product_id" || field == "equipment_id" || field == "qty_sent")
	}
	f.Fuzz(func(t *testing.T, body string) {
		var req TransferCreateRequest
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			return // хендлер ответит 400 «Некорректное тело запроса»
		}
		in := Input{
			Direction: req.Direction, FromWarehouseID: req.FromWarehouseID, ToWarehouseID: req.ToWarehouseID,
			TaskID: req.TaskID, SupplierID: req.SupplierID, Note: req.Note, Items: req.Items,
		}.normalize()
		for _, fld := range append(in.validate(), createFlags(in.Direction, req.Receive)...) {
			if !allowed(fld.Name) {
				t.Fatalf("неожиданное поле в ошибках валидации: %q (%s)", fld.Name, body)
			}
			if fld.Message == "" {
				t.Fatalf("пустой код ошибки поля %q (%s)", fld.Name, body)
			}
		}
	})
}
