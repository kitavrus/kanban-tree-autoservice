package executorpayouts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// fieldsError — ошибки полей, найденные под транзакцией (услуга не найдена / без
// исполнителя); сервис превращает их в 400 VALIDATION.
type fieldsError struct{ Fields []errorspkg.Field }

func (e *fieldsError) Error() string { return "executorpayouts: invalid fields" }

// errNotFound — услуги нет либо она без исполнителя (не элемент отчёта).
var errNotFound = errors.New("executorpayouts: not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// payoutWhereSQL — общий фильтр списка и итогов: только услуги с исполнителем (JOIN users),
// $1 — исполнитель, $2 — статус выплаты, $3 — только выполненные. Услуги мягко удалённых
// заказов не участвуют, включая уже выплаченные (soft-delete-core DS-010, DS-029) — запрос,
// использующий фильтр, обязан соединять tasks под алиасом t.
const payoutWhereSQL = `
WHERE t.deleted_at IS NULL
  AND ($1::uuid IS NULL OR s.assignee_id = $1)
  AND ($2::boolean IS NULL OR s.is_executor_paid = $2)
  AND (NOT $3::boolean OR s.status = 'done')`

// List — услуги с исполнителем под фильтром; total — отдельным count (не зависит от
// limit/offset, в т.ч. на странице за пределами выдачи).
func (r *Repo) List(ctx context.Context, f Filter) ([]PayoutItem, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `
SELECT count(*) FROM subtasks s JOIN users u ON u.id = s.assignee_id JOIN tasks t ON t.id = s.task_id`+payoutWhereSQL,
		f.AssigneeID, f.Paid, f.OnlyDone).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, payoutItemSelectSQL+payoutWhereSQL+`
ORDER BY u.name, u.id, s.task_id, s.position, s.id
LIMIT $4 OFFSET $5`, f.AssigneeID, f.Paid, f.OnlyDone, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []PayoutItem{}
	for rows.Next() {
		it, err := scanPayoutItem(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// payoutItemSelectSQL — строка отчёта: услуга с исполнителем (JOIN users отсекает услуги
// без исполнителя) + заказ/клиент/услуга каталога.
const payoutItemSelectSQL = `
SELECT s.id::text, s.task_id, t.title, cl.company_name, sv.name, s.status, u.id::text, u.name,
       s.price::text, s.executor_payout::text, s.is_executor_paid, s.executor_paid_at
FROM subtasks s
JOIN users u     ON u.id = s.assignee_id
JOIN tasks t     ON t.id = s.task_id
JOIN clients cl  ON cl.id = t.client_id
JOIN services sv ON sv.id = s.service_id`

func scanPayoutItem(row pgx.Row) (PayoutItem, error) {
	var it PayoutItem
	var price, payout sql.NullString
	if err := row.Scan(&it.SubtaskID, &it.TaskID, &it.TaskTitle, &it.ClientName, &it.ServiceName, &it.Status,
		&it.AssigneeID, &it.AssigneeName, &price, &payout, &it.IsExecutorPaid, &it.ExecutorPaidAt); err != nil {
		return it, err
	}
	if price.Valid {
		v := numeric.Amount(price.String)
		it.Price = &v
	}
	if payout.Valid {
		v := numeric.Amount(payout.String)
		it.ExecutorPayout = &v
	}
	return it, nil
}

// UpdatePayout — правка ТОЛЬКО суммы выплаты услуги (D72); payout nil → NULL. Lock Order
// как у Mark и board (tasks → subtasks): заказ услуги FOR UPDATE, затем сама услуга. Услуги
// нет или она без исполнителя (не элемент отчёта) → errNotFound, ничего не меняется.
// Возвращает строку отчёта, прочитанную в той же транзакции после записи.
func (r *Repo) UpdatePayout(ctx context.Context, id string, payout *string) (*PayoutItem, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
SELECT t.id FROM tasks t
WHERE t.id = (SELECT s.task_id FROM subtasks s WHERE s.id = $1::uuid) FOR UPDATE`, id); err != nil {
		return nil, err
	}
	// Услуга мягко удалённого заказа — как несуществующая (soft-delete-core DS-026); условие
	// читается после блокировки заказа — параллельное удаление уже видно.
	var hasAssignee bool
	err = tx.QueryRow(ctx, `
SELECT s.assignee_id IS NOT NULL FROM subtasks s
WHERE s.id = $1::uuid AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = s.task_id AND t.deleted_at IS NULL)
FOR UPDATE OF s`, id).Scan(&hasAssignee)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !hasAssignee) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE subtasks SET executor_payout = $2::numeric, updated_at = now() WHERE id = $1::uuid`, id, payout); err != nil {
		return nil, err
	}
	it, err := scanPayoutItem(tx.QueryRow(ctx, payoutItemSelectSQL+` WHERE s.id = $1::uuid AND t.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &it, nil
}

// Summary — итоги по исполнителям под тем же фильтром. Сумма без заданной выплаты — 0
// (COALESCE, D64); Go десятичной арифметики не делает — считает SQL.
func (r *Repo) Summary(ctx context.Context, f Filter) ([]PayoutSummaryItem, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `
SELECT count(DISTINCT s.assignee_id) FROM subtasks s JOIN users u ON u.id = s.assignee_id JOIN tasks t ON t.id = s.task_id`+payoutWhereSQL,
		f.AssigneeID, f.Paid, f.OnlyDone).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
SELECT u.id::text, u.name,
       (count(*) FILTER (WHERE NOT s.is_executor_paid))::int,
       (count(*) FILTER (WHERE s.is_executor_paid))::int,
       round(coalesce(sum(coalesce(s.executor_payout, 0)) FILTER (WHERE NOT s.is_executor_paid), 0), 2)::text,
       round(coalesce(sum(coalesce(s.executor_payout, 0)) FILTER (WHERE s.is_executor_paid), 0), 2)::text
FROM subtasks s
JOIN users u ON u.id = s.assignee_id
JOIN tasks t ON t.id = s.task_id`+payoutWhereSQL+`
GROUP BY u.id, u.name
ORDER BY u.name, u.id
LIMIT $4 OFFSET $5`, f.AssigneeID, f.Paid, f.OnlyDone, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []PayoutSummaryItem{}
	for rows.Next() {
		var it PayoutSummaryItem
		var toPay, paid string
		if err := rows.Scan(&it.AssigneeID, &it.AssigneeName, &it.UnpaidCount, &it.PaidCount, &toPay, &paid); err != nil {
			return nil, 0, err
		}
		it.ToPay, it.Paid = numeric.Amount(toPay), numeric.Amount(paid)
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// Mark — массовая отметка/снятие выплаты, атомарно (D70): одна транзакция, любая ошибка
// по любой услуге откатывает всё. Lock Order согласован с board (tasks → subtasks):
// сначала заказы затронутых услуг по возрастанию id, затем сами услуги по id.
// Найденные под блокировкой проблемы — *fieldsError по индексам ids: услуги нет →
// not_found (услуга мягко удалённого заказа — тоже, soft-delete-core DS-026); отметка true у услуги без исполнителя → no_assignee (снять отметку можно и без
// исполнителя). executor_paid_at — now() при переходе false→true, NULL при снятии; услуги,
// уже стоящие в нужном состоянии, не трогаются (дата прежней отметки сохраняется).
func (r *Repo) Mark(ctx context.Context, ids []string, paid bool) (int, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
SELECT t.id FROM tasks t
WHERE t.id IN (SELECT s.task_id FROM subtasks s WHERE s.id = ANY($1::text[]::uuid[]))
ORDER BY t.id FOR UPDATE`, ids); err != nil {
		return 0, err
	}

	rows, err := tx.Query(ctx, `
SELECT s.id::text, s.assignee_id IS NOT NULL FROM subtasks s
WHERE s.id = ANY($1::text[]::uuid[])
  AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = s.task_id AND t.deleted_at IS NULL)
ORDER BY s.id FOR UPDATE OF s`, ids)
	if err != nil {
		return 0, err
	}
	hasAssignee := make(map[string]bool, len(ids))
	for rows.Next() {
		var id string
		var ok bool
		if err := rows.Scan(&id, &ok); err != nil {
			rows.Close()
			return 0, err
		}
		hasAssignee[id] = ok
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var fields []errorspkg.Field
	for i, id := range ids {
		assigned, found := hasAssignee[id]
		switch {
		case !found:
			fields = append(fields, errorspkg.Field{Name: fmt.Sprintf("subtask_ids[%d]", i), Message: msgNotFound})
		case paid && !assigned:
			fields = append(fields, errorspkg.Field{Name: fmt.Sprintf("subtask_ids[%d]", i), Message: msgNoAssignee})
		}
	}
	if len(fields) > 0 {
		return 0, &fieldsError{Fields: fields}
	}

	tag, err := tx.Exec(ctx, `
UPDATE subtasks SET is_executor_paid = $2,
       executor_paid_at = CASE WHEN $2 THEN now() ELSE NULL END, updated_at = now()
WHERE id = ANY($1::text[]::uuid[]) AND is_executor_paid <> $2`, ids, paid)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
