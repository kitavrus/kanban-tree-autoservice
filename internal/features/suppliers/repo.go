package suppliers

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Supplier — поставщик (repo-модель, DS-007).
type Supplier struct {
	ID          string
	Name        string
	ContactName string
	Phone       string
	Address     string
	Note        string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (
	ErrNotFound      = pgx.ErrNoRows
	errNameDuplicate = errors.New("suppliers: name duplicate")
)

const columns = `id, name, contact_name, phone, address, note, is_active, created_at, updated_at`

func scan(row pgx.Row) (*Supplier, error) {
	var s Supplier
	if err := row.Scan(&s.ID, &s.Name, &s.ContactName, &s.Phone, &s.Address, &s.Note,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListParams — фильтры списка: IsActive == nil — все; Q — подстрока названия без регистра.
type ListParams struct {
	IsActive      *bool
	Q             string
	Limit, Offset int
}

// List — активные раньше архивных, затем по названию (§6.1).
func (r *Repo) List(ctx context.Context, p ListParams) ([]Supplier, int, error) {
	// Удалённые поставщики не возвращаются ни при каком фильтре архива (DS-009, DS-014).
	where := " WHERE deleted_at IS NULL"
	var args []any
	if p.IsActive != nil {
		args = append(args, *p.IsActive)
		where += " AND is_active = $" + strconv.Itoa(len(args))
	}
	if p.Q != "" {
		args = append(args, "%"+sqlutil.EscapeLike(p.Q)+"%")
		where += " AND name ILIKE $" + strconv.Itoa(len(args))
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM suppliers`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, p.Limit, p.Offset)
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM suppliers`+where+
		` ORDER BY is_active DESC, lower(name), id LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Supplier{}
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

// Create — 23505 (suppliers_name_lower_uq) → errNameDuplicate.
func (r *Repo) Create(ctx context.Context, in Input) (*Supplier, error) {
	s, err := scan(r.pool.QueryRow(ctx,
		`INSERT INTO suppliers (name, contact_name, phone, address, note)
		 VALUES ($1, $2, $3, $4, $5) RETURNING `+columns,
		in.Name, in.ContactName, in.Phone, in.Address, in.Note))
	if err != nil {
		return nil, mapNameConflict(err)
	}
	return s, nil
}

// Update — все поля + is_active, last-write-wins (DS-043); ErrNoRows → ErrNotFound
// (сервис маппит); удалённый поставщик для правки не существует (soft-delete DS-026).
func (r *Repo) Update(ctx context.Context, id string, in Input) (*Supplier, error) {
	s, err := scan(r.pool.QueryRow(ctx,
		`UPDATE suppliers SET name = $2, contact_name = $3, phone = $4, address = $5, note = $6,
		        is_active = $7, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL RETURNING `+columns,
		id, in.Name, in.ContactName, in.Phone, in.Address, in.Note, in.IsActive))
	if err != nil {
		return nil, mapNameConflict(err)
	}
	return s, nil
}

func mapNameConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errNameDuplicate
	}
	return err
}
