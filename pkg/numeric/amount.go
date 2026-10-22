// Package numeric — общий тип для NUMERIC-полей БД (ADR-101: перенос из
// internal/features/catalog, где Amount появился первым, — теперь используется и board,
// и catalog, без кросс-импорта между фичами).
package numeric

import (
	"errors"
	"strings"
)

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

// nbsp — разделитель тысяч в RU: неразрывный пробел, число не переносится посередине.
const nbsp = " "

// RU — сумма для людей: «28400.00» → «28 400,00» (тысячи через неразрывный пробел,
// копейки через запятую, всегда два знака). Нечисловое значение возвращается как есть.
func (a Amount) RU() string {
	return a.ru(func(frac string) string {
		for len(frac) < 2 {
			frac += "0"
		}
		return frac
	})
}

// RUQty — количество для людей: «1.000» → «1», «2.500» → «2,5», «1500.000» → «1 500».
func (a Amount) RUQty() string {
	return a.ru(func(frac string) string { return strings.TrimRight(frac, "0") })
}

// ru — разбор «[-]целая[.дробная]»: группировка тысяч + дробная часть через fracFn
// (пустая дробная часть — без запятой).
func (a Amount) ru(fracFn func(string) string) string {
	s := a.String()
	intPart, frac, _ := strings.Cut(s, ".")
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	if intPart == "" || strings.Trim(intPart, "0123456789") != "" || strings.Trim(frac, "0123456789") != "" {
		return s
	}
	frac = fracFn(frac)
	var b strings.Builder
	if neg {
		b.WriteString("-")
	}
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(nbsp)
		}
		b.WriteRune(r)
	}
	if frac != "" {
		b.WriteString(",")
		b.WriteString(frac)
	}
	return b.String()
}
