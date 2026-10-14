package timezone

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fixtureMoment — TS-F5: 2026-10-06T19:30:00Z = 00:30 7 октября по Алматы, 22:30 6 октября по Москве.
var fixtureMoment = time.Date(2026, 10, 6, 19, 30, 0, 0, time.UTC)

// TS-021 / TS-022 (unit): валидатор имени пояса.
func TestTS021_Valid(t *testing.T) {
	for _, name := range []string{"Asia/Almaty", "Europe/Moscow", "UTC", "America/Argentina/Buenos_Aires", "Etc/GMT+5"} {
		assert.True(t, Valid(name), name)
	}
	bad := map[string]string{
		"пусто":            "",
		"неизвестный":      "Mars/Olympus",
		"обход каталога":   "../../etc/passwd",
		"абсолютный путь":  "/etc/passwd",
		"65 символов":      strings.Repeat("a", 65),
		"10 000 символов":  strings.Repeat("A", 10000),
		"пояс сервера":     "Local",
		"пробел":           "Asia/Almaty ",
		"кириллица":        "Алматы",
		"sql":              "Asia/Almaty';--",
		"каталог без зоны": "Asia/",
		// Служебные имена системного zoneinfo (есть на Linux, браузерный Intl их не знает).
		"posix-копия":       "posix/Asia/Almaty",
		"right-копия":       "right/Europe/Moscow",
		"posix в регистре":  "POSIX/Asia/Almaty",
		"localtime":         "localtime",
		"local в регистре":  "local",
		"posixrules":        "posixrules",
		"обход внутри пути": "Asia/../Asia/Almaty",
		"64+1 символ":       "Asia/" + strings.Repeat("a", 60),
	}
	for label, name := range bad {
		assert.False(t, Valid(name), label)
	}
}

// Кэш: повторная загрузка отдаёт тот же *time.Location; невалидные имена не кэшируются;
// параллельные вызовы безопасны (go test -race).
func TestLoad_Cached(t *testing.T) {
	first := Load("Europe/Samara")
	assert.Same(t, first, Load("Europe/Samara"))
	_, cached := locations.Load("Europe/Samara")
	assert.True(t, cached)

	assert.False(t, Valid("Mars/Olympus"))
	_, cached = locations.Load("Mars/Olympus")
	assert.False(t, cached, "мусор кэш не раздувает")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.Equal(t, "06.10.2026 22:30", Format("Europe/Moscow", "02.01.2006 15:04", fixtureMoment))
			assert.Equal(t, Default, Load("posix/Asia/Almaty").String())
		}()
	}
	wg.Wait()
}

func TestLoad_FallsBackToDefault(t *testing.T) {
	assert.Equal(t, "Europe/Moscow", Load("Europe/Moscow").String())
	for _, name := range []string{"", "Mars/Olympus", "Local", "../../etc/passwd", "posix/Europe/Moscow", "localtime"} {
		assert.Equal(t, Default, Load(name).String(), "%q", name)
	}
}

// TS-041 (unit, форматтер SSR): один момент — разные дата и время в разных поясах.
func TestTS041_Format(t *testing.T) {
	const layout = "02.01.2006 15:04"
	assert.Equal(t, "07.10.2026 00:30", Format("Asia/Almaty", layout, fixtureMoment))
	assert.Equal(t, "06.10.2026 22:30", Format("Europe/Moscow", layout, fixtureMoment))
	// Дата без времени тоже зависит от пояса (печатные формы, история оборудования).
	assert.Equal(t, "07.10.2026", Format("Asia/Almaty", "02.01.2006", fixtureMoment))
	assert.Equal(t, "06.10.2026", Format("Europe/Moscow", "02.01.2006", fixtureMoment))

	// Исходный пояс значения не важен — важен момент.
	almaty := fixtureMoment.In(Load("Asia/Almaty"))
	assert.Equal(t, "06.10.2026 22:30", Format("Europe/Moscow", layout, almaty))

	// *time.Time (PaidAt), nil и нулевое время.
	assert.Equal(t, "06.10.2026 22:30", Format("Europe/Moscow", layout, &fixtureMoment))
	assert.Empty(t, Format("Europe/Moscow", layout, (*time.Time)(nil)))
	assert.Empty(t, Format("Europe/Moscow", layout, time.Time{}))
	assert.Empty(t, Format("Europe/Moscow", layout, "2026-10-06"))

	// Неизвестный пояс → пояс по умолчанию, не паника и не UTC.
	assert.Equal(t, "07.10.2026 00:30", Format("Mars/Olympus", layout, fixtureMoment))
	assert.Equal(t, "07.10.2026 00:30", Format("", layout, fixtureMoment))
}

func TestOptions(t *testing.T) {
	opts := Options(fixtureMoment, Default)
	assert.Equal(t, Option{Value: "Asia/Almaty", Label: "Алматы (UTC+5)"}, opts[0])
	assert.Equal(t, Option{Value: "Europe/Moscow", Label: "Москва (UTC+3)"}, opts[1])
	assert.Len(t, opts, len(zones))

	// Остальные — по алфавиту подписи; все значения валидны и уникальны.
	seen := map[string]bool{}
	for i, o := range opts {
		assert.True(t, Valid(o.Value), o.Value)
		assert.False(t, seen[o.Value], o.Value)
		seen[o.Value] = true
		if i > 2 {
			assert.Less(t, opts[i-1].Label, o.Label)
		}
	}

	// Пояс вне перечня (задан через API) — добавлен последним, select показывает факт.
	extra := Options(fixtureMoment, "Asia/Kolkata")
	assert.Len(t, extra, len(zones)+1)
	assert.Equal(t, Option{Value: "Asia/Kolkata", Label: "Asia/Kolkata (UTC+5:30)"}, extra[len(extra)-1])

	// Невалидное значение (ре-рендер формы с ошибкой) — без смещения.
	bogus := Options(fixtureMoment, "Mars/Olympus")
	assert.Equal(t, Option{Value: "Mars/Olympus", Label: "Mars/Olympus"}, bogus[len(bogus)-1])

	assert.Len(t, Options(fixtureMoment, ""), len(zones))
}

func TestOffsetLabel(t *testing.T) {
	assert.Equal(t, "UTC+0", offsetLabel(fixtureMoment, "UTC"))
	assert.Equal(t, "UTC-3", offsetLabel(fixtureMoment, "America/Argentina/Buenos_Aires"))
	assert.Equal(t, "UTC+5:45", offsetLabel(fixtureMoment, "Asia/Kathmandu"))
}

// TS-023 (unit, transfers-partners DS-034): границы календарного дня в поясе пользователя —
// момент времени для сравнения с timestamptz. fixtureMoment попадает в 7 октября по Алматы и
// в 6 октября по Москве.
func TestTS023_DayStartAndNextDay(t *testing.T) {
	within := func(zone, date string) bool {
		start, ok := DayStart(zone, date)
		if !assert.True(t, ok, zone+" "+date) {
			return false
		}
		return !fixtureMoment.Before(start) && fixtureMoment.Before(NextDay(start))
	}
	assert.True(t, within("Asia/Almaty", "2026-10-07"))
	assert.False(t, within("Asia/Almaty", "2026-10-06"))
	assert.True(t, within("Europe/Moscow", "2026-10-06"))
	assert.False(t, within("Europe/Moscow", "2026-10-07"))

	start, ok := DayStart("Asia/Almaty", "2026-10-07")
	assert.True(t, ok)
	assert.Equal(t, time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC), start.UTC())
	assert.Equal(t, time.Date(2026, 10, 7, 19, 0, 0, 0, time.UTC), NextDay(start).UTC())

	// неизвестный пояс → пояс по умолчанию (как Load), дата по-прежнему разбирается.
	fallback, ok := DayStart("Mars/Olympus", "2026-10-07")
	assert.True(t, ok)
	assert.Equal(t, start, fallback)

	// сутки с переводом часов длятся не 24 часа: следующий день — по календарю.
	spring, ok := DayStart("Europe/Berlin", "2026-03-29")
	assert.True(t, ok)
	assert.Equal(t, 23*time.Hour, NextDay(spring).Sub(spring))
	// конец месяца и года.
	last, ok := DayStart("Asia/Almaty", "2026-12-31")
	assert.True(t, ok)
	assert.Equal(t, "2027-01-01", NextDay(last).Format(time.DateOnly))

	for _, bad := range []string{"", "2026-13-40", "2026-02-30", "2026-10-7", "07.10.2026", "2026-10-07T00:00:00Z", " 2026-10-07"} {
		_, ok := DayStart("Asia/Almaty", bad)
		assert.False(t, ok, bad)
	}
}
