package board

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// parseFilter — ParseTaskFilter(c fiber.Ctx, a Actor) требует реальный fiber.Ctx; здесь он
// нужен только как носитель query-параметров (не тест HTTP-роутинга/middleware — §TD-8
// про app.Test относится к HTTP-уровневым integration-тестам handler/router, не к юнит-
// тесту чистой функции разбора query).
func parseFilter(t *testing.T, rawQuery string, actor Actor) (TaskFilter, error) {
	t.Helper()
	app := fiber.New()
	var got TaskFilter
	var gotErr error
	app.Get("/x", func(c fiber.Ctx) error {
		got, gotErr = ParseTaskFilter(c, actor)
		return c.SendStatus(fiber.StatusOK)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/x?"+rawQuery, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return got, gotErr
}

var admin = Actor{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", IsAdmin: true}
var executor = Actor{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", IsAdmin: false}

// TestParseTaskFilter_HashNumber — UT-001.
func TestParseTaskFilter_HashNumber(t *testing.T) {
	f, err := parseFilter(t, "q=%2342", admin)
	require.NoError(t, err)
	require.NotNil(t, f.TaskNo)
	require.Equal(t, int64(42), *f.TaskNo)
}

// TestParseTaskFilter_PlainNumber — UT-001.
func TestParseTaskFilter_PlainNumber(t *testing.T) {
	f, err := parseFilter(t, "q=42", admin)
	require.NoError(t, err)
	require.NotNil(t, f.TaskNo)
	require.Equal(t, int64(42), *f.TaskNo)
}

// TestParseTaskFilter_InvalidNumber — UT-001: "42a" не число заказа, но валидный текстовый
// поиск (не ошибка).
func TestParseTaskFilter_InvalidNumber(t *testing.T) {
	f, err := parseFilter(t, "q=42a", admin)
	require.NoError(t, err)
	require.Nil(t, f.TaskNo)
	require.Equal(t, "42a", f.Q)
}

// TestParseTaskFilter_DefaultsByRole — UT-001/AC-014: executor без `assignee` → «свои»;
// admin — все.
func TestParseTaskFilter_DefaultsByRole(t *testing.T) {
	f, err := parseFilter(t, "", executor)
	require.NoError(t, err)
	require.NotNil(t, f.AssigneeID)
	require.Equal(t, executor.ID, *f.AssigneeID)
	require.Equal(t, tasksDefaultLimit, f.Limit)
	require.Equal(t, 0, f.Offset)

	f, err = parseFilter(t, "", admin)
	require.NoError(t, err)
	require.Nil(t, f.AssigneeID)
}

// TestParseTaskFilter_EnumValidation — UT-001/LV-037: enum'ы, uuid, лимиты.
func TestParseTaskFilter_EnumValidation(t *testing.T) {
	cases := []string{"paid=maybe", "due=x", "assignee=bad", "limit=-1", "limit=1001", "limit=abc", "offset=-1", "client_id=not-a-uuid"}
	for _, q := range cases {
		_, err := parseFilter(t, q, admin)
		require.Error(t, err, "q=%s", q)
	}

	f, err := parseFilter(t, "paid=paid&due=soon&assignee=all&limit=10&offset=5", admin)
	require.NoError(t, err)
	require.NotNil(t, f.Paid)
	require.True(t, *f.Paid)
	require.Equal(t, DueFilterSoon, f.Due)
	require.Nil(t, f.AssigneeID)
	require.Equal(t, 10, f.Limit)
	require.Equal(t, 5, f.Offset)

	f, err = parseFilter(t, "assignee=me", executor)
	require.NoError(t, err)
	require.NotNil(t, f.AssigneeID)
	require.Equal(t, executor.ID, *f.AssigneeID)

	f, err = parseFilter(t, "assignee="+validUUID, admin)
	require.NoError(t, err)
	require.NotNil(t, f.AssigneeID)
	require.Equal(t, validUUID, *f.AssigneeID)
}

// TestParseTaskFilter_ColumnAndPerColumn — TS-001 (pagination DS-002): column_id невалидный
// → VALIDATION по полю column_id; per_column 0/1001/abc → по полю per_column; валидные
// значения попадают в фильтр, отсутствующие — nil (без окна / без фильтра по колонке).
func TestParseTaskFilter_ColumnAndPerColumn(t *testing.T) {
	cases := map[string]string{
		"column_id=not-a-uuid": "column_id",
		"per_column=0":         "per_column",
		"per_column=1001":      "per_column",
		"per_column=abc":       "per_column",
		"per_column=-3":        "per_column",
	}
	for q, field := range cases {
		_, err := parseFilter(t, q, admin)
		require.Error(t, err, "q=%s", q)
		var ae *errorspkg.AppError
		require.True(t, errors.As(err, &ae), "q=%s", q)
		require.Equal(t, "VALIDATION", ae.Code, "q=%s", q)
		names := make([]string, 0, len(ae.Fields))
		for _, fl := range ae.Fields {
			names = append(names, fl.Name)
		}
		require.Equal(t, []string{field}, names, "q=%s", q)
	}

	f, err := parseFilter(t, "", admin)
	require.NoError(t, err)
	require.Nil(t, f.ColumnID)
	require.Nil(t, f.PerColumn)

	f, err = parseFilter(t, "column_id="+validUUID+"&per_column=50&limit=1000", admin)
	require.NoError(t, err)
	require.NotNil(t, f.ColumnID)
	require.Equal(t, validUUID, *f.ColumnID)
	require.NotNil(t, f.PerColumn)
	require.Equal(t, 50, *f.PerColumn)
	require.Equal(t, 1000, f.Limit)

	for _, n := range []string{"1", "1000"} {
		f, err = parseFilter(t, "per_column="+n, admin)
		require.NoError(t, err, "per_column=%s", n)
		require.NotNil(t, f.PerColumn)
	}
}
