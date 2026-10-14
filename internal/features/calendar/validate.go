package calendar

import (
	"time"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Границы вместимости — те же, что CHECK shop_settings.max_cars.
const (
	MinCars = 5
	MaxCars = 12
	// maxSpanDays — предел окна GET /calendar (сетка месяца — 42 дня).
	maxSpanDays = 62
)

// Коды ошибок полей (details.fields[].message) — клиент переводит их в текст.
const (
	msgRequired     = "required"
	msgOutOfRange   = "out_of_range"
	msgInvalid      = "invalid"
	msgBeforeFrom   = "before_from"
	msgRangeTooWide = "range_too_large"
)

// ValidateSettings — max_cars обязателен и в пределах 5–12.
func ValidateSettings(req SettingsRequest) (Settings, error) {
	if req.MaxCars == nil {
		return Settings{}, errorspkg.Validation(errorspkg.Field{Name: "max_cars", Message: msgRequired})
	}
	if *req.MaxCars < MinCars || *req.MaxCars > MaxCars {
		return Settings{}, errorspkg.Validation(errorspkg.Field{Name: "max_cars", Message: msgOutOfRange})
	}
	return Settings{MaxCars: *req.MaxCars}, nil
}

// ParseRange — окно from..to (YYYY-MM-DD, включительно). Пустые — текущий месяц от today.
func ParseRange(from, to string, today time.Time) (time.Time, time.Time, error) {
	if from == "" && to == "" {
		first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		return first, first.AddDate(0, 1, -1), nil
	}
	var fields []errorspkg.Field
	f, errF := time.Parse(time.DateOnly, from)
	if errF != nil {
		fields = append(fields, errorspkg.Field{Name: "from", Message: msgInvalid})
	}
	t, errT := time.Parse(time.DateOnly, to)
	if errT != nil {
		fields = append(fields, errorspkg.Field{Name: "to", Message: msgInvalid})
	}
	if len(fields) > 0 {
		return time.Time{}, time.Time{}, errorspkg.Validation(fields...)
	}
	if t.Before(f) {
		return time.Time{}, time.Time{}, errorspkg.Validation(errorspkg.Field{Name: "to", Message: msgBeforeFrom})
	}
	if daysBetween(f, t)+1 > maxSpanDays {
		return time.Time{}, time.Time{}, errorspkg.Validation(errorspkg.Field{Name: "to", Message: msgRangeTooWide})
	}
	return f, t, nil
}

// daysBetween — целых дней от a до b (обе — полночь UTC).
func daysBetween(a, b time.Time) int { return int(b.Sub(a).Hours() / 24) }
