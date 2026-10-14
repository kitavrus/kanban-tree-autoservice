//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// StockLiveSuite (§TD-8): экран «Остатки» — список/scope/KPI, ручная правка остатка
// (журнал движений), SSR-каркас `/` и `/stock`, бейдж меню, права executor/admin.
// Отдельная сьюта (свой бинарь+БД) от BoardLiveSuite — фаза 04, design-go-layers.md
// §Package Layout: stock_http_live_test.go.
type StockLiveSuite struct {
	testutil.LiveSuite
	admin  *testutil.Browser
	almaty string // склад «Алматы» (сид миграции 0008, position 1 — склад по умолчанию)
	moscow string // склад «Москва» (position 2)
}

func TestStockLiveSuite(t *testing.T) {
	suite.Run(t, &StockLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *StockLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("stock-admin", "password1", "Остатки Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("stock-admin", "password1")
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Алматы'`).Scan(&s.almaty))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Москва'`).Scan(&s.moscow))
}

// adj — тело PUT /api/v1/stock/:id на склад «Алматы» (warehouse_id обязателен, DS-003).
func (s *StockLiveSuite) adj(stock any) map[string]any {
	return map[string]any{"stock": stock, "warehouse_id": s.almaty}
}

// ---- DTO wire-типы (1:1 с internal/features/board/stock_dto.go, §TD-6.2) ----

type stockRowDTO struct {
	ProductID string      `json:"product_id"`
	Name      string      `json:"name"`
	Unit      string      `json:"unit"`
	Box       string      `json:"box"`
	Place     string      `json:"place"`
	Stock     json.Number `json:"stock"` // numeric.Amount на wire — JSON-число без кавычек
	MinStock  json.Number `json:"min_stock"`
	Pending   json.Number `json:"pending"`
	Forecast  json.Number `json:"forecast"`
	State     string      `json:"state"`
	InTransit json.Number `json:"in_transit"`
	// ByWarehouse — колонки складов (warehouses DS-004).
	ByWarehouse []struct {
		WarehouseID string      `json:"warehouse_id"`
		Stock       json.Number `json:"stock"`
		MinStock    json.Number `json:"min_stock"`
		Pending     json.Number `json:"pending"`
		State       string      `json:"state"`
	} `json:"by_warehouse"`
}

type stockListDTO struct {
	Warehouses []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"warehouses"`
	Items  []stockRowDTO `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type stockSummaryDTO struct {
	ProductsTotal int `json:"products_total"`
	LowCount      int `json:"low_count"`
	OutCount      int `json:"out_count"`
	WillLowCount  int `json:"will_low_count"`
	RiskCount     int `json:"risk_count"`
}

// ---- фикстуры (категория+товар напрямую SQL: вне API этой фазы) ----

func (s *StockLiveSuite) createProduct(stock, minStock string) string {
	ctx := context.Background()
	var categoryID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'StockLive ' || gen_random_uuid()) RETURNING id`).Scan(&categoryID))
	var productID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`WITH p AS (INSERT INTO products (category_id, name) VALUES ($1, 'Товар остатков') RETURNING id),
		      ps AS (INSERT INTO product_stock (product_id, warehouse_id, stock, min_stock) SELECT id, default_warehouse_id(), $2::numeric, $3::numeric FROM p)
		 SELECT id FROM p`,
		categoryID, stock, minStock).Scan(&productID))
	return productID
}

func (s *StockLiveSuite) productStock(id string) string {
	var stock string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT coalesce((SELECT stock FROM product_stock WHERE product_id = $1 AND warehouse_id = default_warehouse_id()), 0)::text`, id).Scan(&stock))
	return stock
}

// ---- HTTP-хелперы ----

func (s *StockLiveSuite) list(b *testutil.Browser, query string) (int, stockListDTO, []byte) {
	st, raw := b.Do(http.MethodGet, "/api/v1/stock"+query, nil)
	var l stockListDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	}
	return st, l, raw
}

func (s *StockLiveSuite) summary(b *testutil.Browser) (int, stockSummaryDTO, []byte) {
	st, raw := b.Do(http.MethodGet, "/api/v1/stock/summary", nil)
	var sm stockSummaryDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &sm), string(raw))
	}
	return st, sm, raw
}

func (s *StockLiveSuite) errCode(body []byte) string {
	var e struct {
		Code string `json:"code"`
	}
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e.Code
}

// ---- LV-012/AC-015: scope и KPI ----

func (s *StockLiveSuite) TestStockLiveSuite_List_ScopeAndKPI() {
	ok := s.createProduct("10", "1") // stock > min_stock → ok
	low := s.createProduct("1", "5") // 0 < stock <= min_stock → low
	out := s.createProduct("0", "5") // stock <= 0 → out

	st, all, raw := s.list(s.admin, "?scope=all&limit=1000")
	s.Require().Equal(http.StatusOK, st, string(raw))
	byID := map[string]stockRowDTO{}
	for _, it := range all.Items {
		byID[it.ProductID] = it
	}
	s.Require().Contains(byID, ok)
	s.Equal("ok", byID[ok].State)
	s.Require().Contains(byID, low)
	s.Equal("low", byID[low].State)
	s.Require().Contains(byID, out)
	s.Equal("out", byID[out].State)

	st, risky, raw := s.list(s.admin, "?scope=risky&limit=1000")
	s.Require().Equal(http.StatusOK, st, string(raw))
	riskyIDs := map[string]bool{}
	for _, it := range risky.Items {
		riskyIDs[it.ProductID] = true
		s.NotEqual("ok", it.State, "scope=risky не должен включать ok-товары")
	}
	s.True(riskyIDs[low])
	s.True(riskyIDs[out])
	s.False(riskyIDs[ok], "ok-товар не входит в risky")

	// дефолт scope (без параметра) — тот же risky.
	st, def, raw := s.list(s.admin, "?limit=1000")
	s.Require().Equal(http.StatusOK, st, string(raw))
	for _, it := range def.Items {
		s.NotEqual("ok", it.State)
	}

	stSum, sm, rawSum := s.summary(s.admin)
	s.Require().Equal(http.StatusOK, stSum, string(rawSum))
	s.GreaterOrEqual(sm.RiskCount, 2)
	s.GreaterOrEqual(sm.LowCount, 1)
	s.GreaterOrEqual(sm.OutCount, 1)
	s.Equal(sm.LowCount+sm.OutCount+sm.WillLowCount, sm.RiskCount)
}

// ---- LV-013/D44: журнал движений при ручной правке ----

func (s *StockLiveSuite) TestStockLiveSuite_Adjust_LogsManualMovement() {
	productID := s.createProduct("10", "1")

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(25))
	s.Require().Equal(http.StatusOK, st, string(raw))
	var row stockRowDTO
	s.Require().NoError(json.Unmarshal(raw, &row))
	s.Equal(productID, row.ProductID)
	s.Equal("25.000", row.Stock.String())
	s.Equal("25.000", s.productStock(productID))

	var count int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE product_id = $1`, productID).Scan(&count))
	s.Require().Equal(1, count)
	var delta, reason string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT qty_delta::text, reason FROM stock_movements WHERE product_id = $1`, productID).Scan(&delta, &reason))
	s.Equal("manual", reason)
	s.Equal("15.000", delta)

	// та же величина повторно — дельта 0, новой записи журнала нет.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(25))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE product_id = $1`, productID).Scan(&count))
	s.Equal(1, count, "нулевая дельта не создаёт запись журнала")

	// отрицательный остаток допустим (D44).
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(-3))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("-3.000", s.productStock(productID))
}

// navBadgeRe — число в SSR-бейдже «Остатки» (layout base.html).
var navBadgeRe = regexp.MustCompile(`data-testid="nav-badge-stock"[^>]*>(\d+)<`)

func (s *StockLiveSuite) ssrNavBadge() int {
	resp := s.admin.Request(http.MethodGet, "/", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	m := navBadgeRe.FindSubmatch(body)
	s.Require().NotNil(m, "бейдж «Остатки» в SSR")
	n, err := strconv.Atoi(string(m[1]))
	s.Require().NoError(err)
	return n
}

// SSR-бейдж кэшируется (nav_badge_cache.go), но изменяющий запрос через приложение
// сбрасывает кэш: следующий рендер сразу показывает новое число, не дожидаясь TTL.
func (s *StockLiveSuite) TestStockLiveSuite_SSRNavBadge_InvalidatedByMutation() {
	productID := s.createProduct("10", "1")
	before := s.ssrNavBadge() // кэш прогрет
	_, sm, raw := s.summary(s.admin)
	s.Require().Equal(sm.RiskCount, before, string(raw))

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(0))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(before+1, s.ssrNavBadge(), "после PUT /stock бейдж пересчитан сразу")
}

// ---- LV-014: SSR-каркас доски/остатков + бейдж меню (ADR-004) ----

func (s *StockLiveSuite) TestStockLiveSuite_SSR_BoardAndStockPages200() {
	// Товар уводится в риск через API (PUT /stock): как у пользователя, изменение идёт через
	// приложение и сбрасывает кэш бейджа. Прямая запись в БД мимо приложения видна в SSR
	// только по истечении TTL (nav_badge_cache.go) — её сразу поправляет nav-badge.js.
	productID := s.createProduct("10", "1")
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(0))
	s.Require().Equal(http.StatusOK, st, string(raw))

	_, sm, rawSum := s.summary(s.admin)
	s.Require().GreaterOrEqual(sm.RiskCount, 1, string(rawSum))

	resp := s.admin.Request(http.MethodGet, "/", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Contains(string(body), `data-testid="board-root"`)
	s.Contains(string(body), `data-nav-badge="/stock"`, "пункт «Остатки» несёт бейдж (ADR-004)")
	s.NotContains(string(body), `data-testid="nav-badge-stock" hidden`,
		"risk_count > 0 — бейдж НЕ должен быть hidden")

	resp2 := s.admin.Request(http.MethodGet, "/stock", nil, nil)
	body2, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp2.StatusCode)
	s.Contains(string(body2), `data-testid="stock-root"`)
	s.Contains(string(body2), `<meta name="csrf-token"`)
}

// ---- LV-043/AC-015: executor читает, но не правит остаток ----

func (s *StockLiveSuite) TestStockLiveSuite_Executor_AdjustForbidden_ReadAllowed() {
	s.SeedUser("stock-exec", "password1", "Остатки Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("stock-exec", "password1")

	productID := s.createProduct("10", "1")

	st, raw := exec.Do(http.MethodGet, "/api/v1/stock", nil)
	s.Equal(http.StatusOK, st, string(raw))

	st, raw = exec.Do(http.MethodGet, "/api/v1/stock/summary", nil)
	s.Equal(http.StatusOK, st, string(raw))

	st, raw = exec.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(99))
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("FORBIDDEN", s.errCode(raw))
	s.Equal("10.000", s.productStock(productID), "запрет не должен применить изменение")
}

// ---- Invalid Input (TD-4) ----

func (s *StockLiveSuite) TestStockLiveSuite_QueryValidation400() {
	for _, q := range []string{"?scope=bad", "?limit=abc", "?limit=0", "?limit=1001", "?offset=-1"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock"+q, nil)
		s.Equal(http.StatusBadRequest, st, "%s → %s", q, raw)
	}
}

func (s *StockLiveSuite) TestStockLiveSuite_AdjustValidation400() {
	productID := s.createProduct("10", "1")

	for _, body := range []string{`{}`, `{"stock":null}`, `{"stock":"5"}`, `[]`, `null`} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}

	// больше 3 знаков после запятой — too_precise (NUMERIC(12,3)).
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, s.adj(10.1234))
	s.Equal(http.StatusBadRequest, st, string(raw))
}

func (s *StockLiveSuite) TestStockLiveSuite_NotFound404() {
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+id, s.adj(1))
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

func (s *StockLiveSuite) TestStockLiveSuite_Auth_NoSession401() {
	anon := s.NewBrowser()
	for _, cs := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/stock"},
		{http.MethodGet, "/api/v1/stock/summary"},
	} {
		st, raw := anon.Do(cs.method, cs.path, nil)
		s.Equal(http.StatusUnauthorized, st, "%s %s → %s", cs.method, cs.path, raw)
	}
}

// ---- warehouses: остатки по складам ----

// setPair — строка product_stock пары товар×склад напрямую (фикстура).
func (s *StockLiveSuite) setPair(productID, warehouseID, stock, minStock string) {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO product_stock (product_id, warehouse_id, stock, min_stock) VALUES ($1, $2, $3::numeric, $4::numeric)
		ON CONFLICT (product_id, warehouse_id) DO UPDATE SET stock = EXCLUDED.stock, min_stock = EXCLUDED.min_stock`,
		productID, warehouseID, stock, minStock)
	s.Require().NoError(err)
}

func (s *StockLiveSuite) pairStock(productID, warehouseID string) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT coalesce((SELECT stock FROM product_stock WHERE product_id = $1 AND warehouse_id = $2), 0)::text`,
		productID, warehouseID).Scan(&v))
	return v
}

func (s *StockLiveSuite) rowOf(l stockListDTO, productID string) stockRowDTO {
	for _, it := range l.Items {
		if it.ProductID == productID {
			return it
		}
	}
	s.FailNow("товар не найден в списке остатков", productID)
	return stockRowDTO{}
}

// TS-003: ручная правка остатка на складе «Москва» → строка product_stock + movement manual
// с warehouse_id; «Алматы» не меняется.
func (s *StockLiveSuite) TestWarehouses_TS003_ManualAdjustOnMoscow() {
	productID := s.createProduct("10", "1") // Алматы: 10/1

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, map[string]any{"stock": 7, "warehouse_id": s.moscow})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var row stockRowDTO
	s.Require().NoError(json.Unmarshal(raw, &row))
	s.Equal("17.000", row.Stock.String(), "ответ — итог по всем складам")

	s.Equal("7.000", s.pairStock(productID, s.moscow), "строка Москвы создана")
	s.Equal("10.000", s.pairStock(productID, s.almaty), "Алматы не тронута")

	var wh, delta, reason string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT warehouse_id::text, qty_delta::text, reason FROM stock_movements WHERE product_id = $1`, productID).
		Scan(&wh, &delta, &reason))
	s.Equal(s.moscow, wh)
	s.Equal("7.000", delta)
	s.Equal("manual", reason)

	// склад не передан / не uuid / архивный → 400 warehouse_id.
	for _, body := range []map[string]any{{"stock": 1}, {"stock": 1, "warehouse_id": "msk"}, {"stock": 1, "warehouse_id": "00000000-0000-0000-0000-000000000000"}} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, body)
		s.Equal(http.StatusBadRequest, st, "%v → %s", body, raw)
	}
}

// TS-007: экран «Остатки» — фильтр по складу, колонки складов, «Итого», «В пути»;
// «ниже минимума» по паре товар×склад.
func (s *StockLiveSuite) TestWarehouses_TS007_StockScreenByWarehouse() {
	productID := s.createProduct("10", "2")  // Алматы: 10/2 → ok
	s.setPair(productID, s.moscow, "1", "3") // Москва: 1/3 → low

	st, all, raw := s.list(s.admin, "?scope=all&limit=1000")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().GreaterOrEqual(len(all.Warehouses), 2, "колонки — все активные склады")
	s.Equal("Алматы", all.Warehouses[0].Name, "порядок — position")
	row := s.rowOf(all, productID)
	s.Equal("11.000", row.Stock.String(), "Итого — сумма по складам")
	s.Equal("5.000", row.MinStock.String())
	s.Equal("0", row.InTransit.String(), "в подфазе A перемещений нет")
	s.Equal("low", row.State, "товар в риске, если риск хоть на одной паре")
	cells := map[string]string{}
	states := map[string]string{}
	for _, c := range row.ByWarehouse {
		cells[c.WarehouseID] = c.Stock.String()
		states[c.WarehouseID] = c.State
	}
	s.Equal("10.000", cells[s.almaty])
	s.Equal("1.000", cells[s.moscow])
	s.Equal("ok", states[s.almaty])
	s.Equal("low", states[s.moscow])

	// фильтр «Алматы» — товар не рисковый; «Москва» — рисковый.
	st, alm, raw := s.list(s.admin, "?scope=risky&limit=1000&warehouse_id="+s.almaty)
	s.Require().Equal(http.StatusOK, st, string(raw))
	for _, it := range alm.Items {
		s.NotEqual(productID, it.ProductID, "на Алматы товар в норме")
	}
	st, msk, raw := s.list(s.admin, "?scope=risky&limit=1000&warehouse_id="+s.moscow)
	s.Require().Equal(http.StatusOK, st, string(raw))
	mrow := s.rowOf(msk, productID)
	s.Equal("1.000", mrow.Stock.String(), "Итого при фильтре — только выбранный склад")
	s.Len(mrow.ByWarehouse, len(all.Warehouses), "колонки складов — всегда все")

	// KPI по парам: Москва-фильтр видит low-пару, Алматы — нет (для этого товара).
	before := func(wh string) stockSummaryDTO {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock/summary?warehouse_id="+wh, nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var sm stockSummaryDTO
		s.Require().NoError(json.Unmarshal(raw, &sm))
		return sm
	}
	mskBefore, almBefore := before(s.moscow), before(s.almaty)
	s.setPair(productID, s.moscow, "10", "3") // Москва → ok
	s.Equal(mskBefore.LowCount-1, before(s.moscow).LowCount, "low-пара Москвы ушла из KPI")
	s.Equal(almBefore.LowCount, before(s.almaty).LowCount, "KPI Алматы не зависит от Москвы")

	// мусорный склад → 400.
	st, _, raw = s.list(s.admin, "?warehouse_id=msk")
	s.Equal(http.StatusBadRequest, st, string(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/stock/summary?warehouse_id=msk", nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
}
