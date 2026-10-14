//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/kitavrus/kanban-tree/internal/features/board"
)

// Местоположение оборудования заказа (docs/features/transfers-core/draft-spec.md: DS-003,
// DS-008, DS-026, DS-046; TS-018, TS-024, TS-027) — оно на самой единице (equipment) и
// меняется документом «Перемещение» /api/v1/transfers; из карточки заказа сняты
// send/receive/cancel-transit. Методы OrderCardLiveSuite — общие фикстуры карточки
// (order_card_http_live_test.go, warehouses_card_http_live_test.go).

func (s *OrderCardLiveSuite) cardEquipment(c orderCardDTO, equipmentID string) cardEquipmentDTO {
	for _, e := range c.Equipment {
		if e.EquipmentID == equipmentID {
			return e
		}
	}
	s.FailNow("единица не найдена в карточке", equipmentID)
	return cardEquipmentDTO{}
}

func strOr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// addEq — POST оборудования заказа. Само добавление местоположение единицы не меняет
// (transfers-partners DS-045); warehouseID != "" — фикстура «единица на складе»: единица,
// ещё числящаяся «у клиента» и свободная, принимается на этот склад документом «клиент →
// склад» (receive: true), ответ несёт карточку после приёма. Единица, уже лежащая на складе
// либо едущая, остаётся где была.
func (s *OrderCardLiveSuite) addEq(taskID int64, equipmentID, warehouseID string) orderCardMutationDTO {
	st, m, raw := s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID),
		map[string]any{"equipment_id": equipmentID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	if warehouseID == "" {
		return m
	}
	if e := s.cardEquipment(m.Card, equipmentID); e.LocationStatus != "at_client" || e.ActiveTransferID != nil {
		return m
	}
	s.clientTransfer("client_to_warehouse", taskID, warehouseID, map[string]any{"receive": true}, equipmentID)
	_, card, _ := s.getCard(s.admin, taskID)
	m.Card = card
	return m
}

// transferDocDTO — документ «Перемещение» (wire §6.2, только нужное тестам доски).
type transferDocDTO struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}

// transferErrDTO — 409 по единице: код, текст, details.transfer_id.
type transferErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *struct {
		TransferID int64 `json:"transfer_id"`
		// TransferIDs — 409 TASK_HAS_TRANSFERS: номера мешающих документов (transfers-revert).
		TransferIDs []int64 `json:"transfer_ids"`
	} `json:"details"`
}

// newTransfer — документ склад → склад с единицами через реальный API перемещений
// (send — сразу «в пути»).
func (s *OrderCardLiveSuite) newTransfer(from, to string, send bool, equipmentIDs ...string) transferDocDTO {
	items := make([]map[string]any, len(equipmentIDs))
	for i, id := range equipmentIDs {
		items[i] = map[string]any{"kind": "equipment", "equipment_id": id}
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"direction": "warehouse_to_warehouse", "from_warehouse_id": from, "to_warehouse_id": to, "items": items, "send": send,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var t transferDocDTO
	s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	return t
}

// clientTransfer — документ с клиентом по заказу через реальный API перемещений: direction —
// client_to_warehouse (warehouseID — склад назначения) либо warehouse_to_client (склад
// отправления); flags — send / receive.
func (s *OrderCardLiveSuite) clientTransfer(direction string, taskID int64, warehouseID string, flags map[string]any, equipmentIDs ...string) transferDocDTO {
	items := make([]map[string]any, len(equipmentIDs))
	for i, id := range equipmentIDs {
		items[i] = map[string]any{"kind": "equipment", "equipment_id": id}
	}
	body := map[string]any{"direction": direction, "task_id": taskID, "items": items}
	if direction == "client_to_warehouse" {
		body["to_warehouse_id"] = warehouseID
	} else {
		body["from_warehouse_id"] = warehouseID
	}
	for k, v := range flags {
		body[k] = v
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/transfers", body)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var t transferDocDTO
	s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	return t
}

func (s *OrderCardLiveSuite) transferAction(t transferDocDTO, action string) transferDocDTO {
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/%s", t.ID, action), map[string]any{"version": t.Version})
	s.Require().Equal(http.StatusOK, st, "%s: %s", action, raw)
	var out transferDocDTO
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	return out
}

// attachRaw — строка task_equipment напрямую SQL: единица в заказе, местоположение не
// трогается (как и у API добавления после transfers-partners DS-045).
func (s *OrderCardLiveSuite) attachRaw(taskID int64, equipmentID string) {
	_, err := s.Pool.Exec(context.Background(), `INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2)`, taskID, equipmentID)
	s.Require().NoError(err)
}

// ---- DS-045: добавление единицы в заказ местоположение не меняет ----

// TS-027 (хвост transfers-partners): POST /tasks/:id/equipment — warehouse_id упразднён:
// передан (верный, чужой, несуществующий, мусор любого типа) — игнорируется, единица остаётся
// «у клиента»; на склад её приводит документ «клиент → склад»; журнала перемещений в карточке
// нет, документы заказа — в card.transfers.
func (s *OrderCardLiveSuite) TestTS027_AddIgnoresWarehouseID_EquipmentStaysAtClient() {
	moscow := s.warehouseID("Москва")
	c := s.newClient("Клиент-Оборудование-Склад")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	path := fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID)

	first := s.newEquipment(c)
	for _, wh := range []any{nil, moscow, "00000000-0000-0000-0000-000000000000", "almaty", "", 42, map[string]any{"id": moscow}} {
		eq := s.newEquipment(c)
		body := map[string]any{"equipment_id": eq}
		if wh != nil {
			body["warehouse_id"] = wh
		}
		st, m, raw := s.doEquipment(s.admin, http.MethodPost, path, body)
		s.Require().Equal(http.StatusCreated, st, "warehouse_id=%v → %s", wh, raw)
		e := s.cardEquipment(m.Card, eq)
		s.Equal("at_client", e.LocationStatus, "warehouse_id=%v игнорируется", wh)
		s.Nil(e.WarehouseID)
		s.Nil(e.WarehouseName)
		s.Nil(e.ActiveTransferID)
		s.Nil(e.TransitToName)
		s.False(m.Card.Task.HasInTransit)
		if wh == moscow {
			first = eq
		}
	}

	// повторный POST той же единицы — 201 без изменений.
	st, m, raw := s.doEquipment(s.admin, http.MethodPost, path, map[string]any{"equipment_id": first, "warehouse_id": moscow})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("at_client", s.cardEquipment(m.Card, first).LocationStatus)

	// wire: удалённых полей в карточке нет (§6.9); документов у заказа ещё нет — пустой массив.
	_, card, raw := s.getCard(s.admin, taskID)
	for _, gone := range []string{`"movements"`, `"equipment_status"`, `"transit_from_warehouse_id"`, `"transit_to_warehouse_id"`} {
		s.NotContains(string(raw), gone)
	}
	s.Contains(string(raw), `"location_status":"at_client"`)
	s.Contains(string(raw), `"active_transfer_id":null`)
	s.Contains(string(raw), `"transit_to_name":null`)
	s.Contains(string(raw), `"active_transfer_direction":null`)
	s.Contains(string(raw), `"transfers":[]`)
	s.NotNil(card.Transfers)

	// на склад единицу приводит документ «клиент → склад».
	doc := s.clientTransfer("client_to_warehouse", taskID, moscow, map[string]any{"receive": true}, first)
	s.Equal("received", doc.Status)
	_, card, _ = s.getCard(s.admin, taskID)
	e := s.cardEquipment(card, first)
	s.Equal("at_warehouse", e.LocationStatus)
	s.Equal(moscow, strOr(e.WarehouseID))
	s.Equal("Москва", strOr(e.WarehouseName))
	s.Nil(e.ActiveTransferID)
}

// ---- DS-012: документы заказа в карточке, запрет удаления заказа ----

// §6.8: card.transfers — документы заказа, новые сверху, склад назначения (приём) либо
// отправления (возврат); единица в пути к клиенту — transit_to_name «клиент», под фильтр
// «Склад» доски такой заказ не попадает (§6.9, DS-013).
func (s *OrderCardLiveSuite) TestCardTransfers_ListAndTransitToClient() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Перемещения-Заказа")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	other, _ := s.newTask(c, s.columnID("Новые"))
	eq, eqOther := s.newEquipment(c), s.newEquipment(c)
	s.addEq(taskID, eq, "")
	s.addEq(other, eqOther, "")

	// приём «в пути» → карточка: «в пути → Москва».
	intake := s.clientTransfer("client_to_warehouse", taskID, moscow, map[string]any{"send": true}, eq)
	_, card, _ := s.getCard(s.admin, taskID)
	e := s.cardEquipment(card, eq)
	s.Equal("in_transit", e.LocationStatus)
	s.Equal("Москва", strOr(e.TransitToName))
	s.Equal("client_to_warehouse", strOr(e.ActiveTransferDirection))
	s.True(s.boardIDs("warehouse_id=" + moscow)[taskID], "единица в пути к складу — заказ под фильтром склада назначения")
	s.transferAction(intake, "receive")

	// возврат «в пути» → «в пути → клиент»; ни под один склад заказ не попадает.
	back := s.clientTransfer("warehouse_to_client", taskID, moscow, map[string]any{"send": true}, eq)
	_, card, raw := s.getCard(s.admin, taskID)
	e = s.cardEquipment(card, eq)
	s.Equal("in_transit", e.LocationStatus)
	s.Nil(e.WarehouseID)
	s.Equal("клиент", strOr(e.TransitToName))
	s.Require().NotNil(e.ActiveTransferID)
	s.Equal(back.ID, *e.ActiveTransferID)
	s.True(card.Task.HasInTransit)
	s.False(s.boardIDs("warehouse_id=" + moscow)[taskID], "в пути к клиенту — не под складом отправления")
	s.False(s.boardIDs("warehouse_id=" + almaty)[taskID])

	s.Require().Len(card.Transfers, 2, string(raw))
	s.Equal(cardTransferDTO{ID: back.ID, Direction: "warehouse_to_client", Status: "in_transit", WarehouseName: "Москва", ItemsCount: 1,
		CreatedAt: card.Transfers[0].CreatedAt, Amount: "0.00"}, card.Transfers[0], "новые сверху; накладная без товаров — сумма 0.00")
	s.Equal(intake.ID, card.Transfers[1].ID)
	s.Equal("client_to_warehouse", card.Transfers[1].Direction)
	s.Equal("received", card.Transfers[1].Status)
	s.Equal("Москва", card.Transfers[1].WarehouseName)
	s.NotEmpty(card.Transfers[1].CreatedAt)

	// документы другого заказа того же клиента в карточку не попадают; исполнитель видит блок.
	s.clientTransfer("client_to_warehouse", other, almaty, nil, eqOther)
	_, card, _ = s.getCard(s.admin, taskID)
	s.Len(card.Transfers, 2)
	login := fmt.Sprintf("card-tr-exec-%d", taskID)
	s.SeedUser(login, "password1", "Перемещения заказа Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login(login, "password1")
	st, execCard, raw := s.getCard(exec, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Len(execCard.Transfers, 2)

	// «Принять» у возврата: единица снова «у клиента».
	s.transferAction(back, "receive")
	_, card, _ = s.getCard(s.admin, taskID)
	e = s.cardEquipment(card, eq)
	s.Equal("at_client", e.LocationStatus)
	s.Nil(e.TransitToName)
	s.Nil(e.ActiveTransferID)
	s.Equal("received", card.Transfers[0].Status)
}

// ---- TS-027 ----

// TS-027: удалённые endpoints (send / receive / cancel-transit оборудования заказа и
// /api/v1/stock-transfers, страница /stock-transfers) → 404.
func (s *OrderCardLiveSuite) TestTS027_RemovedEndpoints404() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Удалённые-Endpoints")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	eq := s.newEquipment(c)
	s.addEq(taskID, eq, almaty)

	for _, action := range []string{"send", "receive", "cancel-transit"} {
		st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s/%s", taskID, eq, action),
			map[string]any{"to_warehouse_id": moscow})
		s.Equal(http.StatusNotFound, st, "%s → %s", action, raw)
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/stock-transfers"},
		{http.MethodPost, "/api/v1/stock-transfers"},
		{http.MethodGet, "/api/v1/stock-transfers/1"},
		{http.MethodPut, "/api/v1/stock-transfers/1"},
		{http.MethodPost, "/api/v1/stock-transfers/1/send"},
		{http.MethodPost, "/api/v1/stock-transfers/1/receive"},
		{http.MethodPost, "/api/v1/stock-transfers/1/cancel"},
		{http.MethodGet, "/stock-transfers"},
	} {
		st, raw := s.admin.Do(c.method, c.path, map[string]any{"version": 1})
		s.Equal(http.StatusNotFound, st, "%s %s → %s", c.method, c.path, raw)
	}
	// местоположение единицы не изменилось.
	_, card, _ := s.getCard(s.admin, taskID)
	e := s.cardEquipment(card, eq)
	s.Equal("at_warehouse", e.LocationStatus)
	s.Equal(almaty, strOr(e.WarehouseID))
}

// TS-027 (негатив): POST /tasks/:id/equipment с warehouse_id для единицы, уже лежащей на
// другом складе либо едущей по документу, — местоположение не меняется.
func (s *OrderCardLiveSuite) TestTS027_AddDoesNotMoveEquipmentAlreadyAtWarehouse() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Склад-Не-Меняется")
	col := s.columnID("Новые")
	first, _ := s.newTask(c, col)
	second, _ := s.newTask(c, col)

	eq := s.newEquipment(c)
	s.addEq(first, eq, moscow)
	m := s.addEq(second, eq, almaty) // та же единица во втором заказе, «склад приёма» — другой
	e := s.cardEquipment(m.Card, eq)
	s.Equal("at_warehouse", e.LocationStatus)
	s.Equal(moscow, strOr(e.WarehouseID), "единица уже на складе — склад приёма не применяется")

	// единица в пути по документу: добавление в заказ её не «принимает».
	moving := s.newEquipment(c)
	s.addEq(first, moving, almaty)
	doc := s.newTransfer(almaty, moscow, true, moving)
	m = s.addEq(second, moving, almaty)
	e = s.cardEquipment(m.Card, moving)
	s.Equal("in_transit", e.LocationStatus)
	s.Nil(e.WarehouseID)
	s.Require().NotNil(e.ActiveTransferID)
	s.Equal(doc.ID, *e.ActiveTransferID)
	s.Equal("Москва", strOr(e.TransitToName))
	s.True(m.Card.Task.HasInTransit)
}

// ---- TS-018 ----

// TS-018: единица в незавершённом документе — DELETE /tasks/:id/equipment/:equipment_id →
// 409 EQUIPMENT_IN_TRANSFER (с номером документа); DELETE /equipment/:id такой единицы → тоже
// 409 EQUIPMENT_IN_TRANSFER (soft-delete-clients DS-014: отказ по строкам документов
// EQUIPMENT_HAS_TRANSFERS снят, действует занятие незавершённым документом); после отмены
// документа единица убирается из заказа, а из справочника не удаляется, пока числится на нашем
// складе (409 EQUIPMENT_AT_WAREHOUSE, DS-041).
func (s *OrderCardLiveSuite) TestTS018_EquipmentInOpenTransferGuards() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Запреты-Оборудования")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	eq := s.newEquipment(c)
	s.addEq(taskID, eq, almaty)
	removePath := fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eq)

	for _, send := range []bool{false, true} { // заявка и «в пути» — оба незавершённые
		doc := s.newTransfer(almaty, moscow, send, eq)

		_, card, _ := s.getCard(s.admin, taskID)
		e := s.cardEquipment(card, eq)
		s.Require().NotNil(e.ActiveTransferID, "send=%v", send)
		s.Equal(doc.ID, *e.ActiveTransferID)
		if send {
			s.Equal("in_transit", e.LocationStatus)
			s.Equal("Москва", strOr(e.TransitToName))
			s.True(card.Task.HasInTransit)
		} else {
			s.Equal("at_warehouse", e.LocationStatus, "заявка не меняет местоположение")
			s.Nil(e.TransitToName, "склад назначения — только пока единица в пути")
			s.False(card.Task.HasInTransit)
		}

		st, raw := s.admin.Do(http.MethodDelete, removePath, nil)
		s.Require().Equal(http.StatusConflict, st, "send=%v: %s", send, raw)
		var te transferErrDTO
		s.Require().NoError(json.Unmarshal(raw, &te), string(raw))
		s.Equal("EQUIPMENT_IN_TRANSFER", te.Code)
		s.Equal(fmt.Sprintf("Оборудование в перемещении #%d — сначала отмените его", doc.ID), te.Message)
		s.Require().NotNil(te.Details)
		s.Equal(doc.ID, te.Details.TransferID)

		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+eq, map[string]any{"reason": "Списана"})
		s.Require().Equal(http.StatusConflict, st, "send=%v: %s", send, raw)
		te = transferErrDTO{}
		s.Require().NoError(json.Unmarshal(raw, &te), string(raw))
		s.Equal("EQUIPMENT_IN_TRANSFER", te.Code)
		s.Require().NotNil(te.Details)
		s.Equal(doc.ID, te.Details.TransferID)

		// исполнитель упирается в тот же запрет (права как у правки оборудования, D19).
		login := fmt.Sprintf("ts018-exec-%v", send)
		s.SeedUser(login, "password1", "TS018 Исполнитель", "executor", true)
		exec := s.NewBrowser()
		exec.Login(login, "password1")
		st, raw = exec.Do(http.MethodDelete, removePath, nil)
		s.Equal(http.StatusConflict, st, string(raw))

		s.transferAction(doc, "cancel")
		_, card, _ = s.getCard(s.admin, taskID)
		e = s.cardEquipment(card, eq)
		s.Nil(e.ActiveTransferID, "после отмены единица свободна")
		s.Equal("at_warehouse", e.LocationStatus)
		s.Equal(almaty, strOr(e.WarehouseID), "из «в пути» — обратно на склад отправления")
	}

	// негатив: после отмены документа единица убирается из заказа…
	st, m, raw := s.doEquipment(s.admin, http.MethodDelete, removePath, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(m.Card.Equipment)
	// …но из справочника — нет: она числится на нашем складе (DS-041); строки отменённых
	// документов её уже не удерживают (DS-014).
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+eq, map[string]any{"reason": "Списана"})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("EQUIPMENT_AT_WAREHOUSE", s.errOf(raw).Code)
	// единица вне заказа сохраняет местоположение.
	var loc string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT location_status FROM equipment WHERE id = $1`, eq).Scan(&loc))
	s.Equal("at_warehouse", loc)
}

// TS-018 (смена клиента заказа, DS-026): чужие единицы прежнего клиента «тихо» убираются из
// заказа при смене client_id — но не те, что стоят в незавершённом документе: заявка и «в
// пути» → 409 EQUIPMENT_IN_TRANSFER с номером документа, заказ и единица не изменились;
// после отмены документа смена клиента проходит.
//
// Документы здесь — «склад → склад» БЕЗ заказа (transfers.task_id IS NULL): правило D84
// (клиент заказа с документами заказа не меняется, TASK_CLIENT_LOCKED_BY_TRANSFERS —
// TestTaskClientLockedByTransfers) их не касается, для них действует проверка по единицам.
// Поэтому единицы стоят на складе фикстурой SQL, а не приёмом «клиент → склад»: документ
// приёма был бы документом заказа и закрыл бы смену клиента сам по себе.
func (s *OrderCardLiveSuite) TestTS018_ClientChangeBlockedByOpenTransfer() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c1 := s.newClient("Клиент-Смена-С-Перемещением-Раз")
	c2 := s.newClient("Клиент-Смена-С-Перемещением-Два")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c1, col)
	free, moved := s.newEquipment(c1), s.newEquipment(c1)
	for _, eq := range []string{free, moved} {
		s.attachRaw(taskID, eq)
		_, err := s.Pool.Exec(context.Background(),
			`UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = $1`, eq, almaty) // фикстура местоположения
		s.Require().NoError(err)
	}
	s.Equal("0", s.revertScalar(`SELECT count(*)::text FROM transfers WHERE task_id = $1`, taskID), "документов заказа нет")

	changeClient := func() (int, []byte) {
		return s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
			"title": "Заказ", "client_id": c2, "column_id": col,
			"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
		})
	}
	state := func() string {
		var v string
		s.Require().NoError(s.Pool.QueryRow(context.Background(), `
			SELECT t.client_id::text || '|' || t.version::text || '|' ||
			       (SELECT count(*) FROM task_equipment te WHERE te.task_id = t.id)::text || '|' ||
			       e.location_status || '|' || coalesce(e.warehouse_id::text, '') || '|' || coalesce(e.active_transfer_id::text, '')
			FROM tasks t, equipment e WHERE t.id = $1 AND e.id = $2`, taskID, moved).Scan(&v))
		return v
	}

	var doc transferDocDTO
	for _, send := range []bool{false, true} { // заявка, затем «в пути»
		doc = s.newTransfer(almaty, moscow, send, moved)
		before := state()

		st, raw := changeClient()
		s.Require().Equal(http.StatusConflict, st, "send=%v: %s", send, raw)
		var te transferErrDTO
		s.Require().NoError(json.Unmarshal(raw, &te), string(raw))
		s.Equal("EQUIPMENT_IN_TRANSFER", te.Code)
		s.Equal(fmt.Sprintf("Оборудование в перемещении #%d — сначала отмените его", doc.ID), te.Message)
		s.Require().NotNil(te.Details)
		s.Equal(doc.ID, te.Details.TransferID)

		s.Equal(before, state(), "send=%v: заказ (клиент, версия, оборудование) и единица не изменились", send)
		_, card, _ := s.getCard(s.admin, taskID)
		s.Len(card.Equipment, 2, "свободная единица тоже осталась — смена клиента отклонена целиком")
		s.Equal(c1, card.Task.ClientID)

		if !send {
			s.transferAction(doc, "cancel") // освободить под следующий виток
		}
	}

	// после отмены документа («в пути» → обратно на склад отправления) смена клиента проходит,
	// чужие единицы убираются из заказа; местоположение на единице сохраняется.
	s.transferAction(doc, "cancel")
	st, raw := changeClient()
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, card, _ := s.getCard(s.admin, taskID)
	s.Equal(c2, card.Task.ClientID)
	s.Empty(card.Equipment, "оборудование прежнего клиента убрано из заказа")
	var loc string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT e.location_status || '|' || w.name || '|' || coalesce(e.active_transfer_id::text, '')
		FROM equipment e JOIN warehouses w ON w.id = e.warehouse_id WHERE e.id = $1`, moved).Scan(&loc))
	s.Equal("at_warehouse|Алматы|", loc)
}

// ---- TS-024 ----

type boardListItemDTO struct {
	ID           int64 `json:"id"`
	HasInTransit bool  `json:"has_in_transit"`
}

func (s *OrderCardLiveSuite) boardIDs(query string) map[int64]bool {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?assignee=all&"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []boardListItemDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	out := map[int64]bool{}
	for _, it := range l.Items {
		out[it.ID] = it.HasInTransit
	}
	return out
}

func keys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// TS-024: GET /tasks?warehouse_id=X — заказ с единицей на X и заказ с единицей в пути к X;
// has_in_transit — по equipment.location_status; единица «у клиента» не попадает ни под
// один склад; после приёмки бейджа «в пути» нет.
func (s *OrderCardLiveSuite) TestTS024_BoardWarehouseFilterAndTransitBadge() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Доска-Склад")
	col := s.columnID("Новые")
	atMoscow, _ := s.newTask(c, col)
	toMoscow, _ := s.newTask(c, col)
	atAlmaty, _ := s.newTask(c, col)
	atClient, _ := s.newTask(c, col)
	requested, _ := s.newTask(c, col)
	noEquipment, _ := s.newTask(c, col)

	s.addEq(atMoscow, s.newEquipment(c), moscow)
	eqTransit := s.newEquipment(c)
	s.addEq(toMoscow, eqTransit, almaty)
	doc := s.newTransfer(almaty, moscow, true, eqTransit)
	s.addEq(atAlmaty, s.newEquipment(c), almaty)
	s.attachRaw(atClient, s.newEquipment(c)) // единица «у клиента»
	eqRequested := s.newEquipment(c)
	s.addEq(requested, eqRequested, almaty)
	s.newTransfer(almaty, moscow, false, eqRequested) // заявка: единица ещё на Алматы

	got := s.boardIDs("client_id=" + c + "&warehouse_id=" + moscow)
	s.Equal([]int64{atMoscow, toMoscow}, keys(got), "Москва: на складе + едет в Москву")
	s.False(got[atMoscow])
	s.True(got[toMoscow], "бейдж «в пути»")

	got = s.boardIDs("client_id=" + c + "&warehouse_id=" + almaty)
	s.Equal([]int64{atAlmaty, requested}, keys(got), "Алматы: единица в пути из Алматы не считается, заявка — ещё на Алматы")
	s.False(got[requested], "заявка — не «в пути»")

	all := s.boardIDs("client_id=" + c)
	s.Len(all, 6, "без фильтра — все заказы")
	s.True(all[toMoscow])
	s.False(all[noEquipment])
	s.False(all[atClient], "единица «у клиента» — не «в пути»")
	for _, wh := range []string{almaty, moscow} {
		s.NotContains(keys(s.boardIDs("client_id="+c+"&warehouse_id="+wh)), atClient, "«у клиента» — ни под одним складом")
	}

	got = s.boardIDs("client_id=" + c + "&warehouse_id=" + moscow + "&per_column=1&column_id=" + col)
	s.Len(got, 1, "фильтр склада совместим с окном per_column/column_id")

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?warehouse_id=msk", nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Contains(s.fields(raw), "warehouse_id")

	// GET /tasks/:id тоже несёт has_in_transit (мутации доски перерисовывают карточку из него).
	st, raw = s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", toMoscow), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var one boardListItemDTO
	s.Require().NoError(json.Unmarshal(raw, &one))
	s.True(one.HasInTransit)

	// после приёмки: заказ по-прежнему под Москвой (единица уже на складе), бейджа нет.
	s.transferAction(doc, "receive")
	got = s.boardIDs("client_id=" + c + "&warehouse_id=" + moscow)
	s.Equal([]int64{atMoscow, toMoscow}, keys(got))
	s.False(got[toMoscow], "после приёмки бейджа «в пути» нет")
	st, raw = s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", toMoscow), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &one))
	s.False(one.HasInTransit)

	// SSR доски рендерит опции фильтра «Склад».
	st, raw = s.admin.Do(http.MethodGet, "/", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), `data-testid="filter-warehouse"`)
	s.Contains(string(raw), `value="`+moscow+`"`)
}

// equipmentSearchDTO — конверт GET /api/v1/equipment (§6.8).
type equipmentSearchDTO struct {
	Items []struct {
		ID               string  `json:"id"`
		ClientID         string  `json:"client_id"`
		ClientName       string  `json:"client_name"`
		TypeName         string  `json:"type_name"`
		Brand            string  `json:"brand"`
		Model            string  `json:"model"`
		Serial           string  `json:"serial"`
		LocationStatus   string  `json:"location_status"`
		WarehouseID      *string `json:"warehouse_id"`
		WarehouseName    *string `json:"warehouse_name"`
		ActiveTransferID *int64  `json:"active_transfer_id"`
	} `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (s *OrderCardLiveSuite) searchEquipment(query string) equipmentSearchDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment?"+query, nil)
	s.Require().Equal(http.StatusOK, st, "%s → %s", query, raw)
	var l equipmentSearchDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	return l
}

// TS-024 (доп., §6.8, DS-057): GET /api/v1/equipment — фильтры склад / местоположение / q по
// «И», конверт §API-1, сортировка клиент → модель; location_status: invalid → 400.
func (s *OrderCardLiveSuite) TestTS024_EquipmentSearchEndpoint() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("ТОО Поисковый Клиент ZX81")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	eqAlmaty, eqMoscow, eqClient, eqTransit := s.newEquipment(c), s.newEquipment(c), s.newEquipment(c), s.newEquipment(c)
	ctx := context.Background()
	for id, model := range map[string]string{eqAlmaty: "ZX81-Alpha", eqMoscow: "ZX81-Beta", eqClient: "ZX81-Gamma", eqTransit: "ZX81-Delta"} {
		_, err := s.Pool.Exec(ctx, `UPDATE equipment SET model = $2, serial = 'SER-' || $2 WHERE id = $1`, id, model)
		s.Require().NoError(err)
	}
	s.addEq(taskID, eqAlmaty, almaty)
	s.addEq(taskID, eqMoscow, moscow)
	s.addEq(taskID, eqTransit, almaty)
	doc := s.newTransfer(almaty, moscow, true, eqTransit)

	ids := func(l equipmentSearchDTO) []string {
		out := []string{}
		for _, it := range l.Items {
			out = append(out, it.Model)
		}
		return out
	}

	// q — по названию клиента, без учёта регистра; сортировка — клиент, затем модель.
	all := s.searchEquipment("q=zx81")
	s.Equal([]string{"ZX81-Alpha", "ZX81-Beta", "ZX81-Delta", "ZX81-Gamma"}, ids(all))
	s.Equal(4, all.Total)
	s.Equal(20, all.Limit, "лимит по умолчанию")
	s.Equal(0, all.Offset)
	first := all.Items[0]
	s.Equal(eqAlmaty, first.ID)
	s.Equal(c, first.ClientID)
	s.Equal("ТОО Поисковый Клиент ZX81", first.ClientName)
	s.Equal("Brand", first.Brand)
	s.NotEmpty(first.TypeName)
	s.Equal("at_warehouse", first.LocationStatus)
	s.Equal(almaty, strOr(first.WarehouseID))
	s.Equal("Алматы", strOr(first.WarehouseName))
	s.Nil(first.ActiveTransferID)

	// фильтры по «И»: склад отправления + at_warehouse (запрос «+ Оборудование», §8.1).
	s.Equal([]string{"ZX81-Alpha"}, ids(s.searchEquipment("q=ZX81&location_status=at_warehouse&warehouse_id="+almaty)))
	s.Equal([]string{"ZX81-Beta"}, ids(s.searchEquipment("q=ZX81&warehouse_id="+moscow)))
	s.Equal([]string{"ZX81-Gamma"}, ids(s.searchEquipment("q=ZX81&location_status=at_client")))
	transit := s.searchEquipment("q=ZX81&location_status=in_transit")
	s.Require().Equal([]string{"ZX81-Delta"}, ids(transit))
	s.Nil(transit.Items[0].WarehouseID, "в пути — склада нет")
	s.Require().NotNil(transit.Items[0].ActiveTransferID)
	s.Equal(doc.ID, *transit.Items[0].ActiveTransferID)
	s.Empty(ids(s.searchEquipment("q=ZX81&location_status=at_client&warehouse_id="+almaty)), "«у клиента» — ни на одном складе")

	// q — по модели и серийному номеру; спецсимволы LIKE экранируются.
	s.Equal([]string{"ZX81-Beta"}, ids(s.searchEquipment("q=zx81-be")))
	s.Equal([]string{"ZX81-Gamma"}, ids(s.searchEquipment("q=ser-zx81-gam")))
	s.Empty(ids(s.searchEquipment("q=ZX81%25")), "% — литерал, не шаблон")
	s.Empty(ids(s.searchEquipment("q=ZX8_-Alpha")), "_ — литерал, не шаблон")

	// пагинация: limit/offset, total — всего под фильтрами; limit > 100 → 100.
	page := s.searchEquipment("q=zx81&limit=2&offset=1")
	s.Equal([]string{"ZX81-Beta", "ZX81-Delta"}, ids(page))
	s.Equal(4, page.Total)
	s.Equal(2, page.Limit)
	s.Equal(1, page.Offset)
	s.Equal(100, s.searchEquipment("q=zx81&limit=5000").Limit)
	beyond := s.searchEquipment("q=zx81&offset=50")
	s.Empty(beyond.Items)
	s.Equal(4, beyond.Total, "total не зависит от offset")

	// валидация query → 400 по полю.
	for query, field := range map[string]string{
		"location_status=lost":                           "location_status",
		"location_status=AT_WAREHOUSE":                   "location_status",
		"warehouse_id=msk":                               "warehouse_id",
		"q=" + url.QueryEscape(strings.Repeat("я", 101)): "q",
		"warehouse_id=%7B" + almaty + "%7D":              "warehouse_id",
	} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment?"+query, nil)
		s.Equal(http.StatusBadRequest, st, "%s → %s", query, raw)
		s.Contains(s.fields(raw), field, query)
	}
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment?q="+url.QueryEscape(strings.Repeat("я", 101)), nil)
	s.Equal(http.StatusBadRequest, st)
	s.Equal("too_long", s.fields(raw)["q"])
	s.Empty(s.searchEquipment("q="+url.QueryEscape(strings.Repeat("я", 100))).Items, "100 символов — допустимо")

	// доступ: любой авторизованный (исполнитель — да), без сессии — 401.
	s.SeedUser("eq-search-exec", "password1", "Поиск Исполнитель", "executor", true)
	exec := s.NewBrowser()
	exec.Login("eq-search-exec", "password1")
	st, raw = exec.Do(http.MethodGet, "/api/v1/equipment?q=zx81", nil)
	s.Equal(http.StatusOK, st, string(raw))
	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/equipment", nil)
	s.Equal(http.StatusUnauthorized, st)
}

// ---- DS-046: склад по умолчанию строки товара (D58) ----

// Склад по умолчанию строки товара = склад единицы оборудования услуги (equipment), если она
// на складе и склад активен; в пути / у клиента — первый активный.
func (s *OrderCardLiveSuite) TestEquipmentLocation_DefaultProductWarehouseFromEquipment() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Оборудование-Товар")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	eq := s.newEquipment(c)
	s.addEq(taskID, eq, moscow)
	subID := s.addSubtask(taskID, s.newService("100"), "100")
	s.attachSubtaskEquipment(subID, eq)
	productID := s.newProduct("10", "10", "0")

	st, m, raw := s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Products, 1)
	s.Equal(moscow, m.Card.Subtasks[0].Products[0].WarehouseID, "склад единицы оборудования услуги")

	// единица уехала документом Москва → Алматы: в пути → первый активный склад.
	doc := s.newTransfer(moscow, almaty, true, eq)
	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Products, 2)
	s.Equal(almaty, m.Card.Subtasks[0].Products[1].WarehouseID, "оборудование в пути → первый активный")
	s.Equal(moscow, m.Card.Subtasks[0].Products[0].WarehouseID, "существующая строка не пересчитывается")
	s.transferAction(doc, "cancel") // обратно на Москву

	// единица «у клиента» (в заказе без приёма на склад) → первый активный.
	other, _ := s.newTask(c, s.columnID("Новые"))
	atClient := s.newEquipment(c)
	s.attachRaw(other, atClient)
	sub2 := s.addSubtask(other, s.newService("100"), "100")
	s.attachSubtaskEquipment(sub2, atClient)
	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+sub2+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(almaty, m.Card.Subtasks[0].Products[0].WarehouseID, "единица у клиента → первый активный")
	s.Equal("at_client", s.cardEquipment(m.Card, atClient).LocationStatus)
	s.Nil(s.cardEquipment(m.Card, atClient).WarehouseID)
}

// Enum местоположения совпадает с CHECK миграции 0014 (pg_get_constraintdef).
func (s *OrderCardLiveSuite) TestEquipmentLocation_EnumMatchesDBCheck() {
	var def string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'equipment_location_status_check'`).Scan(&def))
	for _, v := range board.EquipmentLocations {
		s.Contains(def, "'"+v+"'")
	}
	s.Equal(len(board.EquipmentLocations), strings.Count(def, "'::text"), "в CHECK нет значений сверх enum: %s", def)
}

// Код-ревью MEDIUM-1 (warehouses): склад строки товара в верхнем регистре = текущий → без
// смены склада и без пересчёта списания; склад приёма оборудования в верхнем регистре
// принимается.
func (s *OrderCardLiveSuite) TestEquipmentLocation_UppercaseUUIDs() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-UUID-Регистр")
	taskID, _ := s.newTask(c, s.columnID("В работе"))
	eq := s.newEquipment(c)
	m := s.addEq(taskID, eq, strings.ToUpper(moscow))
	s.Equal(moscow, strOr(s.cardEquipment(m.Card, eq).WarehouseID))

	subID := s.addSubtask(taskID, s.newService("100"), "100")
	productID := s.newProduct("10", "10", "0")
	st, mc, raw := s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products",
		map[string]any{"product_id": productID, "qty": 2, "warehouse_id": strings.ToUpper(almaty)})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	rowID := mc.Card.Subtasks[0].Products[0].ID
	s.Equal(almaty, mc.Card.Subtasks[0].Products[0].WarehouseID)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtask-products/"+rowID,
		map[string]any{"qty": 2, "price": 10, "warehouse_id": strings.ToUpper(almaty)})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(map[string]int{"Алматы:task_consume": 1}, s.movements(taskID), "тот же склад — без пересчёта")
}
