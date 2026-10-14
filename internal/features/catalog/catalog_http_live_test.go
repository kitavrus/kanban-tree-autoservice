//go:build integration

package catalog_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// CatalogLiveSuite (§TD-8): категории и товары каталога — CRUD, D16 (409 дубль имени),
// архив, мягкое удаление при ссылках (soft-delete-catalog DS-014), stock_movements manual при ручной правке остатка, права
// (D19), IDOR/битый uuid, мусор → 400, CSRF.
type CatalogLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestCatalogLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка видов каталога (soft-delete-catalog §6.9, DS-027).
	suite.Run(t, &CatalogLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *CatalogLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("catalog4-admin", "password1", "Каталог4 Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("catalog4-admin", "password1")
	loadTestWarehouses(&s.LiveSuite)
}

// testAlmatyID/testMoscowID — склады сида миграции 0008 (warehouses D45) в БД ТЕКУЩЕЙ
// сьюты: SetupSuite каждой сьюты пакета перечитывает их (сьюты идут последовательно).
var testAlmatyID, testMoscowID string

func loadTestWarehouses(ls *testutil.LiveSuite) {
	ls.Require().NoError(ls.Pool.QueryRow(context.Background(), `SELECT id FROM warehouses WHERE name = 'Алматы'`).Scan(&testAlmatyID))
	ls.Require().NoError(ls.Pool.QueryRow(context.Background(), `SELECT id FROM warehouses WHERE name = 'Москва'`).Scan(&testMoscowID))
}

// withStocks — короткие ключи тела товара (stock/min_stock/box/place, как до складов)
// сворачиваются в stocks[] со строкой склада «Алматы» (D46: wire — только stocks[]).
func withStocks(m map[string]any) map[string]any {
	row := map[string]any{}
	for _, k := range []string{"stock", "min_stock", "box", "place"} {
		if v, ok := m[k]; ok {
			row[k] = v
			delete(m, k)
		}
	}
	if len(row) > 0 {
		row["warehouse_id"] = testAlmatyID
		m["stocks"] = []map[string]any{row}
	}
	return m
}

type categoryDTO struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	ItemCount int    `json:"item_count"`
}

// amount — тестовый двойник catalog.Amount: на wire — JSON-число без кавычек
// (CLAUDE.md §Уроки: «NUMERIC в Go — строкой, JSON — число»), здесь достаточно
// json.Number, чтобы не тянуть внутренний тип пакета.
type amount = json.Number

type productDTO struct {
	ID         string            `json:"id"`
	CategoryID string            `json:"category_id"`
	Name       string            `json:"name"`
	SKU        string            `json:"sku"`
	Unit       string            `json:"unit"`
	Price      amount            `json:"price"`
	IsActive   bool              `json:"is_active"`
	Stocks     []productStockDTO `json:"stocks"`
	TotalStock amount            `json:"total_stock"`
}

// productStockDTO — строка блока «Склады» товара (warehouses DS-002).
type productStockDTO struct {
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	IsActive      bool   `json:"is_active"`
	Tracked       bool   `json:"tracked"`
	Stock         amount `json:"stock"`
	MinStock      amount `json:"min_stock"`
	Box           string `json:"box"`
	Place         string `json:"place"`
}

// at — строка склада warehouseID (пустая, если склада нет в ответе).
func (p productDTO) at(warehouseID string) productStockDTO {
	for _, st := range p.Stocks {
		if st.WarehouseID == warehouseID {
			return st
		}
	}
	return productStockDTO{}
}

// delReason — тело DELETE мягкого удаления (soft-delete-catalog DS-020, DS-028).
var delReason = map[string]any{"reason": "Создан по ошибке"}

type apiErr struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

func (s *CatalogLiveSuite) errOf(body []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *CatalogLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *CatalogLiveSuite) newCategory(b *testutil.Browser, kind, name string) categoryDTO {
	st, raw := b.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": kind, "name": name})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var c categoryDTO
	s.Require().NoError(json.Unmarshal(raw, &c))
	return c
}

func productBody(categoryID string, patch map[string]any) map[string]any {
	m := map[string]any{
		"category_id": categoryID, "name": "Фильтр масляный", "sku": "F-001", "unit": "шт",
		"price": 199.9, "stock": 5, "min_stock": 2, "box": "К-07", "place": "Стеллаж A",
	}
	for k, v := range patch {
		m[k] = v
	}
	return withStocks(m)
}

func (s *CatalogLiveSuite) newProduct(b *testutil.Browser, categoryID string, patch map[string]any) productDTO {
	st, raw := b.Do(http.MethodPost, "/api/v1/products", productBody(categoryID, patch))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var p productDTO
	s.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

func productPutBody(p productDTO, patch map[string]any) map[string]any {
	m := map[string]any{
		"category_id": p.CategoryID, "name": p.Name, "sku": p.SKU, "unit": p.Unit,
		"price": p.Price, "is_active": p.IsActive,
	}
	alm := p.at(testAlmatyID)
	m["stock"], m["min_stock"], m["box"], m["place"] = alm.Stock, alm.MinStock, alm.Box, alm.Place
	for k, v := range patch {
		m[k] = v
	}
	return withStocks(m)
}

// ---- happy path: категории + товары CRUD ----

func (s *CatalogLiveSuite) TestCategoryProductCreateGetList() {
	cat := s.newCategory(s.admin, "product", "  Расходники  ")
	s.Equal("Расходники", cat.Name)
	s.True(cat.IsActive)
	s.Equal(0, cat.ItemCount)

	p := s.newProduct(s.admin, cat.ID, nil)
	s.Equal("Фильтр масляный", p.Name)
	s.Equal("199.90", p.Price.String())
	s.True(p.IsActive)

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/products/"+p.ID, nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), `"sku":"F-001"`)

	// category_id сужает поиск — БД общая на весь suite, другие тесты тоже заводят товары
	// с дефолтным именем "Фильтр масляный" (productBody).
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?category_id="+cat.ID+"&q=фильтр", nil)
	s.Equal(http.StatusOK, st)
	var list struct {
		Items  []productDTO `json:"items"`
		Total  int          `json:"total"`
		Limit  int          `json:"limit"`
		Offset int          `json:"offset"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(1, list.Total)
	s.Equal(50, list.Limit, "лимит по умолчанию для списка товаров — 50 (task.md §API)")

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+cat.ID, nil)
	s.Equal(http.StatusOK, st)
	var got categoryDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(1, got.ItemCount, "категория теперь используется одним товаром")
}

// ---- D16: дубль имени категории без учёта регистра/пробелов → 409 ----

func (s *CatalogLiveSuite) TestCategory_DuplicateName409() {
	s.newCategory(s.admin, "product", "Крепёж")
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": "product", "name": "  КРЕПЁЖ  "})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CATEGORY_NAME_TAKEN", s.errOf(raw).Code)

	// разные виды (kind) с тем же именем — не конфликт.
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": "service", "name": "Крепёж"})
	s.Equal(http.StatusCreated, st, string(raw))
}

// ---- рename конфликтует с существующей категорией того же вида ----

func (s *CatalogLiveSuite) TestCategory_RenameConflict() {
	a := s.newCategory(s.admin, "product", "Электрика")
	b := s.newCategory(s.admin, "product", "Сантехника")
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+b.ID, map[string]any{"name": "электрика"})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CATEGORY_NAME_TAKEN", s.errOf(raw).Code)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+a.ID, map[string]any{"name": "Электрика 2"})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// ---- архив категории (as-is) и мягкое удаление категории с товаром: отказ CATEGORY_IN_USE
// снят (soft-delete-catalog DS-014) — 200 с телом, товар остаётся действующим ----

func (s *CatalogLiveSuite) TestCategory_ArchiveRestore_UsedDeletes200() {
	used := s.newCategory(s.admin, "product", "Используемая")
	p := s.newProduct(s.admin, used.ID, nil)

	st, raw := s.admin.Do(http.MethodPost, "/catalog/categories/"+used.ID+"/archive", url.Values{"kind": {"product"}})
	_ = raw
	s.Equal(http.StatusSeeOther, st)
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+used.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got categoryDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.False(got.IsActive)

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/catalog-categories/"+used.ID, delReason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"deleted_at"`)
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+used.ID, nil)
	s.Equal(http.StatusNotFound, st, "удалённая категория для обычных адресов не существует")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products/"+p.ID, nil)
	s.Require().Equal(http.StatusOK, st, "товар удалённой категории остаётся действующим: %s", raw)
	s.Contains(string(raw), `"category_is_deleted":true`)

	free := s.newCategory(s.admin, "product", "Свободная")
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/catalog-categories/"+free.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- товар, используемый в комплекте услуги, удаляется мягко: отказ PRODUCT_IN_USE снят
// (soft-delete-catalog DS-014); архив работает как раньше. Комплект (service_products)
// заводится подфазой B — здесь строка пишется напрямую в БД. Остаток товаров нулевой: товар с
// остатком не удаляется (DS-021, отдельные тесты). ----

func (s *CatalogLiveSuite) TestProduct_UsedInKit_ArchiveAndDelete200() {
	cat := s.newCategory(s.admin, "product", "Для комплекта")
	used := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "USED-1", "stock": 0})

	svcCat := s.newCategory(s.admin, "service", "Услуги для теста")
	var serviceID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO services (category_id, name) VALUES ($1::uuid, 'Тестовая услуга') RETURNING id`, svcCat.ID).Scan(&serviceID))
	_, err := s.Pool.Exec(s.T().Context(),
		`INSERT INTO service_products (service_id, product_id, qty) VALUES ($1::uuid, $2::uuid, 1)`, serviceID, used.ID)
	s.Require().NoError(err)

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+used.ID, productPutBody(used, map[string]any{"is_active": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/products/"+used.ID, delReason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var kept int
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT count(*) FROM service_products WHERE product_id = $1::uuid`, used.ID).Scan(&kept))
	s.Equal(1, kept, "строка комплекта остаётся при удалении товара")

	free := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "FREE-1", "stock": 0})
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/products/"+free.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- stock_movements manual: delta = новое-старое, автор = текущий пользователь;
// delta=0 → записи нет. ----

func (s *CatalogLiveSuite) TestProduct_Update_StockMovementManual() {
	cat := s.newCategory(s.admin, "product", "Учёт остатка")
	p := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "STK-1", "stock": 10})

	var adminID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(), `SELECT id FROM users WHERE login = 'catalog4-admin'`).Scan(&adminID))

	// изменение остатка 10 → 7: delta -3.
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, productPutBody(p, map[string]any{"stock": 7}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	var cnt int
	var delta string
	var userID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT count(*) FROM stock_movements WHERE product_id = $1::uuid`, p.ID).Scan(&cnt))
	s.Equal(1, cnt)
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT qty_delta::text, user_id, reason FROM stock_movements WHERE product_id = $1::uuid`, p.ID).Scan(&delta, &userID, new(string)))
	s.Equal("-3.000", delta)
	s.Equal(adminID, userID)

	// повторное сохранение с тем же значением остатка (delta=0) — новой записи нет.
	var updated productDTO
	s.Require().NoError(json.Unmarshal(raw, &updated))
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, productPutBody(updated, nil))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT count(*) FROM stock_movements WHERE product_id = $1::uuid`, p.ID).Scan(&cnt))
	s.Equal(1, cnt, "delta=0 — новой записи stock_movements нет")
}

// ---- мусор в теле/параметрах → 400, не 500 ----

func (s *CatalogLiveSuite) TestValidation_GarbageNot500() {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"kind": "invalid", "name": "required"}, s.fields(raw))

	cat := s.newCategory(s.admin, "product", "Валидация")
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", map[string]any{"category_id": cat.ID})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["name"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody(cat.ID, map[string]any{"price": -5}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["price"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody(cat.ID, map[string]any{"unit": "тонна"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["unit"])

	for _, body := range []string{`{`, `null`, `"str"`, `{"price":"5"}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?limit=-5&offset=-1&category_id=not-a-uuid", nil)
	s.Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody(cat.ID, map[string]any{"name": "Тип\u0000"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["name"])
}

// ---- code review Блокер 1: границы NUMERIC(12,2)/(12,3) → 400 по своему полю, не 500 ----

func (s *CatalogLiveSuite) TestProduct_NumericBoundaries400() {
	cat := s.newCategory(s.admin, "product", "Границы NUMERIC")

	for _, tc := range []struct {
		name    string
		patch   map[string]any
		field   string
		message string
	}{
		{"цена 12345678901 — переполнение целой части", map[string]any{"price": 12345678901}, "price", "too_large"},
		{"цена 1.239 — лишняя точность", map[string]any{"price": 1.239}, "price", "too_precise"},
		{"цена -1 — отрицательная", map[string]any{"price": -1}, "price", "invalid"},
		{"min_stock 0.0001 — лишняя точность", map[string]any{"min_stock": 0.0001}, "stocks[0].min_stock", "too_precise"},
		{"min_stock 1000000000 — переполнение", map[string]any{"min_stock": 1000000000}, "stocks[0].min_stock", "too_large"},
		{"stock 0.0004 — лишняя точность", map[string]any{"stock": 0.0004}, "stocks[0].stock", "too_precise"},
	} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", productBody(cat.ID, tc.patch))
		s.Equal(http.StatusBadRequest, st, "%s → %s", tc.name, raw)
		s.NotEqual(http.StatusInternalServerError, st, "%s → 500 недопустим", tc.name)
		s.Equal(tc.message, s.fields(raw)[tc.field], "%s → %s", tc.name, raw)
	}

	// экспонента — литерал JSON-числа (не строка: map[string]any{"price":"1e10"} после
	// json.Marshal превратился бы в JSON-строку "1e10", которую Amount.UnmarshalJSON
	// отклоняет ДО валидатора как «не число вовсе» generic 400 без fields — сырое тело
	// нужно, чтобы литерал "1e10"/"1e5" дошёл до decimalRe как есть.
	for _, tc := range []struct {
		name  string
		field string
		body  string
	}{
		{"цена 1e10 — экспонента", "price", `{"category_id":%q,"name":"Тип","sku":"","unit":"","price":1e10,"stocks":[]}`},
		{"цена 1e5 — экспонента", "price", `{"category_id":%q,"name":"Тип","sku":"","unit":"","price":1e5,"stocks":[]}`},
		{"stock 1e10 — экспонента", "stocks[0].stock", `{"category_id":%q,"name":"Тип","sku":"","unit":"","price":0,"stocks":[{"warehouse_id":%q,"stock":1e10}]}`},
	} {
		body := fmt.Sprintf(tc.body, cat.ID, testAlmatyID)
		if tc.field == "price" {
			body = fmt.Sprintf(tc.body, cat.ID)
		}
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", tc.name, raw)
		s.NotEqual(http.StatusInternalServerError, st, "%s → 500 недопустим", tc.name)
		s.Equal("invalid_format", s.fields(raw)[tc.field], "%s → %s", tc.name, raw)
	}
}

// ---- переполнение ДЕЛЬТЫ stock_movements (оба конца валидны по отдельности, разность —
// нет): старое -600000000, новое 600000000 → delta 1.2e9 не влезает в NUMERIC(12,3)
// (9 целых цифр) → 400 по полю "stock", не 500. ----

func (s *CatalogLiveSuite) TestProduct_StockDeltaOverflow400() {
	cat := s.newCategory(s.admin, "product", "Переполнение дельты остатка")
	p := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "DELTA-1", "stock": -600000000})

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, productPutBody(p, map[string]any{"stock": 600000000}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.NotEqual(http.StatusInternalServerError, st)
	s.Equal("too_large", s.fields(raw)["stocks[0].stock"])
}

// ---- IDOR/битый uuid → 404 ----

func (s *CatalogLiveSuite) TestBadOrMissingID_404() {
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000", "1'; DROP TABLE products;--"} {
		esc := url.PathEscape(id)
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/products/"+esc, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+esc, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/products/"+esc, delReason)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// ---- невалидная категория (не существует / не тот kind) → 400, не 500/404 ----

func (s *CatalogLiveSuite) TestProduct_InvalidCategory400() {
	svcCat := s.newCategory(s.admin, "service", "Услуги, не товары")
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", productBody(svcCat.ID, nil))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["category_id"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody("00000000-0000-0000-0000-000000000000", nil))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["category_id"])
}

// ---- D19: исполнитель читает только активные, мутации 403; аноним 401 ----

func (s *CatalogLiveSuite) TestAuthz_ExecutorReadOnlyActive_MutationsForbidden_AnonUnauthorized() {
	cat := s.newCategory(s.admin, "product", "Права")
	active := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "ACT-1"})
	archived := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "ARC-1"})
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+archived.ID, productPutBody(archived, map[string]any{"is_active": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	s.SeedUser("catalog4-exec", "password1", "Каталог4 Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("catalog4-exec", "password1")

	st, raw = exec.Do(http.MethodGet, "/api/v1/products?include_archived=true", nil)
	s.Equal(http.StatusOK, st, "чтение активных товаров разрешено исполнителю")
	s.NotContains(string(raw), "ARC-1", "исполнитель не видит архив даже с include_archived=true (D19)")
	s.Contains(string(raw), "ACT-1")

	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/products"},
		{http.MethodPut, "/api/v1/products/" + active.ID},
		{http.MethodDelete, "/api/v1/products/" + active.ID},
		{http.MethodPost, "/api/v1/catalog-categories"},
		{http.MethodDelete, "/api/v1/catalog-categories/" + cat.ID},
	} {
		st, raw := exec.Do(c.method, c.path, map[string]any{"name": "x"})
		s.Equal(http.StatusForbidden, st, "%s %s", c.method, c.path)
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}

	st, page := exec.Do(http.MethodGet, "/catalog/products", nil)
	s.Equal(http.StatusForbidden, st)
	s.Contains(string(page), "Нет доступа")

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/products", nil)
	s.Equal(http.StatusUnauthorized, st)
	st, _ = anon.Do(http.MethodGet, "/catalog/products", nil)
	s.Equal(http.StatusFound, st, "аноним — редирект на /login")
}

// ---- CSRF: мутация без корректного токена → 403 ----

func (s *CatalogLiveSuite) TestCSRF_MissingToken403() {
	cat := s.newCategory(s.admin, "product", "CSRF")
	resp := s.admin.Request(http.MethodPost, "/api/v1/products", productBody(cat.ID, nil), map[string]string{"X-Csrf-Token": ""})
	defer resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)
}

// ---- SSR: /catalog → 303 /catalog/services; вкладки; создание товара через форму ----

func (s *CatalogLiveSuite) TestSSR_RootRedirect_ProductFormFlow() {
	resp := s.admin.Request(http.MethodGet, "/catalog", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/services", resp.Header.Get("Location"))

	// подфаза B заменила заглушку реальным списком услуг — другие тесты сьюты могли уже
	// завести услугу (TestProduct_UsedInKit_ArchiveAndDelete200 пишет строку в БД
	// напрямую), поэтому проверяем что-то присутствующее в обоих состояниях, а не
	// services-empty безусловно.
	st, page := s.admin.Do(http.MethodGet, "/catalog/services", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="catalog-tabs"`)
	s.Contains(string(page), `data-testid="services-filters"`)

	st, page = s.admin.Do(http.MethodGet, "/catalog/products", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="catalog-tab-products"`)

	cat := s.newCategory(s.admin, "product", "SSR категория")
	form := url.Values{"category_id": {cat.ID}, "name": {"SSR Товар"}, "sku": {"SSR-1"}, "unit": {"шт"}, "price": {"10"},
		"wh_id": {testAlmatyID}, "wh_name": {"Алматы"}, "wh_stock": {"1"}, "wh_min_stock": {"0"}, "wh_box": {""}, "wh_place": {""}}
	resp = s.admin.Request(http.MethodPost, "/catalog/products", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/products?flash=created", resp.Header.Get("Location"))
}

// ---- warehouses (DS-002/DS-003, TS-003): блок «Склады» товара ----

// TS-003 через карточку товара: правка остатка на складе «Москва» → строка product_stock +
// movement manual с warehouse_id; «Алматы» не меняется; «Итого» — сумма по складам.
func (s *CatalogLiveSuite) TestWarehouses_ProductStocksByWarehouse() {
	cat := s.newCategory(s.admin, "product", "Склады товара")
	p := s.newProduct(s.admin, cat.ID, map[string]any{"sku": "WH-1", "stock": 10, "min_stock": 2})

	s.Require().GreaterOrEqual(len(p.Stocks), 2, "блок «Склады» — все активные склады")
	s.Equal("Алматы", p.Stocks[0].WarehouseName)
	s.Equal("10.000", p.at(testAlmatyID).Stock.String())
	s.True(p.at(testAlmatyID).Tracked)
	s.Equal("0", p.at(testMoscowID).Stock.String(), "нет строки — 0")
	s.False(p.at(testMoscowID).Tracked, "пустая строка Москвы не создаётся (D48)")
	s.Equal("10.000", p.TotalStock.String())

	body := productPutBody(p, nil)
	body["stocks"] = []map[string]any{{"warehouse_id": testMoscowID, "stock": 4, "min_stock": 1, "box": "М-1", "place": "Москва-полка"}}
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, body)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var upd productDTO
	s.Require().NoError(json.Unmarshal(raw, &upd))
	s.Equal("4.000", upd.at(testMoscowID).Stock.String())
	s.Equal("М-1", upd.at(testMoscowID).Box)
	s.Equal("10.000", upd.at(testAlmatyID).Stock.String(), "Алматы не тронута")
	s.Equal("14.000", upd.TotalStock.String())

	var wh, delta, reason string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT warehouse_id::text, qty_delta::text, reason FROM stock_movements WHERE product_id = $1::uuid`, p.ID).Scan(&wh, &delta, &reason))
	s.Equal(testMoscowID, wh)
	s.Equal("4.000", delta)
	s.Equal("manual", reason)

	// поиск по коробке/месту на любом складе.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?q="+url.QueryEscape("Москва-полка"), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), p.ID)

	// несуществующий склад / дубль склада / мусор → 400 по полю строки.
	for _, tc := range []struct {
		stocks []map[string]any
		field  string
	}{
		{[]map[string]any{{"warehouse_id": "00000000-0000-0000-0000-000000000000", "stock": 1}}, "stocks[0].warehouse_id"},
		{[]map[string]any{{"warehouse_id": testMoscowID, "stock": 1}, {"warehouse_id": testMoscowID, "stock": 2}}, "stocks[1].warehouse_id"},
		{[]map[string]any{{"warehouse_id": "msk", "stock": 1}}, "stocks[0].warehouse_id"},
		{[]map[string]any{{"warehouse_id": testMoscowID, "min_stock": -1}}, "stocks[0].min_stock"},
	} {
		b := productPutBody(upd, nil)
		b["stocks"] = tc.stocks
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, b)
		s.Equal(http.StatusBadRequest, st, "%v → %s", tc.stocks, raw)
		s.Contains(s.fields(raw), tc.field, string(raw))
	}

	// SSR-форма товара — строка на каждый активный склад.
	st, page := s.admin.Do(http.MethodGet, "/catalog/products/"+p.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="product-stocks"`)
	s.Contains(string(page), `data-warehouse="Алматы"`)
	s.Contains(string(page), `data-warehouse="Москва"`)
}
