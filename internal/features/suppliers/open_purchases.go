package suppliers

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Инвариант DS-043 (purchase-orders §6.18): у архивного/удалённого поставщика нет незакрытых
// закупок. Архив и удаление идут в ОДНОЙ транзакции: строка поставщика блокируется FOR UPDATE,
// затем проверяются закупки. Создание, восстановление и отправка закупки берут строку поставщика
// FOR SHARE и проверяют «активен и не удалён» — гонка «создать закупку ↔ архивировать» исключена:
// кто первый взял блокировку, тот и прав, второй видит результат первого.

// inTx — fn в транзакции; ошибка fn — откат.
func (r *Repo) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lockNoOpenPurchases — блокирует строку поставщика (FOR UPDATE) и проверяет, что у него нет
// незакрытых закупок (draft/sent/ready, не удалённых): иначе 409 SUPPLIER_HAS_OPEN_PURCHASES с
// номерами. Поставщика нет — без ошибки (следующий UPDATE вернёт «не найден»).
func lockNoOpenPurchases(ctx context.Context, tx pgx.Tx, id string) error {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM suppliers WHERE id = $1::uuid FOR UPDATE`, id); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
SELECT number FROM purchase_orders
WHERE supplier_id = $1::uuid AND deleted_at IS NULL AND status = ANY($2::text[])
ORDER BY number`, id, constants.OpenStatuses)
	if err != nil {
		return err
	}
	defer rows.Close()
	var numbers []string
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return err
		}
		numbers = append(numbers, constants.NumberDisplay(n))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(numbers) > 0 {
		return errorspkg.SupplierHasOpenPurchases(numbers)
	}
	return nil
}
