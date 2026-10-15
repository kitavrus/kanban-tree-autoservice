package calendar

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGantt_Bars(t *testing.T) {
	today := day("2026-10-15")
	selected := day("2026-10-15")
	from, to := GanttWindow(selected) // 08.10 .. 04.11
	require.Equal(t, day("2026-10-08"), from)
	require.Equal(t, day("2026-11-04"), to)

	rows := []orderRow{
		// со сроком: 14..18 — 5 дней, столбцы 7..11
		{ID: 1, StartDate: "2026-10-14", DueDate: strp("2026-10-18")},
		// принят до шкалы, выдан 15-го: обрезан слева
		{ID: 2, StartDate: "2026-10-01", FinishedOn: strp("2026-10-15")},
		// без срока: бессрочно до правого края
		{ID: 3, StartDate: "2026-10-13"},
		// срок 10-го прошёл — бессрочно, просрочен, отметка срока в шкале
		{ID: 4, StartDate: "2026-10-09", DueDate: strp("2026-10-10")},
		// 16-го не стоит в выбранный день — в диаграмму не попадает
		{ID: 5, StartDate: "2026-10-16", DueDate: strp("2026-10-17")},
	}
	win := build(rows, 5, today, from, to)
	var ids []int64
	for _, d := range win.Days {
		if d.Date == "2026-10-15" {
			ids = d.TaskIDs
		}
	}
	g := buildGantt(selected, today, win, ids)

	require.Len(t, g.Days, GanttDays)
	assert.Equal(t, GanttBefore+1, g.SelectedCol)
	assert.Equal(t, GanttBefore+1, g.TodayCol)
	assert.Equal(t, "Пт", g.Days[1].Weekday) // 09.10.2026 — пятница
	assert.True(t, g.Days[2].IsWeekend)

	byID := map[int64]GanttRow{}
	var order []int64
	for _, r := range g.Rows {
		byID[r.ID] = r
		order = append(order, r.ID)
	}
	require.Len(t, byID, 4)
	assert.Equal(t, []int64{2, 4, 3, 1}, order, "по началу полосы")

	r1 := byID[1]
	assert.Equal(t, 7, r1.Col)
	assert.Equal(t, 5, r1.Span)
	assert.Equal(t, 5, r1.Days)
	assert.Equal(t, 11, r1.DueCol)
	assert.False(t, r1.CutLeft || r1.CutRight || r1.OpenEnded || r1.Overdue)
	assert.Equal(t, "14.10 → срок 18.10", r1.Period)

	r2 := byID[2]
	assert.True(t, r2.CutLeft)
	assert.True(t, r2.IsFinished)
	assert.Equal(t, 1, r2.Col)
	assert.Equal(t, 8, r2.Span)
	assert.Equal(t, 15, r2.Days, "всего дней на месте, а не только видимых")
	assert.Equal(t, "выдан", r2.Status)

	r3 := byID[3]
	assert.True(t, r3.OpenEnded)
	assert.True(t, r3.CutRight)
	assert.Equal(t, 6, r3.Col)
	assert.Equal(t, GanttDays-5, r3.Span, "до правого края шкалы")
	assert.Equal(t, 3, r3.Days, "13..15 — пока стоит")
	assert.Equal(t, "срок не указан", r3.Status)

	r4 := byID[4]
	assert.True(t, r4.Overdue)
	assert.Equal(t, 3, r4.DueCol)
	assert.Equal(t, "срок 10.10 просрочен", r4.Status)
}

func TestBuildGantt_Empty(t *testing.T) {
	selected := day("2026-10-20")
	from, to := GanttWindow(selected)
	g := buildGantt(selected, day("2026-10-15"), build(nil, 6, day("2026-10-15"), from, to), nil)
	assert.Empty(t, g.Rows)
	assert.Len(t, g.Days, GanttDays)
	assert.Equal(t, 6, g.Capacity)
	assert.Equal(t, 3, g.TodayCol)
	assert.Equal(t, "13 октября — 9 ноября", g.RangeTitle)
}
