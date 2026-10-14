package softdelete

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TransfersSource — удалённые документы перемещения (вкладка «Перемещения», часть 2
// soft-delete-transfers). Читает таблицу transfers напрямую: softdelete — reader, отметку
// удаления пишет пакет transfers.
type TransfersSource struct {
	pool *pgxpool.Pool
}

func NewTransfersSource(pool *pgxpool.Pool) *TransfersSource { return &TransfersSource{pool: pool} }

// transferDirectionLabels / transferStatusLabels — подписи направления и статуса документа в
// подзаголовке строки («Склад → склад · Заявка»): те же, что на экране «Перемещения»
// (web/ssr/_shared/js/transfers.js). Удалён может быть только документ в «Заявка» / «Отменён».
var (
	transferDirectionLabels = map[string]string{
		"client_to_warehouse":    "Приём от клиента",
		"warehouse_to_client":    "Возврат клиенту",
		"warehouse_to_warehouse": "Склад → склад",
		"supplier_to_warehouse":  "Приход от поставщика",
	}
	transferStatusLabels = map[string]string{
		"requested": "Заявка",
		"cancelled": "Отменён",
	}
)

// TransferSubtitle — подзаголовок строки удалённого документа: «направление · статус»;
// неизвестное значение отдаётся как есть.
func TransferSubtitle(direction, status string) string {
	d, ok := transferDirectionLabels[direction]
	if !ok {
		d = direction
	}
	s, ok := transferStatusLabels[status]
	if !ok {
		s = status
	}
	return d + " · " + s
}

// transfersWhereSQL — фильтр вкладки «Перемещения»: $1 — строка поиска (причина; уже
// экранирована), $2 — номер документа из строки поиска, $3/$4 — период [с; по), $5 — кто
// удалил. Частичный индекс transfers_deleted_at_idx покрывает условие и сортировку.
const transfersWhereSQL = `
FROM transfers t
JOIN users u ON u.id = t.deleted_by
WHERE t.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR t.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\'
       OR t.id = $2::bigint)
  AND ($3::timestamptz IS NULL OR t.deleted_at >= $3)
  AND ($4::timestamptz IS NULL OR t.deleted_at < $4)
  AND ($5::uuid IS NULL OR t.deleted_by = $5)`

func (s *TransfersSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	var q, by any
	if f.Q != "" {
		q = f.Q
	}
	if f.DeletedBy != "" {
		by = f.DeletedBy
	}
	args := []any{q, f.QNumber, f.DeletedFrom, f.DeletedTo, by}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+transfersWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT t.id, t.direction, t.status, t.deleted_at, u.id::text, coalesce(nullif(u.name, ''), u.login), t.delete_reason`+
		transfersWhereSQL+`
ORDER BY t.deleted_at DESC, t.id
LIMIT $6 OFFSET $7`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		var id int64
		var direction, status string
		it := Item{Kind: string(KindTransfers)}
		if err := rows.Scan(&id, &direction, &status, &it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.ID = strconv.FormatInt(id, 10)
		it.Title = "Перемещение #" + it.ID
		it.Subtitle = TransferSubtitle(direction, status)
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *TransfersSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM transfers WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}
