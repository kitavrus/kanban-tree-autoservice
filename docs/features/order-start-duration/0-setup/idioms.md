# Идиомы проекта — слепок с фичи `calendar`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/calendar`

## Образцы слоёв (по одному файлу на слой; ещё 9 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/calendar/calendar_http_live_test.go`

```go
//go:build integration

package calendar_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// CalendarLiveSuite (§TD-8): календарь загрузки — настройка вместимости, расчёт занятых мест
// по дням, триггер finished_at, SSR-экран. Реальный бинарь + TCP + реальный логин; фикстуры —
// SQL. БД общая с другими заказами (бессрочные заказы «сегодня» попадают в любое будущее
// окно), поэтому проверки — по своим task_id, а окно — в 2031 году.
type CalendarLiveSuite struct {
	testutil.LiveSuite
	admin, executor *testutil.Browser
	adminID         string
	clientID        string
	workColumnID    string
	finalColumnID   string
}

func TestCalendarLiveSuite(t *testing.T)

func (s *CalendarLiveSuite) SetupSuite()

// ---- wire-типы (1:1 с internal/features/calendar/dto.go, §TD-6.2) ----

type settingsDTO struct {
	MaxCars int `json:"max_cars"`
}

type dayDTO struct {
	Date     string  `json:"date"`
	Occupied int     `json:"occupied"`
	Free     int     `json:"free"`
	TaskIDs  []int64 `json:"task_ids"`
}

type orderDTO struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	ClientName string  `json:"client_name"`
	ColumnName string  `json:"column_name"`
	StartDate  string  `json:"start_date"`
	DueDate    *string `json:"due_date"`
	EndDate    *string `json:"end_date"`
	OpenEnded  bool    `json:"open_ended"`
	IsFinished bool    `json:"is_finished"`
}

type calendarDTO struct {
	Capacity int        `json:"capacity"`
	Today    string     `json:"today"`
	From     string     `json:"from"`
	To       string     `json:"to"`
	Days     []dayDTO   `json:"days"`
	Orders   []orderDTO `json:"orders"`
}

type errDTO struct {
	Code    string `json:"code"`
	Details struct {
		Fields []struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"details"`
}

// insertTask — заказ с явным моментом приёмки (Asia/Almaty — пояс seed-пользователя).
func (s *CalendarLiveSuite) insertTask(title, columnID, createdAt string, due, finishedAt *string) int64

func strp(v string) *string { return &v }

func (s *CalendarLiveSuite) getCalendar(b *testutil.Browser, from, to string) calendarDTO

func contains(ids []int64, id int64) bool

func (s *CalendarLiveSuite) TestSettings_AdminSetsCapacity()

func (s *CalendarLiveSuite) TestSettings_Validation()

func (s *CalendarLiveSuite) TestSettings_ExecutorForbidden()

func (s *CalendarLiveSuite) TestCalendar_RangeValidation()

func (s *CalendarLiveSuite) TestCalendar_RequiresAuth()

func (s *CalendarLiveSuite) TestCalendar_OccupancyByDay()

func (s *CalendarLiveSuite) TestFinishedAtTrigger()

func (s *CalendarLiveSuite) TestSSR_Page()
```

### `internal/features/calendar/gantt.go`

```go
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
	CutLeft     bool // принята раньше начала шкалы
	CutRight    bool // занимает место и после конца шкалы (в т.ч. бессрочно)
	OpenEnded   bool
	IsFinished  bool
	Overdue     bool // срок прошёл, машина не выдана
	DueCol      int  // столбец срока (0 — срока нет или он вне шкалы)
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
func GanttWindow(selected time.Time) (time.Time, time.Time)

func shortDate(d time.Time) string { return d.Format("02.01") }

func parseDay(s string) time.Time { t, _ := time.Parse(time.DateOnly, s); return t }

// buildGantt — чистый расчёт: win — ответ Calendar на окно GanttWindow(selected), ids — заказы
// выбранного дня (Day.TaskIDs).
func buildGantt(selected, today time.Time, win *CalendarResponse, ids []int64) *Gantt
```

---

Исходники образца: 39,815 символов · слепок: 4,870 · сжатие ×8.2
