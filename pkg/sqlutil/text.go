package sqlutil

import (
	"strings"
	"unicode/utf8"
)

// HasInvalidText — строка непригодна для хранения/поиска в Postgres text: невалидная
// UTF-8-последовательность или NUL-байт (`\u0000`). Postgres отклоняет такие значения на
// уровне протокола (SQLSTATE 22021 invalid_byte_sequence / 22P05 untranslatable_character)
// ещё до записи в колонку — без этой проверки на входе ошибка доходит до БД и превращается
// в 500 вместо 400 (code review, §TD-4 «мусор → 400, не 500»).
func HasInvalidText(s string) bool {
	return !utf8.ValidString(s) || strings.ContainsRune(s, 0)
}
