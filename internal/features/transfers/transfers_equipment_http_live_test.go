//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Строки-единицы оборудования клиентов в документе «Перемещение» (transfers-core DS-001,
// DS-008, DS-019, DS-036, DS-037, DS-041) — методы TransfersLiveSuite.

// newUnit — единица оборудования клиента фикстуры: warehouseID "" — «у клиента», иначе
// «на складе» warehouseID (SQL: фикстура местоположения, не предмет теста).
func (s *TransfersLiveSuite) newUnit(warehouseID string) string {
	s.seq++
	ctx := context.Background()
	var id string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO equipment (client_id, type_id, brand, model, serial)
		VALUES ($1, $2, 'Bodor', $3, $4) RETURNING id::text`,
		s.clientID, s.typeID, fmt.Sprintf("ST-%d", s.seq), fmt.Sprintf("SN-ST-%d", s.seq)).Scan(&id))
	if warehouseID != "" {
		_, err := s.Pool.Exec(ctx, `UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = $1`, id, warehouseID)
		s.Require().NoError(err)
	}
	return id
}

// unitLoc — «статус|склад|документ» единицы (склад — имя, пусто — нет; документ — номер).
func (s *TransfersLiveSuite) unitLoc(id string) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT e.location_status || '|' || coalesce(w.name, '') || '|' || coalesce(e.active_transfer_id::text, '')
		FROM equipment e LEFT JOIN warehouses w ON w.id = e.warehouse_id WHERE e.id = $1`, id).Scan(&v))
	return v
}

func (s *TransfersLiveSuite) transfersCount() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM transfers`).Scan(&n))
	return n
}

func unitItem(id string) map[string]any {
	return map[string]any{"kind": "equipment", "equipment_id": id}
}

func productItem(id string, qty any) map[string]any {
	return map[string]any{"kind": "product", "product_id": id, "qty_sent": qty}
}

// body — тело документа «склад → склад» Алматы → Москва без direction: годится и для PUT
// (direction в правке запрещён), а при создании через do/create направление подставляется.
func (s *TransfersLiveSuite) body(items ...map[string]any) map[string]any {
	return map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items}
}

// w2wBody — тело создания «склад → склад» с явным direction (запросы мимо хелпера do).
func (s *TransfersLiveSuite) w2wBody(items ...map[string]any) map[string]any {
	b := s.body(items...)
	b["direction"] = "warehouse_to_warehouse"
	return b
}

// ---- DS-001 / DS-037: смешанный документ ----

// Смешанный документ: заявка занимает единицу, отправка → «в пути» (склад пуст), приёмка —
// товар по факту, единица целиком на склад назначения и свободна. Факт по строке-единице →
// 400 not_allowed.
func (s *TransfersLiveSuite) TestMixedDocument_RequestSendReceive() {
	p := s.newProduct("10")
	u := s.newUnit(s.almaty)

	t := s.create(s.body(unitItem(u), productItem(p, 4)))
	s.Equal("requested", t.Status)
	s.Require().Len(t.Items, 2)
	s.Equal("equipment", t.Items[0].Kind)
	s.Equal(u, t.Items[0].EquipmentID)
	s.Contains(t.Items[0].EquipmentName, "Лазерный станок ST Bodor ST-", "«тип марка модель»")
	s.Contains(t.Items[0].EquipmentSerial, "SN-ST-")
	s.Equal("ТОО Перемещаемый", t.Items[0].EquipmentClientName)
	s.Nil(t.Items[0].QtySent, "у строки-единицы количества нет")
	s.Empty(t.Items[0].ProductID)
	s.Equal("product", t.Items[1].Kind)
	s.Empty(t.Items[1].EquipmentID)
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", t.ID), s.unitLoc(u), "заявка занимает единицу, местоположение прежнее")

	// список считает строки обоих видов.
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/transfers?status=requested&warehouse_id="+s.moscow, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l listDTO
	s.Require().NoError(json.Unmarshal(raw, &l))
	found := false
	for _, it := range l.Items {
		if it.ID == t.ID {
			found = true
			s.Equal(2, it.ItemsCount, "товары + единицы")
		}
	}
	s.True(found)

	st, sent, raw := s.action(t, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("in_transit", sent.Status)
	s.Equal(fmt.Sprintf("in_transit||%d", t.ID), s.unitLoc(u))
	s.Equal("6.000", s.pairStock(p, s.almaty))
	s.Equal("4.000", s.inTransit(p, s.moscow), "«в пути» на остатках — только товарные строки")

	// факт по строке-единице не вводится.
	st, _, raw = s.action(sent, "receive", map[string]any{"items": []map[string]any{{"id": sent.Items[0].ID, "qty_received": 1}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].id": "not_allowed"}, s.fields(raw))
	s.Equal(fmt.Sprintf("in_transit||%d", t.ID), s.unitLoc(u), "неудачная приёмка ничего не меняет")

	st, rec, raw := s.action(sent, "receive", map[string]any{"items": []map[string]any{{"id": sent.Items[1].ID, "qty_received": 3}}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", rec.Status)
	s.Nil(rec.Items[0].QtyReceived, "единица принимается целиком, факта нет")
	s.Require().NotNil(rec.Items[1].QtyReceived)
	s.Equal("3.000", rec.Items[1].QtyReceived.String())
	s.Equal("at_warehouse|Москва|", s.unitLoc(u), "единица на складе назначения и свободна")
	s.Equal("3.000", s.pairStock(p, s.moscow))
	s.ElementsMatch([]string{"transfer_out:Алматы:-4.000", "transfer_in:Москва:4.000", "transfer_loss:Москва:-1.000"}, s.movements(t.ID),
		"движения — только по товарным строкам")

	// GET отдаёт тот же объект.
	st, got, raw := s.do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", t.ID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(rec.Version, got.Version)
	s.Len(got.Items, 2)
}

// Документ только из единиц (без товаров): send/receive не трогают остатки и движения.
func (s *TransfersLiveSuite) TestEquipmentOnlyDocument_CreateAndSend() {
	u1, u2 := s.newUnit(s.almaty), s.newUnit(s.almaty)
	body := s.body(unitItem(u1), unitItem(u2))
	body["send"] = true
	t := s.create(body)
	s.Equal("in_transit", t.Status)
	s.Empty(t.Warnings)
	s.Empty(s.movements(t.ID))
	s.Equal(fmt.Sprintf("in_transit||%d", t.ID), s.unitLoc(u1))
	s.Equal(fmt.Sprintf("in_transit||%d", t.ID), s.unitLoc(u2))

	st, rec, raw := s.action(t, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", rec.Status)
	s.Equal("at_warehouse|Москва|", s.unitLoc(u1))
	s.Equal("at_warehouse|Москва|", s.unitLoc(u2))
	s.Empty(s.movements(t.ID))
}

// Отмена: заявка — единица свободна на прежнем месте; «в пути» — возврат на склад
// отправления (вместе с товаром) и освобождение (DS-019).
func (s *TransfersLiveSuite) TestCancel_ReleasesAndReturnsEquipment() {
	p := s.newProduct("5")
	u := s.newUnit(s.almaty)

	req := s.create(s.body(unitItem(u)))
	st, c1, raw := s.action(req, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", c1.Status)
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u))

	body := s.body(unitItem(u), productItem(p, 2))
	body["send"] = true
	tr := s.create(body)
	s.Equal(fmt.Sprintf("in_transit||%d", tr.ID), s.unitLoc(u))
	st, c2, raw := s.action(tr, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", c2.Status)
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u), "единица вернулась на склад отправления")
	s.Equal("5.000", s.pairStock(p, s.almaty))
	s.ElementsMatch([]string{"transfer_out:Алматы:-2.000", "transfer_return:Алматы:2.000"}, s.movements(tr.ID))
}

// ---- TS-005 ----

// TS-005: вторая заявка с той же единицей → 409 EQUIPMENT_IN_TRANSFER с details.transfer_id;
// после отмены первой — успех; снятая в PUT единица освобождается; занятая единица в PUT —
// 409 без изменений документа.
func (s *TransfersLiveSuite) TestTS005_EquipmentClaimedByOneOpenDocument() {
	u1, u2 := s.newUnit(s.almaty), s.newUnit(s.almaty)

	first := s.create(s.body(unitItem(u1)))
	before := s.transfersCount()

	st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", s.body(unitItem(u1)))
	s.Require().Equal(http.StatusConflict, st, string(raw))
	e := s.errOf(raw)
	s.Equal("EQUIPMENT_IN_TRANSFER", e.Code)
	s.Equal(fmt.Sprintf("Оборудование уже в перемещении #%d", first.ID), e.Message)
	s.Require().NotNil(e.Details)
	s.Equal(first.ID, e.Details.TransferID)
	s.Equal(u1, e.Details.EquipmentID)
	s.Equal(before, s.transfersCount(), "документ не создан")

	// после отмены первой заявки единица свободна — вторая создаётся.
	st, _, raw = s.action(first, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u1))
	second := s.create(s.body(unitItem(u1), unitItem(u2)))
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", second.ID), s.unitLoc(u1))
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", second.ID), s.unitLoc(u2))

	// PUT без u1 — она освобождается, u2 остаётся занятой.
	put := s.body(unitItem(u2))
	put["version"] = second.Version
	st, upd, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", second.ID), put)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Len(upd.Items, 1)
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u1), "снятая в PUT единица свободна")
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", second.ID), s.unitLoc(u2))

	// свободную u1 занимает третий документ; вернуть её во второй через PUT — 409, документ цел.
	third := s.create(s.body(unitItem(u1)))
	put = s.body(unitItem(u2), unitItem(u1))
	put["version"] = upd.Version
	st, _, raw = s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", second.ID), put)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	e = s.errOf(raw)
	s.Equal("EQUIPMENT_IN_TRANSFER", e.Code)
	s.Require().NotNil(e.Details)
	s.Equal(third.ID, e.Details.TransferID)
	s.Equal(u1, e.Details.EquipmentID)
	_, cur, _ := s.do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", second.ID), nil)
	s.Equal(upd.Version, cur.Version, "неудачный PUT не меняет документ")
	s.Len(cur.Items, 1)
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", second.ID), s.unitLoc(u2), "u2 по-прежнему занята вторым")
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", third.ID), s.unitLoc(u1))

	// приёмка освобождает единицу — её можно ставить в новый документ (уже с Москвы).
	st, sent, raw := s.action(third, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	back := s.create(map[string]any{"from_warehouse_id": s.moscow, "to_warehouse_id": s.almaty, "items": []map[string]any{unitItem(u1)}})
	s.Equal(fmt.Sprintf("at_warehouse|Москва|%d", back.ID), s.unitLoc(u1))
}

// TS-005 (негатив, abuse case 2): два и более параллельных POST с одной единицей → ровно
// один 201, остальные 409 EQUIPMENT_IN_TRANSFER (не 500 от взаимной блокировки).
func (s *TransfersLiveSuite) TestTS005_ParallelCreateSameEquipment() {
	u := s.newUnit(s.almaty)
	p := s.newProduct("100")
	const n = 6
	var wg sync.WaitGroup
	codes := make([]int, n)
	bodies := make([][]byte, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := s.w2wBody(productItem(p, 1), unitItem(u))
			body["send"] = i%2 == 0 // вперемешку «заявка» и «сразу отправить»
			codes[i], bodies[i] = s.admin.Do(http.MethodPost, "/api/v1/transfers", body)
		}(i)
	}
	wg.Wait()
	created := 0
	for i, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			s.Equal("EQUIPMENT_IN_TRANSFER", s.errOf(bodies[i]).Code, string(bodies[i]))
		default:
			s.Failf("неожиданный статус", "%d: %s", c, bodies[i])
		}
	}
	s.Equal(1, created, "ровно один документ занял единицу")
	var docs int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transfer_items WHERE equipment_id = $1`, u).Scan(&docs))
	s.Equal(1, docs, "единица — строкой ровно в одном документе")
}

// ---- TS-006 ----

// TS-006: отправка единицы не со склада отправления (на другом складе либо «у клиента») →
// 409 TRANSFER_EQUIPMENT_NOT_AT_SOURCE; остатки и местоположение не изменились.
func (s *TransfersLiveSuite) TestTS006_SendEquipmentNotAtSource() {
	p := s.newProduct("10")
	for name, wh := range map[string]string{"на другом складе": s.moscow, "у клиента": ""} {
		u := s.newUnit(wh)
		locBefore := s.unitLoc(u)

		// заявка создаётся (занятие не зависит от местоположения), отправка — 409.
		t := s.create(s.body(productItem(p, 3), unitItem(u)))
		st, _, raw := s.action(t, "send", nil)
		s.Require().Equal(http.StatusConflict, st, "%s: %s", name, raw)
		e := s.errOf(raw)
		s.Equal("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", e.Code, name)
		s.Equal("Оборудование находится не на складе отправления", e.Message)
		s.Require().NotNil(e.Details)
		s.Equal(u, e.Details.EquipmentID)
		s.Equal("10.000", s.pairStock(p, s.almaty), "%s: остаток не списан", name)
		s.Empty(s.movements(t.ID), name)
		_, cur, _ := s.do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", t.ID), nil)
		s.Equal("requested", cur.Status, name)
		s.Equal(t.Version, cur.Version, "%s: версия не выросла", name)
		// местоположение прежнее, единица занята заявкой.
		wantLoc := locBefore + fmt.Sprint(t.ID)
		s.Equal(wantLoc, s.unitLoc(u), name)
		st, _, raw = s.action(cur, "cancel", nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Equal(locBefore, s.unitLoc(u), "%s: после отмены единица свободна на прежнем месте", name)

		// создание сразу с send — 409, документ не создан, единица не занята.
		before := s.transfersCount()
		body := s.body(productItem(p, 3), unitItem(u))
		body["send"] = true
		st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", body)
		s.Require().Equal(http.StatusConflict, st, "%s: %s", name, raw)
		s.Equal("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", s.errOf(raw).Code)
		s.Equal(before, s.transfersCount(), "%s: документ не создан", name)
		s.Equal(locBefore, s.unitLoc(u), name)
		s.Equal("10.000", s.pairStock(p, s.almaty))
	}
}

// ---- TS-009 ----

// TS-009: устаревшая version → 409 TRANSFER_VERSION_CONFLICT и сверяется ДО статуса; каждый
// недопустимый переход §5 → 409 TRANSFER_STATUS_CONFLICT со своим сообщением.
func (s *TransfersLiveSuite) TestTS009_VersionBeforeStatusAndTransitionMessages() {
	p := s.newProduct("50")
	mk := func(send bool) transferDTO {
		body := s.body(productItem(p, 1), unitItem(s.newUnit(s.almaty)))
		body["send"] = send
		return s.create(body)
	}
	put := func(t transferDTO, version int) (int, []byte) {
		body := s.body(productItem(p, 1))
		body["version"] = version
		st, _, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), body)
		return st, raw
	}
	expect := func(st int, raw []byte, code, msg, ctx string) {
		s.Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
		e := s.errOf(raw)
		s.Equal(code, e.Code, ctx)
		s.Equal(msg, e.Message, ctx)
	}

	inTransit := mk(true)
	received := mk(true)
	st, rcv, raw := s.action(received, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	cancelled := mk(false)
	st, cnc, raw := s.action(cancelled, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	requested := mk(false)

	// version сверяется ДО статуса: устаревшая версия на документе в неподходящем статусе —
	// конфликт версии, а не статуса.
	const versionMsg = "Перемещение уже изменено, обновите"
	for _, act := range []string{"send", "receive", "cancel"} {
		st, _, raw := s.action(rcv, act, map[string]any{"version": rcv.Version - 1})
		expect(st, raw, "TRANSFER_VERSION_CONFLICT", versionMsg, "stale "+act+" on received")
		st, _, raw = s.action(requested, act, map[string]any{"version": requested.Version + 7})
		expect(st, raw, "TRANSFER_VERSION_CONFLICT", versionMsg, "future "+act+" on requested")
	}
	st, raw = put(cnc, cnc.Version-1)
	expect(st, raw, "TRANSFER_VERSION_CONFLICT", versionMsg, "stale PUT on cancelled")

	// недопустимые переходы с актуальной версией — свой текст на каждый.
	st, raw = put(inTransit, inTransit.Version)
	expect(st, raw, "TRANSFER_STATUS_CONFLICT", "Перемещение уже отправлено — править нельзя", "PUT in_transit")
	st, raw = put(rcv, rcv.Version)
	expect(st, raw, "TRANSFER_STATUS_CONFLICT", "Перемещение уже получено — править нельзя", "PUT received")
	st, raw = put(cnc, cnc.Version)
	expect(st, raw, "TRANSFER_STATUS_CONFLICT", "Перемещение отменено — править нельзя", "PUT cancelled")

	for _, tc := range []struct {
		doc      transferDTO
		act, msg string
	}{
		{inTransit, "send", "Перемещение уже отправлено"},
		{rcv, "send", "Перемещение уже получено"},
		{cnc, "send", "Перемещение отменено"},
		{requested, "receive", "Перемещение ещё не отправлено"},
		{rcv, "receive", "Перемещение уже получено"},
		{cnc, "receive", "Перемещение отменено"},
		// transfers-revert (хвост TS-009): «Получено» не окончательно — путь к отмене назван.
		{rcv, "cancel", "Перемещение получено — сначала отмените проведение"},
		{cnc, "cancel", "Перемещение уже отменено"},
	} {
		st, _, raw := s.action(tc.doc, tc.act, nil)
		expect(st, raw, "TRANSFER_STATUS_CONFLICT", tc.msg, tc.act+" on "+tc.doc.Status)
	}

	// version обязателен на каждом действии и на правке.
	for _, act := range []string{"send", "receive", "cancel"} {
		st, _, raw := s.do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/%s", requested.ID, act), map[string]any{})
		s.Equal(http.StatusBadRequest, st, string(raw))
		s.Equal(map[string]string{"version": "required"}, s.fields(raw))
	}
	st, _, raw = s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", requested.ID), s.body(productItem(p, 1)))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"version": "required"}, s.fields(raw))
}

// TS-009 (негатив, abuse case 4): параллельные receive смешанного документа → один 200,
// остальные 409; остаток получателя вырос один раз, единица принята один раз.
func (s *TransfersLiveSuite) TestTS009_ParallelReceiveOnce() {
	p := s.newProduct("10")
	u := s.newUnit(s.almaty)
	body := s.body(productItem(p, 2), unitItem(u))
	body["send"] = true
	t := s.create(body)

	var wg sync.WaitGroup
	codes := make([]int, 5)
	bodies := make([][]byte, 5)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], bodies[i] = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", t.ID), map[string]any{"version": t.Version})
		}(i)
	}
	wg.Wait()
	ok := 0
	for i, c := range codes {
		if c == http.StatusOK {
			ok++
			continue
		}
		s.Equal(http.StatusConflict, c, string(bodies[i]))
		s.Contains([]string{"TRANSFER_VERSION_CONFLICT", "TRANSFER_STATUS_CONFLICT"}, s.errOf(bodies[i]).Code)
	}
	s.Equal(1, ok, "ровно одна приёмка")
	s.Equal("2.000", s.pairStock(p, s.moscow), "остаток получателя вырос один раз")
	s.Equal("at_warehouse|Москва|", s.unitLoc(u), "единица принята один раз")
	s.ElementsMatch([]string{"transfer_out:Алматы:-2.000", "transfer_in:Москва:2.000"}, s.movements(t.ID))
}

// ---- TS-028 ----

// TS-028: произвольное/мусорное тело POST /api/v1/transfers на живом сервере — только
// 201/400/409 (у исполнителя — ещё 403 на документ без клиента), 500 недопустим; abuse case 3 (qty_sent у единицы,
// equipment_id у товара) — 400, документ не создан.
func (s *TransfersLiveSuite) TestTS028_CreateArbitraryJSON_No5xx() {
	p := s.newProduct("5")
	u := s.newUnit(s.almaty)
	wh := fmt.Sprintf(`"direction":"warehouse_to_warehouse","from_warehouse_id":%q,"to_warehouse_id":%q`, s.almaty, s.moscow)
	before := s.transfersCount()
	bad := []string{
		``, `{`, `null`, `[]`, `"x"`, `0`, `{"items":"x"}`, `{"items":{}}`, `{"items":[null]}`, `{"items":[[]]}`, `{"items":[1]}`,
		`{"send":"yes"}`, `{"note":{}}`, `{"from_warehouse_id":1}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":1}],"unknown_field":[1,2,3]}`,
		`{` + wh + `}`,
		`{` + wh + `,"items":[]}`,
		`{` + wh + `,"items":[{}]}`,
		`{` + wh + `,"items":[{"kind":7}]}`,
		`{` + wh + `,"items":[{"kind":"service","product_id":"` + p + `","qty_sent":1}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":"1"}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":1e400}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":1e3}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":99999999999999999999}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":-0}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"urn:uuid:` + p + `","qty_sent":1}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"{` + p + `}","qty_sent":1}]}`,
		`{` + wh + `,"items":[{"kind":"equipment","equipment_id":"` + u + `","qty_sent":1}]}`,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":1,"equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"items":[{"kind":"equipment","equipment_id":"` + u + `"},{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"note":"a\u0000b","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{"direction":"warehouse_to_warehouse","from_warehouse_id":"{` + s.almaty + `}","to_warehouse_id":"` + s.moscow + `","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{"direction":"warehouse_to_warehouse","from_warehouse_id":"` + s.almaty + `","to_warehouse_id":"` + s.almaty + `","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		// transfers-partners: направление — мусор / не из перечня / чужие точки маршрута.
		`{"direction":7,"items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{"direction":"sideways","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{"direction":null,"from_warehouse_id":"` + s.almaty + `","to_warehouse_id":"` + s.moscow + `","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"task_id":"7","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"task_id":1,"supplier_id":"x","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"task_id":1e30,"items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"receive":true,"items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{` + wh + `,"receive":"yes","items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{"direction":"client_to_warehouse","to_warehouse_id":"` + s.moscow + `","task_id":999999999,"items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`,
		`{"direction":"supplier_to_warehouse","to_warehouse_id":"` + s.moscow + `","supplier_id":"00000000-0000-0000-0000-000000000000","items":[{"kind":"product","product_id":"` + p + `","qty_sent":1}]}`,
	}
	for _, body := range bad {
		resp := s.admin.Request(http.MethodPost, "/api/v1/transfers", body, nil)
		_ = resp.Body.Close()
		s.Equal(http.StatusBadRequest, resp.StatusCode, "тело %s", body)
	}
	s.Equal(before, s.transfersCount(), "ни один мусорный запрос не создал документ")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u), "единица не занята")

	// корректные тела — 201; повтор с занятой единицей — 409, не 500.
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":1.5}]}`, http.StatusCreated},
		{`{` + wh + `,"items":[{"kind":"equipment","equipment_id":"` + u + `","qty_sent":null,"product_id":null}],"send":true}`, http.StatusCreated},
		{`{` + wh + `,"items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`, http.StatusConflict},
	} {
		resp := s.admin.Request(http.MethodPost, "/api/v1/transfers", tc.body, nil)
		_ = resp.Body.Close()
		s.Equal(tc.want, resp.StatusCode, "тело %s", tc.body)
	}

	// исполнитель (transfers-partners DS-018): мусор — 400, документ без клиента — 403, 5xx нет.
	s.SeedUser("st-fuzz-exec", "password1", "Фазз Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("st-fuzz-exec", "password1")
	for body, want := range map[string]int{
		`{}`:   http.StatusBadRequest,
		`null`: http.StatusBadRequest,
		`{` + wh + `,"items":[{"kind":"product","product_id":"` + p + `","qty_sent":1}]}`:                                   http.StatusForbidden,
		`{"direction":"supplier_to_warehouse"}`:                                                                             http.StatusForbidden,
		`{"direction":"warehouse_to_warehouse","items":"x"}`:                                                                http.StatusBadRequest,
		`{"direction":"client_to_warehouse","task_id":999999999,"items":[{"kind":"equipment","equipment_id":"` + u + `"}]}`: http.StatusBadRequest,
	} {
		resp := exec.Request(http.MethodPost, "/api/v1/transfers", body, nil)
		_ = resp.Body.Close()
		s.Equal(want, resp.StatusCode, "executor, тело %s", body)
	}
}
