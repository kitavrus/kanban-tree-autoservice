//go:build integration

package catalog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Мягкое удаление складов в каталоге товаров
// (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §6.5, §8.3 п.5, DS-014): блок
// stocks товара — без удалённых складов; удалённый склад в запросе остатков → 400
// stocks[i].warehouse_id: not_found; гонка правки остатков с удалением склада. Методы
// CatalogLiveSuite (§TD-8: боевой бинарь, реальный логин); склад создаётся и удаляется через API
// справочника того же сервера.

// whNew — действующий активный склад, созданный через API справочника.
func (s *CatalogLiveSuite) whNew(name string) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": name})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var d struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	return d.ID
}

func (s *CatalogLiveSuite) whMustDelete(id string) {
	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/warehouses/"+id, map[string]any{"reason": "Склад закрыт"})
	s.Require().Equal(http.StatusOK, st, "удаление склада %s: %s", id, raw)
}

// whProduct — товар через GET /api/v1/products/:id со строками складов.
func (s *CatalogLiveSuite) whProduct(id string) productDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/products/"+id, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var p productDTO
	s.Require().NoError(json.Unmarshal(raw, &p), string(raw))
	return p
}

func stockOn(p productDTO, warehouseID string) (string, bool) {
	for _, r := range p.Stocks {
		if r.WarehouseID == warehouseID {
			return r.Stock.String(), true
		}
	}
	return "", false
}

func (s *CatalogLiveSuite) whPutStocks(productID, categoryID, name string, stocks []map[string]any) (int, []byte) {
	return s.admin.Do(http.MethodPut, "/api/v1/products/"+productID, map[string]any{
		"category_id": categoryID, "name": name, "sku": "", "unit": "шт", "price": 100, "is_active": true, "stocks": stocks,
	})
}

// §6.5 (DS-014, abuse case 2): блок stocks товара — без удалённых складов (GET товара, список
// товаров, серверная форма товара), «Итого» остаток удалённого склада не включает; удалённый
// склад в запросе остатков — при создании и при правке товара → 400 stocks[i].warehouse_id:
// not_found (а не invalid, как у несуществующего идентификатора); остаток на удалённом складе не
// создан и не изменён. До удаления склад в блоке есть, и его остаток правится как обычно.
func (s *CatalogLiveSuite) TestDeletedWarehouse_NotInProductStocks() {
	m := sdMark()
	cat := s.newCategory(s.admin, "product", "SD-склад "+m)
	whName := "SD-каталог Склад " + m
	w := s.whNew(whName)
	name := "SD-склад товар " + m
	p := s.sdProduct(cat.ID, name, map[string]any{testAlmatyID: 5, w: 3})

	got, ok := stockOn(s.whProduct(p), w)
	s.Require().True(ok, "до удаления склад есть в блоке «Склады»")
	s.Equal("3.000", got)
	s.Equal("8.000", s.whProduct(p).TotalStock.String())

	// склад с товаром не удаляется; после списания — удаляется.
	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/warehouses/"+w, map[string]any{"reason": "Склад закрыт"})
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("WAREHOUSE_IN_USE", s.sdErr(raw).Code)
	st, raw = s.whPutStocks(p, cat.ID, name, []map[string]any{{"warehouse_id": testAlmatyID, "stock": 5}, {"warehouse_id": w, "stock": 0}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.whMustDelete(w)

	after := s.whProduct(p)
	_, ok = stockOn(after, w)
	s.False(ok, "строки удалённого склада в блоке «Склады» нет")
	s.Equal("5.000", after.TotalStock.String())
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?q="+url.QueryEscape(name), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []productDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	s.Require().Len(l.Items, 1)
	_, ok = stockOn(l.Items[0], w)
	s.False(ok, "и в списке товаров")
	pst, page := s.page("/catalog/products/" + p)
	s.Require().Equal(http.StatusOK, pst)
	s.Contains(page, `data-testid="product-stocks"`)
	s.NotContains(page, whName, "в форме товара строки удалённого склада нет")
	pst, page = s.page("/catalog/products/new")
	s.Require().Equal(http.StatusOK, pst)
	s.NotContains(page, whName, "и в форме нового товара")

	// удалённый склад в запросе остатков — как несуществующий, но с кодом not_found.
	st, raw = s.whPutStocks(p, cat.ID, name, []map[string]any{{"warehouse_id": testAlmatyID, "stock": 5}, {"warehouse_id": w, "stock": 9}})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"stocks[1].warehouse_id": "not_found"}, s.fields(raw))
	s.Equal("0.000", s.sdScalar(`SELECT stock::text FROM product_stock WHERE product_id = $1 AND warehouse_id = $2`, p, w), "остаток на удалённом складе не изменён")
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/products", map[string]any{
		"category_id": cat.ID, "name": "SD-склад новый " + m, "sku": "", "unit": "шт", "price": 1,
		"stocks": []map[string]any{{"warehouse_id": w, "stock": 2}},
	})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"stocks[0].warehouse_id": "not_found"}, s.fields(raw))
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM products WHERE name = $1`, "SD-склад новый "+m), "товар не создан")
	// несуществующий идентификатор — прежний код invalid (as-is).
	st, raw = s.whPutStocks(p, cat.ID, name, []map[string]any{{"warehouse_id": "00000000-0000-0000-0000-000000000000", "stock": 1}})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"stocks[0].warehouse_id": "invalid"}, s.fields(raw))
}

// §3 п. 1 (DS-021, abuse case 3): удаление склада идёт (строка под FOR UPDATE, отметка
// поставлена, транзакция не зафиксирована) — в это окно правка товара кладёт на этот склад
// остаток. Правка обязана дождаться удаления на строке склада (FOR SHARE, до строк остатка) и
// получить 400 stocks[0].warehouse_id: not_found: остатка на удалённом складе нет. Реализация без
// блокировки после фиксации записала бы остаток на только что удалённый склад.
func (s *CatalogLiveSuite) TestRace_ProductStocksUpdateWaitsForWarehouseDelete() {
	m := sdMark()
	cat := s.newCategory(s.admin, "product", "SD-гонка склад "+m)
	w := s.whNew("SD-гонка каталог " + m)
	name := "SD-гонка склад товар " + m
	p := s.sdProduct(cat.ID, name, nil)
	admin := s.sdAdminID()

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, w)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE warehouses SET deleted_at = now(), deleted_by = $2, delete_reason = 'Гонка' WHERE id = $1`, w, admin)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) {
		return s.whPutStocks(p, cat.ID, name, []map[string]any{{"warehouse_id": w, "stock": 6}})
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal(map[string]string{"stocks[0].warehouse_id": "not_found"}, s.fields(res.raw))
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM product_stock WHERE product_id = $1 AND warehouse_id = $2`, p, w), "остатка на удалённом складе нет")
}

// Симметричный случай к «архивный товар × удалённый склад» (ревью части 6): остаток АРХИВНОГО
// товара на АРХИВНОМ (не удалённом) складе невидимым не становится — он есть в блоке «Склады»
// товара (GET товара) и правится оттуда; после списания до нуля товар удаляется. Поэтому в отчёт
// «Остатки» такой товар не добавлен.
func (s *CatalogLiveSuite) TestArchivedProductOnArchivedWarehouse_VisibleInProductStocks() {
	m := sdMark()
	cat := s.newCategory(s.admin, "product", "SD-арх-арх "+m)
	whName := "SD-каталог Архивный " + m
	w := s.whNew(whName)
	name := "SD-арх-арх товар " + m
	p := s.sdProduct(cat.ID, name, map[string]any{testAlmatyID: 0, w: 6})
	s.sdArchiveProduct(p, cat.ID, name)
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w, map[string]any{"name": whName, "is_active": false})
	s.Require().Equal(http.StatusOK, st, string(raw))

	got, ok := stockOn(s.whProduct(p), w)
	s.Require().True(ok, "строка архивного склада с остатком есть в блоке «Склады» архивного товара")
	s.Equal("6.000", got)
	st, _, raw = s.sdDelete(s.admin, "/api/v1/products/"+p, "Снят")
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("PRODUCT_HAS_STOCK", s.sdErr(raw).Code)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+p, map[string]any{
		"category_id": cat.ID, "name": name, "sku": "", "unit": "шт", "price": 100, "is_active": false,
		"stocks": []map[string]any{{"warehouse_id": w, "stock": 0}},
	})
	s.Require().Equal(http.StatusOK, st, "остаток на архивном складе списывается из формы товара: %s", raw)
	s.sdMustDelete("/api/v1/products/"+p, "Снят")
}
