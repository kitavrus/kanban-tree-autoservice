//go:build integration

package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// PurchaseOrdersLiveSuite (§TD-8: боевой бинарь, реальный вход) — закупки по дефициту
// (docs/features/purchase-orders/draft-spec.md §11): TS-003…TS-007, TS-009…TS-013 + PDF,
// SSR-экраны, «Удалённые». Wire-типы — models/dto фичи (§TD-6.2).
type PurchaseOrdersLiveSuite struct {
	testutil.LiveSuite
	admin, execA, execB, supA, supB *testutil.Browser
	adminID, execAID, execBID       string
	supplierA, supplierB, supplierX string // X — поставщик без входа
	supAUserID                      string
	column, category, service       string
}

func TestPurchaseOrdersLiveSuite(t *testing.T) {
	suite.Run(t, &PurchaseOrdersLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server",
		ExtraEnv: []string{"TEST_PURGE_ENABLED=true", "FIRM_NAME=СТО Тест", "FIRM_PHONE=+7 700 000 00 00"}}})
}

const poAPI = "/api/v1/purchase-orders"
const spAPI = "/api/v1/supplier/purchase-orders"

func mark() string { return uuid.NewString()[:8] }

func (s *PurchaseOrdersLiveSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *PurchaseOrdersLiveSuite) scalar(q string, args ...any) string {
	var v *string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	if v == nil {
		return "<nil>"
	}
	return *v
}

func (s *PurchaseOrdersLiveSuite) newSupplier(name string) string {
	return s.scalar(`INSERT INTO suppliers (name, phone) VALUES ($1, '+7 701 111 22 33') RETURNING id::text`, name)
}

func (s *PurchaseOrdersLiveSuite) login(login, role string) (*testutil.Browser, string) {
	id := s.SeedUser(login, "password1", "Пользователь "+login, role, true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b, id
}

func (s *PurchaseOrdersLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.admin, s.adminID = s.login("po-admin", "admin")
	s.execA, s.execAID = s.login("po-exec-a", "executor")
	s.execB, s.execBID = s.login("po-exec-b", "executor")
	m := mark()
	s.supplierA = s.newSupplier("АвтоДеталь Плюс " + m)
	s.supplierB = s.newSupplier("Второй поставщик " + m)
	s.supplierX = s.newSupplier("Без кабинета " + m)
	s.supAUserID = s.SeedSupplierUser("po-sup-a", "password1", "Продавец Ерлан", s.supplierA, true)
	s.SeedSupplierUser("po-sup-b", "password1", "Продавец Б", s.supplierB, true)
	s.supA = s.NewBrowser()
	s.supA.Login("po-sup-a", "password1")
	s.supB = s.NewBrowser()
	s.supB.Login("po-sup-b", "password1")

	// Колонка «принят, не в работе» (не consumes_stock, не финальная) — даёт pending (TS-F1).
	s.column = s.scalar(`INSERT INTO columns (name, color, position) VALUES ('Закупки-тест', '#123456',
		(SELECT coalesce(max(position), 0) + 1 FROM columns)) RETURNING id::text`)
	s.category = s.scalar(`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'Закупки ' || $1) RETURNING id::text`, m)
	s.service = s.scalar(`INSERT INTO services (category_id, name) VALUES ($1, 'Замена') RETURNING id::text`, s.category)
}

// product — запчасть со складом stock и минимумом minStock (SQL-фикстура каталога).
func (s *PurchaseOrdersLiveSuite) product(name, stock, minStock string) string {
	return s.scalar(`INSERT INTO products (name, sku, unit, stock, min_stock) VALUES ($1, 'SKU-' || $1, 'шт', $2::numeric, $3::numeric)
		RETURNING id::text`, name+" "+mark(), stock, minStock)
}

// pending — заказ клиента в колонке без consumes_stock, которому нужна запчасть (pending).
func (s *PurchaseOrdersLiveSuite) pending(productID, qty string) int64 {
	client := s.scalar(`INSERT INTO clients (company_name) VALUES ('Клиент ' || $1) RETURNING id::text`, mark())
	car := s.scalar(`INSERT INTO equipment (client_id, plate_number) VALUES ($1::uuid, $2) RETURNING id::text`, client, "PO"+mark()[:6])
	var task int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by, start_date, duration_days)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'Ремонт ' || $4, 1, $5::uuid, current_date, 1) RETURNING id`, s.column, client, car, mark(), s.adminID).Scan(&task))
	sub := s.scalar(`INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2::uuid) RETURNING id::text`, task, s.service)
	s.exec(`INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1::uuid, $2::uuid, $3::numeric)`, sub, productID, qty)
	return task
}

func ptr(v string) *string { return &v }

func line(productID, qty string) dto.ItemRequest {
	return dto.ItemRequest{ProductID: productID, Qty: dto.Num(qty)}
}

func (s *PurchaseOrdersLiveSuite) create(b *testutil.Browser, supplierID *string, items ...dto.ItemRequest) (int, dto.Order, []byte) {
	st, raw := b.Do(http.MethodPost, poAPI, dto.CreateRequest{SupplierID: supplierID, Note: "Забрать до 17:00", Items: items})
	var o dto.Order
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &o), string(raw))
	}
	return st, o, raw
}

func (s *PurchaseOrdersLiveSuite) mustCreate(b *testutil.Browser, supplierID *string, items ...dto.ItemRequest) dto.Order {
	st, o, raw := s.create(b, supplierID, items...)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return o
}

func (s *PurchaseOrdersLiveSuite) get(b *testutil.Browser, id string) dto.Order {
	st, raw := b.Do(http.MethodGet, poAPI+"/"+id, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var o dto.Order
	s.Require().NoError(json.Unmarshal(raw, &o))
	return o
}

func (s *PurchaseOrdersLiveSuite) act(b *testutil.Browser, id, action string, version int64) (int, dto.Order, []byte) {
	st, raw := b.Do(http.MethodPost, poAPI+"/"+id+"/"+action, dto.VersionRequest{Version: version})
	var o dto.Order
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &o), string(raw))
	}
	return st, o, raw
}

func (s *PurchaseOrdersLiveSuite) mustAct(b *testutil.Browser, id, action string, version int64) dto.Order {
	st, o, raw := s.act(b, id, action, version)
	s.Require().Equal(http.StatusOK, st, "%s: %s", action, raw)
	return o
}

type errDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details struct {
		Fields []struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"fields"`
		ItemIDs []string `json:"item_ids"`
		Numbers []string `json:"numbers"`
	} `json:"details"`
}

func (s *PurchaseOrdersLiveSuite) err(raw []byte) errDTO {
	var e errDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *PurchaseOrdersLiveSuite) field(raw []byte) map[string]string {
	m := map[string]string{}
	for _, f := range s.err(raw).Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *PurchaseOrdersLiveSuite) expectErr(st int, raw []byte, wantStatus int, wantCode, wantMsg string) {
	s.Equal(wantStatus, st, string(raw))
	e := s.err(raw)
	s.Equal(wantCode, e.Code, string(raw))
	if wantMsg != "" {
		s.Equal(wantMsg, e.Message)
	}
}

func (s *PurchaseOrdersLiveSuite) answer(b *testutil.Browser, orderID, itemID string, version int64, qty, price *string, note string) (int, []byte) {
	body := map[string]any{"version": version, "supplier_qty": qty, "supplier_price": price, "supplier_note": note}
	return b.Do(http.MethodPut, spAPI+"/"+orderID+"/items/"+itemID, body)
}

func (s *PurchaseOrdersLiveSuite) supplierOrder(b *testutil.Browser, id string) dto.SupplierOrder {
	st, raw := b.Do(http.MethodGet, spAPI+"/"+id, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var o dto.SupplierOrder
	s.Require().NoError(json.Unmarshal(raw, &o))
	return o
}

// sentOrder — заказ поставщику A от автора b с n строками, отправлен.
func (s *PurchaseOrdersLiveSuite) sentOrder(b *testutil.Browser, n int) dto.Order {
	items := make([]dto.ItemRequest, n)
	for i := range items {
		items[i] = line(s.product(fmt.Sprintf("Колодки %d", i), "0", "0"), "4")
	}
	o := s.mustCreate(b, &s.supplierA, items...)
	return s.mustAct(b, o.ID, "send", o.Version)
}

// ---- TS-003: матрица переходов §5 ----

// TS-003 (DS-011, DS-012, DS-042, DS-075): полный круг draft → sent → draft → sent → ready →
// picked_up через API; каждый запрещённый переход → 409 со своим кодом.
func (s *PurchaseOrdersLiveSuite) TestTS003_TransitionMatrix() {
	o := s.sentOrder(s.admin, 1)
	s.Equal("sent", o.Status)
	s.NotNil(o.SentAt)

	st, _, raw := s.act(s.admin, o.ID, "send", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Заказ уже отправлен")
	st, _, raw = s.act(s.admin, o.ID, "pick-up", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_NOT_READY", "Дождитесь отметки поставщика «Собрано»")

	o = s.mustAct(s.admin, o.ID, "recall", o.Version)
	s.Equal("draft", o.Status)
	st, _, raw = s.act(s.admin, o.ID, "recall", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Отозвать можно только отправленный заказ")
	st, _, raw = s.act(s.admin, o.ID, "pick-up", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_NOT_READY", "Сначала отправьте заказ поставщику")
	o = s.mustAct(s.admin, o.ID, "send", o.Version)

	// поставщик отвечает и «Собрано»
	st, raw = s.answer(s.supA, o.ID, o.Items[0].ID, o.Items[0].Version, ptr("4"), ptr("100"), "")
	s.Require().Equal(200, st, string(raw))
	st, raw = s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.Require().Equal(200, st, string(raw))
	o = s.get(s.admin, o.ID)
	s.Equal("ready", o.Status)
	s.False(o.CanEdit)

	st, _, raw = s.act(s.admin, o.ID, "recall", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Поставщик уже собрал заказ — отозвать нельзя")
	st, _, raw = s.act(s.admin, o.ID, "send", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Отправить можно только черновик")
	st, raw = s.admin.Do(http.MethodDelete, poAPI+"/"+o.ID, dto.ReasonRequest{Version: o.Version, Reason: ptr("x")})
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Удалить можно только черновик")

	o = s.mustAct(s.admin, o.ID, "pick-up", o.Version)
	s.Equal("picked_up", o.Status)
	s.NotNil(o.PickedUpAt)
	st, _, raw = s.act(s.admin, o.ID, "pick-up", o.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Заказ уже закрыт")
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+o.ID+"/cancel", dto.ReasonRequest{Version: o.Version, Reason: ptr("Поздно")})
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Заказ уже закрыт — отмена недоступна")

	// Отмена из ready и повторная отмена.
	c := s.sentOrder(s.admin, 1)
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+c.ID+"/cancel", dto.ReasonRequest{Version: c.Version, Reason: ptr("  ")})
	s.Equal(400, st, string(raw))
	s.Equal("required", s.field(raw)["reason"])
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+c.ID+"/cancel", dto.ReasonRequest{Version: c.Version, Reason: ptr("Нашли дешевле")})
	s.Require().Equal(200, st, string(raw))
	var cancelled dto.Order
	s.Require().NoError(json.Unmarshal(raw, &cancelled))
	s.Equal("cancelled", cancelled.Status)
	s.Equal("Нашли дешевле", *cancelled.CancelReason)
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+c.ID+"/cancel", dto.ReasonRequest{Version: cancelled.Version, Reason: ptr("ещё")})
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Заказ уже отменён")
	st, _, raw = s.act(s.admin, c.ID, "pick-up", cancelled.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Заказ отменён")
	// у поставщика отменённого нет (DS-036)
	st, _ = s.supA.Do(http.MethodGet, spAPI+"/"+c.ID, nil)
	s.Equal(404, st)
}

// TS-003 негатив (DS-058, DS-077, DS-076, DS-042): Базар не отправляется; пустой заказ не
// отправить и не забрать; поставщик без входа — «Забрали» прямо из sent.
func (s *PurchaseOrdersLiveSuite) TestTS003_BazaarEmptyAndNoLogin() {
	b := s.mustCreate(s.admin, nil, line(s.product("Масло", "0", "0"), "2"))
	s.Equal("Базар (сами)", b.RecipientName)
	s.Nil(b.SupplierID)
	st, _, raw := s.act(s.admin, b.ID, "send", b.Version)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_NO_SUPPLIER", "")
	b = s.mustAct(s.admin, b.ID, "pick-up", b.Version)
	s.Equal("picked_up", b.Status)

	// Пустой заказ (только SQL — API не создаёт заказ без строк).
	emptyBazaar := s.scalar(`INSERT INTO purchase_orders (created_by) VALUES ($1::uuid) RETURNING id::text`, s.adminID)
	st, _, raw = s.act(s.admin, emptyBazaar, "pick-up", 1)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_EMPTY", "Добавьте хотя бы одну строку")
	emptySup := s.scalar(`INSERT INTO purchase_orders (created_by, supplier_id) VALUES ($1::uuid, $2::uuid) RETURNING id::text`, s.adminID, s.supplierA)
	st, _, raw = s.act(s.admin, emptySup, "send", 1)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_EMPTY", "")

	x := s.mustCreate(s.admin, &s.supplierX, line(s.product("Свеча", "0", "0"), "1"))
	s.False(x.SupplierHasLogin)
	x = s.mustAct(s.admin, x.ID, "send", x.Version)
	x = s.mustAct(s.admin, x.ID, "pick-up", x.Version)
	s.Equal("picked_up", x.Status)
}

// ---- TS-004: конкурентность ----

func (s *PurchaseOrdersLiveSuite) parallel(n int, fn func(i int) int) []int {
	out := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			out[i] = fn(i)
		}(i)
	}
	close(start)
	wg.Wait()
	return out
}

func count(codes []int, want int) int {
	n := 0
	for _, c := range codes {
		if c == want {
			n++
		}
	}
	return n
}

// TS-004 (DS-032): два PUT с одной версией → один 200, второй 409; «Собрано» и «Отозвать»
// одновременно → ровно один успех; два ответа по строке с одной версией → второй 409;
// устаревшая версия на каждом переходе → 409 VERSION_CONFLICT.
func (s *PurchaseOrdersLiveSuite) TestTS004_Concurrency() {
	o := s.mustCreate(s.admin, &s.supplierA, line(s.product("Ремень", "0", "0"), "1"))
	codes := s.parallel(2, func(i int) int {
		st, _ := s.admin.Do(http.MethodPut, poAPI+"/"+o.ID, dto.UpdateRequest{Version: o.Version, Note: fmt.Sprint("note ", i),
			Items: []dto.ItemRequest{{ID: &o.Items[0].ID, ProductID: o.Items[0].ProductID, Qty: "2"}}})
		return st
	})
	s.Equal(1, count(codes, 200), "%v", codes)
	s.Equal(1, count(codes, 409), "%v", codes)

	o = s.get(s.admin, o.ID)
	for _, a := range []string{"send", "pick-up", "recall"} {
		st, _, raw := s.act(s.admin, o.ID, a, o.Version-1)
		if a == "send" {
			s.expectErr(st, raw, 409, "PURCHASE_ORDER_VERSION_CONFLICT", "Заказ изменился, обновите страницу")
		} else {
			s.Equal(409, st, string(raw))
		}
	}
	st, raw := s.admin.Do(http.MethodPost, poAPI+"/"+o.ID+"/cancel", dto.ReasonRequest{Version: o.Version - 1, Reason: ptr("x")})
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_VERSION_CONFLICT", "")

	o = s.mustAct(s.admin, o.ID, "send", o.Version)
	it := o.Items[0]
	codes = s.parallel(2, func(i int) int {
		st, _ := s.answer(s.supA, o.ID, it.ID, it.Version, ptr(fmt.Sprint(i+1)), ptr("10"), "")
		return st
	})
	s.Equal(1, count(codes, 200), "%v", codes)
	s.Equal(1, count(codes, 409), "%v", codes)
	st, raw = s.answer(s.supA, o.ID, it.ID, it.Version, ptr("1"), ptr("10"), "")
	s.expectErr(st, raw, 409, "PURCHASE_ITEM_VERSION_CONFLICT", "Строку изменили, обновите страницу")

	codes = s.parallel(2, func(i int) int {
		if i == 0 {
			st, _ := s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
			return st
		}
		st, _, _ := s.act(s.admin, o.ID, "recall", o.Version)
		return st
	})
	s.Equal(1, count(codes, 200), "ровно один из «Собрано»/«Отозвать»: %v", codes)
	s.Equal(1, count(codes, 409)+count(codes, 404), "%v", codes)
}

// ---- TS-005: сброс ответа ----

// TS-005 (DS-034, DS-035, DS-048): в sent смена qty строки очищает её ответ и ставит
// «Изменено заказчиком»; смена примечания ответы не трогает, у поставщика «Изменён» и
// подсветка; recall + send без правок — ответы целы. Негатив: правка в ready → 409, ответы целы.
func (s *PurchaseOrdersLiveSuite) TestTS005_ResetAnswers() {
	o := s.sentOrder(s.admin, 2)
	for _, it := range o.Items {
		st, raw := s.answer(s.supA, o.ID, it.ID, it.Version, ptr("3"), ptr("2450"), "4-й в пятницу")
		s.Require().Equal(200, st, string(raw))
	}
	o = s.get(s.admin, o.ID)
	s.Require().NotNil(o.Items[0].SupplierQty)
	s.True(o.Items[0].HasDiscrepancy, "3 ≠ 4")
	s.Equal("7350.00", *o.Items[0].LineAmount)
	s.Equal("14700.00", *o.TotalAmount)
	v0 := o.Items[0].Version

	st, raw := s.admin.Do(http.MethodPut, poAPI+"/"+o.ID, dto.UpdateRequest{Version: o.Version, Note: "Новое примечание", Items: []dto.ItemRequest{
		{ID: &o.Items[0].ID, ProductID: o.Items[0].ProductID, Qty: "6"},
		{ID: &o.Items[1].ID, ProductID: o.Items[1].ProductID, Qty: "4"},
	}})
	s.Require().Equal(200, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &o))
	s.Nil(o.Items[0].SupplierQty)
	s.Nil(o.Items[0].SupplierPrice)
	s.Equal("", o.Items[0].SupplierNote)
	s.True(o.Items[0].CustomerChanged)
	s.Equal(v0+1, o.Items[0].Version)
	s.NotNil(o.Items[1].SupplierQty, "вторая строка с ответом")
	s.False(o.Items[1].CustomerChanged)
	s.Equal("sent", o.Status)

	so := s.supplierOrder(s.supA, o.ID)
	s.True(so.NoteChanged)
	s.Equal("Новое примечание", so.Note)
	s.True(so.Items[0].CustomerChanged)
	s.Nil(so.Items[0].SupplierQty)
	var list dto.SupplierListResponse
	st, raw = s.supA.Do(http.MethodGet, spAPI+"?status=sent", nil)
	s.Require().Equal(200, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &list))
	found := false
	for _, it := range list.Items {
		if it.ID == o.ID {
			found = true
			s.True(it.IsChanged)
			s.Equal(1, it.AnsweredCount)
			s.Equal(2, it.ItemsCount)
		}
	}
	s.True(found)

	// поставщик ответил заново — метка снимается
	st, raw = s.answer(s.supA, o.ID, so.Items[0].ID, so.Items[0].Version, ptr("6"), ptr("100"), "")
	s.Require().Equal(200, st, string(raw))
	var ans dto.AnswerResponse
	s.Require().NoError(json.Unmarshal(raw, &ans))
	s.False(ans.CustomerChanged)
	s.Equal("600.00", *ans.LineAmount)
	s.Require().NotNil(ans.TotalAmount)

	// recall + send без правок — ответы целы
	o = s.get(s.admin, o.ID)
	o = s.mustAct(s.admin, o.ID, "recall", o.Version)
	s.NotNil(o.Items[0].SupplierQty, "при «Отозвать» ответы сохраняются")
	st, _ = s.supA.Do(http.MethodGet, spAPI+"/"+o.ID, nil)
	s.Equal(404, st, "отозванный заказ скрыт из кабинета")
	o = s.mustAct(s.admin, o.ID, "send", o.Version)
	s.NotNil(o.Items[0].SupplierQty)
	s.NotNil(o.Items[1].SupplierQty)

	// Негатив: после «Собрано» правка → 409, ответы не тронуты.
	st, raw = s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.Require().Equal(200, st, string(raw))
	o = s.get(s.admin, o.ID)
	st, raw = s.admin.Do(http.MethodPut, poAPI+"/"+o.ID, dto.UpdateRequest{Version: o.Version, Items: []dto.ItemRequest{
		{ID: &o.Items[0].ID, ProductID: o.Items[0].ProductID, Qty: "1"}}})
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_READ_ONLY", "Поставщик уже собрал заказ — правка недоступна")
	after := s.get(s.admin, o.ID)
	s.Equal(o.Items, after.Items)
}

// §6.4: добавление/удаление строк, чужой id строки → 400, сведение дублей, новая строка в
// отправленном — «Изменено заказчиком».
func (s *PurchaseOrdersLiveSuite) TestUpdateLines() {
	o := s.sentOrder(s.admin, 2)
	newP := s.product("Новая", "0", "0")
	foreign := s.sentOrder(s.admin, 1)
	st, raw := s.admin.Do(http.MethodPut, poAPI+"/"+o.ID, dto.UpdateRequest{Version: o.Version, Items: []dto.ItemRequest{
		{ID: &foreign.Items[0].ID, ProductID: foreign.Items[0].ProductID, Qty: "1"}}})
	s.Equal(400, st, string(raw))
	s.Equal("invalid", s.field(raw)["items[0].id"])

	st, raw = s.admin.Do(http.MethodPut, poAPI+"/"+o.ID, dto.UpdateRequest{Version: o.Version, Items: []dto.ItemRequest{
		{ID: &o.Items[1].ID, ProductID: o.Items[1].ProductID, Qty: "4"},
		line(newP, "1"), line(newP, "2"),
	}})
	s.Require().Equal(200, st, string(raw))
	var u dto.Order
	s.Require().NoError(json.Unmarshal(raw, &u))
	s.Require().Len(u.Items, 2)
	s.Equal(o.Items[1].ID, u.Items[0].ID)
	s.Equal(newP, u.Items[1].ProductID)
	s.Equal("3.000", u.Items[1].Qty)
	s.Equal("manual", u.Items[1].Reason)
	s.True(u.Items[1].CustomerChanged, "новая строка после отправки")
	s.Equal(o.Version+1, u.Version)

	// архивная запчасть в новой строке → 400 «Запчасть недоступна»
	arch := s.product("Архивная", "0", "0")
	s.exec(`UPDATE products SET is_active = false WHERE id = $1::uuid`, arch)
	st, raw = s.admin.Do(http.MethodPut, poAPI+"/"+o.ID, dto.UpdateRequest{Version: u.Version, Items: []dto.ItemRequest{line(arch, "1")}})
	s.Equal(400, st, string(raw))
	s.Equal("unavailable", s.field(raw)["items[0].product_id"])
	s.Equal("Запчасть недоступна", s.err(raw).Message)
}

// §6.2: валидация создания через API (TS-001 на проводе) и created_by из сессии.
func (s *PurchaseOrdersLiveSuite) TestCreateValidation() {
	st, _, raw := s.create(s.admin, nil)
	s.Equal(400, st)
	s.Equal("required", s.field(raw)["items"])
	s.Equal("Добавьте хотя бы одну строку", s.err(raw).Message)
	st, _, raw = s.create(s.admin, ptr(uuid.NewString()), line(s.product("Х", "0", "0"), "1"))
	s.Equal(400, st)
	s.Equal("unavailable", s.field(raw)["supplier_id"])
	archived := s.newSupplier("Архивный " + mark())
	s.exec(`UPDATE suppliers SET is_active = false WHERE id = $1::uuid`, archived)
	st, _, raw = s.create(s.admin, &archived, line(s.product("Х", "0", "0"), "1"))
	s.Equal(400, st)
	s.Equal("Поставщик недоступен", s.err(raw).Message)
	st, _, raw = s.create(s.admin, nil, line(s.product("Х", "0", "0"), "0"))
	s.Equal(400, st)
	s.Equal("invalid", s.field(raw)["items[0].qty"])
	st, raw = s.admin.Do(http.MethodPost, poAPI, `{"supplier_id":null,"items":[`)
	s.Equal(400, st, string(raw))
}

// ---- TS-006 ----

// TS-006 (DS-038) + TS-002 негатив: «Собрано» с неотвеченной строкой → 409 с details.item_ids;
// qty > 0 без цены → 409; 0 без цены — допустимо; ответ в ready → 409 READ_ONLY.
func (s *PurchaseOrdersLiveSuite) TestTS006_ReadyIncomplete() {
	o := s.sentOrder(s.admin, 2)
	a, b := o.Items[0], o.Items[1]
	st, raw := s.answer(s.supA, o.ID, a.ID, a.Version, ptr("2"), nil, "")
	s.Require().Equal(200, st, string(raw))
	st, raw = s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_ITEMS_INCOMPLETE", "Заполните «Соберу» во всех строках")
	s.Equal([]string{b.ID}, s.err(raw).Details.ItemIDs)

	st, raw = s.answer(s.supA, o.ID, b.ID, b.Version, ptr("0"), nil, "нет в наличии")
	s.Require().Equal(200, st, string(raw))
	st, raw = s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_ITEMS_INCOMPLETE", "Укажите цену у строк, которые соберёте")
	s.Equal([]string{a.ID}, s.err(raw).Details.ItemIDs)

	so := s.supplierOrder(s.supA, o.ID)
	st, raw = s.answer(s.supA, o.ID, a.ID, so.Items[0].Version, ptr("2"), ptr("1250.5"), "")
	s.Require().Equal(200, st, string(raw))
	st, raw = s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.Require().Equal(200, st, string(raw))
	var ready dto.SupplierOrder
	s.Require().NoError(json.Unmarshal(raw, &ready))
	s.Equal("ready", ready.Status)
	s.False(ready.CanAnswer)
	s.Equal("2501.00", *ready.TotalAmount)

	st, raw = s.answer(s.supA, o.ID, a.ID, ready.Items[0].Version, ptr("1"), ptr("1"), "")
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_READ_ONLY", "Заказ уже отмечен «Собрано»")
	st, raw = s.supA.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_INVALID_STATUS", "Заказ уже отмечен «Собрано»")

	// валидация ответа на проводе (TS-002)
	o2 := s.sentOrder(s.admin, 1)
	st, raw = s.answer(s.supA, o2.ID, o2.Items[0].ID, o2.Items[0].Version, ptr("-1"), nil, "")
	s.Equal(400, st)
	s.Equal("invalid", s.field(raw)["supplier_qty"])
	s.Equal("Количество от 0 до 99 999.999", s.err(raw).Message)
	// supplier_id/order_id в теле игнорируются (abuse case 3)
	st, raw = s.supA.Do(http.MethodPut, spAPI+"/"+o2.ID+"/items/"+o2.Items[0].ID,
		map[string]any{"version": o2.Items[0].Version, "supplier_qty": "5", "supplier_price": "10", "supplier_id": s.supplierB, "order_id": uuid.NewString()})
	s.Equal(200, st, string(raw))
}

// ---- TS-007: подсказка дефицита ----

func findSuggestion(items []dto.Suggestion, pid string) *dto.Suggestion {
	for i := range items {
		if items[i].ProductID == pid {
			return &items[i]
		}
	}
	return nil
}

func (s *PurchaseOrdersLiveSuite) suggestions(b *testutil.Browser, query string) dto.SuggestionsResponse {
	st, raw := b.Do(http.MethodGet, poAPI+"/suggestions"+query, nil)
	s.Require().Equal(200, st, string(raw))
	var out dto.SuggestionsResponse
	s.Require().NoError(json.Unmarshal(raw, &out))
	return out
}

// TS-007 (DS-003, DS-004, DS-049, DS-078): to_order и reason по сидам; открытые заказы — только
// sent/ready; executor видит в in_open_orders только свои заказы, admin — все.
func (s *PurchaseOrdersLiveSuite) TestTS007_Suggestions() {
	pFor := s.product("Под заказы", "1", "2")
	task := s.pending(pFor, "3")
	pMin := s.product("Ниже минимума", "3", "5")
	s.pending(pMin, "1")
	pOk := s.product("Без дефицита", "10", "1")
	pAdmin := s.product("В заказе admin", "0", "1")
	pExec := s.product("В заказе executor", "0", "1")
	pDraft := s.product("В черновике", "0", "1")

	adm := s.mustCreate(s.admin, &s.supplierA, line(pAdmin, "5"))
	adm = s.mustAct(s.admin, adm.ID, "send", adm.Version)
	ex := s.mustCreate(s.execA, &s.supplierA, line(pExec, "2"))
	ex = s.mustAct(s.execA, ex.ID, "send", ex.Version)
	s.mustCreate(s.admin, &s.supplierA, line(pDraft, "7"))

	all := s.suggestions(s.admin, "")
	f := findSuggestion(all.Items, pFor)
	s.Require().NotNil(f)
	s.Equal("4.000", f.ToOrder)
	s.Equal("for_orders", f.Reason)
	s.Equal("3.000", f.Pending)
	s.Require().Len(f.Tasks, 1)
	s.Equal(task, f.Tasks[0].ID)
	s.True(f.Checked)
	m := findSuggestion(all.Items, pMin)
	s.Require().NotNil(m)
	s.Equal("3.000", m.ToOrder)
	s.Equal("below_min", m.Reason)
	s.Empty(m.Tasks)
	s.Nil(findSuggestion(all.Items, pOk), "to_order 0 — строки нет")

	a := findSuggestion(all.Items, pAdmin)
	s.Require().NotNil(a)
	s.Require().Len(a.InOpenOrders, 1)
	s.Equal(adm.NumberDisplay, a.InOpenOrders[0].NumberDisplay)
	s.Equal("5.000", a.InOpenOrders[0].Qty)
	s.False(a.Checked)
	s.Equal("1.000", a.ToOrder, "количество без вычета")
	d := findSuggestion(all.Items, pDraft)
	s.Require().NotNil(d)
	s.Empty(d.InOpenOrders, "черновик не учитывается")
	s.True(d.Checked)
	s.Len(findSuggestion(all.Items, pExec).InOpenOrders, 1, "admin видит и заказ executor")

	mine := s.suggestions(s.execA, "")
	a = findSuggestion(mine.Items, pAdmin)
	s.Require().NotNil(a)
	s.Empty(a.InOpenOrders, "чужой заказ executor'у не раскрывается")
	s.True(a.Checked)
	e := findSuggestion(mine.Items, pExec)
	s.Require().Len(e.InOpenOrders, 1)
	s.Equal(ex.ID, e.InOpenOrders[0].ID)
	s.False(e.Checked)

	onlyMin := s.suggestions(s.admin, "?reason=below_min")
	s.Nil(findSuggestion(onlyMin.Items, pFor))
	s.NotNil(findSuggestion(onlyMin.Items, pMin))
	st, raw := s.admin.Do(http.MethodGet, poAPI+"/suggestions?reason=zzz", nil)
	s.expectErr(st, raw, 400, "VALIDATION", "Неизвестный тип строки")

	// for_orders идут первыми
	seenMin := false
	for _, it := range all.Items {
		if it.Reason == "below_min" {
			seenMin = true
		} else {
			s.False(seenMin, "for_orders после below_min")
		}
	}
}

// ---- TS-008: схема ----

// TS-008 (DS-007, DS-015): CHECK users_supplier_chk и purchase_orders_bazaar_chk; номер — из
// sequence, растёт и не переиспользуется.
func (s *PurchaseOrdersLiveSuite) TestTS008_SchemaChecks() {
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role) VALUES ($1, 'x', 'x', 'supplier')`, "po-chk-"+mark())
	s.Error(err, "supplier без supplier_id")
	s.Contains(err.Error(), "users_supplier_chk")
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role, supplier_id) VALUES ($1, 'x', 'x', 'executor', $2::uuid)`, "po-chk-"+mark(), s.supplierA)
	s.Error(err, "executor с supplier_id")
	_, err = s.Pool.Exec(ctx, `INSERT INTO purchase_orders (created_by, status) VALUES ($1::uuid, 'sent')`, s.adminID)
	s.Error(err, "Базар в sent")
	s.Contains(err.Error(), "purchase_orders_bazaar_chk")

	o1 := s.mustCreate(s.admin, nil, line(s.product("Seq", "0", "0"), "1"))
	o2 := s.mustCreate(s.admin, nil, line(s.product("Seq", "0", "0"), "1"))
	s.Greater(o2.Number, o1.Number)
	s.Equal(fmt.Sprintf("З-%04d", o1.Number), o1.NumberDisplay)
	s.True(strings.HasPrefix(o1.NumberDisplay, "З-"))
}

// ---- TS-009: поставщик заперт в кабинете ----

// TS-009 (DS-017, DS-053): сессия supplier на каждом разделе — SSR 302 на /supplier/orders,
// API 403; разрешены смена своего пояса и выход. Негатив: чужой заказ (поставщик Б) → 404.
func (s *PurchaseOrdersLiveSuite) TestTS009_SupplierLockedInCabinet() {
	o := s.sentOrder(s.admin, 1)
	task := s.pending(o.Items[0].ProductID, "1")
	eq := s.scalar(`SELECT id::text FROM equipment LIMIT 1`)
	if eq == "<nil>" {
		eq = uuid.NewString()
	}
	ssrPaths := []string{"/", "/stock", "/clients", "/equipment/" + eq, "/catalog", "/catalog/products", "/catalog/services",
		"/catalog/categories", "/suppliers", "/users", "/calendar", "/executor-payouts", "/deleted",
		fmt.Sprintf("/tasks/%d/print/handover", task), fmt.Sprintf("/tasks/%d/print/handover/pdf", task),
		"/purchases", "/purchases/new", "/purchases/" + o.ID, "/purchases/" + o.ID + "/print"}
	for _, p := range ssrPaths {
		resp := s.supA.Request(http.MethodGet, p, nil, nil)
		resp.Body.Close()
		s.Equal(http.StatusFound, resp.StatusCode, p)
		s.Equal("/supplier/orders", resp.Header.Get("Location"), p)
	}
	apiPaths := []string{"/api/v1/tasks", "/api/v1/columns", "/api/v1/subtasks", "/api/v1/subtask-products", "/api/v1/subtask-logs",
		"/api/v1/attachments", "/api/v1/assignees", "/api/v1/stock", "/api/v1/clients", "/api/v1/equipment/" + eq, "/api/v1/products",
		"/api/v1/services", "/api/v1/catalog-categories", "/api/v1/suppliers", "/api/v1/users", "/api/v1/calendar",
		"/api/v1/purchase-orders", "/api/v1/purchase-orders/" + o.ID, "/api/v1/purchase-orders/suggestions", "/api/v1/deleted"}
	for _, p := range apiPaths {
		st, raw := s.supA.Do(http.MethodGet, p, nil)
		s.Equal(http.StatusForbidden, st, "%s: %s", p, raw)
	}
	st, raw := s.supA.Do(http.MethodPost, poAPI, dto.CreateRequest{Items: []dto.ItemRequest{line(o.Items[0].ProductID, "1")}})
	s.Equal(http.StatusForbidden, st, string(raw))

	// Свой кабинет открыт.
	resp := s.supA.Request(http.MethodGet, "/supplier/orders", nil, nil)
	resp.Body.Close()
	s.Equal(200, resp.StatusCode)
	resp = s.supA.Request(http.MethodGet, "/supplier/orders/"+o.ID, nil, nil)
	resp.Body.Close()
	s.Equal(200, resp.StatusCode)
	resp = s.supA.Request(http.MethodGet, "/supplier/orders/"+o.ID+"/print", nil, nil)
	resp.Body.Close()
	s.Equal(200, resp.StatusCode)
	s.Equal("application/pdf", resp.Header.Get("Content-Type"))
	st, raw = s.supA.Do(http.MethodPut, "/api/v1/users/me/timezone", map[string]string{"timezone": "Europe/Moscow"})
	s.Equal(200, st, string(raw))
	st, _ = s.supA.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(200, st)

	// Чужой поставщик → 404 на §6.13–§6.16.
	st, _ = s.supB.Do(http.MethodGet, spAPI+"/"+o.ID, nil)
	s.Equal(404, st)
	st, _ = s.answer(s.supB, o.ID, o.Items[0].ID, o.Items[0].Version, ptr("1"), ptr("1"), "")
	s.Equal(404, st)
	st, _ = s.supB.Do(http.MethodPost, spAPI+"/"+o.ID+"/ready", nil)
	s.Equal(404, st)
	resp = s.supB.Request(http.MethodGet, "/supplier/orders/"+o.ID+"/print", nil, nil)
	resp.Body.Close()
	s.Equal(404, resp.StatusCode)
	var lst dto.SupplierListResponse
	st, raw = s.supB.Do(http.MethodGet, spAPI, nil)
	s.Require().Equal(200, st)
	s.Require().NoError(json.Unmarshal(raw, &lst))
	for _, it := range lst.Items {
		s.NotEqual(o.ID, it.ID)
	}
	st, raw = s.supA.Do(http.MethodGet, spAPI+"?status=draft", nil)
	s.expectErr(st, raw, 400, "VALIDATION", "Неизвестный статус")

	// Сотрудник в кабинет не попадает.
	st, raw = s.admin.Do(http.MethodGet, spAPI, nil)
	s.expectErr(st, raw, 403, "FORBIDDEN", "Раздел только для поставщиков")
	resp = s.admin.Request(http.MethodGet, "/supplier/orders", nil, nil)
	resp.Body.Close()
	s.Equal(403, resp.StatusCode)

	// Выход разрешён.
	out := s.NewBrowser()
	out.Login("po-sup-a", "password1")
	st, _ = out.Do(http.MethodPost, "/api/v1/auth/logout", nil)
	s.Less(st, 300)
}

// Вход поставщика ведёт в кабинет, ?redirect= на чужой раздел игнорируется (DS-018).
func (s *PurchaseOrdersLiveSuite) TestSupplierLoginRedirect() {
	b := s.NewBrowser()
	form := map[string][]string{"login": {"po-sup-a"}, "password": {"password1"}, "redirect": {"/stock"}}
	resp := b.Request(http.MethodPost, "/login", url.Values(form), nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/supplier/orders", resp.Header.Get("Location"))
}

// ---- TS-010: executor видит только свои ----

// TS-010 (DS-050, DS-051, DS-060): список executor — только свои; чужой заказ по id → 404 на
// GET/PUT/send/recall/pick-up/cancel/PDF; DELETE своего черновика — 403; admin видит и меняет
// оба. Негатив: created_by в теле игнорируется.
func (s *PurchaseOrdersLiveSuite) TestTS010_ExecutorOwnOrders() {
	p := s.product("Свой", "0", "0")
	st, raw := s.execA.Do(http.MethodPost, poAPI, map[string]any{"supplier_id": s.supplierA, "created_by": s.execBID,
		"items": []map[string]any{{"product_id": p, "qty": "1"}}})
	s.Require().Equal(201, st, string(raw))
	var mine dto.Order
	s.Require().NoError(json.Unmarshal(raw, &mine))
	s.Equal("Пользователь po-exec-a", mine.CreatedByName, "автор — из сессии")
	theirs := s.mustCreate(s.execB, &s.supplierA, line(p, "2"))

	var list dto.ListResponse
	st, raw = s.execA.Do(http.MethodGet, poAPI+"?limit=100", nil)
	s.Require().Equal(200, st)
	s.Require().NoError(json.Unmarshal(raw, &list))
	n := s.scalar(`SELECT count(*)::text FROM purchase_orders WHERE created_by = $1::uuid AND deleted_at IS NULL`, s.execAID)
	s.Equal(n, fmt.Sprint(list.Total), "total считает только свои")
	for _, it := range list.Items {
		s.NotEqual(theirs.ID, it.ID)
	}

	st, raw = s.execA.Do(http.MethodGet, poAPI+"/"+theirs.ID, nil)
	s.expectErr(st, raw, 404, "PURCHASE_ORDER_NOT_FOUND", "Заказ не найден")
	st, raw = s.execA.Do(http.MethodPut, poAPI+"/"+theirs.ID, dto.UpdateRequest{Version: theirs.Version, Items: []dto.ItemRequest{line(p, "1")}})
	s.expectErr(st, raw, 404, "PURCHASE_ORDER_NOT_FOUND", "")
	for _, a := range []string{"send", "recall", "pick-up"} {
		st, _, raw = s.act(s.execA, theirs.ID, a, theirs.Version)
		s.expectErr(st, raw, 404, "PURCHASE_ORDER_NOT_FOUND", "")
	}
	st, raw = s.execA.Do(http.MethodPost, poAPI+"/"+theirs.ID+"/cancel", dto.ReasonRequest{Version: theirs.Version, Reason: ptr("x")})
	s.expectErr(st, raw, 404, "PURCHASE_ORDER_NOT_FOUND", "")
	resp := s.execA.Request(http.MethodGet, "/purchases/"+theirs.ID+"/print", nil, nil)
	resp.Body.Close()
	s.Equal(404, resp.StatusCode)
	resp = s.execA.Request(http.MethodGet, "/purchases/"+theirs.ID, nil, nil)
	resp.Body.Close()
	s.Equal(404, resp.StatusCode)

	st, raw = s.execA.Do(http.MethodDelete, poAPI+"/"+mine.ID, dto.ReasonRequest{Version: mine.Version, Reason: ptr("x")})
	s.expectErr(st, raw, 403, "FORBIDDEN", "Удалять может только администратор")

	// admin видит и меняет оба
	s.get(s.admin, theirs.ID)
	s.mustAct(s.admin, mine.ID, "send", mine.Version)
	s.mustAct(s.admin, theirs.ID, "send", theirs.Version)
	// свой — executor правит и печатает
	resp = s.execA.Request(http.MethodGet, "/purchases/"+mine.ID+"/print", nil, nil)
	resp.Body.Close()
	s.Equal(200, resp.StatusCode)
	s.Equal("application/pdf", resp.Header.Get("Content-Type"))
	s.Contains(resp.Header.Get("Content-Disposition"), ".pdf")
}

// ---- TS-011: архив/удаление поставщика ----

// TS-011 (DS-043): архив и мягкое удаление поставщика с заказом в draft/sent/ready → 409
// SUPPLIER_HAS_OPEN_PURCHASES с номерами; с заказами только picked_up/cancelled — проходят.
func (s *PurchaseOrdersLiveSuite) TestTS011_SupplierWithOpenPurchases() {
	sup := s.newSupplier("Закрываемый " + mark())
	o := s.mustCreate(s.admin, &sup, line(s.product("Т", "0", "0"), "1"))
	archive := map[string]any{"name": "Закрываемый " + mark(), "contact_name": "", "phone": "", "address": "", "note": "", "is_active": false}
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/suppliers/"+sup, archive)
	s.expectErr(st, raw, 409, "SUPPLIER_HAS_OPEN_PURCHASES", "У поставщика есть незакрытые закупки: "+o.NumberDisplay)
	s.Equal([]string{o.NumberDisplay}, s.err(raw).Details.Numbers)
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/suppliers/"+sup, map[string]string{"reason": "Больше не работаем"})
	s.expectErr(st, raw, 409, "SUPPLIER_HAS_OPEN_PURCHASES", "")

	o = s.mustAct(s.admin, o.ID, "send", o.Version)
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/suppliers/"+sup, archive)
	s.Equal(409, st, string(raw))

	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+o.ID+"/cancel", dto.ReasonRequest{Version: o.Version, Reason: ptr("закрываем")})
	s.Require().Equal(200, st, string(raw))
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/suppliers/"+sup, archive)
	s.Equal(200, st, string(raw))
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/suppliers/"+sup, map[string]string{"reason": "Больше не работаем"})
	s.Equal(200, st, string(raw))
}

// ---- TS-012: учётки поставщиков ----

// TS-012 (DS-044, DS-045): supplier без supplier_id → 400; архивный поставщик → 400; executor с
// supplier_id → 400; учётка создаётся и входит в кабинет; блокировка → следующий запрос 401.
func (s *PurchaseOrdersLiveSuite) TestTS012_SupplierAccounts() {
	base := func(login, role string, supplierID *string) map[string]any {
		return map[string]any{"login": login, "name": "Продавец", "password": "secret123", "role": role, "supplier_id": supplierID,
			"is_active": true, "timezone": "Asia/Almaty", "phone": ""}
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/users", base("po-u-"+mark(), "supplier", nil))
	s.Equal(400, st, string(raw))
	s.Equal("required", s.field(raw)["supplier_id"])
	archived := s.newSupplier("Архив " + mark())
	s.exec(`UPDATE suppliers SET is_active = false WHERE id = $1::uuid`, archived)
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/users", base("po-u-"+mark(), "supplier", &archived))
	s.Equal(400, st, string(raw))
	s.Equal("unavailable", s.field(raw)["supplier_id"])
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/users", base("po-u-"+mark(), "executor", &s.supplierA))
	s.Equal(400, st, string(raw))
	s.Equal("not_allowed", s.field(raw)["supplier_id"])
	st, raw = s.execA.Do(http.MethodPost, "/api/v1/users", base("po-u-"+mark(), "supplier", &s.supplierA))
	s.Equal(403, st, string(raw))

	login := "po-u-" + mark()
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/users", base(login, "supplier", &s.supplierA))
	s.Require().Equal(201, st, string(raw))
	var u struct {
		ID           string  `json:"id"`
		Role         string  `json:"role"`
		SupplierID   *string `json:"supplier_id"`
		SupplierName string  `json:"supplier_name"`
	}
	s.Require().NoError(json.Unmarshal(raw, &u))
	s.Equal("supplier", u.Role)
	s.Equal(s.supplierA, *u.SupplierID)
	s.Contains(u.SupplierName, "АвтоДеталь Плюс")

	b := s.NewBrowser()
	b.Login(login, "secret123")
	st, _ = b.Do(http.MethodGet, spAPI, nil)
	s.Equal(200, st)

	upd := base(login, "supplier", &s.supplierA)
	upd["is_active"] = false
	delete(upd, "password")
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/"+u.ID, upd)
	s.Require().Equal(200, st, string(raw))
	st, _ = b.Do(http.MethodGet, spAPI, nil)
	s.Equal(401, st, "блокировка обрывает сессию")

	// Смена роли supplier → executor: supplier_id сбрасывается (DS-069).
	upd["is_active"] = true
	upd["role"] = "executor"
	upd["supplier_id"] = nil
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/"+u.ID, upd)
	s.Require().Equal(200, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &u))
	s.Nil(u.SupplierID)
}

// ---- TS-013 ----

// TS-013 (DS-010): «Забрали» не меняет products.stock и не пишет stock_movements.
func (s *PurchaseOrdersLiveSuite) TestTS013_PickUpKeepsStock() {
	p := s.product("Остаток", "7", "0")
	o := s.mustCreate(s.admin, nil, line(p, "3"))
	s.mustAct(s.admin, o.ID, "pick-up", o.Version)
	s.Equal("7.000", s.scalar(`SELECT stock::text FROM products WHERE id = $1::uuid`, p))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1::uuid`, p))
}

// ---- удаление и восстановление черновика (DS-037, DS-072) ----

func (s *PurchaseOrdersLiveSuite) TestSoftDeleteRestorePurge() {
	o := s.mustCreate(s.admin, nil, line(s.product("Удаляемый", "0", "0"), "1"))
	st, raw := s.admin.Do(http.MethodDelete, poAPI+"/"+o.ID, dto.ReasonRequest{Version: o.Version, Reason: ptr("")})
	s.Equal(400, st, string(raw))
	st, raw = s.admin.Do(http.MethodDelete, poAPI+"/"+o.ID, dto.ReasonRequest{Version: o.Version + 5, Reason: ptr("Создан по ошибке")})
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_VERSION_CONFLICT", "")
	st, raw = s.admin.Do(http.MethodDelete, poAPI+"/"+o.ID, dto.ReasonRequest{Version: o.Version, Reason: ptr("Создан по ошибке")})
	s.Require().Equal(204, st, string(raw))
	st, _ = s.admin.Do(http.MethodGet, poAPI+"/"+o.ID, nil)
	s.Equal(404, st)
	st, _ = s.admin.Do(http.MethodDelete, poAPI+"/"+o.ID, dto.ReasonRequest{Version: o.Version + 1, Reason: ptr("ещё")})
	s.Equal(404, st, "повтор по удалённому")

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/deleted?kind=purchase-orders&limit=100", nil)
	s.Require().Equal(200, st, string(raw))
	s.Contains(string(raw), o.ID)
	s.Contains(string(raw), o.NumberDisplay)
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/deleted/summary", nil)
	s.Require().Equal(200, st, string(raw))
	s.Contains(string(raw), `"purchase_orders"`)

	st, _ = s.execA.Do(http.MethodPost, poAPI+"/"+o.ID+"/restore", nil)
	s.Equal(403, st)
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+o.ID+"/restore", nil)
	s.Require().Equal(200, st, string(raw))
	s.Equal("draft", s.get(s.admin, o.ID).Status)
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+o.ID+"/restore", nil)
	s.expectErr(st, raw, 409, "NOT_DELETED", "")

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/test/purge/purchase-orders/"+o.ID, nil)
	s.Less(st, 300, string(raw))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM purchase_order_items WHERE order_id = $1::uuid`, o.ID), "строки каскадом")
}

// SSR-экраны и PDF §6.11: страницы в общем shell, PDF — application/pdf в любом статусе.
func (s *PurchaseOrdersLiveSuite) TestSSRPagesAndPDF() {
	o := s.mustCreate(s.admin, nil, line(s.product("Печать", "0", "0"), "1"))
	for _, p := range []string{"/purchases", "/purchases/new", "/purchases/" + o.ID} {
		st, raw := s.admin.Do(http.MethodGet, p, nil)
		s.Equal(200, st, p)
		s.Contains(string(raw), `data-testid="sidebar"`, p)
		s.Contains(string(raw), `href="/purchases"`, "пункт «Закупки» в сайдбаре")
	}
	st, _ := s.admin.Do(http.MethodGet, "/purchases/"+uuid.NewString(), nil)
	s.Equal(404, st)
	resp := s.admin.Request(http.MethodGet, "/purchases/"+o.ID+"/print", nil, nil)
	defer resp.Body.Close()
	s.Equal(200, resp.StatusCode)
	s.Equal("application/pdf", resp.Header.Get("Content-Type"))

	st, raw := s.supA.Do(http.MethodGet, "/supplier/orders", nil)
	s.Equal(200, st)
	s.Contains(string(raw), "СТО Тест", "реквизиты мастерской")
	s.NotContains(string(raw), `href="/stock"`, "упрощённый сайдбар")
	st, raw = s.supA.Do(http.MethodGet, "/supplier/orders/"+uuid.NewString(), nil)
	s.Equal(404, st)
	s.Contains(string(raw), "Заказ не найден")
}

// ---- FIX-LOOP 3: DS-045 и инвариант DS-043 ----

// DS-045: учётку поставщика, ушедшего в архив, можно заблокировать (проверка доступности
// поставщика — только при смене роли/поставщика); следующий запрос учётки → 401.
func (s *PurchaseOrdersLiveSuite) TestBlockAccountOfArchivedSupplier() {
	sup := s.newSupplier("Уходящий " + mark())
	login := "po-arch-" + mark()
	body := map[string]any{"login": login, "name": "Продавец", "password": "secret123", "role": "supplier",
		"supplier_id": sup, "is_active": true, "timezone": "Asia/Almaty", "phone": ""}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/users", body)
	s.Require().Equal(201, st, string(raw))
	var u struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &u))
	b := s.NewBrowser()
	b.Login(login, "secret123")

	archive := map[string]any{"name": "Уходящий " + mark(), "contact_name": "", "phone": "", "address": "", "note": "", "is_active": false}
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/suppliers/"+sup, archive)
	s.Require().Equal(200, st, string(raw))

	delete(body, "password")
	body["name"] = "Переименован"
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/"+u.ID, body)
	s.Equal(200, st, "правка прочих полей при неизменном поставщике: %s", raw)
	body["is_active"] = false
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/"+u.ID, body)
	s.Require().Equal(200, st, "блокировка учётки архивного поставщика: %s", raw)
	st, _ = b.Do(http.MethodGet, spAPI, nil)
	s.Equal(401, st, "сессия оборвана")

	// Привязать учётку к архивному поставщику заново — по-прежнему нельзя (смена supplier_id).
	other := s.newSupplier("Другой " + mark())
	body["supplier_id"] = other
	st, _ = s.admin.Do(http.MethodPut, "/api/v1/users/"+u.ID, body)
	s.Equal(200, st)
	body["supplier_id"] = sup
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/"+u.ID, body)
	s.Equal(400, st, string(raw))
	s.Equal("unavailable", s.field(raw)["supplier_id"])
}

// DS-043 (а): восстановить удалённый черновик архивного поставщика нельзя — 409
// PURCHASE_ORDER_SUPPLIER_UNAVAILABLE; черновик остаётся удалённым.
func (s *PurchaseOrdersLiveSuite) TestRestoreDraftOfArchivedSupplier() {
	sup := s.newSupplier("Архив-восст " + mark())
	o := s.mustCreate(s.admin, &sup, line(s.product("В", "0", "0"), "1"))
	st, raw := s.admin.Do(http.MethodDelete, poAPI+"/"+o.ID, dto.ReasonRequest{Version: o.Version, Reason: ptr("ошибка")})
	s.Require().Equal(204, st, string(raw))
	archive := map[string]any{"name": "Архив-восст " + mark(), "contact_name": "", "phone": "", "address": "", "note": "", "is_active": false}
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/suppliers/"+sup, archive)
	s.Require().Equal(200, st, "удалённый черновик архиву не мешает: %s", raw)

	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+o.ID+"/restore", nil)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_SUPPLIER_UNAVAILABLE", "Поставщик недоступен — восстановить черновик нельзя")
	s.Equal("true", s.scalar(`SELECT (deleted_at IS NOT NULL)::text FROM purchase_orders WHERE id = $1::uuid`, o.ID))

	// Мягко удалённый поставщик — так же.
	sup2 := s.newSupplier("Удалён-восст " + mark())
	o2 := s.mustCreate(s.admin, &sup2, line(s.product("В2", "0", "0"), "1"))
	st, _ = s.admin.Do(http.MethodDelete, poAPI+"/"+o2.ID, dto.ReasonRequest{Version: o2.Version, Reason: ptr("ошибка")})
	s.Require().Equal(204, st)
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/suppliers/"+sup2, map[string]string{"reason": "закрыт"})
	s.Require().Equal(200, st, string(raw))
	st, raw = s.admin.Do(http.MethodPost, poAPI+"/"+o2.ID+"/restore", nil)
	s.expectErr(st, raw, 409, "PURCHASE_ORDER_SUPPLIER_UNAVAILABLE", "")
}

// DS-043 (б): отправить черновик архивному/удалённому поставщику нельзя — 400 supplier_id:
// unavailable, как при создании (§6.2). Черновик у архивного поставщика сделан в обход API
// (архив через API при черновике запрещён).
func (s *PurchaseOrdersLiveSuite) TestSendToArchivedSupplier() {
	sup := s.newSupplier("Архив-отпр " + mark())
	o := s.mustCreate(s.admin, &sup, line(s.product("О", "0", "0"), "1"))
	s.exec(`UPDATE suppliers SET is_active = false WHERE id = $1::uuid`, sup)
	st, _, raw := s.act(s.admin, o.ID, "send", o.Version)
	s.Equal(400, st, string(raw))
	s.Equal("unavailable", s.field(raw)["supplier_id"])
	s.Equal("Поставщик недоступен", s.err(raw).Message)
	s.Equal("draft", s.get(s.admin, o.ID).Status)

	s.exec(`UPDATE suppliers SET is_active = true, deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'x' WHERE id = $1::uuid`, sup, s.adminID)
	st, _, raw = s.act(s.admin, o.ID, "send", o.Version)
	s.Equal(400, st, string(raw))
}

func (s *PurchaseOrdersLiveSuite) lockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

// race — запрос req уходит, пока тест держит блокировку в tx; по pg_stat_activity тест
// дожидается, что запрос встал в ожидание, затем выполняет finish (фиксация tx). Без sleep.
func (s *PurchaseOrdersLiveSuite) race(finish func(), req func() (int, []byte)) (int, []byte) {
	base := s.lockWaiters()
	type res struct {
		st  int
		raw []byte
	}
	done := make(chan res, 1)
	go func() { st, raw := req(); done <- res{st, raw} }()
	s.Require().Eventually(func() bool { return s.lockWaiters() > base }, 10*time.Second, 20*time.Millisecond, "запрос ждёт блокировку")
	select {
	case r := <-done:
		s.FailNow("запрос не дождался блокировки", "%d %s", r.st, r.raw)
	default:
	}
	finish()
	r := <-done
	return r.st, r.raw
}

// DS-043 (в), сериализация: архив поставщика идёт (строка поставщика FOR UPDATE, is_active=false,
// tx не зафиксирована) — создание закупки ждёт блокировку и после фиксации получает 400 «Поставщик
// недоступен».
func (s *PurchaseOrdersLiveSuite) TestRace_CreateWaitsForArchive() {
	ctx := context.Background()
	sup := s.newSupplier("Гонка-архив " + mark())
	p := s.product("Г", "0", "0")
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT 1 FROM suppliers WHERE id = $1::uuid FOR UPDATE`, sup)
	s.Require().NoError(err)
	_, err = tx.Exec(ctx, `UPDATE suppliers SET is_active = false WHERE id = $1::uuid`, sup)
	s.Require().NoError(err)

	st, raw := s.race(func() { s.Require().NoError(tx.Commit(ctx)) }, func() (int, []byte) {
		st, _, raw := s.create(s.admin, &sup, line(p, "1"))
		return st, raw
	})
	s.Equal(400, st, string(raw))
	s.Equal("unavailable", s.field(raw)["supplier_id"])
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM purchase_orders WHERE supplier_id = $1::uuid`, sup))
}

// DS-043 (в), сериализация в обратную сторону: создание закупки идёт (строка поставщика FOR SHARE,
// черновик вставлен, tx не зафиксирована) — архив поставщика ждёт блокировку и после фиксации
// получает 409 SUPPLIER_HAS_OPEN_PURCHASES; поставщик остаётся активным.
func (s *PurchaseOrdersLiveSuite) TestRace_ArchiveWaitsForCreate() {
	ctx := context.Background()
	sup := s.newSupplier("Гонка-создание " + mark())
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT 1 FROM suppliers WHERE id = $1::uuid FOR SHARE`, sup)
	s.Require().NoError(err)
	var number int64
	s.Require().NoError(tx.QueryRow(ctx, `INSERT INTO purchase_orders (supplier_id, created_by) VALUES ($1::uuid, $2::uuid) RETURNING number`,
		sup, s.adminID).Scan(&number))

	archive := map[string]any{"name": "Гонка-создание " + mark(), "contact_name": "", "phone": "", "address": "", "note": "", "is_active": false}
	st, raw := s.race(func() { s.Require().NoError(tx.Commit(ctx)) }, func() (int, []byte) {
		return s.admin.Do(http.MethodPut, "/api/v1/suppliers/"+sup, archive)
	})
	s.expectErr(st, raw, 409, "SUPPLIER_HAS_OPEN_PURCHASES", fmt.Sprintf("У поставщика есть незакрытые закупки: З-%04d", number))
	s.Equal("true", s.scalar(`SELECT is_active::text FROM suppliers WHERE id = $1::uuid`, sup))
}
