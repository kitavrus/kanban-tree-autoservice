package sqlutil

import "testing"

func TestHasInvalidText(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want bool
	}{
		{"пусто", "", false},
		{"обычный текст", "ООО Ромашка", false},
		{"NUL-байт", "abc\u0000def", true},
		{"только NUL", "\u0000", true},
		{"невалидный UTF-8", string([]byte{0xff, 0xfe}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasInvalidText(tc.s); got != tc.want {
				t.Errorf("HasInvalidText(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}
