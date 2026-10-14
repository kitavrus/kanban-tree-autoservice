//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Часовой пояс пользователя на SSR-печати и в фильтре сроков доски
// (docs/features/user-timezone/draft-spec.md §8.1 «Где применяется», TS-041, DS-032).
// Методы — на PrintLiveSuite: те же фикстуры заказа, живой бинарь и реальный вход (§TD-8).

// tzFixtureMoment — TS-F5: 00:30 7 октября по Алматы = 22:30 6 октября по Москве.
const tzFixtureMoment = "2026-10-06T19:30:00Z"

// tzAdmin — новый администратор с поясом tz, выставленным настоящим endpoint'ом.
func (s *PrintLiveSuite) tzAdmin(login, tz string) *testutil.Browser {
	s.SeedUser(login, "password1", "Пояс "+login, "admin", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	st, raw := b.Do(http.MethodPut, "/api/v1/users/me/timezone", map[string]any{"timezone": tz})
	s.Require().Equal(http.StatusOK, st, string(raw))
	return b
}

func (s *PrintLiveSuite) printPage(b *testutil.Browser, taskID int64, kind string) string {
	resp := b.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/%s", taskID, kind), nil, nil)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	return s.readBody(resp)
}

// TS-041: печать «Выдача» — дата оплаты одного и того же заказа в поясе смотрящего.
func (s *PrintLiveSuite) TestTS041_Handover_PaidDate_InViewerTimezone() {
	c := s.newClient("Клиент-Пояс-Оплата", "", "")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c, col, "")
	s.markPaid(fmt.Sprintf("%d", taskID), c, col, version)
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE tasks SET paid_at = $2::timestamptz WHERE id = $1`, taskID, tzFixtureMoment)
	s.Require().NoError(err)

	almaty := s.tzAdmin("tz-print-almaty", "Asia/Almaty")
	moscow := s.tzAdmin("tz-print-moscow", "Europe/Moscow")

	s.Contains(s.printPage(almaty, taskID, "handover"), "Оплачено 07.10.2026")
	body := s.printPage(moscow, taskID, "handover")
	s.Contains(body, "Оплачено 06.10.2026")
	s.NotContains(body, "Оплачено 07.10.2026")
}

// TS-041: печать «Приём» и «Выдача» — дата печати в поясе смотрящего, а не сервера.
// Пояса UTC+14 и UTC-12 разнесены на 26 часов: их календарные даты различаются ВСЕГДА,
// поэтому тест не зависит от времени запуска и от пояса сервера.
func (s *PrintLiveSuite) TestTS041_PrintDate_InViewerTimezone() {
	c := s.newClient("Клиент-Пояс-Дата-Печати", "", "")
	taskID, _ := s.newTask(c, s.columnID("Новые"), "")

	for i, zone := range []string{"Pacific/Kiritimati", "Etc/GMT+12"} {
		loc, err := time.LoadLocation(zone)
		s.Require().NoError(err)
		b := s.tzAdmin(fmt.Sprintf("tz-date-%d", i), zone)
		for _, kind := range []string{"intake", "handover"} {
			before := time.Now().In(loc).Format("02.01.2006")
			body := s.printPage(b, taskID, kind)
			after := time.Now().In(loc).Format("02.01.2006")
			// before/after — на случай полуночи пояса ровно во время запроса.
			s.True(strings.Contains(body, `data-testid="print-date">Дата: `+before+`<`) ||
				strings.Contains(body, `data-testid="print-date">Дата: `+after+`<`),
				"%s %s: ожидалась дата %s", zone, kind, before)
		}
	}
	east := time.Now().In(time.FixedZone("", 14*3600)).Format("02.01.2006")
	west := time.Now().In(time.FixedZone("", -12*3600)).Format("02.01.2006")
	s.NotEqual(east, west, "предпосылка теста: даты поясов различаются")
}

// TS-041 (негатив «срок считается по поясу пользователя»): фильтр доски due=overdue/soon и
// счётчик subtasks_late берут «сегодня» из пояса пользователя, а не из current_date БД.
// Срок = «вчера» по UTC+14: для UTC+14 заказ просрочен, для UTC-12 (там ещё «вчера» или
// раньше) — нет.
func (s *PrintLiveSuite) TestTS041_DueFilter_TodayInViewerTimezone() {
	ctx := context.Background()
	c := s.newClient("Клиент-Пояс-Срок", "", "")
	taskID, _ := s.newTask(c, s.columnID("Новые"), "")

	eastToday := time.Now().In(time.FixedZone("", 14*3600))
	due := eastToday.AddDate(0, 0, -1).Format(time.DateOnly)
	_, err := s.Pool.Exec(ctx, `UPDATE tasks SET due_date = $2::date WHERE id = $1`, taskID, due)
	s.Require().NoError(err)
	// Услуга с тем же сроком — для счётчика subtasks_late.
	svc := s.newService("Услуга-Пояс-Срок", "100")
	sub := s.addSubtask(taskID, svc)
	_, err = s.Pool.Exec(ctx, `UPDATE subtasks SET due_date = $2::date WHERE id::text = $1`, sub, due)
	s.Require().NoError(err)

	east := s.tzAdmin("tz-due-east", "Pacific/Kiritimati")
	west := s.tzAdmin("tz-due-west", "Etc/GMT+12")

	type listResp struct {
		Items []struct {
			ID           int64 `json:"id"`
			SubtasksLate int   `json:"subtasks_late"`
		} `json:"items"`
		Total int `json:"total"`
	}
	list := func(b *testutil.Browser, dueFilter string) listResp {
		st, raw := b.Do(http.MethodGet, "/api/v1/tasks?assignee=all&client_id="+c+"&due="+dueFilter, nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var r listResp
		s.Require().NoError(json.Unmarshal(raw, &r), string(raw))
		return r
	}

	// Восток (UTC+14): срок — вчера → просрочен, услуга просрочена.
	r := list(east, "overdue")
	s.Require().Equal(1, r.Total, "UTC+14: срок %s уже прошёл", due)
	s.Equal(taskID, r.Items[0].ID)
	s.Equal(1, r.Items[0].SubtasksLate)

	// Запад (UTC-12): для него этот срок — сегодня или позже → не просрочен.
	// Услугу снимаем с просрочки фильтра отдельно: EXISTS по subtasks тоже считается от «сегодня».
	r = list(west, "overdue")
	s.Equal(0, r.Total, "UTC-12: срок %s ещё не прошёл", due)
	r = list(west, "all")
	s.Require().Equal(1, r.Total)
	s.Equal(0, r.Items[0].SubtasksLate, "UTC-12: услуга не просрочена")
}

// TS-041: счётчик просроченных услуг (subtasks_late) в карточке заказа и в ответах мутаций
// считается от «сегодня» в поясе пользователя — так же, как в списке (Actor.Today, один
// источник на пакет). Срок услуги = «вчера» по UTC+14: для UTC+14 она просрочена, для
// UTC-12 — нет, какой бы ни была дата сервера БД. Список, GET /tasks/:id, GET /tasks/:id/card
// и ответы PUT заказа / PUT услуги дают одному зрителю одно и то же число.
func (s *PrintLiveSuite) TestTS041_CardAndMutations_SubtasksLate_TodayInViewerTimezone() {
	c := s.newClient("Клиент-Пояс-Карточка", "", "")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col, "")
	sub := s.addSubtask(taskID, s.newService("Услуга-Пояс-Карточка", "100"))
	due := time.Now().In(time.FixedZone("", 14*3600)).AddDate(0, 0, -1).Format(time.DateOnly)

	east := s.tzAdmin("tz-card-east", "Pacific/Kiritimati")
	west := s.tzAdmin("tz-card-west", "Etc/GMT+12")

	type taskBody struct {
		ID           int64 `json:"id"`
		Version      int64 `json:"version"`
		SubtasksLate int   `json:"subtasks_late"`
	}
	decode := func(raw []byte, v any) {
		s.Require().NoError(json.Unmarshal(raw, v), string(raw))
	}
	// Срок услуге ставит настоящая мутация PUT /subtasks/:id — её ответ (карточка) тоже проверяем.
	putSubtask := func(b *testutil.Browser, status string) int {
		st, raw := b.Do(http.MethodPut, "/api/v1/subtasks/"+sub, map[string]any{
			"status": status, "assignee_id": nil, "due_date": due, "price": 100, "equipment_id": nil,
		})
		s.Require().Equal(http.StatusOK, st, string(raw))
		var m struct {
			Card struct {
				Task taskBody `json:"task"`
			} `json:"card"`
		}
		decode(raw, &m)
		return m.Card.Task.SubtasksLate
	}
	getTask := func(b *testutil.Browser) taskBody {
		st, raw := b.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", taskID), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var t taskBody
		decode(raw, &t)
		return t
	}
	getCard := func(b *testutil.Browser) int {
		st, raw := b.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var m struct {
			Task taskBody `json:"task"`
		}
		decode(raw, &m)
		return m.Task.SubtasksLate
	}
	listLate := func(b *testutil.Browser) int {
		st, raw := b.Do(http.MethodGet, "/api/v1/tasks?assignee=all&client_id="+c, nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var r struct {
			Items []taskBody `json:"items"`
		}
		decode(raw, &r)
		s.Require().Len(r.Items, 1)
		return r.Items[0].SubtasksLate
	}
	// PUT /tasks/:id — ответ мутации заказа (Re-fetch-after-write).
	putTask := func(b *testutil.Browser, title string) int {
		st, raw := b.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
			"title": title, "client_id": c, "column_id": col,
			"assignee_id": nil, "due_date": nil, "is_paid": false, "version": getTask(b).Version,
		})
		s.Require().Equal(http.StatusOK, st, string(raw))
		var m struct {
			Task taskBody `json:"task"`
		}
		decode(raw, &m)
		return m.Task.SubtasksLate
	}

	for _, v := range []struct {
		name string
		b    *testutil.Browser
		want int
	}{
		{"UTC+14: срок — вчера, услуга просрочена", east, 1},
		{"UTC-12: срок — сегодня или позже, не просрочена", west, 0},
	} {
		s.Equal(v.want, putSubtask(v.b, "in_progress"), "%s: ответ PUT /subtasks/:id", v.name)
		s.Equal(v.want, getTask(v.b).SubtasksLate, "%s: GET /tasks/:id", v.name)
		s.Equal(v.want, getCard(v.b), "%s: GET /tasks/:id/card", v.name)
		s.Equal(v.want, putTask(v.b, "Заказ печати — "+v.name[:6]), "%s: ответ PUT /tasks/:id", v.name)
		s.Equal(v.want, listLate(v.b), "%s: список — то же число, что карточка", v.name)
	}

	// Выполненная услуга не просрочена ни для кого.
	s.Equal(0, putSubtask(east, "done"))
	s.Equal(0, getTask(east).SubtasksLate)
}
