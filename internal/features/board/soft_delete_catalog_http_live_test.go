//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Мягкое удаление записей каталога на доске и в отчёте «Остатки»
// (docs/features/soft-delete-catalog/draft-spec.md §6.9, §8.3, §11): TS-026 (пометки услуги и
// товара в карточке), TS-030 (услуга), TS-027 негатив (удалённый товар в новой строке), TS-054
// (остаток на удалённом товаре) и гонки §3. Сьюта — OrderCardLiveSuite (§TD-8: боевой бинарь,
// реальный логин); услуги и товары каталога удаляются через API каталога того же сервера.

// ---- wire-типы (§TD-6.2) ----

type catCardDTO struct {
	Subtasks []struct {
		ID               string `json:"id"`
		ServiceID        string `json:"service_id"`
		ServiceName      string `json:"service_name"`
		ServiceIsDeleted bool   `json:"service_is_deleted"`
		Status           string `json:"status"`
		Products         []struct {
			ID               string      `json:"id"`
			ProductID        string      `json:"product_id"`
			ProductIsDeleted bool        `json:"product_is_deleted"`
			Qty              json.Number `json:"qty"`
		} `json:"products"`
	} `json:"subtasks"`
}

type catStockRowDTO struct {
	ProductID string      `json:"product_id"`
	IsDeleted bool        `json:"is_deleted"`
	Stock     json.Number `json:"stock"`
	State     string      `json:"state"`
}

type catStockSummaryDTO struct {
	ProductsTotal int `json:"products_total"`
	LowCount      int `json:"low_count"`
	OutCount      int `json:"out_count"`
	WillLowCount  int `json:"will_low_count"`
	RiskCount     int `json:"risk_count"`
}

// ---- помощники ----

func (s *OrderCardLiveSuite) catCard(taskID int64) catCardDTO {
	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var c catCardDTO
	s.Require().NoError(json.Unmarshal(raw, &c), string(raw))
	return c
}

// catDelete — мягкое удаление записи каталога через API: path — /api/v1/{products|services}/:id.
func (s *OrderCardLiveSuite) catDelete(path string) (int, []byte) {
	return s.admin.Do(http.MethodDelete, path, map[string]any{"reason": "Снят с продажи"})
}

func (s *OrderCardLiveSuite) catMustDelete(path string) {
	st, raw := s.catDelete(path)
	s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
}

func (s *OrderCardLiveSuite) catAdminID() string {
	return s.sdScalar(`SELECT id::text FROM users WHERE login = 'card-admin'`)
}

func (s *OrderCardLiveSuite) catAddSubtask(taskID int64, serviceID string) (int, []byte) {
	return s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": serviceID})
}

func (s *OrderCardLiveSuite) catAddProduct(subtaskID, productID string, qty int) (int, []byte) {
	return s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/products", map[string]any{"product_id": productID, "qty": qty})
}

// catStock — строки отчёта «Остатки» по товарам ids (query — параметры списка).
func (s *OrderCardLiveSuite) catStock(query string, ids ...string) map[string]catStockRowDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock?limit=1000"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []catStockRowDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	out := map[string]catStockRowDTO{}
	for _, it := range l.Items {
		for _, id := range ids {
			if it.ProductID == id {
				out[id] = it
			}
		}
	}
	return out
}

func (s *OrderCardLiveSuite) catSummary() catStockSummaryDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock/summary", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var sm catStockSummaryDTO
	s.Require().NoError(json.Unmarshal(raw, &sm), string(raw))
	return sm
}

func (s *OrderCardLiveSuite) catAdjust(productID string, stock any) (int, catStockRowDTO, []byte) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, map[string]any{"stock": stock})
	var row catStockRowDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &row), string(raw))
	}
	return st, row, raw
}

// catSetStock — остаток товара напрямую (фикстура: остаток, появившийся после удаления).
func (s *OrderCardLiveSuite) catSetStock(productID, stock string) {
	_, err := s.Pool.Exec(context.Background(), `UPDATE products SET stock = $2::numeric WHERE id = $1`, productID, stock)
	s.Require().NoError(err)
}

// ---- TS-026 ----

// TS-026 (DS-014): у действующего заказа с услугой и товаром, удалёнными из каталога, в GET
// /api/v1/tasks/:id/card — service_is_deleted и product_is_deleted = true; у действующих услуги
// и товара того же заказа оба признака false (и у всех — до удаления). Работа с услугой заказа,
// удалённой из каталога, идёт как обычно (§8.3 п.4): статус услуги, количество и удаление строки
// удалённого товара.
func (s *OrderCardLiveSuite) TestTS026_CardMarks_ServiceAndProduct() {
	taskID, _ := s.newTask(s.newClient("SD-026 Клиент "+uuid.NewString()[:8]), s.columnID("Новые"))
	svcGone, svcLive := s.newService("100"), s.newService("200")
	pGone, pLive := s.newProduct("10", "0", "0"), s.newProduct("20", "0", "0")

	var gone, live string
	for _, svc := range []string{svcGone, svcLive} {
		st, raw := s.catAddSubtask(taskID, svc)
		s.Require().Equal(http.StatusCreated, st, string(raw))
	}
	for _, sub := range s.catCard(taskID).Subtasks {
		if sub.ServiceID == svcGone {
			gone = sub.ID
		} else {
			live = sub.ID
		}
	}
	for _, p := range []string{pGone, pLive} {
		st, raw := s.catAddProduct(live, p, 1)
		s.Require().Equal(http.StatusCreated, st, string(raw))
	}
	for _, sub := range s.catCard(taskID).Subtasks {
		s.False(sub.ServiceIsDeleted, "до удаления признак услуги false")
		for _, p := range sub.Products {
			s.False(p.ProductIsDeleted, "до удаления признак товара false")
		}
	}

	s.catMustDelete("/api/v1/services/" + svcGone)
	s.catMustDelete("/api/v1/products/" + pGone)

	card := s.catCard(taskID)
	s.Require().Len(card.Subtasks, 2, "услуги заказа остались")
	flags := map[string]bool{}
	products := map[string]bool{}
	var goneRow string
	for _, sub := range card.Subtasks {
		flags[sub.ServiceID] = sub.ServiceIsDeleted
		s.Equal("Услуга", sub.ServiceName, "название удалённой услуги по-прежнему видно")
		for _, p := range sub.Products {
			products[p.ProductID] = p.ProductIsDeleted
			if p.ProductID == pGone {
				goneRow = p.ID
			}
		}
	}
	s.Equal(map[string]bool{svcGone: true, svcLive: false}, flags)
	s.Equal(map[string]bool{pGone: true, pLive: false}, products)

	// услуга заказа, удалённая из каталога, правится как обычно.
	st, _, raw := s.doCard(s.admin, http.MethodPut, "/api/v1/subtasks/"+gone, map[string]any{
		"status": "in_progress", "assignee_id": nil, "due_date": nil, "price": 150, "equipment_id": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+goneRow, map[string]any{"qty": 3, "price": 10})
	s.Require().Equal(http.StatusOK, st, "строка удалённого товара правится: %s", raw)
	card = s.catCard(taskID)
	for _, sub := range card.Subtasks {
		if sub.ID == gone {
			s.Equal("in_progress", sub.Status)
			s.True(sub.ServiceIsDeleted)
		}
		for _, p := range sub.Products {
			if p.ID == goneRow {
				s.Equal("3.000", p.Qty.String())
				s.True(p.ProductIsDeleted)
			}
		}
	}
	st, _, raw = s.doCard(s.admin, http.MethodDelete, "/api/v1/subtask-products/"+goneRow, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// ---- TS-030 (услуга), TS-027 негатив ----

// TS-030 + TS-027 негатив (DS-014, abuse case 2): услуга каталога удаляется при наличии услуг
// заказов (200), существующие услуги заказов не меняются; удалённая услуга в новой услуге заказа
// → 400 service_id: not_found; удалённый товар в новой строке услуги заказа → 400 product_id:
// not_found — в обход выпадающего списка. Товары комплекта услуги, удалённые из каталога, в
// заказ не подставляются: в комплекте действующий и удалённый товар → в заказ попал только
// действующий. Архивные по-прежнему дают archived (не «не найдена»).
func (s *OrderCardLiveSuite) TestTS030_DeletedServiceAndProductNotSelectable() {
	m := uuid.NewString()[:8]
	taskID, _ := s.newTask(s.newClient("SD-030 Клиент "+m), s.columnID("Новые"))
	other, _ := s.newTask(s.newClient("SD-030 Второй клиент "+m), s.columnID("Новые"))
	svc := s.newService("100")
	st, raw := s.catAddSubtask(taskID, svc)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	before := s.sdScalar(`SELECT id::text || '|' || status || '|' || price::text FROM subtasks WHERE task_id = $1`, taskID)

	s.catMustDelete("/api/v1/services/" + svc)
	s.Equal(before, s.sdScalar(`SELECT id::text || '|' || status || '|' || price::text FROM subtasks WHERE task_id = $1`, taskID),
		"существующая услуга заказа не изменилась")

	st, raw = s.catAddSubtask(other, svc)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"service_id": "not_found"}, s.fields(raw))
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM subtasks WHERE task_id = $1`, other))

	// комплект: действующий и удалённый товар → в заказ попадает только действующий.
	kitSvc := s.newService("300")
	pLive, pGone := s.newProduct("10", "0", "0"), s.newProduct("20", "0", "0")
	s.addServiceKit(kitSvc, pGone, "1", 1)
	s.addServiceKit(kitSvc, pLive, "2", 2)
	s.catMustDelete("/api/v1/products/" + pGone)
	st, raw = s.catAddSubtask(other, kitSvc)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	card := s.catCard(other)
	s.Require().Len(card.Subtasks, 1)
	s.Require().Len(card.Subtasks[0].Products, 1, "удалённый товар комплекта не подставлен")
	s.Equal(pLive, card.Subtasks[0].Products[0].ProductID)
	s.False(card.Subtasks[0].Products[0].ProductIsDeleted)

	// удалённый товар в новой строке услуги заказа.
	st, raw = s.catAddProduct(card.Subtasks[0].ID, pGone, 1)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"product_id": "not_found"}, s.fields(raw))
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM subtask_products WHERE subtask_id = $1`, card.Subtasks[0].ID))

	// архив — прежний код.
	archSvc, archProduct := s.newService("50"), s.newProduct("5", "0", "0")
	s.setServiceActive(archSvc, false)
	s.setProductActive(archProduct, false)
	st, raw = s.catAddSubtask(other, archSvc)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("archived", s.fields(raw)["service_id"])
	st, raw = s.catAddProduct(card.Subtasks[0].ID, archProduct, 1)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("archived", s.fields(raw)["product_id"])
}

// ---- TS-054 ----

// TS-054 (DS-042, DS-021): товар списан заказом в списывающей колонке до нуля и удалён при
// нулевом остатке; затем заказ уходит из списывающей колонки — остаток возвращается удалённому
// товару. В GET /api/v1/stock — строка товара с is_deleted = true и возвращённым остатком (в
// обоих режимах списка: «рисковые» и «все»); в счётчики /api/v1/stock/summary удалённый товар
// не входит, хотя его остаток ниже минимума. Ручная корректировка до нуля → 200 (в ответе строка
// с is_deleted и нулём), строка исчезает из отчёта, повторная корректировка → 404
// PRODUCT_NOT_FOUND. Негатив: удалённый товар с нулевым остатком в отчёте не показывается;
// добавить его в новый заказ нельзя — 400 product_id: not_found; у действующих товаров
// is_deleted = false.
func (s *OrderCardLiveSuite) TestTS054_StockOnDeletedProduct() {
	m := uuid.NewString()[:8]
	taskID, _, product := s.sdTaskWithProduct(s.newClient("SD-054 Клиент "+m), s.columnID("В работе"), "5", 5)
	s.Require().Equal("0.000", s.defaultStock(product), "товар списан заказом до нуля")
	// минимум выше возвращаемого остатка: вернувшийся остаток 5 < 10 — товар был бы «low».
	s.sdScalar(`UPDATE products SET min_stock = 10 WHERE id = $1 RETURNING '1'`, product)
	liveProduct := s.newProduct("10", "50", "1")

	// товар с остатком не удаляется; списанный до нуля — удаляется (товар в заказе не мешает).
	st, raw := s.catDelete("/api/v1/products/" + liveProduct)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("PRODUCT_HAS_STOCK", s.sdErr(raw).Code)
	before := s.catSummary()
	s.catMustDelete("/api/v1/products/" + product)
	afterDelete := s.catSummary()
	s.Equal(before.ProductsTotal-1, afterDelete.ProductsTotal, "удалённый товар не входит в products_total")
	s.Equal(before.OutCount-1, afterDelete.OutCount, "удалённый товар (остаток 0) ушёл из счётчика «нет в наличии»")

	// негатив: удалённый товар с нулевым остатком в отчёте не показывается.
	for _, q := range []string{"", "&scope=all"} {
		rows := s.catStock(q, product, liveProduct)
		s.NotContains(rows, product, "нулевой остаток — строки нет (%q)", q)
	}
	s.False(s.catStock("&scope=all", liveProduct)[liveProduct].IsDeleted, "у действующего товара is_deleted = false")

	// заказ уходит из списывающей колонки — остаток вернулся удалённому товару.
	s.sdMove(taskID, s.columnID("Новые"))
	s.Require().Equal("5.000", s.defaultStock(product))
	for _, q := range []string{"", "&scope=all"} {
		row, ok := s.catStock(q, product)[product]
		s.Require().True(ok, "удалённый товар с остатком виден в отчёте (%q)", q)
		s.True(row.IsDeleted)
		s.Equal("5.000", row.Stock.String())
	}
	s.Equal(afterDelete, s.catSummary(), "удалённый товар в счётчики не входит — и с остатком ниже минимума")

	// добавить удалённый товар в новый заказ нельзя.
	otherTask, _ := s.newTask(s.newClient("SD-054 Новый заказ "+m), s.columnID("Новые"))
	st, raw = s.catAddProduct(s.addSubtask(otherTask, s.newService("10"), "10"), product, 1)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["product_id"])

	// ручная корректировка: частичная — строка остаётся; до нуля — 200 и строка исчезает.
	st, row, raw := s.catAdjust(product, 2)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(row.IsDeleted)
	s.Equal("2.000", row.Stock.String())
	st, row, raw = s.catAdjust(product, 0)
	s.Require().Equal(http.StatusOK, st, "списание остатка удалённого товара до нуля: %s", raw)
	s.Equal(product, row.ProductID)
	s.True(row.IsDeleted)
	s.Equal("0.000", row.Stock.String())
	s.Equal("0.000", s.defaultStock(product))
	s.Equal("manual:-3.000,manual:-2.000", s.sdScalar(`
		SELECT string_agg(reason || ':' || qty_delta::text, ',' ORDER BY created_at, id) FROM stock_movements
		WHERE product_id = $1 AND reason = 'manual'`, product), "корректировки записаны в журнал склада")
	for _, q := range []string{"", "&scope=all"} {
		s.NotContains(s.catStock(q, product), product, "после списания до нуля строка исчезла (%q)", q)
	}
	st, _, raw = s.catAdjust(product, 4)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(raw).Code)
	s.Equal("0.000", s.defaultStock(product), "остаток удалённого товара с нулём не правится")
}

// DS-042 + DS-021 (ревью части 5): остаток удалённого товара можно только СПИСАТЬ. Нулевой
// остаток → 404 PRODUCT_NOT_FOUND; увеличение и смена знака → 409 PRODUCT_DELETED_WRITE_OFF_ONLY,
// остаток не изменился, движений нет. Уменьшение и списание до нуля → 200; после нуля строка
// исчезает из «Остатков». Отрицательный остаток (долг) списывается к нулю так же: −3 → −1 → 0,
// но не −4 и не +1. Действующий архивный товар в отчёт не входит (as-is).
func (s *OrderCardLiveSuite) TestStock_DeletedProductAdjustOnlyWriteOff() {
	expectStatus := func(productID string, stock any, status int, code, ctx string) {
		st, _, raw := s.catAdjust(productID, stock)
		s.Equal(status, st, "%s: %s", ctx, raw)
		if code != "" {
			s.Equal(code, s.sdErr(raw).Code, ctx)
		}
	}
	movements := func(productID string) string {
		return s.sdScalar(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1 AND reason = 'manual'`, productID)
	}

	zero := s.newProduct("10", "0", "0")
	s.catMustDelete("/api/v1/products/" + zero)
	expectStatus(zero, 3, http.StatusNotFound, "PRODUCT_NOT_FOUND", "нулевой остаток")
	expectStatus(zero, 0, http.StatusNotFound, "PRODUCT_NOT_FOUND", "нулевой остаток, ноль")

	p := s.newProduct("10", "0", "0")
	s.setProductActive(p, false) // удалённый из архива — признак архива отчёту не мешает
	s.catMustDelete("/api/v1/products/" + p)
	s.catSetStock(p, "4") // остаток пришёл после удаления (возврат из заказа)
	rows := s.catStock("&scope=all", p)
	s.Require().Contains(rows, p)
	s.True(rows[p].IsDeleted)

	// увеличение и смена знака — отказ.
	expectStatus(p, 5, http.StatusConflict, "PRODUCT_DELETED_WRITE_OFF_ONLY", "увеличение")
	expectStatus(p, 4.001, http.StatusConflict, "PRODUCT_DELETED_WRITE_OFF_ONLY", "увеличение на тысячную")
	expectStatus(p, -1, http.StatusConflict, "PRODUCT_DELETED_WRITE_OFF_ONLY", "смена знака")
	s.Equal("4.000|0", s.defaultStock(p)+"|"+movements(p), "отказы остаток и журнал не тронули")
	st, _, raw := s.catAdjust(p, 5)
	s.Equal("Товар удалён — его остаток можно только списать", s.sdErr(raw).Message, "%d", st)

	// списание: частично, затем до нуля.
	expectStatus(p, 1.5, http.StatusOK, "", "частичное списание")
	s.Contains(s.catStock("&scope=all", p), p, "остаток ещё не списан — товар в отчёте")
	expectStatus(p, 0, http.StatusOK, "", "списание до нуля")
	s.Equal("0.000|2", s.defaultStock(p)+"|"+movements(p))
	s.NotContains(s.catStock("&scope=all", p), p, "после нуля строка исчезла")
	expectStatus(p, 0, http.StatusNotFound, "PRODUCT_NOT_FOUND", "остаток уже нулевой")

	// долг: отрицательный остаток списывается к нулю, но не увеличивается по модулю.
	debt := s.newProduct("10", "0", "0")
	s.catMustDelete("/api/v1/products/" + debt)
	s.catSetStock(debt, "-3")
	expectStatus(debt, -4, http.StatusConflict, "PRODUCT_DELETED_WRITE_OFF_ONLY", "долг растёт")
	expectStatus(debt, 1, http.StatusConflict, "PRODUCT_DELETED_WRITE_OFF_ONLY", "долг → плюс")
	expectStatus(debt, -1, http.StatusOK, "", "долг уменьшен")
	expectStatus(debt, 0, http.StatusOK, "", "долг списан")
	s.NotContains(s.catStock("&scope=all", debt), debt)

	// действующий архивный товар в отчёт не входит; действующему товару остаток можно увеличивать.
	archivedLive := s.newProduct("10", "3", "0")
	s.setProductActive(archivedLive, false)
	s.NotContains(s.catStock("&scope=all", archivedLive), archivedLive, "действующий архивный товар в отчёт не входит (as-is)")
	live := s.newProduct("10", "1", "0")
	expectStatus(live, 7, http.StatusOK, "", "действующий товар: увеличение")
}

// ---- гонки (§3 «Конкурентность») ----

func (s *OrderCardLiveSuite) catLockWaiters() int {
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
func (s *OrderCardLiveSuite) catRace(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) catRaceResult {
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
// catalog.ProductRepo.SoftDelete (товар FOR NO KEY UPDATE с чтением остатка → отметка),
// транзакция не зафиксирована.
func (s *OrderCardLiveSuite) catHoldProductDelete(productID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE`, productID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE products SET deleted_at = now(), deleted_by = $2, delete_reason = 'Гонка' WHERE id = $1`, productID, s.catAdminID())
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 п. 5 (DS-014): удаление услуги каталога идёт (UPDATE строки не зафиксирован) — в это окно
// приходит добавление этой услуги в заказ. Добавление обязано дождаться удаления (строка услуги
// FOR SHARE) и получить 400 service_id: not_found; услуга заказа не создана. Реализация,
// проверяющая услугу без блокировки, создала бы новую ссылку на удалённую услугу.
func (s *OrderCardLiveSuite) TestRace_SubtaskCreateWaitsForServiceDelete() {
	taskID, _ := s.newTask(s.newClient("SD-race Клиент-услуга "+uuid.NewString()[:8]), s.columnID("Новые"))
	svc := s.newService("100")

	res := s.catRace(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE services SET deleted_at = now(), deleted_by = $2, delete_reason = 'Гонка' WHERE id = $1 AND deleted_at IS NULL`,
			svc, s.catAdminID())
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.catAddSubtask(taskID, svc) })
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["service_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM subtasks WHERE task_id = $1`, taskID), "услуга заказа не создана")
}

// §3 п. 5 (DS-014): удаление товара идёт — в это окно приходит добавление товара в услугу заказа
// в СПИСЫВАЮЩЕЙ колонке. Добавление обязано дождаться удаления (строка товара под блокировкой) и
// получить 400 product_id: not_found: строки нет, остаток не списан. Реализация
// без блокировки добавила бы строку и увела остаток удалённого товара в минус.
func (s *OrderCardLiveSuite) TestRace_SubtaskProductAddWaitsForProductDelete() {
	taskID, _ := s.newTask(s.newClient("SD-race Клиент-товар "+uuid.NewString()[:8]), s.columnID("В работе"))
	subtaskID := s.addSubtask(taskID, s.newService("100"), "100")
	product := s.newProduct("10", "0", "0")

	res := s.catRace(s.catHoldProductDelete(product), func() (int, []byte) { return s.catAddProduct(subtaskID, product, 2) })
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["product_id"])
	s.Equal("0|0.000|0", s.sdScalar(`
		SELECT (SELECT count(*) FROM subtask_products WHERE subtask_id = $1)::text || '|' ||
		       (SELECT stock::text FROM products WHERE id = $2) || '|' ||
		       (SELECT count(*) FROM stock_movements WHERE product_id = $2)::text`, subtaskID, product),
		"строки нет, остаток и журнал не тронуты")
}

// §3 п. 1 (DS-021): товар списывается заказом в незавершённой транзакции (перенос в списывающую
// колонку: строка товара под блокировкой, остаток 3 → 0 ещё не зафиксирован… здесь обратный
// случай — возврат 0 → 3) — в это окно приходит удаление товара. Удаление обязано дождаться
// строки товара и увидеть возвращённый остаток: 409 PRODUCT_HAS_STOCK. Реализация, читающая
// остаток без блокировки, удалила бы товар с остатком 3.
func (s *OrderCardLiveSuite) TestRace_ProductDeleteWaitsForStockReturn() {
	product := s.newProduct("10", "0", "0")
	res := s.catRace(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1 FOR NO KEY UPDATE`, product)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE products SET stock = stock + 3 WHERE id = $1`, product)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.catDelete("/api/v1/products/" + product) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("PRODUCT_HAS_STOCK", s.sdErr(res.raw).Code)
	s.Equal("false", s.sdScalar(`SELECT (deleted_at IS NOT NULL)::text FROM products WHERE id = $1`, product))
}

// §3 п. 1 (DS-042): удаление товара с нулевым остатком идёт — в это окно приходит ручная
// корректировка его остатка из отчёта «Остатки». Корректировка обязана дождаться удаления
// (строка товара под блокировкой) и получить 404 PRODUCT_NOT_FOUND — товар удалён, остаток
// нулевой; остаток не изменился. Реализация без проверки записала бы остаток 5 только что
// удалённому товару в обход запрета DS-021.
func (s *OrderCardLiveSuite) TestRace_StockAdjustWaitsForProductDelete() {
	product := s.newProduct("10", "0", "0")

	res := s.catRace(s.catHoldProductDelete(product), func() (int, []byte) {
		st, _, raw := s.catAdjust(product, 5)
		return st, raw
	})
	s.Require().Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("0.000", s.defaultStock(product))
}

// DS-042 (ревью части 5): остаток удалённого товара списывается до нуля в незавершённой
// транзакции (строка товара под блокировкой, 4 → 0) — в это окно приходит вторая
// корректировка того же товара (4 → 1). Она обязана дождаться блокировки строки и увидеть ноль: 404
// PRODUCT_NOT_FOUND, остаток остался нулевым. Реализация, проверяющая «остаток не ноль» до
// блокировки строки, записала бы остаток 1 товару, у которого остатка уже нет.
func (s *OrderCardLiveSuite) TestRace_DeletedProductAdjustWaitsForWriteOff() {
	product := s.newProduct("10", "0", "0")
	s.catMustDelete("/api/v1/products/" + product)
	s.catSetStock(product, "4")

	res := s.catRace(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1 FOR NO KEY UPDATE`, product)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE products SET stock = 0 WHERE id = $1`, product)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) {
		st, _, raw := s.catAdjust(product, 1)
		return st, raw
	})
	s.Require().Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("0.000", s.defaultStock(product))
}
