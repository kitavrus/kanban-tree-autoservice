package softdelete

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

// TS-001 (DS-020): причина удаления — 1–500 символов после обрезки крайних пробелов.
func TestValidateReason(t *testing.T) {
	cases := []struct {
		name    string
		in      *string
		want    string
		wantErr string
	}{
		{"нет поля reason", nil, "", "required"},
		{"пусто", ptr(""), "", "required"},
		{"одни пробелы", ptr(" \t\n  "), "", "required"},
		{"1 символ", ptr("x"), "x", ""},
		{"крайние пробелы обрезаны", ptr("  Создан по ошибке \n"), "Создан по ошибке", ""},
		{"500 символов", ptr(strings.Repeat("я", 500)), strings.Repeat("я", 500), ""},
		{"500 символов и пробелы по краям", ptr(" " + strings.Repeat("я", 500) + " "), strings.Repeat("я", 500), ""},
		{"501 символ", ptr(strings.Repeat("я", 501)), "", "too_long"},
		{"NUL", ptr("при\x00чина"), "", "invalid_text"},
		{"битый UTF-8", ptr("при\xffчина"), "", "invalid_text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, fields := ValidateReason(tc.in)
			if tc.wantErr == "" {
				require.Empty(t, fields)
				assert.Equal(t, tc.want, got)
				return
			}
			require.Len(t, fields, 1)
			assert.Equal(t, "reason", fields[0].Name)
			assert.Equal(t, tc.wantErr, fields[0].Message)
			assert.Empty(t, got, "при ошибке причина не возвращается")
		})
	}
}

// Реестр видов (DS-001): 9 видов, ключи счётчиков summary — snake_case (§API-1).
func TestKinds(t *testing.T) {
	require.Len(t, Kinds, 9)
	keys := map[string]bool{}
	for _, k := range Kinds {
		assert.True(t, IsKind(string(k)))
		assert.NotContains(t, k.CountKey(), "-")
		keys[k.CountKey()] = true
	}
	assert.Len(t, keys, 9, "ключи счётчиков уникальны")
	assert.Equal(t, "catalog_categories", KindCatalogCategories.CountKey())
	assert.Equal(t, "equipment_types", KindEquipmentTypes.CountKey())
	assert.False(t, IsKind("orders"))
	assert.False(t, IsKind(""))
}

func fieldsOf(t *testing.T, q ListQuery, tz string) map[string]string {
	t.Helper()
	_, _, fields := parseQuery(q, tz)
	m := map[string]string{}
	for _, f := range fields {
		m[f.Name] = f.Message
	}
	return m
}

// TS-009 (DS-011): параметры списка — обязательность и границы.
func TestParseQuery_Validation(t *testing.T) {
	cases := []struct {
		name string
		q    ListQuery
		want map[string]string
	}{
		{"без kind", ListQuery{}, map[string]string{"kind": "required"}},
		{"неизвестный kind", ListQuery{Kind: "orders"}, map[string]string{"kind": "invalid"}},
		{"q длиннее 100", ListQuery{Kind: "tasks", Q: strings.Repeat("я", 101)}, map[string]string{"q": "too_long"}},
		{"q ровно 100", ListQuery{Kind: "tasks", Q: " " + strings.Repeat("я", 100) + " "}, map[string]string{}},
		{"не дата «с»", ListQuery{Kind: "tasks", DeletedFrom: "2026-13-01"}, map[string]string{"deleted_from": "invalid"}},
		{"не дата «по»", ListQuery{Kind: "tasks", DeletedTo: "вчера"}, map[string]string{"deleted_to": "invalid"}},
		{"«по» раньше «с»", ListQuery{Kind: "tasks", DeletedFrom: "2026-10-08", DeletedTo: "2026-10-07"}, map[string]string{"deleted_to": "invalid"}},
		{"«по» равно «с»", ListQuery{Kind: "tasks", DeletedFrom: "2026-10-08", DeletedTo: "2026-10-08"}, map[string]string{}},
		{"deleted_by не uuid", ListQuery{Kind: "tasks", DeletedBy: "42"}, map[string]string{"deleted_by": "invalid"}},
		{"limit 0", ListQuery{Kind: "tasks", Limit: "0"}, map[string]string{"limit": "invalid"}},
		{"limit 201", ListQuery{Kind: "tasks", Limit: "201"}, map[string]string{"limit": "invalid"}},
		{"limit не число", ListQuery{Kind: "tasks", Limit: "many"}, map[string]string{"limit": "invalid"}},
		{"offset -1", ListQuery{Kind: "tasks", Offset: "-1"}, map[string]string{"offset": "invalid"}},
		{"все ошибки разом", ListQuery{Kind: "x", Limit: "x", Offset: "x", DeletedBy: "x"},
			map[string]string{"kind": "invalid", "limit": "invalid", "offset": "invalid", "deleted_by": "invalid"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, fieldsOf(t, tc.q, "UTC"))
		})
	}
}

// Умолчания и разбор значений: limit 50, offset 0, поиск экранируется для ILIKE, номер
// заказа из строки поиска («128» и «#128»).
func TestParseQuery_Values(t *testing.T) {
	kind, f, fields := parseQuery(ListQuery{Kind: "tasks"}, "UTC")
	require.Empty(t, fields)
	assert.Equal(t, KindTasks, kind)
	assert.Equal(t, 50, f.Limit)
	assert.Equal(t, 0, f.Offset)
	assert.Empty(t, f.Q)
	assert.Nil(t, f.QNumber)
	assert.Nil(t, f.DeletedFrom)
	assert.Nil(t, f.DeletedTo)

	_, f, fields = parseQuery(ListQuery{Kind: "tasks", Q: " 50%_скидка ", Limit: "200", Offset: "100",
		DeletedBy: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"}, "UTC")
	require.Empty(t, fields)
	assert.Equal(t, `50\%\_скидка`, f.Q)
	assert.Nil(t, f.QNumber)
	assert.Equal(t, 200, f.Limit)
	assert.Equal(t, 100, f.Offset)
	assert.Equal(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", f.DeletedBy)

	for _, q := range []string{"128", "#128"} {
		_, f, fields = parseQuery(ListQuery{Kind: "tasks", Q: q}, "UTC")
		require.Empty(t, fields)
		require.NotNil(t, f.QNumber, q)
		assert.Equal(t, int64(128), *f.QNumber)
	}
}

// TS-010 (DS-025): день периода — с 00:00 до 24:00 в поясе пользователя. Момент 23:30 UTC
// 6 октября — это 6 октября для пользователя в UTC и 7 октября для пользователя в +03:00.
func TestParseQuery_PeriodInUserTimezone(t *testing.T) {
	at := time.Date(2026, 10, 6, 23, 30, 0, 0, time.UTC)
	in := func(tz, day string) bool {
		_, f, fields := parseQuery(ListQuery{Kind: "tasks", DeletedFrom: day, DeletedTo: day}, tz)
		require.Empty(t, fields)
		require.NotNil(t, f.DeletedFrom)
		require.NotNil(t, f.DeletedTo)
		return !at.Before(*f.DeletedFrom) && at.Before(*f.DeletedTo)
	}
	assert.True(t, in("UTC", "2026-10-06"), "UTC: этот день")
	assert.False(t, in("UTC", "2026-10-07"))
	assert.True(t, in("Europe/Moscow", "2026-10-07"), "+03:00: следующий день")
	assert.False(t, in("Europe/Moscow", "2026-10-06"))
}
