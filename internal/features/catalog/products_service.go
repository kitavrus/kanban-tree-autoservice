package catalog

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
	ErrProductNotFoundErr = errorspkg.New("PRODUCT_NOT_FOUND", http.StatusNotFound, "Запчасть не найдена")
	// ErrProductInUse — только служебная уборка автотестов (DS-027): продуктовое удаление мягкое,
	// ссылки ему не мешают (DS-014).
	ErrProductInUse = errorspkg.New("PRODUCT_IN_USE", http.StatusConflict,
		"Запчасть используется в услугах, заказах или журнале склада — удалить нельзя, отправьте её в архив")
	// ErrProductHasStock — товар с ненулевым остатком не удаляется (DS-021).
	ErrProductHasStock = errorspkg.New("PRODUCT_HAS_STOCK", http.StatusConflict,
		"У запчасти есть остаток на складе — сначала спишите её")
)

type ProductService struct {
	repo *ProductRepo
}

func NewProductService(pool *pgxpool.Pool) *ProductService {
	return &ProductService{repo: NewProductRepo(pool)}
}

type ProductListQuery struct {
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

const productLookupDefaultLimit = 50 // API §Лимиты task.md: default 50 / max 500

// List — !isAdmin форсирует IncludeArchived=false (исполнитель видит только активные —
// D19).
func (s *ProductService) List(ctx context.Context, q ProductListQuery, isAdmin bool) (*ProductListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = productLookupDefaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	includeArchived := q.IncludeArchived && isAdmin

	query := strings.TrimSpace(q.Q)
	if sqlutil.HasInvalidText(query) {
		query = ""
	}
	items, total, err := s.repo.List(ctx, ProductListParams{
		Q: query, IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out, err := s.toResponses(ctx, items)
	if err != nil {
		return nil, err
	}
	return &ProductListResponse{Items: out, Total: total, Limit: limit, Offset: offset}, nil
}

// Get — D19 «активные записи» относится к List (для исполнителя IncludeArchived
// форсится в false); Get по конкретному id архив НЕ фильтрует ни для админа, ни для
// исполнителя — as-is решение, зафиксировано по прецеденту equipmenttypes.Service.Get
// (code review LOW#5, не блокер: осмысленный сценарий — открыть карточку архивной
// записи по прямой ссылке из другого места экрана, а не «спрятать» её вовсе).
func (s *ProductService) Get(ctx context.Context, id string) (*ProductResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrProductNotFoundErr
	}
	p, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrProductNotFound) {
		return nil, ErrProductNotFoundErr
	}
	if err != nil {
		return nil, err
	}
	return s.toResponse(ctx, *p)
}

func (s *ProductService) Create(ctx context.Context, in ProductInput) (*ProductResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	p, err := s.repo.Create(ctx, in)
	switch {
	case errors.Is(err, errAmountOutOfRange):
		return nil, ErrAmountOutOfRange
	case err != nil:
		return nil, err
	}
	return s.toResponse(ctx, *p)
}

// Update — userID пишется в stock_movements при ручном изменении остатка (в той же
// транзакции, что и UPDATE карточки — репозиторий).
func (s *ProductService) Update(ctx context.Context, id, userID string, in ProductInput) (*ProductResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrProductNotFoundErr
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	p, err := s.repo.Update(ctx, id, userID, in)
	switch {
	case errors.Is(err, ErrProductNotFound):
		return nil, ErrProductNotFoundErr
	case errors.Is(err, errStockDeltaOutOfRange):
		return nil, errorspkg.Validation(errorspkg.Field{Name: "stock", Message: msgTooLarge})
	case errors.Is(err, errAmountOutOfRange):
		return nil, ErrAmountOutOfRange
	case err != nil:
		return nil, err
	}
	return s.toResponse(ctx, *p)
}

func (s *ProductService) SetActive(ctx context.Context, id string, active bool) error {
	if uuid.Validate(id) != nil {
		return ErrProductNotFoundErr
	}
	if err := s.repo.SetActive(ctx, id, active); errors.Is(err, ErrProductNotFound) {
		return ErrProductNotFoundErr
	} else if err != nil {
		return err
	}
	return nil
}

func (s *ProductService) toResponse(_ context.Context, p Product) (*ProductResponse, error) {
	r := toProductResponse(p)
	return &r, nil
}

func (s *ProductService) toResponses(_ context.Context, items []Product) ([]ProductResponse, error) {
	out := make([]ProductResponse, len(items))
	for i, p := range items {
		out[i] = toProductResponse(p)
	}
	return out, nil
}

func toProductResponse(p Product) ProductResponse {
	return ProductResponse{
		ID: p.ID, Name: p.Name, SKU: p.SKU, Unit: p.Unit,
		Price: Amount(p.Price), IsActive: p.IsActive,
		Stock: Amount(p.Stock), MinStock: Amount(p.MinStock), Box: p.Box, Place: p.Place,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
