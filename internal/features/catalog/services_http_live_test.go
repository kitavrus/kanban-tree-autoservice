//go:build integration

package catalog_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ServicesLiveSuite (§TD-8, подфаза B task.md): CRUD услуг, комплект (дубли/qty≤0/
// архивный товар → 400), удаление товара из комплекта (PRODUCT_IN_USE) сквозным путём
// через API услуг, «по факту», права (D19), IDOR, мусор → 400, CSRF, SSR.
type ServicesLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestServicesLiveSuite(t *testing.T) {
	suite.Run(t, &ServicesLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *ServicesLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("catalog4b-admin", "password1", "Каталог4Б Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("catalog4b-admin", "password1")
}

type svcProductDTO struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Price     amount `json:"price"`
	Qty       amount `json:"qty"`
}

type serviceDTO struct {
	ID         string          `json:"id"`
	CategoryID string          `json:"category_id"`
	Name       string          `json:"name"`
	Price      *amount         `json:"price"`
	IsActive   bool            `json:"is_active"`
	Products   []svcProductDTO `json:"products"`
}

// errOf/fields — копии хелперов CatalogLiveSuite (тот же пакет catalog_test, но другой
// приёмник suite — методы не наследуются между независимыми типами suite.Suite).
func (s *ServicesLiveSuite) errOf(body []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *ServicesLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

// newCategory — копия хелпера CatalogLiveSuite (тот же пакет catalog_test, но другой
// приёмник suite — методы не наследуются между независимыми типами suite.Suite).
func (s *ServicesLiveSuite) newCategory(kind, name string) categoryDTO {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": kind, "name": name})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var c categoryDTO
	s.Require().NoError(json.Unmarshal(raw, &c))
	return c
}

func (s *ServicesLiveSuite) newProductB(patch map[string]any) productDTO {
	m := map[string]any{"name": "Товар комплекта", "sku": "CMP-1", "unit": "шт", "price": 100, "stock": 10, "min_stock": 0}
	for k, v := range patch {
		m[k] = v
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", m)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var p productDTO
	s.Require().NoError(json.Unmarshal(raw, &p))
	return p
}

func serviceBody(categoryID string, patch map[string]any) map[string]any {
	m := map[string]any{"category_id": categoryID, "name": "Замена двигателя", "price": 1500}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *ServicesLiveSuite) newService(categoryID string, patch map[string]any) serviceDTO {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(categoryID, patch))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var sv serviceDTO
	s.Require().NoError(json.Unmarshal(raw, &sv))
	return sv
}

// ---- happy path: категория+услуга+комплект CRUD, re-fetch-after-write ----

func (s *ServicesLiveSuite) TestServiceCreateGetList_WithProducts() {
	cat := s.newCategory("service", "Ремонт двигателей")
	p1 := s.newProductB(map[string]any{"sku": "P-1", "name": "Прокладка"})
	p2 := s.newProductB(map[string]any{"sku": "P-2", "name": "Масло"})

	sv := s.newService(cat.ID, map[string]any{
		"products": []map[string]any{{"product_id": p1.ID, "qty": 2}, {"product_id": p2.ID, "qty": 1.5}},
	})
	s.Equal("Замена двигателя", sv.Name)
	s.Require().NotNil(sv.Price)
	s.Equal("1500.00", sv.Price.String())
	s.Require().Len(sv.Products, 2)
	s.Equal(p1.ID, sv.Products[0].ProductID)
	s.Equal("Прокладка", sv.Products[0].Name)
	s.Equal("2.000", sv.Products[0].Qty.String())
	s.Equal(p2.ID, sv.Products[1].ProductID)
	s.Equal("1.500", sv.Products[1].Qty.String())

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/services/"+sv.ID, nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), "Прокладка")

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/services?category_id="+cat.ID, nil)
	s.Equal(http.StatusOK, st)
	var list struct {
		Items  []serviceDTO `json:"items"`
		Total  int          `json:"total"`
		Limit  int          `json:"limit"`
		Offset int          `json:"offset"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(1, list.Total)
	s.Equal(50, list.Limit, "лимит по умолчанию — 50 (task.md §API)")
}

// ---- «по факту» — price отсутствует/null → null в ответе ----

func (s *ServicesLiveSuite) TestService_PriceByFact_NullBothWays() {
	cat := s.newCategory("service", "Диагностика")

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", map[string]any{"category_id": cat.ID, "name": "Диагностика"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var sv serviceDTO
	s.Require().NoError(json.Unmarshal(raw, &sv))
	s.Nil(sv.Price, "price отсутствует в теле — «по факту»")

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{"name": "Диагностика 2", "price": nil}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &sv))
	s.Nil(sv.Price, "price:null — «по факту»")
}

// ---- D10: дубли товара в комплекте объединяются суммой qty ----

func (s *ServicesLiveSuite) TestService_DuplicateProducts_QtySummed() {
	cat := s.newCategory("service", "Комплект с дублями")
	p := s.newProductB(map[string]any{"sku": "DUP-1"})

	sv := s.newService(cat.ID, map[string]any{
		"products": []map[string]any{{"product_id": p.ID, "qty": 1}, {"product_id": p.ID, "qty": 2.5}},
	})
	s.Require().Len(sv.Products, 1, "дубли объединены в одну строку")
	s.Equal("3.500", sv.Products[0].Qty.String())
}

// ---- qty ≤ 0 → 400; архивный/несуществующий товар → 400 по полю строки ----

func (s *ServicesLiveSuite) TestService_Validation_QtyAndProductRow() {
	cat := s.newCategory("service", "Валидация комплекта")
	p := s.newProductB(map[string]any{"sku": "VAL-1"})

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{
		"products": []map[string]any{{"product_id": p.ID, "qty": 0}},
	}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["products[0].qty"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{
		"products": []map[string]any{{"product_id": p.ID, "qty": -1}},
	}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["products[0].qty"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{
		"products": []map[string]any{{"product_id": "00000000-0000-0000-0000-000000000000", "qty": 1}},
	}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["products[0].product_id"], "несуществующий товар")

	// архивируем товар — ссылка на него в новом комплекте тоже 400.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+p.ID, map[string]any{
		"name": "Валидация", "sku": "VAL-1", "unit": "", "price": 0, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{
		"products": []map[string]any{{"product_id": p.ID, "qty": 1}},
	}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["products[0].product_id"], "архивный товар")
}

// ---- code review Медиум#2: товар, УЖЕ бывший в комплекте услуги, архивированный (ровно
// поток CT-005: PRODUCT_IN_USE → архив вместо удаления) — пересохранение услуги с ЭТОЙ ЖЕ
// строкой должно проходить; НОВАЯ строка с архивным товаром (которого раньше не было в
// комплекте) — 400. task.md: «архивный товар не предлагается ПРИ ДОБАВЛЕНИИ», не при
// пересохранении уже существующей. ----

func (s *ServicesLiveSuite) TestService_Update_ArchivedProductInComplect_Grandfathered() {
	svcCat := s.newCategory("service", "Грандфазер архивного товара")
	used := s.newProductB(map[string]any{"sku": "GF-USED"})
	other := s.newProductB(map[string]any{"sku": "GF-OTHER"})

	sv := s.newService(svcCat.ID, map[string]any{
		"products": []map[string]any{{"product_id": used.ID, "qty": 1}},
	})

	// used уходит в архив (ровно тот же поток, что CT-005: удалить нельзя — PRODUCT_IN_USE
	// — архивируем вместо удаления).
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+used.ID, map[string]any{
		"name": "Товар в комплекте", "sku": "GF-USED", "unit": "", "price": 100, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// пересохранение услуги с ТЕМ ЖЕ комплектом (used всё ещё там) — должно пройти, а не
	// упасть 400 на любое сохранение услуги.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/services/"+sv.ID, map[string]any{
		"category_id": svcCat.ID, "name": "Переименованная услуга", "price": 1500, "is_active": true,
		"products": []map[string]any{{"product_id": used.ID, "qty": 2}},
	})
	s.Require().Equal(http.StatusOK, st, string(raw), "уже существующая строка с архивным товаром — не блокер сохранения")
	var updated serviceDTO
	s.Require().NoError(json.Unmarshal(raw, &updated))
	s.Equal("2.000", updated.Products[0].Qty.String(), "qty у грандфазер-строки можно менять")

	// НОВАЯ строка с ДРУГИМ архивным товаром — 400 (его раньше не было в комплекте).
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+other.ID, map[string]any{
		"name": "Другой товар", "sku": "GF-OTHER", "unit": "", "price": 50, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/services/"+sv.ID, map[string]any{
		"category_id": svcCat.ID, "name": "Переименованная услуга", "price": 1500, "is_active": true,
		"products": []map[string]any{{"product_id": used.ID, "qty": 2}, {"product_id": other.ID, "qty": 1}},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["products[1].product_id"], "новая строка с архивным товаром — 400")
}

// ---- code review Блокер 1: границы NUMERIC(12,2)/(12,3) для услуг → 400 по своему полю,
// не 500. ----

func (s *ServicesLiveSuite) TestService_NumericBoundaries400() {
	cat := s.newCategory("service", "Границы NUMERIC услуг")
	p := s.newProductB(map[string]any{"sku": "BOUND-1"})

	for _, tc := range []struct {
		name    string
		patch   map[string]any
		field   string
		message string
	}{
		{"цена 12345678901 — переполнение целой части", map[string]any{"price": 12345678901}, "price", "too_large"},
		{"цена 1.239 — лишняя точность", map[string]any{"price": 1.239}, "price", "too_precise"},
		{"цена -1 — отрицательная", map[string]any{"price": -1}, "price", "invalid"},
	} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, tc.patch))
		s.Equal(http.StatusBadRequest, st, "%s → %s", tc.name, raw)
		s.NotEqual(http.StatusInternalServerError, st, "%s → 500 недопустим", tc.name)
		s.Equal(tc.message, s.fields(raw)[tc.field], "%s → %s", tc.name, raw)
	}

	for _, tc := range []struct {
		name    string
		qty     any
		message string
	}{
		{"qty 0.0001 — лишняя точность", 0.0001, "too_precise"},
		{"qty 1000000000 — переполнение", 1000000000, "too_large"},
	} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{
			"products": []map[string]any{{"product_id": p.ID, "qty": tc.qty}},
		}))
		s.Equal(http.StatusBadRequest, st, "%s → %s", tc.name, raw)
		s.NotEqual(http.StatusInternalServerError, st, "%s → 500 недопустим", tc.name)
		s.Equal(tc.message, s.fields(raw)["products[0].qty"], "%s → %s", tc.name, raw)
	}

	// экспонента — литерал JSON-числа сырым телом (map[string]any{"price":"1e5"}
	// после json.Marshal стал бы JSON-строкой, которую Amount.UnmarshalJSON отклоняет ДО
	// валидатора — generic 400 без fields).
	for _, body := range []string{
		fmt.Sprintf(`{"category_id":%q,"name":"Услуга","price":1e5}`, cat.ID),
		fmt.Sprintf(`{"category_id":%q,"name":"Услуга","price":1e10}`, cat.ID),
	} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", body)
		s.Equal(http.StatusBadRequest, st, string(raw))
		s.NotEqual(http.StatusInternalServerError, st)
		s.Equal("invalid_format", s.fields(raw)["price"], string(raw))
	}
}

// ---- невалидная категория (не существует) → 400 ----

func (s *ServicesLiveSuite) TestService_InvalidCategory400() {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody("00000000-0000-0000-0000-000000000000", nil))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["category_id"])
}

// ---- удаление товара из комплекта услуги сквозным путём через API услуг: отказ
// PRODUCT_IN_USE снят (soft-delete-catalog DS-014) — товар удаляется мягко, строка комплекта
// остаётся с признаком product_is_deleted; архивирование работает как раньше. Остаток товаров
// нулевой (товар с остатком не удаляется, DS-021). ----

func (s *ServicesLiveSuite) TestService_ProductInComplect_ArchiveWorks_DeleteKeepsRow() {
	cat := s.newCategory("service", "Комплект и удаление")
	used := s.newProductB(map[string]any{"sku": "DEL-USED", "stock": 0})
	free := s.newProductB(map[string]any{"sku": "DEL-FREE", "stock": 0})

	sv := s.newService(cat.ID, map[string]any{"products": []map[string]any{{"product_id": used.ID, "qty": 1}}})

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+used.ID, map[string]any{
		"name": "Товар в комплекте", "sku": "DEL-USED", "unit": "", "price": 100, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw), "архивирование работает")

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/products/"+used.ID, delReason)
	s.Require().Equal(http.StatusOK, st, string(raw), "товар в комплекте удаляется — ссылки не мешают")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/services/"+sv.ID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"product_is_deleted":true`, "строка комплекта остаётся с пометкой")

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/products/"+free.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- удаление/архив услуги, is_active обязателен на PUT ----

func (s *ServicesLiveSuite) TestService_Update_ArchiveRestore_Delete() {
	cat := s.newCategory("service", "Жизненный цикл услуги")
	sv := s.newService(cat.ID, nil)

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/services/"+sv.ID, map[string]any{
		"category_id": cat.ID, "name": sv.Name, "price": 1500,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["is_active"])

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/services/"+sv.ID, map[string]any{
		"category_id": cat.ID, "name": sv.Name, "price": 1500, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var updated serviceDTO
	s.Require().NoError(json.Unmarshal(raw, &updated))
	s.False(updated.IsActive)

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/services/"+sv.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/services/"+sv.ID, nil)
	s.Equal(http.StatusNotFound, st, string(raw))
}

// ---- IDOR/битый uuid → 404 ----

func (s *ServicesLiveSuite) TestBadOrMissingServiceID_404() {
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/services/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/services/"+id, delReason)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// ---- мусор в теле → 400, не 500 ----

func (s *ServicesLiveSuite) TestService_Validation_GarbageNot500() {
	cat := s.newCategory("service", "Мусор в запросе")
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", map[string]any{"category_id": cat.ID})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["name"])

	for _, body := range []string{`{`, `null`, `"str"`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/services", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{"name": "Услуга\u0000"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["name"])
}

// ---- D19: исполнитель читает только активные, мутации 403; аноним 401 ----

func (s *ServicesLiveSuite) TestAuthz_ExecutorReadOnlyActive_MutationsForbidden() {
	cat := s.newCategory("service", "Права услуг")
	active := s.newService(cat.ID, map[string]any{"name": "Активная услуга"})
	archived := s.newService(cat.ID, map[string]any{"name": "Архивная услуга"})
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/services/"+archived.ID, map[string]any{
		"category_id": cat.ID, "name": archived.Name, "price": 1500, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	s.SeedUser("catalog4b-exec", "password1", "Каталог4Б Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("catalog4b-exec", "password1")

	st, raw = exec.Do(http.MethodGet, "/api/v1/services?include_archived=true", nil)
	s.Equal(http.StatusOK, st)
	s.NotContains(string(raw), "Архивная услуга", "исполнитель не видит архив даже с include_archived=true (D19)")
	s.Contains(string(raw), "Активная услуга")

	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/services"},
		{http.MethodPut, "/api/v1/services/" + active.ID},
		{http.MethodDelete, "/api/v1/services/" + active.ID},
	} {
		st, raw := exec.Do(c.method, c.path, map[string]any{"name": "x"})
		s.Equal(http.StatusForbidden, st, "%s %s", c.method, c.path)
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/services", nil)
	s.Equal(http.StatusUnauthorized, st)
}

// ---- CSRF: мутация без корректного токена → 403 ----

func (s *ServicesLiveSuite) TestCSRF_MissingToken403() {
	cat := s.newCategory("service", "CSRF услуг")
	resp := s.admin.Request(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, nil), map[string]string{"X-Csrf-Token": ""})
	defer resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)
}

// ---- SSR: список услуг, форма создания с комплектом (row-list product_id[]/qty[]) ----

func (s *ServicesLiveSuite) TestSSR_ServicesListAndFormFlow() {
	st, page := s.admin.Do(http.MethodGet, "/catalog/services", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="services-table"`)

	cat := s.newCategory("service", "SSR категория услуг")
	p := s.newProductB(map[string]any{"sku": "SSR-P1", "name": "SSR товар"})

	form := url.Values{
		"category_id": {cat.ID}, "name": {"SSR Услуга"}, "price": {"500"},
		"product_id": {p.ID}, "qty": {"3"},
	}
	resp := s.admin.Request(http.MethodPost, "/catalog/services", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/services?flash=service_created", resp.Header.Get("Location"))

	st, page = s.admin.Do(http.MethodGet, "/catalog/services", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), "SSR Услуга")
	s.Contains(string(page), "SSR товар")
}

// ---- code review fix-loop 2 Medium: errServiceProductInvalid.Index считался по списку
// ПОСЛЕ объединения дублей, а SSR-форма/API-клиент видят ИСХОДНЫЕ строки — [A,A,B(архив)]
// после мерджа даёт [A(сумма),B], и ошибка «products[1]» указывала бы на ВТОРУЮ строку
// исходного списка (тоже A), а не на третью (B). Теперь checkProducts проверяет
// СУЩЕСТВОВАНИЕ/АКТИВНОСТЬ по исходному (не смерженному) списку — индекс должен указывать
// именно на строку с архивным товаром B (индекс 2), не на дубль A (индекс 1). ----

func (s *ServicesLiveSuite) TestService_DuplicateThenArchived_ErrorIndexPointsAtArchivedRow() {
	cat := s.newCategory("service", "Индекс ошибки после дедупа")
	a := s.newProductB(map[string]any{"sku": "IDX-A"})
	b := s.newProductB(map[string]any{"sku": "IDX-B"})

	// b архивируем ДО отправки — новая строка с архивным товаром всегда 400 (Create —
	// грандфазера ещё нет).
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+b.ID, map[string]any{
		"name": "Товар B", "sku": "IDX-B", "unit": "", "price": 0, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// исходный список: [A, A, B(архивный)] — после дедупа стал бы [A(qty=сумма), B]
	// (индекс 1 указывал бы на B в СМЕРЖЕННОМ списке, но НЕ на исходную строку 1, которая
	// у пользователя — вторая A).
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/services", serviceBody(cat.ID, map[string]any{
		"products": []map[string]any{
			{"product_id": a.ID, "qty": 1},
			{"product_id": a.ID, "qty": 2},
			{"product_id": b.ID, "qty": 3},
		},
	}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	fields := s.fields(raw)
	s.Equal("invalid", fields["products[2].product_id"], "индекс 2 — третья исходная строка (B), %s", raw)
	s.NotContains(fields, "products[1].product_id", "вторая исходная строка — дубль A, валиден сам по себе, %s", raw)
}

// ---- code review fix-loop 2 LOW: переполнение суммы дублей → 400 на поле "products", но
// раньше services-form.html это поле нигде не выводил (vm.Error заполняется только когда
// app.Fields пуст) — SSR отдавала 422 без единого видимого сообщения об ошибке. ----

// ---- LV-058 → soft-delete-catalog DS-014: услуга, используемая в заказе, удаляется мягко
// (отказ SERVICE_IN_USE снят) — услуга заказа остаётся. Минимальная строка `tasks`/`subtasks`
// вставляется напрямую SQL. ----

func (s *ServicesLiveSuite) TestDelete_InUseBySubtask_Deletes200() {
	cat := s.newCategory("service", "Guard Услуга Категория")
	sv := s.newService(cat.ID, map[string]any{})

	var clientID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO clients (company_name) VALUES ('Guard Клиент Услуга') RETURNING id`).Scan(&clientID))
	var columnID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT id FROM columns ORDER BY position LIMIT 1`).Scan(&columnID))
	var adminID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT id FROM users WHERE login = 'catalog4b-admin'`).Scan(&adminID))
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO tasks (column_id, client_id, title, position, created_by)
		 VALUES ($1::uuid, $2::uuid, 'Guard task', 1024, $3::uuid) RETURNING id`,
		columnID, clientID, adminID).Scan(&taskID))
	_, err := s.Pool.Exec(s.T().Context(),
		`INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2::uuid)`, taskID, sv.ID)
	s.Require().NoError(err)

	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/services/"+sv.ID, delReason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var subtasks int
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT count(*) FROM subtasks WHERE task_id = $1 AND service_id = $2::uuid`, taskID, sv.ID).Scan(&subtasks))
	s.Equal(1, subtasks, "услуга заказа не изменилась")
}

func (s *ServicesLiveSuite) TestSSR_MergeOverflow_ShowsVisibleError() {
	cat := s.newCategory("service", "SSR переполнение суммы")
	p := s.newProductB(map[string]any{"sku": "OVF-1"})

	// два дубля по 9 цифр каждый — сумма 10 цифр, переполняет qtyScale.IntDigits=9.
	form := url.Values{
		"category_id": {cat.ID}, "name": {"Услуга с переполнением"},
		"product_id": {p.ID, p.ID}, "qty": {"999999999", "999999999"},
	}
	resp := s.admin.Request(http.MethodPost, "/catalog/services", form, nil)
	defer resp.Body.Close()
	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.Contains(string(body), `data-testid="error-products"`, string(body))
	s.Contains(string(body), "Слишком большое число", string(body))
}
