package board

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// msgNotFound/msgInvalidDate — коды полей заказов; msgRequired/msgTooLong/msgInvalid/
// msgInvalidText — общие (columns_validate.go).
const (
	msgNotFound    = "not_found"
	msgInvalidDate = "invalid_date"
)

// taskTitleMax — design-errors.md §Validation Fields (`title`, trim 1..200).
const taskTitleMax = 200

// taskDescriptionMax — design-errors.md §Validation Fields (`description`, ≤10000
// символов; NULL — PUT «не менять», POST «пусто» — длина не проверяется).
const taskDescriptionMax = 10000

// dateLayout — YYYY-MM-DD (design-errors.md §Validation Fields `due_date`).
const dateLayout = "2006-01-02"

// normalize — trim текстовых полей; nil-указатели (assignee_id/due_date) остаются nil,
// пустая строка внутри указателя не трактуется как отсутствие значения здесь — это делает
// nilIfEmpty() в сервисном слое перед записью в БД.
func (in TaskCreateRequest) normalize() TaskCreateRequest {
	in.Title = strings.TrimSpace(in.Title)
	in.ClientID = strings.TrimSpace(in.ClientID)
	in.ColumnID = strings.TrimSpace(in.ColumnID)
	if in.AssigneeID != nil {
		v := strings.TrimSpace(*in.AssigneeID)
		in.AssigneeID = &v
	}
	if in.DueDate != nil {
		v := strings.TrimSpace(*in.DueDate)
		in.DueDate = &v
	}
	return in
}

// validate — формат полей после normalize(); существование client_id/column_id/
// assignee_id проверяет сервис (design-errors.md: «404 клиент/ответственный — в форме
// ошибка поля»).
func (in TaskCreateRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch n := utf8.RuneCountInString(in.Title); {
	case n == 0:
		add("title", msgRequired)
	case n > taskTitleMax:
		add("title", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Title) {
		add("title", msgInvalidText)
	}

	switch {
	case in.ClientID == "":
		add("client_id", msgRequired)
	case uuid.Validate(in.ClientID) != nil:
		add("client_id", msgInvalid)
	}

	switch {
	case in.ColumnID == "":
		add("column_id", msgRequired)
	case uuid.Validate(in.ColumnID) != nil:
		add("column_id", msgInvalid)
	}

	if in.AssigneeID != nil && *in.AssigneeID != "" && uuid.Validate(*in.AssigneeID) != nil {
		add("assignee_id", msgInvalid)
	}

	if in.DueDate != nil && *in.DueDate != "" {
		if _, err := time.Parse(dateLayout, *in.DueDate); err != nil {
			add("due_date", msgInvalidDate)
		}
	}

	if in.Description != nil && utf8.RuneCountInString(*in.Description) > taskDescriptionMax {
		add("description", msgTooLong)
	}
	return f
}

// normalize/validate TaskUpdateRequest — та же форма плюс обязательный version (≥1).
func (in TaskUpdateRequest) normalize() TaskUpdateRequest {
	in.TaskCreateRequest = in.TaskCreateRequest.normalize()
	return in
}

func (in TaskUpdateRequest) validate() []errorspkg.Field {
	f := in.TaskCreateRequest.validate()
	if in.Version == nil || *in.Version < 1 {
		f = append(f, errorspkg.Field{Name: "version", Message: msgRequired})
	}
	return f
}
