package transfers

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Unit-сценарии части transfers-revert (docs/features/transfers-revert/draft-spec.md §11):
// TS-010 — причина отмены проведения и «свои» сообщения 409 отката; TS-012 — текст и details
// 409 TRANSFER_UNRECEIVE_BLOCKED; TS-016 — query-параметр version и сообщения 409 удаления;
// TS-009 (хвост) — новый текст отмены полученного документа.

func TestTS010_ValidateReason(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want string
		fields          map[string]string
	}{
		{"обычная", "ошибка в факте", "ошибка в факте", map[string]string{}},
		{"trim", "  \n ошибка\t ", "ошибка", map[string]string{}},
		{"пусто", "", "", map[string]string{"reason": msgRequired}},
		{"только пробелы", " \t\n ", "", map[string]string{"reason": msgRequired}},
		{"1 символ", "я", "я", map[string]string{}},
		{"ровно 500 символов (не байт)", strings.Repeat("я", reasonMax), strings.Repeat("я", reasonMax), map[string]string{}},
		{"500 символов после trim", " " + strings.Repeat("a", reasonMax) + " ", strings.Repeat("a", reasonMax), map[string]string{}},
		{"501 символ", strings.Repeat("я", reasonMax+1), strings.Repeat("я", reasonMax+1), map[string]string{"reason": msgTooLong}},
		{"NUL", "a\x00b", "a\x00b", map[string]string{"reason": msgInvalidText}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, f := validateReason(tc.raw)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.fields, fieldMap(f))
		})
	}
}

// soft-delete-transfers TS-016: version удаления документа — поле JSON-тела (DS-028, раньше
// query): целое ≥ 1; нет / null → required; строка, дробь, ноль, отрицательное → invalid.
func TestTS016_ParseVersion(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		want   int
		fields map[string]string
	}{
		{"1", 1, map[string]string{}},
		{" 42 ", 42, map[string]string{}},
		{"", 0, map[string]string{"version": msgRequired}},
		{"   ", 0, map[string]string{"version": msgRequired}},
		{"null", 0, map[string]string{"version": msgRequired}},
		{`"abc"`, 0, map[string]string{"version": msgInvalid}},
		{`"2"`, 0, map[string]string{"version": msgInvalid}},
		{"true", 0, map[string]string{"version": msgInvalid}},
		{"1.5", 0, map[string]string{"version": msgInvalid}},
		{"0", 0, map[string]string{"version": msgInvalid}},
		{"-3", 0, map[string]string{"version": msgInvalid}},
		{"99999999999999999999", 0, map[string]string{"version": msgInvalid}},
	} {
		t.Run("version="+tc.raw, func(t *testing.T) {
			got, f := parseVersion(json.RawMessage(tc.raw))
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.fields, fieldMap(f))
		})
	}
}

// soft-delete-transfers §6.1: тело удаления {reason, version} — version остаётся сырым JSON
// (не число → 400 invalid, а не «битое тело»), reason без значения — nil (required).
func TestTS016_DeleteRequestBody(t *testing.T) {
	var req TransferDeleteRequest
	require.NoError(t, json.Unmarshal([]byte(`{"reason":"Создан по ошибке","version":2}`), &req))
	require.NotNil(t, req.Reason)
	assert.Equal(t, "Создан по ошибке", *req.Reason)
	v, f := parseVersion(req.Version)
	assert.Equal(t, 2, v)
	assert.Empty(t, f)

	req = TransferDeleteRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{"reason":"  ","version":"x"}`), &req))
	_, f = parseVersion(req.Version)
	assert.Equal(t, map[string]string{"version": msgInvalid}, fieldMap(f))

	req = TransferDeleteRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{}`), &req))
	assert.Nil(t, req.Reason)
	_, f = parseVersion(req.Version)
	assert.Equal(t, map[string]string{"version": msgRequired}, fieldMap(f))
}

// soft-delete-transfers §6.2, §6.3: ошибки repo мягкого удаления → wire-коды 409 со своими
// сообщениями (DS-007, DS-024, DS-040).
func TestTS015_TS017_TS050_SoftDeleteErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err       error
		code, msg string
	}{
		{errTaskDeleted, "TASK_DELETED", "Заказ удалён — сначала восстановите его"},
		{errNotDeleted, "NOT_DELETED", "Запись уже восстановлена"},
		{errClientMismatch, "TRANSFER_CLIENT_MISMATCH", "Клиент заказа изменён — восстановить перемещение нельзя"},
		{&equipmentBusyError{equipmentID: "e", transferID: 7}, "EQUIPMENT_IN_TRANSFER", "Оборудование уже в перемещении #7"},
		{&notInTaskError{equipmentID: "e"}, "TRANSFER_EQUIPMENT_NOT_IN_TASK", "Оборудование убрано из заказа — восстановить перемещение нельзя"},
	} {
		var app *errorspkg.AppError
		require.True(t, errors.As(mapErr(tc.err), &app), tc.code)
		assert.Equal(t, tc.code, app.Code)
		assert.Equal(t, 409, app.HTTP)
		assert.Equal(t, tc.msg, app.Message)
	}
}

// Каждый недопустимый статус отката / удаления / отмены — 409 TRANSFER_STATUS_CONFLICT со
// своим сообщением (§5, §6.1, §6.2, §6.4).
func TestTS010_TS016_StatusConflictMessages(t *testing.T) {
	for _, tc := range []struct{ action, status, msg string }{
		{actionUnreceive, StatusRequested, "Перемещение ещё не проведено"},
		{actionUnreceive, StatusInTransit, "Перемещение ещё не получено"},
		{actionUnreceive, StatusCancelled, "Перемещение отменено"},
		{actionDelete, StatusInTransit, "Перемещение в пути — сначала отмените его"},
		{actionDelete, StatusReceived, "Перемещение получено — сначала отмените проведение, затем отмените"},
		{actionCancel, StatusReceived, "Перемещение получено — сначала отмените проведение"}, // TS-009
	} {
		e := mapErr(&statusError{action: tc.action, status: tc.status})
		var app *errorspkg.AppError
		require.True(t, errors.As(e, &app), "%s in %s", tc.action, tc.status)
		assert.Equal(t, "TRANSFER_STATUS_CONFLICT", app.Code)
		assert.Equal(t, 409, app.HTTP)
		assert.Equal(t, tc.msg, app.Message, "%s in %s", tc.action, tc.status)
	}
}

func TestTS012_UnreceiveBlockedError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    unreceiveBlockedError
		msg   string
		extra map[string]any
	}{
		{"более поздний полученный документ", unreceiveBlockedError{equipmentID: "eq-1", transferID: 14, later: true},
			"Сначала отмените проведение перемещения #14", map[string]any{"equipment_id": "eq-1", "transfer_id": int64(14)}},
		{"другой незавершённый документ", unreceiveBlockedError{equipmentID: "eq-2", transferID: 15},
			"Сначала отмените перемещение #15", map[string]any{"equipment_id": "eq-2", "transfer_id": int64(15)}},
		{"единица не на месте, мешающего документа нет", unreceiveBlockedError{equipmentID: "eq-3"},
			"Оборудование находится не там, куда его привезло это перемещение", map[string]any{"equipment_id": "eq-3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			var app *errorspkg.AppError
			require.True(t, errors.As(mapErr(&in), &app))
			assert.Equal(t, "TRANSFER_UNRECEIVE_BLOCKED", app.Code)
			assert.Equal(t, 409, app.HTTP)
			assert.Equal(t, tc.msg, app.Message)
			assert.Equal(t, tc.extra, app.Extra)
		})
	}
}

func TestTS010_ReceivesStockByDirection(t *testing.T) {
	assert.True(t, receivesStock(DirWarehouseToWarehouse))
	assert.True(t, receivesStock(DirSupplierToWarehouse))
	assert.False(t, receivesStock(DirWarehouseToClient), "товар накладной списан при отправке")
	assert.False(t, receivesStock(DirClientToWarehouse), "товарных строк нет")
}
