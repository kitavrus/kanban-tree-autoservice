//go:build integration

package board_test

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Мягкое удаление документов перемещения — сценарии со стороны заказа
// (docs/features/soft-delete-transfers/draft-spec.md §11), методы OrderCardLiveSuite (§TD-8,
// сьюта запущена с TEST_PURGE_ENABLED=true): TS-050 (смена клиента заказа при мягко удалённом
// документе, DS-040), TS-051 (служебная уборка вида transfers и уборка заказа с мягко
// удалённым документом, DS-038). TS-014 (удаление заказа с документами) —
// order_card_revert_http_live_test.go; удаление и восстановление самого документа (TS-015…
// TS-018, TS-033) — сьюта пакета transfers.

// restoreTransfer — POST /api/v1/transfers/:id/restore от администратора.
func (s *OrderCardLiveSuite) restoreTransfer(id int64) (int, []byte) {
	return s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/restore", id), nil)
}

// transferRow — «статус|удалён» строки документа; пусто — строки нет.
func (s *OrderCardLiveSuite) transferRow(id int64) string {
	return s.revertScalar(`
		SELECT coalesce((SELECT status || '|' || (deleted_at IS NOT NULL)::text FROM transfers WHERE id = $1), '')`, id)
}

func (s *OrderCardLiveSuite) unitBusy(eq string) string {
	return s.revertScalar(`
		SELECT location_status || '|' || coalesce(active_transfer_id::text, '') FROM equipment WHERE id = $1`, eq)
}

// ---- TS-050 ----

// TS-050 (DS-040): мягко удалённый документ не мешает сменить клиента заказа — PUT
// /api/v1/tasks/:id с другим client_id → 200; последующее восстановление документа → 409
// TRANSFER_CLIENT_MISMATCH, документ остаётся удалённым, единица — свободной. Без смены
// клиента документ восстанавливается и снова занимает единицу. Негатив: при действующем
// документе смена клиента по-прежнему → 409 TASK_CLIENT_LOCKED_BY_TRANSFERS, в
// details.transfer_ids — только действующие документы.
func (s *OrderCardLiveSuite) TestTS050_ClientChangeWithSoftDeletedTransfer() {
	almaty := s.warehouseID("Алматы")
	c1 := s.newClient("Клиент-Смена-При-Удалённом-Раз")
	c2 := s.newClient("Клиент-Смена-При-Удалённом-Два")
	col := s.columnID("Новые")
	put := func(taskID int64, clientID string) (int, []byte) {
		version := s.revertScalar(`SELECT version::text FROM tasks WHERE id = $1`, taskID)
		return s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
			"title": "Смена клиента", "client_id": clientID, "column_id": col,
			"assignee_id": nil, "due_date": nil, "is_paid": false, "version": json.Number(version),
		})
	}

	taskID, _ := s.newTask(c1, col)
	u1, u2 := s.newEquipment(c1), s.newEquipment(c1)
	s.addEq(taskID, u1, "")
	s.addEq(taskID, u2, "")
	gone := s.clientTransfer("client_to_warehouse", taskID, almaty, nil, u1)
	live := s.clientTransfer("client_to_warehouse", taskID, almaty, nil, u2)
	s.mustDeleteTransfer(gone.ID, gone.Version)
	s.Equal("at_client|", s.unitBusy(u1), "единица удалённой заявки свободна")

	// негатив: действующий документ по-прежнему держит клиента; удалённый в список не входит.
	st, raw := put(taskID, c2)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	var e transferErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	s.Equal("TASK_CLIENT_LOCKED_BY_TRANSFERS", e.Code)
	s.Require().NotNil(e.Details, string(raw))
	s.Equal([]int64{live.ID}, e.Details.TransferIDs, "только действующие документы")

	// без смены клиента удалённый документ восстанавливается и снова занимает единицу.
	st, raw = s.restoreTransfer(gone.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("requested|false", s.transferRow(gone.ID))
	s.Equal(fmt.Sprintf("at_client|%d", gone.ID), s.unitBusy(u1))

	// оба документа удалены → смена клиента проходит.
	s.mustDeleteTransfer(gone.ID, gone.Version+2)
	s.mustDeleteTransfer(live.ID, live.Version)
	st, raw = put(taskID, c2)
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, card, _ := s.getCard(s.admin, taskID)
	s.Equal(c2, card.Task.ClientID, "клиент сменён")
	s.Empty(card.Transfers, "удалённые документы в карточку не входят")

	// восстановление документа с прежним клиентом → 409, документ остаётся удалённым.
	for _, doc := range []transferDocDTO{gone, live} {
		st, raw = s.restoreTransfer(doc.ID)
		s.Require().Equal(http.StatusConflict, st, "#%d: %s", doc.ID, raw)
		s.Equal("TRANSFER_CLIENT_MISMATCH", s.sdErr(raw).Code)
		s.Equal("Клиент заказа изменён — восстановить перемещение нельзя", s.sdErr(raw).Message)
		s.Equal("requested|true", s.transferRow(doc.ID), "документ остаётся удалённым")
	}
	s.Equal("at_client|", s.unitBusy(u1), "отказ единицу не занял")
	s.Equal("at_client|", s.unitBusy(u2))
	s.Equal(c1, s.revertScalar(`SELECT client_id::text FROM transfers WHERE id = $1`, gone.ID), "снимок клиента документа не меняется")
}

// ---- TS-051 ----

// TS-051 (DS-038): служебная уборка DELETE /api/v1/test/purge/transfers/:id физически удаляет
// документ в «Заявка» и «Отменён» — действующий и мягко удалённый (строки — каскадом, единица
// заявки освобождается); документ «в пути» и «получено» → 409 TRANSFER_STATUS_CONFLICT, строка
// на месте; нет документа / нечисловой номер → 404 NOT_FOUND. Уборка заказа с мягко удалённым
// документом проходит — строки документа в базе нет. Негативы: исполнитель → 403; «без
// TEST_PURGE_ENABLED адреса нет (404)» — BoardLiveSuite.
func (s *OrderCardLiveSuite) TestTS051_PurgeTransfers() {
	almaty := s.warehouseID("Алматы")
	c := s.newClient("Клиент-Уборка-Документов")
	col := s.columnID("Новые")
	mk := func(flags map[string]any) (int64, string, transferDocDTO) {
		taskID, _ := s.newTask(c, col)
		eq := s.newEquipment(c)
		s.addEq(taskID, eq, "")
		return taskID, eq, s.clientTransfer("client_to_warehouse", taskID, almaty, flags, eq)
	}
	purgeDoc := func(id int64) (int, []byte) { return s.purge(s.admin, "transfers", fmt.Sprint(id)) }
	gone := func(doc transferDocDTO, ctx string) {
		st, raw := purgeDoc(doc.ID)
		s.Require().Equal(http.StatusOK, st, "%s: %s", ctx, raw)
		var out struct {
			ID string `json:"id"`
		}
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		s.Equal(fmt.Sprint(doc.ID), out.ID, ctx)
		s.Equal("", s.transferRow(doc.ID), "%s: строки документа нет", ctx)
		s.Equal("0", s.revertScalar(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = $1`, doc.ID), "%s: строки ушли каскадом", ctx)
	}

	// действующая заявка: единица освобождается.
	_, eq, requested := mk(nil)
	exec, _ := s.sdExecutor()
	for _, target := range []string{fmt.Sprint(requested.ID), "999999999"} {
		st, raw := s.purge(exec, "transfers", target)
		s.Equal(http.StatusForbidden, st, string(raw))
		s.Equal("Нет доступа", s.sdErr(raw).Message)
	}
	s.Equal(fmt.Sprintf("at_client|%d", requested.ID), s.unitBusy(eq))
	gone(requested, "действующая заявка")
	s.Equal("at_client|", s.unitBusy(eq), "единица заявки свободна")
	st, raw := purgeDoc(requested.ID)
	s.Equal(http.StatusNotFound, st, "повтор: %s", raw)
	s.Equal("NOT_FOUND", s.sdErr(raw).Code)

	// действующий отменённый.
	_, _, doc := mk(nil)
	gone(s.transferAction(doc, "cancel"), "действующий отменённый")

	// мягко удалённые: заявка и отменённый.
	_, _, doc = mk(nil)
	s.mustDeleteTransfer(doc.ID, doc.Version)
	gone(doc, "мягко удалённая заявка")
	_, _, doc = mk(nil)
	doc = s.transferAction(doc, "cancel")
	s.mustDeleteTransfer(doc.ID, doc.Version)
	gone(doc, "мягко удалённый отменённый")

	// проведённые документы уборка не трогает.
	for _, tc := range []struct {
		flags       map[string]any
		status, msg string
	}{
		{map[string]any{"send": true}, "in_transit", "Перемещение в пути — сначала отмените его"},
		{map[string]any{"receive": true}, "received", "Перемещение получено — сначала отмените проведение, затем отмените"},
	} {
		_, _, doc = mk(tc.flags)
		st, raw = purgeDoc(doc.ID)
		s.Require().Equal(http.StatusConflict, st, "%s: %s", tc.status, raw)
		s.Equal("TRANSFER_STATUS_CONFLICT", s.sdErr(raw).Code, tc.status)
		s.Equal(tc.msg, s.sdErr(raw).Message, tc.status)
		s.Equal(tc.status+"|false", s.transferRow(doc.ID), "%s: документ на месте", tc.status)
	}

	for _, id := range []string{"abc", "0", "-1", "999999999"} {
		st, raw = s.purge(s.admin, "transfers", id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
		s.Equal("Не найдено", s.sdErr(raw).Message)
	}

	// уборка заказа: мягко удалённая заявка уходит вместе с заказом (действующий заказ).
	taskID, eq, doc := mk(nil)
	s.mustDeleteTransfer(doc.ID, doc.Version)
	st, raw = s.purge(s.admin, "tasks", fmt.Sprint(taskID))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0", s.revertScalar(`SELECT count(*)::text FROM tasks WHERE id = $1`, taskID))
	s.Equal("", s.transferRow(doc.ID), "мягко удалённый документ убран вместе с заказом")
	s.Equal("1", s.revertScalar(`SELECT count(*)::text FROM equipment WHERE id = $1`, eq), "единица остаётся в справочнике")

	// …и у мягко удалённого заказа; действующая заявка по-прежнему держит заказ от уборки.
	taskID, eq, doc = mk(nil)
	eq2 := s.newEquipment(c)
	s.addEq(taskID, eq2, "")
	keep := s.clientTransfer("client_to_warehouse", taskID, almaty, nil, eq2)
	s.mustDeleteTransfer(doc.ID, doc.Version)
	s.mustSoftDelete(taskID)
	st, raw = s.purge(s.admin, "tasks", fmt.Sprint(taskID))
	s.Require().Equal(http.StatusConflict, st, string(raw))
	var e transferErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	s.Equal("TASK_HAS_TRANSFERS", e.Code)
	s.Require().NotNil(e.Details, string(raw))
	s.Equal([]int64{keep.ID}, e.Details.TransferIDs, "мешает только действующий неотменённый документ")
	s.Equal("requested|true", s.transferRow(doc.ID), "отказ ничего не удалил")
	gone(keep, "заявка удалённого заказа")
	st, raw = s.purge(s.admin, "tasks", fmt.Sprint(taskID))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("", s.transferRow(doc.ID))
	s.Equal("at_client|", s.unitBusy(eq))
}
