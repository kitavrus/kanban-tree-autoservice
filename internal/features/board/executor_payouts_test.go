package board

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Юнит-тесты выплат исполнителям (executor-payouts, TS-020): формат executor_payout в
// PUT /subtasks/:id и разбор query executor_paid списка заказов.

func subtaskFieldMap(in SubtaskUpdateRequest) map[string]string {
	got := map[string]string{}
	for _, f := range in.normalize().validate() {
		got[f.Name] = f.Message
	}
	return got
}

func amountPtr(v string) *numeric.Amount {
	a := numeric.Amount(v)
	return &a
}

// TS-020/TS-006: executor_payout — NUMERIC(12,2) ≥ 0; nil (нет поля/null) валиден.
func TestTS020_SubtaskUpdateRequestValidate_ExecutorPayout(t *testing.T) {
	paid := true
	ok := []SubtaskUpdateRequest{
		{Status: SubtaskStatusDone},
		{Status: SubtaskStatusDone, ExecutorPayout: OptionalAmount{Set: true, Value: amountPtr("0")}},
		{Status: SubtaskStatusDone, ExecutorPayout: OptionalAmount{Set: true, Value: amountPtr("1500.50")}, IsExecutorPaid: &paid},
		{Status: SubtaskStatusDone, ExecutorPayout: OptionalAmount{Set: true, Value: amountPtr("9999999999.99")}},
	}
	for _, in := range ok {
		assert.NotContains(t, subtaskFieldMap(in), "executor_payout", "payout=%v", in.ExecutorPayout.Value)
	}

	bad := map[string]string{
		"1.234":       numeric.MsgTooPrecise,
		"10000000000": numeric.MsgTooLarge,
		"abc":         numeric.MsgInvalidFormat,
		"1e3":         numeric.MsgInvalidFormat,
	}
	for raw, want := range bad {
		got := subtaskFieldMap(SubtaskUpdateRequest{Status: SubtaskStatusDone, ExecutorPayout: OptionalAmount{Set: true, Value: amountPtr(raw)}})
		assert.Equal(t, want, got["executor_payout"], "payout=%q", raw)
	}
	// отрицательная сумма — ошибка поля (точный код — pkg/numeric).
	neg := subtaskFieldMap(SubtaskUpdateRequest{Status: SubtaskStatusDone, ExecutorPayout: OptionalAmount{Set: true, Value: amountPtr("-1")}})
	assert.NotEmpty(t, neg["executor_payout"])
}

// TS-020/TS-008: executor_paid — all (и отсутствие параметра) → без фильтра; paid/unpaid.
func TestTS020_ParseTaskFilter_ExecutorPaid(t *testing.T) {
	for raw, want := range map[string]string{
		"":                     "",
		"executor_paid=all":    "",
		"executor_paid=paid":   PaidFilterPaid,
		"executor_paid=unpaid": PaidFilterUnpaid,
	} {
		f, err := parseFilter(t, raw, admin)
		require.NoError(t, err, raw)
		assert.Equal(t, want, f.ExecutorPaid, raw)
	}
}

// TS-020/TS-009: мусорное значение executor_paid → VALIDATION по полю executor_paid.
func TestTS020_ParseTaskFilter_ExecutorPaidInvalid(t *testing.T) {
	for _, raw := range []string{"executor_paid=yes", "executor_paid=PAID", "executor_paid=1", "executor_paid=%00"} {
		_, err := parseFilter(t, raw, admin)
		var ae *errorspkg.AppError
		require.True(t, errors.As(err, &ae), raw)
		require.Len(t, ae.Fields, 1, raw)
		assert.Equal(t, "executor_paid", ae.Fields[0].Name)
		assert.Equal(t, msgInvalid, ae.Fields[0].Message)
	}
}

// TS-020: независимость от фильтра оплаты заказа — оба параметра разбираются раздельно.
func TestTS020_ParseTaskFilter_ExecutorPaidIndependentOfPaid(t *testing.T) {
	f, err := parseFilter(t, "paid=paid&executor_paid=unpaid", admin)
	require.NoError(t, err)
	require.NotNil(t, f.Paid)
	assert.True(t, *f.Paid)
	assert.Equal(t, PaidFilterUnpaid, f.ExecutorPaid)
}

// TS-020/TS-025: executor_payout различает «поля нет» / null / число (D65).
func TestTS020_SubtaskUpdateRequest_ExecutorPayoutPresence(t *testing.T) {
	var absent SubtaskUpdateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"status":"done"}`), &absent))
	assert.False(t, absent.ExecutorPayout.Set, "поля нет — сумму не менять")
	assert.Nil(t, absent.ExecutorPayout.Value)
	assert.Nil(t, absent.IsExecutorPaid)

	var null SubtaskUpdateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"status":"done","executor_payout":null}`), &null))
	assert.True(t, null.ExecutorPayout.Set, "явный null — очистить")
	assert.Nil(t, null.ExecutorPayout.Value)

	var set SubtaskUpdateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"status":"done","executor_payout":1500.50}`), &set))
	assert.True(t, set.ExecutorPayout.Set)
	require.NotNil(t, set.ExecutorPayout.Value)
	assert.Equal(t, "1500.50", set.ExecutorPayout.Value.String())

	// не число — ошибка разбора тела (хендлер отвечает 400 BAD_REQUEST).
	var bad SubtaskUpdateRequest
	assert.Error(t, json.Unmarshal([]byte(`{"status":"done","executor_payout":"abc"}`), &bad))
	assert.Error(t, json.Unmarshal([]byte(`{"status":"done","executor_payout":{}}`), &bad))
}
