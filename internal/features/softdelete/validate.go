package softdelete

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

const (
	defaultLimit = 50
	maxLimit     = 200
	qMax         = 100
)

// parseQuery — валидация параметров списка (§6.3) → вид и фильтр. tz — часовой пояс
// пользователя: день периода — с 00:00 до 24:00 в этом поясе (DS-025).
func parseQuery(q ListQuery, tz string) (Kind, Filter, []errorspkg.Field) {
	var fields []errorspkg.Field
	add := func(name, msg string) { fields = append(fields, errorspkg.Field{Name: name, Message: msg}) }
	f := Filter{Limit: defaultLimit}

	kind := strings.TrimSpace(q.Kind)
	switch {
	case kind == "":
		add("kind", msgRequired)
	case !IsKind(kind):
		add("kind", msgInvalid)
	}

	search := strings.TrimSpace(q.Q)
	switch {
	case sqlutil.HasInvalidText(search):
		add("q", msgInvalid)
	case utf8.RuneCountInString(search) > qMax:
		add("q", msgTooLong)
	case search != "":
		f.Q = sqlutil.EscapeLike(search)
		if n, err := strconv.ParseInt(strings.TrimPrefix(search, "#"), 10, 64); err == nil && n > 0 {
			f.QNumber = &n
		}
	}

	fromOK := true
	if v := strings.TrimSpace(q.DeletedFrom); v != "" {
		from, ok := timezone.DayStart(tz, v)
		if fromOK = ok; ok {
			f.DeletedFrom = &from
		} else {
			add("deleted_from", msgInvalid)
		}
	}
	if v := strings.TrimSpace(q.DeletedTo); v != "" {
		to, ok := timezone.DayStart(tz, v)
		switch {
		case !ok:
			add("deleted_to", msgInvalid)
		case fromOK && f.DeletedFrom != nil && to.Before(*f.DeletedFrom):
			add("deleted_to", msgInvalid)
		default:
			end := timezone.NextDay(to)
			f.DeletedTo = &end
		}
	}

	if v := strings.TrimSpace(q.DeletedBy); v != "" {
		// Только каноническая 36-символьная форма: прочие варианты ::uuid в PG не примет.
		if len(v) != 36 || uuid.Validate(v) != nil {
			add("deleted_by", msgInvalid)
		} else {
			f.DeletedBy = strings.ToLower(v)
		}
	}

	if v := strings.TrimSpace(q.Limit); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxLimit {
			add("limit", msgInvalid)
		} else {
			f.Limit = n
		}
	}
	if v := strings.TrimSpace(q.Offset); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			add("offset", msgInvalid)
		} else {
			f.Offset = n
		}
	}
	return Kind(kind), f, fields
}
