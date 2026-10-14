//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// Мягкое удаление складов на доске и в отчёте «Остатки»
// (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §6.5, §8.3, §11): TS-055 (остаток
// на удалённом складе, DS-042, DS-048), TS-065 (архивные и удалённые склады с остатком в отчёте,
// DS-049), пометка is_deleted склада в карточке заказа и запрет выбора удалённого склада
// (DS-014), фильтр доски, гонки §3. Сьюта — OrderCardLiveSuite (§TD-8: боевой бинарь, реальный
// логин); склады создаются, архивируются и удаляются через API справочника того же сервера.

// ---- wire-типы (§TD-6.2) ----

// whStockDTO — GET /api/v1/stock: склады отчёта с признаками архива и удаления, строки с
// ячейками по складам.
type whStockDTO struct {
	Warehouses []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		IsArchived bool   `json:"is_archived"`
		IsDeleted  bool   `json:"is_deleted"`
	} `json:"warehouses"`
	Items []whStockRowDTO `json:"items"`
}

type whStockRowDTO struct {
	ProductID   string      `json:"product_id"`
	IsDeleted   bool        `json:"is_deleted"`
	IsArchived  bool        `json:"is_archived"`
	Stock       json.Number `json:"stock"`
	MinStock    json.Number `json:"min_stock"`
	State       string      `json:"state"`
	ByWarehouse []struct {
		WarehouseID string      `json:"warehouse_id"`
		Stock       json.Number `json:"stock"`
		State       string      `json:"state"`
	} `json:"by_warehouse"`
}

// whCardDTO — GET /api/v1/tasks/:id/card: склады карточки с признаком удаления, склад строки.
type whCardDTO struct {
	Subtasks []struct {
		ID       string `json:"id"`
		Products []struct {
			ID          string `json:"id"`
			WarehouseID string `json:"warehouse_id"`
		} `json:"products"`
	} `json:"subtasks"`
	Warehouses []struct {
		ID        string `json:"id"`
		IsActive  bool   `json:"is_active"`
		IsDeleted bool   `json:"is_deleted"`
	} `json:"warehouses"`
}

const msgWarehouseWriteOffOnly = "Склад удалён — остаток на нём можно только списать"

// ---- помощники ----

// whNew — действующий активный склад, созданный через API справочника.
func (s *OrderCardLiveSuite) whNew(name string) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": name})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var d struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	return d.ID
}

func (s *OrderCardLiveSuite) whMustDelete(id string) {
	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/warehouses/"+id, map[string]any{"reason": "Склад закрыт"})
	s.Require().Equal(http.StatusOK, st, "удаление склада %s: %s", id, raw)
}

func (s *OrderCardLiveSuite) whMustArchive(id, name string) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+id, map[string]any{"name": name, "is_active": false})
	s.Require().Equal(http.StatusOK, st, "архив склада %s: %s", id, raw)
}

// whStock — отчёт «Остатки» (query — параметры списка, начиная с «&»).
func (s *OrderCardLiveSuite) whStock(query string) whStockDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock?limit=1000"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l whStockDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	return l
}

// mark — пометка склада в шапке отчёта: "" — склада в отчёте нет, иначе «archived=…,deleted=…».
func (l whStockDTO) mark(warehouseID string) string {
	for _, w := range l.Warehouses {
		if w.ID == warehouseID {
			return fmt.Sprintf("archived=%t,deleted=%t", w.IsArchived, w.IsDeleted)
		}
	}
	return ""
}

func (l whStockDTO) row(productID string) (whStockRowDTO, bool) {
	for _, it := range l.Items {
		if it.ProductID == productID {
			return it, true
		}
	}
	return whStockRowDTO{}, false
}

// cell — остаток ячейки «товар × склад» строки ("" — ячейки нет: склада нет в отчёте).
func (r whStockRowDTO) cell(warehouseID string) string {
	for _, c := range r.ByWarehouse {
		if c.WarehouseID == warehouseID {
			return c.Stock.String()
		}
	}
	return ""
}

// whAdjust — ручная корректировка остатка пары товар × склад.
func (s *OrderCardLiveSuite) whAdjust(productID, warehouseID string, stock any) (int, []byte) {
	return s.admin.Do(http.MethodPut, "/api/v1/stock/"+productID, map[string]any{"stock": stock, "warehouse_id": warehouseID})
}

func (s *OrderCardLiveSuite) whCard(taskID int64) whCardDTO {
	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var c whCardDTO
	s.Require().NoError(json.Unmarshal(raw, &c), string(raw))
	return c
}

// cardMark — склад в массиве warehouses карточки: "" — нет, иначе «active=…,deleted=…».
func (c whCardDTO) cardMark(warehouseID string) string {
	for _, w := range c.Warehouses {
		if w.ID == warehouseID {
			return fmt.Sprintf("active=%t,deleted=%t", w.IsActive, w.IsDeleted)
		}
	}
	return ""
}

func (s *OrderCardLiveSuite) whAddProduct(subtaskID, productID, warehouseID string, qty int) (int, []byte) {
	return s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/products",
		map[string]any{"product_id": productID, "qty": qty, "warehouse_id": warehouseID})
}

// ---- TS-055 ----

// TS-055 (DS-042, DS-048, DS-014): склад удалён пустым (товар списан заказом до нуля; строка
// товара заказа склад не удерживает), затем заказ уходит из списывающей колонки — остаток
// возвращается на удалённый склад. В GET /api/v1/stock склад — в warehouses с is_deleted = true,
// строка товара видна и в режиме «рисковые» (остаток не теряется из вида), ячейка склада — с
// возвращённым остатком; в счётчики summary остаток на удалённом складе не входит. В карточке
// заказа склад — с is_deleted = true; в карточке заказа без строк на этом складе его нет.
// Корректировка на удалённом складе — только списание: увеличение и смена знака → 409
// WAREHOUSE_DELETED_WRITE_OFF_ONLY, остаток не изменён; частичное списание и до нуля → 200, после
// нуля склад исчезает из отчёта. Негатив: удалённый склад без остатка в отчёте не показывается,
// фильтр по нему → 400 warehouse_id: not_found; новая строка товара заказа на удалённом складе и
// смена склада строки на удалённый → 400 warehouse_id: not_found; корректировка пары без остатка
// → 400 warehouse_id: not_found, строка остатка не создаётся.
func (s *OrderCardLiveSuite) TestTS055_StockOnDeletedWarehouse() {
	m := uuid.NewString()[:8]
	almaty := s.warehouseID("Алматы")
	w := s.whNew("SD-055 Склад " + m)
	taskID, _ := s.newTask(s.newClient("SD-055 Клиент "+m), s.columnID("В работе"))
	subID := s.addSubtask(taskID, s.newService("1000"), "1000")
	product := s.newProduct("50", "20", "0") // Алматы: 20 — у товара есть «спокойная» действующая пара
	s.setPairStock(product, w, "3")
	st, raw := s.whAddProduct(subID, product, w, 3)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Equal("0.000", s.pairStock(product, w), "товар списан заказом со склада до нуля")
	other := s.newProduct("50", "20", "0")

	s.whMustDelete(w)
	deletedSummary := s.catSummary()

	// негатив: удалённый склад без остатка в отчёте не показывается; фильтр по нему — 400.
	for _, q := range []string{"", "&scope=all"} {
		s.Empty(s.whStock(q).mark(w), "нулевой остаток — склада в отчёте нет (%q)", q)
	}
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/stock?warehouse_id="+w, nil)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"warehouse_id": "not_found"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?warehouse_id="+w, nil)
	s.Require().Equal(http.StatusBadRequest, st, "фильтр доски по удалённому складу: %s", raw)
	s.Equal(map[string]string{"warehouse_id": "not_found"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?warehouse_id="+almaty, nil)
	s.Equal(http.StatusOK, st, "фильтр доски по действующему складу: %s", raw)

	// негатив: выбрать удалённый склад в новой строке и при смене склада строки нельзя.
	st, raw = s.whAddProduct(subID, other, w, 1)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["warehouse_id"])
	st, raw = s.whAddProduct(subID, other, almaty, 1)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	card := s.whCard(taskID)
	var otherRow, goneRow string
	for _, p := range card.Subtasks[0].Products {
		if p.WarehouseID == almaty {
			otherRow = p.ID
		} else {
			goneRow = p.ID
		}
	}
	s.Require().NotEmpty(otherRow)
	s.Require().NotEmpty(goneRow)
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtask-products/"+otherRow, map[string]any{"qty": 1, "warehouse_id": w})
	s.Require().Equal(http.StatusBadRequest, st, "смена склада строки на удалённый: %s", raw)
	s.Equal("not_found", s.fields(raw)["warehouse_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM subtask_products WHERE product_id = $1 AND warehouse_id = $2`, other, w))

	// карточка: удалённый склад — только там, где на него ссылается строка заказа.
	s.Equal("active=true,deleted=true", card.cardMark(w), "склад строки заказа — с пометкой «удалён»")
	s.Equal("active=true,deleted=false", card.cardMark(almaty))
	foreign, _ := s.newTask(s.newClient("SD-055 Чужой "+m), s.columnID("Новые"))
	s.Empty(s.whCard(foreign).cardMark(w), "в карточке без строк на удалённом складе его нет")

	// заказ уходит из списывающей колонки — остаток вернулся на удалённый склад.
	s.moveTask(taskID, "Новые")
	s.Require().Equal("3.000", s.pairStock(product, w))
	for _, q := range []string{"", "&scope=all"} {
		l := s.whStock(q)
		s.Equal("archived=false,deleted=true", l.mark(w), "удалённый склад с остатком — в отчёте с пометкой (%q)", q)
		row, ok := l.row(product)
		s.Require().True(ok, "товар с остатком на удалённом складе виден в отчёте (%q)", q)
		s.False(row.IsDeleted, "сам товар действует")
		s.Equal("3.000", row.cell(w), q)
		s.Equal("20.000", row.cell(almaty), "ячейка действующего склада — как была")
		s.Equal("23.000", row.Stock.String(), "«Итого» включает остаток удалённого склада")
	}
	s.Equal("archived=false,deleted=true", s.whStock("&warehouse_id="+w).mark(w), "фильтр по удалённому складу с остатком работает")
	s.Equal(deletedSummary, s.catSummary(), "остаток на удалённом складе в счётчики не входит")

	// корректировка на удалённом складе — только списание.
	for name, v := range map[string]any{"увеличение": 4, "увеличение на тысячную": 3.001, "смена знака": -1} {
		st, raw = s.whAdjust(product, w, v)
		s.Require().Equal(http.StatusConflict, st, "%s: %s", name, raw)
		s.Equal("WAREHOUSE_DELETED_WRITE_OFF_ONLY", s.sdErr(raw).Code, name)
		s.Equal(msgWarehouseWriteOffOnly, s.sdErr(raw).Message, name)
	}
	s.Equal("3.000", s.pairStock(product, w), "отказ остаток не меняет")
	// пара без остатка на удалённом складе: строка остатка не создаётся.
	st, raw = s.whAdjust(other, w, 0)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"warehouse_id": "not_found"}, s.fields(raw))
	st, raw = s.whAdjust(other, w, 5)
	s.Require().Equal(http.StatusBadRequest, st, "положить товар на удалённый склад корректировкой нельзя: %s", raw)
	s.Equal(map[string]string{"warehouse_id": "not_found"}, s.fields(raw))
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM product_stock WHERE product_id = $1 AND warehouse_id = $2`, other, w))

	st, raw = s.whAdjust(product, w, 1)
	s.Require().Equal(http.StatusOK, st, "частичное списание: %s", raw)
	s.Equal("1.000", s.pairStock(product, w))
	s.Equal("archived=false,deleted=true", s.whStock("").mark(w), "остаток ещё есть — склад в отчёте")
	st, raw = s.whAdjust(product, w, 0)
	s.Require().Equal(http.StatusOK, st, "списание до нуля: %s", raw)
	s.Equal("0.000", s.pairStock(product, w))
	for _, q := range []string{"", "&scope=all"} {
		l := s.whStock(q)
		s.Empty(l.mark(w), "после списания до нуля склад исчез из отчёта (%q)", q)
		if row, ok := l.row(product); ok {
			s.Empty(row.cell(w), "ячейки удалённого склада больше нет")
		}
	}
	st, raw = s.whAdjust(product, w, 2)
	s.Require().Equal(http.StatusBadRequest, st, "остаток уже нулевой — склада для корректировки нет: %s", raw)
	s.Equal(map[string]string{"warehouse_id": "not_found"}, s.fields(raw))
	s.Equal("0.000", s.pairStock(product, w))

	// долг на удалённом складе (списание заказом при нулевом остатке) так же гасится к нулю.
	s.setPairStock(product, w, "-2")
	for name, v := range map[string]any{"долг растёт": -3, "долг → плюс": 1} {
		st, raw = s.whAdjust(product, w, v)
		s.Require().Equal(http.StatusConflict, st, "%s: %s", name, raw)
		s.Equal("WAREHOUSE_DELETED_WRITE_OFF_ONLY", s.sdErr(raw).Code, name)
	}
	st, raw = s.whAdjust(product, w, -1)
	s.Require().Equal(http.StatusOK, st, "долг уменьшается: %s", raw)
	st, raw = s.whAdjust(product, w, 0)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0.000", s.pairStock(product, w))
}

// ---- TS-065 ----

// TS-065 (DS-049, DS-048): в GET /api/v1/stock архивный склад с ненулевым остатком товара отчёта
// присутствует в warehouses с is_archived = true (is_deleted = false), удалённый — с is_deleted =
// true; архивный склад без остатка и архивный склад с остатком только архивного (не входящего в
// отчёт) товара — отсутствуют. Удалённый товар с остатком ТОЛЬКО на архивном складе показан с
// этим остатком, а не нулями, — и ячейки с ненулевым остатком у него только на этом складе (окно
// корректировки предлагает только их); остаток списывается до нуля. Остаток на архивном складе в
// счётчики не входит — даже отрицательный. Корректировка действующего товара на архивном складе
// — как раньше, 400 warehouse_id: invalid. Негатив: активный склад без остатка по-прежнему
// показывается колонкой.
func (s *OrderCardLiveSuite) TestTS065_StockOnArchivedAndDeletedWarehouses() {
	m := uuid.NewString()[:8]
	withStock, empty, hidden, idle := s.whNew("SD-065 С остатком "+m), s.whNew("SD-065 Пустой "+m),
		s.whNew("SD-065 Архивный товар "+m), s.whNew("SD-065 Активный пустой "+m)
	live := s.newProduct("50", "20", "0")
	s.setPairStock(live, withStock, "4")
	archivedProduct := s.newProduct("50", "0", "0")
	s.setProductActive(archivedProduct, false)
	s.setPairStock(archivedProduct, hidden, "7")
	gone := s.newProduct("50", "0", "0")
	s.catMustDelete("/api/v1/products/" + gone)
	s.setPairStock(gone, withStock, "5") // остаток появился после удаления товара — только на будущем архивном складе
	before := s.catSummary()

	s.whMustArchive(withStock, "SD-065 С остатком "+m)
	s.whMustArchive(empty, "SD-065 Пустой "+m)
	s.whMustArchive(hidden, "SD-065 Архивный товар "+m)

	for _, q := range []string{"", "&scope=all"} {
		l := s.whStock(q)
		s.Equal("archived=true,deleted=false", l.mark(withStock), "архивный склад с остатком — в отчёте с пометкой (%q)", q)
		s.Empty(l.mark(empty), "архивный склад без остатка в отчёте не показывается (%q)", q)
		s.Empty(l.mark(hidden), "остаток архивного товара склад в отчёт не приводит (%q)", q)
		s.Equal("archived=false,deleted=false", l.mark(idle), "активный склад без остатка — колонкой, как раньше (%q)", q)

		row, ok := l.row(gone)
		s.Require().True(ok, "удалённый товар с остатком на архивном складе виден (%q)", q)
		s.True(row.IsDeleted)
		s.Equal("5.000", row.Stock.String(), "остаток — не нули")
		for _, c := range row.ByWarehouse {
			v, err := c.Stock.Float64()
			s.Require().NoError(err)
			want := 0.0
			if c.WarehouseID == withStock {
				want = 5
			}
			s.Equal(want, v, "ненулевая ячейка удалённого товара — только на складе с его остатком (%s)", c.WarehouseID)
		}
	}
	all := s.whStock("&scope=all")
	liveRow, ok := all.row(live)
	s.Require().True(ok)
	s.Equal("4.000", liveRow.cell(withStock))
	s.Equal("24.000", liveRow.Stock.String(), "«Итого» включает остаток архивного склада")
	minStock, err := liveRow.MinStock.Float64()
	s.Require().NoError(err)
	s.Zero(minStock, "минимум архивного склада в «Мин.» строки не входит")
	s.Equal(before, s.catSummary(), "архивирование склада счётчики не меняет")

	// отрицательный остаток на архивном складе в счётчики риска не входит.
	s.setPairStock(live, withStock, "-1")
	s.Equal(before, s.catSummary(), "долг на архивном складе — не «нет в наличии»")
	liveRow, _ = s.whStock("&scope=all").row(live)
	s.Equal("ok", liveRow.State, "состояние строки определяют действующие склады")
	s.setPairStock(live, withStock, "4")

	// действующий товар на архивном складе вручную не правится (as-is).
	st, raw := s.whAdjust(live, withStock, 1)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"warehouse_id": "invalid"}, s.fields(raw))
	s.Equal("4.000", s.pairStock(live, withStock))

	// удалённый товар на архивном складе списывается (DS-048: активность склада не требуется).
	st, raw = s.whAdjust(gone, withStock, 0)
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, ok = s.whStock("&scope=all").row(gone)
	s.False(ok, "после списания до нуля строка удалённого товара исчезла")
	s.Equal("archived=true,deleted=false", s.whStock("").mark(withStock), "на складе остался остаток действующего товара")

	// архивный склад удалён: пометка «в архиве» сменяется на «удалён» — после списания его остатка
	// и возврата (остаток появился уже на удалённом складе).
	s.setPairStock(live, withStock, "0")
	s.whMustDelete(withStock)
	s.setPairStock(live, withStock, "2")
	s.Equal("archived=false,deleted=true", s.whStock("").mark(withStock), "у удалённого склада пометка одна — «удалён»")
	st, raw = s.whAdjust(live, withStock, 3)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("WAREHOUSE_DELETED_WRITE_OFF_ONLY", s.sdErr(raw).Code)
	st, raw = s.whAdjust(live, withStock, 0)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(s.whStock("").mark(withStock))
}

// ---- fix-loop 1: количество строки на удалённом складе, долг, архивный товар ----

func (s *OrderCardLiveSuite) whPutRow(rowID string, body map[string]any) (int, []byte) {
	return s.admin.Do(http.MethodPut, "/api/v1/subtask-products/"+rowID, body)
}

func (s *OrderCardLiveSuite) whRowQty(rowID string) string {
	return s.sdScalar(`SELECT qty::text || '|' || warehouse_id::text FROM subtask_products WHERE id = $1`, rowID)
}

// whRow — заказ в колонке columnName со строкой товара qty на складе w; возвращает заказ, товар
// и строку.
func (s *OrderCardLiveSuite) whRow(m, columnName, w string, stock string, qty int) (int64, string, string) {
	taskID, _ := s.newTask(s.newClient("SD-строка "+m+" "+uuid.NewString()[:6]), s.columnID(columnName))
	subID := s.addSubtask(taskID, s.newService("1000"), "1000")
	product := s.newProduct("50", "20", "0")
	s.setPairStock(product, w, stock)
	st, raw := s.whAddProduct(subID, product, w, qty)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	rowID := s.sdScalar(`SELECT id::text FROM subtask_products WHERE product_id = $1`, product)
	return taskID, product, rowID
}

// Решение по ревью части 6: УВЕЛИЧЕНИЕ количества в строке товара заказа, склад которой удалён,
// отклоняется — 409 WAREHOUSE_DELETED «Склад строки удалён — выберите другой склад»; количество и
// остаток не изменились (заказ в списывающей колонке не уводит удалённый склад в долг). То же
// количество, уменьшение (возврат на удалённый склад), смена склада на действующий (в том числе
// вместе с увеличением) и удаление строки — проходят.
func (s *OrderCardLiveSuite) TestQtyIncreaseRefusedOnDeletedWarehouse() {
	m := uuid.NewString()[:8]
	almaty := s.warehouseID("Алматы")
	w := s.whNew("SD-кол-во " + m)
	_, product, rowID := s.whRow(m, "В работе", w, "3", 3) // списано 3 → на складе 0
	_, second, secondRow := s.whRow(m, "Новые", w, "0", 2)
	s.Require().Equal("0.000", s.pairStock(product, w))
	s.whMustDelete(w)

	for name, qty := range map[string]any{"на единицу": 4, "на тысячную": 3.001} {
		st, raw := s.whPutRow(rowID, map[string]any{"qty": qty})
		s.Require().Equal(http.StatusConflict, st, "%s: %s", name, raw)
		s.Equal("WAREHOUSE_DELETED", s.sdErr(raw).Code, name)
		s.Equal("Склад строки удалён — выберите другой склад", s.sdErr(raw).Message, name)
	}
	s.Equal("3.000|"+w, s.whRowQty(rowID), "количество не изменилось")
	s.Equal("0.000", s.pairStock(product, w), "удалённый склад в долг не ушёл")

	st, raw := s.whPutRow(rowID, map[string]any{"qty": 3})
	s.Require().Equal(http.StatusOK, st, "то же количество: %s", raw)
	st, raw = s.whPutRow(rowID, map[string]any{"qty": 1})
	s.Require().Equal(http.StatusOK, st, "уменьшение: %s", raw)
	s.Equal("1.000|"+w, s.whRowQty(rowID))
	s.Equal("2.000", s.pairStock(product, w), "уменьшение вернуло товар на удалённый склад")
	s.Equal("archived=false,deleted=true", s.whStock("").mark(w), "возвращённый остаток виден с пометкой")

	// смена склада на действующий — вместе с увеличением количества.
	st, raw = s.whPutRow(rowID, map[string]any{"qty": 5, "warehouse_id": almaty})
	s.Require().Equal(http.StatusOK, st, "смена склада на действующий: %s", raw)
	s.Equal("5.000|"+almaty, s.whRowQty(rowID))
	s.Equal("3.000|15.000", s.pairStock(product, w)+"|"+s.pairStock(product, almaty), "возврат на старый склад, списание с нового")

	// строка в несписывающей колонке: правило то же; удаление строки проходит.
	st, raw = s.whPutRow(secondRow, map[string]any{"qty": 3})
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("WAREHOUSE_DELETED", s.sdErr(raw).Code)
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/subtask-products/"+secondRow, nil)
	s.Require().Equal(http.StatusOK, st, "удаление строки: %s", raw)
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM subtask_products WHERE product_id = $1`, second))
}

// Перенос заказа по колонкам не блокируется (DS-048): заказ со строкой на удалённом складе
// уходит в списывающую колонку — на удалённом складе появляется ДОЛГ. Он виден: склад в отчёте с
// пометкой «удалён», строка товара — и в режиме «рисковые», ячейка склада отрицательная; долг
// гасится корректировкой к нулю, после чего склад из отчёта исчезает.
func (s *OrderCardLiveSuite) TestDebtOnDeletedWarehouseVisibleAndWrittenOff() {
	m := uuid.NewString()[:8]
	w := s.whNew("SD-долг " + m)
	taskID, product, _ := s.whRow(m, "Новые", w, "0", 2)
	s.whMustDelete(w)
	s.Empty(s.whStock("").mark(w), "потребность без остатка склад в отчёт не приводит")

	s.moveTask(taskID, "В работе")
	s.Require().Equal("-2.000", s.pairStock(product, w))
	for _, q := range []string{"", "&scope=all"} {
		l := s.whStock(q)
		s.Equal("archived=false,deleted=true", l.mark(w), "склад с долгом — в отчёте с пометкой (%q)", q)
		row, ok := l.row(product)
		s.Require().True(ok, "товар с долгом на удалённом складе виден (%q)", q)
		s.Equal("-2.000", row.cell(w), q)
	}
	st, raw := s.whAdjust(product, w, -3)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("WAREHOUSE_DELETED_WRITE_OFF_ONLY", s.sdErr(raw).Code)
	st, raw = s.whAdjust(product, w, 0)
	s.Require().Equal(http.StatusOK, st, "долг списан к нулю: %s", raw)
	s.Empty(s.whStock("&scope=all").mark(w), "после списания склад исчез из отчёта")
}

// Решение по ревью части 6 (DS-042): остаток АРХИВНОГО (не удалённого) товара на удалённом складе
// виден в отчёте «Остатки» — строка с is_archived = true (форма товара удалённые склады не
// показывает, иначе остаток не был бы виден нигде и молча блокировал бы удаление товара).
// Остаток только списывается (DS-048); после списания до нуля строка и склад исчезают, и товар
// удаляется. В счётчики архивный товар не входит. Негатив: архивный товар без остатка на
// удалённом складе в отчёте не показывается (как раньше); его корректировка на действующем складе
// — по-прежнему 404.
func (s *OrderCardLiveSuite) TestArchivedProductStockOnDeletedWarehouse() {
	m := uuid.NewString()[:8]
	almaty := s.warehouseID("Алматы")
	w := s.whNew("SD-архивный товар " + m)
	p := s.newProduct("50", "0", "0")
	s.setProductActive(p, false)
	s.whMustDelete(w)
	before := s.catSummary()

	_, ok := s.whStock("&scope=all").row(p)
	s.False(ok, "архивный товар без остатка на удалённом складе в отчёт не входит")
	st, raw := s.whAdjust(p, almaty, 1)
	s.Equal(http.StatusNotFound, st, "корректировка архивного товара на действующем складе — как раньше: %s", raw)

	s.setPairStock(p, w, "4") // остаток появился на уже удалённом складе (возврат из заказа)
	st, raw = s.catDelete("/api/v1/products/" + p)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("PRODUCT_HAS_STOCK", s.sdErr(raw).Code)
	for _, q := range []string{"", "&scope=all"} {
		l := s.whStock(q)
		s.Equal("archived=false,deleted=true", l.mark(w), q)
		row, ok := l.row(p)
		s.Require().True(ok, "архивный товар с остатком на удалённом складе виден (%q)", q)
		s.True(row.IsArchived, "пометка «в архиве»")
		s.False(row.IsDeleted)
		s.Equal("4.000", row.cell(w), q)
		s.Equal("4.000", row.Stock.String(), q)
		s.Equal("ok", row.State, "архивный товар в риски не попадает")
	}
	s.Equal(before, s.catSummary(), "архивный товар в счётчики не входит")

	st, raw = s.whAdjust(p, w, 5)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("WAREHOUSE_DELETED_WRITE_OFF_ONLY", s.sdErr(raw).Code)
	st, raw = s.whAdjust(p, w, 0)
	s.Require().Equal(http.StatusOK, st, "списание до нуля: %s", raw)
	var row whStockRowDTO
	s.Require().NoError(json.Unmarshal(raw, &row), string(raw))
	s.Equal(p, row.ProductID)
	s.True(row.IsArchived)
	l := s.whStock("&scope=all")
	_, ok = l.row(p)
	s.False(ok, "после списания строка архивного товара исчезла")
	s.Empty(l.mark(w))
	s.catMustDelete("/api/v1/products/" + p)
}

// §8.3 п.6 (DS-014): в фильтре «Склад» доски удалённых складов нет; действующий архивный —
// остаётся (на нём ещё может числиться оборудование).
func (s *OrderCardLiveSuite) TestBoardWarehouseFilter_NoDeletedWarehouses() {
	m := uuid.NewString()[:8]
	goneName, archName := "SD-фильтр Удалённый "+m, "SD-фильтр Архивный "+m
	gone, arch := s.whNew(goneName), s.whNew(archName)
	s.whMustArchive(arch, archName)
	s.whMustDelete(gone)

	resp := s.admin.Request(http.MethodGet, "/", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Contains(string(body), `id="filter-warehouse"`)
	s.Contains(string(body), archName, "архивный склад в фильтре остаётся")
	s.NotContains(string(body), goneName, "удалённого склада в фильтре нет")
	s.NotContains(string(body), gone)
}

// ---- гонки (§3 «Конкурентность») ----

// whHoldDelete — «удаление склада в полёте»: строка склада под FOR UPDATE (как в
// warehouses.Service.softDelete) и отметка удаления, транзакция не зафиксирована.
func (s *OrderCardLiveSuite) whHoldDelete(warehouseID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, warehouseID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE warehouses SET deleted_at = now(), deleted_by = $2, delete_reason = 'Гонка' WHERE id = $1`, warehouseID, s.catAdminID())
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 п. 1 (DS-014, DS-021, abuse case 3): удаление склада идёт — в это окно в заказ списывающей
// колонки добавляется строка товара с этим складом (она сразу списала бы остаток в минус).
// Добавление обязано дождаться удаления на строке склада (FOR SHARE) и получить 400 warehouse_id:
// not_found; строки и остатка на удалённом складе нет. Реализация, читающая «склад не удалён»
// без блокировки, после фиксации создала бы строку и долг на только что удалённом складе.
func (s *OrderCardLiveSuite) TestRace_SubtaskProductAddWaitsForWarehouseDelete() {
	m := uuid.NewString()[:8]
	w := s.whNew("SD-гонка строка " + m)
	taskID, _ := s.newTask(s.newClient("SD-гонка строка "+m), s.columnID("В работе"))
	subID := s.addSubtask(taskID, s.newService("1000"), "1000")
	product := s.newProduct("50", "20", "0")

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) { return s.whAddProduct(subID, product, w, 2) })
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["warehouse_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM subtask_products WHERE product_id = $1`, product), "строка не создана")
	s.Equal("0", s.pairStock(product, w), "остатка на удалённом складе нет (строки остатка нет)")
}

// Решение по ревью части 6: удаление склада идёт (строка под FOR UPDATE, отметка поставлена,
// транзакция не зафиксирована) — в это окно приходит увеличение количества в строке товара заказа
// на этом складе. Правка обязана дождаться удаления на строке склада (FOR SHARE) и получить 409
// WAREHOUSE_DELETED; количество и остаток не изменились. Реализация, читающая «склад не удалён»
// без блокировки, после фиксации увеличила бы количество и увела бы удалённый склад в долг.
func (s *OrderCardLiveSuite) TestRace_QtyIncreaseWaitsForWarehouseDelete() {
	m := uuid.NewString()[:8]
	w := s.whNew("SD-гонка кол-во " + m)
	_, product, rowID := s.whRow(m, "В работе", w, "3", 3)

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) { return s.whPutRow(rowID, map[string]any{"qty": 5}) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("WAREHOUSE_DELETED", s.sdErr(res.raw).Code)
	s.Equal("3.000|"+w, s.whRowQty(rowID))
	s.Equal("0.000", s.pairStock(product, w))
}

// DS-048 + §3 п. 1: удаление склада идёт — в это окно приходит ручная корректировка, кладущая
// товар на этот склад. Корректировка обязана дождаться удаления на строке склада (FOR SHARE, до
// строки остатка) и получить 400 warehouse_id: not_found: остатка на удалённом складе нет.
// Реализация без блокировки после фиксации записала бы остаток на удалённый склад.
func (s *OrderCardLiveSuite) TestRace_StockAdjustWaitsForWarehouseDelete() {
	w := s.whNew("SD-гонка корректировка " + uuid.NewString()[:8])
	product := s.newProduct("50", "20", "0")

	res := s.catRace(s.whHoldDelete(w), func() (int, []byte) { return s.whAdjust(product, w, 5) })
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal(map[string]string{"warehouse_id": "not_found"}, s.fields(res.raw))
	s.Equal("0", s.pairStock(product, w), "остатка на удалённом складе нет (строки остатка нет)")
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM product_stock WHERE product_id = $1 AND warehouse_id = $2`, product, w), "строка остатка не создана")
}
