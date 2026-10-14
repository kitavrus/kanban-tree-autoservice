// Package timezone — часовой пояс пользователя (DS-031, DS-033): валидация имени IANA,
// перевод времени в пояс для SSR-шаблонов и перечень значений select'а.
//
// Хранение времени — UTC (timestamptz); пояс влияет только на ПОКАЗ.
package timezone

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	// База IANA вшивается в бинарь: образ без /usr/share/zoneinfo не ломает LoadLocation.
	_ "time/tzdata"
)

const (
	// Default — пояс по умолчанию для новых и существующих пользователей (DS-033).
	Default = "Asia/Almaty"
	// MaxLen — предел длины имени пояса (CHECK в БД: 1..64).
	MaxLen = 64
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_+\-/]+$`)

// locations — кэш загруженных поясов по имени. Кладутся только ВАЛИДНЫЕ имена (их множество
// конечно — база IANA), поэтому мусор из запросов кэш не раздувает.
var locations sync.Map // string → *time.Location

// lookup — пояс по имени или nil. Проверка детерминирована и не зависит от ОС: системный
// zoneinfo (у Go он в приоритете над вшитой базой) на Linux содержит не-IANA имена —
// posix/…, right/…, localtime, posixrules, — которые браузерный Intl не знает: JS ушёл бы
// на пояс по умолчанию, а SSR показал бы реальный пояс. Такие имена отклоняются до загрузки.
func lookup(name string) *time.Location {
	if loc, ok := locations.Load(name); ok {
		return loc.(*time.Location)
	}
	if name == "" || len(name) > MaxLen || !nameRe.MatchString(name) {
		return nil
	}
	lower := strings.ToLower(name)
	if name[0] == '/' || strings.Contains(name, "..") ||
		strings.HasPrefix(lower, "posix/") || strings.HasPrefix(lower, "right/") ||
		lower == "local" || lower == "localtime" || lower == "posixrules" {
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil
	}
	locations.Store(name, loc)
	return loc
}

// Valid — имя распознаётся как пояс IANA: 1–64 символа, только [A-Za-z0-9_+-/], не
// служебное имя zoneinfo (см. lookup), известно time.LoadLocation.
func Valid(name string) bool { return lookup(name) != nil }

// Load — пояс по имени; неизвестное/пустое имя → пояс по умолчанию (страница не ломается).
// Один поиск в кэше на вызов.
func Load(name string) *time.Location {
	if loc := lookup(name); loc != nil {
		return loc
	}
	if loc := lookup(Default); loc != nil {
		return loc
	}
	return time.FixedZone(Default, 5*60*60) // недостижимо: tzdata вшита
}

// In — момент t в поясе name (см. Load).
func In(t time.Time, name string) time.Time { return t.In(Load(name)) }

// DayStart — начало календарного дня date (YYYY-MM-DD) в поясе name как момент времени
// (границы дня фильтров по дате: сравнение с timestamptz идёт в UTC, DS-034). false — date не
// календарная дата (в т.ч. 2026-13-40, 2026-02-30).
func DayStart(name, date string) (time.Time, bool) {
	t, err := time.ParseInLocation(time.DateOnly, date, Load(name))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// NextDay — начало следующего календарного дня в поясе момента dayStart (исключающая верхняя
// граница «по дату включительно»). Через календарь, а не +24 ч: сутки с переводом часов
// длятся 23 или 25 часов.
func NextDay(dayStart time.Time) time.Time {
	y, m, d := dayStart.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, dayStart.Location())
}

// Format — время в поясе name по layout. v — time.Time или *time.Time; nil/нулевое → "".
// Шаблонная функция SSR «время в поясе пользователя» (ssr.Funcs "timeIn").
func Format(name, layout string, v any) string {
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case *time.Time:
		if x == nil {
			return ""
		}
		t = *x
	default:
		return ""
	}
	if t.IsZero() {
		return ""
	}
	return In(t, name).Format(layout)
}

// Option — пункт select'а «Часовой пояс».
type Option struct {
	Value string // имя IANA
	Label string // «Алматы (UTC+5)»
}

// zones — фиксированный перечень select'а (§2 спеки): первые два — Алматы и Москва,
// далее остальные по алфавиту подписи. Валидатор принимает и пояса вне перечня.
var zones = []struct{ name, city string }{
	{"Asia/Almaty", "Алматы"},
	{"Europe/Moscow", "Москва"},
	{"Asia/Aqtau", "Актау"},
	{"Asia/Ashgabat", "Ашхабад"},
	{"Asia/Baku", "Баку"},
	{"Asia/Bishkek", "Бишкек"},
	{"Asia/Vladivostok", "Владивосток"},
	{"Asia/Dushanbe", "Душанбе"},
	{"Asia/Yekaterinburg", "Екатеринбург"},
	{"Asia/Yerevan", "Ереван"},
	{"Asia/Irkutsk", "Иркутск"},
	{"Europe/Kaliningrad", "Калининград"},
	{"Asia/Krasnoyarsk", "Красноярск"},
	{"Europe/Minsk", "Минск"},
	{"Asia/Novosibirsk", "Новосибирск"},
	{"Asia/Omsk", "Омск"},
	{"Europe/Samara", "Самара"},
	{"Asia/Tashkent", "Ташкент"},
	{"Asia/Tbilisi", "Тбилиси"},
	{"Asia/Yakutsk", "Якутск"},
}

// Options — пункты select'а с подписью смещения на момент now. current вне перечня
// (задан через API) добавляется последним пунктом, чтобы select показывал факт.
func Options(now time.Time, current string) []Option {
	out := make([]Option, 0, len(zones)+1)
	found := current == ""
	for _, z := range zones {
		out = append(out, Option{Value: z.name, Label: z.city + " (" + offsetLabel(now, z.name) + ")"})
		found = found || z.name == current
	}
	if !found {
		label := current
		if Valid(current) {
			label += " (" + offsetLabel(now, current) + ")"
		}
		out = append(out, Option{Value: current, Label: label})
	}
	return out
}

// offsetLabel — «UTC+5», «UTC-3», «UTC+5:30».
func offsetLabel(now time.Time, name string) string {
	_, sec := In(now, name).Zone()
	sign := "+"
	if sec < 0 {
		sign, sec = "-", -sec
	}
	h, m := sec/3600, sec%3600/60
	if m == 0 {
		return fmt.Sprintf("UTC%s%d", sign, h)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, h, m)
}
