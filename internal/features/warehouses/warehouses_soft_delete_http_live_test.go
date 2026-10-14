//go:build integration

package warehouses_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление складов (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §4.1,
// §6.1, §6.2, §6.5, §11) — методы WarehousesLiveSuite (§TD-8: боевой бинарь, реальный логин):
// TS-031 (удаление только пустого склада, три разных сообщения WAREHOUSE_IN_USE, последний
// активный), TS-057 (мягко удалённая заявка склад не удерживает, DS-044), DS-017/DS-009 (название
// свободно, восстановление), TS-033 (права), причина удаления, серверная форма, служебная уборка и
// гонки §3. Каждый тест возвращает сьюту в состояние «склады никем не заняты» — остальные тесты
// архивируют все склады (D47).

// ---- wire-типы (§TD-6.2) ----

type sdDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type sdRestoreDTO struct {
	ID string `json:"id"`
}

type sdErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	msgHasStock     = "На складе есть товар — сначала переместите или спишите остаток"
	msgHasEquipment = "На складе есть оборудование — сначала переместите его"
	msgHasTransfers = "На складе есть незавершённые перемещения"
	msgLastActive   = "Нельзя удалить последний активный склад"
)

// ---- помощники ----

func sdMark() string { return uuid.NewString()[:8] }

func (s *WarehousesLiveSuite) sdErr(raw []byte) sdErrDTO {
	var e sdErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *WarehousesLiveSuite) sdScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *WarehousesLiveSuite) sdExec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *WarehousesLiveSuite) sdAdminID() string {
	return s.sdScalar(`SELECT id::text FROM users WHERE login = 'wh-admin'`)
}

// whState — «удалён|is_active|position|причина|кто удалил» строки склада; "" — строки нет.
func (s *WarehousesLiveSuite) whState(id string) string {
	return s.sdScalar(`
		SELECT coalesce((SELECT (deleted_at IS NOT NULL)::text || '|' || is_active::text || '|' || position::text || '|' ||
		                        coalesce(delete_reason, '') || '|' || coalesce(deleted_by::text, '')
		                 FROM warehouses WHERE id = $1), '')`, id)
}

func (s *WarehousesLiveSuite) deleteWh(b *testutil.Browser, id string, reason any) (int, sdDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, "/api/v1/warehouses/"+id, body)
	var d sdDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	}
	return st, d, raw
}

func (s *WarehousesLiveSuite) mustDeleteWh(id string) {
	st, _, raw := s.deleteWh(s.admin, id, "Создан по ошибке")
	s.Require().Equal(http.StatusOK, st, "удаление склада %s: %s", id, raw)
}

// refused — удаление обязано быть отклонено 409 с кодом code и сообщением msg; склад действует.
func (s *WarehousesLiveSuite) refused(id, code, msg, ctx string) {
	st, _, raw := s.deleteWh(s.admin, id, "Создан по ошибке")
	s.Require().Equal(http.StatusConflict, st, "%s: %s", ctx, raw)
	e := s.sdErr(raw)
	s.Equal(code, e.Code, ctx)
	s.Equal(msg, e.Message, ctx)
	s.True(strings.HasPrefix(s.whState(id), "false|"), "%s: склад не удалён", ctx)
}

func (s *WarehousesLiveSuite) restoreDeleted(b *testutil.Browser, id string) (int, []byte) {
	return b.Do(http.MethodPost, "/api/v1/warehouses/"+id+"/restore", nil)
}

func (s *WarehousesLiveSuite) listed(id string) bool {
	for _, w := range s.list("?include_archived=true").Items {
		if w.ID == id {
			return true
		}
	}
	return false
}

// sdProduct — товар (SQL-фикстура каталога); deleted — мягко удалён.
func (s *WarehousesLiveSuite) sdProduct(deleted bool) string {
	m := sdMark()
	cat := s.sdScalar(`INSERT INTO catalog_categories (kind, name) VALUES ('product', $1) RETURNING id::text`, "WH-SD "+m)
	p := s.sdScalar(`INSERT INTO products (category_id, name) VALUES ($1, $2) RETURNING id::text`, cat, "WH-SD товар "+m)
	if deleted {
		s.sdExec(`UPDATE products SET deleted_at = now(), deleted_by = $2, delete_reason = 'Фикстура' WHERE id = $1`, p, s.sdAdminID())
	}
	return p
}

func (s *WarehousesLiveSuite) sdStock(productID, warehouseID, stock string) {
	s.sdExec(`
		INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, $3::numeric)
		ON CONFLICT (product_id, warehouse_id) DO UPDATE SET stock = EXCLUDED.stock`, productID, warehouseID, stock)
}

// onlyActive — все действующие активные склады, кроме keep, отправлены в архив; возвращает
// функцию возврата их в актив.
func (s *WarehousesLiveSuite) onlyActive(keep ...string) func() {
	keepSet := map[string]bool{}
	for _, id := range keep {
		keepSet[id] = true
	}
	var archived []warehouseDTO
	for _, w := range s.list("").Items {
		if keepSet[w.ID] {
			continue
		}
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": false})
		s.Require().Equal(http.StatusOK, st, string(raw))
		archived = append(archived, w)
	}
	return func() {
		for _, w := range archived {
			st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": true})
			s.Require().Equal(http.StatusOK, st, string(raw))
		}
	}
}

func (s *WarehousesLiveSuite) sdExecutor(login string) *testutil.Browser {
	s.SeedUser(login, "password1", "Склады Исполнитель", "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b
}

func (s *WarehousesLiveSuite) page(resp *http.Response) string {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	return string(body)
}

// ---- TS-031 ----

// TS-031 (DS-021, DS-020, DS-026): пустой склад удаляется — 200 {id, deleted_at}; отметка «кто,
// когда, почему» записана, причина обрезана, признак архива и позиция не изменились; для обычных
// адресов склада больше нет: списки (при любом фильтре архива), PUT, повторное удаление,
// серверные страница и архив — 404. Нулевая строка остатка удалению не мешает.
func (s *WarehousesLiveSuite) TestTS031_DeleteEmptyWarehouse() {
	m := sdMark()
	w := s.create("TS031 Пустой " + m)
	s.sdStock(s.sdProduct(false), w.ID, "0") // нулевая строка остатка — не «товар на складе»

	before := time.Now().Add(-time.Minute)
	st, d, raw := s.deleteWh(s.admin, w.ID, "  Создан по ошибке  ")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(w.ID, d.ID)
	s.True(d.DeletedAt.After(before), "deleted_at — момент удаления")
	s.Equal(time.UTC, d.DeletedAt.Location(), "deleted_at — UTC")
	s.Equal(fmt.Sprintf("true|true|%d|Создан по ошибке|%s", w.Position, s.sdAdminID()), s.whState(w.ID))

	s.False(s.listed(w.ID), "удалённого склада нет в списке и с архивными")
	_, ok := s.byName(s.list(""), w.Name)
	s.False(ok)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": false})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("WAREHOUSE_NOT_FOUND", s.sdErr(raw).Code)
	st, _, raw = s.deleteWh(s.admin, w.ID, "Повтор")
	s.Equal(http.StatusNotFound, st, "повтор удаления: %s", raw)
	s.Equal("WAREHOUSE_NOT_FOUND", s.sdErr(raw).Code)
	s.Equal("Склад не найден", s.sdErr(raw).Message)

	for _, path := range []string{"", "/archive", "/restore", "/delete"} {
		method, form := http.MethodPost, any(url.Values{"reason": {"Повтор"}, "name": {"X"}})
		if path == "" {
			method, form = http.MethodGet, nil
		}
		resp := s.admin.Request(method, "/warehouses/"+w.ID+path, form, nil)
		resp.Body.Close()
		s.Equal(http.StatusNotFound, resp.StatusCode, "%s /warehouses/:id%s удалённого склада", method, path)
	}
	s.Equal(fmt.Sprintf("true|true|%d|Создан по ошибке|%s", w.Position, s.sdAdminID()), s.whState(w.ID), "отметка удаления не тронута")
}

// TS-031 (DS-021): склад с товаром, с оборудованием, с незавершённым документом не удаляется —
// 409 WAREHOUSE_IN_USE с ТРЕМЯ РАЗНЫМИ сообщениями. Остаток считается и отрицательный (долг), и
// у товара, находящегося в удалённых; документ — и «Заявка», и «В пути», склад в нём — и
// отправитель, и получатель. Негатив: склад только с завершёнными документами и нулевым остатком
// удаляется.
func (s *WarehousesLiveSuite) TestTS031_DeleteRefusedWhileInUse() {
	f := s.newFixture()
	defer s.cleanup(f)
	m := sdMark()
	src, dst := s.create("TS031 Источник "+m), s.create("TS031 Получатель "+m)

	// товар: положительный остаток действующего товара, долг, остаток удалённого товара.
	for name, tc := range map[string]struct {
		deleted bool
		stock   string
	}{"остаток": {false, "3"}, "долг": {false, "-2"}, "остаток удалённого товара": {true, "1.500"}} {
		w := s.create("TS031 Товар " + sdMark())
		p := s.sdProduct(tc.deleted)
		s.sdStock(p, w.ID, tc.stock)
		s.refused(w.ID, "WAREHOUSE_IN_USE", msgHasStock, name)
		s.sdStock(p, w.ID, "0")
		s.mustDeleteWh(w.ID) // после списания склад удаляется
	}

	// оборудование на складе.
	unit := s.unitAt(f, src.ID)
	s.refused(src.ID, "WAREHOUSE_IN_USE", msgHasEquipment, "единица на складе")

	// заявка: получатель (на нём ни товара, ни оборудования) занят только документом.
	doc := s.transfer(src.ID, dst.ID, unit, false)
	s.refused(dst.ID, "WAREHOUSE_IN_USE", msgHasTransfers, "получатель заявки")

	// в пути: единица уехала с источника — оба склада заняты только документом.
	doc = s.transferAction(doc, "send")
	s.refused(src.ID, "WAREHOUSE_IN_USE", msgHasTransfers, "отправитель документа «в пути»")
	s.refused(dst.ID, "WAREHOUSE_IN_USE", msgHasTransfers, "получатель документа «в пути»")

	// получено: источник свободен (только завершённый документ) — удаляется; на получателе единица.
	s.transferAction(doc, "receive")
	s.refused(dst.ID, "WAREHOUSE_IN_USE", msgHasEquipment, "получатель с принятой единицей")
	s.mustDeleteWh(src.ID)
	s.True(strings.HasPrefix(s.whState(src.ID), "true|"), "склад с завершёнными документами удалён")
}

// TS-031 (DS-021, abuse case 4): последний активный склад не удаляется — 409
// WAREHOUSE_LAST_ACTIVE; архивный склад удаляется без этой проверки, даже когда активный всего
// один; при двух активных один из них удаляется.
func (s *WarehousesLiveSuite) TestTS031_LastActiveCannotBeDeleted() {
	m := sdMark()
	last, second, archived := s.create("TS031 Последний "+m), s.create("TS031 Второй "+m), s.create("TS031 Архивный "+m)
	st, code := s.archive(archived)
	s.Require().Equal(http.StatusOK, st, code)
	back := s.onlyActive(last.ID, second.ID)
	defer back()

	// два активных: удаление одного проходит.
	s.mustDeleteWh(second.ID)
	// один активный: его удалить нельзя.
	s.refused(last.ID, "WAREHOUSE_LAST_ACTIVE", msgLastActive, "последний активный")
	s.Len(s.list("").Items, 1, "активный склад остался")
	// архивный удаляется и при единственном активном.
	s.mustDeleteWh(archived.ID)
	s.True(strings.HasPrefix(s.whState(archived.ID), "true|false|"), "архивный склад удалён, признак архива прежний")
	s.Equal(last.ID, s.sdScalar(`SELECT default_warehouse_id()::text`), "склад по умолчанию — оставшийся активный")
}

// ---- TS-057 ----

// TS-057 (DS-044): склад, на который ссылается только МЯГКО УДАЛЁННАЯ заявка, архивируется и
// удаляется; пока заявка действует — 409 WAREHOUSE_IN_USE в обоих случаях (негатив).
func (s *WarehousesLiveSuite) TestTS057_DeletedRequestDoesNotHoldWarehouse() {
	f := s.newFixture()
	defer s.cleanup(f)
	m := sdMark()
	src, dst := s.create("TS057 Источник "+m), s.create("TS057 Получатель "+m)
	doc := s.transfer(src.ID, dst.ID, s.unitAt(f, src.ID), false)

	// негатив: действующая заявка держит склад.
	st, code := s.archive(dst)
	s.Equal(http.StatusConflict, st)
	s.Equal("WAREHOUSE_IN_USE", code)
	s.refused(dst.ID, "WAREHOUSE_IN_USE", msgHasTransfers, "действующая заявка")

	st, raw := s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/transfers/%d", doc.ID), map[string]any{"reason": "Создана по ошибке", "version": doc.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, code = s.archive(dst)
	s.Require().Equal(http.StatusOK, st, "склад удалённой заявки архивируется: %s", code)
	s.False(s.isActive(dst.ID))
	s.restore(dst)
	s.mustDeleteWh(dst.ID)
	s.True(strings.HasPrefix(s.whState(dst.ID), "true|"), "склад удалённой заявки удалён")
}

// ---- DS-017, DS-009: название и восстановление ----

// DS-017, DS-009 (§6.2): название удалённого склада свободно — создание с тем же названием
// проходит; восстановление при занятом названии → 409 NAME_TAKEN, склад остаётся удалённым; после
// переименования действующего — 200 {id}, отметка стёрта, позиция прежняя. Повтор восстановления
// → 409 NOT_DELETED; несуществующий склад → 404. Склад, удалённый из архива, возвращается в
// архив. Негатив (as-is): название действующего склада при создании по-прежнему занято — 400.
func (s *WarehousesLiveSuite) TestNameFreeAfterDelete_Restore() {
	m := sdMark()
	name := "Склад имя " + m
	first := s.create(name)
	s.mustDeleteWh(first.ID)

	twin := s.create("  " + strings.ToUpper(name) + " ") // название удалённого свободно (без учёта регистра)
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": name})
	s.Equal(http.StatusBadRequest, st, "название действующего занято: %s", raw)
	s.Equal(map[string]string{"name": "taken"}, s.fields(raw))

	st, raw = s.restoreDeleted(s.admin, first.ID)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	s.Equal("NAME_TAKEN", s.sdErr(raw).Code)
	s.Equal("Название занято — переименуйте действующую запись", s.sdErr(raw).Message)
	s.True(strings.HasPrefix(s.whState(first.ID), "true|"), "склад остался удалённым")

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+twin.ID, map[string]any{"name": name + " второй", "is_active": true})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.restoreDeleted(s.admin, first.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var r sdRestoreDTO
	s.Require().NoError(json.Unmarshal(raw, &r), string(raw))
	s.Equal(first.ID, r.ID)
	s.Equal(fmt.Sprintf("false|true|%d||", first.Position), s.whState(first.ID), "отметка стёрта, архив и позиция прежние")
	s.True(s.listed(first.ID))

	st, raw = s.restoreDeleted(s.admin, first.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NOT_DELETED", s.sdErr(raw).Code)
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		st, raw = s.restoreDeleted(s.admin, id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("WAREHOUSE_NOT_FOUND", s.sdErr(raw).Code, id)
	}

	// удалён из архива → восстановлен в архив (DS-009).
	arch := s.create("Склад архив " + m)
	st, code := s.archive(arch)
	s.Require().Equal(http.StatusOK, st, code)
	s.mustDeleteWh(arch.ID)
	st, raw = s.restoreDeleted(s.admin, arch.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(fmt.Sprintf("false|false|%d||", arch.Position), s.whState(arch.ID), "склад вернулся в архив")
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): исполнитель → 403 «Нет доступа» на удалении и восстановлении —
// до поиска записи (существующий, несуществующий и битый идентификатор) и до проверки причины;
// серверная форма удаления — страница отказа 403. Склад не изменён. Негатив: без сессии → 401.
func (s *WarehousesLiveSuite) TestTS033_SoftDeleteAdminOnly() {
	m := sdMark()
	w := s.create("TS033 Склад " + m)
	gone := s.create("TS033 Удалённый " + m)
	s.mustDeleteWh(gone.ID)
	exec := s.sdExecutor("wh-sd-exec-" + m)
	anon := s.NewBrowser()

	for _, id := range []string{w.ID, gone.ID, "00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		for _, rq := range []struct {
			method, path string
			body         any
		}{
			{http.MethodDelete, "/api/v1/warehouses/" + id, map[string]any{"reason": "Чужое"}},
			{http.MethodDelete, "/api/v1/warehouses/" + id, nil},
			{http.MethodPost, "/api/v1/warehouses/" + id + "/restore", nil},
		} {
			st, raw := exec.Do(rq.method, rq.path, rq.body)
			s.Equal(http.StatusForbidden, st, "%s %s: %s", rq.method, rq.path, raw)
			s.Equal("Нет доступа", s.sdErr(raw).Message, rq.path)
			st, raw = anon.Do(rq.method, rq.path, rq.body)
			s.Equal(http.StatusUnauthorized, st, "без сессии %s %s: %s", rq.method, rq.path, raw)
		}
	}
	resp := exec.Request(http.MethodPost, "/warehouses/"+w.ID+"/delete", url.Values{"reason": {"Чужое"}}, nil)
	body := s.page(resp)
	s.Equal(http.StatusForbidden, resp.StatusCode)
	s.Contains(body, "Нет доступа")

	s.True(strings.HasPrefix(s.whState(w.ID), "false|"), "склад исполнителем не удалён")
	s.True(strings.HasPrefix(s.whState(gone.ID), "true|"), "склад исполнителем не восстановлен")
}

// ---- причина и серверная форма ----

// DS-020 (abuse case 5): причина обязательна — нет тела, null, пробелы → 400 reason: required;
// 501 символ → too_long; NUL → invalid_text; склад не удалён. 500 символов — допустимо.
func (s *WarehousesLiveSuite) TestDeleteReasonValidation() {
	w := s.create("Причина " + sdMark())
	for name, tc := range map[string]struct {
		reason any
		want   string
	}{
		"нет тела": {nil, "required"}, "пробелы": {"   ", "required"}, "пусто": {"", "required"},
		"501 символ": {strings.Repeat("я", 501), "too_long"}, "NUL": {"при\u0000чина", "invalid_text"},
	} {
		st, _, raw := s.deleteWh(s.admin, w.ID, tc.reason)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal(map[string]string{"reason": tc.want}, s.fields(raw), name)
	}
	s.True(strings.HasPrefix(s.whState(w.ID), "false|"), "склад не удалён")
	st, _, raw := s.deleteWh(s.admin, w.ID, strings.Repeat("я", 500))
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// §6.5, §8.1 (DS-032): серверная форма POST /warehouses/:id/delete. Кнопка «Удалить» — в строке
// склада; отказ по причине — список (422) с окном «Удаление» этой строки и текстом под полем;
// отказ 409 — переход к списку с красным блоком и текстом, что мешает; успех — переход к списку
// с тостом, строки склада нет.
func (s *WarehousesLiveSuite) TestSSR_DeleteForm() {
	m := sdMark()
	w, busy := s.create("SSR-удаление "+m), s.create("SSR-занятый "+m)
	p := s.sdProduct(false)
	s.sdStock(p, busy.ID, "2")
	defer s.sdStock(p, busy.ID, "0")

	list := s.page(s.admin.Request(http.MethodGet, "/warehouses", nil, nil))
	s.Contains(list, `action="/warehouses/`+w.ID+`/delete"`)
	s.Contains(list, `data-testid="warehouse-delete"`)
	s.Contains(list, "Удалить можно только пустой склад: без товара, оборудования и незавершённых перемещений")
	s.NotContains(list, "data-delete-error", "без отказа окно само не открывается")

	resp := s.admin.Request(http.MethodPost, "/warehouses/"+w.ID+"/delete", url.Values{"reason": {"   "}}, nil)
	body := s.page(resp)
	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)
	s.Equal(1, strings.Count(body, `data-delete-error="Укажите причину удаления"`), "ошибка — у формы именно этого склада")
	s.Contains(body, w.Name)
	s.True(strings.HasPrefix(s.whState(w.ID), "false|"))

	resp = s.admin.Request(http.MethodPost, "/warehouses/"+busy.ID+"/delete", url.Values{"reason": {"Закрыт"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/warehouses?error=delete_stock", resp.Header.Get("Location"))
	body = s.page(s.admin.Request(http.MethodGet, "/warehouses?error=delete_stock", nil, nil))
	s.Contains(body, `data-testid="list-error"`)
	s.Contains(body, msgHasStock)

	resp = s.admin.Request(http.MethodPost, "/warehouses/"+w.ID+"/delete", url.Values{"reason": {"Создан по ошибке"}, "archived": {"1"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/warehouses?archived=1&flash=deleted", resp.Header.Get("Location"))
	body = s.page(s.admin.Request(http.MethodGet, "/warehouses?archived=1&flash=deleted", nil, nil))
	s.Contains(body, `data-kt-toast="Удалено. Восстановить можно в разделе «Удалённые»"`)
	s.NotContains(body, w.Name, "удалённого склада в списке нет")
	s.True(strings.HasPrefix(s.whState(w.ID), "true|"))
}

// ---- служебная уборка ----

// DS-027 (§6.5): DELETE /api/v1/test/purge/warehouses/:id (TEST_PURGE_ENABLED=true) — действующий
// и мягко удалённый склад стираются физически вместе с нулевыми строками остатков и своим
// журналом (stock_movements); склад, на который ссылаются другие строки (документ, ненулевой
// остаток), → 409 CONFLICT «Запись используется» и остаётся вместе с журналом; нет склада либо
// битый идентификатор → 404 NOT_FOUND. Негатив: исполнитель → 403.
func (s *WarehousesLiveSuite) TestPurge_Warehouses() {
	f := s.newFixture()
	defer s.cleanup(f)
	m := sdMark()
	purge := func(b *testutil.Browser, id string) (int, []byte) {
		return b.Do(http.MethodDelete, "/api/v1/test/purge/warehouses/"+id, nil)
	}
	product := s.sdProduct(false)
	journal := func(warehouseID string) string {
		return s.sdScalar(`SELECT count(*)::text FROM stock_movements WHERE warehouse_id = $1::uuid`, warehouseID)
	}
	addJournal := func(warehouseID string) {
		s.sdExec(`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1::uuid, $2::uuid, 1, 'manual', $3::uuid)`,
			product, warehouseID, s.sdAdminID())
	}
	defer s.sdExec(`DELETE FROM stock_movements WHERE product_id = $1::uuid`, product)
	held, other := s.create("Purge С документом "+m), s.create("Purge Источник "+m)
	doc := s.transfer(other.ID, held.ID, s.unitAt(f, other.ID), false)
	s.transferAction(doc, "cancel")
	addJournal(held.ID)

	st, raw := purge(s.sdExecutor("wh-purge-exec-"+m), held.ID)
	s.Equal(http.StatusForbidden, st, string(raw))
	st, raw = purge(s.admin, held.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CONFLICT", s.sdErr(raw).Code)
	s.Equal("Запись используется", s.sdErr(raw).Message)
	s.NotEmpty(s.whState(held.ID), "склад с документом на месте")
	s.Equal("1", journal(held.ID), "отказ уборки журнал склада не трогает")

	// ненулевой остаток — ссылка: журнал её не заменяет, склад остаётся.
	stocked := s.create("Purge С остатком " + m)
	s.sdStock(product, stocked.ID, "2")
	addJournal(stocked.ID)
	st, raw = purge(s.admin, stocked.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("1", journal(stocked.ID), "склад с остатком и его журнал на месте")
	s.sdStock(product, stocked.ID, "0")

	live, gone := s.create("Purge Свободный "+m), s.create("Purge Удалённый "+m)
	s.sdStock(product, live.ID, "0")
	addJournal(live.ID) // журнал склада уборке не мешает — уходит вместе со складом
	addJournal(gone.ID)
	s.mustDeleteWh(gone.ID)
	for _, id := range []string{stocked.ID, live.ID, gone.ID} {
		st, raw = purge(s.admin, id)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Empty(s.whState(id), "строка склада физически удалена")
		s.Equal("0", journal(id), "журнал убранного склада стёрт")
		st, raw = purge(s.admin, id)
		s.Equal(http.StatusNotFound, st, string(raw))
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
	}
	st, raw = purge(s.admin, "not-uuid")
	s.Equal(http.StatusNotFound, st, string(raw))
}

// ---- гонки (§3 «Конкурентность») ----

func (s *WarehousesLiveSuite) lockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

type raceResult struct {
	status int
	raw    []byte
}

// race — запрос req уходит, пока тест держит блокировку в tx; по pg_stat_activity тест
// дожидается, что запрос встал в ожидание, затем фиксирует tx и возвращает ответ. Без sleep.
func (s *WarehousesLiveSuite) race(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) raceResult {
	ctx := context.Background()
	commit := hold(ctx)
	base := s.lockWaiters()
	done := make(chan raceResult, 1)
	go func() {
		st, raw := req()
		done <- raceResult{st, raw}
	}()
	s.Require().Eventually(func() bool { return s.lockWaiters() > base }, 10*time.Second, 20*time.Millisecond,
		"запрос ждёт блокировку")
	select {
	case res := <-done:
		s.FailNow("запрос не дождался блокировки", "%d %s", res.status, res.raw)
	default:
	}
	commit()
	return <-done
}

func (s *WarehousesLiveSuite) delReq(id string) func() (int, []byte) {
	return func() (int, []byte) {
		st, _, raw := s.deleteWh(s.admin, id, "Гонка")
		return st, raw
	}
}

// DS-024: удаление склада идёт (строка под FOR UPDATE, отметка поставлена, транзакция не
// зафиксирована) — второй запрос удаления обязан дождаться и получить 404; отметка первого не
// перезаписана.
func (s *WarehousesLiveSuite) TestRace_SecondDeleteGets404() {
	w := s.create("Гонка двойное " + sdMark())
	admin := s.sdAdminID()

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, w.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE warehouses SET deleted_at = now(), deleted_by = $2, delete_reason = 'Первый' WHERE id = $1`, w.ID, admin)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, s.delReq(w.ID))
	s.Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("WAREHOUSE_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("Первый", s.sdScalar(`SELECT delete_reason FROM warehouses WHERE id = $1`, w.ID), "отметка первого удаления не перезаписана")
}

// DS-021, abuse case 3: приход товара на склад идёт (строка склада под FOR SHARE — как её берут
// корректировка, правка остатков товара и документ, — остаток записан, транзакция не
// зафиксирована) — удаление склада обязано дождаться на строке склада (FOR UPDATE) и затем
// увидеть остаток: 409 WAREHOUSE_IN_USE «На складе есть товар…». Удаление, не блокирующее строку
// склада, не увидело бы незафиксированный остаток и удалило бы склад с товаром.
func (s *WarehousesLiveSuite) TestRace_DeleteWaitsForStockArrival() {
	w := s.create("Гонка приход " + sdMark())
	p := s.sdProduct(false)
	defer s.sdStock(p, w.ID, "0")

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, w.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, 5)`, p, w.ID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, s.delReq(w.ID))
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("WAREHOUSE_IN_USE", s.sdErr(res.raw).Code)
	s.Equal(msgHasStock, s.sdErr(res.raw).Message)
	s.True(strings.HasPrefix(s.whState(w.ID), "false|"), "склад не удалён")
}

// §3 п. 1 (DS-021): два последних активных склада — один архивируется (активные склады под FOR
// UPDATE по возрастанию id, запись сделана, транзакция не зафиксирована), второй в это окно
// удаляется. Удаление обязано дождаться архивирования на той же блокировке активных складов и
// получить 409 WAREHOUSE_LAST_ACTIVE: активный склад остаётся. Удаление, блокирующее только свою
// строку, не увидело бы незафиксированный архив и оставило бы систему без склада по умолчанию.
func (s *WarehousesLiveSuite) TestRace_DeleteWaitsForArchiveOfOtherLastActive() {
	m := sdMark()
	a, b := s.create("Гонка последний А "+m), s.create("Гонка последний Б "+m)
	back := s.onlyActive(a.ID, b.ID)
	defer back()
	defer func() { // вернуть архивированный в гонке склад в актив
		if !s.isActive(a.ID) {
			s.restore(a)
		}
	}()

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT id FROM warehouses WHERE is_active AND deleted_at IS NULL ORDER BY id FOR UPDATE`)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE warehouses SET is_active = false WHERE id = $1`, a.ID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, s.delReq(b.ID))
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("WAREHOUSE_LAST_ACTIVE", s.sdErr(res.raw).Code)
	s.Equal(msgLastActive, s.sdErr(res.raw).Message)
	s.Len(s.list("").Items, 1, "активный склад остался")
}

// §3 п. 4 (DS-017): создание склада с названием удалённого идёт (строка вставлена, транзакция не
// зафиксирована) — восстановление удалённого склада обязано дождаться на частичном уникальном
// индексе и получить 409 NAME_TAKEN: ровно один из двух получает название.
func (s *WarehousesLiveSuite) TestRace_RestoreWaitsForCreateSameName() {
	name := "Гонка имя " + sdMark()
	w := s.create(name)
	s.mustDeleteWh(w.ID)

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `INSERT INTO warehouses (name, position) VALUES ($1, 900)`, strings.ToUpper(name))
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.restoreDeleted(s.admin, w.ID) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("NAME_TAKEN", s.sdErr(res.raw).Code)
	s.True(strings.HasPrefix(s.whState(w.ID), "true|"), "склад остался удалённым")
}
