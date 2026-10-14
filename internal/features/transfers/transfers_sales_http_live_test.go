//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Накладная клиенту — товар с ценой в документе «склад → клиент»
// (docs/features/transfers-sales/draft-spec.md §11) — методы TransfersLiveSuite (§TD-8):
// TS-003 (цена: валидация, цена каталога по умолчанию, правка только в заявке), TS-014 (сумма
// документа и списка), хвосты TS-002 (товар в «склад → клиент» допустим) и TS-007 (движения
// client_out / transfer_return).

// ---- фикстуры ----

// newPricedProduct — товар с остатком stock на «Алматы» и ценой каталога price.
func (s *TransfersLiveSuite) newPricedProduct(stock, price string) string {
	id := s.newProduct(stock)
	s.setCatalogPrice(id, price)
	return id
}

func (s *TransfersLiveSuite) setCatalogPrice(productID, price string) {
	_, err := s.Pool.Exec(context.Background(), `UPDATE products SET price = $2::numeric WHERE id = $1`, productID, price)
	s.Require().NoError(err)
}

// pricedItem — товарная строка накладной с ценой продажи (JSON-число).
func pricedItem(productID string, qty, price any) map[string]any {
	return map[string]any{"kind": "product", "product_id": productID, "qty_sent": qty, "price": price}
}

// invoiceBody — накладная «склад → клиент» со склада «Алматы» по заказу taskID.
func (s *TransfersLiveSuite) invoiceBody(taskID int64, items ...map[string]any) map[string]any {
	return map[string]any{"direction": "warehouse_to_client", "from_warehouse_id": s.almaty, "task_id": taskID, "items": items}
}

// taskWithStockUnit — заказ клиента фикстуры с единицей на складе «Алматы» в его оборудовании.
func (s *TransfersLiveSuite) taskWithStockUnit() (int64, string) {
	taskID := s.newTask(s.clientID)
	unit := s.newUnitOf(s.clientID, s.almaty)
	s.attach(taskID, unit)
	return taskID, unit
}

func num(n *json.Number) string {
	if n == nil {
		return "<null>"
	}
	return n.String()
}

// ---- TS-002 (хвост) ----

// TS-002: товар в «склад → клиент» теперь допустим (201) — и вместе с единицей, и один; в
// «клиент → склад» по-прежнему 400 items[i].kind: not_allowed.
func (s *TransfersLiveSuite) TestTS002_ProductAllowedInClientReturn() {
	taskID, unit := s.taskWithStockUnit()
	p := s.newPricedProduct("5", "100")

	for _, send := range []bool{false, true} {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers",
			with(intakeBody(taskID, s.almaty), "items", []map[string]any{productItem(p, 1)}, "send", send))
		s.Equal(http.StatusBadRequest, st, "товар в приёме от клиента (send=%v) → %s", send, raw)
		s.Equal(map[string]string{"items[0].kind": "not_allowed"}, s.fields(raw))
	}

	st, t, raw := s.do(http.MethodPost, "/api/v1/transfers", s.invoiceBody(taskID, unitItem(unit), productItem(p, 2)))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("warehouse_to_client", t.Direction)
	s.Require().Len(t.Items, 2)
	s.Equal("equipment", t.Items[0].Kind)
	s.Equal("product", t.Items[1].Kind)
	s.Equal(p, t.Items[1].ProductID)
	s.Equal("2.000", num(t.Items[1].QtySent))

	// накладная из одного товара, без оборудования.
	st, only, raw := s.do(http.MethodPost, "/api/v1/transfers", s.invoiceBody(taskID, productItem(p, 1)))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(only.Items, 1)

	// правкой товар добавляется и в существующий «возврат клиенту».
	st, upd, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", only.ID),
		map[string]any{"version": only.Version, "from_warehouse_id": s.almaty, "items": []map[string]any{productItem(p, 3)}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("3.000", num(upd.Items[0].QtySent))
}

// ---- TS-003 ----

// TS-003: цена товарной строки накладной — в ответе price и line_total; не передана либо null
// → цена каталога на момент сохранения (дальнейшие правки каталога документ не меняют; при
// правке без цены строка получает текущую цену каталога — строки заменяются целиком);
// отрицательная → negative; три знака, больше максимума → invalid; не число → 400; цена в
// «склад → склад», «поставщик → склад» и в строке-единице → not_allowed. Отклонённый запрос
// документ не создаёт.
func (s *TransfersLiveSuite) TestTS003_PriceValidationAndCatalogDefault() {
	taskID, unit := s.taskWithStockUnit()
	p := s.newPricedProduct("50", "150.50")
	p2 := s.newPricedProduct("50", "70")

	// цена передана: хранится как есть, line_total = price × qty_sent (2 знака).
	t := s.create(s.invoiceBody(taskID, unitItem(unit), pricedItem(p, 2, 18500.5), pricedItem(p2, 1.5, 0)))
	s.Equal("requested", t.Status)
	s.Equal("<null>", num(t.Items[0].Price), "у строки-единицы цены нет")
	s.Equal("<null>", num(t.Items[0].LineTotal))
	s.Equal("18500.50", num(t.Items[1].Price))
	s.Equal("37001.00", num(t.Items[1].LineTotal))
	s.Equal("0.00", num(t.Items[2].Price), "нулевая цена допустима")
	s.Equal("0.00", num(t.Items[2].LineTotal))
	s.Equal("37001.00", t.Amount.String())

	// не передана / null → цена каталога на момент сохранения.
	task2 := s.newTask(s.clientID)
	def := s.create(s.invoiceBody(task2, productItem(p, 2), pricedItem(p2, 3, nil)))
	s.Equal("150.50", num(def.Items[0].Price), "нет поля → цена каталога")
	s.Equal("301.00", num(def.Items[0].LineTotal))
	s.Equal("70.00", num(def.Items[1].Price), "null → цена каталога")
	s.Equal("210.00", num(def.Items[1].LineTotal))
	s.Equal("511.00", def.Amount.String())

	// цена каталога изменилась — в документе остаётся сохранённая (DS-039).
	s.setCatalogPrice(p, "999")
	s.Equal("150.50", num(s.get(def.ID).Items[0].Price))
	// правка без цены: строки заменяются целиком — берётся текущая цена каталога.
	st, upd, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", def.ID), map[string]any{
		"version": def.Version, "from_warehouse_id": s.almaty,
		"items": []map[string]any{productItem(p, 2), pricedItem(p2, 3, 80.25)}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("999.00", num(upd.Items[0].Price))
	s.Equal("80.25", num(upd.Items[1].Price), "цена правится в документе")
	s.Equal("2238.75", upd.Amount.String(), "1998.00 + 240.75")
	s.Equal("70.00", s.scalarText(`SELECT price::text FROM products WHERE id = $1`, p2), "каталог документом не меняется")

	// округление line_total до 2 знаков: 0.333 × 0.01 = 0.00333 → 0.00; 1.005 × 3.33 = 3.34665 → 3.35.
	task3 := s.newTask(s.clientID)
	r := s.create(s.invoiceBody(task3, pricedItem(p, 0.333, 0.01), pricedItem(p2, 1.005, 3.33)))
	s.Equal("0.00", num(r.Items[0].LineTotal))
	s.Equal("3.35", num(r.Items[1].LineTotal))
	s.Equal("3.35", r.Amount.String(), "сумма документа = Σ line_total")
	// округление ровно на «половине» — half-up, как round(numeric, 2): 0.15 × 1.5 = 0.225 → 0.23;
	// 0.10 × 0.35 = 0.035 → 0.04 (float-арифметика дала бы 0.22 и 0.03).
	half := s.create(s.invoiceBody(task3, pricedItem(p, 1.5, 0.15), pricedItem(p2, 0.35, 0.10)))
	s.Equal("0.23", num(half.Items[0].LineTotal))
	s.Equal("0.04", num(half.Items[1].LineTotal))
	s.Equal("0.27", half.Amount.String(), "сумма документа = Σ округлённых строк")
	s.Equal("0.27", s.get(half.ID).Amount.String())
	// верхняя граница цены.
	maxDoc := s.create(s.invoiceBody(task3, pricedItem(p, 1, json.Number("9999999999.99"))))
	s.Equal("9999999999.99", num(maxDoc.Items[0].Price))

	before := s.transfersCount()
	sup := s.newSupplier(true)
	for name, tc := range map[string]struct {
		body map[string]any
		want map[string]string
	}{
		"отрицательная":            {s.invoiceBody(taskID, pricedItem(p, 1, -1)), map[string]string{"items[0].price": "negative"}},
		"отрицательная копейка":    {s.invoiceBody(taskID, pricedItem(p, 1, -0.01)), map[string]string{"items[0].price": "negative"}},
		"три знака":                {s.invoiceBody(taskID, pricedItem(p, 1, 1.999)), map[string]string{"items[0].price": "invalid"}},
		"больше максимума":         {s.invoiceBody(taskID, pricedItem(p, 1, json.Number("10000000000"))), map[string]string{"items[0].price": "invalid"}},
		"1e12":                     {s.invoiceBody(taskID, pricedItem(p, 1, json.Number("1e12"))), map[string]string{"items[0].price": "invalid"}},
		"вторая строка":            {s.invoiceBody(taskID, pricedItem(p, 1, 5), pricedItem(p2, 1, -3)), map[string]string{"items[1].price": "negative"}},
		"цена и количество":        {s.invoiceBody(taskID, pricedItem(p, 0, 1.999)), map[string]string{"items[0].qty_sent": "not_positive", "items[0].price": "invalid"}},
		"цена в склад → склад":     {s.w2wBody(pricedItem(p, 1, 100)), map[string]string{"items[0].price": "not_allowed"}},
		"нулевая в склад → склад":  {s.w2wBody(pricedItem(p, 1, 0)), map[string]string{"items[0].price": "not_allowed"}},
		"цена в приходе":           {with(s.supplyBody(sup, p, 1), "items", []map[string]any{pricedItem(p, 1, 100)}), map[string]string{"items[0].price": "not_allowed"}},
		"цена у единицы накладной": {s.invoiceBody(taskID, map[string]any{"kind": "equipment", "equipment_id": unit, "price": 100}), map[string]string{"items[0].price": "not_allowed"}},
	} {
		for _, send := range []bool{false, true} {
			st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", with(tc.body, "send", send))
			s.Equal(http.StatusBadRequest, st, "%s (send=%v) → %s", name, send, raw)
			s.Equal(tc.want, s.fields(raw), "%s (send=%v)", name, send)
		}
	}
	// не число (строка) → 400 на разборе тела — без 5xx, документ не создан.
	for _, bad := range []any{"abc", "18500.00", true, []int{1}} {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", s.invoiceBody(taskID, pricedItem(p, 1, bad)))
		s.Equal(http.StatusBadRequest, st, "price=%v → %s", bad, raw)
	}
	s.Equal(before, s.transfersCount(), "отклонённые запросы документ не создали")

	// те же правила — при правке заявки; документ остаётся прежним.
	for price, want := range map[any]string{-1: "negative", 1.999: "invalid"} {
		st, _, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
			"version": t.Version, "from_warehouse_id": s.almaty, "items": []map[string]any{pricedItem(p, 2, price)}})
		s.Equal(http.StatusBadRequest, st, string(raw))
		s.Equal(map[string]string{"items[0].price": want}, s.fields(raw))
	}
	same := s.get(t.ID)
	s.Equal(t.Version, same.Version)
	s.Equal("37001.00", same.Amount.String())
}

// scalarText — одно текстовое значение из БД (проверка состояния, не предмет теста).
func (s *TransfersLiveSuite) scalarText(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

// TS-003 (негатив, abuse case 3): правка цены вне «Заявки» → 409 TRANSFER_STATUS_CONFLICT со
// своим сообщением; цена и сумма документа не меняются.
func (s *TransfersLiveSuite) TestTS003_PriceEditOnlyInRequested() {
	p := s.newPricedProduct("50", "100")
	edit := func(t transferDTO) (int, []byte) {
		st, _, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
			"version": t.Version, "from_warehouse_id": s.almaty, "items": []map[string]any{pricedItem(p, 1, 1)}})
		return st, raw
	}
	check := func(t transferDTO, msg string) {
		st, raw := edit(t)
		s.Equal(http.StatusConflict, st, string(raw))
		e := s.errOf(raw)
		s.Equal("TRANSFER_STATUS_CONFLICT", e.Code)
		s.Equal(msg, e.Message)
		got := s.get(t.ID)
		s.Equal("100.00", num(got.Items[0].Price), "цена не изменилась")
		s.Equal("200.00", got.Amount.String(), "сумма не изменилась")
	}

	task := s.newTask(s.clientID)
	sent := s.create(with(s.invoiceBody(task, pricedItem(p, 2, 100)), "send", true))
	s.Equal("in_transit", sent.Status)
	check(sent, "Перемещение уже отправлено — править нельзя")

	st, got, raw := s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	check(got, "Перемещение уже получено — править нельзя")

	req := s.create(s.invoiceBody(task, pricedItem(p, 2, 100)))
	st, cancelled, raw := s.action(req, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	check(cancelled, "Перемещение отменено — править нельзя")
}

// ---- TS-007 (хвост) ----

// TS-007: накладная «склад → клиент» — заявка движений не создаёт; отправка → client_out
// −qty_sent на складе отправления (остаток может уйти в минус — warnings); приёмка — без
// движений, qty_received остаётся null, items → 400 not_allowed; отмена «в пути» →
// transfer_return +qty_sent; отмена заявки — без движений. Остаток product_stock сходится с
// суммой движений. Единица документа проходит тот же путь, что и в части 4.
func (s *TransfersLiveSuite) TestTS007_InvoiceMovements() {
	taskID, unit := s.taskWithStockUnit()
	p := s.newPricedProduct("0", "250") // остаток 0 без движений — дальше он равен сумме движений
	low := s.newPricedProduct("0", "10")

	t := s.create(s.invoiceBody(taskID, unitItem(unit), pricedItem(p, 3, 250), productItem(low, 1.5)))
	s.Empty(s.movements(t.ID), "заявка остаток не двигает")
	s.Equal("0.000", s.pairStock(p, s.almaty))

	st, sent, raw := s.action(t, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("in_transit", sent.Status)
	s.ElementsMatch([]string{"client_out:Алматы:-3.000", "client_out:Алматы:-1.500"}, s.movements(t.ID))
	s.Equal("-3.000", s.pairStock(p, s.almaty))
	s.Equal("-1.500", s.pairStock(low, s.almaty))
	s.Equal("765.00", sent.Amount.String(), "750.00 + 15.00")
	// движения накладной — без task_id: связь с заказом идёт через transfer_id (§4.2).
	s.Equal("0", s.scalarText(`SELECT count(*)::text FROM stock_movements WHERE transfer_id = $1 AND task_id IS NOT NULL`, t.ID))
	// отправка в минус разрешена — с предупреждением по каждому товару.
	s.Require().Len(sent.Warnings, 2, string(raw))
	kinds := map[string]string{}
	for _, w := range sent.Warnings {
		s.Equal(s.almaty, w.WarehouseID)
		kinds[w.ProductID] = w.Kind + ":" + w.Stock.String()
	}
	s.Equal(map[string]string{p: "negative:-3.000", low: "negative:-1.500"}, kinds)
	s.Equal("in_transit||"+fmt.Sprint(t.ID), s.unitLoc(unit))

	// приёмка: факт по товару не вводится.
	st, _, raw = s.action(sent, "receive", map[string]any{"items": []map[string]any{{"id": sent.Items[1].ID, "qty_received": 3}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items": "not_allowed"}, s.fields(raw))
	st, got, raw := s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", got.Status)
	s.Equal("<null>", num(got.Items[1].QtyReceived), "факт по товару накладной не вводится")
	s.Equal("3.000", num(got.Items[1].QtySent))
	s.ElementsMatch([]string{"client_out:Алматы:-3.000", "client_out:Алматы:-1.500"}, s.movements(t.ID), "приёмка движений не создаёт")
	s.Equal("-3.000", s.pairStock(p, s.almaty))
	s.Equal("765.00", got.Amount.String())
	s.Equal("at_client||", s.unitLoc(unit))
	s.Equal(s.pairStock(p, s.almaty), s.productMovementsSum(p, s.almaty), "остаток = сумма движений")
	s.Equal(s.pairStock(low, s.almaty), s.productMovementsSum(low, s.almaty))

	// отмена «в пути»: товар возвращается на склад отправления.
	c := s.create(with(s.invoiceBody(taskID, pricedItem(p, 2, 250)), "send", true))
	s.ElementsMatch([]string{"client_out:Алматы:-2.000"}, s.movements(c.ID), "send при создании списывает так же")
	s.Equal("-5.000", s.pairStock(p, s.almaty))
	st, cancelled, raw := s.action(c, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", cancelled.Status)
	s.ElementsMatch([]string{"client_out:Алматы:-2.000", "transfer_return:Алматы:2.000"}, s.movements(c.ID))
	s.Equal("-3.000", s.pairStock(p, s.almaty))
	s.Equal("500.00", cancelled.Amount.String(), "сумма документа не обнуляется — из итога заказа её убирает статус")
	s.Equal(s.pairStock(p, s.almaty), s.productMovementsSum(p, s.almaty))

	// отмена заявки: движений нет.
	r := s.create(s.invoiceBody(taskID, pricedItem(p, 4, 250)))
	st, _, raw = s.action(r, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(s.movements(r.ID))
	s.Equal("-3.000", s.pairStock(p, s.almaty))

	// достаточный остаток — без предупреждений; остаток на «Москве» не затронут.
	rich := s.newPricedProduct("10", "5")
	ok := s.create(with(s.invoiceBody(taskID, pricedItem(rich, 4, 5)), "send", true))
	s.Empty(ok.Warnings)
	s.Equal("6.000", s.pairStock(rich, s.almaty))
	s.Equal("0", s.pairStock(rich, s.moscow))
}

// TS-007 (негатив, §3 п.2): двойная параллельная отправка накладной — один успех, остальные
// 409; списание одно.
func (s *TransfersLiveSuite) TestTS007_ParallelSendInvoice_SingleWriteOff() {
	task := s.newTask(s.clientID)
	p := s.newPricedProduct("0", "40")
	t := s.create(s.invoiceBody(task, pricedItem(p, 6, 40)))

	var wg sync.WaitGroup
	codes := make([]int, 4)
	bodies := make([][]byte, len(codes))
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], bodies[i] = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/send", t.ID), map[string]any{"version": t.Version})
		}(i)
	}
	wg.Wait()
	okCount := 0
	for i, c := range codes {
		if c == http.StatusOK {
			okCount++
			continue
		}
		s.Equal(http.StatusConflict, c, string(bodies[i]))
	}
	s.Equal(1, okCount, "ровно одна отправка")
	s.ElementsMatch([]string{"client_out:Алматы:-6.000"}, s.movements(t.ID), "списание не удвоилось")
	s.Equal("-6.000", s.pairStock(p, s.almaty))

	// повторная отправка с актуальной версией → 409 статуса, списание не удваивается (§6.4).
	cur := s.get(t.ID)
	st, _, raw := s.action(cur, "send", nil)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TRANSFER_STATUS_CONFLICT", s.errOf(raw).Code)
	s.Equal("-6.000", s.pairStock(p, s.almaty))
}

// ---- TS-014 (документ и список) ----

// TS-014: amount в объекте документа и в списке = Σ price × qty_sent товарных строк; накладная
// без товаров и документы других направлений → 0.00, price / line_total у них null; сумма —
// у документа в любом статусе.
func (s *TransfersLiveSuite) TestTS014_DocumentAndListAmount() {
	taskID, unit := s.taskWithStockUnit()
	p := s.newPricedProduct("100", "10")
	p2 := s.newPricedProduct("100", "20")

	inv := s.create(s.invoiceBody(taskID, pricedItem(p, 3, 10.1), pricedItem(p2, 2, 20.25)))
	s.Equal("70.80", inv.Amount.String(), "30.30 + 40.50")
	unitsOnly := s.create(s.invoiceBody(taskID, unitItem(unit)))
	s.Equal("0.00", unitsOnly.Amount.String(), "накладная без товаров")
	s.Equal("<null>", num(unitsOnly.Items[0].Price))

	w2w := s.create(s.w2wBody(productItem(p, 5)))
	s.Equal("0.00", w2w.Amount.String(), "вне накладной клиенту суммы нет")
	s.Equal("<null>", num(w2w.Items[0].Price))
	s.Equal("<null>", num(w2w.Items[0].LineTotal))
	supply := s.create(s.supplyBody(s.newSupplier(true), p, 5))
	s.Equal("0.00", supply.Amount.String())
	s.Equal("<null>", num(supply.Items[0].Price))

	_, l := s.listIDs(s.admin, fmt.Sprintf("&task_id=%d", taskID))
	amounts := map[int64]string{}
	for _, it := range l.Items {
		amounts[it.ID] = it.Amount.String()
	}
	s.Equal(map[int64]string{inv.ID: "70.80", unitsOnly.ID: "0.00"}, amounts)
	_, all := s.listIDs(s.admin, "")
	for _, it := range all.Items {
		if it.ID == w2w.ID || it.ID == supply.ID {
			s.Equal("0.00", it.Amount.String(), it.Direction)
		}
	}

	// сумма в ответах переходов и GET — та же.
	st, sent, raw := s.action(inv, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("70.80", sent.Amount.String())
	s.Equal("70.80", s.get(inv.ID).Amount.String())
	s.Contains(string(raw), `"amount":70.80`, "деньги на wire — JSON-число с двумя знаками")
	s.Contains(string(raw), `"price":10.10`)
	s.Contains(string(raw), `"line_total":30.30`)
}

// TS-003 / TS-015: исполнитель работает с ценами накладной так же, как администратор —
// создаёт накладную с товаром и ценой, правит цену, отправляет и принимает.
func (s *TransfersLiveSuite) TestTS003_ExecutorWorksWithPrices() {
	ex := s.executor()
	taskID, unit := s.taskWithStockUnit()
	p := s.newPricedProduct("10", "300")

	st, t, raw := s.doAs(ex, http.MethodPost, "/api/v1/transfers", s.invoiceBody(taskID, unitItem(unit), productItem(p, 1)))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("300.00", num(t.Items[1].Price), "цена каталога по умолчанию")

	st, upd, raw := s.doAs(ex, http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
		"version": t.Version, "from_warehouse_id": s.almaty, "items": []map[string]any{unitItem(unit), pricedItem(p, 2, 275.5)}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("551.00", upd.Amount.String())

	st, sent, raw := s.doAs(ex, http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/send", t.ID), map[string]any{"version": upd.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.ElementsMatch([]string{"client_out:Алматы:-2.000"}, s.movements(t.ID))
	st, got, raw := s.doAs(ex, http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", t.ID), map[string]any{"version": sent.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", got.Status)
	s.Equal("551.00", got.Amount.String())

	// цена в «склад → склад» исполнителю недоступна вместе с самим направлением (403).
	st, _, raw = s.doAs(ex, http.MethodPost, "/api/v1/transfers", with(s.w2wBody(pricedItem(p, 1, 1)), "direction", "warehouse_to_warehouse"))
	s.Equal(http.StatusForbidden, st, string(raw))
}
