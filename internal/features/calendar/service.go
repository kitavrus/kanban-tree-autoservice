package calendar

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// Service — расчёт загрузки по дням и настройка вместимости.
type Service struct {
	repo *Repo
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepo(pool), now: time.Now}
}

// Today — сегодняшняя дата в поясе tz как полночь UTC (арифметика дат без переводов часов).
func (s *Service) Today(tz string) time.Time {
	y, m, d := timezone.In(s.now(), tz).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Calendar — загрузка окна from..to (включительно) в поясе tz.
func (s *Service) Calendar(ctx context.Context, tz string, from, to time.Time) (*CalendarResponse, error) {
	set, err := s.repo.Settings(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.Orders(ctx, tz, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	return build(rows, set.MaxCars, s.Today(tz), from, to), nil
}

// Settings / SaveSettings — вместимость (5–12 машин одновременно).
func (s *Service) Settings(ctx context.Context) (Settings, error) { return s.repo.Settings(ctx) }

func (s *Service) SaveSettings(ctx context.Context, req SettingsRequest) (Settings, error) {
	set, err := ValidateSettings(req)
	if err != nil {
		return Settings{}, err
	}
	if err := s.repo.SaveSettings(ctx, set); err != nil {
		return Settings{}, err
	}
	return set, nil
}

// build — чистый расчёт: интервал каждого заказа (правила — doc пакета) и счётчик по дням.
func build(rows []orderRow, capacity int, today, from, to time.Time) *CalendarResponse {
	resp := &CalendarResponse{
		Capacity: capacity,
		Today:    today.Format(time.DateOnly),
		From:     from.Format(time.DateOnly),
		To:       to.Format(time.DateOnly),
		Days:     []Day{},
		Orders:   []Order{},
	}
	type span struct {
		id         int64
		start, end time.Time // end.IsZero() — бессрочно
	}
	var spans []span
	for _, r := range rows {
		start, err := time.Parse(time.DateOnly, r.StartDate)
		if err != nil {
			continue
		}
		o := Order{ID: r.ID, Title: r.Title, ClientName: r.ClientName, ColumnName: r.ColumnName,
			ColumnColor: r.ColumnColor, StartDate: r.StartDate, DueDate: r.DueDate}
		var end time.Time
		switch {
		case r.FinishedOn != nil:
			o.IsFinished = true
			end, _ = time.Parse(time.DateOnly, *r.FinishedOn)
			if end.Before(start) {
				// выдан раньше плановой даты начала (отмена/ошибка записи) — машины в сервисе в
				// плановые дни не было: места не занимает ни в сетке, ни на диаграмме.
				continue
			}
		case r.DueDate != nil:
			if due, err := time.Parse(time.DateOnly, *r.DueDate); err == nil && !due.Before(today) {
				end = due
			}
		}
		if end.IsZero() {
			o.OpenEnded = true
		} else {
			e := end.Format(time.DateOnly)
			o.EndDate = &e
			if end.Before(from) {
				continue
			}
		}
		spans = append(spans, span{id: r.ID, start: start, end: end})
		resp.Orders = append(resp.Orders, o)
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		day := Day{Date: d.Format(time.DateOnly), TaskIDs: []int64{}}
		for _, sp := range spans {
			if !d.Before(sp.start) && (sp.end.IsZero() || !d.After(sp.end)) {
				day.TaskIDs = append(day.TaskIDs, sp.id)
			}
		}
		day.Occupied = len(day.TaskIDs)
		if day.Occupied < capacity {
			day.Free = capacity - day.Occupied
		}
		resp.Days = append(resp.Days, day)
	}
	return resp
}
