package warehouses

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

var (
	ErrWarehouseNotFound = errorspkg.New("WAREHOUSE_NOT_FOUND", http.StatusNotFound, "Склад не найден")
	// ErrLastActive — D47: последний активный склад архивировать нельзя (склад по умолчанию
	// строки товара — «первый активный», DS-005; без активных складов добавить товар в заказ
	// было бы некуда).
	ErrLastActive = errorspkg.New("WAREHOUSE_LAST_ACTIVE", http.StatusConflict,
		"Нельзя отправить в архив последний активный склад")
	// ErrInUse — transfers-core DS-026: склад нельзя архивировать, пока на нём числится
	// оборудование клиентов (equipment.warehouse_id) либо есть незавершённый документ
	// «Перемещение» (requested / in_transit) с этим складом.
	ErrInUse = errorspkg.New("WAREHOUSE_IN_USE", http.StatusConflict,
		"На складе есть оборудование или незавершённые перемещения")
)

const (
	defaultLimit = 500
	maxLimit     = 500
)

type Service struct {
	repo *Repo
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepo(pool), pool: pool}
}

type ListQuery struct {
	IncludeArchived bool
	Limit, Offset   int
}

func (s *Service) List(ctx context.Context, q ListQuery) (*WarehouseListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	items, total, err := s.repo.List(ctx, ListParams{IncludeArchived: q.IncludeArchived, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	resp := &WarehouseListResponse{Items: make([]WarehouseResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, w := range items {
		resp.Items[i] = toResponse(w)
	}
	return resp, nil
}

func (s *Service) Get(ctx context.Context, id string) (*WarehouseResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrWarehouseNotFound
	}
	w, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrWarehouseNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := toResponse(*w)
	return &resp, nil
}

func (s *Service) Create(ctx context.Context, in Input) (*WarehouseResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	w, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := toResponse(*w)
	return &resp, nil
}

func (s *Service) Update(ctx context.Context, id string, in Input) (*WarehouseResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrWarehouseNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	var w *Warehouse
	var err error
	if in.IsActive {
		w, err = s.repo.Update(ctx, id, in)
	} else {
		err = s.withLastActiveGuard(ctx, id, func(tx pgx.Tx) error {
			var uerr error
			w, uerr = s.repo.UpdateIn(ctx, tx, id, in)
			return uerr
		})
	}
	if err != nil {
		return nil, mapErr(err)
	}
	resp := toResponse(*w)
	return &resp, nil
}

// SetActive — SSR archive/restore.
func (s *Service) SetActive(ctx context.Context, id string, active bool) error {
	if uuid.Validate(id) != nil {
		return ErrWarehouseNotFound
	}
	var err error
	if active {
		err = s.repo.SetActive(ctx, id, true)
	} else {
		err = s.withLastActiveGuard(ctx, id, func(tx pgx.Tx) error { return s.repo.SetActiveIn(ctx, tx, id, false) })
	}
	if errors.Is(err, ErrNotFound) {
		return ErrWarehouseNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// withLastActiveGuard — архив склада id запрещён, если он — единственный активный (D47),
// либо на нём числится оборудование / есть незавершённые перемещения (ErrInUse, DS-026).
// Проверка и запись — одной транзакцией: активные склады блокируются FOR UPDATE в порядке
// id, поэтому параллельные архивы двух последних складов сериализуются, и второй видит
// уже архивированный первый (READ COMMITTED перепроверяет is_active) → ErrLastActive.
func (s *Service) withLastActiveGuard(ctx context.Context, id string, write func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT id::text FROM warehouses WHERE is_active AND deleted_at IS NULL ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	var active []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			rows.Close()
			return err
		}
		active = append(active, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(active) == 1 && strings.EqualFold(active[0], id) {
		return ErrLastActive
	}
	// DS-026 — только при переходе «активный → архив» (правка уже архивного склада с
	// is_active=false архивированием не является).
	archiving := false
	for _, a := range active {
		if strings.EqualFold(a, id) {
			archiving = true
		}
	}
	if archiving {
		inUse, err := s.repo.InUse(ctx, tx, id)
		if err != nil {
			return err
		}
		if inUse {
			return ErrInUse
		}
	}
	if err := write(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return ErrWarehouseNotFound
	case errors.Is(err, errNameTaken):
		return errorspkg.Validation(errorspkg.Field{Name: "name", Message: msgTaken})
	case errors.Is(err, errNameArchived):
		return errorspkg.Validation(errorspkg.Field{Name: "name", Message: msgArchived})
	}
	return err
}

func toResponse(w Warehouse) WarehouseResponse {
	return WarehouseResponse{
		ID: w.ID, Name: w.Name, Position: w.Position, IsActive: w.IsActive,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
}
