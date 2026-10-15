package calendar

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны экрана «Календарь» относительно корня модуля.
const Root = "web/ssr/calendar/templates"

// SSR — экран «Календарь»: сетка месяца с загрузкой по дням и диаграмма Ганта выбранного дня
// (машины этого дня и их сроки — gantt.go).
// Сетка рисуется на сервере; js/calendar.js — сохранение вместимости (admin) и новый заказ
// (карточка заказа js/task-card.js, срок по умолчанию — выбранный день).
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

// DayView — выбранный день (заголовок диаграммы Ганта и кнопка «+ Заказ со сроком»).
type DayView struct {
	Date   string
	Title  string
	IsPast bool // прошедший день — заказ «со сроком на этот день» не предлагается
}

// PageVM — view-model страницы.
type PageVM struct {
	Shell      ssr.Shell
	ViewerID   string // для карточки заказа (js/task-card.js)
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
	Gantt      *Gantt // диаграмма Ганта выбранного дня (nil — день не выбран)
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
		Shell: sh, ViewerID: middleware.CurrentPrincipal(c).ID, IsAdmin: sh.IsAdmin,
		MonthTitle: fmt.Sprintf("%s %d", monthNames[month.Month()-1], month.Year()),
		WeekDays:   []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"},
		PrevURL:    pageURL(month.AddDate(0, -1, 0), ""),
		NextURL:    pageURL(month.AddDate(0, 1, 0), ""),
		TodayURL:   "/calendar",
		Capacity:   cal.Capacity, MinCars: MinCars, MaxCars: MaxCars,
	}
	var week []Cell
	var selectedIDs []int64
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
			vm.Selected = dayView(dt, cell)
			selectedIDs = d.TaskIDs
		}
		week = append(week, cell)
		if len(week) == 7 {
			vm.Weeks = append(vm.Weeks, week)
			week = nil
		}
	}
	if vm.Selected != nil {
		sel, _ := time.Parse(time.DateOnly, vm.Selected.Date)
		gFrom, gTo := GanttWindow(sel)
		win, err := h.svc.Calendar(c.Context(), tz, gFrom, gTo)
		if err != nil {
			return err
		}
		vm.Gantt = buildGantt(sel, today, win, selectedIDs)
	}
	return h.r.Page(c, "calendar", vm)
}

func dayView(dt time.Time, cell Cell) *DayView {
	return &DayView{
		Date:   cell.Date,
		Title:  fmt.Sprintf("%s, %s", humanDate(dt), weekdayNames[dt.Weekday()]),
		IsPast: cell.IsPast,
	}
}
