package equipmenttypes

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

var (
	ErrTypeNotFound  = errorspkg.New("EQUIPMENT_TYPE_NOT_FOUND", http.StatusNotFound, "Тип оборудования не найден")
	ErrTypeNameTaken = errorspkg.New("EQUIPMENT_TYPE_NAME_TAKEN", http.StatusConflict, "Такой тип уже есть")
	// ADR-007: отдельный код → клиент показывает CTA «Вернуть из архива» по code, не по тексту.
	ErrTypeNameArchived = errorspkg.New("EQUIPMENT_TYPE_NAME_ARCHIVED", http.StatusConflict,
		"Такой тип уже есть в архиве — верните его из архива")
	// ErrTypeInUse — только служебная уборка автотестов (DS-027): продуктовое удаление мягкое,
	// единицы оборудования ему не мешают (DS-014).
	ErrTypeInUse = errorspkg.New("EQUIPMENT_TYPE_IN_USE", http.StatusConflict,
		"Тип используется в оборудовании клиентов — удалить нельзя, отправьте его в архив")
)

const (
	// defaultLimit/maxLimit — 500: список типов используется как lookup (комбобокс).
	defaultLimit = 500
	maxLimit     = 500
)

type Service struct {
	repo *Repo
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepo(pool)}
}

// ListQuery — параметры списка из HTTP-слоя (сырые).
type ListQuery struct {
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

// List — !isAdmin форсирует IncludeArchived=false (ADR-004): исполнитель видит
// только активные типы, даже если сам попросил include_archived=true.
func (s *Service) List(ctx context.Context, q ListQuery, isAdmin bool) (*TypeListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	includeArchived := q.IncludeArchived && isAdmin

	query := strings.TrimSpace(q.Q)
	if sqlutil.HasInvalidText(query) {
		// NUL-байт / невалидный UTF-8 в q — не 500: фильтр игнорируется (200, как без q),
		// а не ошибка (code review §TD-4).
		query = ""
	}
	items, total, err := s.repo.List(ctx, ListParams{
		Q: query, IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	resp := &TypeListResponse{Items: make([]TypeResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, t := range items {
		resp.Items[i] = toResponse(t)
	}
	return resp, nil
}

func (s *Service) Get(ctx context.Context, id string) (*TypeResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrTypeNotFound
	}
	t, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrTypeNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := toResponse(*t)
	return &resp, nil
}

func (s *Service) Create(ctx context.Context, in Input) (*TypeResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	t, err := s.repo.Create(ctx, in)
	switch {
	case errors.Is(err, errNameTaken):
		return nil, ErrTypeNameTaken
	case errors.Is(err, errNameArchived):
		return nil, ErrTypeNameArchived
	case err != nil:
		return nil, err
	}
	resp := toResponse(*t)
	return &resp, nil
}

func (s *Service) Update(ctx context.Context, id string, in Input) (*TypeResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrTypeNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	t, err := s.repo.Update(ctx, id, in)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrTypeNotFound
	case errors.Is(err, errNameTaken):
		return nil, ErrTypeNameTaken
	case errors.Is(err, errNameArchived):
		return nil, ErrTypeNameArchived
	case err != nil:
		return nil, err
	}
	resp := toResponse(*t)
	return &resp, nil
}

// SetActive — SSR archive/restore (Phase 4), без перезаписи имени.
func (s *Service) SetActive(ctx context.Context, id string, active bool) error {
	if uuid.Validate(id) != nil {
		return ErrTypeNotFound
	}
	if err := s.repo.SetActive(ctx, id, active); errors.Is(err, ErrNotFound) {
		return ErrTypeNotFound
	} else if err != nil {
		return err
	}
	return nil
}

func toResponse(t Type) TypeResponse {
	return TypeResponse{
		ID: t.ID, Name: t.Name, SpecKeys: t.SpecKeys, IsActive: t.IsActive,
		EquipmentCount: t.EquipmentCount, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}
