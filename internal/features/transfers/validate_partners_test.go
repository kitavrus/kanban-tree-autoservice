package transfers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Unit-сценарии части transfers-partners (docs/features/transfers-partners/draft-spec.md §11):
// TS-001 — точки маршрута по направлению, TS-002 — матрица «направление × тип строки»,
// TS-008 — границы факта приёмки по направлению, TS-015 — скоуп исполнителя, TS-023 — границы
// дня фильтра «период» в часовом поясе пользователя.

const supX = "77777777-7777-7777-7777-777777777777"

func taskID(n int64) *int64 { return &n }

// validRoute — корректный документ направления со строкой допустимого типа.
func validRoute(direction string) Input {
	in := Input{Direction: direction}
	switch direction {
	case DirClientToWarehouse:
		in.ToWarehouseID, in.TaskID, in.Items = whB, taskID(37), []ItemInput{unit(eqX)}
	case DirWarehouseToClient:
		in.FromWarehouseID, in.TaskID, in.Items = whA, taskID(37), []ItemInput{unit(eqX)}
	case DirWarehouseToWarehouse:
		in.FromWarehouseID, in.ToWarehouseID, in.Items = whA, whB, []ItemInput{product(prodX, "1"), unit(eqX)}
	case DirSupplierToWarehouse:
		in.ToWarehouseID, in.SupplierID, in.Items = whB, supX, []ItemInput{product(prodX, "10")}
	}
	return in
}

// TS-001: direction обязателен и из перечня; у каждого из четырёх направлений свой набор
// точек маршрута — недостающая → required, лишняя → not_allowed; from = to → same_warehouse.
func TestTS001_RouteByDirection(t *testing.T) {
	for _, d := range Directions {
		t.Run("корректный "+d, func(t *testing.T) {
			assert.Empty(t, validRoute(d).normalize().validate())
		})
	}

	t.Run("direction required / invalid", func(t *testing.T) {
		in := validRoute(DirWarehouseToWarehouse)
		in.Direction = ""
		assert.Equal(t, map[string]string{"direction": msgRequired}, fieldMap(in.normalize().validate()))
		in.Direction = "warehouse_to_supplier"
		assert.Equal(t, map[string]string{"direction": msgInvalid}, fieldMap(in.normalize().validate()))
		in.Direction = "Client_To_Warehouse"
		assert.Equal(t, map[string]string{"direction": msgInvalid}, fieldMap(in.normalize().validate()), "регистр значим")
	})

	// недостающая точка маршрута → required (негатив §11: client_to_warehouse без task_id).
	missing := []struct {
		name      string
		direction string
		drop      func(*Input)
		want      map[string]string
	}{
		{"client_to_warehouse без task_id", DirClientToWarehouse, func(in *Input) { in.TaskID = nil }, map[string]string{"task_id": msgRequired}},
		{"client_to_warehouse без склада назначения", DirClientToWarehouse, func(in *Input) { in.ToWarehouseID = "" }, map[string]string{"to_warehouse_id": msgRequired}},
		{"warehouse_to_client без task_id", DirWarehouseToClient, func(in *Input) { in.TaskID = nil }, map[string]string{"task_id": msgRequired}},
		{"warehouse_to_client без склада отправления", DirWarehouseToClient, func(in *Input) { in.FromWarehouseID = "" }, map[string]string{"from_warehouse_id": msgRequired}},
		{"warehouse_to_warehouse без складов", DirWarehouseToWarehouse, func(in *Input) { in.FromWarehouseID, in.ToWarehouseID = "", "" },
			map[string]string{"from_warehouse_id": msgRequired, "to_warehouse_id": msgRequired}},
		{"supplier_to_warehouse без поставщика", DirSupplierToWarehouse, func(in *Input) { in.SupplierID = "" }, map[string]string{"supplier_id": msgRequired}},
		{"supplier_to_warehouse без склада назначения", DirSupplierToWarehouse, func(in *Input) { in.ToWarehouseID = "" }, map[string]string{"to_warehouse_id": msgRequired}},
	}
	for _, tc := range missing {
		t.Run(tc.name, func(t *testing.T) {
			in := validRoute(tc.direction)
			tc.drop(&in)
			assert.Equal(t, tc.want, fieldMap(in.normalize().validate()))
		})
	}

	// лишняя точка маршрута → not_allowed: каждому направлению подставлены ВСЕ четыре точки.
	extra := map[string]map[string]string{
		DirClientToWarehouse:    {"from_warehouse_id": msgNotAllowed, "supplier_id": msgNotAllowed},
		DirWarehouseToClient:    {"to_warehouse_id": msgNotAllowed, "supplier_id": msgNotAllowed},
		DirWarehouseToWarehouse: {"task_id": msgNotAllowed, "supplier_id": msgNotAllowed},
		DirSupplierToWarehouse:  {"from_warehouse_id": msgNotAllowed, "task_id": msgNotAllowed},
	}
	for d, want := range extra {
		t.Run("лишние точки "+d, func(t *testing.T) {
			in := validRoute(d)
			in.FromWarehouseID, in.ToWarehouseID, in.TaskID, in.SupplierID = whA, whB, taskID(37), supX
			assert.Equal(t, want, fieldMap(in.normalize().validate()))
		})
	}

	t.Run("формат точек маршрута", func(t *testing.T) {
		in := validRoute(DirSupplierToWarehouse)
		in.SupplierID = "x"
		assert.Equal(t, map[string]string{"supplier_id": msgInvalid}, fieldMap(in.normalize().validate()))
		in = validRoute(DirClientToWarehouse)
		in.TaskID = taskID(0)
		assert.Equal(t, map[string]string{"task_id": msgInvalid}, fieldMap(in.normalize().validate()))
		in.TaskID = taskID(-5)
		assert.Equal(t, map[string]string{"task_id": msgInvalid}, fieldMap(in.normalize().validate()))
	})

	t.Run("same_warehouse — только там, где оба склада", func(t *testing.T) {
		in := validRoute(DirWarehouseToWarehouse)
		in.ToWarehouseID = whA
		assert.Equal(t, map[string]string{"to_warehouse_id": msgSameWarehouse}, fieldMap(in.normalize().validate()))
		// у направления с одним складом лишний склад — not_allowed, а не same_warehouse.
		in = validRoute(DirClientToWarehouse)
		in.FromWarehouseID = in.ToWarehouseID
		assert.Equal(t, map[string]string{"from_warehouse_id": msgNotAllowed}, fieldMap(in.normalize().validate()))
	})
}

// TS-001: direction и task_id в теле PUT → not_allowed (после создания не меняются); флаг
// receive допустим только для приёма от клиента.
func TestTS001_ImmutableAndReceiveFlag(t *testing.T) {
	assert.Empty(t, immutableGuards(nil, nil))
	d := DirWarehouseToClient
	assert.Equal(t, map[string]string{"direction": msgNotAllowed}, fieldMap(immutableGuards(&d, nil)))
	assert.Equal(t, map[string]string{"task_id": msgNotAllowed}, fieldMap(immutableGuards(nil, taskID(37))))
	// то же значение, что уже в документе, — тоже not_allowed: поле не принимается вовсе.
	assert.Equal(t, map[string]string{"direction": msgNotAllowed, "task_id": msgNotAllowed}, fieldMap(immutableGuards(&d, taskID(37))))

	assert.Empty(t, createFlags(DirClientToWarehouse, true))
	for _, dir := range []string{DirWarehouseToClient, DirWarehouseToWarehouse, DirSupplierToWarehouse} {
		assert.Equal(t, map[string]string{"receive": msgNotAllowed}, fieldMap(createFlags(dir, true)), dir)
		assert.Empty(t, createFlags(dir, false), dir)
	}
	assert.Empty(t, createFlags("нет такого", true), "некорректное направление — ошибка direction, не receive")
}

// TS-002: матрица «направление × kind» — товар в приёме от клиента и единица в приходе от
// поставщика → items[i].kind: not_allowed; прочие ошибки строки при этом не дублируются. Товар
// в накладной «склад → клиент» допустим (хвост transfers-sales, DS-011).
func TestTS002_KindByDirection(t *testing.T) {
	allowed := map[string]map[string]bool{
		DirClientToWarehouse:    {KindProduct: false, KindEquipment: true},
		DirWarehouseToClient:    {KindProduct: true, KindEquipment: true},
		DirWarehouseToWarehouse: {KindProduct: true, KindEquipment: true},
		DirSupplierToWarehouse:  {KindProduct: true, KindEquipment: false},
	}
	items := map[string]ItemInput{KindProduct: product(prodX, "1"), KindEquipment: unit(eqX)}
	for _, d := range Directions {
		for kind, item := range items {
			t.Run(d+"/"+kind, func(t *testing.T) {
				in := validRoute(d)
				in.Items = []ItemInput{item}
				want := map[string]string{}
				if !allowed[d][kind] {
					want["items[0].kind"] = msgNotAllowed
				}
				assert.Equal(t, want, fieldMap(in.normalize().validate()))
				assert.Equal(t, allowed[d][kind], kindAllowed(d, kind))
			})
		}
	}

	// недопустимая строка с мусором в полях — одна ошибка kind, без product_id / qty_sent.
	in := validRoute(DirClientToWarehouse)
	in.Items = []ItemInput{unit(eqX), {Kind: KindProduct, ProductID: "x", QtySent: "-1"}}
	assert.Equal(t, map[string]string{"items[1].kind": msgNotAllowed}, fieldMap(in.normalize().validate()))

	// неизвестный kind остаётся invalid в любом направлении.
	in = validRoute(DirSupplierToWarehouse)
	in.Items = []ItemInput{{Kind: "service"}}
	assert.Equal(t, map[string]string{"items[0].kind": msgInvalid}, fieldMap(in.normalize().validate()))
	assert.False(t, kindAllowed(DirWarehouseToWarehouse, "service"))
}

// TS-008: факт приёмки по направлению — склад → склад: факт > отправленного → exceeds_sent;
// приход от поставщика: факт больше и меньше заказанного допустим (предел — NUMERIC(12,3));
// документы с клиентом: любой items → not_allowed; отрицательный факт → negative везде.
func TestTS008_ResolveReceiveByDirection(t *testing.T) {
	items := []sentItem{{ID: "aaaaaaaa-0000-0000-0000-000000000001", Kind: KindProduct, QtySent: "10.000"}}
	fact := func(q string) []ReceiveItemInput {
		return []ReceiveItemInput{{ID: items[0].ID, QtyReceived: numeric.Amount(q)}}
	}

	_, f := resolveReceive(DirWarehouseToWarehouse, items, fact("12"))
	assert.Equal(t, map[string]string{"items[0].qty_received": msgExceedsSent}, fieldMap(f))

	facts, f := resolveReceive(DirSupplierToWarehouse, items, fact("12"))
	require.Empty(t, f, "приход от поставщика: факт больше заказанного")
	assert.Equal(t, "12", facts[items[0].ID])
	facts, f = resolveReceive(DirSupplierToWarehouse, items, fact("7.5"))
	require.Empty(t, f, "приход от поставщика: факт меньше заказанного")
	assert.Equal(t, "7.5", facts[items[0].ID])
	facts, f = resolveReceive(DirSupplierToWarehouse, items, fact("0"))
	require.Empty(t, f)
	assert.Equal(t, "0", facts[items[0].ID])
	facts, f = resolveReceive(DirSupplierToWarehouse, items, nil)
	require.Empty(t, f)
	assert.Equal(t, "10.000", facts[items[0].ID], "без записи — факт = заказанное")
	facts, f = resolveReceive(DirSupplierToWarehouse, items, fact("999999999.999"))
	require.Empty(t, f, "верхняя граница NUMERIC(12,3)")
	assert.Equal(t, "999999999.999", facts[items[0].ID])
	_, f = resolveReceive(DirSupplierToWarehouse, items, fact("1000000000"))
	assert.Equal(t, map[string]string{"items[0].qty_received": "too_large"}, fieldMap(f))

	for _, d := range Directions {
		_, f = resolveReceive(d, items, fact("-1"))
		if isClientDirection(d) {
			continue
		}
		assert.Equal(t, map[string]string{"items[0].qty_received": msgNegative}, fieldMap(f), d)
	}

	units := []sentItem{{ID: "aaaaaaaa-0000-0000-0000-000000000002", Kind: KindEquipment}}
	for _, d := range []string{DirClientToWarehouse, DirWarehouseToClient} {
		facts, f = resolveReceive(d, units, nil)
		require.Empty(t, f, d)
		assert.Empty(t, facts, "у документа с клиентом факта нет")
		_, f = resolveReceive(d, units, []ReceiveItemInput{{ID: units[0].ID, QtyReceived: "1"}})
		assert.Equal(t, map[string]string{"items": msgNotAllowed}, fieldMap(f), d)
		// мусор в items тоже not_allowed — разбор строк не начинается.
		_, f = resolveReceive(d, units, []ReceiveItemInput{{}})
		assert.Equal(t, map[string]string{"items": msgNotAllowed}, fieldMap(f), d)
	}

	// накладная «склад → клиент» с товаром (transfers-sales DS-038): факт не вводится — товарная
	// строка в факты не попадает (qty_received остаётся NULL), items по-прежнему not_allowed.
	facts, f = resolveReceive(DirWarehouseToClient, append(units, items...), nil)
	require.Empty(t, f)
	assert.Empty(t, facts, "товар накладной списан при отправке — факта нет")
	_, f = resolveReceive(DirWarehouseToClient, items, fact("10"))
	assert.Equal(t, map[string]string{"items": msgNotAllowed}, fieldMap(f))
}

// TS-015 (unit): администратору доступно любое направление, исполнителю — только документы с
// клиентом; тот же перечень уходит в скоуп списка.
func TestTS015_ActorScope(t *testing.T) {
	admin, executor := Actor{IsAdmin: true}, Actor{}
	for _, d := range Directions {
		assert.True(t, admin.canAccess(d), d)
	}
	assert.True(t, executor.canAccess(DirClientToWarehouse))
	assert.True(t, executor.canAccess(DirWarehouseToClient))
	assert.False(t, executor.canAccess(DirWarehouseToWarehouse))
	assert.False(t, executor.canAccess(DirSupplierToWarehouse))
	assert.ElementsMatch(t, []string{DirClientToWarehouse, DirWarehouseToClient}, clientDirections)

	var app *errorspkg.AppError
	require.ErrorAs(t, error(ErrForbidden), &app)
	assert.Equal(t, 403, app.HTTP)
	assert.Equal(t, "FORBIDDEN", app.Code)
}

// TS-023 (unit): границы дня фильтра «период» считаются в часовом поясе пользователя и
// уходят в SQL моментами UTC: [начало created_from; начало дня после created_to).
func TestTS023_PeriodBoundsInUserTimezone(t *testing.T) {
	created := time.Date(2026, 10, 6, 19, 30, 0, 0, time.UTC) // 00:30 7 окт. по Алматы, 22:30 6 окт. по Москве
	within := func(p ListParams) bool {
		return !created.Before(*p.CreatedFrom) && created.Before(*p.CreatedTo)
	}
	day := func(tz, date string) ListParams {
		p, f := parseListQuery(ListQuery{CreatedFrom: date, CreatedTo: date}, tz)
		require.Empty(t, f, tz+" "+date)
		require.NotNil(t, p.CreatedFrom)
		require.NotNil(t, p.CreatedTo)
		return p
	}

	almaty := day("Asia/Almaty", "2026-10-07")
	assert.Equal(t, time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC), almaty.CreatedFrom.UTC())
	assert.Equal(t, time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC), almaty.CreatedTo.UTC())
	assert.True(t, within(almaty), "Алматы, 7 октября — найден")
	assert.False(t, within(day("Asia/Almaty", "2026-10-06")), "Алматы, 6 октября — нет")

	moscow := day("Europe/Moscow", "2026-10-06")
	assert.Equal(t, time.Date(2026, 10, 5, 21, 0, 0, 0, time.UTC), moscow.CreatedFrom.UTC())
	assert.Equal(t, time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC), moscow.CreatedTo.UTC())
	assert.True(t, within(moscow), "Москва, 6 октября — найден")
	assert.False(t, within(day("Europe/Moscow", "2026-10-07")), "Москва, 7 октября — нет")

	// только одна граница — вторая не задаётся.
	p, f := parseListQuery(ListQuery{CreatedFrom: "2026-10-07"}, "Asia/Almaty")
	require.Empty(t, f)
	assert.NotNil(t, p.CreatedFrom)
	assert.Nil(t, p.CreatedTo)
	p, f = parseListQuery(ListQuery{CreatedTo: "2026-10-07"}, "Asia/Almaty")
	require.Empty(t, f)
	assert.Nil(t, p.CreatedFrom)
	assert.Equal(t, time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC), p.CreatedTo.UTC(), "конец дня включительно")

	// диапазон в несколько дней: «по» включительно.
	p, f = parseListQuery(ListQuery{CreatedFrom: "2026-10-01", CreatedTo: "2026-10-07"}, "Asia/Almaty")
	require.Empty(t, f)
	assert.True(t, within(p))

	// негатив: «по» раньше «с», несуществующая дата, не дата.
	for name, tc := range map[string]struct {
		q    ListQuery
		want map[string]string
	}{
		"по раньше с":        {ListQuery{CreatedFrom: "2026-10-07", CreatedTo: "2026-10-06"}, map[string]string{"created_to": msgBeforeFrom}},
		"2026-13-40":         {ListQuery{CreatedFrom: "2026-13-40"}, map[string]string{"created_from": msgInvalid}},
		"30 февраля":         {ListQuery{CreatedTo: "2026-02-30"}, map[string]string{"created_to": msgInvalid}},
		"дата со временем":   {ListQuery{CreatedFrom: "2026-10-07T00:00:00Z"}, map[string]string{"created_from": msgInvalid}},
		"обе границы битые":  {ListQuery{CreatedFrom: "вчера", CreatedTo: "07.10.2026"}, map[string]string{"created_from": msgInvalid, "created_to": msgInvalid}},
		"битая с, верная по": {ListQuery{CreatedFrom: "x", CreatedTo: "2026-10-07"}, map[string]string{"created_from": msgInvalid}},
	} {
		_, f := parseListQuery(tc.q, "Asia/Almaty")
		assert.Equal(t, tc.want, fieldMap(f), name)
	}
}

// Фильтры списка §6.1: направление, заказ, поставщик, склад, статус — формат.
func TestParseListQuery_Filters(t *testing.T) {
	p, f := parseListQuery(ListQuery{
		Status: StatusInTransit, WarehouseID: " " + whA + " ", Direction: DirSupplierToWarehouse,
		SupplierID: supX, TaskID: "37",
	}, "Asia/Almaty")
	require.Empty(t, f)
	assert.Equal(t, whA, p.WarehouseID)
	assert.Equal(t, DirSupplierToWarehouse, p.Direction)
	assert.Equal(t, supX, p.SupplierID)
	require.NotNil(t, p.TaskID)
	assert.Equal(t, int64(37), *p.TaskID)

	_, f = parseListQuery(ListQuery{Status: "done", WarehouseID: "x", Direction: "sideways", SupplierID: "y", TaskID: "abc"}, "Asia/Almaty")
	assert.Equal(t, map[string]string{
		"status": msgInvalid, "warehouse_id": msgInvalid, "direction": msgInvalid, "supplier_id": msgInvalid, "task_id": msgInvalid,
	}, fieldMap(f))
	for _, bad := range []string{"0", "-1", "1.5", "9223372036854775808", " 37"} {
		_, f = parseListQuery(ListQuery{TaskID: bad}, "Asia/Almaty")
		assert.Equal(t, map[string]string{"task_id": msgInvalid}, fieldMap(f), bad)
	}
	p, f = parseListQuery(ListQuery{TaskID: "9223372036854775807"}, "Asia/Almaty")
	require.Empty(t, f)
	assert.Equal(t, int64(9223372036854775807), *p.TaskID)
}

// TS-006 (хвост, unit): приём от клиента — единица «уже не у клиента» получает свой текст при
// том же коде 409.
func TestTS006_NotAtClientMessage(t *testing.T) {
	var app *errorspkg.AppError
	require.ErrorAs(t, mapErr(&notAtSourceError{equipmentID: eqX, fromClient: true}), &app)
	assert.Equal(t, "TRANSFER_EQUIPMENT_NOT_AT_SOURCE", app.Code)
	assert.Equal(t, 409, app.HTTP)
	assert.Equal(t, "Оборудование уже не у клиента", app.Message)
	assert.Equal(t, map[string]any{"equipment_id": eqX}, app.Extra)
	assert.Nil(t, ErrEquipmentNotAtClient.Extra, "sentinel не мутируется")

	require.ErrorAs(t, mapErr(&notAtSourceError{equipmentID: eqX}), &app)
	assert.Equal(t, "Оборудование находится не на складе отправления", app.Message)
}
