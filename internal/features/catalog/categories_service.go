package catalog

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

var (
	ErrCategoryNotFoundErr = errorspkg.New("CATEGORY_NOT_FOUND", http.StatusNotFound, "Категория не найдена")
	ErrCategoryNameTaken   = errorspkg.New("CATEGORY_NAME_TAKEN", http.StatusConflict, "Такая категория уже есть")
	// ErrCategoryInUse — только служебная уборка автотестов (DS-027): продуктовое удаление
	// мягкое, товары и услуги категории ему не мешают (DS-014).
	ErrCategoryInUse = errorspkg.New("CATEGORY_IN_USE", http.StatusConflict,
		"Категория используется — удалить нельзя, отправьте её в архив")
)

// defaultLimit/maxLimit — списки каталога используются как lookup (комбобоксы, панель
// категорий, фильтры) — как у equipmenttypes. Значение поднято с 500 до 2000 (FIX-LOOP
// изоляции E2E, 2026-09-27): категории/товары/услуги сортируются по имени и обрезаются
// этим лимитом без индикации «показаны N из M» — на разросшемся dev-стенде (сотни
// прогонов) свежесозданная запись с произвольным именем переставала попадать в первую
// страницу. В реальной мастерской категорий/товаров/услуг — десятки-сотни, 2000 — запас
// на годы роста без пагинации UI.
const (
	defaultLimit = 2000
	maxLimit     = 2000
)

type CategoryService struct {
	repo *CategoryRepo
}

func NewCategoryService(pool *pgxpool.Pool) *CategoryService {
	return &CategoryService{repo: NewCategoryRepo(pool)}
}

type CategoryListQuery struct {
	Kind            string
	IncludeArchived bool
	Limit, Offset   int
}

// List — !isAdmin форсирует IncludeArchived=false (как equipmenttypes ADR-004).
func (s *CategoryService) List(ctx context.Context, q CategoryListQuery, isAdmin bool) (*CategoryListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	includeArchived := q.IncludeArchived && isAdmin

	kind := strings.TrimSpace(q.Kind)
	if kind != KindService && kind != KindProduct {
		kind = ""
	}
	items, total, err := s.repo.List(ctx, CategoryListParams{Kind: kind, IncludeArchived: includeArchived, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	resp := &CategoryListResponse{Items: make([]CategoryResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, c := range items {
		resp.Items[i] = toCategoryResponse(c)
	}
	return resp, nil
}

func (s *CategoryService) Get(ctx context.Context, id string) (*CategoryResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrCategoryNotFoundErr
	}
	c, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrCategoryNotFound) {
		return nil, ErrCategoryNotFoundErr
	}
	if err != nil {
		return nil, err
	}
	resp := toCategoryResponse(*c)
	return &resp, nil
}

func (s *CategoryService) Create(ctx context.Context, in CategoryInput) (*CategoryResponse, error) {
	in = in.normalize()
	if f := in.validateCreate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	c, err := s.repo.Create(ctx, in.Kind, in.Name)
	if errors.Is(err, errCategoryNameTaken) {
		return nil, ErrCategoryNameTaken
	}
	if err != nil {
		return nil, err
	}
	resp := toCategoryResponse(*c)
	return &resp, nil
}

// Rename — единственная мутация карточки категории (kind неизменяем).
func (s *CategoryService) Rename(ctx context.Context, id, name string) (*CategoryResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrCategoryNotFoundErr
	}
	name = strings.TrimSpace(name)
	if f := validateCategoryName(name); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	c, err := s.repo.Rename(ctx, id, name)
	switch {
	case errors.Is(err, ErrCategoryNotFound):
		return nil, ErrCategoryNotFoundErr
	case errors.Is(err, errCategoryNameTaken):
		return nil, ErrCategoryNameTaken
	case err != nil:
		return nil, err
	}
	resp := toCategoryResponse(*c)
	return &resp, nil
}

func (s *CategoryService) SetActive(ctx context.Context, id string, active bool) error {
	if uuid.Validate(id) != nil {
		return ErrCategoryNotFoundErr
	}
	if err := s.repo.SetActive(ctx, id, active); errors.Is(err, ErrCategoryNotFound) {
		return ErrCategoryNotFoundErr
	} else if err != nil {
		return err
	}
	return nil
}

func toCategoryResponse(c Category) CategoryResponse {
	return CategoryResponse{
		ID: c.ID, Kind: c.Kind, Name: c.Name, IsActive: c.IsActive,
		ItemCount: c.ItemCount, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}
