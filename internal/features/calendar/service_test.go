package calendar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func strp(s string) *string { return &s }

func occupied(resp *CalendarResponse) map[string]int {
	out := map[string]int{}
	for _, d := range resp.Days {
		out[d.Date] = d.Occupied
	}
	return out
}

func TestBuild_Spans(t *testing.T) {
	today := day("2026-10-15")
	rows := []orderRow{
		// выдан: 10..12
		{ID: 1, StartDate: "2026-10-10", FinishedOn: strp("2026-10-12")},
		// в работе со сроком в будущем: 14..16
		{ID: 2, StartDate: "2026-10-14", DueDate: strp("2026-10-16")},
		// в работе без срока: с 13 бессрочно
		{ID: 3, StartDate: "2026-10-13"},
		// в работе, срок прошёл: с 11 бессрочно
		{ID: 4, StartDate: "2026-10-11", DueDate: strp("2026-10-12")},
	}
	resp := build(rows, 5, today, day("2026-10-09"), day("2026-10-18"))

	occ := occupied(resp)
	assert.Equal(t, 0, occ["2026-10-09"])
	assert.Equal(t, 1, occ["2026-10-10"])
	assert.Equal(t, 2, occ["2026-10-11"]) // 1, 4
	assert.Equal(t, 2, occ["2026-10-12"]) // 1, 4
	assert.Equal(t, 2, occ["2026-10-13"]) // 3, 4
	assert.Equal(t, 3, occ["2026-10-14"]) // 2, 3, 4
	assert.Equal(t, 3, occ["2026-10-16"])
	assert.Equal(t, 2, occ["2026-10-17"]) // 3, 4 бессрочные
	assert.Equal(t, 2, occ["2026-10-18"])
	assert.Len(t, resp.Days, 10)
	assert.Equal(t, "2026-10-15", resp.Today)

	byID := map[int64]Order{}
	for _, o := range resp.Orders {
		byID[o.ID] = o
	}
	assert.True(t, byID[1].IsFinished)
	assert.Equal(t, "2026-10-12", *byID[1].EndDate)
	assert.Equal(t, "2026-10-16", *byID[2].EndDate)
	assert.True(t, byID[3].OpenEnded)
	assert.Nil(t, byID[3].EndDate)
	assert.True(t, byID[4].OpenEnded)
}

func TestBuild_FreeAndOverload(t *testing.T) {
	var rows []orderRow
	for i := int64(1); i <= 7; i++ {
		rows = append(rows, orderRow{ID: i, StartDate: "2026-10-01"})
	}
	resp := build(rows, 5, day("2026-10-01"), day("2026-10-01"), day("2026-10-01"))
	require.Len(t, resp.Days, 1)
	assert.Equal(t, 7, resp.Days[0].Occupied)
	assert.Equal(t, 0, resp.Days[0].Free)
	assert.Len(t, resp.Days[0].TaskIDs, 7)

	resp = build(rows[:3], 5, day("2026-10-01"), day("2026-10-01"), day("2026-10-01"))
	assert.Equal(t, 2, resp.Days[0].Free)
}

func TestBuild_SkipsOrdersEndedBeforeWindow(t *testing.T) {
	rows := []orderRow{{ID: 1, StartDate: "2026-09-01", DueDate: strp("2026-10-20")}}
	// окно в будущем, после срока: заказ не занимает мест и не попадает в orders
	resp := build(rows, 5, day("2026-10-15"), day("2026-11-01"), day("2026-11-03"))
	assert.Empty(t, resp.Orders)
	assert.Equal(t, 0, resp.Days[0].Occupied)
}

func TestLevel(t *testing.T) {
	assert.Equal(t, LevelFree, level(3, 5))
	assert.Equal(t, LevelBusy, level(4, 5))
	assert.Equal(t, LevelFull, level(5, 5))
	assert.Equal(t, LevelOver, level(6, 5))
}

func fieldOf(t *testing.T, err error) errorspkg.Field {
	t.Helper()
	app, ok := err.(*errorspkg.AppError)
	require.True(t, ok, "%v", err)
	require.Len(t, app.Fields, 1)
	return app.Fields[0]
}

func TestValidateSettings(t *testing.T) {
	_, err := ValidateSettings(SettingsRequest{})
	assert.Equal(t, errorspkg.Field{Name: "max_cars", Message: msgRequired}, fieldOf(t, err))
	for _, n := range []int{4, 13, 0, -1} {
		v := n
		_, err := ValidateSettings(SettingsRequest{MaxCars: &v})
		assert.Equal(t, msgOutOfRange, fieldOf(t, err).Message, n)
	}
	for _, n := range []int{5, 8, 12} {
		v := n
		s, err := ValidateSettings(SettingsRequest{MaxCars: &v})
		require.NoError(t, err)
		assert.Equal(t, n, s.MaxCars)
	}
}

func TestParseRange(t *testing.T) {
	today := day("2026-02-10")
	f, to, err := ParseRange("", "", today)
	require.NoError(t, err)
	assert.Equal(t, "2026-02-01", f.Format(time.DateOnly))
	assert.Equal(t, "2026-02-28", to.Format(time.DateOnly))

	_, _, err = ParseRange("2026-02-30", "2026-03-01", today)
	assert.Equal(t, "from", fieldOf(t, err).Name)
	_, _, err = ParseRange("2026-03-02", "2026-03-01", today)
	assert.Equal(t, msgBeforeFrom, fieldOf(t, err).Message)
	_, _, err = ParseRange("2026-01-01", "2026-03-05", today)
	assert.Equal(t, msgRangeTooWide, fieldOf(t, err).Message)
	_, _, err = ParseRange("2026-01-01", "2026-03-03", today) // ровно 62 дня
	assert.NoError(t, err)
}
