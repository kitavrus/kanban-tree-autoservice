package board

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Column — repo-модель колонки (JOIN подсчёта заказов — по месту вызова).
type Column struct {
	ID            string
	Name          string
	Color         string
	IsFinal       bool
	ConsumesStock bool
	Position      int
	TasksCount    int
	// Deleted — колонка мягко удалена (soft-delete-board-columns DS-019); заполняет только
	// LockAllAny — остальные чтения удалённых колонок не возвращают.
	Deleted bool
}

type ColumnRepo struct {
	pool *pgxpool.Pool
}

func NewColumnRepo(pool *pgxpool.Pool) *ColumnRepo { return &ColumnRepo{pool: pool} }

const columnColumns = `id, name, color, is_final, consumes_stock, position`

// List — без пагинации по данным (колонок — единицы), но конверт списка §API-1 общий
// для всех эндпоинтов; total — реальное число строк (не count(*) OVER(), пагинации нет
// в design-sql.md — набор колонок мал). Только действующие колонки (soft-delete-board-columns
// DS-026): удалённая колонка в список и в total не входит.
func (r *ColumnRepo) List(ctx context.Context, limit, offset int) ([]Column, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM columns WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+columnColumns+`, (SELECT count(*) FROM tasks t WHERE t.column_id = c.id AND t.deleted_at IS NULL) AS tasks_count
		FROM columns c
		WHERE c.deleted_at IS NULL
		ORDER BY position, id
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Column{}
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.IsFinal, &c.ConsumesStock, &c.Position, &c.TasksCount); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// LockAll — вся таблица под FOR UPDATE (ADR-104): держит блокировку колонок на время CRUD/
// смены флагов, чтобы инвариант «≥1 нефинальная колонка» и перенумерация были устойчивы к
// конкурентным запросам. Блокируются ВСЕ строки (и удалённые — их восстановление идёт под той
// же блокировкой), возвращаются только действующие (soft-delete-board-columns DS-019).
func (r *ColumnRepo) LockAll(ctx context.Context, tx pgx.Tx) ([]Column, error) {
	all, err := r.LockAllAny(ctx, tx)
	if err != nil {
		return nil, err
	}
	out := make([]Column, 0, len(all))
	for _, c := range all {
		if !c.Deleted {
			out = append(out, c)
		}
	}
	return out, nil
}

// LockAllAny — все колонки под FOR UPDATE, включая мягко удалённые (Column.Deleted):
// восстановление колонки и служебная уборка (soft-delete-board-columns DS-019, DS-024).
func (r *ColumnRepo) LockAllAny(ctx context.Context, tx pgx.Tx) ([]Column, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+columnColumns+`, (SELECT count(*) FROM tasks t WHERE t.column_id = c.id AND t.deleted_at IS NULL) AS tasks_count,
		       c.deleted_at IS NOT NULL
		FROM columns c
		ORDER BY id
		FOR UPDATE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Column{}
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.IsFinal, &c.ConsumesStock, &c.Position, &c.TasksCount, &c.Deleted); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Insert — INSERT с временной (provisional) позицией; итоговую расставляет Renumber
// сразу следом в той же транзакции (design-sql.md §Column Renumber Query) — позиция
// не UNIQUE, дублирование на миг между Insert и Renumber безопасно.
func (r *ColumnRepo) Insert(ctx context.Context, tx pgx.Tx, in ColumnInput, provisionalPosition int) (*Column, error) {
	var c Column
	err := tx.QueryRow(ctx, `
		INSERT INTO columns (name, color, is_final, consumes_stock, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+columnColumns,
		in.Name, in.Color, in.IsFinal, in.ConsumesStock, provisionalPosition,
	).Scan(&c.ID, &c.Name, &c.Color, &c.IsFinal, &c.ConsumesStock, &c.Position)
	return &c, err
}

// Update — карточка колонки без position (позицию двигает Renumber отдельно, вызывающая
// сторона знает, менялась ли она). 0 строк → errNotFound — в том числе для мягко удалённой
// колонки (soft-delete-board-columns DS-026).
func (r *ColumnRepo) Update(ctx context.Context, tx pgx.Tx, id string, in ColumnInput) (*Column, error) {
	var c Column
	err := tx.QueryRow(ctx, `
		UPDATE columns SET name = $2, color = $3, is_final = $4, consumes_stock = $5, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+columnColumns,
		id, in.Name, in.Color, in.IsFinal, in.ConsumesStock,
	).Scan(&c.ID, &c.Name, &c.Color, &c.IsFinal, &c.ConsumesStock, &c.Position)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	return &c, err
}

// Renumber — design-sql.md §Column Renumber Query: сдвигает остальные колонки и ставит
// movingID на targetPosition (1..N, кэлмп — вызывающая сторона). Два отдельных Exec —
// оператора с общими $1/$2 не объединяются в одну команду (pgx extended protocol). Только
// действующие колонки: удалённая хранит прежнюю позицию без смысла (soft-delete-board-columns
// DS-019).
func (r *ColumnRepo) Renumber(ctx context.Context, tx pgx.Tx, movingID string, targetPosition int) error {
	if _, err := tx.Exec(ctx, `
		WITH ordered AS (
		    SELECT id, row_number() OVER (ORDER BY position, (id = $1) DESC, id) AS rn FROM columns WHERE id <> $1 AND deleted_at IS NULL)
		UPDATE columns c SET position = CASE WHEN o.rn >= $2 THEN o.rn + 1 ELSE o.rn END
		FROM ordered o WHERE c.id = o.id`, movingID, targetPosition); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE columns SET position = $2, updated_at = now() WHERE id = $1`, movingID, targetPosition)
	return err
}

// closeGaps — после удаления колонки перенумеровывает оставшиеся ДЕЙСТВУЮЩИЕ 1..N
// (design-sql.md: «position не UNIQUE: перенумерация 1..N в транзакции при
// create/update/delete»; soft-delete-board-columns DS-019 — удалённые не участвуют).
func (r *ColumnRepo) closeGaps(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		UPDATE columns c SET position = o.rn, updated_at = now()
		FROM (SELECT id, row_number() OVER (ORDER BY position, id) AS rn FROM columns WHERE deleted_at IS NULL) o
		WHERE c.id = o.id AND c.position <> o.rn`)
	return err
}

// HasTasks — в колонке есть ДЕЙСТВУЮЩИЕ заказы (EXISTS-проверка до удаления колонки). Мягко
// удалённые заказы колонку не удерживают (soft-delete-board-columns DS-019; переходное правило
// части 1 DS-035 для колонок снято). Отдельный оператор после LockAll: видит заказ, созданный
// или перенесённый транзакцией, которую удаление ждало на блокировке колонки.
func (r *ColumnRepo) HasTasks(ctx context.Context, tx pgx.Tx, columnID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tasks WHERE column_id = $1 AND deleted_at IS NULL)`, columnID).Scan(&exists)
	return exists, err
}

// TaskIDsForUpdate — design-sql.md §Column Recalc Query: заказы колонки под FOR UPDATE
// (порядок id) перед циклом StockEngine.Reconcile при смене consumes_stock (AC-007). Только
// действующие заказы (soft-delete-core DS-004): товар мягко удалённого заказа возвращён на
// склад и по правке колонки заново не списывается.
func (r *ColumnRepo) TaskIDsForUpdate(ctx context.Context, tx pgx.Tx, columnID string) ([]int64, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM tasks WHERE column_id = $1 AND deleted_at IS NULL ORDER BY id FOR UPDATE`, columnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// OpenSubtasksForColumn — незавершённые услуги ВСЕХ заказов колонки (эталон
// TaskRepo.OpenSubtasks, но по всей колонке разом) — используется ColumnService.Update
// при смене is_final false→true (AC-003, code review MEDIUM-3): заказ с открытыми
// услугами не должен молча попасть в финальную колонку.
func (r *ColumnRepo) OpenSubtasksForColumn(ctx context.Context, tx pgx.Tx, columnID string) ([]OpenSubtask, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, sv.name, s.status
		FROM subtasks s
		JOIN services sv ON sv.id = s.service_id
		JOIN tasks t ON t.id = s.task_id
		WHERE t.column_id = $1 AND t.deleted_at IS NULL AND s.status <> 'done'
		ORDER BY s.position, s.id`, columnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenSubtask{}
	for rows.Next() {
		var o OpenSubtask
		if err := rows.Scan(&o.ID, &o.ServiceName, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SoftDelete — отметка удаления колонки (soft-delete-board-columns §6.1, DS-024): атомарный
// UPDATE с условием «ещё не удалена» + уплотнение позиций действующих колонок. 0 строк →
// errNotFound. HasTasks/LAST_NON_FINAL_COLUMN проверяет сервис (данные под FOR UPDATE из LockAll).
func (r *ColumnRepo) SoftDelete(ctx context.Context, tx pgx.Tx, id, userID, reason string) (time.Time, error) {
	var at time.Time
	err := tx.QueryRow(ctx, `
		UPDATE columns SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING deleted_at`, id, userID, reason).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return at, errNotFound
	}
	if err != nil {
		return at, err
	}
	return at.UTC(), r.closeGaps(ctx, tx)
}

// Restore — снятие отметки удаления (soft-delete-board-columns §6.2, DS-024): атомарный UPDATE
// с условием «удалена»; три поля очищаются, позиция = максимум среди действующих + 1 (колонка
// встаёт в конец доски, DS-019). 0 строк → errNotFound (не удалена). Вызывается под LockAllAny.
func (r *ColumnRepo) Restore(ctx context.Context, tx pgx.Tx, id string) (int, error) {
	var position int
	err := tx.QueryRow(ctx, `
		UPDATE columns SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
		       position = (SELECT coalesce(max(position), 0) + 1 FROM columns WHERE deleted_at IS NULL),
		       updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL
		RETURNING position`, id).Scan(&position)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errNotFound
	}
	return position, err
}

// Purge — ФИЗИЧЕСКОЕ удаление строки колонки: только служебная уборка автотестов
// (ColumnService.Purge, DS-027) — действующей или мягко удалённой. На колонку ссылается заказ
// (в том числе удалённый) → 23503/23001 → errColumnHasTasks. 0 строк → errNotFound.
func (r *ColumnRepo) Purge(ctx context.Context, tx pgx.Tx, id string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM columns WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return errColumnHasTasks
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return r.closeGaps(ctx, tx)
}
