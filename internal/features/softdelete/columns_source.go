package softdelete

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ColumnsSource — удалённые колонки доски (вкладка «Колонки», часть 3
// soft-delete-board-columns). Читает таблицу columns напрямую: softdelete — reader, отметку
// удаления пишет пакет board.
type ColumnsSource struct {
	pool *pgxpool.Pool
}

func NewColumnsSource(pool *pgxpool.Pool) *ColumnsSource { return &ColumnsSource{pool: pool} }

// ColumnSubtitle — подзаголовок строки удалённой колонки: «финальная» либо пустая строка
// (soft-delete-board-columns §6.3).
func ColumnSubtitle(isFinal bool) string {
	if isFinal {
		return "финальная"
	}
	return ""
}

// columnsWhereSQL — фильтр вкладки «Колонки»: $1 — строка поиска (название, причина; уже
// экранирована), $2/$3 — период [с; по), $4 — кто удалил. Номера у колонки нет — номер из
// строки поиска не используется. Частичный индекс columns_deleted_at_idx покрывает условие и
// сортировку.
const columnsWhereSQL = `
FROM columns c
JOIN users u ON u.id = c.deleted_by
WHERE c.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR c.name ILIKE '%' || $1 || '%' ESCAPE '\'
       OR c.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\')
  AND ($2::timestamptz IS NULL OR c.deleted_at >= $2)
  AND ($3::timestamptz IS NULL OR c.deleted_at < $3)
  AND ($4::uuid IS NULL OR c.deleted_by = $4)`

func (s *ColumnsSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	var q, by any
	if f.Q != "" {
		q = f.Q
	}
	if f.DeletedBy != "" {
		by = f.DeletedBy
	}
	args := []any{q, f.DeletedFrom, f.DeletedTo, by}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+columnsWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT c.id::text, c.name, c.is_final, c.deleted_at, u.id::text, coalesce(nullif(u.name, ''), u.login), c.delete_reason`+
		columnsWhereSQL+`
ORDER BY c.deleted_at DESC, c.id
LIMIT $5 OFFSET $6`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		var isFinal bool
		it := Item{Kind: string(KindColumns)}
		if err := rows.Scan(&it.ID, &it.Title, &isFinal, &it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.Subtitle = ColumnSubtitle(isFinal)
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *ColumnsSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM columns WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}
