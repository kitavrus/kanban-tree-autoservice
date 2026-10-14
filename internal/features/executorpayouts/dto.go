// Package executorpayouts — отчёт «Выплаты исполнителям» (D62–D70): список услуг заказов с
// назначенным исполнителем, итоги «к выплате / выплачено» по исполнителям и массовая
// отметка выплаты. Отдельный пакет, не board (как transfers, D50): отчёт — свой экран
// и свой ресурс API; правка одной услуги из карточки заказа остаётся в board
// (PUT /api/v1/subtasks/:id).
package executorpayouts

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Wire-типы — JSON только snake_case (§API-1), списки — {items,total,limit,offset}.

// PayoutItem — услуга заказа с исполнителем. Price — цена для клиента (null = «по факту»);
// ExecutorPayout — сумма выплаты исполнителю (null = не задана, в итогах считается как 0).
type PayoutItem struct {
	SubtaskID      string          `json:"subtask_id"`
	TaskID         int64           `json:"task_id"`
	TaskTitle      string          `json:"task_title"`
	ClientName     string          `json:"client_name"`
	ServiceName    string          `json:"service_name"`
	Status         string          `json:"status"` // new | in_progress | done
	AssigneeID     string          `json:"assignee_id"`
	AssigneeName   string          `json:"assignee_name"`
	Price          *numeric.Amount `json:"price"`
	ExecutorPayout *numeric.Amount `json:"executor_payout"`
	IsExecutorPaid bool            `json:"is_executor_paid"`
	ExecutorPaidAt *time.Time      `json:"executor_paid_at"`
}

// PayoutListResponse — GET /api/v1/executor-payouts. Порядок items: исполнитель (имя, id),
// заказ (id), позиция услуги в заказе, id услуги — услуги одного исполнителя идут подряд.
type PayoutListResponse struct {
	Items  []PayoutItem `json:"items"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

// PayoutSummaryItem — итоги одного исполнителя по тем же фильтрам, что и список (без учёта
// limit/offset списка). ToPay/Paid — суммы COALESCE(executor_payout, 0) невыплаченных/
// выплаченных услуг; счётчики — число таких услуг.
type PayoutSummaryItem struct {
	AssigneeID   string         `json:"assignee_id"`
	AssigneeName string         `json:"assignee_name"`
	UnpaidCount  int            `json:"unpaid_count"`
	PaidCount    int            `json:"paid_count"`
	ToPay        numeric.Amount `json:"to_pay"`
	Paid         numeric.Amount `json:"paid"`
}

// PayoutSummaryResponse — GET /api/v1/executor-payouts/summary. Порядок: имя, id.
type PayoutSummaryResponse struct {
	Items  []PayoutSummaryItem `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// MarkRequest — POST /api/v1/executor-payouts/mark: массовая отметка/снятие выплаты.
// IsExecutorPaid обязателен (указатель — отличить «нет поля» от false).
type MarkRequest struct {
	SubtaskIDs     []string `json:"subtask_ids"`
	IsExecutorPaid *bool    `json:"is_executor_paid"`
}

// MarkResponse — Updated: сколько услуг реально сменило отметку (уже стоявшие в нужном
// состоянии не считаются — повтор запроса идемпотентен и даёт 0).
type MarkResponse struct {
	Updated int `json:"updated"`
}

// optionalAmount — денежное поле запроса с признаком присутствия: поля нет (Set=false),
// явный null (Set=true, Value=nil) или число. Своя декларация, не импорт из board (фичи
// друг от друга не зависят). encoding/json зовёт UnmarshalJSON только для присутствующего
// поля, в т.ч. для null.
type optionalAmount struct {
	Set   bool
	Value *numeric.Amount
}

func (o *optionalAmount) UnmarshalJSON(b []byte) error {
	o.Set = true
	o.Value = nil
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return nil
	}
	var a numeric.Amount
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	o.Value = &a
	return nil
}

// PayoutUpdateRequest — PUT /api/v1/executor-payouts/:id (D72): правка ТОЛЬКО суммы выплаты
// услуги из отчёта. Поле обязательно: null — очистить сумму, число ≥ 0 — установить.
type PayoutUpdateRequest struct {
	ExecutorPayout optionalAmount `json:"executor_payout"`
}
