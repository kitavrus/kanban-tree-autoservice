package warehouses

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Warehouse — склад (repo-модель, DS-001).
type Warehouse struct {
	ID        string
	Name      string
	Position  int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (
	ErrNotFound     = pgx.ErrNoRows
	errNameTaken    = errors.New("warehouses: name taken")
	errNameArchived = errors.New("warehouses: name archived")
)

const columns = `id, name, position, is_active, created_at, updated_at`

func scan(row pgx.Row) (*Warehouse, error) {
	var w Warehouse
	if err := row.Scan(&w.ID, &w.Name, &w.Position, &w.IsActive, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return nil, err
	}
	return &w, nil
}

type ListParams struct {
	IncludeArchived bool
	Limit, Offset   int
}

// List — порядок по position (склад по умолчанию — первый активный, DS-005).
func (r *Repo) List(ctx context.Context, p ListParams) ([]Warehouse, int, error) {
	// Удалённые склады не возвращаются ни при каком значении фильтра архива (DS-009, DS-014).
	where := " WHERE deleted_at IS NULL"
	if !p.IncludeArchived {
		where += " AND is_active"
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM warehouses`+where).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM warehouses`+where+
		` ORDER BY position, id LIMIT $1 OFFSET $2`, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Warehouse{}
	for rows.Next() {
		w, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *w)
	}
	return out, total, rows.Err()
}

// Get — действующий склад; удалённый для обычных адресов не существует (DS-026).
func (r *Repo) Get(ctx context.Context, id string) (*Warehouse, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+columns+` FROM warehouses WHERE id = $1::uuid AND deleted_at IS NULL`, id))
}

// Create — position = max+1 (в конец списка); 23505 → errNameTaken/errNameArchived.
func (r *Repo) Create(ctx context.Context, in Input) (*Warehouse, error) {
	w, err := scan(r.pool.QueryRow(ctx,
		`INSERT INTO warehouses (name, position)
		 VALUES ($1, (SELECT coalesce(max(position), 0) + 1 FROM warehouses))
		 RETURNING `+columns, in.Name))
	if err != nil {
		return nil, r.mapNameConflict(ctx, in.Name, err)
	}
	return w, nil
}

// dbtx — пул или транзакция (архив склада идёт в tx под блокировкой активных, D47).
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Update — переименование + is_active; ErrNoRows → ErrNotFound (сервис маппит).
func (r *Repo) Update(ctx context.Context, id string, in Input) (*Warehouse, error) {
	return r.UpdateIn(ctx, r.pool, id, in)
}

// UpdateIn — Update в заданной tx/пуле. Классификация 23505 — отдельным запросом на пуле
// (mapNameConflict), поэтому работает и после аборта tx.
func (r *Repo) UpdateIn(ctx context.Context, q dbtx, id string, in Input) (*Warehouse, error) {
	w, err := scan(q.QueryRow(ctx,
		`UPDATE warehouses SET name = $2, is_active = $3, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL RETURNING `+columns, id, in.Name, in.IsActive))
	if err != nil {
		return nil, r.mapNameConflict(ctx, in.Name, err)
	}
	return w, nil
}

// SetActive — SSR archive/restore, без перезаписи имени.
func (r *Repo) SetActive(ctx context.Context, id string, active bool) error {
	return r.SetActiveIn(ctx, r.pool, id, active)
}

// SetActiveIn — SetActive в заданной tx/пуле.
func (r *Repo) SetActiveIn(ctx context.Context, q dbtx, id string, active bool) error {
	tag, err := q.Exec(ctx, `UPDATE warehouses SET is_active = $2, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// InUse — на складе числится оборудование клиентов либо есть ДЕЙСТВУЮЩИЙ незавершённый документ
// «Перемещение» (requested / in_transit), где склад — отправитель или получатель
// (transfers-core DS-026). Мягко удалённая заявка не мешает: администратору она не видна
// (soft-delete DS-044).
func (r *Repo) InUse(ctx context.Context, q dbtx, id string) (bool, error) {
	var inUse bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM equipment WHERE warehouse_id = $1::uuid)
		    OR EXISTS (SELECT 1 FROM transfers
		               WHERE status IN ('requested', 'in_transit') AND deleted_at IS NULL
		                 AND (from_warehouse_id = $1::uuid OR to_warehouse_id = $1::uuid))`, id).Scan(&inUse)
	return inUse, err
}

// mapNameConflict — 23505 (warehouses_name_lower_uq, частичный: только действующие, DS-017) →
// errNameTaken | errNameArchived.
func (r *Repo) mapNameConflict(ctx context.Context, name string, err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	var active bool
	if scanErr := r.pool.QueryRow(ctx,
		`SELECT is_active FROM warehouses WHERE lower(btrim(name)) = lower(btrim($1)) AND deleted_at IS NULL`, name).Scan(&active); scanErr != nil {
		return err
	}
	if active {
		return errNameTaken
	}
	return errNameArchived
}
