//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Выплаты исполнителям в карточке заказа и на доске (executor-payouts, docs/plan.md
// D62–D68; сценарии TS-001…TS-010 — docs/features/executor-payouts/0-setup/light-progress.md).
// Методы OrderCardLiveSuite — общие фикстуры карточки (order_card_http_live_test.go):
// реальный бинарь + TCP + реальный логин (§TD-8).

// payoutExecutor — активный исполнитель + залогиненный браузер.
func (s *OrderCardLiveSuite) payoutExecutor(login string) (string, *testutil.Browser) {
	id := s.SeedUser(login, "password1", "Исполнитель "+login, "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return id, b
}

// payoutSubtask — заказ в «Приёмка» с одной услугой (цена 1000); возвращает id заказа и услуги.
func (s *OrderCardLiveSuite) payoutSubtask(company string) (int64, string) {
	taskID, _ := s.newTask(s.newClient(company), s.columnID("Приёмка"))
	return taskID, s.addSubtask(taskID, s.newService("1000"), "1000")
}

// setPayoutState — состояние услуги напрямую SQL (фикстуры фильтра/метки доски).
func (s *OrderCardLiveSuite) setPayoutState(subtaskID, status string, assigneeID *string, paid bool) {
	_, err := s.Pool.Exec(context.Background(), `
		UPDATE subtasks SET status = $2, assignee_id = $3::uuid, is_executor_paid = $4,
		       executor_paid_at = CASE WHEN $4 THEN now() END
		WHERE id = $1`, subtaskID, status, assigneeID, paid)
	s.Require().NoError(err)
}

func (s *OrderCardLiveSuite) putSubtask(b *testutil.Browser, subtaskID string, body map[string]any) (int, cardSubtaskDTO, []byte) {
	st, m, raw := s.doCard(b, http.MethodPut, "/api/v1/subtasks/"+subtaskID, body)
	var sub cardSubtaskDTO
	for _, x := range m.Card.Subtasks {
		if x.ID == subtaskID {
			sub = x
		}
	}
	return st, sub, raw
}

// payoutRow — отметка/дата/сумма услуги из БД (проверка «ничего не изменилось»).
func (s *OrderCardLiveSuite) payoutRow(subtaskID string) (paid bool, paidAt *string, payout *string, assigneeID *string) {
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT is_executor_paid, executor_paid_at::text, executor_payout::text, assignee_id::text
		FROM subtasks WHERE id = $1`, subtaskID).Scan(&paid, &paidAt, &payout, &assigneeID))
	return
}

type payoutBoardItemDTO struct {
	ID                 int64 `json:"id"`
	ExecutorUnpaidDone int   `json:"executor_unpaid_done"`
}

func (s *OrderCardLiveSuite) payoutBoard(query string) map[int64]int {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?assignee=all&"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []payoutBoardItemDTO `json:"items"`
		Total int                  `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	out := map[int64]int{}
	for _, it := range l.Items {
		out[it.ID] = it.ExecutorUnpaidDone
	}
	s.Len(out, l.Total, "total = числу заказов под фильтром")
	return out
}

func payoutKeys(m map[int64]int) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// TS-001: admin отмечает выплату услуги с исполнителем — флаг, дата и сумма сохранены и
// видны в карточке (GET /card — тот же снимок).
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS001_AdminMarksPaid() {
	execID, _ := s.payoutExecutor("payout-ts001")
	taskID, subID := s.payoutSubtask("Клиент-Выплата-TS001")

	st, sub, raw := s.putSubtask(s.admin, subID, map[string]any{
		"status": "done", "assignee_id": execID, "price": 1000,
		"is_executor_paid": true, "executor_payout": 1500.5,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sub.IsExecutorPaid)
	s.Require().NotNil(sub.ExecutorPaidAt, "дата отметки ставится при false→true")
	s.Require().NotNil(sub.ExecutorPayout)
	s.Equal("1500.50", sub.ExecutorPayout.String())
	s.Require().NotNil(sub.Price)
	s.Equal("1000.00", sub.Price.String(), "цена для клиента — отдельное поле")

	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(card.Subtasks, 1)
	s.True(card.Subtasks[0].IsExecutorPaid)
	s.Equal(*sub.ExecutorPaidAt, *card.Subtasks[0].ExecutorPaidAt)
}

// TS-002: права — executor (не admin) отмечает выплату, правит сумму и снимает отметку.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS002_ExecutorAllowed() {
	execID, exec := s.payoutExecutor("payout-ts002")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS002")

	st, sub, raw := s.putSubtask(exec, subID, map[string]any{
		"status": "done", "assignee_id": execID, "is_executor_paid": true, "executor_payout": 300,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sub.IsExecutorPaid)
	s.Equal("300.00", sub.ExecutorPayout.String())

	st, sub, raw = s.putSubtask(exec, subID, map[string]any{
		"status": "done", "assignee_id": execID, "is_executor_paid": false, "executor_payout": 450,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(sub.IsExecutorPaid)
	s.Nil(sub.ExecutorPaidAt)
	s.Equal("450.00", sub.ExecutorPayout.String())
}

// TS-003: отметить выплату у услуги без исполнителя нельзя — 400 по полю, в БД без изменений.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS003_NoAssignee400() {
	_, subID := s.payoutSubtask("Клиент-Выплата-TS003")

	st, _, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "done", "is_executor_paid": true})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("VALIDATION", s.errOf(raw).Code)
	s.Equal("no_assignee", s.fields(raw)["is_executor_paid"])

	// исполнитель снимается тем же запросом, которым ставится отметка — тоже 400.
	execID, _ := s.payoutExecutor("payout-ts003")
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "assignee_id": execID})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "assignee_id": nil, "is_executor_paid": true})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("no_assignee", s.fields(raw)["is_executor_paid"])

	paid, paidAt, _, assignee := s.payoutRow(subID)
	s.False(paid)
	s.Nil(paidAt)
	s.Require().NotNil(assignee, "отклонённый запрос исполнителя не снял")
	s.Equal(execID, *assignee)
}

// TS-004: у выплаченной услуги исполнителя не сменить и не снять — 409 SUBTASK_EXECUTOR_PAID;
// со снятием отметки (тем же запросом или заранее) — можно.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS004_ChangeAssigneeOfPaid409() {
	execA, _ := s.payoutExecutor("payout-ts004-a")
	execB, _ := s.payoutExecutor("payout-ts004-b")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS004")

	st, _, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execA, "is_executor_paid": true})
	s.Require().Equal(http.StatusOK, st, string(raw))

	for name, body := range map[string]map[string]any{
		"смена, флаг не прислан": {"status": "done", "assignee_id": execB},
		"смена, флаг true":       {"status": "done", "assignee_id": execB, "is_executor_paid": true},
		"снятие исполнителя":     {"status": "done", "assignee_id": nil},
		"пустая строка":          {"status": "done", "assignee_id": ""},
	} {
		st, _, raw = s.putSubtask(s.admin, subID, body)
		s.Equal(http.StatusConflict, st, "%s: %s", name, raw)
		s.Equal("SUBTASK_EXECUTOR_PAID", s.errOf(raw).Code, name)
	}
	paid, _, _, assignee := s.payoutRow(subID)
	s.True(paid, "отклонённые запросы отметку не сняли")
	s.Equal(execA, *assignee)

	// прежний исполнитель + прочие поля — правятся свободно.
	st, sub, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execA, "price": 2000})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sub.IsExecutorPaid)

	// снять отметку и сменить исполнителя одним запросом — итог согласован.
	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execB, "is_executor_paid": false})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(sub.IsExecutorPaid)
	s.Equal(execB, *sub.AssigneeID)
}

// TS-005: executor_paid_at — ставится при false→true, не меняется при повторном true,
// обнуляется при снятии, ставится заново при новой отметке.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS005_PaidAtLifecycle() {
	execID, _ := s.payoutExecutor("payout-ts005")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS005")
	body := func(paid bool) map[string]any {
		return map[string]any{"status": "done", "assignee_id": execID, "is_executor_paid": paid}
	}

	st, sub, raw := s.putSubtask(s.admin, subID, body(true))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NotNil(sub.ExecutorPaidAt)
	first := *sub.ExecutorPaidAt

	// дата сдвинута в прошлое — повторный true не должен перезаписать её текущим временем.
	_, err := s.Pool.Exec(context.Background(), `UPDATE subtasks SET executor_paid_at = executor_paid_at - interval '2 hours' WHERE id = $1`, subID)
	s.Require().NoError(err)
	_, shifted, _, _ := s.payoutRow(subID)

	st, sub, raw = s.putSubtask(s.admin, subID, body(true))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NotNil(sub.ExecutorPaidAt)
	s.NotEqual(first, *sub.ExecutorPaidAt)
	_, again, _, _ := s.payoutRow(subID)
	s.Equal(*shifted, *again, "повторная отметка дату не трогает")

	st, sub, raw = s.putSubtask(s.admin, subID, body(false))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(sub.IsExecutorPaid)
	s.Nil(sub.ExecutorPaidAt, "снятие отметки обнуляет дату")

	st, sub, raw = s.putSubtask(s.admin, subID, body(true))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NotNil(sub.ExecutorPaidAt)
	_, fresh, _, _ := s.payoutRow(subID)
	s.NotEqual(*shifted, *fresh, "новая отметка — новая дата")
}

// TS-006: сумма выплаты правится независимо от флага (и без исполнителя); null очищает;
// невалидная сумма — 400, сохранённое значение не меняется.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS006_PayoutIndependentOfFlag() {
	_, subID := s.payoutSubtask("Клиент-Выплата-TS006")

	st, sub, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "new", "executor_payout": 700})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(sub.IsExecutorPaid)
	s.Nil(sub.ExecutorPaidAt)
	s.Require().NotNil(sub.ExecutorPayout)
	s.Equal("700.00", sub.ExecutorPayout.String())

	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "executor_payout": 0})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0.00", sub.ExecutorPayout.String(), "0 — допустимая сумма")

	for name, v := range map[string]any{"отрицательная": -1, "3 знака": 1.234, "слишком большая": 10000000000} {
		st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "executor_payout": v})
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Contains(s.fields(raw), "executor_payout", name)
	}
	// не число вовсе — 400 (не 500).
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "executor_payout": "abc"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "is_executor_paid": "yes"})
	s.Equal(http.StatusBadRequest, st, string(raw))

	_, _, payout, _ := s.payoutRow(subID)
	s.Require().NotNil(payout)
	s.Equal("0.00", *payout, "отклонённые запросы сумму не изменили")

	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "new", "executor_payout": nil})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Nil(sub.ExecutorPayout, "null — сумма не задана")
}

// TS-007: PUT без is_executor_paid и executor_payout не меняет ни флаг, ни дату, ни сумму
// (клиенты до фичи шлют строку без них; у полей есть второй писатель — отчёт, D65).
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS007_FlagOmittedKeepsValue() {
	execID, _ := s.payoutExecutor("payout-ts007")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS007")

	st, sub, raw := s.putSubtask(s.admin, subID, map[string]any{
		"status": "in_progress", "assignee_id": execID, "is_executor_paid": true, "executor_payout": 900,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	paidAt := *sub.ExecutorPaidAt

	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execID, "price": 1200})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sub.IsExecutorPaid, "флаг сохранён")
	s.Require().NotNil(sub.ExecutorPaidAt)
	s.Equal(paidAt, *sub.ExecutorPaidAt, "дата отметки не тронута")
	s.Equal("done", sub.Status)
	s.Require().NotNil(sub.ExecutorPayout, "поле не прислано → сумма не тронута")
	s.Equal("900.00", sub.ExecutorPayout.String())
	s.Require().NotNil(sub.Price)
	s.Equal("1200.00", sub.Price.String())
}

// TS-008: фильтр доски executor_paid. unpaid — есть услуга с исполнителем без выплаты;
// paid — услуги с исполнителем есть и все выплачены; заказ без исполнителей услуг — ни там,
// ни там. Совместим с окном per_column/column_id.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS008_BoardFilter() {
	execID, _ := s.payoutExecutor("payout-ts008")
	clientID := s.newClient("Клиент-Выплата-TS008")
	col := s.columnID("Приёмка")
	svc := s.newService("100")
	mk := func() int64 { id, _ := s.newTask(clientID, col); return id }

	unpaidTask := mk() // услуга с исполнителем, не выплачена
	s.setPayoutState(s.addSubtask(unpaidTask, svc, "100"), "new", &execID, false)

	paidTask := mk() // услуга с исполнителем выплачена + услуга без исполнителя
	s.setPayoutState(s.addSubtask(paidTask, svc, "100"), "done", &execID, true)
	s.setPayoutState(s.addSubtask(paidTask, svc, "100"), "new", nil, false)

	noExecutorTask := mk() // услуга без исполнителя
	s.addSubtask(noExecutorTask, svc, "100")

	mixedTask := mk() // одна выплачена, другая нет
	s.setPayoutState(s.addSubtask(mixedTask, svc, "100"), "done", &execID, true)
	s.setPayoutState(s.addSubtask(mixedTask, svc, "100"), "done", &execID, false)

	emptyTask := mk() // без услуг

	base := "client_id=" + clientID
	s.Equal([]int64{unpaidTask, mixedTask}, payoutKeys(s.payoutBoard(base+"&executor_paid=unpaid")))
	s.Equal([]int64{paidTask}, payoutKeys(s.payoutBoard(base+"&executor_paid=paid")))
	all := []int64{unpaidTask, paidTask, noExecutorTask, mixedTask, emptyTask}
	s.Equal(all, payoutKeys(s.payoutBoard(base+"&executor_paid=all")))
	s.Equal(all, payoutKeys(s.payoutBoard(base)), "без параметра — все")

	s.Equal([]int64{unpaidTask, mixedTask},
		payoutKeys(s.payoutBoard(base+"&executor_paid=unpaid&per_column=50&column_id="+col)), "совместим с per_column/column_id")
	// фильтры складываются: оплата заказа и выплата исполнителю — разные оси.
	s.Empty(s.payoutBoard(base + "&executor_paid=unpaid&paid=paid"))
}

// TS-009: невалидное значение executor_paid → 400 VALIDATION по полю.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS009_BoardFilterInvalid400() {
	for _, v := range []string{"yes", "PAID", "1", "paid%20"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?executor_paid="+v, nil)
		s.Equal(http.StatusBadRequest, st, "%s: %s", v, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code, v)
		s.Equal("invalid", s.fields(raw)["executor_paid"], v)
	}
}

// TS-010: метка доски executor_unpaid_done — услуги done с исполнителем без выплаты; в
// GET /tasks и GET /tasks/:id; после отметки выплаты — 0.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS010_BoardBadgeCount() {
	execID, _ := s.payoutExecutor("payout-ts010")
	clientID := s.newClient("Клиент-Выплата-TS010")
	taskID, _ := s.newTask(clientID, s.columnID("Приёмка"))
	svc := s.newService("100")

	counted := s.addSubtask(taskID, svc, "100")
	s.setPayoutState(counted, "done", &execID, false)                                 // считается
	s.setPayoutState(s.addSubtask(taskID, svc, "100"), "done", &execID, true)         // выплачена
	s.setPayoutState(s.addSubtask(taskID, svc, "100"), "done", nil, false)            // без исполнителя
	s.setPayoutState(s.addSubtask(taskID, svc, "100"), "in_progress", &execID, false) // не выполнена
	quiet, _ := s.newTask(clientID, s.columnID("Приёмка"))

	board := s.payoutBoard("client_id=" + clientID)
	s.Equal(1, board[taskID])
	s.Equal(0, board[quiet], "заказ без услуг — метки нет")

	one := func() payoutBoardItemDTO {
		st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", taskID), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var it payoutBoardItemDTO
		s.Require().NoError(json.Unmarshal(raw, &it), string(raw))
		return it
	}
	s.Equal(1, one().ExecutorUnpaidDone, "GET /tasks/:id несёт ту же метку")

	st, _, raw := s.putSubtask(s.admin, counted, map[string]any{"status": "done", "assignee_id": execID, "is_executor_paid": true})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(0, s.payoutBoard("client_id=" + clientID)[taskID])
	s.Equal(0, one().ExecutorUnpaidDone)
}

// markPayout — массовая отметка из отчёта «Выплаты» (второй писатель флага, D70).
func (s *OrderCardLiveSuite) markPayout(b *testutil.Browser, paid bool, subtaskIDs ...string) (int, []byte) {
	return b.Do(http.MethodPost, "/api/v1/executor-payouts/mark", map[string]any{"subtask_ids": subtaskIDs, "is_executor_paid": paid})
}

// TS-025 (code review HIGH-1/MEDIUM-1): отметка поставлена из отчёта, затем «устаревшая»
// карточка правит другое поле — PUT без is_executor_paid/executor_payout не трогает флаг,
// дату и сумму. Зеркально: отметка снята из отчёта — PUT её не возвращает.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS025_StaleCardKeepsMarkFromReport() {
	execID, _ := s.payoutExecutor("payout-ts025")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS025")
	st, _, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execID, "executor_payout": 640})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// отчёт: отметили выплату.
	st, raw = s.markPayout(s.admin, true, subID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, markedAt, _, _ := s.payoutRow(subID)
	s.Require().NotNil(markedAt)

	// карточка (открыта до отметки): срок, цена, статус — тело без полей выплаты.
	st, sub, raw := s.putSubtask(s.admin, subID, map[string]any{
		"status": "in_progress", "assignee_id": execID, "due_date": "2030-01-15", "price": 2500,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("in_progress", sub.Status)
	s.Equal("2500.00", sub.Price.String())
	s.True(sub.IsExecutorPaid, "отметка из отчёта цела")
	s.Require().NotNil(sub.ExecutorPayout)
	s.Equal("640.00", sub.ExecutorPayout.String(), "сумма цела")
	paid, paidAt, payout, _ := s.payoutRow(subID)
	s.True(paid)
	s.Equal(*markedAt, *paidAt, "дата отметки не изменилась")
	s.Equal("640.00", *payout)

	// отчёт: сняли отметку — устаревшая карточка её не возвращает.
	st, raw = s.markPayout(s.admin, false, subID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execID, "price": 2500})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(sub.IsExecutorPaid, "снятая отметка не вернулась")
	s.Nil(sub.ExecutorPaidAt)
	s.Equal("640.00", sub.ExecutorPayout.String())

	// сумма: явный null очищает, число ставит — флаг при этом не трогается.
	st, raw = s.markPayout(s.admin, true, subID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execID, "executor_payout": nil})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Nil(sub.ExecutorPayout, "явный null — очистить")
	s.True(sub.IsExecutorPaid)
	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execID, "executor_payout": 15})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("15.00", sub.ExecutorPayout.String())
	s.True(sub.IsExecutorPaid)
}

// TS-026 (code review MEDIUM-3): конкуренция двух писателей одной услуги — POST /mark и
// PUT /subtasks/:id другого поля идут параллельно. Без deadlock/500; при любом порядке
// отметка из mark цела, правка PUT применена.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS026_ConcurrentMarkAndPut() {
	execID, exec := s.payoutExecutor("payout-ts026")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS026")
	st, _, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execID, "executor_payout": 77})
	s.Require().Equal(http.StatusOK, st, string(raw))

	const iterations = 25
	for i := 1; i <= iterations; i++ {
		target := i%2 == 1 // чередуем отметку и снятие
		price := 1000 + i
		var markSt, putSt int
		var markRaw, putRaw []byte
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			markSt, markRaw = s.markPayout(exec, target, subID)
		}()
		go func() {
			defer wg.Done()
			putSt, putRaw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID,
				map[string]any{"status": "done", "assignee_id": execID, "price": price})
		}()
		wg.Wait()
		s.Require().Equal(http.StatusOK, markSt, "итерация %d mark: %s", i, markRaw)
		s.Require().Equal(http.StatusOK, putSt, "итерация %d put: %s", i, putRaw)

		paid, paidAt, payout, assignee := s.payoutRow(subID)
		s.Require().Equal(target, paid, "итерация %d: отметка из mark цела", i)
		s.Require().Equal(target, paidAt != nil, "итерация %d: дата согласована с флагом", i)
		s.Require().Equal("77.00", *payout, "итерация %d: сумма цела", i)
		s.Require().Equal(execID, *assignee)
		var gotPrice string
		s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT price::text FROM subtasks WHERE id = $1`, subID).Scan(&gotPrice))
		s.Require().Equal(fmt.Sprintf("%d.00", price), gotPrice, "итерация %d: правка PUT применена", i)
	}
}

// TS-027 (code review MEDIUM-3): mark ставит отметку ↔ PUT снимает исполнителя. Исход
// зависит от порядка, но всегда согласован: либо PUT первым (mark → 400 no_assignee),
// либо mark первым (PUT → 409 SUBTASK_EXECUTOR_PAID); 500 и «выплачено без исполнителя»
// не бывает.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS027_ConcurrentMarkAndUnassign() {
	execID, _ := s.payoutExecutor("payout-ts027")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS027")

	const iterations = 20
	for i := 1; i <= iterations; i++ {
		s.setPayoutState(subID, "done", &execID, false)
		var markSt, putSt int
		var markRaw, putRaw []byte
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			markSt, markRaw = s.markPayout(s.admin, true, subID)
		}()
		go func() {
			defer wg.Done()
			putSt, putRaw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{"status": "done", "assignee_id": nil})
		}()
		wg.Wait()

		paid, _, _, assignee := s.payoutRow(subID)
		switch {
		case markSt == http.StatusOK: // mark первым — снять исполнителя уже нельзя
			s.Require().Equal(http.StatusConflict, putSt, "итерация %d put: %s", i, putRaw)
			s.Require().Equal("SUBTASK_EXECUTOR_PAID", s.errOf(putRaw).Code)
			s.Require().True(paid)
			s.Require().NotNil(assignee)
		case putSt == http.StatusOK: // PUT первым — отметить без исполнителя нельзя
			s.Require().Equal(http.StatusBadRequest, markSt, "итерация %d mark: %s", i, markRaw)
			s.Require().Equal("no_assignee", s.fields(markRaw)["subtask_ids[0]"])
			s.Require().False(paid)
			s.Require().Nil(assignee)
		default:
			s.Require().Failf("оба запроса отклонены", "итерация %d: mark %d %s; put %d %s", i, markSt, markRaw, putSt, putRaw)
		}
	}
}

// TS-028 (code review MEDIUM-4): выплаченная услуга осталась без исполнителя (сотрудника
// удалили — assignee_id ON DELETE SET NULL). Она правится без 400, ей можно назначить
// исполнителя; после назначения снова действует 409 на смену.
func (s *OrderCardLiveSuite) TestExecutorPayouts_TS028_OrphanPaidSubtaskEditable() {
	execA, _ := s.payoutExecutor("payout-ts028-a")
	execB, _ := s.payoutExecutor("payout-ts028-b")
	_, subID := s.payoutSubtask("Клиент-Выплата-TS028")
	_, err := s.Pool.Exec(context.Background(), `
		UPDATE subtasks SET status = 'done', assignee_id = NULL, is_executor_paid = true,
		       executor_paid_at = now() - interval '1 day', executor_payout = 320
		WHERE id = $1`, subID)
	s.Require().NoError(err)
	_, orphanAt, _, _ := s.payoutRow(subID)

	// правка другого поля, флаг не прислан — 200, выплата не тронута.
	st, sub, raw := s.putSubtask(s.admin, subID, map[string]any{"status": "done", "price": 1800})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sub.IsExecutorPaid)
	s.Equal("1800.00", sub.Price.String())
	s.Equal("320.00", sub.ExecutorPayout.String())
	// тот же флаг true явно (не переход false→true) — тоже 200.
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "is_executor_paid": true})
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, at, _, _ := s.payoutRow(subID)
	s.Equal(*orphanAt, *at, "дата отметки не изменилась")

	// назначить исполнителя «осиротевшей» выплаченной услуге можно (NULL → uuid).
	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execA})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sub.IsExecutorPaid)
	s.Equal(execA, *sub.AssigneeID)

	// теперь исполнитель есть — смена/снятие при остающейся отметке снова 409.
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": execB})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("SUBTASK_EXECUTOR_PAID", s.errOf(raw).Code)
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "assignee_id": nil})
	s.Equal(http.StatusConflict, st, string(raw))

	// снятие отметки у услуги без исполнителя и повторная отметка без исполнителя → 400.
	_, err = s.Pool.Exec(context.Background(), `UPDATE subtasks SET assignee_id = NULL WHERE id = $1`, subID)
	s.Require().NoError(err)
	st, sub, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "is_executor_paid": false})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(sub.IsExecutorPaid)
	st, _, raw = s.putSubtask(s.admin, subID, map[string]any{"status": "done", "is_executor_paid": true})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("no_assignee", s.fields(raw)["is_executor_paid"])
}
