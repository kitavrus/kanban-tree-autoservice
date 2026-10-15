package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// SubtaskRepo — услуги заказа (design-sql.md §Subtask Parent/Lock/Insert/Update/Delete
// Query). Без pool: все методы принимают чужую tx — мутации всегда под LockTask
// (Lock Order ADR-104/ADR-100: columns→tasks→subtasks→subtask_products→products).
type SubtaskRepo struct{}

func NewSubtaskRepo() *SubtaskRepo { return &SubtaskRepo{} }

// SubtaskState — строка services под FOR UPDATE (design-sql.md §Lock Subtask Query).
type SubtaskState struct {
	ID         string
	Status     string
	AssigneeID *string
	// IsExecutorPaid — текущая отметка выплаты исполнителю (D66: защита смены исполнителя).
	IsExecutorPaid bool
}

// TaskIDOf — design-sql.md §Subtask Parent Query (первый запрос): владелец-заказ услуги.
// 0 строк → errNotFound (ErrSubtaskNotFound).
func (r *SubtaskRepo) TaskIDOf(ctx context.Context, tx pgx.Tx, id string) (int64, error) {
	var taskID int64
	err := tx.QueryRow(ctx, `SELECT task_id FROM subtasks WHERE id = $1`, id).Scan(&taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errNotFound
	}
	return taskID, err
}

// Lock — design-sql.md §Lock Subtask Query (первый запрос): FOR UPDATE ПОСЛЕ LockTask
// (Lock Order). 0 строк (удалена/перенесена параллельно) → errNotFound.
func (r *SubtaskRepo) Lock(ctx context.Context, tx pgx.Tx, id string, taskID int64) (*SubtaskState, error) {
	var st SubtaskState
	err := tx.QueryRow(ctx, `SELECT id, status, assignee_id::text, is_executor_paid FROM subtasks WHERE id = $1 AND task_id = $2 FOR UPDATE`,
		id, taskID).Scan(&st.ID, &st.Status, &st.AssigneeID, &st.IsExecutorPaid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	return &st, err
}

// ServiceState — существование + is_active услуги (400 различает «не найдена»/«в архиве»,
// design-errors.md §Validation Fields). Мягко удалённая услуга каталога — «не найдена»
// (soft-delete-catalog DS-014). Строка услуги берётся FOR SHARE до конца транзакции: удаление
// услуги (UPDATE строки) либо уже видно, либо ждёт — новая услуга заказа не сошлётся на
// услугу, удалённую «между проверкой и вставкой».
func (r *SubtaskRepo) ServiceState(ctx context.Context, tx pgx.Tx, id string) (exists, active bool, err error) {
	err = tx.QueryRow(ctx, `SELECT true, is_active FROM services WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, id).Scan(&exists, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return exists, active, err
}

// Insert — design-sql.md §Insert Subtask Query (первая половина): услуга + позиция в
// конец. 0 строк (услуга не найдена/архивна — гонка после ServiceState) → errNotFound.
func (r *SubtaskRepo) Insert(ctx context.Context, tx pgx.Tx, taskID int64, serviceID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO subtasks (task_id, service_id, price, position)
		SELECT $1, sv.id, sv.price, coalesce((SELECT max(position) FROM subtasks WHERE task_id = $1), 0) + 1
		FROM services sv WHERE sv.id = $2 AND sv.is_active AND sv.deleted_at IS NULL
		RETURNING id`, taskID, serviceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return id, err
}

// CopyKit — design-sql.md §Insert Subtask Query (вторая половина): типовой комплект
// товаров услуги, цена — ТЕКУЩАЯ products.price на момент добавления (AC-008), не
// service_products (там своей цены нет).
// Товары комплекта, мягко удалённые из каталога, в заказ не подставляются (soft-delete-catalog §6.9, DS-014);
// действующие товары комплекта сначала берутся FOR NO KEY UPDATE по возрастанию id (тем же
// режимом, что затем у Reconcile) — удаление товара либо уже видно, либо ждёт.
func (r *SubtaskRepo) CopyKit(ctx context.Context, tx pgx.Tx, subtaskID, serviceID string) error {
	if _, err := tx.Exec(ctx, `
		SELECT p.id FROM service_products spr JOIN products p ON p.id = spr.product_id
		WHERE spr.service_id = $1 AND p.deleted_at IS NULL ORDER BY p.id FOR NO KEY UPDATE OF p`, serviceID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO subtask_products (subtask_id, product_id, qty, price, position)
		SELECT $1, spr.product_id, spr.qty, p.price, row_number() OVER (ORDER BY spr.position, spr.product_id)
		FROM service_products spr JOIN products p ON p.id = spr.product_id
		WHERE spr.service_id = $2 AND p.deleted_at IS NULL`, subtaskID, serviceID)
	return err
}

// subtaskUpdateRow — параметры Update (design-go-layers.md §Repository Interface).
type subtaskUpdateRow struct {
	Status     string
	AssigneeID *string
	DueDate    *string
	Price      *string
	// IsExecutorPaid — итоговое значение флага (сервис подставляет текущее, если клиент
	// поле не прислал). SetExecutorPayout=false — сумму не трогать (поля в запросе нет);
	// true — записать ExecutorPayout (nil → NULL).
	IsExecutorPaid    bool
	SetExecutorPayout bool
	ExecutorPayout    *string
}

// Update — design-sql.md §Update Subtask Query. 0 строк (удалена параллельно) →
// errNotFound. executor_paid_at — как tasks.paid_at: now() при переходе false→true,
// NULL при снятии, иначе не трогается (D63).
func (r *SubtaskRepo) Update(ctx context.Context, tx pgx.Tx, id string, in subtaskUpdateRow) error {
	tag, err := tx.Exec(ctx, `
		UPDATE subtasks SET status = $2, assignee_id = $3::uuid, due_date = $4::date,
		       price = $5::numeric,
		       executor_paid_at = CASE WHEN $6 AND NOT is_executor_paid THEN now() WHEN NOT $6 THEN NULL ELSE executor_paid_at END,
		       is_executor_paid = $6,
		       executor_payout = CASE WHEN $8::boolean THEN $7::numeric ELSE executor_payout END,
		       updated_at = now()
		WHERE id = $1`, id, in.Status, in.AssigneeID, in.DueDate, in.Price,
		in.IsExecutorPaid, in.ExecutorPayout, in.SetExecutorPayout)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// Delete — design-sql.md §Delete Subtask Query: CASCADE subtask_products/subtask_logs
// (FK ON DELETE CASCADE, миграция 0005/0006). 0 строк → errNotFound.
func (r *SubtaskRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM subtasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}
