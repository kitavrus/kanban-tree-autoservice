package board

import (
	"html"
	"strings"
)

// descriptionToHTML — ADR-006: единственное место, где описание заказа превращается в
// HTML для хранения (tasks.description_html) и последующего вывода как есть (клиент берёт
// текст через DOMParser, не заново экранирует). Абзац — группа строк, отделённая от
// соседних пустой строкой, каждый абзац оборачивается в <p>...</p>; одиночный перевод
// строки внутри абзаца — <br>. Экранирование — html.EscapeString (единственная защита от
// XSS: описание заказа — произвольный пользовательский текст).
func descriptionToHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	var b strings.Builder
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		lines := strings.Split(para, "\n")
		for i, l := range lines {
			lines[i] = html.EscapeString(l)
		}
		b.WriteString("<p>")
		b.WriteString(strings.Join(lines, "<br>"))
		b.WriteString("</p>")
	}
	return b.String()
}
