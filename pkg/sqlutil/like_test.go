package sqlutil

import "testing"

func TestEscapeLike_SpecialChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"percent", "50%", `50\%`},
		{"underscore", "a_b", `a\_b`},
		{"backslash", `a\b`, `a\\b`},
		{"combined", `100%_off\`, `100\%\_off\\`},
		{"plain", "abc", "abc"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EscapeLike(c.in); got != c.want {
				t.Errorf("EscapeLike(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
