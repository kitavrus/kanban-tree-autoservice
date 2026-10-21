package suppliers

import (
	"context"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// CheckNoOpenPurchases — у поставщика нет незакрытых закупок (draft/sent/ready, не удалённых):
// иначе 409 SUPPLIER_HAS_OPEN_PURCHASES с номерами (purchase-orders §6.18, DS-043). Удалённые
// черновики не мешают.
func (r *Repo) CheckNoOpenPurchases(ctx context.Context, id string) error {
	rows, err := r.pool.Query(ctx, `
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
