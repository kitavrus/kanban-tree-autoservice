//go:build integration

package catalog_test

import (
	"encoding/json"
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	SKU      string `json:"sku"`
	Unit     string `json:"unit"`
	Price    amount `json:"price"`
	IsActive bool   `json:"is_active"`
	Stock    amount `json:"stock"`
	MinStock amount `json:"min_stock"`
	Box      string `json:"box"`
	Place    string `json:"place"`
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

// newService — услуга в категории напрямую SQL (категории есть только у услуг).
func (s *CatalogLiveSuite) newService(categoryID, name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO services (category_id, name) VALUES ($1::uuid, $2) RETURNING id`, categoryID, name).Scan(&id))
	return id
}

func productBody(patch map[string]any) map[string]any {
	m := map[string]any{
		"name": "Фильтр масляный", "sku": "F-001", "unit": "шт",
		"price": 199.9, "stock": 5, "min_stock": 2, "box": "К-07", "place": "Стеллаж A",
	}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *CatalogLiveSuite) newProduct(b *testutil.Browser, patch map[string]any) productDTO {
	st, raw := b.Do(http.MethodPost, "/api/v1/products", productBody(patch))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var p productDTO
	s.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

func productPutBody(p productDTO, patch map[string]any) map[string]any {
	m := map[string]any{
		"name": p.Name, "sku": p.SKU, "unit": p.Unit,
		"price": p.Price, "stock": p.Stock, "min_stock": p.MinStock, "box": p.Box, "place": p.Place,
		"is_active": p.IsActive,
	}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

// ---- happy path: категории + товары CRUD ----

func (s *CatalogLiveSuite) TestCategoryProductCreateGetList() {
	cat := s.newCategory(s.admin, "service", "  Расходники  ")
	s.Equal("Расходники", cat.Name)
	s.Equal("service", cat.Kind)
	s.True(cat.IsActive)
	s.Equal(0, cat.ItemCount)

	p := s.newProduct(s.admin, map[string]any{"name": "Фильтр масляный CPG-1"})
	s.Equal("Фильтр масляный CPG-1", p.Name)
	s.Equal("199.90", p.Price.String())
	s.True(p.IsActive)

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/products/"+p.ID, nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), `"sku":"F-001"`)

	// уникальное имя сужает поиск — БД общая на весь suite, другие тесты тоже заводят товары
	// с дефолтным именем "Фильтр масляный" (productBody).
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?q="+url.QueryEscape("cpg-1"), nil)
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

	s.newService(cat.ID, "Услуга CPG-1")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+cat.ID, nil)
	s.Equal(http.StatusOK, st)
	var got categoryDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(1, got.ItemCount, "категория теперь используется одной услугой")
}

// ---- D16: дубль имени категории без учёта регистра/пробелов → 409 ----

func (s *CatalogLiveSuite) TestCategory_DuplicateName409() {
	s.newCategory(s.admin, "service", "Крепёж")
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": "service", "name": "  КРЕПЁЖ  "})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CATEGORY_NAME_TAKEN", s.errOf(raw).Code)

	// категорий товаров больше нет: kind=product → 400 kind invalid.
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": "product", "name": "Крепёж товары"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["kind"])
}

// ---- рename конфликтует с существующей категорией того же вида ----

func (s *CatalogLiveSuite) TestCategory_RenameConflict() {
	a := s.newCategory(s.admin, "service", "Электрика")
	b := s.newCategory(s.admin, "service", "Сантехника")
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+b.ID, map[string]any{"name": "электрика"})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CATEGORY_NAME_TAKEN", s.errOf(raw).Code)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+a.ID, map[string]any{"name": "Электрика 2"})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// ---- архив категории (as-is) и мягкое удаление категории с услугой: отказ CATEGORY_IN_USE
// снят (soft-delete-catalog DS-014) — 200 с телом, услуга остаётся действующей ----

func (s *CatalogLiveSuite) TestCategory_ArchiveRestore_UsedDeletes200() {
	used := s.newCategory(s.admin, "service", "Используемая")
	svcID := s.newService(used.ID, "Услуга используемой категории")

	st, raw := s.admin.Do(http.MethodPost, "/catalog/categories/"+used.ID+"/archive", url.Values{"kind": {"service"}})
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
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/services/"+svcID, nil)
	s.Require().Equal(http.StatusOK, st, "услуга удалённой категории остаётся действующей: %s", raw)
	s.Contains(string(raw), `"category_is_deleted":true`)

	free := s.newCategory(s.admin, "service", "Свободная")
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/catalog-categories/"+free.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- товар, используемый в комплекте услуги, удаляется мягко: отказ PRODUCT_IN_USE снят
// (soft-delete-catalog DS-014); архив работает как раньше. Комплект (service_products)
// заводится подфазой B — здесь строка пишется напрямую в БД. Остаток товаров нулевой: товар с
// остатком не удаляется (DS-021, отдельные тесты). ----

func (s *CatalogLiveSuite) TestProduct_UsedInKit_ArchiveAndDelete200() {
	used := s.newProduct(s.admin, map[string]any{"sku": "USED-1", "stock": 0})

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

	free := s.newProduct(s.admin, map[string]any{"sku": "FREE-1", "stock": 0})
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/products/"+free.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- stock_movements manual: delta = новое-старое, автор = текущий пользователь;
// delta=0 → записи нет. ----

func (s *CatalogLiveSuite) TestProduct_Update_StockMovementManual() {
	p := s.newProduct(s.admin, map[string]any{"sku": "STK-1", "stock": 10})

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

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", map[string]any{"sku": "VAL-0"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["name"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody(map[string]any{"price": -5}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["price"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody(map[string]any{"unit": "тонна"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["unit"])

	for _, body := range []string{`{`, `null`, `"str"`, `{"price":"5"}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?limit=-5&offset=-1", nil)
	s.Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", productBody(map[string]any{"name": "Тип\u0000"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["name"])
}

// ---- code review Блокер 1: границы NUMERIC(12,2)/(12,3) → 400 по своему полю, не 500 ----

func (s *CatalogLiveSuite) TestProduct_NumericBoundaries400() {
	for _, tc := range []struct {
		name    string
		patch   map[string]any
		field   string
		message string
	}{
		{"цена 12345678901 — переполнение целой части", map[string]any{"price": 12345678901}, "price", "too_large"},
		{"цена 1.239 — лишняя точность", map[string]any{"price": 1.239}, "price", "too_precise"},
		{"цена -1 — отрицательная", map[string]any{"price": -1}, "price", "invalid"},
		{"min_stock 0.0001 — лишняя точность", map[string]any{"min_stock": 0.0001}, "min_stock", "too_precise"},
		{"min_stock 1000000000 — переполнение", map[string]any{"min_stock": 1000000000}, "min_stock", "too_large"},
		{"stock 0.0004 — лишняя точность", map[string]any{"stock": 0.0004}, "stock", "too_precise"},
	} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", productBody(tc.patch))
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
		{"цена 1e10 — экспонента", "price", `{"name":"Тип","sku":"","unit":"","price":1e10,"stock":0,"min_stock":0,"box":"","place":""}`},
		{"цена 1e5 — экспонента", "price", `{"name":"Тип","sku":"","unit":"","price":1e5,"stock":0,"min_stock":0,"box":"","place":""}`},
		{"stock 1e10 — экспонента", "stock", `{"name":"Тип","sku":"","unit":"","price":0,"stock":1e10,"min_stock":0,"box":"","place":""}`},
	} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", tc.body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", tc.name, raw)
		s.NotEqual(http.StatusInternalServerError, st, "%s → 500 недопустим", tc.name)
		s.Equal("invalid_format", s.fields(raw)[tc.field], "%s → %s", tc.name, raw)
	}
}

// ---- переполнение ДЕЛЬТЫ stock_movements (оба конца валидны по отдельности, разность —
// нет): старое -600000000, новое 600000000 → delta 1.2e9 не влезает в NUMERIC(12,3)
// (9 целых цифр) → 400 по полю "stock", не 500. ----

func (s *CatalogLiveSuite) TestProduct_StockDeltaOverflow400() {
	p := s.newProduct(s.admin, map[string]any{"sku": "DELTA-1", "stock": -600000000})

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, productPutBody(p, map[string]any{"stock": 600000000}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.NotEqual(http.StatusInternalServerError, st)
	s.Equal("too_large", s.fields(raw)["stock"])
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

// ---- D19: исполнитель читает только активные, мутации 403; аноним 401 ----

func (s *CatalogLiveSuite) TestAuthz_ExecutorReadOnlyActive_MutationsForbidden_AnonUnauthorized() {
	cat := s.newCategory(s.admin, "service", "Права")
	active := s.newProduct(s.admin, map[string]any{"sku": "ACT-1"})
	archived := s.newProduct(s.admin, map[string]any{"sku": "ARC-1"})
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
	resp := s.admin.Request(http.MethodPost, "/api/v1/products", productBody(nil), map[string]string{"X-Csrf-Token": ""})
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

	form := url.Values{"name": {"SSR Товар"}, "sku": {"SSR-1"}, "unit": {"шт"}, "price": {"10"},
		"stock": {"1"}, "min_stock": {"0"}, "box": {"SSR-К"}, "place": {"SSR-полка"}}
	resp = s.admin.Request(http.MethodPost, "/catalog/products", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/products?flash=created", resp.Header.Get("Location"))

	var stock, box, place string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT stock::text, box, place FROM products WHERE sku = 'SSR-1'`).Scan(&stock, &box, &place))
	s.Equal("1.000", stock)
	s.Equal("SSR-К", box)
	s.Equal("SSR-полка", place)
}

// ---- единый остаток товара (склады убраны): плоские поля stock/min_stock/box/place,
// создание не пишет движение, поиск q по коробке/месту, отрицательный остаток допустим,
// min_stock < 0 и старый формат stocks[] → 400; SSR-форма — одиночные поля. ----

func (s *CatalogLiveSuite) TestProduct_SingleStock() {
	p := s.newProduct(s.admin, map[string]any{"sku": "ONE-1", "stock": 10, "min_stock": 2, "box": "Б-9", "place": "Полка-единая"})
	s.Equal("10.000", p.Stock.String())
	s.Equal("2.000", p.MinStock.String())
	s.Equal("Б-9", p.Box)
	s.Equal("Полка-единая", p.Place)

	var cnt int
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT count(*) FROM stock_movements WHERE product_id = $1::uuid`, p.ID).Scan(&cnt))
	s.Equal(0, cnt, "создание товара движение не пишет")

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/products/"+p.ID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var resp map[string]any
	s.Require().NoError(json.Unmarshal(raw, &resp))
	for _, k := range []string{"stock", "min_stock", "box", "place", "created_at", "updated_at"} {
		s.Contains(resp, k)
	}
	s.NotContains(resp, "category_id", "у товара больше нет категории")
	s.NotContains(resp, "category_is_deleted")
	s.NotContains(resp, "stocks")
	s.NotContains(resp, "total_stock")

	for _, q := range []string{"Б-9", "Полка-единая"} {
		st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?q="+url.QueryEscape(q), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Contains(string(raw), p.ID, "поиск q=%s", q)
	}

	// отрицательный остаток допустим (уходит в минус при списании).
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, productPutBody(p, map[string]any{"stock": -3}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	var upd productDTO
	s.Require().NoError(json.Unmarshal(raw, &upd))
	s.Equal("-3.000", upd.Stock.String())
	var delta, reason string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT qty_delta::text, reason FROM stock_movements WHERE product_id = $1::uuid`, p.ID).Scan(&delta, &reason))
	s.Equal("-13.000", delta)
	s.Equal("manual", reason)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, productPutBody(upd, map[string]any{"min_stock": -1}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Contains(s.fields(raw), "min_stock")

	// старый формат stocks[] — неизвестное поле, строгий JSON → 400.
	body := productBody(map[string]any{"sku": "ONE-2"})
	body["stocks"] = []map[string]any{{"stock": 1}}
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", body)
	s.Equal(http.StatusBadRequest, st, string(raw))

	// SSR-форма товара — одиночные поля остатка, без таблицы складов.
	st, page := s.admin.Do(http.MethodGet, "/catalog/products/"+p.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	for _, tid := range []string{"product-stock", "product-min-stock", "product-box", "product-place"} {
		s.Contains(string(page), `data-testid="`+tid+`"`)
	}
	s.Contains(string(page), `name="stock"`)
	s.NotContains(string(page), `data-testid="product-stocks"`)
	s.NotContains(string(page), `name="wh_`)
}
