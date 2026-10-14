package executorpayouts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Юнит-тесты отчёта «Выплаты» (TS-020): разбор фильтров и валидация тела массовой отметки.

const (
	idA = "11111111-1111-1111-1111-111111111111"
	idB = "22222222-2222-2222-2222-222222222222"
)

var (
	adminActor    = Actor{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", IsAdmin: true}
	executorActor = Actor{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", IsAdmin: false}
)

func fieldMap(f []errorspkg.Field) map[string]string {
	m := map[string]string{}
	for _, x := range f {
		if _, ok := m[x.Name]; !ok {
			m[x.Name] = x.Message
		}
	}
	return m
}

func validationFields(t *testing.T, err error) map[string]string {
	t.Helper()
	var ae *errorspkg.AppError
	require.True(t, errors.As(err, &ae), "ожидалась *errorspkg.AppError, получено %v", err)
	assert.Equal(t, "VALIDATION", ae.Code)
	return fieldMap(ae.Fields)
}

func boolPtr(v bool) *bool { return &v }

// TS-020/TS-012: без assignee — executor видит «мои», admin — все (D69).
func TestTS020_ParseFilter_DefaultsByRole(t *testing.T) {
	f, err := parseFilter(ListQuery{}, executorActor)
	require.NoError(t, err)
	require.NotNil(t, f.AssigneeID)
	assert.Equal(t, executorActor.ID, *f.AssigneeID)
	assert.Nil(t, f.Paid)
	assert.False(t, f.OnlyDone)
	assert.Equal(t, defaultLimit, f.Limit)
	assert.Equal(t, 0, f.Offset)

	f, err = parseFilter(ListQuery{}, adminActor)
	require.NoError(t, err)
	assert.Nil(t, f.AssigneeID)
}

// TS-020/TS-012: assignee=me|all|<uuid> — явное значение сильнее роли.
func TestTS020_ParseFilter_Assignee(t *testing.T) {
	f, err := parseFilter(ListQuery{Assignee: AssigneeAll}, executorActor)
	require.NoError(t, err)
	assert.Nil(t, f.AssigneeID, "executor может смотреть всех")

	f, err = parseFilter(ListQuery{Assignee: AssigneeMe}, adminActor)
	require.NoError(t, err)
	require.NotNil(t, f.AssigneeID)
	assert.Equal(t, adminActor.ID, *f.AssigneeID)

	f, err = parseFilter(ListQuery{Assignee: " " + idA + " "}, adminActor)
	require.NoError(t, err)
	require.NotNil(t, f.AssigneeID)
	assert.Equal(t, idA, *f.AssigneeID)
}

// TS-020/TS-012: status и only_done.
func TestTS020_ParseFilter_StatusAndOnlyDone(t *testing.T) {
	f, err := parseFilter(ListQuery{Status: StatusPaid, OnlyDone: "true"}, adminActor)
	require.NoError(t, err)
	require.NotNil(t, f.Paid)
	assert.True(t, *f.Paid)
	assert.True(t, f.OnlyDone)

	f, err = parseFilter(ListQuery{Status: StatusUnpaid, OnlyDone: "false"}, adminActor)
	require.NoError(t, err)
	require.NotNil(t, f.Paid)
	assert.False(t, *f.Paid)
	assert.False(t, f.OnlyDone)

	f, err = parseFilter(ListQuery{Status: StatusAll}, adminActor)
	require.NoError(t, err)
	assert.Nil(t, f.Paid)
}

// TS-020/TS-012: мусор в любом параметре → VALIDATION по полю (все поля разом).
func TestTS020_ParseFilter_Invalid(t *testing.T) {
	_, err := parseFilter(ListQuery{Assignee: "nobody", Status: "done", OnlyDone: "1", Limit: "0", Offset: "-1"}, adminActor)
	got := validationFields(t, err)
	for _, name := range []string{"assignee", "status", "only_done", "limit", "offset"} {
		assert.Equal(t, msgInvalid, got[name], name)
	}

	_, err = parseFilter(ListQuery{Limit: fmt.Sprint(maxLimit + 1)}, adminActor)
	assert.Equal(t, msgInvalid, validationFields(t, err)["limit"])
	_, err = parseFilter(ListQuery{Limit: "abc"}, adminActor)
	assert.Equal(t, msgInvalid, validationFields(t, err)["limit"])

	f, err := parseFilter(ListQuery{Limit: fmt.Sprint(maxLimit), Offset: "7"}, adminActor)
	require.NoError(t, err)
	assert.Equal(t, maxLimit, f.Limit)
	assert.Equal(t, 7, f.Offset)
}

// TS-020/TS-016: валидное тело; uuid приводится к каноническому виду (trim + lower).
func TestTS020_MarkRequestValidate_OK(t *testing.T) {
	in := MarkRequest{SubtaskIDs: []string{" " + strings.ToUpper(idA) + " ", idB}, IsExecutorPaid: boolPtr(false)}.normalize()
	assert.Empty(t, in.validate())
	assert.Equal(t, []string{idA, idB}, in.SubtaskIDs)
}

// TS-020/TS-016: пустой список / нет флага.
func TestTS020_MarkRequestValidate_Required(t *testing.T) {
	got := fieldMap(MarkRequest{}.normalize().validate())
	assert.Equal(t, msgRequired, got["subtask_ids"])
	assert.Equal(t, msgRequired, got["is_executor_paid"])

	got = fieldMap(MarkRequest{SubtaskIDs: []string{}, IsExecutorPaid: boolPtr(true)}.normalize().validate())
	assert.Equal(t, msgRequired, got["subtask_ids"])
	assert.NotContains(t, got, "is_executor_paid")
}

// TS-020/TS-016: битый uuid, пустая строка, дубль (в т.ч. в другом регистре) — по индексу.
func TestTS020_MarkRequestValidate_InvalidItems(t *testing.T) {
	in := MarkRequest{
		SubtaskIDs:     []string{idA, "not-a-uuid", "  ", strings.ToUpper(idA), idB},
		IsExecutorPaid: boolPtr(true),
	}.normalize()
	got := fieldMap(in.validate())
	assert.Equal(t, map[string]string{
		"subtask_ids[1]": msgInvalid,
		"subtask_ids[2]": msgRequired,
		"subtask_ids[3]": msgDuplicate,
	}, got)
}

// TS-020/TS-016: граница размера списка — markMax допустим, markMax+1 → too_many.
func TestTS020_MarkRequestValidate_TooMany(t *testing.T) {
	ids := make([]string, markMax+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
	}
	got := fieldMap(MarkRequest{SubtaskIDs: ids, IsExecutorPaid: boolPtr(true)}.normalize().validate())
	assert.Equal(t, map[string]string{"subtask_ids": msgTooMany}, got)

	assert.Empty(t, MarkRequest{SubtaskIDs: ids[:markMax], IsExecutorPaid: boolPtr(true)}.normalize().validate())
}

func decodePayout(t *testing.T, body string) (PayoutUpdateRequest, error) {
	t.Helper()
	var in PayoutUpdateRequest
	err := json.Unmarshal([]byte(body), &in)
	return in, err
}

// TS-031: тело PUT /executor-payouts/:id — поле обязательно; null и число ≥ 0 валидны.
func TestTS031_PayoutUpdateRequestValidate(t *testing.T) {
	in, err := decodePayout(t, `{}`)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"executor_payout": msgRequired}, fieldMap(in.validate()))

	in, err = decodePayout(t, `{"executor_payout": null}`)
	require.NoError(t, err)
	assert.True(t, in.ExecutorPayout.Set)
	assert.Nil(t, in.ExecutorPayout.Value)
	assert.Empty(t, in.validate(), "null — очистить сумму")

	for _, ok := range []string{"0", "0.01", "1500.50", "9999999999.99"} {
		in, err = decodePayout(t, `{"executor_payout": `+ok+`}`)
		require.NoError(t, err, ok)
		assert.Empty(t, in.validate(), ok)
		require.NotNil(t, in.ExecutorPayout.Value)
		assert.Equal(t, ok, in.ExecutorPayout.Value.String())
	}

	for raw, want := range map[string]string{
		"1.234":       numeric.MsgTooPrecise,
		"10000000000": numeric.MsgTooLarge,
		"1e3":         numeric.MsgInvalidFormat,
		"true":        numeric.MsgInvalidFormat, // литерал проходит разбор тела, отсекается форматом
	} {
		in, err = decodePayout(t, `{"executor_payout": `+raw+`}`)
		require.NoError(t, err, raw)
		assert.Equal(t, want, fieldMap(in.validate())["executor_payout"], raw)
	}
	in, err = decodePayout(t, `{"executor_payout": -1}`)
	require.NoError(t, err)
	assert.NotEmpty(t, fieldMap(in.validate())["executor_payout"], "отрицательная сумма")

	// не число — ошибка разбора тела (хендлер отвечает 400 BAD_REQUEST).
	for _, bad := range []string{`"abc"`, `{}`, `[1]`} {
		_, err = decodePayout(t, `{"executor_payout": `+bad+`}`)
		assert.Error(t, err, bad)
	}
}
