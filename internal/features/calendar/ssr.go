package calendar

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны экрана «Календарь» относительно корня модуля.
const Root = "web/ssr/calendar/templates"

// SSR — экран «Календарь»: сетка месяца с загрузкой по дням и список машин выбранного дня.
// Сетка рисуется на сервере; js/calendar.js — только сохранение вместимости (admin).
type SSR struct {
	svc   *Service
	r     *ssr.Renderer
	shell ssr.ShellBuilder
}

func NewSSR(svc *Service, r *ssr.Renderer, shell ssr.ShellBuilder) *SSR {
	return &SSR{svc: svc, r: r, shell: shell}
}

var (
	monthNames    = [...]string{"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}
	monthGenitive = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	weekdayNames  = [...]string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"}
)

// Уровни загрузки дня (CSS-класс ячейки).
const (
	LevelFree = "free" // свободно 2+ места
	LevelBusy = "busy" // осталось 1 место
	LevelFull = "full" // мест нет
	LevelOver = "over" // машин больше вместимости
)

// Cell — ячейка сетки месяца.
type Cell struct {
	Date       string
	Day        int
	InMonth    bool
	IsToday    bool
	IsPast     bool
	IsSelected bool
	Occupied   int
	Free       int
	Over       int
	Level      string
	URL        string
}

// DayOrder — машина выбранного дня.
type DayOrder struct {
	ID          int64
	Title       string
	ClientName  string
	ColumnName  string
	ColumnColor string
	Period      string
	OpenEnded   bool
	IsFinished  bool
}

// DayView — выбранный день.
type DayView struct {
	Date     string
	Title    string
	Occupied int
	Free     int
	Over     int
	Level    string
	Orders   []DayOrder
}

// PageVM — view-model страницы.
type PageVM struct {
	Shell      ssr.Shell
	IsAdmin    bool
	MonthTitle string
	WeekDays   []string
	PrevURL    string
	NextURL    string
	TodayURL   string
	Weeks      [][]Cell
	Capacity   int
	MinCars    int
	MaxCars    int
	Selected   *DayView
	Today      *Cell
}

func level(occupied, capacity int) string {
	switch {
	case occupied > capacity:
		return LevelOver
	case occupied == capacity:
		return LevelFull
	case capacity-occupied == 1:
		return LevelBusy
	default:
		return LevelFree
	}
}

func pageURL(month time.Time, day string) string {
	u := "/calendar?month=" + month.Format("2006-01")
	if day != "" {
		u += "&day=" + day
	}
	return u
}

func humanDate(d time.Time) string {
	return fmt.Sprintf("%d %s", d.Day(), monthGenitive[d.Month()-1])
}

// page — GET /calendar?month=YYYY-MM&day=YYYY-MM-DD.
func (h *SSR) page(c fiber.Ctx) error {
	sh := h.shell.Build(c, "Календарь")
	tz := ssr.ViewerTimezone(c)
	today := h.svc.Today(tz)

	month := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	if m, err := time.Parse("2006-01", c.Query("month")); err == nil {
		month = m
	}
	// Сетка — с понедельника до воскресенья, покрывая весь месяц (≤ 42 дня).
	gridFrom := month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7))
	last := month.AddDate(0, 1, -1)
	gridTo := last.AddDate(0, 0, (7-int(last.Weekday()))%7)

	cal, err := h.svc.Calendar(c.Context(), tz, gridFrom, gridTo)
	if err != nil {
		return err
	}

	selected := c.Query("day")
	if _, err := time.Parse(time.DateOnly, selected); err != nil {
		selected = ""
	}
	if selected == "" {
		if today.Year() == month.Year() && today.Month() == month.Month() {
			selected = today.Format(time.DateOnly)
		} else {
			selected = month.Format(time.DateOnly)
		}
	}

	vm := PageVM{
		Shell: sh, IsAdmin: sh.IsAdmin,
		MonthTitle: fmt.Sprintf("%s %d", monthNames[month.Month()-1], month.Year()),
		WeekDays:   []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"},
		PrevURL:    pageURL(month.AddDate(0, -1, 0), ""),
		NextURL:    pageURL(month.AddDate(0, 1, 0), ""),
		TodayURL:   "/calendar",
		Capacity:   cal.Capacity, MinCars: MinCars, MaxCars: MaxCars,
	}
	orders := make(map[int64]Order, len(cal.Orders))
	for _, o := range cal.Orders {
		orders[o.ID] = o
	}
	var week []Cell
	for _, d := range cal.Days {
		dt, _ := time.Parse(time.DateOnly, d.Date)
		cell := Cell{
			Date: d.Date, Day: dt.Day(), InMonth: dt.Month() == month.Month(),
			IsToday: d.Date == cal.Today, IsPast: dt.Before(today), IsSelected: d.Date == selected,
			Occupied: d.Occupied, Free: d.Free, Level: level(d.Occupied, cal.Capacity),
			URL: pageURL(month, d.Date),
		}
		if d.Occupied > cal.Capacity {
			cell.Over = d.Occupied - cal.Capacity
		}
		if cell.IsToday {
			t := cell
			vm.Today = &t
		}
		if cell.IsSelected {
			vm.Selected = dayView(dt, cell, d.TaskIDs, orders)
		}
		week = append(week, cell)
		if len(week) == 7 {
			vm.Weeks = append(vm.Weeks, week)
			week = nil
		}
	}
	return h.r.Page(c, "calendar", vm)
}

func dayView(dt time.Time, cell Cell, ids []int64, orders map[int64]Order) *DayView {
	v := &DayView{
		Date:     cell.Date,
		Title:    fmt.Sprintf("%s, %s", humanDate(dt), weekdayNames[dt.Weekday()]),
		Occupied: cell.Occupied, Free: cell.Free, Over: cell.Over, Level: cell.Level,
	}
	for _, id := range ids {
		o := orders[id]
		start, _ := time.Parse(time.DateOnly, o.StartDate)
		period := "с " + humanDate(start)
		switch {
		case o.IsFinished && o.EndDate != nil:
			end, _ := time.Parse(time.DateOnly, *o.EndDate)
			period += " — выдан " + humanDate(end)
		case o.EndDate != nil:
			end, _ := time.Parse(time.DateOnly, *o.EndDate)
			period += " по " + humanDate(end)
		case o.DueDate != nil:
			due, _ := time.Parse(time.DateOnly, *o.DueDate)
			period += " · срок " + humanDate(due) + " просрочен"
		default:
			period += " · срок не указан"
		}
		v.Orders = append(v.Orders, DayOrder{
			ID: o.ID, Title: o.Title, ClientName: o.ClientName, ColumnName: o.ColumnName,
			ColumnColor: o.ColumnColor, Period: period, OpenEnded: o.OpenEnded, IsFinished: o.IsFinished,
		})
	}
	return v
}
