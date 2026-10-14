package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// SubtaskProductRepo — товары услуги (design-sql.md §Subtask Parent/Lock/Subtask Product
// Queries). Без pool: мутации всегда под LockTask+Lock Subtask (Lock Order).
type SubtaskProductRepo struct{}

func NewSubtaskProductRepo() *SubtaskProductRepo { return &SubtaskProductRepo{} }

// ParentOf — design-sql.md §Subtask Parent Query (второй запрос): task_id + subtask_id
// владельца строки товара. 0 → errNotFound (SUBTASK_PRODUCT_NOT_FOUND).
func (r *SubtaskProductRepo) ParentOf(ctx context.Context, tx pgx.Tx, id string) (taskID int64, subtaskID string, err error) {
	err = tx.QueryRow(ctx, `
		SELECT s.task_id, sp.subtask_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
		WHERE sp.id = $1`, id).Scan(&taskID, &subtaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", errNotFound
	}
	return taskID, subtaskID, err
}

// Lock — design-sql.md §Lock Subtask Query (второй запрос), ПОСЛЕ SubtaskRepo.Lock
// (Lock Order). 0 → errNotFound.
func (r *SubtaskProductRepo) Lock(ctx context.Context, tx pgx.Tx, id, subtaskID string) error {
	var got string
	err := tx.QueryRow(ctx, `SELECT id FROM subtask_products WHERE id = $1 AND subtask_id = $2 FOR UPDATE`, id, subtaskID).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	return err
}

// ProductState — существование + is_active товара (400 различает «не найден»/«в архиве»).
// Мягко удалённый товар — «не найден» (soft-delete-catalog DS-014). Строка товара берётся FOR NO
// KEY UPDATE до конца транзакции — тем же режимом, что затем у Reconcile (без повышения
// блокировки, которое дало бы взаимную блокировку двух заказов с одним товаром): удаление
// товара либо уже видно, либо ждёт.
func (r *SubtaskProductRepo) ProductState(ctx context.Context, tx pgx.Tx, id string) (exists, active bool, err error) {
	err = tx.QueryRow(ctx, `SELECT true, is_active FROM products WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE`, id).Scan(&exists, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return exists, active, err
}

// Insert — design-sql.md §Subtask Product Queries: цена копируется из ТЕКУЩЕЙ
// products.price (AC-008). 0 строк (товар не найден/архивен — гонка) → errNotFound.
func (r *SubtaskProductRepo) Insert(ctx context.Context, tx pgx.Tx, subtaskID, productID, qty string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO subtask_products (subtask_id, product_id, qty, price, position)
		SELECT $1, p.id, $3::numeric, p.price, coalesce((SELECT max(position) FROM subtask_products WHERE subtask_id = $1), 0) + 1
		FROM products p WHERE p.id = $2 AND p.is_active AND p.deleted_at IS NULL
		RETURNING id`, subtaskID, productID, qty).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return id, err
}

// Update — LWW (D28): полная замена qty/price. price=nil → NULL («без цены», итог 0).
func (r *SubtaskProductRepo) Update(ctx context.Context, tx pgx.Tx, id, qty string, price *string) error {
	tag, err := tx.Exec(ctx, `UPDATE subtask_products SET qty = $2::numeric, price = $3::numeric WHERE id = $1`,
		id, qty, price)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// Delete — 0 строк → errNotFound (идемпотентность DELETE — повтор тоже 404, LV-173).
func (r *SubtaskProductRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM subtask_products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}
