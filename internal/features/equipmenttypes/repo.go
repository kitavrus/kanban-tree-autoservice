package equipmenttypes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Type — тип оборудования (repo-модель).
type Type struct {
	ID             string
	Name           string
	SpecKeys       []string
	IsActive       bool
	EquipmentCount int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (
	ErrNotFound     = pgx.ErrNoRows
	errNameTaken    = errors.New("equipmenttypes: name taken")
	errNameArchived = errors.New("equipmenttypes: name archived")
	errInUse        = errors.New("equipmenttypes: in use")
)

// listColumns — проекция списка/получения (design-sql.md `## List/Get Equipment Type Query`).
// equipment_count — только действующие единицы: мягко удалённые не в счёт (soft-delete-catalog
// §4.4, TS-061, DS-014).
const listColumns = `t.id, t.name, t.spec_keys, t.is_active, t.created_at, t.updated_at,
	(SELECT count(*) FROM equipment e WHERE e.type_id = t.id AND e.deleted_at IS NULL)`

func scanWithCount(row pgx.Row) (*Type, error) {
	var t Type
	if err := row.Scan(&t.ID, &t.Name, &t.SpecKeys, &t.IsActive, &t.CreatedAt, &t.UpdatedAt, &t.EquipmentCount); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListParams — фильтры списка. IncludeArchived — сервис уже применил ADR-004
// (!isAdmin → false).
type ListParams struct {
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

// List — мягко удалённые типы не отдаются ни при каком фильтре архива (DS-009).
func (r *Repo) List(ctx context.Context, p ListParams) ([]Type, int, error) {
	conds := []string{"deleted_at IS NULL"}
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if !p.IncludeArchived {
		conds = append(conds, "is_active")
	}
	if p.Q != "" {
		add("name ILIKE $%d", "%"+sqlutil.EscapeLike(p.Q)+"%")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM equipment_types`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	// id вторым ключом сортировки — стабильный порядок строк с равным именем.
	q := `SELECT ` + listColumns + ` FROM equipment_types t` + where +
		fmt.Sprintf(" ORDER BY lower(t.name), t.id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Type{}
	for rows.Next() {
		t, err := scanWithCount(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

// Get — id уже провалидирован как uuid сервисом.
func (r *Repo) Get(ctx context.Context, id string) (*Type, error) {
	return scanWithCount(r.pool.QueryRow(ctx, `SELECT `+listColumns+` FROM equipment_types t WHERE t.id = $1::uuid AND t.deleted_at IS NULL`, id))
}

// Create — INSERT; 23505 (equipment_types_name_lower_uq) → errNameTaken/errNameArchived
// в зависимости от is_active строки-конфликта (ADR-007).
func (r *Repo) Create(ctx context.Context, in Input) (*Type, error) {
	var t Type
	err := r.pool.QueryRow(ctx,
		`INSERT INTO equipment_types (name, spec_keys) VALUES ($1, $2::text[])
		 RETURNING id, name, spec_keys, is_active, created_at, updated_at`,
		in.Name, in.SpecKeys).Scan(&t.ID, &t.Name, &t.SpecKeys, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, r.mapNameConflict(ctx, in.Name, err)
	}
	// Новый тип ещё не может использоваться в оборудовании.
	t.EquipmentCount = 0
	return &t, nil
}

// Update — полная замена карточки типа; ErrNoRows → ErrNotFound (сервис маппит),
// 23505 → как в Create (конфликт по строке ≠ правимая запись).
func (r *Repo) Update(ctx context.Context, id string, in Input) (*Type, error) {
	var t Type
	err := r.pool.QueryRow(ctx,
		`UPDATE equipment_types SET name = $2, spec_keys = $3::text[], is_active = $4, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL RETURNING id, name, spec_keys, is_active, created_at, updated_at`,
		id, in.Name, in.SpecKeys, in.IsActive).Scan(&t.ID, &t.Name, &t.SpecKeys, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, r.mapNameConflict(ctx, in.Name, err)
	}
	cnt, err := r.equipmentCount(ctx, id)
	if err != nil {
		return nil, err
	}
	t.EquipmentCount = cnt
	return &t, nil
}

// SetActive — SSR archive/restore, без перезаписи имени.
func (r *Repo) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE equipment_types SET is_active = $2, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка типа автотестами (soft-delete-catalog §6.9, DS-027),
// действующего либо мягко удалённого. RowsAffected 0 → ErrNotFound; FK-нарушение (equipment_type_id_fkey, RESTRICT)
// → errInUse. Postgres на явный ON DELETE RESTRICT отдаёт SQLSTATE 23001
// (restrict_violation), а не 23503 (foreign_key_violation, NO ACTION) — проверяем оба.
func (r *Repo) Purge(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM equipment_types WHERE id = $1::uuid`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return errInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) equipmentCount(ctx context.Context, typeID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM equipment WHERE type_id = $1::uuid AND deleted_at IS NULL`, typeID).Scan(&n)
	return n, err
}

// mapNameConflict — 23505 (equipment_types_name_lower_uq) → errNameTaken (строка-конфликт
// активна) | errNameArchived (в архиве); прочие ошибки (в т.ч. pgx.ErrNoRows) — как есть.
// Строка-конфликт ищется только среди действующих: индекс частичный, название удалённого типа
// свободно (DS-017).
func (r *Repo) mapNameConflict(ctx context.Context, name string, err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	var active bool
	if scanErr := r.pool.QueryRow(ctx,
		`SELECT is_active FROM equipment_types WHERE lower(btrim(name)) = lower(btrim($1)) AND deleted_at IS NULL`, name).Scan(&active); scanErr != nil {
		return err
	}
	if active {
		return errNameTaken
	}
	return errNameArchived
}
