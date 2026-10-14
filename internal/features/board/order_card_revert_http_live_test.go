//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Отмена проведения и удаление в карточке заказа
// (docs/features/transfers-revert/draft-spec.md §11) — методы OrderCardLiveSuite (§TD-8):
// TS-013 (откат накладной оплаченного заказа), TS-017 (удаление заказа с документами), хвост
// TS-018 (удаление документа-заявки освобождает единицу). Документы создаются и проводятся
// через API перемещений того же сервера.

// unreceiveTransfer — «Отменить проведение» документа (актуальная версия) с причиной.
func (s *OrderCardLiveSuite) unreceiveTransfer(id int64, version int) salesTransferDTO {
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/unreceive", id),
		map[string]any{"version": version, "reason": "Ошибочное проведение"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var out salesTransferDTO
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	s.Require().Equal("in_transit", out.Status)
	return out
}

func (s *OrderCardLiveSuite) revertScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *OrderCardLiveSuite) stockAt(productID, warehouseID string) string {
	return s.revertScalar(`SELECT stock::text FROM product_stock WHERE product_id = $1 AND warehouse_id = $2`, productID, warehouseID)
}

// deleteReason — тело мягкого удаления заказа (soft-delete-core DS-020: причина обязательна).
var deleteReason = map[string]any{"reason": "Создан по ошибке"}

// deleteTransfer — мягкое удаление документа «Перемещение» (soft-delete-transfers §6.1):
// причина и version — в теле.
func (s *OrderCardLiveSuite) deleteTransfer(id int64, version int) (int, []byte) {
	return s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/transfers/%d", id),
		map[string]any{"reason": "Создан по ошибке", "version": version})
}

func (s *OrderCardLiveSuite) mustDeleteTransfer(id int64, version int) {
	st, raw := s.deleteTransfer(id, version)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// ---- TS-013 ----

// TS-013: откат полученной накладной оплаченного заказа разрешён (DS-020 / DS-023): 200,
// is_paid заказа не изменился, total_amount по-прежнему включает накладную (она «В пути»),
// товар остаётся списанным (движений отката нет). Негатив: последующая отмена накладной
// убирает её сумму из итога и возвращает товар на склад; отметка «Оплачен» остаётся.
func (s *OrderCardLiveSuite) TestTS013_UnreceiveInvoiceOfPaidOrder() {
	ctx := context.Background()
	almaty := s.warehouseID("Алматы")
	c := s.newClient("Клиент-Откат-Накладной")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	svc := s.newService("1000")
	st, _, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svc})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	goods := s.newProduct("100.10", "20", "1")

	inv := s.invoice(taskID, true, [3]any{goods, 3, 100.10})
	inv = s.invoiceAction(inv, "receive")
	s.Require().Equal("received", inv.Status)
	_, err := s.Pool.Exec(ctx, `UPDATE tasks SET is_paid = true, paid_at = now() WHERE id = $1`, taskID)
	s.Require().NoError(err)
	total, sold, paid, _ := s.totals(taskID, c)
	s.Equal("1300.30", total, "1000 + 300.30")
	s.Equal("300.30", sold)
	s.True(paid)
	s.Equal("17.000", s.stockAt(goods, almaty))

	// откат: документ «В пути», сумма остаётся в итоге, оплата не тронута, товар списан.
	back := s.unreceiveTransfer(inv.ID, inv.Version)
	s.Equal("300.30", back.Amount.String())
	total, sold, paid, card := s.totals(taskID, c)
	s.Equal("1300.30", total, "накладная «В пути» остаётся в итоге заказа")
	s.Equal("300.30", sold)
	s.True(paid, "отметка «Оплачен» откатом не трогается")
	s.Require().Len(card.Transfers, 1)
	s.Equal("in_transit", card.Transfers[0].Status)
	s.Equal("17.000", s.stockAt(goods, almaty), "товар накладной остаётся списанным")
	s.Equal("client_out", s.revertScalar(`SELECT string_agg(reason, ',' ORDER BY reason) FROM stock_movements WHERE transfer_id = $1`, inv.ID),
		"откат накладной движений не создаёт")
	s.Equal("t", s.revertScalar(`SELECT (is_paid AND paid_at IS NOT NULL)::text FROM tasks WHERE id = $1`, taskID)[:1])

	// негатив: отмена накладной после отката — сумма уходит из итога, товар возвращается.
	cnc := s.invoiceAction(back, "cancel")
	s.Equal("cancelled", cnc.Status)
	total, sold, paid, _ = s.totals(taskID, c)
	s.Equal("1000.00", total, "отменённая накладная в итог не входит")
	s.Equal("0.00", sold)
	s.True(paid, "отметка «Оплачен» остаётся и после отмены накладной")
	s.Equal("20.000", s.stockAt(goods, almaty), "товар вернулся на склад отправления")
}

// ---- TS-014 (soft-delete-transfers; заменяет TS-017 transfers-revert и TS-048 части 1) ----

// TS-014 (DS-006, DS-008): DELETE /api/v1/tasks/:id — заказ удаляется (мягко) с документами в
// ЛЮБОМ статусе: заявка / в пути / получено / отменён → 200. Документы, их строки, остатки,
// движения и местоположение единиц не изменились; документы остаются в «Перемещениях» с
// task_is_deleted = true и в карточку восстановленного заказа возвращаются. Негатив: ответа
// 409 TASK_HAS_TRANSFERS больше нет (отказ части 1 DS-034 снят).
func (s *OrderCardLiveSuite) TestTS014_DeleteTaskWithTransfersAnyStatus() {
	almaty := s.warehouseID("Алматы")
	c := s.newClient("Клиент-Удаление-С-Перемещениями")
	col := s.columnID("Новые")
	// docState — документ (статус, версия, отметка удаления, число строк) и его единица.
	docState := func(docID int64, eq string) string {
		return s.revertScalar(`
			SELECT d.status || '|' || d.version::text || '|' || (d.deleted_at IS NOT NULL)::text || '|' ||
			       (SELECT count(*) FROM transfer_items i WHERE i.transfer_id = d.id)::text || '|' ||
			       e.location_status || '|' || coalesce(e.warehouse_id::text, '') || '|' || coalesce(e.active_transfer_id::text, '')
			FROM transfers d, equipment e WHERE d.id = $1 AND e.id = $2`, docID, eq)
	}

	for _, tc := range []struct {
		name   string
		flags  map[string]any
		cancel bool
		status string
	}{
		{"заявка", nil, false, "requested"},
		{"в пути", map[string]any{"send": true}, false, "in_transit"},
		{"получено", map[string]any{"receive": true}, false, "received"},
		{"отменён", nil, true, "cancelled"},
	} {
		taskID, _ := s.newTask(c, col)
		eq := s.newEquipment(c)
		s.addEq(taskID, eq, "")
		doc := s.clientTransfer("client_to_warehouse", taskID, almaty, tc.flags, eq)
		if tc.cancel {
			doc = s.transferAction(doc, "cancel")
		}
		s.Require().Equal(tc.status, doc.Status, tc.name)
		before := docState(doc.ID, eq)

		st, raw := s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), deleteReason)
		s.Require().Equal(http.StatusOK, st, "%s: %s", tc.name, raw)
		s.NotContains(string(raw), "TASK_HAS_TRANSFERS", tc.name)
		st, _, _ = s.getCard(s.admin, taskID)
		s.Equal(http.StatusNotFound, st, "%s: заказ удалён", tc.name)
		s.Equal(before, docState(doc.ID, eq), "%s: документ, строки и единица не изменились", tc.name)

		// документ удалённого заказа остаётся в «Перемещениях» с пометкой «заказ удалён».
		st, raw = s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", doc.ID), nil)
		s.Require().Equal(http.StatusOK, st, "%s: %s", tc.name, raw)
		var got struct {
			Status        string `json:"status"`
			TaskID        *int64 `json:"task_id"`
			TaskIsDeleted bool   `json:"task_is_deleted"`
		}
		s.Require().NoError(json.Unmarshal(raw, &got), string(raw))
		s.Equal(tc.status, got.Status, tc.name)
		s.Require().NotNil(got.TaskID, tc.name)
		s.Equal(taskID, *got.TaskID, tc.name)
		s.True(got.TaskIsDeleted, "%s: task_is_deleted", tc.name)

		// восстановленный заказ видит свой документ в карточке.
		st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", taskID), nil)
		s.Require().Equal(http.StatusOK, st, "%s: %s", tc.name, raw)
		_, card, _ := s.getCard(s.admin, taskID)
		s.Require().Len(card.Transfers, 1, tc.name)
		s.Equal(doc.ID, card.Transfers[0].ID, tc.name)
		s.Equal(tc.status, card.Transfers[0].Status, tc.name)
		s.Equal(before, docState(doc.ID, eq), "%s: восстановление заказа документ не тронуло", tc.name)
	}

	// ---- несколько документов и накладная с товаром: остатки и движения не меняются ----
	taskID, _ := s.newTask(c, col)
	units := []string{s.newEquipment(c), s.newEquipment(c), s.newEquipment(c)}
	for _, u := range units {
		s.addEq(taskID, u, "")
	}
	goods := s.newProduct("10", "50", "0")
	d1 := s.clientTransfer("client_to_warehouse", taskID, almaty, nil, units[0])
	d2 := s.transferAction(s.clientTransfer("client_to_warehouse", taskID, almaty, nil, units[1]), "cancel")
	d3 := s.clientTransfer("client_to_warehouse", taskID, almaty, map[string]any{"receive": true}, units[2])
	inv := s.invoice(taskID, true, [3]any{goods, 4, 10})
	s.Equal("46.000", s.stockAt(goods, almaty))
	docIDs := []int64{d1.ID, d2.ID, d3.ID, inv.ID}
	docs := func() string {
		return s.revertScalar(`
			SELECT string_agg(id::text || ':' || status || ':' || version::text || ':' || (deleted_at IS NOT NULL)::text, ',' ORDER BY id)
			FROM transfers WHERE id = ANY($1)`, docIDs)
	}
	eqs := func() string {
		return s.revertScalar(`
			SELECT string_agg(location_status || ':' || coalesce(warehouse_id::text, '') || ':' || coalesce(active_transfer_id::text, ''), ',' ORDER BY id)
			FROM equipment WHERE id = ANY($1::uuid[])`, units)
	}
	docsBefore, eqsBefore := docs(), eqs()
	movesBefore := s.revertScalar(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1`, goods)

	st, raw := s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), deleteReason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("t", s.revertScalar(`SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $1`, taskID)[:1], "строка заказа помечена удалённой")
	s.Equal(docsBefore, docs(), "документы не изменились")
	s.Equal(eqsBefore, eqs(), "местоположение и занятость единиц не изменились")
	s.Equal("46.000", s.stockAt(goods, almaty), "остаток по накладной «в пути» не изменился")
	s.Equal(movesBefore, s.revertScalar(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1`, goods), "движений не добавилось")
	s.Equal("4", s.revertScalar(`SELECT count(DISTINCT transfer_id)::text FROM transfer_items WHERE transfer_id = ANY($1)`, docIDs))

	// негатив: заказ без документов удаляется как раньше.
	free, _ := s.newTask(c, col)
	s.addEq(free, s.newEquipment(c), "")
	st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", free), deleteReason)
	s.Equal(http.StatusOK, st, string(raw))
	st, _, _ = s.getCard(s.admin, free)
	s.Equal(http.StatusNotFound, st)
}

// ---- TS-018 (хвост) ----

// TS-018 (transfers-revert): единица в заявке не убирается из заказа и не удаляется из
// справочника (409 EQUIPMENT_IN_TRANSFER); после УДАЛЕНИЯ документа-заявки (DELETE
// /api/v1/transfers/:id) она свободна и убирается из заказа. Строки мягко удалённого документа
// единицу в справочнике больше не держат (soft-delete-clients DS-014, DS-018: отказ
// EQUIPMENT_HAS_TRANSFERS снят) — но эта единица числится на нашем складе, поэтому удалить её
// нельзя по DS-041: 409 EQUIPMENT_AT_WAREHOUSE.
func (s *OrderCardLiveSuite) TestTS018_DeleteRequestFreesEquipment() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	c := s.newClient("Клиент-Удаление-Заявки")
	taskID, _ := s.newTask(c, s.columnID("Новые"))
	eq := s.newEquipment(c)
	s.attachRaw(taskID, eq)
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = $1`, eq, almaty) // фикстура местоположения
	s.Require().NoError(err)
	removePath := fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eq)

	doc := s.newTransfer(almaty, moscow, false, eq)
	st, raw := s.admin.Do(http.MethodDelete, removePath, nil)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("EQUIPMENT_IN_TRANSFER", s.errOf(raw).Code)
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+eq, map[string]any{"reason": "Списана"})
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("EQUIPMENT_IN_TRANSFER", s.errOf(raw).Code)

	s.mustDeleteTransfer(doc.ID, doc.Version)
	_, card, _ := s.getCard(s.admin, taskID)
	e := s.cardEquipment(card, eq)
	s.Nil(e.ActiveTransferID, "после удаления заявки единица свободна")
	s.Equal("at_warehouse", e.LocationStatus, "заявка местоположение не меняла")
	s.Empty(card.Transfers)

	st, m, raw := s.doEquipment(s.admin, http.MethodDelete, removePath, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(m.Card.Equipment, "единица убрана из заказа")
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+eq, map[string]any{"reason": "Списана"})
	s.Require().Equal(http.StatusConflict, st, "единица на нашем складе не удаляется: %s", raw)
	s.Equal("EQUIPMENT_AT_WAREHOUSE", s.errOf(raw).Code)
	s.Equal("1", s.revertScalar(`SELECT count(*)::text FROM equipment WHERE id = $1 AND deleted_at IS NULL`, eq))
}

// ---- D84: клиент заказа с документами перемещения не меняется ----

// D84 (доработка transfers, 2026-10-08): PUT /api/v1/tasks/:id со сменой client_id у заказа,
// у которого есть ХОТЯ БЫ ОДИН документ «Перемещение» в любом статусе (заявка / в пути /
// получено / отменён), → 409 TASK_CLIENT_LOCKED_BY_TRANSFERS с details.transfer_ids (номера по
// возрастанию); заказ, его оборудование и документ не изменились (transfers.client_id — снимок
// клиента заказа). Негативы: тот же client_id (правка без смены клиента) проходит; после
// (мягкого) удаления единственного (отменённого) документа смена клиента проходит — удалённые
// документы смене не мешают (soft-delete-transfers DS-040); заказ без документов меняет
// клиента как раньше.
func (s *OrderCardLiveSuite) TestTaskClientLockedByTransfers() {
	almaty := s.warehouseID("Алматы")
	c1 := s.newClient("Клиент-Замок-Клиента-Раз")
	c2 := s.newClient("Клиент-Замок-Клиента-Два")
	col := s.columnID("Новые")

	put := func(taskID int64, clientID, title string) (int, []byte) {
		version := s.revertScalar(`SELECT version::text FROM tasks WHERE id = $1`, taskID)
		return s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
			"title": title, "client_id": clientID, "column_id": col,
			"assignee_id": nil, "due_date": nil, "is_paid": false, "version": json.Number(version),
		})
	}
	// state — заказ (клиент, версия, название, состав оборудования), единица и документы заказа.
	state := func(taskID int64, eq string) string {
		return s.revertScalar(`
			SELECT t.client_id::text || '|' || t.version::text || '|' || t.title || '|' ||
			       (SELECT count(*) FROM task_equipment te WHERE te.task_id = t.id)::text || '|' ||
			       e.location_status || '|' || coalesce(e.warehouse_id::text, '') || '|' || coalesce(e.active_transfer_id::text, '') || '|' ||
			       coalesce((SELECT string_agg(d.id::text || ':' || d.status || ':' || d.client_id::text || ':' || d.version::text, ',' ORDER BY d.id)
			                 FROM transfers d WHERE d.task_id = t.id), '')
			FROM tasks t, equipment e WHERE t.id = $1 AND e.id = $2`, taskID, eq)
	}
	locked := func(taskID int64, eq, ctx string, ids ...int64) {
		before := state(taskID, eq)
		st, raw := put(taskID, c2, "Смена клиента")
		s.Require().Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
		var e transferErrDTO
		s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
		s.Equal("TASK_CLIENT_LOCKED_BY_TRANSFERS", e.Code, ctx)
		s.Equal("У заказа есть перемещения — сменить клиента нельзя", e.Message, ctx)
		s.Require().NotNil(e.Details, string(raw))
		s.Equal(ids, e.Details.TransferIDs, "%s: номера по возрастанию", ctx)
		s.Equal(before, state(taskID, eq), "%s: заказ, оборудование и документы не изменились", ctx)
		_, card, _ := s.getCard(s.admin, taskID)
		s.Equal(c1, card.Task.ClientID, ctx)
		s.Len(card.Equipment, 1, "%s: оборудование заказа не тронуто", ctx)
	}

	var cancelledTask int64
	var cancelledDoc transferDocDTO
	var cancelledUnit string
	for _, tc := range []struct {
		name   string
		flags  map[string]any
		cancel bool
		status string
	}{
		{"заявка", nil, false, "requested"},
		{"в пути", map[string]any{"send": true}, false, "in_transit"},
		{"получено", map[string]any{"receive": true}, false, "received"},
		{"отменён", nil, true, "cancelled"},
	} {
		taskID, _ := s.newTask(c1, col)
		eq := s.newEquipment(c1)
		s.addEq(taskID, eq, "")
		doc := s.clientTransfer("client_to_warehouse", taskID, almaty, tc.flags, eq)
		if tc.cancel {
			doc = s.transferAction(doc, "cancel")
		}
		s.Require().Equal(tc.status, doc.Status, tc.name)
		locked(taskID, eq, tc.name, doc.ID)

		// негатив: тот же клиент — правка заказа проходит как раньше, документ не тронут.
		st, raw := put(taskID, c1, "Правка без смены клиента")
		s.Require().Equal(http.StatusOK, st, "%s: %s", tc.name, raw)
		_, card, _ := s.getCard(s.admin, taskID)
		s.Equal("Правка без смены клиента", card.Task.Title, tc.name)
		s.Equal(c1, card.Task.ClientID, tc.name)
		s.Len(card.Equipment, 1, tc.name)
		s.Require().Len(card.Transfers, 1, tc.name)
		s.Equal(tc.status, card.Transfers[0].Status, tc.name)
		// …а смена клиента по-прежнему закрыта.
		locked(taskID, eq, tc.name+", после правки", doc.ID)

		if tc.cancel {
			cancelledTask, cancelledDoc, cancelledUnit = taskID, doc, eq
		}
	}

	// единственный (отменённый) документ удалён → смена клиента проходит, оборудование
	// прежнего клиента убирается из заказа (прежняя логика), в справочнике остаётся.
	s.mustDeleteTransfer(cancelledDoc.ID, cancelledDoc.Version)
	st, raw := put(cancelledTask, c2, "Смена клиента")
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, card, _ := s.getCard(s.admin, cancelledTask)
	s.Equal(c2, card.Task.ClientID)
	s.Empty(card.Equipment, "оборудование прежнего клиента убрано из заказа")
	s.Empty(card.Transfers)
	s.Equal("at_client|", s.revertScalar(`
		SELECT location_status || '|' || coalesce(active_transfer_id::text, '') FROM equipment WHERE id = $1`, cancelledUnit))

	// несколько документов: в ответе все, включая отменённый, по возрастанию; пока остаётся
	// хотя бы один — смена закрыта.
	taskID, _ := s.newTask(c1, col)
	u1, u2 := s.newEquipment(c1), s.newEquipment(c1)
	s.addEq(taskID, u1, "")
	s.addEq(taskID, u2, "")
	d1 := s.transferAction(s.clientTransfer("client_to_warehouse", taskID, almaty, nil, u1), "cancel")
	d2 := s.clientTransfer("client_to_warehouse", taskID, almaty, nil, u2)
	multi := func(ctx string, ids ...int64) {
		st, raw := put(taskID, c2, "Смена клиента")
		s.Require().Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
		var e transferErrDTO
		s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
		s.Equal("TASK_CLIENT_LOCKED_BY_TRANSFERS", e.Code, ctx)
		s.Require().NotNil(e.Details, string(raw))
		s.Equal(ids, e.Details.TransferIDs, ctx)
	}
	multi("отменённый и заявка", d1.ID, d2.ID)
	s.mustDeleteTransfer(d2.ID, d2.Version)
	multi("остался отменённый", d1.ID)
	s.mustDeleteTransfer(d1.ID, d1.Version)
	st, raw = put(taskID, c2, "Смена клиента")
	s.Require().Equal(http.StatusOK, st, string(raw))

	// негатив: заказ без документов меняет клиента как раньше.
	free, _ := s.newTask(c1, col)
	s.addEq(free, s.newEquipment(c1), "")
	st, raw = put(free, c2, "Смена клиента")
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, card, _ = s.getCard(s.admin, free)
	s.Equal(c2, card.Task.ClientID)
	s.Empty(card.Equipment)
}
