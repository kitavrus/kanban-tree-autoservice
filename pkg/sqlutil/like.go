// Package sqlutil — общие мелкие хелперы для построения SQL-запросов,
// переиспользуемые несколькими фичами.
package sqlutil

import "strings"

// EscapeLike экранирует спецсимволы ILIKE/LIKE-паттерна (`%`, `_`, `\`), чтобы
// пользовательский ввод, вставляемый в паттерн подстроки (`"%" + EscapeLike(q) + "%"`),
// искался буквально, а не как шаблон.
func EscapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
