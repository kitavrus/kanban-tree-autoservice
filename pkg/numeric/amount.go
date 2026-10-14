// Package numeric — общий тип для NUMERIC-полей БД (ADR-101: перенос из
// internal/features/catalog, где Amount появился первым, — теперь используется и board,
// и catalog, без кросс-импорта между фичами).
package numeric

import "errors"

// Amount — денежная/количественная величина NUMERIC. В Go хранится строкой (без потери
// точности — float64 не годится для точных денежных сумм), на wire — JSON-числом без
// кавычек. Пустая строка на входе/выходе трактуется как "0".
type Amount string

// errAmountNotNumber — присланное значение не JSON-число (строка/объект/массив).
var errAmountNotNumber = errors.New("numeric: amount must be a JSON number")

func (a Amount) MarshalJSON() ([]byte, error) {
	s := string(a)
	if s == "" {
		s = "0"
	}
	return []byte(s), nil
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		*a = ""
		return nil
	}
	if len(s) == 0 || s[0] == '"' || s[0] == '{' || s[0] == '[' {
		return errAmountNotNumber
	}
	*a = Amount(s)
	return nil
}

// String — как хранится (для передачи в SQL как text-параметр с явным ::numeric cast).
func (a Amount) String() string {
	if a == "" {
		return "0"
	}
	return string(a)
}
