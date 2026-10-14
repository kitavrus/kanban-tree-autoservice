package board

import (
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// subtaskLogTextMax — design-errors.md §Validation Fields (`text`, trim 1..4000).
const subtaskLogTextMax = 4000

func (in SubtaskLogRequest) normalize() SubtaskLogRequest {
	in.Text = strings.TrimSpace(in.Text)
	return in
}

func (in SubtaskLogRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	switch n := utf8.RuneCountInString(in.Text); {
	case n == 0:
		f = append(f, errorspkg.Field{Name: "text", Message: msgRequired})
	case n > subtaskLogTextMax:
		f = append(f, errorspkg.Field{Name: "text", Message: msgTooLong})
	}
	return f
}
