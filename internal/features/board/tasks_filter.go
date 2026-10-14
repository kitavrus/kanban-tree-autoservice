package board

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

const (
	tasksDefaultLimit = 500
	tasksMaxLimit     = 1000
	taskFilterQMax    = 100
	// tasksPerColumnMax — верхняя граница окна per_column (pagination DS-002: 1..1000).
	tasksPerColumnMax = 1000
)

// taskNoRe — design-sql.md §Board List Query: `q` трактуется как номер заказа `#N`/`N`.
var taskNoRe = regexp.MustCompile(`^#?\d{1,18}$`)

// TaskFilter — параметры GET /api/v1/tasks после разбора и валидации query (design-go-
// layers.md §DTO TaskFilter).
type TaskFilter struct {
	ClientID *string
	Q        string
	// QDesc — тот же поиск для tasks.description_html: описание хранится экранированным
	// (ADR-006), поэтому искомая строка экранируется так же (html.EscapeString) до LIKE.
	QDesc      string
	TaskNo     *int64
	CategoryID *string
	Paid       *bool
	AssigneeID *string
	Due        string
	// Today — «сегодня» (YYYY-MM-DD) в часовом поясе пользователя (user-timezone DS-032):
	// от него считаются soon/overdue фильтра Due и subtasks_late. Берётся из Actor.Today;
	// пусто — current_date сервера БД.
	Today string
	// ExecutorPaid — фильтр «Выплата исполнителю» (D67): "" — все; "unpaid" — у заказа есть
	// услуга с исполнителем без отметки выплаты; "paid" — услуги с исполнителем есть и все
	// они выплачены. Заказ без услуг с исполнителем не попадает ни в paid, ни в unpaid.
	ExecutorPaid string
	// ColumnID — фильтр по колонке (pagination DS-002, догрузка «Показать ещё» DS-001).
	ColumnID *string
	// PerColumn — окно «не более N первых (t.position, t.id) карточек на колонку»
	// (pagination DS-002); nil — без окна. limit/offset применяются ПОСЛЕ окна.
	PerColumn *int
	Limit     int
	Offset    int
}

// ParseTaskFilter — разбор и валидация query GET /tasks (design-sql.md §Board List Query,
// design-go-layers.md §DTO TaskFilter). Actor нужен для дефолта assignee по роли (AC-014).
func ParseTaskFilter(c fiber.Ctx, a Actor) (TaskFilter, error) {
	var f TaskFilter
	var fields []errorspkg.Field
	add := func(name string) { fields = append(fields, errorspkg.Field{Name: name, Message: msgInvalid}) }

	if raw := strings.TrimSpace(c.Query("client_id")); raw != "" {
		if uuid.Validate(raw) != nil {
			add("client_id")
		} else {
			f.ClientID = &raw
		}
	}

	q := strings.TrimSpace(c.Query("q"))
	if sqlutil.HasInvalidText(q) {
		q = "" // NUL-байт/невалидный UTF-8 — фильтр молча игнорируется (TD-4), не 400/500.
	}
	if utf8.RuneCountInString(q) > taskFilterQMax {
		r := []rune(q)
		q = string(r[:taskFilterQMax])
	}
	if taskNoRe.MatchString(q) {
		digits := strings.TrimPrefix(q, "#")
		if n, err := strconv.ParseInt(digits, 10, 64); err == nil {
			f.TaskNo = &n
		}
	}
	if q != "" {
		f.Q = sqlutil.EscapeLike(q)
		f.QDesc = sqlutil.EscapeLike(html.EscapeString(q))
	}

	if raw := strings.TrimSpace(c.Query("category_id")); raw != "" {
		if uuid.Validate(raw) != nil {
			add("category_id")
		} else {
			f.CategoryID = &raw
		}
	}

	switch p := c.Query("paid", PaidFilterAll); p {
	case PaidFilterAll:
		f.Paid = nil
	case PaidFilterPaid:
		v := true
		f.Paid = &v
	case PaidFilterUnpaid:
		v := false
		f.Paid = &v
	default:
		add("paid")
	}

	switch p := c.Query("executor_paid", PaidFilterAll); p {
	case PaidFilterAll:
		f.ExecutorPaid = ""
	case PaidFilterPaid, PaidFilterUnpaid:
		f.ExecutorPaid = p
	default:
		add("executor_paid")
	}

	assignee := strings.TrimSpace(c.Query("assignee"))
	switch {
	case assignee == "":
		if !a.IsAdmin {
			id := a.ID
			f.AssigneeID = &id
		}
	case assignee == AssigneeMe:
		id := a.ID
		f.AssigneeID = &id
	case assignee == AssigneeAll:
		f.AssigneeID = nil
	case uuid.Validate(assignee) == nil:
		f.AssigneeID = &assignee
	default:
		add("assignee")
	}

	f.Today = a.Today
	switch d := c.Query("due", DueFilterAll); d {
	case DueFilterAll:
		f.Due = ""
	case DueFilterSoon, DueFilterOverdue:
		f.Due = d
	default:
		add("due")
	}

	// pagination DS-002: column_id/per_column — аддитивные опциональные параметры.
	if raw := strings.TrimSpace(c.Query("column_id")); raw != "" {
		if uuid.Validate(raw) != nil {
			add("column_id")
		} else {
			f.ColumnID = &raw
		}
	}
	if raw := c.Query("per_column"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > tasksPerColumnMax {
			add("per_column")
		} else {
			f.PerColumn = &n
		}
	}

	f.Limit = tasksDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > tasksMaxLimit {
			add("limit")
		} else {
			f.Limit = n
		}
	}
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			add("offset")
		} else {
			f.Offset = n
		}
	}

	if len(fields) > 0 {
		return TaskFilter{}, errorspkg.Validation(fields...)
	}
	return f, nil
}
