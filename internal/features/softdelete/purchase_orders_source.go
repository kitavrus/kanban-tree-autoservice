package softdelete

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PurchaseOrdersSource — вкладка «Закупки» (purchase-orders DS-037, DS-072): мягко удалённые
// черновики. Читает purchase_orders напрямую; отметку удаления пишет пакет purchase-orders.
// Title — номер «З-0012», Subtitle — получатель (поставщик либо «Базар (сами)»).
type PurchaseOrdersSource struct {
	pool *pgxpool.Pool
}

func NewPurchaseOrdersSource(pool *pgxpool.Pool) *PurchaseOrdersSource {
	return &PurchaseOrdersSource{pool: pool}
}

// purchaseOrdersWhereSQL — $1 поиск (получатель, причина; экранирован), $2 номер из строки
// поиска, $3/$4 период [с; по), $5 кто удалил. Индекс purchase_orders_deleted_at_idx.
const purchaseOrdersWhereSQL = `
FROM purchase_orders r
LEFT JOIN suppliers s ON s.id = r.supplier_id
JOIN users u ON u.id = r.deleted_by
WHERE r.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR coalesce(s.name, 'Базар') ILIKE '%' || $1 || '%' ESCAPE '\'
       OR r.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\'
       OR r.number = $2::bigint)
  AND ($3::timestamptz IS NULL OR r.deleted_at >= $3)
  AND ($4::timestamptz IS NULL OR r.deleted_at < $4)
  AND ($5::uuid IS NULL OR r.deleted_by = $5)`

func (s *PurchaseOrdersSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	base := filterArgs(f)
	args := []any{base[0], f.QNumber, base[1], base[2], base[3]}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+purchaseOrdersWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT r.id::text, r.number, coalesce(s.name, 'Базар (сами)'), r.deleted_at, u.id::text,
       coalesce(nullif(u.name, ''), u.login), r.delete_reason`+purchaseOrdersWhereSQL+`
ORDER BY r.deleted_at DESC, r.id
LIMIT $6 OFFSET $7`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it := Item{Kind: string(KindPurchaseOrders)}
		var number int64
		if err := rows.Scan(&it.ID, &number, &it.Subtitle, &it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.Title = fmt.Sprintf("З-%04d", number)
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *PurchaseOrdersSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM purchase_orders WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}
