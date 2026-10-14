package sqlutil

import (
	"strings"
	"unicode"
)

// PlateCompactSQL — SQL-выражение «госномер без пробелов и дефисов» для колонки col.
// В БД госномер хранится нормализованным (верхний регистр), поэтому сравнение с
// PlatePattern идёт обычным LIKE — регистр приведён на стороне Go (ILIKE для кириллицы
// зависит от LC_CTYPE БД, верхний регистр в Go — нет).
func PlateCompactSQL(col string) string {
	return "replace(replace(" + col + ", ' ', ''), '-', '')"
}

// PlateCompact — госномер для ТОЧНОГО сравнения с PlateCompactSQL: без пробельных символов
// и дефисов, в верхнем регистре («а123-вс 777» → «А123ВС777»). "" — сравнивать нечего.
func PlateCompact(q string) string {
	var b strings.Builder
	for _, r := range q {
		if unicode.IsSpace(r) || r == '-' {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

// PlatePattern — LIKE-шаблон поиска по госномеру: запрос без пробельных символов и дефисов,
// в верхнем регистре, с экранированием LIKE («а123вс 777» → «%А123ВС777%»). "" — в запросе
// нет ни одного символа для сравнения (сравнение по госномеру не применяется).
func PlatePattern(q string) string {
	c := PlateCompact(q)
	if c == "" {
		return ""
	}
	return "%" + EscapeLike(c) + "%"
}
