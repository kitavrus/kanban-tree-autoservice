package numeric

import (
	"regexp"
	"strings"
)

// Коды ошибок диапазона/точности NUMERIC-полей (message-код в details.fields[]; текст
// подбирает клиент/SSR) — общие для всех фич, использующих Amount (ADR-101).
const (
	MsgInvalidFormat = "invalid_format" // не число вовсе: мусор, экспонента, NaN/Inf
	MsgInvalid       = "invalid"        // отрицательное значение там, где не допускается
	MsgTooPrecise    = "too_precise"    // больше знаков после запятой, чем допускает scale колонки
	MsgTooLarge      = "too_large"      // целая часть длиннее, чем допускает precision-scale колонки
)

// Scale — предел целой/дробной части для конкретной NUMERIC(precision,scale) колонки.
// IntDigits = precision - scale — сколько цифр допустимо ДО точки, иначе PG вернёт 22003
// numeric_value_out_of_range на INSERT/UPDATE.
type Scale struct {
	IntDigits  int
	FracDigits int
}

// decimalRe — целое/десятичное число, опциональный минус, без ведущих нулей (кроме самого
// "0"), без экспоненты — исключает мусор вида "1e400"/"NaN"/"Inf".
var decimalRe = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?$`)

// Validate — decimalRe уже отсекает всё, что не «-?(0|[1-9]\d*)(\.\d+)?» (экспонента,
// NaN/Inf, произвольный мусор — MsgInvalidFormat). Дальше — величина целой части (иначе PG
// молча не округляет, а падает 22003) и точность дробной части (PG НЕ отклоняет лишние
// знаки — она их МОЛЧА обрезает при записи). Пустая строка — валидно.
func (a Amount) Validate(scale Scale, allowNegative bool) string {
	val := string(a)
	if !decimalRe.MatchString(val) {
		return MsgInvalidFormat
	}
	digits := val
	if strings.HasPrefix(val, "-") {
		if !allowNegative {
			return MsgInvalid
		}
		digits = val[1:]
	}
	intPart, fracPart, _ := strings.Cut(digits, ".")
	if len(fracPart) > scale.FracDigits {
		return MsgTooPrecise
	}
	if len(intPart) > scale.IntDigits {
		return MsgTooLarge
	}
	return ""
}
