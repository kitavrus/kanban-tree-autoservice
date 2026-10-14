package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Category — repo-модель категории каталога.
type Category struct {
	ID        string
	Kind      string
	Name      string
	IsActive  bool
	ItemCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CategoryRepo struct {
	pool *pgxpool.Pool
}

func NewCategoryRepo(pool *pgxpool.Pool) *CategoryRepo { return &CategoryRepo{pool: pool} }

var (
	ErrCategoryNotFound  = pgx.ErrNoRows
	errCategoryNameTaken = errors.New("catalog: category name taken")
	errCategoryInUse     = errors.New("catalog: category in use")
	// errCategoryDeleted — выбранная категория мягко удалена (400 category_id: not_found, DS-014).
	errCategoryDeleted = errors.New("catalog: category deleted")
)

// categoryCountSQL — счётчик item_count категории (алиас c): только действующие услуги —
// мягко удалённые не в счёт, архивные считаются (soft-delete-catalog §4.3, DS-014).
const categoryCountSQL = `(SELECT count(*) FROM services s WHERE s.category_id = c.id AND s.deleted_at IS NULL)`

type CategoryListParams struct {
	Kind            string // "" — оба вида
	IncludeArchived bool
	Limit, Offset   int
}

// List — kind пуст → оба вида (используется редко, экраны фильтруют по конкретному
// виду); item_count считается по kind СТРОКИ (учитывая, что список может быть смешанным,
// считаем через CASE). Мягко удалённые категории не отдаются ни при каком фильтре архива.
func (r *CategoryRepo) List(ctx context.Context, p CategoryListParams) ([]Category, int, error) {
	conds := []string{"c.deleted_at IS NULL"}
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if p.Kind != "" {
		add("c.kind = $%d", p.Kind)
	}
	if !p.IncludeArchived {
		conds = append(conds, "c.is_active")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_categories c`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT c.id, c.kind, c.name, c.is_active, c.created_at, c.updated_at, ` + categoryCountSQL +
		` FROM catalog_categories c` + where +
		fmt.Sprintf(" ORDER BY lower(c.name), c.id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Kind, &c.Name, &c.IsActive, &c.CreatedAt, &c.UpdatedAt, &c.ItemCount); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Get — id уже провалидирован как uuid сервисом.
func (r *CategoryRepo) Get(ctx context.Context, id string) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.kind, c.name, c.is_active, c.created_at, c.updated_at, `+categoryCountSQL+`
		 FROM catalog_categories c WHERE c.id = $1::uuid AND c.deleted_at IS NULL`, id).
		Scan(&c.ID, &c.Kind, &c.Name, &c.IsActive, &c.CreatedAt, &c.UpdatedAt, &c.ItemCount)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create — INSERT; 23505 (catalog_categories_kind_name_lower_uq) → errCategoryNameTaken.
func (r *CategoryRepo) Create(ctx context.Context, kind, name string) (*Category, error) {
	var c Category
	c.Kind = kind
	err := r.pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ($1, $2)
		 RETURNING id, name, is_active, created_at, updated_at`,
		kind, name).Scan(&c.ID, &c.Name, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, mapUniqueViolation(err, errCategoryNameTaken)
	}
	c.ItemCount = 0
	return &c, nil
}

// Rename — UPDATE name; ErrNoRows → ErrCategoryNotFound, 23505 → errCategoryNameTaken.
func (r *CategoryRepo) Rename(ctx context.Context, id, name string) (*Category, error) {
	var c Category
	err := r.pool.QueryRow(ctx,
		`UPDATE catalog_categories SET name = $2, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL
		 RETURNING id, kind, name, is_active, created_at, updated_at`,
		id, name).Scan(&c.ID, &c.Kind, &c.Name, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, mapUniqueViolation(err, errCategoryNameTaken)
	}
	cnt, err := r.itemCount(ctx, id)
	if err != nil {
		return nil, err
	}
	c.ItemCount = cnt
	return &c, nil
}

// SetActive — SSR архив/восстановление.
func (r *CategoryRepo) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE catalog_categories SET is_active = $2, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка категории автотестами (§6.9, DS-027), действующей либо
// мягко удалённой. RowsAffected 0 → ErrCategoryNotFound; FK-нарушение (RESTRICT из products/services)
// → errCategoryInUse. Postgres на явный ON DELETE RESTRICT отдаёт 23001, а не 23503.
func (r *CategoryRepo) Purge(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM catalog_categories WHERE id = $1::uuid`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return errCategoryInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func (r *CategoryRepo) itemCount(ctx context.Context, id string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM services WHERE category_id = $1::uuid AND deleted_at IS NULL`, id).Scan(&n)
	return n, err
}

// mapUniqueViolation — 23505 → target; прочие ошибки (в т.ч. pgx.ErrNoRows) — как есть.
func mapUniqueViolation(err error, target error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return target
	}
	return err
}
