package executorpayouts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// payoutScale — NUMERIC(12,2) колонки subtasks.executor_payout: те же границы, что у
// executor_payout в PUT /subtasks/:id (board).
var payoutScale = numeric.Scale{IntDigits: 10, FracDigits: 2}

// Коды ошибок полей (details.fields[].message) — клиент переводит их в текст.
const (
	msgRequired  = "required"
	msgInvalid   = "invalid"
	msgTooMany   = "too_many"
	msgDuplicate = "duplicate"
	msgNotFound  = "not_found"
	// msgNoAssignee — отметить выплату можно только у услуги с исполнителем (D65).
	msgNoAssignee = "no_assignee"
)

// Значения query-фильтров (те же слова, что у фильтров доски: paid=all|paid|unpaid,
// assignee=me|all|<uuid>).
const (
	StatusAll    = "all"
	StatusPaid   = "paid"
	StatusUnpaid = "unpaid"

	AssigneeMe  = "me"
	AssigneeAll = "all"
)

var Statuses = []string{StatusAll, StatusPaid, StatusUnpaid}

const (
	defaultLimit = 200
	maxLimit     = 1000
	// markMax — верхняя граница subtask_ids одного запроса массовой отметки.
	markMax = 500
)

// Actor — вызывающий пользователь: нужен для дефолта фильтра assignee по роли (D69).
type Actor struct {
	ID      string
	IsAdmin bool
}

// ListQuery — сырые query-параметры GET /executor-payouts и /executor-payouts/summary.
type ListQuery struct {
	Assignee, Status, OnlyDone, Limit, Offset string
}

// Filter — разобранные фильтры. AssigneeID nil — все исполнители; Paid nil — любой статус
// выплаты.
type Filter struct {
	AssigneeID *string
	Paid       *bool
	OnlyDone   bool
	Limit      int
	Offset     int
}

// parseFilter — разбор и валидация query (мусор → 400 по полю, не тихий clamp). assignee
// отсутствует → executor видит «мои», admin — все (D69, аналог D19 доски).
func parseFilter(q ListQuery, a Actor) (Filter, error) {
	var f Filter
	var fields []errorspkg.Field
	add := func(name string) { fields = append(fields, errorspkg.Field{Name: name, Message: msgInvalid}) }

	assignee := strings.TrimSpace(q.Assignee)
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

	switch q.Status {
	case "", StatusAll:
		f.Paid = nil
	case StatusPaid:
		v := true
		f.Paid = &v
	case StatusUnpaid:
		v := false
		f.Paid = &v
	default:
		add("status")
	}

	switch q.OnlyDone {
	case "", "false":
		f.OnlyDone = false
	case "true":
		f.OnlyDone = true
	default:
		add("only_done")
	}

	f.Limit = defaultLimit
	if q.Limit != "" {
		n, err := strconv.Atoi(q.Limit)
		if err != nil || n < 1 || n > maxLimit {
			add("limit")
		} else {
			f.Limit = n
		}
	}
	if q.Offset != "" {
		n, err := strconv.Atoi(q.Offset)
		if err != nil || n < 0 {
			add("offset")
		} else {
			f.Offset = n
		}
	}

	if len(fields) > 0 {
		return Filter{}, errorspkg.Validation(fields...)
	}
	return f, nil
}

// normalize — trim + канонический вид uuid (нижний регистр, с дефисами): Postgres отдаёт
// uuid именно так, а сверка найденных строк с запрошенными и поиск дублей идут по строке.
// Не-uuid остаётся как есть — его отсеет validate().
func (in MarkRequest) normalize() MarkRequest {
	ids := make([]string, len(in.SubtaskIDs))
	for i, id := range in.SubtaskIDs {
		id = strings.TrimSpace(id)
		if u, err := uuid.Parse(id); err == nil {
			id = u.String()
		}
		ids[i] = id
	}
	in.SubtaskIDs = ids
	return in
}

// validate — формат тела после normalize(); существование услуг и наличие исполнителя —
// под транзакцией (repo.Mark).
func (in MarkRequest) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	switch {
	case len(in.SubtaskIDs) == 0:
		add("subtask_ids", msgRequired)
	case len(in.SubtaskIDs) > markMax:
		add("subtask_ids", msgTooMany)
	default:
		seen := make(map[string]bool, len(in.SubtaskIDs))
		for i, id := range in.SubtaskIDs {
			name := fmt.Sprintf("subtask_ids[%d]", i)
			switch {
			case id == "":
				add(name, msgRequired)
			case uuid.Validate(id) != nil:
				add(name, msgInvalid)
			case seen[id]:
				add(name, msgDuplicate)
			}
			seen[id] = true
		}
	}
	if in.IsExecutorPaid == nil {
		add("is_executor_paid", msgRequired)
	}
	return f
}

// validate — executor_payout обязателен (поля нет → required); null допустим (очистить);
// число — NUMERIC(12,2) ≥ 0 (коды pkg/numeric: invalid_format / too_precise / too_large / …).
func (in PayoutUpdateRequest) validate() []errorspkg.Field {
	if !in.ExecutorPayout.Set {
		return []errorspkg.Field{{Name: "executor_payout", Message: msgRequired}}
	}
	if in.ExecutorPayout.Value != nil {
		if code := in.ExecutorPayout.Value.Validate(payoutScale, false); code != "" {
			return []errorspkg.Field{{Name: "executor_payout", Message: code}}
		}
	}
	return nil
}
