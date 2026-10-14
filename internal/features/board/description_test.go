package board

import "testing"

// TestDescriptionToHTML_Escaping — UT-001/AC-002: символы <>&"' экранируются
// html.EscapeString, что исключает разметку/атрибуты, введённые пользователем.
func TestDescriptionToHTML_Escaping(t *testing.T) {
	got := descriptionToHTML(`<script>&"'</script>`)
	want := `<p>&lt;script&gt;&amp;&#34;&#39;&lt;/script&gt;</p>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestDescriptionToHTML_Paragraphs — UT-001: пустая строка разделяет абзацы (<p>),
// одиночный \n внутри абзаца — <br>.
func TestDescriptionToHTML_Paragraphs(t *testing.T) {
	got := descriptionToHTML("Первый абзац\nвторая строка\n\nВторой абзац")
	want := "<p>Первый абзац<br>вторая строка</p><p>Второй абзац</p>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// три и более переводов строки подряд — не порождают пустых <p></p>.
	got = descriptionToHTML("A\n\n\n\nB")
	want = "<p>A</p><p>B</p>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestDescriptionToHTML_Empty — UT-001: "" (и строка из одних пробелов) → "".
func TestDescriptionToHTML_Empty(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\n  \n"} {
		if got := descriptionToHTML(in); got != "" {
			t.Fatalf("descriptionToHTML(%q) = %q, want \"\"", in, got)
		}
	}
}
