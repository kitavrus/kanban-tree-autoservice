package softdelete

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TasksSource — удалённые заказы (вкладка «Заказы», часть 1 soft-delete-core). Читает
// таблицу tasks напрямую: softdelete — reader, отметку удаления пишет пакет board.
type TasksSource struct {
	pool *pgxpool.Pool
}

func NewTasksSource(pool *pgxpool.Pool) *TasksSource { return &TasksSource{pool: pool} }

// tasksWhereSQL — фильтр вкладки «Заказы»: $1 — строка поиска (название, причина; уже
// экранирована), $2 — номер заказа из строки поиска, $3/$4 — период [с; по), $5 — кто удалил.
// Частичный индекс tasks_deleted_at_idx покрывает условие и сортировку.
const tasksWhereSQL = `
FROM tasks t
JOIN clients cl ON cl.id = t.client_id
JOIN users u    ON u.id = t.deleted_by
WHERE t.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR t.title ILIKE '%' || $1 || '%' ESCAPE '\'
       OR t.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\'
       OR t.id = $2::bigint)
  AND ($3::timestamptz IS NULL OR t.deleted_at >= $3)
  AND ($4::timestamptz IS NULL OR t.deleted_at < $4)
  AND ($5::uuid IS NULL OR t.deleted_by = $5)`

func (s *TasksSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	var q, by any
	if f.Q != "" {
		q = f.Q
	}
	if f.DeletedBy != "" {
		by = f.DeletedBy
	}
	args := []any{q, f.QNumber, f.DeletedFrom, f.DeletedTo, by}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+tasksWhereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT t.id, t.title, cl.company_name, t.deleted_at, u.id::text, coalesce(nullif(u.name, ''), u.login), t.delete_reason`+
		tasksWhereSQL+`
ORDER BY t.deleted_at DESC, t.id
LIMIT $6 OFFSET $7`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		var id int64
		var title string
		it := Item{Kind: string(KindTasks)}
		if err := rows.Scan(&id, &title, &it.Subtitle, &it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.ID = strconv.FormatInt(id, 10)
		it.Title = "#" + it.ID + " " + title
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *TasksSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}
