package calendar

import (
	"fmt"
	"sort"
	"time"
)

// Диаграмма Ганта выбранного дня (экран «Календарь»): машины, стоящие в ремонте в этот день,
// полосами на шкале дней — откуда пришла, до какого дня занимает место, сколько дней всего.
// Шкала — GanttDays дней, выбранный день — (GanttBefore+1)-й столбец; полоса, выходящая за
// шкалу, обрезается (CutLeft/CutRight), бессрочная (OpenEnded) тянется до правого края.
const (
	GanttBefore = 7
	GanttDays   = 28
)

var weekdayShort = [...]string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"}

// GanttDay — столбец шкалы с загрузкой дня.
type GanttDay struct {
	Col        int
	Date       string
	Day        int
	Weekday    string
	IsWeekend  bool
	IsToday    bool
	IsSelected bool
	IsPast     bool
	Occupied   int
	Level      string
}

// GanttRow — машина (заказ) и её полоса.
type GanttRow struct {
	ID          int64
	Title       string
	ClientName  string
	ColumnName  string
	ColumnColor string
	Col         int  // первый столбец полосы (1..GanttDays)
	Span        int  // число столбцов полосы
	CutLeft     bool // работы начаты раньше начала шкалы
	CutRight    bool // занимает место и после конца шкалы (в т.ч. бессрочно)
	OpenEnded   bool
	IsFinished  bool
	Overdue     bool // плановое окончание прошло, машина не выдана
	DueCol      int  // столбец планового окончания (0 — оно вне шкалы)
	Days        int  // дней на месте: всего (известен конец) или пока (бессрочно)
	Period      string
	Status      string
}

// Gantt — диаграмма выбранного дня.
type Gantt struct {
	From, To    string
	RangeTitle  string
	Capacity    int
	Days        []GanttDay
	Rows        []GanttRow
	TodayCol    int // 0 — сегодня вне шкалы
	SelectedCol int
}

// GanttWindow — шкала диаграммы для выбранного дня.
func GanttWindow(selected time.Time) (time.Time, time.Time) {
	from := selected.AddDate(0, 0, -GanttBefore)
	return from, from.AddDate(0, 0, GanttDays-1)
}

func shortDate(d time.Time) string { return d.Format("02.01") }

func parseDay(s string) time.Time { t, _ := time.Parse(time.DateOnly, s); return t }

// buildGantt — чистый расчёт: win — ответ Calendar на окно GanttWindow(selected), ids — заказы
// выбранного дня (Day.TaskIDs).
func buildGantt(selected, today time.Time, win *CalendarResponse, ids []int64) *Gantt {
	from, to := GanttWindow(selected)
	g := &Gantt{
		From: from.Format(time.DateOnly), To: to.Format(time.DateOnly),
		RangeTitle: humanDate(from) + " — " + humanDate(to),
		Capacity:   win.Capacity,
	}
	for i, d := range win.Days {
		dt := parseDay(d.Date)
		gd := GanttDay{
			Col: i + 1, Date: d.Date, Day: dt.Day(), Weekday: weekdayShort[dt.Weekday()],
			IsWeekend: dt.Weekday() == time.Saturday || dt.Weekday() == time.Sunday,
			IsToday:   dt.Equal(today), IsSelected: dt.Equal(selected), IsPast: dt.Before(today),
			Occupied: d.Occupied, Level: level(d.Occupied, win.Capacity),
		}
		if gd.IsToday {
			g.TodayCol = gd.Col
		}
		if gd.IsSelected {
			g.SelectedCol = gd.Col
		}
		g.Days = append(g.Days, gd)
	}

	orders := make(map[int64]Order, len(win.Orders))
	for _, o := range win.Orders {
		orders[o.ID] = o
	}
	col := func(d time.Time) int { return daysBetween(from, d) + 1 }
	for _, id := range ids {
		o, ok := orders[id]
		if !ok {
			continue
		}
		start := parseDay(o.StartDate)
		end := to
		if o.EndDate != nil {
			end = parseDay(*o.EndDate)
		}
		r := GanttRow{
			ID: o.ID, Title: o.Title, ClientName: o.ClientName, ColumnName: o.ColumnName, ColumnColor: o.ColumnColor,
			OpenEnded: o.OpenEnded, IsFinished: o.IsFinished,
			CutLeft: start.Before(from), CutRight: o.OpenEnded || end.After(to),
		}
		barStart, barEnd := start, end
		if barStart.Before(from) {
			barStart = from
		}
		if barEnd.After(to) {
			barEnd = to
		}
		r.Col, r.Span = col(barStart), daysBetween(barStart, barEnd)+1

		var due time.Time
		if o.DueDate != nil {
			due = parseDay(*o.DueDate)
			if !due.Before(from) && !due.After(to) {
				r.DueCol = col(due)
			}
			r.Overdue = !o.IsFinished && due.Before(today)
		}

		switch {
		case o.IsFinished && o.EndDate != nil:
			r.Days = daysBetween(start, end) + 1
			r.Period = fmt.Sprintf("%s → выдан %s", shortDate(start), shortDate(end))
			r.Status = "выдан"
		case o.EndDate != nil:
			r.Days = daysBetween(start, end) + 1
			r.Period = fmt.Sprintf("%s → срок %s", shortDate(start), shortDate(end))
			r.Status = "до срока"
		default: // бессрочно — плановое окончание (всегда задано) прошло, машина не выдана
			r.Days = daysBetween(start, today) + 1
			r.Period = "с " + shortDate(start)
			r.Status = "срок " + shortDate(due) + " просрочен"
		}
		if r.Days < 1 {
			r.Days = 1
		}
		g.Rows = append(g.Rows, r)
	}
	sort.SliceStable(g.Rows, func(i, j int) bool {
		if g.Rows[i].Col != g.Rows[j].Col {
			return g.Rows[i].Col < g.Rows[j].Col
		}
		return g.Rows[i].ID < g.Rows[j].ID
	})
	return g
}
