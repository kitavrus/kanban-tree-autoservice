//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Отмена проведения и удаление документа «Перемещение»
// (docs/features/transfers-revert/draft-spec.md §11) — методы TransfersLiveSuite (§TD-8):
// TS-010 (откат каждого из четырёх направлений, причина, повторная приёмка, параллельный
// откат), TS-011 (израсходованный товар — минус с предупреждением), TS-012 (обратный порядок:
// более поздний документ, «увезли и вернули»), TS-016 (удаление документа по статусам),
// хвост TS-015 (исполнитель → 403). Откат накладной оплаченного заказа (TS-013) и удаление
// заказа (TS-017) — в сьюте доски (internal/features/board).

// ---- хелперы ----

// unreceive — «Отменить проведение» документа t (актуальная версия) с причиной reason.
func (s *TransfersLiveSuite) unreceive(t transferDTO, reason string) (int, transferDTO, []byte) {
	return s.action(t, "unreceive", map[string]any{"reason": reason})
}

func (s *TransfersLiveSuite) mustUnreceive(t transferDTO, reason string) transferDTO {
	st, out, raw := s.unreceive(t, reason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Equal("in_transit", out.Status)
	return out
}

// must — действие act над документом t обязано пройти (200).
func (s *TransfersLiveSuite) must(t transferDTO, act string, body map[string]any) transferDTO {
	st, out, raw := s.action(t, act, body)
	s.Require().Equal(http.StatusOK, st, "%s #%d → %s", act, t.ID, raw)
	return out
}

// deleteBody — тело мягкого удаления документа (soft-delete-transfers §6.1, DS-028): причина и
// version — в JSON-теле (раньше version шёл в query).
func deleteBody(version any) map[string]any {
	return map[string]any{"reason": "Создан по ошибке", "version": version}
}

func deletePath(id int64) string { return fmt.Sprintf("/api/v1/transfers/%d", id) }

// productLine — id товарной строки документа по товару.
func (s *TransfersLiveSuite) productLine(t transferDTO, productID string) string {
	for _, it := range t.Items {
		if it.Kind == "product" && it.ProductID == productID {
			return it.ID
		}
	}
	s.FailNow("товарная строка не найдена", productID)
	return ""
}

func fact(lineID string, qty any) map[string]any {
	return map[string]any{"id": lineID, "qty_received": qty}
}

func (s *TransfersLiveSuite) expectConflict(st int, raw []byte, code, msg, ctx string) apiErr {
	s.Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
	e := s.errOf(raw)
	s.Equal(code, e.Code, ctx)
	s.Equal(msg, e.Message, ctx)
	return e
}

// ---- TS-010 ----

// TS-010 (склад → склад): приёмка с недостачей дала на получателе чистый прирост на факт
// (transfer_in +5, transfer_loss −2) — откат снимает ровно факт (transfer_unreceive −3);
// единица снова «в пути» и занята документом; факт и отметка приёмки обнулены, на документе —
// кто / когда / причина (после trim). Повторная приёмка с другим фактом и отмена после
// отката проходят; сведения об откате остаются.
func (s *TransfersLiveSuite) TestTS010_Unreceive_WarehouseToWarehouse() {
	p := s.newProduct("50")
	u := s.newUnit(s.almaty)
	body := s.body(productItem(p, 5), unitItem(u))
	body["send"] = true
	doc := s.create(body)
	rcv := s.must(doc, "receive", map[string]any{"items": []map[string]any{fact(s.productLine(doc, p), 3)}})
	s.Equal("45.000", s.pairStock(p, s.almaty))
	s.Equal("3.000", s.pairStock(p, s.moscow))
	s.Equal("at_warehouse|Москва|", s.unitLoc(u))
	s.Require().NotNil(rcv.ReceivedAt)
	s.Nil(rcv.UnreceivedAt, "отката ещё не было")
	s.Empty(rcv.UnreceiveReason)

	back := s.mustUnreceive(rcv, "  Ошибка в факте \n")
	s.Equal(rcv.Version+1, back.Version)
	s.Nil(back.ReceivedAt)
	s.Nil(back.ReceivedBy)
	s.Require().NotNil(back.UnreceivedBy)
	s.Equal(s.adminID, *back.UnreceivedBy)
	s.Require().NotNil(back.UnreceivedByName)
	s.Equal("Перемещения Админ", *back.UnreceivedByName)
	s.Require().NotNil(back.UnreceivedAt)
	s.Equal("Ошибка в факте", back.UnreceiveReason, "причина — после trim")
	s.NotNil(back.SentAt, "отметка отправки сохраняется")
	for _, it := range back.Items {
		s.Nil(it.QtyReceived, "факт обнулён: %s", it.Kind)
	}
	s.Equal("45.000", s.pairStock(p, s.almaty), "источник не тронут")
	s.Equal("0.000", s.pairStock(p, s.moscow), "получатель: −факт приёмки")
	s.Equal(fmt.Sprintf("in_transit||%d", doc.ID), s.unitLoc(u), "единица снова в пути и занята документом")
	s.ElementsMatch([]string{
		"transfer_out:Алматы:-5.000", "transfer_in:Москва:5.000", "transfer_loss:Москва:-2.000",
		"transfer_unreceive:Москва:-3.000",
	}, s.movements(doc.ID))
	s.Equal("1", s.scalarText(`SELECT count(*)::text FROM transfers WHERE id = $1 AND received_by IS NULL AND received_at IS NULL
		AND unreceived_by = $2 AND unreceived_at IS NOT NULL`, doc.ID, s.adminID))
	// GET отдаёт то же состояние.
	got := s.get(doc.ID)
	s.Equal("in_transit", got.Status)
	s.Equal("Ошибка в факте", got.UnreceiveReason)

	// повторная приёмка с другим фактом (полным): сведения об откате остаются.
	again := s.must(back, "receive", map[string]any{"items": []map[string]any{fact(s.productLine(back, p), 5)}})
	s.Equal("received", again.Status)
	s.Equal("5.000", s.pairStock(p, s.moscow))
	s.Equal("at_warehouse|Москва|", s.unitLoc(u))
	s.NotNil(again.ReceivedAt)
	s.Equal("Ошибка в факте", again.UnreceiveReason, "сведения об откате переживают повторную приёмку")
	s.Equal(back.UnreceivedAt, again.UnreceivedAt)
	s.Equal("5.000", s.productMovementsSum(p, s.moscow), "остаток получателя = Σ движений")

	// второй откат и отмена: всё возвращается в источник, причина — последняя.
	back2 := s.mustUnreceive(again, "Вторая причина")
	s.Equal("Вторая причина", back2.UnreceiveReason, "хранится последний откат")
	s.Equal("0.000", s.pairStock(p, s.moscow))
	cnc := s.must(back2, "cancel", nil)
	s.Equal("cancelled", cnc.Status)
	s.Equal("50.000", s.pairStock(p, s.almaty), "товар вернулся на склад отправления")
	s.Equal("0.000", s.pairStock(p, s.moscow))
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u), "единица вернулась в источник и свободна")
	s.Equal("Вторая причина", cnc.UnreceiveReason)
	s.Equal("0.000", s.productMovementsSum(p, s.moscow))
	s.Equal("0.000", s.productMovementsSum(p, s.almaty), "−5 отправка, +5 возврат")
}

// TS-010 (приход от поставщика): откат снимает факт приёмки (в т.ч. факт больше заказанного),
// строка с фактом 0 движения не даёт; повторная приёмка — с новым фактом.
func (s *TransfersLiveSuite) TestTS010_Unreceive_SupplierToWarehouse() {
	sup := s.newSupplier(true)
	p, zero := s.newProduct("0"), s.newProduct("0")
	doc := s.create(with(s.supplyBody(sup, p, 10), "send", true,
		"items", []map[string]any{productItem(p, 10), productItem(zero, 4)}))
	rcv := s.must(doc, "receive", map[string]any{"items": []map[string]any{
		fact(s.productLine(doc, p), 12), fact(s.productLine(doc, zero), 0),
	}})
	s.Equal("12.000", s.pairStock(p, s.almaty))
	s.Equal("0.000", s.pairStock(zero, s.almaty))
	s.ElementsMatch([]string{"supplier_in:Алматы:12.000"}, s.movements(doc.ID))

	back := s.mustUnreceive(rcv, "Пересчитали поставку")
	s.Equal("supplier_to_warehouse", back.Direction)
	s.Equal("0.000", s.pairStock(p, s.almaty))
	s.Equal("0.000", s.pairStock(zero, s.almaty))
	s.ElementsMatch([]string{"supplier_in:Алматы:12.000", "transfer_unreceive:Алматы:-12.000"}, s.movements(doc.ID),
		"строка с фактом 0 компенсирующего движения не даёт")
	for _, it := range back.Items {
		s.Nil(it.QtyReceived)
	}
	s.Nil(back.ReceivedAt)
	s.Equal("Пересчитали поставку", back.UnreceiveReason)
	// предупреждение — только по товару с движением: строка с фактом 0 остаток не двигала.
	s.Require().Len(back.Warnings, 1)
	s.Equal(p, back.Warnings[0].ProductID)
	s.Equal("below_min", back.Warnings[0].Kind)

	again := s.must(back, "receive", map[string]any{"items": []map[string]any{
		fact(s.productLine(back, p), 7), fact(s.productLine(back, zero), 2),
	}})
	s.Equal("received", again.Status)
	s.Equal("7.000", s.pairStock(p, s.almaty))
	s.Equal("2.000", s.pairStock(zero, s.almaty))
	s.Equal("7.000", s.productMovementsSum(p, s.almaty), "+12 −12 +7")
	s.Equal("Пересчитали поставку", again.UnreceiveReason)
}

// TS-010 (документы с клиентом): откат движений остатка не создаёт. Приём от клиента —
// единица со склада назначения снова «в пути» (отмена вернёт её клиенту); накладная «склад →
// клиент» — единица от клиента снова «в пути», товар остаётся списанным (списан при отправке),
// пока документ «в пути»; отмена после отката возвращает товар и единицу на склад.
func (s *TransfersLiveSuite) TestTS010_Unreceive_ClientDocuments() {
	// клиент → склад
	taskID, units := s.taskWithUnits(1)
	u := units[0]
	intake := s.create(with(intakeBody(taskID, s.almaty, u), "receive", true))
	s.Require().Equal("received", intake.Status)
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u))
	back := s.mustUnreceive(intake, "Приняли не ту единицу")
	s.Equal("client_to_warehouse", back.Direction)
	s.Equal(fmt.Sprintf("in_transit||%d", intake.ID), s.unitLoc(u))
	s.Empty(s.movements(intake.ID), "приём от клиента остаток не двигает — и откат тоже")
	s.Nil(back.ReceivedAt)
	s.Equal("Приняли не ту единицу", back.UnreceiveReason)
	cnc := s.must(back, "cancel", nil)
	s.Equal("cancelled", cnc.Status)
	s.Equal("at_client||", s.unitLoc(u), "отмена после отката возвращает единицу клиенту")

	// склад → клиент (накладная с товаром и единицей)
	task2, unit2 := s.taskWithStockUnit()
	p := s.newPricedProduct("10", "100")
	inv := s.create(with(s.invoiceBody(task2, unitItem(unit2), pricedItem(p, 4, 100)), "send", true))
	s.Equal("6.000", s.pairStock(p, s.almaty))
	rcv := s.must(inv, "receive", nil)
	s.Equal("at_client||", s.unitLoc(unit2))
	back = s.mustUnreceive(rcv, "Клиент не получил")
	s.Equal("warehouse_to_client", back.Direction)
	s.Equal(fmt.Sprintf("in_transit||%d", inv.ID), s.unitLoc(unit2), "единица снова в пути к клиенту")
	s.Equal("6.000", s.pairStock(p, s.almaty), "товар остаётся списанным, пока накладная «в пути»")
	s.ElementsMatch([]string{"client_out:Алматы:-4.000"}, s.movements(inv.ID), "откат накладной движений не создаёт")
	s.Equal("400.00", back.Amount.String(), "сумма накладной не меняется")
	// повторная приёмка — и снова откат с отменой: товар и единица возвращаются на склад.
	rcv = s.must(back, "receive", nil)
	s.Equal("at_client||", s.unitLoc(unit2))
	back = s.mustUnreceive(rcv, "Всё-таки возврат")
	cnc = s.must(back, "cancel", nil)
	s.Equal("cancelled", cnc.Status)
	s.Equal("10.000", s.pairStock(p, s.almaty))
	s.Equal("at_warehouse|Алматы|", s.unitLoc(unit2))
	s.ElementsMatch([]string{"client_out:Алматы:-4.000", "transfer_return:Алматы:4.000"}, s.movements(inv.ID))
}

// TS-010 (причина и статусы): причина обязательна — пусто и пробелы → 400 required, 501
// символ → 400 too_long, ровно 500 — проходит; version обязателен; устаревшая версия → 409;
// откат из requested / in_transit / cancelled → 409 со своим сообщением; при каждом отказе
// документ и остаток не изменились.
func (s *TransfersLiveSuite) TestTS010_Unreceive_ReasonAndStatusGuards() {
	sup := s.newSupplier(true)
	p := s.newProduct("0")
	mk := func() transferDTO { return s.create(with(s.supplyBody(sup, p, 2), "send", true)) }
	rcv := s.must(mk(), "receive", nil)
	s.Equal("2.000", s.pairStock(p, s.almaty))
	path := fmt.Sprintf("/api/v1/transfers/%d/unreceive", rcv.ID)

	for _, tc := range []struct {
		name string
		body map[string]any
		want map[string]string
	}{
		{"причина пустая", map[string]any{"version": rcv.Version, "reason": ""}, map[string]string{"reason": "required"}},
		{"причина из пробелов", map[string]any{"version": rcv.Version, "reason": " \t\n "}, map[string]string{"reason": "required"}},
		{"причины нет", map[string]any{"version": rcv.Version}, map[string]string{"reason": "required"}},
		{"501 символ", map[string]any{"version": rcv.Version, "reason": strings.Repeat("я", 501)}, map[string]string{"reason": "too_long"}},
		{"нет version", map[string]any{"reason": "есть"}, map[string]string{"version": "required"}},
		{"нет ничего", map[string]any{}, map[string]string{"version": "required", "reason": "required"}},
	} {
		st, _, raw := s.do(http.MethodPost, path, tc.body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", tc.name, raw)
		s.Equal(tc.want, s.fields(raw), tc.name)
	}
	st, _, raw := s.do(http.MethodPost, path, map[string]any{"version": rcv.Version - 1, "reason": "есть"})
	s.expectConflict(st, raw, "TRANSFER_VERSION_CONFLICT", "Перемещение уже изменено, обновите", "устаревшая версия")
	st, _, raw = s.do(http.MethodPost, "/api/v1/transfers/999999999/unreceive", map[string]any{"version": 1, "reason": "есть"})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TRANSFER_NOT_FOUND", s.errOf(raw).Code)

	// отказы ничего не изменили.
	same := s.get(rcv.ID)
	s.Equal("received", same.Status)
	s.Equal(rcv.Version, same.Version)
	s.Nil(same.UnreceivedAt)
	s.Equal("2.000", s.pairStock(p, s.almaty))
	s.Len(s.movements(rcv.ID), 1)

	// граница: ровно 500 символов — проходит.
	long := strings.Repeat("я", 500)
	back := s.mustUnreceive(rcv, long)
	s.Equal(long, back.UnreceiveReason)

	// недопустимые статусы — свой текст на каждый.
	requested := s.create(s.supplyBody(sup, p, 1))
	inTransit := back // только что возвращён откатом
	cancelled := s.must(s.create(s.supplyBody(sup, p, 1)), "cancel", nil)
	for _, tc := range []struct {
		doc transferDTO
		msg string
	}{
		{requested, "Перемещение ещё не проведено"},
		{inTransit, "Перемещение ещё не получено"},
		{cancelled, "Перемещение отменено"},
	} {
		st, _, raw := s.unreceive(tc.doc, "причина")
		s.expectConflict(st, raw, "TRANSFER_STATUS_CONFLICT", tc.msg, "unreceive on "+tc.doc.Status)
		s.Equal(tc.doc.Version, s.get(tc.doc.ID).Version, "документ не изменён")
	}
	s.Equal("0.000", s.pairStock(p, s.almaty))
}

// TS-010 (негатив, §3 п.1): параллельные откаты одного документа → ровно один 200, остальные
// 409 (версия либо статус); компенсирующее движение одно, остаток уменьшился один раз,
// единица занята документом.
func (s *TransfersLiveSuite) TestTS010_ParallelUnreceiveOnce() {
	p := s.newProduct("50")
	u := s.newUnit(s.almaty)
	body := s.body(productItem(p, 4), unitItem(u))
	body["send"] = true
	rcv := s.must(s.create(body), "receive", nil)
	s.Equal("4.000", s.pairStock(p, s.moscow))

	var wg sync.WaitGroup
	codes := make([]int, 6)
	bodies := make([][]byte, 6)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], bodies[i] = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/unreceive", rcv.ID),
				map[string]any{"version": rcv.Version, "reason": fmt.Sprintf("гонка %d", i)})
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
	s.Equal(1, ok, "ровно один откат")
	s.Equal("0.000", s.pairStock(p, s.moscow), "остаток уменьшился один раз")
	s.Equal("1", s.scalarText(`SELECT count(*)::text FROM stock_movements WHERE transfer_id = $1 AND reason = 'transfer_unreceive'`, rcv.ID))
	s.Equal(fmt.Sprintf("in_transit||%d", rcv.ID), s.unitLoc(u))
	s.Equal(rcv.Version+1, s.get(rcv.ID).Version, "версия выросла один раз")
}

// ---- TS-011 ----

// TS-011: принятый товар успели израсходовать — откат не блокируется: остаток склада
// назначения уходит в минус, ответ 200 несёт warnings[].kind = negative по этому складу.
func (s *TransfersLiveSuite) TestTS011_Unreceive_ConsumedStockGoesNegative() {
	sup := s.newSupplier(true)
	p := s.newProduct("0")
	rcv := s.must(s.create(with(s.supplyBody(sup, p, 10), "send", true)), "receive", nil)
	s.Equal("10.000", s.pairStock(p, s.almaty))
	// фикстура: 9 из 10 израсходовано (списание услугами — не предмет теста).
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE product_stock SET stock = stock - 9 WHERE product_id = $1 AND warehouse_id = $2`, p, s.almaty)
	s.Require().NoError(err)

	st, back, raw := s.unreceive(rcv, "Поставка оказалась чужой")
	s.Require().Equal(http.StatusOK, st, "откат не блокируется: %s", raw)
	s.Equal("in_transit", back.Status)
	s.Equal("-9.000", s.pairStock(p, s.almaty), "1 − 10")
	s.ElementsMatch([]string{"supplier_in:Алматы:10.000", "transfer_unreceive:Алматы:-10.000"}, s.movements(rcv.ID))
	s.Require().Len(back.Warnings, 1, string(raw))
	w := back.Warnings[0]
	s.Equal(p, w.ProductID)
	s.Equal(s.almaty, w.WarehouseID, "предупреждение — по складу назначения")
	s.Equal("negative", w.Kind)
	stock, err := w.Stock.Float64()
	s.Require().NoError(err)
	s.InDelta(-9.0, stock, 1e-9)
}

// ---- TS-012 ----

// blocked — откат документа doc обязан быть запрещён единицей unit: 409
// TRANSFER_UNRECEIVE_BLOCKED с номером мешающего документа; документ не изменён.
func (s *TransfersLiveSuite) blocked(doc transferDTO, unit string, by int64, msg string) {
	cur := s.get(doc.ID)
	st, _, raw := s.unreceive(cur, "попытка")
	e := s.expectConflict(st, raw, "TRANSFER_UNRECEIVE_BLOCKED", msg, fmt.Sprintf("unreceive #%d", doc.ID))
	s.Require().NotNil(e.Details, string(raw))
	s.Equal(by, e.Details.TransferID)
	s.Equal(unit, e.Details.EquipmentID)
	after := s.get(doc.ID)
	s.Equal("received", after.Status)
	s.Equal(cur.Version, after.Version)
	s.NotNil(after.ReceivedAt)
	s.Nil(after.UnreceivedAt)
}

// TS-012: откат — в обратном порядке. Единицу документа A увёз более поздний документ → 409 с
// номером САМОГО позднего; в т.ч. когда более поздние документы увезли единицу и вернули на
// тот же склад (текущее местоположение совпадает с точкой назначения A — проверяется порядок
// документов, abuse case 3). При отказе остатки, строки и местоположение не изменились. После
// отката более поздних (по одному, с конца) — успех. Негатив: единица в другом незавершённом
// документе → 409 с его номером.
func (s *TransfersLiveSuite) TestTS012_UnreceiveReverseOrder() {
	p := s.newProduct("10")
	u := s.newUnit(s.almaty)
	move := func(from, to string, items ...map[string]any) transferDTO {
		return s.must(s.create(map[string]any{"from_warehouse_id": from, "to_warehouse_id": to, "items": items, "send": true}), "receive", nil)
	}
	a := move(s.almaty, s.moscow, unitItem(u), productItem(p, 2)) // единица и товар → Москва
	b := move(s.moscow, s.almaty, unitItem(u))                    // единицу увезли в Алматы
	c := move(s.almaty, s.moscow, unitItem(u))                    // и вернули в Москву
	s.Equal("at_warehouse|Москва|", s.unitLoc(u), "единица снова там, куда её привёз A")
	s.Equal("2.000", s.pairStock(p, s.moscow))

	later := func(id int64) string {
		return fmt.Sprintf("Сначала отмените проведение перемещения #%d", id)
	}
	open := func(id int64) string {
		return fmt.Sprintf("Сначала отмените перемещение #%d", id)
	}

	// «увезли и вернули»: местоположение совпадает, но откат A запрещён — мешает C.
	s.blocked(a, u, c.ID, later(c.ID))
	s.Equal("2.000", s.pairStock(p, s.moscow), "отказ: остаток не изменился")
	s.Equal("8.000", s.pairStock(p, s.almaty))
	s.Len(s.movements(a.ID), 2, "отказ: компенсирующего движения нет")
	s.Equal("1", s.scalarText(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = $1 AND qty_received = 2`, a.ID),
		"отказ: факт по строке на месте")
	s.Equal("at_warehouse|Москва|", s.unitLoc(u), "отказ: местоположение не изменилось")
	s.blocked(b, u, c.ID, later(c.ID)) // B тоже ждёт C

	// обратный порядок: C → B → A.
	cBack := s.mustUnreceive(s.get(c.ID), "с конца")
	s.Equal(fmt.Sprintf("in_transit||%d", c.ID), s.unitLoc(u))
	s.blocked(a, u, c.ID, open(c.ID)) // C теперь незавершён и держит единицу
	s.must(cBack, "cancel", nil)
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u), "отмена C вернула единицу туда, куда её привёз B")

	s.blocked(a, u, b.ID, later(b.ID))
	bBack := s.mustUnreceive(s.get(b.ID), "дальше")
	s.must(bBack, "cancel", nil)
	s.Equal("at_warehouse|Москва|", s.unitLoc(u))

	aBack := s.mustUnreceive(s.get(a.ID), "теперь можно")
	s.Equal(fmt.Sprintf("in_transit||%d", a.ID), s.unitLoc(u))
	s.Equal("0.000", s.pairStock(p, s.moscow))
	s.Equal("теперь можно", aBack.UnreceiveReason)

	// негатив: единица стоит в другой незавершённой ЗАЯВКЕ → 409 с её номером.
	u2 := s.newUnit(s.almaty)
	d := move(s.almaty, s.moscow, unitItem(u2))
	e := s.create(map[string]any{"from_warehouse_id": s.moscow, "to_warehouse_id": s.almaty, "items": []map[string]any{unitItem(u2)}})
	s.Require().Equal("requested", e.Status)
	s.blocked(d, u2, e.ID, open(e.ID))
	s.Equal(fmt.Sprintf("at_warehouse|Москва|%d", e.ID), s.unitLoc(u2), "отказ: единица по-прежнему занята заявкой")
	// заявку отменили — откат проходит.
	s.must(e, "cancel", nil)
	s.mustUnreceive(s.get(d.ID), "заявка отменена")
	s.Equal(fmt.Sprintf("in_transit||%d", d.ID), s.unitLoc(u2))
}

// TS-012 (негатив): единица числится не там, куда её привёз документ, а мешающего документа
// нет (местоположение правили в обход перемещений) → 409 TRANSFER_UNRECEIVE_BLOCKED без
// details.transfer_id; документ, остатки, движения и единица не изменились. Единицу вернули
// на место — откат проходит.
func (s *TransfersLiveSuite) TestTS012_UnreceiveBlocked_EquipmentNotAtDestination() {
	ctx := context.Background()
	p := s.newProduct("10")
	u := s.newUnit(s.almaty)
	body := s.body(unitItem(u), productItem(p, 2))
	body["send"] = true
	a := s.must(s.create(body), "receive", nil)
	s.Equal("at_warehouse|Москва|", s.unitLoc(u))

	const msg = "Оборудование находится не там, куда его привезло это перемещение"
	for _, tc := range []struct{ name, sql, loc string }{
		{"на другом складе", `UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = $1`, "at_warehouse|Алматы|"},
		{"у клиента", `UPDATE equipment SET location_status = 'at_client', warehouse_id = NULL WHERE id = $1 AND $2::uuid IS NOT NULL`, "at_client||"},
	} {
		// фикстура: местоположение единицы изменено в обход документов.
		_, err := s.Pool.Exec(ctx, tc.sql, u, s.almaty)
		s.Require().NoError(err)
		st, _, raw := s.unreceive(a, "попытка")
		e := s.expectConflict(st, raw, "TRANSFER_UNRECEIVE_BLOCKED", msg, tc.name)
		s.Require().NotNil(e.Details, string(raw))
		s.Equal(u, e.Details.EquipmentID, tc.name)
		s.Zero(e.Details.TransferID, tc.name)
		s.NotContains(string(raw), "transfer_id", "%s: мешающего документа нет — ключа в details нет", tc.name)

		after := s.get(a.ID)
		s.Equal("received", after.Status, tc.name)
		s.Equal(a.Version, after.Version, tc.name)
		s.NotNil(after.ReceivedAt)
		s.Nil(after.UnreceivedAt)
		s.Equal("2.000", s.pairStock(p, s.moscow), "%s: остаток не изменился", tc.name)
		s.Equal("8.000", s.pairStock(p, s.almaty), tc.name)
		s.ElementsMatch([]string{"transfer_out:Алматы:-2.000", "transfer_in:Москва:2.000"}, s.movements(a.ID), tc.name)
		s.Equal("1", s.scalarText(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = $1 AND qty_received = 2`, a.ID), tc.name)
		s.Equal(tc.loc, s.unitLoc(u), "%s: единица не тронута", tc.name)
	}

	_, err := s.Pool.Exec(ctx, `UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = $1`, u, s.moscow)
	s.Require().NoError(err)
	s.mustUnreceive(a, "единица на месте")
	s.Equal(fmt.Sprintf("in_transit||%d", a.ID), s.unitLoc(u))
	s.Equal("0.000", s.pairStock(p, s.moscow))
}

// TS-012 (гонка, §3 п.2): откат документа A одновременно с новым документом B (создание +
// отправка) по той же единице. Оба берут блокировку строки equipment — выигрывает первый.
// Допустимы ровно два исхода: откат прошёл, а B получил 409 EQUIPMENT_IN_TRANSFER с номером A
// (и не создан); либо B создан и «в пути», а откат получил 409 TRANSFER_UNRECEIVE_BLOCKED с
// номером B. Инварианты после каждой итерации: единица «в пути» ровно в одном незавершённом
// документе и занята именно им; остатки обоих складов сходятся с движениями. Создание B
// длиннее отката, поэтому откат стартует со сдвигом 0–8 мс (по итерациям) — иначе гонку всегда
// выигрывал бы он; оба исхода обязаны встретиться.
func (s *TransfersLiveSuite) TestTS012_ParallelUnreceiveVsNewDocument() {
	const iterations = 30
	unreceiveWon, createWon := 0, 0
	for i := 0; i < iterations; i++ {
		p := s.newProduct("10")
		u := s.newUnit(s.almaty)
		body := s.body(unitItem(u), productItem(p, 1))
		body["send"] = true
		a := s.must(s.create(body), "receive", nil)
		before := s.transfersCount()

		var wg sync.WaitGroup
		var stU, stB int
		var rawU, rawB []byte
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(i%5) * 2 * time.Millisecond)
			stU, rawU = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/unreceive", a.ID),
				map[string]any{"version": a.Version, "reason": "гонка с новым документом"})
		}()
		go func() {
			defer wg.Done()
			<-start
			stB, rawB = s.admin.Do(http.MethodPost, "/api/v1/transfers", map[string]any{
				"direction": "warehouse_to_warehouse", "from_warehouse_id": s.moscow, "to_warehouse_id": s.almaty,
				"items": []map[string]any{unitItem(u), productItem(p, 1)}, "send": true,
			})
		}()
		close(start)
		wg.Wait()

		holder := a.ID // документ, в котором единица обязана стоять после гонки
		switch {
		case stU == http.StatusOK && stB == http.StatusConflict:
			unreceiveWon++
			e := s.errOf(rawB)
			s.Equal("EQUIPMENT_IN_TRANSFER", e.Code, string(rawB))
			s.Require().NotNil(e.Details, string(rawB))
			s.Equal(a.ID, e.Details.TransferID)
			s.Equal(before, s.transfersCount(), "B не создан")
			s.Equal("in_transit", s.get(a.ID).Status)
		case stB == http.StatusCreated && stU == http.StatusConflict:
			createWon++
			var b transferDTO
			s.Require().NoError(json.Unmarshal(rawB, &b), string(rawB))
			s.Equal("in_transit", b.Status)
			holder = b.ID
			e := s.errOf(rawU)
			s.Equal("TRANSFER_UNRECEIVE_BLOCKED", e.Code, string(rawU))
			s.Equal(fmt.Sprintf("Сначала отмените перемещение #%d", b.ID), e.Message)
			s.Require().NotNil(e.Details, string(rawU))
			s.Equal(b.ID, e.Details.TransferID)
			s.Equal(u, e.Details.EquipmentID)
			after := s.get(a.ID)
			s.Equal("received", after.Status, "A не изменён")
			s.Equal(a.Version, after.Version)
		default:
			s.FailNow("недопустимый исход гонки", "итерация %d: unreceive %d %s; create %d %s", i, stU, rawU, stB, rawB)
		}

		// единица — «в пути», занята ровно одним незавершённым документом, и именно им.
		s.Equal(fmt.Sprintf("in_transit||%d", holder), s.unitLoc(u), "итерация %d", i)
		s.Equal(fmt.Sprintf("1|%d", holder), s.scalarText(`
			SELECT count(*)::text || '|' || coalesce(min(t.id)::text, '') FROM transfer_items i
			JOIN transfers t ON t.id = i.transfer_id
			WHERE i.equipment_id = $1 AND t.status IN ('requested', 'in_transit')`, u), "итерация %d", i)
		// остатки согласованы с движениями (стартовые 10 на «Алматы» заведены без движения);
		// в обоих исходах единица товара ушла с «Москвы»: откатом либо отправкой B.
		s.Equal("0.000", s.pairStock(p, s.moscow), "итерация %d", i)
		s.Equal(s.pairStock(p, s.moscow), s.productMovementsSum(p, s.moscow), "итерация %d", i)
		s.Equal("9.000", s.pairStock(p, s.almaty), "итерация %d", i)
		s.Equal("-1.000", s.productMovementsSum(p, s.almaty), "итерация %d", i)
	}
	s.Equal(iterations, unreceiveWon+createWon)
	s.Positive(unreceiveWon, "исход «откат первым» встретился")
	s.Positive(createWon, "исход «новый документ первым» встретился")
	s.T().Logf("исходы гонки: откат первым — %d, новый документ первым — %d", unreceiveWon, createWon)
}

// ---- TS-016 ----

func (s *TransfersLiveSuite) deleteDoc(id int64, version any) (int, []byte) {
	return s.admin.Do(http.MethodDelete, deletePath(id), deleteBody(version))
}

// TS-016 (transfers-revert; с soft-delete-transfers DS-018 удаление мягкое): удаление документа
// — только «Заявка» и «Отменён». Заявка: документ помечен удалённым, строки на месте, единица
// свободна; отменённый (после «в пути»): движения остатков остаются привязаны к документу,
// остатки не изменились. «В пути» и «Получено» → 409 со своим сообщением, ничего не
// изменилось. Негатив: без version → 400 required, не число → 400 invalid, устаревшая версия
// → 409, повтор → 404.
func (s *TransfersLiveSuite) TestTS016_DeleteDocumentByStatus() {
	p := s.newProduct("20")
	mk := func(send bool) (transferDTO, string) {
		u := s.newUnit(s.almaty)
		body := s.body(productItem(p, 3), unitItem(u))
		body["send"] = send
		return s.create(body), u
	}

	// ---- заявка ----
	requested, uReq := mk(false)
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", requested.ID), s.unitLoc(uReq), "заявка занимает единицу")
	st, raw := s.admin.Do(http.MethodDelete, deletePath(requested.ID), map[string]any{"reason": "Создан по ошибке"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"version": "required"}, s.fields(raw))
	st, raw = s.deleteDoc(requested.ID, "abc")
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"version": "invalid"}, s.fields(raw))
	st, raw = s.deleteDoc(requested.ID, requested.Version+3)
	s.expectConflict(st, raw, "TRANSFER_VERSION_CONFLICT", "Перемещение уже изменено, обновите", "устаревшая версия")
	s.Equal("requested", s.get(requested.ID).Status, "отказы документ не тронули")

	st, raw = s.deleteDoc(requested.ID, requested.Version)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var deleted struct {
		ID        int64     `json:"id"`
		DeletedAt time.Time `json:"deleted_at"`
	}
	s.Require().NoError(json.Unmarshal(raw, &deleted), string(raw))
	s.Equal(requested.ID, deleted.ID)
	s.WithinDuration(time.Now(), deleted.DeletedAt, time.Minute)
	st, _, raw = s.do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", requested.ID), nil)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("requested|true", s.scalarText(`SELECT status || '|' || (deleted_at IS NOT NULL)::text FROM transfers WHERE id = $1`, requested.ID),
		"документ помечен удалённым, статус не изменился")
	s.Equal("2", s.scalarText(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = $1`, requested.ID), "строки на месте")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(uReq), "единица заявки свободна")
	s.Equal("20.000", s.pairStock(p, s.almaty), "заявка остаток не двигала")
	// свободную единицу можно поставить в новый документ.
	reuse := s.create(s.body(unitItem(uReq)))
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", reuse.ID), s.unitLoc(uReq))
	// повтор → 404.
	st, raw = s.deleteDoc(requested.ID, requested.Version+1)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TRANSFER_NOT_FOUND", s.errOf(raw).Code)

	// ---- в пути и получено: 409, ничего не изменилось ----
	inTransit, uTransit := mk(true)
	st, raw = s.deleteDoc(inTransit.ID, inTransit.Version)
	s.expectConflict(st, raw, "TRANSFER_STATUS_CONFLICT", "Перемещение в пути — сначала отмените его", "delete in_transit")
	s.Equal(fmt.Sprintf("in_transit||%d", inTransit.ID), s.unitLoc(uTransit))
	s.Equal("17.000", s.pairStock(p, s.almaty))

	rcvDoc, uRcv := mk(true)
	received := s.must(rcvDoc, "receive", nil)
	st, raw = s.deleteDoc(received.ID, received.Version)
	s.expectConflict(st, raw, "TRANSFER_STATUS_CONFLICT",
		"Перемещение получено — сначала отмените проведение, затем отмените", "delete received")
	s.Equal("received", s.get(received.ID).Status)
	s.Equal("at_warehouse|Москва|", s.unitLoc(uRcv))
	s.Equal("14.000", s.pairStock(p, s.almaty))
	s.Equal("3.000", s.pairStock(p, s.moscow))
	s.Len(s.movements(received.ID), 2)

	// ---- отменён (после «в пути»): путь «откат → отмена → удаление» у полученного ----
	back := s.mustUnreceive(received, "удаляем ошибочный документ")
	cancelled := s.must(back, "cancel", nil)
	s.Equal("17.000", s.pairStock(p, s.almaty))
	s.Equal("0.000", s.pairStock(p, s.moscow))
	before := s.scalarText(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1`, p)
	s.Len(s.movements(cancelled.ID), 4, "out, in, unreceive, return")
	st, raw = s.deleteDoc(cancelled.ID, cancelled.Version)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled|true", s.scalarText(`SELECT status || '|' || (deleted_at IS NOT NULL)::text FROM transfers WHERE id = $1`, cancelled.ID))
	s.Equal("2", s.scalarText(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = $1`, cancelled.ID), "строки на месте")
	s.Len(s.movements(cancelled.ID), 4, "движения остаются привязаны к документу")
	s.Equal(before, s.scalarText(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1`, p), "журнал остатков сохранён")
	s.Equal("17.000", s.pairStock(p, s.almaty), "удаление остаток не меняет")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(uRcv))
}

// ---- TS-015 (хвост) ----

// TS-015: «Отменить проведение» и удаление документа — только администратор. Исполнитель
// получает 403 и для документов с клиентом (которые сам проводит), и для чужих направлений, и
// для несуществующего документа (роль проверяется первой); документы не изменились.
func (s *TransfersLiveSuite) TestTS015_ExecutorCannotUnreceiveOrDelete() {
	exec := s.executor()
	taskID, units := s.taskWithUnits(2)
	// документ с клиентом, проведённый самим исполнителем.
	st, own, raw := s.doAs(exec, http.MethodPost, "/api/v1/transfers", with(intakeBody(taskID, s.almaty, units[0]), "receive", true))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, ownReq, raw := s.doAs(exec, http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[1]))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	// документ чужого направления.
	w2w := s.must(s.create(with(s.body(unitItem(s.newUnit(s.almaty))), "send", true)), "receive", nil)
	w2wReq := s.create(s.body(unitItem(s.newUnit(s.almaty))))

	forbidden := func(st int, raw []byte, ctx string) {
		s.Equal(http.StatusForbidden, st, "%s: %s", ctx, raw)
		e := s.errOf(raw)
		s.Equal("FORBIDDEN", e.Code, ctx)
		s.Equal("Нет доступа", e.Message, ctx)
	}
	for _, doc := range []transferDTO{own, w2w} {
		st, raw := exec.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/unreceive", doc.ID),
			map[string]any{"version": doc.Version, "reason": "исполнитель"})
		forbidden(st, raw, "unreceive "+doc.Direction)
		after := s.get(doc.ID)
		s.Equal("received", after.Status)
		s.Equal(doc.Version, after.Version)
	}
	for _, doc := range []transferDTO{ownReq, w2wReq} {
		st, raw := exec.Do(http.MethodDelete, deletePath(doc.ID), deleteBody(doc.Version))
		forbidden(st, raw, "delete "+doc.Direction)
		s.Equal("requested", s.get(doc.ID).Status)
	}
	st, raw = exec.Do(http.MethodPost, "/api/v1/transfers/999999999/unreceive", map[string]any{"version": 1, "reason": "x"})
	forbidden(st, raw, "unreceive несуществующего")
	st, raw = exec.Do(http.MethodDelete, deletePath(999999999), deleteBody(1))
	forbidden(st, raw, "delete несуществующего")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(units[0]), "единица проведённого приёма на месте")

	// администратору те же действия доступны.
	s.mustUnreceive(own, "админ")
	st, raw = s.deleteDoc(ownReq.ID, ownReq.Version)
	s.Equal(http.StatusOK, st, string(raw))
}
