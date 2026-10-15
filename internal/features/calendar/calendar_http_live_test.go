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

func TestCalendarLiveSuite(t *testing.T) {
	suite.Run(t, &CalendarLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *CalendarLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.adminID = s.SeedUser("cal-admin", "password1", "Календарь Админ", "admin", true)
	s.SeedUser("cal-exec", "password1", "Календарь Мастер", "executor", true)
	s.admin = s.NewBrowser()
	s.admin.Login("cal-admin", "password1")
	s.executor = s.NewBrowser()
	s.executor.Login("cal-exec", "password1")
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('CAL Клиент') RETURNING id::text`).Scan(&s.clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM columns WHERE NOT is_final AND deleted_at IS NULL ORDER BY position LIMIT 1`).Scan(&s.workColumnID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM columns WHERE is_final AND deleted_at IS NULL ORDER BY position LIMIT 1`).Scan(&s.finalColumnID))
}

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
func (s *CalendarLiveSuite) insertTask(title, columnID, createdAt string, due, finishedAt *string) int64 {
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		WITH car AS (INSERT INTO equipment (client_id, plate_number) VALUES ($2, 'К 001') RETURNING id)
		INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by, created_at, due_date, finished_at)
		VALUES ($1, $2, (SELECT id FROM car), $3, 1, $4, $5::timestamptz, $6::date, $7::timestamptz) RETURNING id`,
		columnID, s.clientID, title, s.adminID, createdAt, due, finishedAt).Scan(&id))
	return id
}

func strp(v string) *string { return &v }

func (s *CalendarLiveSuite) getCalendar(b *testutil.Browser, from, to string) calendarDTO {
	code, body := b.Do(http.MethodGet, "/api/v1/calendar?from="+from+"&to="+to, nil)
	s.Require().Equal(http.StatusOK, code, string(body))
	var out calendarDTO
	s.Require().NoError(json.Unmarshal(body, &out))
	return out
}

func contains(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func (s *CalendarLiveSuite) TestSettings_AdminSetsCapacity() {
	code, body := s.admin.Do(http.MethodPut, "/api/v1/calendar/settings", map[string]any{"max_cars": 10})
	s.Require().Equal(http.StatusOK, code, string(body))

	code, body = s.executor.Do(http.MethodGet, "/api/v1/calendar/settings", nil)
	s.Require().Equal(http.StatusOK, code, string(body))
	var got settingsDTO
	s.Require().NoError(json.Unmarshal(body, &got))
	s.Equal(10, got.MaxCars)

	cal := s.getCalendar(s.admin, "2031-03-01", "2031-03-01")
	s.Equal(10, cal.Capacity)
}

func (s *CalendarLiveSuite) TestSettings_Validation() {
	for _, tc := range []struct {
		body any
		msg  string
	}{
		{map[string]any{"max_cars": 4}, "out_of_range"},
		{map[string]any{"max_cars": 13}, "out_of_range"},
		{map[string]any{}, "required"},
	} {
		code, body := s.admin.Do(http.MethodPut, "/api/v1/calendar/settings", tc.body)
		s.Require().Equal(http.StatusBadRequest, code, string(body))
		var e errDTO
		s.Require().NoError(json.Unmarshal(body, &e))
		s.Require().Len(e.Details.Fields, 1, string(body))
		s.Equal("max_cars", e.Details.Fields[0].Name)
		s.Equal(tc.msg, e.Details.Fields[0].Message)
	}
	code, _ := s.admin.Do(http.MethodPut, "/api/v1/calendar/settings", map[string]any{"max_cars": "abc"})
	s.Equal(http.StatusBadRequest, code)
}

func (s *CalendarLiveSuite) TestSettings_ExecutorForbidden() {
	code, body := s.executor.Do(http.MethodPut, "/api/v1/calendar/settings", map[string]any{"max_cars": 6})
	s.Equal(http.StatusForbidden, code, string(body))
}

func (s *CalendarLiveSuite) TestCalendar_RangeValidation() {
	code, body := s.admin.Do(http.MethodGet, "/api/v1/calendar?from=2031-02-30&to=2031-03-01", nil)
	s.Require().Equal(http.StatusBadRequest, code, string(body))
	code, body = s.admin.Do(http.MethodGet, "/api/v1/calendar?from=2031-01-01&to=2031-06-01", nil)
	s.Require().Equal(http.StatusBadRequest, code, string(body))
	var e errDTO
	s.Require().NoError(json.Unmarshal(body, &e))
	s.Equal("range_too_large", e.Details.Fields[0].Message)
}

func (s *CalendarLiveSuite) TestCalendar_RequiresAuth() {
	anon := s.NewBrowser()
	code, _ := anon.Do(http.MethodGet, "/api/v1/calendar", nil)
	s.Equal(http.StatusUnauthorized, code)
}

func (s *CalendarLiveSuite) TestCalendar_OccupancyByDay() {
	// Приёмка 2031-03-02 09:00 Алматы; срок 2031-03-04 — в будущем относительно «сегодня».
	withDue := s.insertTask("CAL А001АА", s.workColumnID, "2031-03-02T09:00:00+05:00", strp("2031-03-04"), nil)
	// Выдан 2031-03-03 (финальная колонка, finished_at задан явно).
	finished := s.insertTask("CAL В002ВВ", s.finalColumnID, "2031-03-01T10:00:00+05:00", nil, strp("2031-03-03T18:00:00+05:00"))
	// Без срока — бессрочно с 2031-03-05.
	open := s.insertTask("CAL С003СС", s.workColumnID, "2031-03-05T12:00:00+05:00", nil, nil)
	// Приёмка 2031-03-01 00:30 по Алматы = 2031-02-28 19:30 UTC — день берётся в поясе пользователя.
	tzEdge := s.insertTask("CAL Е004ЕЕ", s.finalColumnID, "2031-03-01T00:30:00+05:00", nil, strp("2031-03-01T20:00:00+05:00"))
	// Удалённый заказ не учитывается.
	deleted := s.insertTask("CAL Х005ХХ", s.workColumnID, "2031-03-02T09:00:00+05:00", nil, nil)
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE tasks SET deleted_at = now(), deleted_by = $2, delete_reason = 'тест' WHERE id = $1`, deleted, s.adminID)
	s.Require().NoError(err)

	cal := s.getCalendar(s.admin, "2031-02-28", "2031-03-07")
	s.Require().Len(cal.Days, 8)
	s.Equal("2031-02-28", cal.From)

	want := map[string][]int64{
		"2031-02-28": {},
		"2031-03-01": {finished, tzEdge},
		"2031-03-02": {finished, withDue},
		"2031-03-03": {finished, withDue},
		"2031-03-04": {withDue},
		"2031-03-05": {open},
		"2031-03-07": {open},
	}
	mine := map[int64]bool{withDue: true, finished: true, open: true, tzEdge: true, deleted: true}
	for _, d := range cal.Days {
		exp, ok := want[d.Date]
		if !ok {
			continue
		}
		var got []int64
		for _, id := range d.TaskIDs {
			if mine[id] {
				got = append(got, id)
			}
		}
		s.ElementsMatch(exp, got, d.Date)
		s.Equal(len(d.TaskIDs), d.Occupied, d.Date)
		s.Equal(max(cal.Capacity-d.Occupied, 0), d.Free, d.Date)
	}

	byID := map[int64]orderDTO{}
	for _, o := range cal.Orders {
		byID[o.ID] = o
	}
	s.NotContains(byID, deleted)
	s.True(byID[finished].IsFinished)
	s.Equal("2031-03-03", *byID[finished].EndDate)
	s.True(byID[open].OpenEnded)
	s.Nil(byID[open].EndDate)
	s.Equal("CAL Клиент", byID[withDue].ClientName)
	s.Equal("2031-03-02", byID[withDue].StartDate)
	s.Equal("2031-03-01", byID[tzEdge].StartDate)
}

func (s *CalendarLiveSuite) TestFinishedAtTrigger() {
	ctx := context.Background()
	id := s.insertTask("CAL trigger", s.workColumnID, "2031-04-01T09:00:00+05:00", nil, nil)
	var finished *string
	read := func() {
		s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT finished_at::text FROM tasks WHERE id = $1`, id).Scan(&finished))
	}
	read()
	s.Nil(finished)

	_, err := s.Pool.Exec(ctx, `UPDATE tasks SET column_id = $2 WHERE id = $1`, id, s.finalColumnID)
	s.Require().NoError(err)
	read()
	s.Require().NotNil(finished, "перенос в финальную колонку ставит finished_at")

	_, err = s.Pool.Exec(ctx, `UPDATE tasks SET column_id = $2 WHERE id = $1`, id, s.workColumnID)
	s.Require().NoError(err)
	read()
	s.Nil(finished, "возврат из финальной колонки снимает finished_at")
}

func (s *CalendarLiveSuite) TestSSR_Page() {
	id := s.insertTask("CAL ssr К777КК", s.workColumnID, "2031-05-03T09:00:00+05:00", strp("2031-05-04"), nil)
	for _, b := range []*testutil.Browser{s.admin, s.executor} {
		code, body := b.Do(http.MethodGet, "/calendar?month=2031-05&day=2031-05-03", nil)
		s.Require().Equal(http.StatusOK, code)
		page := string(body)
		s.Contains(page, `data-testid="cal-grid"`)
		s.Contains(page, "Май 2031")
		s.Contains(page, fmt.Sprintf(`data-task-id="%d"`, id))
		s.Contains(page, `data-testid="cal-gantt"`, "диаграмма Ганта выбранного дня")
		s.Contains(page, `data-testid="cal-gantt-row" data-task-id="`+fmt.Sprint(id)+`" data-col="8" data-span="2" data-days="2"`, "03..04 мая со шкалой с 26 апреля")
		s.Contains(page, "CAL ssr К777КК")
		s.Contains(page, `href="/calendar"`, "пункт сайдбара")
	}
	_, adminPage := s.admin.Do(http.MethodGet, "/calendar", nil)
	s.Contains(string(adminPage), `id="cal-save"`)
	_, execPage := s.executor.Do(http.MethodGet, "/calendar", nil)
	s.NotContains(string(execPage), `id="cal-save"`)
}
