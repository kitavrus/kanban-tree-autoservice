package sqlutil

import "testing"

func TestPlatePattern(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"  - ":       "",
		"а123вс777":  "%А123ВС777%",
		"А123ВС 777": "%А123ВС777%",
		" a-12 3b ":  "%A123B%",
		"x%_":        `%X\%\_%`,
		"\tв 77 ":    "%В77%",
	}
	for in, want := range cases {
		if got := PlatePattern(in); got != want {
			t.Errorf("PlatePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlateCompactSQL(t *testing.T) {
	want := "replace(replace(e.plate_number, ' ', ''), '-', '')"
	if got := PlateCompactSQL("e.plate_number"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
