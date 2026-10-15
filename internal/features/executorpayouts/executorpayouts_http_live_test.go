//go:build integration

package executorpayouts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ExecutorPayoutsLiveSuite (§TD-8): отчёт «Выплаты» и массовая отметка (docs/plan.md
// D69/D70; сценарии TS-011…TS-017, TS-024 —
// docs/features/executor-payouts/0-setup/light-progress.md). Реальный бинарь + TCP +
// реальный логин; фикстуры — напрямую SQL. Тесты делят одну БД, поэтому каждый заводит
// своих исполнителей и фильтрует по ним.
type ExecutorPayoutsLiveSuite struct {
	testutil.LiveSuite
	admin    *testutil.Browser
	adminID  string
	clientID string
	columnID string
	svcID    string
	seq      int
}

func TestExecutorPayoutsLiveSuite(t *testing.T) {
	suite.Run(t, &ExecutorPayoutsLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *ExecutorPayoutsLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.adminID = s.SeedUser("ep-admin", "password1", "Выплаты Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("ep-admin", "password1")
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('EP Клиент') RETURNING id::text`).Scan(&s.clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM columns ORDER BY position, id LIMIT 1`).Scan(&s.columnID))
	var catID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'EP услуги') RETURNING id::text`).Scan(&catID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO services (category_id, name, price) VALUES ($1, 'EP диагностика', 1000) RETURNING id::text`, catID).Scan(&s.svcID))
}

// ---- wire-типы (1:1 с internal/features/executorpayouts/dto.go, §TD-6.2) ----

type payoutItemDTO struct {
	SubtaskID      string       `json:"subtask_id"`
	TaskID         int64        `json:"task_id"`
	TaskTitle      string       `json:"task_title"`
	ClientName     string       `json:"client_name"`
	ServiceName    string       `json:"service_name"`
	Status         string       `json:"status"`
	AssigneeID     string       `json:"assignee_id"`
	AssigneeName   string       `json:"assignee_name"`
	Price          *json.Number `json:"price"`
	ExecutorPayout *json.Number `json:"executor_payout"`
	IsExecutorPaid bool         `json:"is_executor_paid"`
	ExecutorPaidAt *string      `json:"executor_paid_at"`
}

type payoutListDTO struct {
	Items  []payoutItemDTO `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type summaryItemDTO struct {
	AssigneeID   string      `json:"assignee_id"`
	AssigneeName string      `json:"assignee_name"`
	UnpaidCount  int         `json:"unpaid_count"`
	PaidCount    int         `json:"paid_count"`
	ToPay        json.Number `json:"to_pay"`
	Paid         json.Number `json:"paid"`
}

type summaryDTO struct {
	Items  []summaryItemDTO `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type markDTO struct {
	Updated int `json:"updated"`
}

type apiErrDTO struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

// ---- помощники ----

func (s *ExecutorPayoutsLiveSuite) errOf(body []byte) apiErrDTO {
	var e apiErrDTO
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *ExecutorPayoutsLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

// newExecutor — активный исполнитель; имя задаёт порядок групп отчёта.
func (s *ExecutorPayoutsLiveSuite) newExecutor(name string) (string, string) {
	s.seq++
	login := fmt.Sprintf("ep-exec-%d", s.seq)
	return s.SeedUser(login, "password1", name, "executor", true), login
}

func (s *ExecutorPayoutsLiveSuite) newTask(title string) int64 {
	s.seq++
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`WITH car AS (INSERT INTO equipment (client_id, plate_number) VALUES ($2, 'ВП 001') RETURNING id)
		 INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by)
		 VALUES ($1, $2, (SELECT id FROM car), $3, $4, $5) RETURNING id`,
		s.columnID, s.clientID, title, s.seq*1024, s.adminID).Scan(&id))
	return id
}

type subtaskFx struct {
	status   string
	assignee string // "" — без исполнителя
	price    string // "" — по факту
	payout   string // "" — не задана
	paid     bool
	position int
}

func nullable(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (s *ExecutorPayoutsLiveSuite) newSubtask(taskID int64, fx subtaskFx) string {
	if fx.status == "" {
		fx.status = "new"
	}
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		INSERT INTO subtasks (task_id, service_id, status, assignee_id, price, position,
		                      executor_payout, is_executor_paid, executor_paid_at)
		VALUES ($1, $2, $3, $4::uuid, $5::numeric, $6, $7::numeric, $8, CASE WHEN $8 THEN now() END)
		RETURNING id::text`,
		taskID, s.svcID, fx.status, nullable(fx.assignee), nullable(fx.price), fx.position, nullable(fx.payout), fx.paid).Scan(&id))
	return id
}

func (s *ExecutorPayoutsLiveSuite) list(b *testutil.Browser, query string) payoutListDTO {
	st, raw := b.Do(http.MethodGet, "/api/v1/executor-payouts?"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l payoutListDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	s.Require().NotNil(l.Items, "items — всегда массив: %s", raw)
	return l
}

func (s *ExecutorPayoutsLiveSuite) summary(b *testutil.Browser, query string) summaryDTO {
	st, raw := b.Do(http.MethodGet, "/api/v1/executor-payouts/summary?"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l summaryDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	s.Require().NotNil(l.Items, "items — всегда массив: %s", raw)
	return l
}

func (s *ExecutorPayoutsLiveSuite) summaryOf(l summaryDTO, assigneeID string) *summaryItemDTO {
	for i := range l.Items {
		if l.Items[i].AssigneeID == assigneeID {
			return &l.Items[i]
		}
	}
	return nil
}

func (s *ExecutorPayoutsLiveSuite) mark(b *testutil.Browser, ids []string, paid bool) (int, markDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/executor-payouts/mark", map[string]any{"subtask_ids": ids, "is_executor_paid": paid})
	var m markDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	}
	return st, m, raw
}

// state — отметка и дата услуги из БД.
func (s *ExecutorPayoutsLiveSuite) state(subtaskID string) (bool, *string) {
	var paid bool
	var paidAt *string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT is_executor_paid, executor_paid_at::text FROM subtasks WHERE id = $1`, subtaskID).Scan(&paid, &paidAt))
	return paid, paidAt
}

func ids(items []payoutItemDTO) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.SubtaskID
	}
	return out
}

// ---- список ----

// TS-011: список — только услуги с исполнителем; конверт {items,total,limit,offset}; поля
// строки; порядок — заказ, позиция услуги; limit/offset; total не зависит от страницы.
func (s *ExecutorPayoutsLiveSuite) TestTS011_List_EnvelopeFieldsOrder() {
	execID, _ := s.newExecutor("TS011 Исполнитель")
	t1 := s.newTask("TS011 первый заказ")
	t2 := s.newTask("TS011 второй заказ")
	second := s.newSubtask(t1, subtaskFx{status: "done", assignee: execID, price: "1000", payout: "500.5", paid: true, position: 2})
	first := s.newSubtask(t1, subtaskFx{status: "in_progress", assignee: execID, position: 1})
	third := s.newSubtask(t2, subtaskFx{assignee: execID, price: "250", position: 1})
	s.newSubtask(t1, subtaskFx{status: "done", price: "777", position: 3}) // без исполнителя — не в отчёте

	l := s.list(s.admin, "assignee="+execID)
	s.Equal(3, l.Total)
	s.Equal(200, l.Limit, "limit по умолчанию")
	s.Equal(0, l.Offset)
	s.Equal([]string{first, second, third}, ids(l.Items), "порядок: заказ, позиция услуги")

	paid := l.Items[1]
	s.Equal(t1, paid.TaskID)
	s.Equal("TS011 первый заказ", paid.TaskTitle)
	s.Equal("EP Клиент", paid.ClientName)
	s.Equal("EP диагностика", paid.ServiceName)
	s.Equal("done", paid.Status)
	s.Equal(execID, paid.AssigneeID)
	s.Equal("TS011 Исполнитель", paid.AssigneeName)
	s.Require().NotNil(paid.Price)
	s.Equal("1000.00", paid.Price.String())
	s.Require().NotNil(paid.ExecutorPayout)
	s.Equal("500.50", paid.ExecutorPayout.String())
	s.True(paid.IsExecutorPaid)
	s.NotNil(paid.ExecutorPaidAt)

	unpaid := l.Items[0]
	s.Nil(unpaid.Price, "цена «по факту» — null")
	s.Nil(unpaid.ExecutorPayout, "сумма не задана — null")
	s.False(unpaid.IsExecutorPaid)
	s.Nil(unpaid.ExecutorPaidAt)

	page := s.list(s.admin, "assignee="+execID+"&limit=1&offset=1")
	s.Equal([]string{second}, ids(page.Items))
	s.Equal(3, page.Total)
	s.Equal(1, page.Limit)
	s.Equal(1, page.Offset)

	beyond := s.list(s.admin, "assignee="+execID+"&offset=50")
	s.Empty(beyond.Items)
	s.Equal(3, beyond.Total, "total не зависит от страницы")
}

// TS-012: фильтры status / only_done / assignee (uuid|me|all) и дефолт assignee по роли:
// executor — «мои», admin — все.
func (s *ExecutorPayoutsLiveSuite) TestTS012_List_FiltersAndRoleDefault() {
	execID, login := s.newExecutor("TS012 Свой")
	otherID, _ := s.newExecutor("TS012 Чужой")
	task := s.newTask("TS012 заказ")
	doneUnpaid := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, position: 1})
	donePaid := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, paid: true, position: 2})
	newUnpaid := s.newSubtask(task, subtaskFx{status: "new", assignee: execID, position: 3})
	foreign := s.newSubtask(task, subtaskFx{status: "done", assignee: otherID, position: 4})

	base := "assignee=" + execID
	s.Equal([]string{doneUnpaid, donePaid, newUnpaid}, ids(s.list(s.admin, base).Items))
	s.Equal([]string{doneUnpaid, donePaid, newUnpaid}, ids(s.list(s.admin, base+"&status=all&only_done=false").Items))
	s.Equal([]string{doneUnpaid, newUnpaid}, ids(s.list(s.admin, base+"&status=unpaid").Items))
	s.Equal([]string{donePaid}, ids(s.list(s.admin, base+"&status=paid").Items))
	s.Equal([]string{doneUnpaid, donePaid}, ids(s.list(s.admin, base+"&only_done=true").Items))
	s.Equal([]string{doneUnpaid}, ids(s.list(s.admin, base+"&only_done=true&status=unpaid").Items))

	exec := s.NewBrowser()
	exec.Login(login, "password1")
	// executor без параметра — только свои услуги.
	s.Equal([]string{doneUnpaid, donePaid, newUnpaid}, ids(s.list(exec, "").Items))
	s.Equal([]string{doneUnpaid, donePaid, newUnpaid}, ids(s.list(exec, "assignee=me").Items))
	// executor может переключиться на всех и на другого исполнителя.
	s.Contains(ids(s.list(exec, "assignee=all&limit=1000").Items), foreign)
	s.Equal([]string{foreign}, ids(s.list(exec, "assignee="+otherID).Items))

	// admin без параметра — все; «мои» у админа без услуг — пусто.
	all := ids(s.list(s.admin, "limit=1000").Items)
	s.Contains(all, doneUnpaid)
	s.Contains(all, foreign)
	mine := s.list(s.admin, "assignee=me")
	s.Empty(mine.Items)
	s.Equal(0, mine.Total)
}

// TS-012: невалидные параметры — 400 VALIDATION по полю (список и итоги).
func (s *ExecutorPayoutsLiveSuite) TestTS012_List_InvalidParams400() {
	for _, path := range []string{"/api/v1/executor-payouts", "/api/v1/executor-payouts/summary"} {
		for field, query := range map[string]string{
			"assignee":  "assignee=nobody",
			"status":    "status=done",
			"only_done": "only_done=1",
			"limit":     "limit=0",
			"offset":    "offset=-1",
		} {
			st, raw := s.admin.Do(http.MethodGet, path+"?"+query, nil)
			s.Equal(http.StatusBadRequest, st, "%s?%s: %s", path, query, raw)
			s.Equal("VALIDATION", s.errOf(raw).Code, query)
			s.Equal("invalid", s.fields(raw)[field], query)
		}
		for _, query := range []string{"limit=1001", "limit=abc", "offset=x", "assignee=%00"} {
			st, raw := s.admin.Do(http.MethodGet, path+"?"+query, nil)
			s.Equal(http.StatusBadRequest, st, "%s?%s: %s", path, query, raw)
		}
	}
}

// ---- итоги ----

// TS-013: итоги по исполнителям — to_pay/paid = Σ COALESCE(executor_payout, 0) невыплаченных/
// выплаченных, счётчики; те же фильтры; не зависят от limit/offset списка.
func (s *ExecutorPayoutsLiveSuite) TestTS013_Summary_TotalsByExecutor() {
	aID, _ := s.newExecutor("AAA TS013 Payout")
	bID, _ := s.newExecutor("BBB TS013 Payout")
	task := s.newTask("TS013 заказ")
	s.newSubtask(task, subtaskFx{status: "done", assignee: aID, payout: "100.50", position: 1})
	s.newSubtask(task, subtaskFx{status: "new", assignee: aID, position: 2}) // сумма не задана → 0
	s.newSubtask(task, subtaskFx{status: "done", assignee: aID, payout: "200", paid: true, position: 3})
	s.newSubtask(task, subtaskFx{status: "done", assignee: bID, payout: "50", paid: true, position: 4})
	s.newSubtask(task, subtaskFx{status: "done", payout: "999", position: 5}) // без исполнителя — вне итогов

	all := s.summary(s.admin, "assignee=all&limit=1000")
	s.Equal(len(all.Items), all.Total)
	a, b := s.summaryOf(all, aID), s.summaryOf(all, bID)
	s.Require().NotNil(a)
	s.Require().NotNil(b)
	s.Equal("AAA TS013 Payout", a.AssigneeName)
	s.Equal(2, a.UnpaidCount)
	s.Equal(1, a.PaidCount)
	s.Equal("100.50", a.ToPay.String())
	s.Equal("200.00", a.Paid.String())
	s.Equal(0, b.UnpaidCount)
	s.Equal(1, b.PaidCount)
	s.Equal("0.00", b.ToPay.String())
	s.Equal("50.00", b.Paid.String())
	// порядок — по имени исполнителя.
	var ai, bi int
	for i, it := range all.Items {
		switch it.AssigneeID {
		case aID:
			ai = i
		case bID:
			bi = i
		}
	}
	s.Less(ai, bi)

	// status=unpaid: выплаченные услуги не входят — у «A» paid = 0, «B» в итогах нет.
	unpaid := s.summary(s.admin, "assignee=all&status=unpaid&limit=1000")
	a = s.summaryOf(unpaid, aID)
	s.Require().NotNil(a)
	s.Equal(0, a.PaidCount)
	s.Equal("0.00", a.Paid.String())
	s.Equal("100.50", a.ToPay.String())
	s.Nil(s.summaryOf(unpaid, bID))

	// only_done: невыполненная услуга «A» без суммы выпадает из счётчика.
	done := s.summary(s.admin, "assignee="+aID+"&only_done=true")
	s.Require().Len(done.Items, 1)
	s.Equal(1, done.Total)
	s.Equal(1, done.Items[0].UnpaidCount)
	s.Equal("100.50", done.Items[0].ToPay.String())

	// постраничность списка на итоги не влияет: список limit=1, итоги — по всем услугам.
	page := s.list(s.admin, "assignee="+aID+"&limit=1")
	s.Len(page.Items, 1)
	s.Equal(3, page.Total)
	one := s.summary(s.admin, "assignee="+aID)
	s.Require().Len(one.Items, 1)
	s.Equal(2, one.Items[0].UnpaidCount)
	s.Equal(1, one.Items[0].PaidCount)

	// исполнитель без услуг под фильтром — пустые итоги.
	nobody, _ := s.newExecutor("TS013 Без услуг")
	empty := s.summary(s.admin, "assignee="+nobody)
	s.Empty(empty.Items)
	s.Equal(0, empty.Total)
}

// ---- массовая отметка ----

// TS-014: массовая отметка и снятие; executor_paid_at ставится/обнуляется; повтор
// идемпотентен (updated=0, дата прежней отметки не меняется); uuid в верхнем регистре.
func (s *ExecutorPayoutsLiveSuite) TestTS014_Mark_SetUnsetIdempotent() {
	execID, _ := s.newExecutor("TS014 Исполнитель")
	task := s.newTask("TS014 заказ")
	s1 := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, payout: "100", position: 1})
	s2 := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, position: 2})
	s3 := s.newSubtask(task, subtaskFx{status: "new", assignee: execID, position: 3})

	st, m, raw := s.mark(s.admin, []string{s1, strings.ToUpper(s2)}, true)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(2, m.Updated)
	for _, id := range []string{s1, s2} {
		paid, at := s.state(id)
		s.True(paid)
		s.NotNil(at)
	}
	paid, at := s.state(s3)
	s.False(paid, "не выбранная услуга не тронута")
	s.Nil(at)

	// дата сдвинута в прошлое — повтор не должен её перезаписать.
	_, err := s.Pool.Exec(context.Background(), `UPDATE subtasks SET executor_paid_at = executor_paid_at - interval '3 hours' WHERE id = $1`, s1)
	s.Require().NoError(err)
	_, before := s.state(s1)
	st, m, raw = s.mark(s.admin, []string{s1, s2}, true)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(0, m.Updated, "повтор идемпотентен")
	_, after := s.state(s1)
	s.Equal(*before, *after, "дата прежней отметки сохранена")

	st, m, raw = s.mark(s.admin, []string{s1, s3}, true)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(1, m.Updated, "меняется только ещё не отмеченная")
	_, after = s.state(s1)
	s.Equal(*before, *after)

	// отчёт видит отметки.
	s.Len(s.list(s.admin, "assignee="+execID+"&status=paid").Items, 3)

	st, m, raw = s.mark(s.admin, []string{s1, s2, s3}, false)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(3, m.Updated)
	for _, id := range []string{s1, s2, s3} {
		paid, at := s.state(id)
		s.False(paid)
		s.Nil(at, "снятие отметки обнуляет дату")
	}
	var payout string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT executor_payout::text FROM subtasks WHERE id = $1`, s1).Scan(&payout))
	s.Equal("100.00", payout, "сумма выплаты отметкой не меняется")
}

// TS-015: атомарность — услуга без исполнителя или несуществующая в списке → 400 по
// индексу, остальные услуги запроса НЕ отмечены. Снять отметку без исполнителя можно.
func (s *ExecutorPayoutsLiveSuite) TestTS015_Mark_AtomicOnInvalidItem() {
	execID, _ := s.newExecutor("TS015 Исполнитель")
	task := s.newTask("TS015 заказ")
	ok1 := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, position: 1})
	ok2 := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, position: 2})
	orphan := s.newSubtask(task, subtaskFx{status: "done", position: 3})
	const ghost = "99999999-9999-4999-8999-999999999999"

	st, _, raw := s.mark(s.admin, []string{ok1, orphan, ok2}, true)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("VALIDATION", s.errOf(raw).Code)
	s.Equal(map[string]string{"subtask_ids[1]": "no_assignee"}, s.fields(raw))

	st, _, raw = s.mark(s.admin, []string{ok1, ok2, ghost}, true)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"subtask_ids[2]": "not_found"}, s.fields(raw))

	st, _, raw = s.mark(s.admin, []string{ghost, orphan}, true)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"subtask_ids[0]": "not_found", "subtask_ids[1]": "no_assignee"}, s.fields(raw))

	for _, id := range []string{ok1, ok2, orphan} {
		paid, at := s.state(id)
		s.False(paid, "отклонённый запрос ничего не отметил")
		s.Nil(at)
	}

	// снятие: несуществующая услуга тоже 400 и тоже без частичного эффекта.
	st, m, raw := s.mark(s.admin, []string{ok1, ok2}, true)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(2, m.Updated)
	st, _, raw = s.mark(s.admin, []string{ok1, ghost}, false)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	paid, _ := s.state(ok1)
	s.True(paid, "отклонённое снятие отметку не сняло")

	// услуга осталась выплаченной без исполнителя (сотрудника удалили) — снять можно.
	_, err := s.Pool.Exec(context.Background(), `UPDATE subtasks SET is_executor_paid = true, executor_paid_at = now() WHERE id = $1`, orphan)
	s.Require().NoError(err)
	st, m, raw = s.mark(s.admin, []string{orphan}, false)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(1, m.Updated)
}

// TS-016: валидация тела массовой отметки — 400, не 500.
func (s *ExecutorPayoutsLiveSuite) TestTS016_Mark_InvalidBody400() {
	execID, _ := s.newExecutor("TS016 Исполнитель")
	sub := s.newSubtask(s.newTask("TS016 заказ"), subtaskFx{status: "done", assignee: execID, position: 1})
	post := func(body any) (int, []byte) {
		return s.admin.Do(http.MethodPost, "/api/v1/executor-payouts/mark", body)
	}

	st, raw := post(map[string]any{})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"subtask_ids": "required", "is_executor_paid": "required"}, s.fields(raw))

	st, raw = post(map[string]any{"subtask_ids": []string{}, "is_executor_paid": true})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["subtask_ids"])

	st, raw = post(map[string]any{"subtask_ids": []string{sub}})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["is_executor_paid"])

	st, raw = post(map[string]any{"subtask_ids": []string{sub, "not-a-uuid", "", strings.ToUpper(sub)}, "is_executor_paid": true})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"subtask_ids[1]": "invalid", "subtask_ids[2]": "required", "subtask_ids[3]": "duplicate"}, s.fields(raw))

	many := make([]string, 501)
	for i := range many {
		many[i] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
	}
	st, raw = post(map[string]any{"subtask_ids": many, "is_executor_paid": true})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_many", s.fields(raw)["subtask_ids"])

	// мусор / неверные типы / лишнее поле (strictJSON) — 400 BAD_REQUEST.
	for name, body := range map[string]any{
		"битый JSON":        `{"subtask_ids": [`,
		"не объект":         `[1, 2]`,
		"пустое тело":       ``,
		"флаг строкой":      map[string]any{"subtask_ids": []string{sub}, "is_executor_paid": "yes"},
		"ids строкой":       map[string]any{"subtask_ids": sub, "is_executor_paid": true},
		"ids числами":       map[string]any{"subtask_ids": []int{1}, "is_executor_paid": true},
		"неизвестное поле":  map[string]any{"subtask_ids": []string{sub}, "is_executor_paid": true, "executor_payout": 5},
		"null вместо флага": `{"subtask_ids": ["` + sub + `"], "is_executor_paid": null}`,
	} {
		st, raw = post(body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
	}

	paid, _ := s.state(sub)
	s.False(paid, "ни один невалидный запрос услугу не отметил")
}

// ---- доступ ----

// TS-017: без сессии — 401 на всех эндпоинтах; executor имеет доступ к отчёту и отметке
// (в т.ч. чужой услуги — права как у admin, D62).
func (s *ExecutorPayoutsLiveSuite) TestTS017_Access_NoSession401_ExecutorAllowed() {
	ownerID, _ := s.newExecutor("TS017 Владелец услуги")
	_, login := s.newExecutor("TS017 Другой исполнитель")
	sub := s.newSubtask(s.newTask("TS017 заказ"), subtaskFx{status: "done", assignee: ownerID, position: 1})

	anon := s.NewBrowser()
	for _, req := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/executor-payouts", nil},
		{http.MethodGet, "/api/v1/executor-payouts/summary", nil},
		{http.MethodPost, "/api/v1/executor-payouts/mark", map[string]any{"subtask_ids": []string{sub}, "is_executor_paid": true}},
	} {
		st, raw := anon.Do(req.method, req.path, req.body)
		s.Equal(http.StatusUnauthorized, st, "%s %s: %s", req.method, req.path, raw)
	}
	paid, _ := s.state(sub)
	s.False(paid, "аноним ничего не отметил")

	exec := s.NewBrowser()
	exec.Login(login, "password1")
	s.Equal([]string{sub}, ids(s.list(exec, "assignee="+ownerID).Items))
	s.Require().Len(s.summary(exec, "assignee="+ownerID).Items, 1)
	st, m, raw := s.mark(exec, []string{sub}, true)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(1, m.Updated)
	paid, _ = s.state(sub)
	s.True(paid)
}

// TS-024: SSR-страница «Выплаты» — admin и executor (200, оболочка, пункт сайдбара,
// скрипт); без сессии — редирект на вход.
func (s *ExecutorPayoutsLiveSuite) TestTS024_SSR_Page() {
	page := func(b *testutil.Browser) (int, string) {
		resp := b.Request(http.MethodGet, "/executor-payouts", nil, nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.Require().NoError(err)
		return resp.StatusCode, string(body)
	}

	st, body := page(s.admin)
	s.Equal(http.StatusOK, st)
	s.Contains(body, `data-testid="ep-root"`)
	s.Contains(body, `data-is-admin="true"`)
	s.Contains(body, `href="/executor-payouts"`, "пункт «Выплаты» в сайдбаре")
	s.Contains(body, `/assets/ssr-shared/js/executor-payouts.js`)

	_, login := s.newExecutor("TS024 Исполнитель")
	exec := s.NewBrowser()
	exec.Login(login, "password1")
	st, body = page(exec)
	s.Equal(http.StatusOK, st)
	s.Contains(body, `data-is-admin="false"`)
	s.Contains(body, `href="/executor-payouts"`, "пункт «Выплаты» виден и исполнителю")

	// доска несёт фильтр «Выплата исполнителю».
	resp := s.admin.Request(http.MethodGet, "/", nil, nil)
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Contains(string(raw), `id="filter-executor-paid"`)

	resp = s.NewBrowser().Request(http.MethodGet, "/executor-payouts", nil, nil)
	resp.Body.Close()
	s.Contains([]int{http.StatusFound, http.StatusSeeOther}, resp.StatusCode, "без сессии — редирект на вход")
	s.Contains(resp.Header.Get("Location"), "/login")
}

// ---- правка суммы выплаты из отчёта (D72) ----

func (s *ExecutorPayoutsLiveSuite) putPayout(b *testutil.Browser, subtaskID string, body any) (int, payoutItemDTO, []byte) {
	st, raw := b.Do(http.MethodPut, "/api/v1/executor-payouts/"+subtaskID, body)
	var it payoutItemDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &it), string(raw))
	}
	return st, it, raw
}

// row — поля услуги из БД, которые правка суммы трогать не должна, + сама сумма.
func (s *ExecutorPayoutsLiveSuite) row(subtaskID string) (payout *string, paid bool, paidAt *string, price *string, status string, assignee *string) {
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT executor_payout::text, is_executor_paid, executor_paid_at::text, price::text, status, assignee_id::text
		FROM subtasks WHERE id = $1`, subtaskID).Scan(&payout, &paid, &paidAt, &price, &status, &assignee))
	return
}

// TS-029: PUT /executor-payouts/:id меняет ТОЛЬКО сумму (admin и executor): ответ — строка
// отчёта; флаг, дата отметки, цена, статус и исполнитель целы; null очищает; итоги summary
// отражают новую сумму.
func (s *ExecutorPayoutsLiveSuite) TestTS029_UpdatePayout_OnlySumChanges() {
	ownerID, _ := s.newExecutor("TS029 Исполнитель")
	_, otherLogin := s.newExecutor("TS029 Другой")
	task := s.newTask("TS029 заказ")
	paidSub := s.newSubtask(task, subtaskFx{status: "done", assignee: ownerID, price: "1000", payout: "100", paid: true, position: 1})
	unpaidSub := s.newSubtask(task, subtaskFx{status: "in_progress", assignee: ownerID, position: 2})
	_, _, paidAtBefore, _, _, _ := s.row(paidSub)
	s.Require().NotNil(paidAtBefore)

	// admin: выплаченная услуга — сумма меняется, отметка и дата остаются.
	st, it, raw := s.putPayout(s.admin, paidSub, map[string]any{"executor_payout": 250.5})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(paidSub, it.SubtaskID)
	s.Equal(task, it.TaskID)
	s.Equal("TS029 заказ", it.TaskTitle)
	s.Equal("EP Клиент", it.ClientName)
	s.Equal("EP диагностика", it.ServiceName)
	s.Equal("done", it.Status)
	s.Equal(ownerID, it.AssigneeID)
	s.Equal("TS029 Исполнитель", it.AssigneeName)
	s.Require().NotNil(it.ExecutorPayout)
	s.Equal("250.50", it.ExecutorPayout.String())
	s.Require().NotNil(it.Price)
	s.Equal("1000.00", it.Price.String())
	s.True(it.IsExecutorPaid)
	s.NotNil(it.ExecutorPaidAt)

	payout, paid, paidAt, price, status, assignee := s.row(paidSub)
	s.Equal("250.50", *payout)
	s.True(paid, "флаг не тронут")
	s.Equal(*paidAtBefore, *paidAt, "дата отметки не тронута")
	s.Equal("1000.00", *price, "цена для клиента не тронута")
	s.Equal("done", status)
	s.Equal(ownerID, *assignee)

	// строка списка и итоги отражают новую сумму.
	l := s.list(s.admin, "assignee="+ownerID)
	s.Require().Len(l.Items, 2)
	s.Equal("250.50", l.Items[0].ExecutorPayout.String())
	sum := s.summary(s.admin, "assignee="+ownerID)
	s.Require().Len(sum.Items, 1)
	s.Equal("250.50", sum.Items[0].Paid.String())
	s.Equal("0.00", sum.Items[0].ToPay.String())

	// executor (не владелец услуги): невыплаченная услуга без суммы — задать, поправить, 0.
	exec := s.NewBrowser()
	exec.Login(otherLogin, "password1")
	st, it, raw = s.putPayout(exec, strings.ToUpper(unpaidSub), map[string]any{"executor_payout": 75})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(unpaidSub, it.SubtaskID, "uuid в пути — в любом регистре")
	s.Equal("75.00", it.ExecutorPayout.String())
	s.False(it.IsExecutorPaid, "сумма не ставит отметку")
	s.Nil(it.ExecutorPaidAt)
	s.Nil(it.Price, "цена «по факту» осталась null")
	s.Equal("75.00", s.summary(s.admin, "assignee="+ownerID).Items[0].ToPay.String())

	st, it, raw = s.putPayout(exec, unpaidSub, map[string]any{"executor_payout": 0})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0.00", it.ExecutorPayout.String(), "0 — допустимая сумма")

	// null очищает сумму; отметка выплаченной услуги при этом остаётся.
	st, it, raw = s.putPayout(s.admin, paidSub, map[string]any{"executor_payout": nil})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Nil(it.ExecutorPayout)
	s.True(it.IsExecutorPaid)
	payout, paid, paidAt, _, _, _ = s.row(paidSub)
	s.Nil(payout)
	s.True(paid)
	s.Equal(*paidAtBefore, *paidAt)
	s.Equal("0.00", s.summary(s.admin, "assignee="+ownerID).Items[0].Paid.String(), "без суммы — 0 в итогах")

	// повтор того же значения — идемпотентно.
	st, it, raw = s.putPayout(s.admin, paidSub, map[string]any{"executor_payout": nil})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Nil(it.ExecutorPayout)
}

// TS-030: PUT /executor-payouts/:id — отказы: нет поля/невалидная сумма/мусор/лишнее поле →
// 400; битый uuid в пути → 400 по полю id; нет услуги или услуга без исполнителя → 404; без
// сессии → 401. Ни один отказ сумму не меняет.
func (s *ExecutorPayoutsLiveSuite) TestTS030_UpdatePayout_Rejections() {
	execID, _ := s.newExecutor("TS030 Исполнитель")
	task := s.newTask("TS030 заказ")
	sub := s.newSubtask(task, subtaskFx{status: "done", assignee: execID, payout: "40", position: 1})
	orphan := s.newSubtask(task, subtaskFx{status: "done", payout: "55", position: 2})
	const ghost = "99999999-9999-4999-8999-999999999998"

	st, _, raw := s.putPayout(s.admin, sub, map[string]any{})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("VALIDATION", s.errOf(raw).Code)
	s.Equal(map[string]string{"executor_payout": "required"}, s.fields(raw))

	for name, v := range map[string]any{"отрицательная": -1, "3 знака": 1.234, "слишком большая": 10000000000} {
		st, _, raw = s.putPayout(s.admin, sub, map[string]any{"executor_payout": v})
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Contains(s.fields(raw), "executor_payout", name)
	}

	// мусор / неверные типы / лишнее поле (strictJSON) — 400, не 500.
	for name, body := range map[string]any{
		"строка":            map[string]any{"executor_payout": "abc"},
		"объект":            map[string]any{"executor_payout": map[string]any{}},
		"битый JSON":        `{"executor_payout": `,
		"пустое тело":       ``,
		"не объект":         `[1]`,
		"лишнее поле флага": map[string]any{"executor_payout": 5, "is_executor_paid": true},
		"лишнее поле цены":  map[string]any{"executor_payout": 5, "price": 1},
	} {
		st, _, raw = s.putPayout(s.admin, sub, body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
	}

	// битый uuid в пути.
	for _, id := range []string{"not-a-uuid", "123"} {
		st, _, raw = s.putPayout(s.admin, id, map[string]any{"executor_payout": 5})
		s.Equal(http.StatusBadRequest, st, "%s: %s", id, raw)
		s.Equal("invalid", s.fields(raw)["id"], id)
	}

	// услуги нет / услуга без исполнителя (не строка отчёта) → 404.
	st, _, raw = s.putPayout(s.admin, ghost, map[string]any{"executor_payout": 5})
	s.Require().Equal(http.StatusNotFound, st, string(raw))
	s.Equal("EXECUTOR_PAYOUT_NOT_FOUND", s.errOf(raw).Code)
	st, _, raw = s.putPayout(s.admin, orphan, map[string]any{"executor_payout": 5})
	s.Require().Equal(http.StatusNotFound, st, string(raw))
	s.Equal("EXECUTOR_PAYOUT_NOT_FOUND", s.errOf(raw).Code)

	// без сессии.
	st, _, raw = s.putPayout(s.NewBrowser(), sub, map[string]any{"executor_payout": 5})
	s.Equal(http.StatusUnauthorized, st, string(raw))

	payout, _, _, _, _, _ := s.row(sub)
	s.Equal("40.00", *payout, "отказы сумму не изменили")
	payout, _, _, _, _, _ = s.row(orphan)
	s.Equal("55.00", *payout, "сумма услуги без исполнителя не тронута")
}

// TS-032: конкуренция правки суммы из отчёта с отметкой (POST /mark) и с правкой карточки
// (PUT /subtasks/:id без полей выплаты) — без deadlock/500, каждое изменение на месте.
func (s *ExecutorPayoutsLiveSuite) TestTS032_UpdatePayout_ConcurrentWithMarkAndCardPut() {
	execID, _ := s.newExecutor("TS032 Исполнитель")
	sub := s.newSubtask(s.newTask("TS032 заказ"), subtaskFx{status: "done", assignee: execID, price: "10", position: 1})
	second := s.NewBrowser()
	second.Login("ep-admin", "password1")

	const iterations = 20
	for i := 1; i <= iterations; i++ {
		target := i%2 == 1
		var sumSt, markSt, cardSt int
		var sumRaw, markRaw, cardRaw []byte
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			sumSt, sumRaw = s.admin.Do(http.MethodPut, "/api/v1/executor-payouts/"+sub, map[string]any{"executor_payout": 100 + i})
		}()
		go func() {
			defer wg.Done()
			markSt, markRaw = second.Do(http.MethodPost, "/api/v1/executor-payouts/mark",
				map[string]any{"subtask_ids": []string{sub}, "is_executor_paid": target})
		}()
		go func() {
			defer wg.Done()
			cardSt, cardRaw = second.Do(http.MethodPut, "/api/v1/subtasks/"+sub,
				map[string]any{"status": "done", "assignee_id": execID, "price": 500 + i})
		}()
		wg.Wait()
		s.Require().Equal(http.StatusOK, sumSt, "итерация %d сумма: %s", i, sumRaw)
		s.Require().Equal(http.StatusOK, markSt, "итерация %d mark: %s", i, markRaw)
		s.Require().Equal(http.StatusOK, cardSt, "итерация %d карточка: %s", i, cardRaw)

		payout, paid, paidAt, price, _, assignee := s.row(sub)
		s.Require().Equal(fmt.Sprintf("%d.00", 100+i), *payout, "итерация %d: сумма из отчёта", i)
		s.Require().Equal(target, paid, "итерация %d: отметка из mark", i)
		s.Require().Equal(target, paidAt != nil, "итерация %d: дата согласована с флагом", i)
		s.Require().Equal(fmt.Sprintf("%d.00", 500+i), *price, "итерация %d: цена из карточки", i)
		s.Require().Equal(execID, *assignee)
	}
}
