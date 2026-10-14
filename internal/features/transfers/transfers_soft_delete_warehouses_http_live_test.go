//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Мягкое удаление складов и поставщиков в документах «Перемещение»
// (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §6.5, §11) — методы
// TransfersLiveSuite (§TD-8: боевой бинарь, реальный логин): TS-059 (восстановление заявки с
// удалённым складом или поставщиком, DS-045), TS-060 (отмена проведения документа с удалённым
// складом; удалённый поставщик не мешает, DS-046), TS-064 (отправка заявки, DS-047), TS-032/TS-055
// негатив (удалённые склад и поставщик в новом документе, DS-014), пометки *_is_deleted и гонки
// §3. Склад и поставщик создаются и удаляются через API справочников того же сервера.

// ---- wire-типы (§TD-6.2) ----

// whDocFlags — новые поля документа: признаки удаления точек маршрута (§6.5).
type whDocFlags struct {
	ID                     int64 `json:"id"`
	FromWarehouseIsDeleted bool  `json:"from_warehouse_is_deleted"`
	ToWarehouseIsDeleted   bool  `json:"to_warehouse_is_deleted"`
	SupplierIsDeleted      bool  `json:"supplier_is_deleted"`
}

type whIDResp struct {
	ID string `json:"id"`
}

const whReason = "Создан по ошибке"

// ---- помощники ----

// whNew — действующий активный склад, созданный через API справочника.
func (s *TransfersLiveSuite) whNew() string {
	s.seq++
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": fmt.Sprintf("Склад SDW %d", s.seq)})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var d whIDResp
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	return d.ID
}

func (s *TransfersLiveSuite) whDelete(id string) (int, []byte) {
	return s.admin.Do(http.MethodDelete, "/api/v1/warehouses/"+id, map[string]any{"reason": whReason})
}

func (s *TransfersLiveSuite) whMustDelete(id string) {
	st, raw := s.whDelete(id)
	s.Require().Equal(http.StatusOK, st, "удаление склада %s: %s", id, raw)
}

func (s *TransfersLiveSuite) whRestore(id string) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses/"+id+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *TransfersLiveSuite) supMustDelete(id string) {
	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/suppliers/"+id, map[string]any{"reason": whReason})
	s.Require().Equal(http.StatusOK, st, "удаление поставщика %s: %s", id, raw)
}

func (s *TransfersLiveSuite) supRestore(id string) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/suppliers/"+id+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// whExpectRefs — ответ обязан быть 409 TRANSFER_HAS_DELETED_REFS с details.kind = kind и
// details.id = id.
func (s *TransfersLiveSuite) whExpectRefs(st int, raw []byte, kind, id, ctx string) {
	s.Require().Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
	var e catRefErr
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	s.Equal("TRANSFER_HAS_DELETED_REFS", e.Code, ctx)
	s.Equal(catRefsMessage, e.Message, ctx)
	s.Equal(kind, e.Details.Kind, ctx)
	s.Equal(id, e.Details.ID, ctx)
}

// whFlags — признаки удаления точек маршрута документа: «from|to|supplier» из GET документа;
// те же значения обязаны прийти и в строке списка.
func (s *TransfersLiveSuite) whFlags(id int64) string {
	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", id), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var d whDocFlags
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	one := fmt.Sprintf("%t|%t|%t", d.FromWarehouseIsDeleted, d.ToWarehouseIsDeleted, d.SupplierIsDeleted)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/transfers?limit=200", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []whDocFlags `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	for _, it := range l.Items {
		if it.ID == id {
			s.Equal(one, fmt.Sprintf("%t|%t|%t", it.FromWarehouseIsDeleted, it.ToWarehouseIsDeleted, it.SupplierIsDeleted),
				"строка списка — те же признаки, что и документ")
		}
	}
	return one
}

// whStock — остаток товара на складе задан напрямую (фикстура: товар лежит на новом складе).
func (s *TransfersLiveSuite) whStock(productID, warehouseID, stock string) {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, $3::numeric)
		ON CONFLICT (product_id, warehouse_id) DO UPDATE SET stock = EXCLUDED.stock`, productID, warehouseID, stock)
	s.Require().NoError(err)
}

// whMarkDeleted — склад помечен удалённым ПРЯМОЙ записью в базу, в обход правил (TS-064:
// страховка отправки; по правилам склад с действующей заявкой не удаляется).
func (s *TransfersLiveSuite) whMarkDeleted(id string, deleted bool) {
	q := `UPDATE warehouses SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`
	args := []any{id}
	if deleted {
		q = `UPDATE warehouses SET deleted_at = now(), deleted_by = $2, delete_reason = 'В обход правил' WHERE id = $1`
		args = append(args, s.adminID)
	}
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err)
}

func (s *TransfersLiveSuite) whBody(from, to string, items ...map[string]any) map[string]any {
	return map[string]any{"from_warehouse_id": from, "to_warehouse_id": to, "items": items}
}

func (s *TransfersLiveSuite) whSupplyBody(supplierID, to, productID string, qty any) map[string]any {
	return with(s.supplyBody(supplierID, productID, qty), "to_warehouse_id", to)
}

// ---- TS-059 (склад, поставщик) ----

// TS-059 (DS-045): заявка мягко удалена, затем удалён её склад (удалённая заявка склад не
// удерживает, DS-044) — восстановление заявки → 409 TRANSFER_HAS_DELETED_REFS, details.kind =
// warehouse, details.id — удалённый склад; заявка остаётся удалённой. Проверяются ОБЕ точки:
// у первой заявки удалён склад назначения, у второй — склад отправления (проверка только одной
// стороны восстановила бы одну из заявок). После восстановления склада заявка восстанавливается.
// То же — с поставщиком: details.kind = supplier. Негатив: ОТМЕНЁННЫЙ документ с удалёнными
// складом и поставщиком восстанавливается — 200, в документе и в списке — пометки.
func (s *TransfersLiveSuite) TestTS059_RestoreRequestWithDeletedWarehouseOrSupplier() {
	p := s.newProduct("0")
	dst, src := s.whNew(), s.whNew()
	for _, tc := range []struct{ name, from, to, gone string }{
		{"склад назначения", s.almaty, dst, dst},
		{"склад отправления", src, s.moscow, src},
	} {
		name := tc.name
		doc := s.create(s.whBody(tc.from, tc.to, productItem(p, 2)))
		s.mustDeleteDoc(doc)
		s.whMustDelete(tc.gone)

		st, raw := s.restoreDoc(doc.ID)
		s.whExpectRefs(st, raw, "warehouse", tc.gone, "восстановление заявки: "+name)
		s.Equal(fmt.Sprintf("requested|true|%d", doc.Version+1), s.docRow(doc.ID), "%s: заявка осталась удалённой", name)

		s.whRestore(tc.gone)
		st, raw = s.restoreDoc(doc.ID)
		s.Require().Equal(http.StatusOK, st, "%s: %s", name, raw)
		s.Equal(fmt.Sprintf("requested|false|%d", doc.Version+2), s.docRow(doc.ID), name)
		s.Equal("false|false|false", s.whFlags(doc.ID), name)
	}

	// поставщик.
	sup := s.newSupplier(true)
	supply := s.create(s.supplyBody(sup, p, 5))
	s.mustDeleteDoc(supply)
	s.supMustDelete(sup)
	st, raw := s.restoreDoc(supply.ID)
	s.whExpectRefs(st, raw, "supplier", sup, "восстановление заявки удалённого поставщика")
	s.Equal(fmt.Sprintf("requested|true|%d", supply.Version+1), s.docRow(supply.ID), "заявка осталась удалённой")
	s.supRestore(sup)
	st, raw = s.restoreDoc(supply.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("false|false|false", s.whFlags(supply.ID))

	// негатив: отменённый документ не проверяется.
	w, sp := s.whNew(), s.newSupplier(true)
	cancelled := s.must(s.create(s.whSupplyBody(sp, w, p, 1)), "cancel", nil)
	s.mustDeleteDoc(cancelled)
	s.whMustDelete(w)
	s.supMustDelete(sp)
	st, raw = s.restoreDoc(cancelled.ID)
	s.Require().Equal(http.StatusOK, st, "отменённый документ с удалёнными складом и поставщиком восстанавливается: %s", raw)
	s.Equal("false|true|true", s.whFlags(cancelled.ID), "склад назначения и поставщик — с пометкой «удалён»")
}

// ---- TS-060 (склад, поставщик) ----

// TS-060 (DS-046): документ получен, затем его склад удалён (остаток на нём израсходован —
// фикстура) — отмена проведения → 409 TRANSFER_HAS_DELETED_REFS, details.kind = warehouse;
// документ остаётся «Получено» с прежней версией, остатки и журнал не изменились. Проверяются
// ОБЕ точки: приход от поставщика на удалённый склад назначения и «склад → склад» с удалённым
// складом ОТПРАВЛЕНИЯ (откат вернул бы документ в «В пути» — с такими документами склад удалённым
// быть не может). После восстановления склада отмена проведения проходит. Документ удалённого
// поставщика при действующих товаре и складе откатывается — 200.
func (s *TransfersLiveSuite) TestTS060_UnreceiveRefusedForDeletedWarehouse() {
	// приход от поставщика на склад, удалённый после приёмки.
	p, w := s.newProduct("0"), s.whNew()
	rcv := s.catReceive(s.create(with(s.whSupplyBody(s.newSupplier(true), w, p, 5), "send", true)))
	s.Require().Equal("received", rcv.Status)
	s.Require().Equal("5.000", s.pairStock(p, w))
	s.whStock(p, w, "0") // товар израсходован — иначе склад не удалить (DS-021)
	s.whMustDelete(w)
	moves := s.movements(rcv.ID)
	s.Equal("false|true|false", s.whFlags(rcv.ID), "полученный документ показывает удалённый склад с пометкой")

	st, _, raw := s.unreceive(rcv, "Ошибка в факте")
	s.whExpectRefs(st, raw, "warehouse", w, "отмена проведения прихода на удалённый склад")
	s.Equal(fmt.Sprintf("received|false|%d", rcv.Version), s.docRow(rcv.ID), "документ остался «Получено»")
	s.Equal("0.000", s.pairStock(p, w), "остаток на удалённом складе не изменился")
	s.ElementsMatch(moves, s.movements(rcv.ID), "новых движений нет")

	s.whRestore(w)
	back := s.mustUnreceive(rcv, "Ошибка в факте")
	s.Equal("in_transit", back.Status)
	s.Equal("-5.000", s.pairStock(p, w), "откат снял факт приёмки")

	// склад → склад: удалён склад ОТПРАВЛЕНИЯ.
	q, src := s.newProduct("0"), s.whNew()
	s.whStock(q, src, "4")
	ww := s.catReceive(s.create(with(s.whBody(src, s.almaty, productItem(q, 4)), "send", true)))
	s.Require().Equal("0.000|4.000", s.pairStock(q, src)+"|"+s.pairStock(q, s.almaty))
	s.whMustDelete(src)
	st, _, raw = s.unreceive(ww, "Ошибка")
	s.whExpectRefs(st, raw, "warehouse", src, "отмена проведения «склад → склад» с удалённым складом отправления")
	s.Equal(fmt.Sprintf("received|false|%d", ww.Version), s.docRow(ww.ID))
	s.Equal("4.000", s.pairStock(q, s.almaty), "остаток получателя не тронут")

	// удалённый поставщик отмене проведения не мешает.
	r, sup := s.newProduct("0"), s.newSupplier(true)
	bySup := s.catReceive(s.create(with(s.supplyBody(sup, r, 3), "send", true)))
	s.supMustDelete(sup)
	s.Equal("false|false|true", s.whFlags(bySup.ID))
	undone := s.mustUnreceive(bySup, "Ошибка в факте")
	s.Equal("in_transit", undone.Status, "документ удалённого поставщика откатан")
	s.Equal("0.000", s.pairStock(r, s.almaty))
}

// ---- TS-064 ----

// TS-064 (DS-047): заявка, склад которой помечен удалённым в обход правил (прямой записью в
// базу), не отправляется — 409 TRANSFER_HAS_DELETED_REFS, details.kind = warehouse; заявка
// остаётся «Заявка» с прежней версией, остатки и журнал не тронуты. Обе точки: склад назначения и
// склад отправления. После снятия пометки отправка проходит. Заявка «поставщик → склад»,
// поставщик которой удалён после создания, отправляется и принимается — 200. Негатив: удаление
// склада при действующей заявке → 409 WAREHOUSE_IN_USE.
func (s *TransfersLiveSuite) TestTS064_SendWithDeletedWarehouseOrSupplier() {
	p := s.newProduct("6")
	dst, src := s.whNew(), s.whNew()
	s.whStock(p, src, "6")
	toDeleted := s.create(s.whBody(s.almaty, dst, productItem(p, 2)))
	fromDeleted := s.create(s.whBody(src, s.moscow, productItem(p, 3)))

	// негатив: по правилам склад с действующей заявкой не удаляется.
	st, raw := s.whDelete(dst)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("WAREHOUSE_IN_USE", s.errOf(raw).Code)
	s.Equal("На складе есть незавершённые перемещения", s.errOf(raw).Message)

	for name, tc := range map[string]struct {
		doc  transferDTO
		gone string
	}{"склад назначения": {toDeleted, dst}, "склад отправления": {fromDeleted, src}} {
		s.whMarkDeleted(tc.gone, true)
		st, _, raw := s.action(tc.doc, "send", nil)
		s.whExpectRefs(st, raw, "warehouse", tc.gone, "отправка: "+name)
		s.Equal(fmt.Sprintf("requested|false|%d", tc.doc.Version), s.docRow(tc.doc.ID), "%s: заявка не отправлена", name)
		s.Empty(s.movements(tc.doc.ID), "%s: движений нет", name)
		s.whMarkDeleted(tc.gone, false)
	}
	s.Equal("6.000|6.000", s.pairStock(p, s.almaty)+"|"+s.pairStock(p, src), "остатки источников не тронуты")

	sent := s.must(toDeleted, "send", nil)
	s.Equal("in_transit", sent.Status, "после снятия пометки отправка проходит")
	s.Equal("4.000", s.pairStock(p, s.almaty))

	// удалённый поставщик отправке и приёмке не мешает.
	sup := s.newSupplier(true)
	supply := s.create(s.supplyBody(sup, p, 5))
	s.supMustDelete(sup)
	supplySent := s.must(supply, "send", nil)
	s.Equal("in_transit", supplySent.Status, "заявка удалённого поставщика отправлена")
	s.Equal("false|false|true", s.whFlags(supply.ID))
	s.Equal("received", s.catReceive(supplySent).Status)
	s.Equal("9.000", s.pairStock(p, s.almaty), "приход удалённого поставщика принят")
}

// ---- DS-014: удалённые склад и поставщик в новом документе ----

// TS-032 негатив, TS-055 негатив (DS-014, abuse case 2): идентификатор удалённого склада или
// поставщика в новом документе и при смене в заявке → 400 not_found по полю (а не archived и не
// 201); документ не создан и не изменён. Поставщик, уже стоявший в заявке и удалённый позже, при
// пересохранении заявки остаётся (доведение начатого документа, DS-047); сменить его на ДРУГОГО
// удалённого нельзя.
func (s *TransfersLiveSuite) TestDeletedWarehouseSupplier_RejectedInNewDocument() {
	p := s.newProduct("0")
	goneWh, goneSup := s.whNew(), s.newSupplier(true)
	s.whMustDelete(goneWh)
	s.supMustDelete(goneSup)
	before := s.scalarText(`SELECT count(*)::text FROM transfers`)

	for name, tc := range map[string]struct {
		body map[string]any
		want map[string]string
	}{
		"склад назначения":          {s.whBody(s.almaty, goneWh, productItem(p, 1)), map[string]string{"to_warehouse_id": "not_found"}},
		"склад отправления":         {s.whBody(goneWh, s.moscow, productItem(p, 1)), map[string]string{"from_warehouse_id": "not_found"}},
		"поставщик":                 {s.supplyBody(goneSup, p, 1), map[string]string{"supplier_id": "not_found"}},
		"приход на удалённый склад": {s.whSupplyBody(s.newSupplier(true), goneWh, p, 1), map[string]string{"to_warehouse_id": "not_found"}},
	} {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", tc.body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal(tc.want, s.fields(raw), name)
	}
	s.Equal(before, s.scalarText(`SELECT count(*)::text FROM transfers`), "документы не созданы")

	// смена склада заявки на удалённый.
	doc := s.create(s.body(productItem(p, 2)))
	st, _, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", doc.ID),
		with(s.whBody(s.almaty, goneWh, productItem(p, 2)), "version", doc.Version))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"to_warehouse_id": "not_found"}, s.fields(raw))
	s.Equal(fmt.Sprintf("requested|false|%d", doc.Version), s.docRow(doc.ID), "заявка не изменена")

	// поставщик удалён после создания заявки: пересохранение проходит, смена на другого удалённого — нет.
	sup := s.newSupplier(true)
	supply := s.create(s.supplyBody(sup, p, 4))
	s.supMustDelete(sup)
	put := func(supplier string, version int) (int, transferDTO, []byte) {
		return s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", supply.ID), map[string]any{
			"to_warehouse_id": s.almaty, "supplier_id": supplier, "items": []map[string]any{productItem(p, 5)}, "version": version,
		})
	}
	st, _, raw = put(goneSup, supply.Version)
	s.Equal(http.StatusBadRequest, st, "сменить на ДРУГОГО удалённого нельзя: %s", raw)
	s.Equal(map[string]string{"supplier_id": "not_found"}, s.fields(raw))
	st, saved, raw := put(sup, supply.Version)
	s.Require().Equal(http.StatusOK, st, "пересохранение заявки своего удалённого поставщика: %s", raw)
	s.Equal(sup, saved.SupplierID)
	s.Equal("false|false|true", s.whFlags(supply.ID))
}

// ---- гонки (§3 «Конкурентность») ----

// whHoldDelete — «удаление склада в полёте»: строка склада под FOR UPDATE (как в
// warehouses.Service.softDelete) и отметка удаления, транзакция не зафиксирована.
func (s *TransfersLiveSuite) whHoldDelete(warehouseID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, warehouseID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE warehouses SET deleted_at = now(), deleted_by = $2, delete_reason = 'Гонка' WHERE id = $1`, warehouseID, s.adminID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 п. 1 (DS-014, DS-021): удаление склада идёт (строка заблокирована, отметка поставлена,
// транзакция не зафиксирована) — в это окно приходит создание документа с этим складом.
// Создание обязано дождаться удаления на строке склада (FOR SHARE) и получить 400
// to_warehouse_id: not_found; документ не создан. Реализация, читающая «склад не удалён» без
// блокировки, после фиксации создала бы действующую заявку на удалённом складе (201).
func (s *TransfersLiveSuite) TestRace_TransferCreateWaitsForWarehouseDelete() {
	p, w := s.newProduct("0"), s.whNew()
	before := s.scalarText(`SELECT count(*)::text FROM transfers`)

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", s.whBody(s.almaty, w, productItem(p, 1)))
		return st, raw
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal(map[string]string{"to_warehouse_id": "not_found"}, s.fields(res.raw))
	s.Equal(before, s.scalarText(`SELECT count(*)::text FROM transfers`), "документ не создан")
}

// DS-046: удаление склада идёт — в это окно приходит отмена проведения документа, принятого на
// этот склад. Отмена обязана дождаться удаления на строке склада (FOR SHARE, до остатков) и
// получить 409 TRANSFER_HAS_DELETED_REFS: документ остался «Получено», остаток не тронут.
// Реализация без блокировки после фиксации откатила бы документ — остаток только что удалённого
// склада ушёл бы в минус, а документ вернулся бы в «В пути».
func (s *TransfersLiveSuite) TestRace_UnreceiveWaitsForWarehouseDelete() {
	p, w := s.newProduct("0"), s.whNew()
	rcv := s.catReceive(s.create(with(s.whSupplyBody(s.newSupplier(true), w, p, 5), "send", true)))
	s.whStock(p, w, "0")

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) {
		st, _, raw := s.unreceive(rcv, "Гонка")
		return st, raw
	})
	s.whExpectRefs(res.status, res.raw, "warehouse", w, "отмена проведения в гонке с удалением склада")
	s.Equal(fmt.Sprintf("received|false|%d", rcv.Version), s.docRow(rcv.ID))
	s.Equal("0.000", s.pairStock(p, w))
}

// DS-045: удаление склада идёт — в это окно приходит восстановление удалённой заявки с этим
// складом. Восстановление обязано дождаться удаления и получить 409 TRANSFER_HAS_DELETED_REFS;
// заявка осталась удалённой. Реализация без блокировки строки склада не ждала бы вовсе и вернула
// бы действующую заявку на склад, который через мгновение удалён.
func (s *TransfersLiveSuite) TestRace_RestoreRequestWaitsForWarehouseDelete() {
	p, w := s.newProduct("0"), s.whNew()
	doc := s.create(s.whBody(s.almaty, w, productItem(p, 2)))
	s.mustDeleteDoc(doc)

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) { return s.restoreDoc(doc.ID) })
	s.whExpectRefs(res.status, res.raw, "warehouse", w, "восстановление заявки в гонке с удалением склада")
	s.Equal(fmt.Sprintf("requested|true|%d", doc.Version+1), s.docRow(doc.ID))
}

// DS-047: склад заявки помечается удалённым (в обход правил, транзакция не зафиксирована) — в
// это окно приходит отправка заявки. Отправка обязана дождаться на строке склада (FOR SHARE, до
// остатков) и получить 409 TRANSFER_HAS_DELETED_REFS: заявка не отправлена, остаток источника не
// тронут.
func (s *TransfersLiveSuite) TestRace_SendWaitsForWarehouseDelete() {
	p, w := s.newProduct("5"), s.whNew()
	doc := s.create(s.whBody(s.almaty, w, productItem(p, 2)))

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) {
		st, _, raw := s.action(doc, "send", nil)
		return st, raw
	})
	s.whExpectRefs(res.status, res.raw, "warehouse", w, "отправка в гонке с удалением склада")
	s.Equal(fmt.Sprintf("requested|false|%d", doc.Version), s.docRow(doc.ID))
	s.Equal("5.000", s.pairStock(p, s.almaty))
	s.Empty(s.movements(doc.ID))
}

// DS-021 + §3 п. 1: создание заявки идёт (строка склада под FOR SHARE, заявка вставлена,
// транзакция не зафиксирована) — в это окно приходит удаление склада. Удаление обязано дождаться
// документа на строке склада (FOR UPDATE) и затем увидеть действующую заявку: 409
// WAREHOUSE_IN_USE «На складе есть незавершённые перемещения», склад не удалён. Удаление, не
// блокирующее строку склада, не увидело бы незафиксированную заявку и удалило бы склад.
func (s *TransfersLiveSuite) TestRace_WarehouseDeleteWaitsForTransferCreate() {
	w := s.whNew()

	res := s.catRace(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, w)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `
			INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, created_by)
			VALUES ('warehouse_to_warehouse', $1, $2, 'requested', $3)`, s.almaty, w, s.adminID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.whDelete(w) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("WAREHOUSE_IN_USE", s.errOf(res.raw).Code)
	s.Equal("На складе есть незавершённые перемещения", s.errOf(res.raw).Message)
	s.Equal("false", s.scalarText(`SELECT (deleted_at IS NOT NULL)::text FROM warehouses WHERE id = $1`, w))
}
