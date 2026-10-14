//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Мягкое удаление товаров каталога в документах «Перемещение»
// (docs/features/soft-delete-catalog/draft-spec.md §6.9, §11) — методы TransfersLiveSuite
// (§TD-8: боевой бинарь, реальный логин): TS-059 (восстановление заявки с удалённым товаром,
// DS-045), TS-060 (отмена проведения документа с удалённым товаром, DS-046; приём «В пути»
// проходит, DS-042), TS-027 негатив (удалённый товар в новой строке документа), пометка
// product_is_deleted и гонки §3. Товар удаляется через API каталога того же сервера.

// ---- wire-типы (§TD-6.2) ----

// catRefErr — отказ 409 TRANSFER_HAS_DELETED_REFS: details.kind / details.id.
type catRefErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"details"`
}

type catDocItems struct {
	Items []struct {
		Kind             string  `json:"kind"`
		ProductID        *string `json:"product_id"`
		ProductIsDeleted bool    `json:"product_is_deleted"`
	} `json:"items"`
}

const catRefsMessage = "В перемещении есть удалённые записи — сначала восстановите их"

// ---- помощники ----

func (s *TransfersLiveSuite) catDeleteProduct(id string) (int, []byte) {
	return s.admin.Do(http.MethodDelete, "/api/v1/products/"+id, map[string]any{"reason": "Снят с продажи"})
}

func (s *TransfersLiveSuite) catMustDeleteProduct(id string) {
	st, raw := s.catDeleteProduct(id)
	s.Require().Equal(http.StatusOK, st, "удаление товара %s: %s", id, raw)
}

func (s *TransfersLiveSuite) catRestoreProduct(id string) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/products/"+id+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// catExpectRefs — ответ обязан быть 409 TRANSFER_HAS_DELETED_REFS с details.kind = product и
// details.id = productID.
func (s *TransfersLiveSuite) catExpectRefs(st int, raw []byte, productID, ctx string) {
	s.Require().Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
	var e catRefErr
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	s.Equal("TRANSFER_HAS_DELETED_REFS", e.Code, ctx)
	s.Equal(catRefsMessage, e.Message, ctx)
	s.Equal("product", e.Details.Kind, ctx)
	s.Equal(productID, e.Details.ID, ctx)
}

// catProductFlags — product_id → product_is_deleted товарных строк документа (GET).
func (s *TransfersLiveSuite) catProductFlags(id int64) map[string]bool {
	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", id), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var d catDocItems
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	out := map[string]bool{}
	for _, it := range d.Items {
		if it.ProductID != nil {
			out[*it.ProductID] = it.ProductIsDeleted
		} else {
			s.False(it.ProductIsDeleted, "у строки-единицы product_is_deleted = false")
		}
	}
	return out
}

// catZeroStock — остаток товара на всех складах обнулён (фикстура: товар израсходован) — иначе
// товар не удалить (DS-021).
func (s *TransfersLiveSuite) catZeroStock(productID string) {
	_, err := s.Pool.Exec(context.Background(), `UPDATE product_stock SET stock = 0 WHERE product_id = $1`, productID)
	s.Require().NoError(err)
}

// catReceive — приёмка документа с фактом, равным отправленному, по каждой товарной строке.
func (s *TransfersLiveSuite) catReceive(doc transferDTO) transferDTO {
	facts := []map[string]any{}
	for _, it := range doc.Items {
		if it.Kind == "product" {
			s.Require().NotNil(it.QtySent)
			facts = append(facts, fact(it.ID, *it.QtySent))
		}
	}
	return s.must(doc, "receive", map[string]any{"items": facts})
}

// ---- TS-059 (товар) ----

// TS-059 (DS-045): заявка мягко удалена, затем удалён её товар (строки удалённой заявки товар
// не удерживают, DS-014) — восстановление заявки → 409 TRANSFER_HAS_DELETED_REFS, details.kind =
// product, details.id — удалённый товар; заявка остаётся удалённой. В заявке два товара, удалён
// ВТОРОЙ по порядку строк — проверка обязана смотреть все строки (проверка только первой
// восстановила бы заявку). После восстановления товара заявка восстанавливается. Негатив:
// ОТМЕНЁННЫЙ документ с удалённым товаром восстанавливается — 200.
func (s *TransfersLiveSuite) TestTS059_RestoreRequestWithDeletedProduct() {
	keep, gone := s.newProduct("0"), s.newProduct("0")
	u := s.newUnit(s.almaty)
	doc := s.create(s.body(productItem(keep, 2), productItem(gone, 3), unitItem(u)))
	s.mustDeleteDoc(doc)
	s.catMustDeleteProduct(gone)

	st, raw := s.restoreDoc(doc.ID)
	s.catExpectRefs(st, raw, gone, "восстановление заявки")
	s.Equal(fmt.Sprintf("requested|true|%d", doc.Version+1), s.docRow(doc.ID), "заявка осталась удалённой")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u), "единица не занята")

	s.catRestoreProduct(gone)
	st, raw = s.restoreDoc(doc.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(fmt.Sprintf("requested|false|%d", doc.Version+2), s.docRow(doc.ID))
	s.Equal(map[string]bool{keep: false, gone: false}, s.catProductFlags(doc.ID))

	// негатив: отменённый документ не проверяется.
	p := s.newProduct("0")
	cancelled := s.must(s.create(s.body(productItem(p, 1))), "cancel", nil)
	s.mustDeleteDoc(cancelled)
	s.catMustDeleteProduct(p)
	st, raw = s.restoreDoc(cancelled.ID)
	s.Require().Equal(http.StatusOK, st, "отменённый документ с удалённым товаром восстанавливается: %s", raw)
	s.Equal(map[string]bool{p: true}, s.catProductFlags(cancelled.ID), "товар строки — с пометкой «удалён»")
}

// ---- TS-027 негатив, пометка в документе ----

// TS-027 негатив (DS-014, abuse case 2): удалённый товар в НОВОЙ строке документа → 400
// items[i].product_id: not_found — при создании и при правке заявки; документ не создан и не
// изменён. Товар, уже стоявший в заявке и удалённый позже, при пересохранении заявки остаётся
// (как архивный, D52) и отдаётся с product_is_deleted = true; у действующего товара — false.
func (s *TransfersLiveSuite) TestDeletedProduct_NewLineRejected_ExistingLineMarked() {
	inDoc, live, other := s.newProduct("0"), s.newProduct("0"), s.newProduct("0")
	doc := s.create(s.body(productItem(live, 1), productItem(inDoc, 2)))
	s.catMustDeleteProduct(inDoc)
	s.catMustDeleteProduct(other)
	s.Equal(map[string]bool{live: false, inDoc: true}, s.catProductFlags(doc.ID))

	before := s.scalarText(`SELECT count(*)::text FROM transfers`)
	st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", s.body(productItem(live, 1), productItem(other, 1)))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[1].product_id": "not_found"}, s.fields(raw))
	s.Equal(before, s.scalarText(`SELECT count(*)::text FROM transfers`), "документ не создан")

	put := func(items ...map[string]any) (int, []byte) {
		body := s.body(items...)
		body["version"] = s.get(doc.ID).Version
		st, _, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", doc.ID), body)
		return st, raw
	}
	// новая строка с удалённым товаром — отказ, заявка не изменилась.
	st, raw = put(productItem(live, 1), productItem(inDoc, 2), productItem(other, 1))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[2].product_id": "not_found"}, s.fields(raw))
	s.Equal(doc.Version, s.get(doc.ID).Version)
	// пересохранение с уже стоявшим удалённым товаром — проходит.
	st, raw = put(productItem(live, 4), productItem(inDoc, 2))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(map[string]bool{live: false, inDoc: true}, s.catProductFlags(doc.ID))
}

// ---- TS-060 (товар) ----

// TS-060 (DS-046): документ «поставщик → склад» получен, товар израсходован до нуля и удалён —
// отмена проведения → 409 TRANSFER_HAS_DELETED_REFS, details.kind = product, details.id — товар;
// документ остаётся «Получено», остатки и журнал склада не изменились. После восстановления
// товара отмена проведения проходит. То же для «склад → склад» с двумя товарами, из которых
// удалён второй.
func (s *TransfersLiveSuite) TestTS060_UnreceiveRefusedForDeletedProduct() {
	p := s.newProduct("0")
	rcv := s.catReceive(s.create(with(s.supplyBody(s.newSupplier(true), p, 5), "send", true)))
	s.Require().Equal("received", rcv.Status)
	s.Require().Equal("5.000", s.pairStock(p, s.almaty))
	s.catZeroStock(p)
	s.catMustDeleteProduct(p)
	moves := s.movements(rcv.ID)

	st, _, raw := s.unreceive(rcv, "Ошибка в факте")
	s.catExpectRefs(st, raw, p, "отмена проведения прихода")
	s.Equal(fmt.Sprintf("received|false|%d", rcv.Version), s.docRow(rcv.ID), "документ остался «Получено»")
	s.Equal("0.000", s.pairStock(p, s.almaty), "остаток удалённого товара не изменился")
	s.ElementsMatch(moves, s.movements(rcv.ID), "новых движений нет")

	s.catRestoreProduct(p)
	back := s.mustUnreceive(rcv, "Ошибка в факте")
	s.Equal("-5.000", s.pairStock(p, s.almaty), "откат снял факт приёмки")
	s.Equal("in_transit", back.Status)

	// склад → склад: удалён второй товар документа.
	keep, gone := s.newProduct("3"), s.newProduct("4")
	body := s.body(productItem(keep, 3), productItem(gone, 4))
	body["send"] = true
	ww := s.catReceive(s.create(body))
	s.Require().Equal("4.000", s.pairStock(gone, s.moscow))
	s.catZeroStock(gone)
	s.catMustDeleteProduct(gone)
	st, _, raw = s.unreceive(ww, "Ошибка")
	s.catExpectRefs(st, raw, gone, "отмена проведения «склад → склад»")
	s.Equal("3.000|0.000", s.pairStock(keep, s.moscow)+"|"+s.pairStock(gone, s.moscow), "остатки получателя не тронуты — и у действующего товара")
	s.Equal(fmt.Sprintf("received|false|%d", ww.Version), s.docRow(ww.ID))
}

// DS-046, распространённый на отправку (решение оркестратора по ревью части 5): заявка, товар
// которой удалён после её создания, не отправляется — 409 TRANSFER_HAS_DELETED_REFS,
// details.kind = product, details.id — товар; заявка остаётся «Заявка» с прежней версией,
// остатки и журнал склада не изменились (отправка увела бы остаток удалённого товара в минус).
// «Склад → склад»: два товара, удалён второй, у первого остаток есть — он тоже не списан.
// «Поставщик → склад»: отправка остатков не двигает, но тоже отклоняется. После восстановления
// товара отправка проходит. Негатив: отмена документа «В пути» с удалённым товаром проходит —
// товар возвращается на источник (DS-042).
func (s *TransfersLiveSuite) TestSendRefusedForDeletedProduct() {
	keep, gone := s.newProduct("6"), s.newProduct("0")
	ww := s.create(s.body(productItem(keep, 2), productItem(gone, 3)))
	supply := s.create(s.supplyBody(s.newSupplier(true), gone, 5))
	s.catMustDeleteProduct(gone)

	for name, doc := range map[string]transferDTO{"склад → склад": ww, "поставщик → склад": supply} {
		st, _, raw := s.action(doc, "send", nil)
		s.catExpectRefs(st, raw, gone, "отправка: "+name)
		s.Equal(fmt.Sprintf("requested|false|%d", doc.Version), s.docRow(doc.ID), "%s: заявка не отправлена", name)
		s.Empty(s.movements(doc.ID), "%s: движений нет", name)
	}
	s.Equal("6.000|0.000", s.pairStock(keep, s.almaty)+"|"+s.pairStock(gone, s.almaty), "остатки источника не тронуты — и у действующего товара")

	s.catRestoreProduct(gone)
	sent := s.must(ww, "send", nil)
	s.Equal("in_transit", sent.Status)
	s.Equal("4.000|-3.000", s.pairStock(keep, s.almaty)+"|"+s.pairStock(gone, s.almaty))
	s.Equal("in_transit", s.must(supply, "send", nil).Status)

	// негатив: отмена «В пути» с удалённым товаром — без запрета, товар вернулся на источник.
	p := s.newProduct("4")
	body := s.body(productItem(p, 4))
	body["send"] = true
	inTransit := s.create(body)
	s.catMustDeleteProduct(p) // весь товар в пути — на складах ноль
	cancelled := s.must(inTransit, "cancel", nil)
	s.Equal("cancelled", cancelled.Status)
	s.Equal("4.000", s.pairStock(p, s.almaty), "остаток вернулся удалённому товару")
}

// TS-060 негатив (DS-042): приём документа «В пути» с удалённым товаром по-прежнему проходит —
// товар удалён, пока документ в пути (на складах остаток нулевой), приёмка даёт остаток
// удалённому товару; в отчёте «Остатки» — строка с is_deleted = true и принятым количеством.
func (s *TransfersLiveSuite) TestTS060_ReceiveInTransitWithDeletedProduct() {
	p := s.newProduct("4")
	body := s.body(productItem(p, 4))
	body["send"] = true
	sent := s.create(body)
	s.Require().Equal("in_transit", sent.Status)
	s.Require().Equal("0.000", s.pairStock(p, s.almaty), "весь товар в пути")
	s.catMustDeleteProduct(p)

	rcv := s.catReceive(sent)
	s.Equal("received", rcv.Status)
	s.Equal("4.000", s.pairStock(p, s.moscow), "остаток пришёл удалённому товару")
	s.Equal(map[string]bool{p: true}, s.catProductFlags(rcv.ID))

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock?limit=1000", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []struct {
			ProductID string      `json:"product_id"`
			IsDeleted bool        `json:"is_deleted"`
			Stock     json.Number `json:"stock"`
		} `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	found := false
	for _, it := range l.Items {
		if it.ProductID == p {
			found = true
			s.True(it.IsDeleted)
			s.Equal("4.000", it.Stock.String())
		}
	}
	s.True(found, "удалённый товар с остатком виден в отчёте «Остатки»")
}

// ---- гонки (§3 «Конкурентность») ----

func (s *TransfersLiveSuite) catLockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

type catRaceResult struct {
	status int
	raw    []byte
}

// catRace — запрос req уходит, пока тест держит блокировку в tx; по pg_stat_activity тест
// дожидается, что запрос встал в ожидание, затем фиксирует tx и возвращает ответ. Без sleep.
func (s *TransfersLiveSuite) catRace(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) catRaceResult {
	ctx := context.Background()
	commit := hold(ctx)
	base := s.catLockWaiters()
	done := make(chan catRaceResult, 1)
	go func() {
		st, raw := req()
		done <- catRaceResult{st, raw}
	}()
	s.Require().Eventually(func() bool { return s.catLockWaiters() > base }, 10*time.Second, 20*time.Millisecond,
		"запрос ждёт блокировку")
	select {
	case res := <-done:
		s.FailNow("запрос не дождался блокировки", "%d %s", res.status, res.raw)
	default:
	}
	commit()
	return <-done
}

// catHoldProductDelete — «удаление товара в полёте»: те же блокировки и UPDATE, что делает
// catalog.ProductRepo.SoftDelete (товар FOR NO KEY UPDATE → строки product_stock FOR UPDATE →
// отметка), транзакция не зафиксирована.
func (s *TransfersLiveSuite) catHoldProductDelete(productID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE`, productID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM product_stock WHERE product_id = $1 ORDER BY warehouse_id FOR UPDATE`, productID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE products SET deleted_at = now(), deleted_by = $2, delete_reason = 'Гонка' WHERE id = $1`, productID, s.adminID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// DS-046: удаление товара идёт (товар и его остатки заблокированы, отметка поставлена,
// транзакция не зафиксирована) — в это окно приходит отмена проведения документа с этим
// товаром. Отмена обязана дождаться удаления на строке товара (FOR SHARE, до строк остатка) и
// получить 409 TRANSFER_HAS_DELETED_REFS: документ остался «Получено», остаток не тронут.
// Реализация, читающая «товар не удалён» без блокировки, дождалась бы только строк остатка и
// после фиксации откатила бы документ — остаток удалённого товара ушёл бы в минус.
func (s *TransfersLiveSuite) TestRace_UnreceiveWaitsForProductDelete() {
	p := s.newProduct("0")
	rcv := s.catReceive(s.create(with(s.supplyBody(s.newSupplier(true), p, 5), "send", true)))
	s.catZeroStock(p)

	res := s.catRace(s.catHoldProductDelete(p), func() (int, []byte) {
		st, _, raw := s.unreceive(rcv, "Гонка")
		return st, raw
	})
	s.catExpectRefs(res.status, res.raw, p, "отмена проведения в гонке с удалением товара")
	s.Equal(fmt.Sprintf("received|false|%d", rcv.Version), s.docRow(rcv.ID))
	s.Equal("0.000", s.pairStock(p, s.almaty))
}

// DS-046 (отправка): удаление товара идёт (товар и его остатки заблокированы, отметка поставлена,
// транзакция не зафиксирована) — в это окно приходит отправка заявки с этим товаром. Отправка
// обязана дождаться удаления на строке товара (FOR SHARE, до строк остатка) и получить 409
// TRANSFER_HAS_DELETED_REFS: заявка не отправлена, остаток не тронут. Реализация, читающая
// «товар не удалён» без блокировки, дождалась бы только строк остатка и после фиксации списала
// бы товар — остаток только что удалённого товара ушёл бы в минус.
func (s *TransfersLiveSuite) TestRace_SendWaitsForProductDelete() {
	p := s.newProduct("0")
	doc := s.create(s.body(productItem(p, 2)))

	res := s.catRace(s.catHoldProductDelete(p), func() (int, []byte) {
		st, _, raw := s.action(doc, "send", nil)
		return st, raw
	})
	s.catExpectRefs(res.status, res.raw, p, "отправка в гонке с удалением товара")
	s.Equal(fmt.Sprintf("requested|false|%d", doc.Version), s.docRow(doc.ID))
	s.Equal("0.000", s.pairStock(p, s.almaty))
	s.Empty(s.movements(doc.ID))
}

// DS-045: удаление товара идёт — в это окно приходит восстановление удалённой заявки с этим
// товаром. Восстановление обязано дождаться удаления и получить 409 TRANSFER_HAS_DELETED_REFS;
// заявка осталась удалённой.
func (s *TransfersLiveSuite) TestRace_RestoreRequestWaitsForProductDelete() {
	p := s.newProduct("0")
	doc := s.create(s.body(productItem(p, 2)))
	s.mustDeleteDoc(doc)

	res := s.catRace(s.catHoldProductDelete(p), func() (int, []byte) { return s.restoreDoc(doc.ID) })
	s.catExpectRefs(res.status, res.raw, p, "восстановление заявки в гонке с удалением товара")
	s.Equal(fmt.Sprintf("requested|true|%d", doc.Version+1), s.docRow(doc.ID))
}

// §3 п. 5 (DS-014): удаление товара идёт — в это окно приходит создание документа с этим
// товаром в строке. Создание обязано дождаться удаления (строка товара FOR SHARE) и получить 400
// items[0].product_id: not_found; документ не создан.
func (s *TransfersLiveSuite) TestRace_TransferCreateWaitsForProductDelete() {
	p := s.newProduct("0")
	before := s.scalarText(`SELECT count(*)::text FROM transfers`)

	res := s.catRace(s.catHoldProductDelete(p), func() (int, []byte) {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", s.body(productItem(p, 1)))
		return st, raw
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["items[0].product_id"])
	s.Equal(before, s.scalarText(`SELECT count(*)::text FROM transfers`), "документ не создан")
}

// DS-021 + DS-046: отмена проведения идёт (документ под FOR UPDATE, товар под FOR SHARE, остаток
// получателя уже уменьшен, транзакция не зафиксирована) — в это окно приходит удаление товара.
// Удаление обязано дождаться отката на строке товара и затем увидеть остаток, который откат
// оставил: 409 PRODUCT_HAS_STOCK (остаток −5 ≠ 0), товар не удалён.
func (s *TransfersLiveSuite) TestRace_ProductDeleteWaitsForUnreceive() {
	p := s.newProduct("0")
	rcv := s.catReceive(s.create(with(s.supplyBody(s.newSupplier(true), p, 5), "send", true)))
	s.catZeroStock(p)

	res := s.catRace(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM transfers WHERE id = $1 FOR UPDATE`, rcv.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1 FOR SHARE`, p)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM product_stock WHERE product_id = $1 AND warehouse_id = $2 FOR UPDATE`, p, s.almaty)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE product_stock SET stock = stock - 5 WHERE product_id = $1 AND warehouse_id = $2`, p, s.almaty)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.catDeleteProduct(p) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("PRODUCT_HAS_STOCK", s.errOf(res.raw).Code)
	s.Equal("false", s.scalarText(`SELECT (deleted_at IS NOT NULL)::text FROM products WHERE id = $1`, p))
}
