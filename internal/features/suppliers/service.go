package suppliers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

var ErrSupplierNotFound = errorspkg.New("SUPPLIER_NOT_FOUND", http.StatusNotFound, "Поставщик не найден")

const (
	defaultLimit = 50
	maxLimit     = 500
)

type Service struct {
	repo *Repo
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepo(pool)}
}

// ListQuery — сырые параметры списка (§6.1). IsActive: "" — все, "true"/"false" — фильтр,
// иное → 400 is_active: invalid.
type ListQuery struct {
	IsActive      string
	Q             string
	Limit, Offset int
}

func (s *Service) List(ctx context.Context, q ListQuery) (*SupplierListResponse, error) {
	p := ListParams{Q: strings.TrimSpace(q.Q)}
	var f []errorspkg.Field
	switch q.IsActive {
	case "":
	case "true", "false":
		active := q.IsActive == "true"
		p.IsActive = &active
	default:
		f = append(f, errorspkg.Field{Name: "is_active", Message: msgInvalid})
	}
	switch {
	case sqlutil.HasInvalidText(p.Q):
		f = append(f, errorspkg.Field{Name: "q", Message: msgInvalidText})
	case utf8.RuneCountInString(p.Q) > queryMax:
		f = append(f, errorspkg.Field{Name: "q", Message: msgTooLong})
	}
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	p.Limit, p.Offset = q.Limit, max(q.Offset, 0)
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	p.Limit = min(p.Limit, maxLimit)

	items, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, err
	}
	resp := &SupplierListResponse{Items: make([]SupplierResponse, len(items)), Total: total, Limit: p.Limit, Offset: p.Offset}
	for i, it := range items {
		resp.Items[i] = toResponse(it)
	}
	return resp, nil
}

func (s *Service) Create(ctx context.Context, in Input) (*SupplierResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	sup, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := toResponse(*sup)
	return &resp, nil
}

// Update — правка полей и архив/возврат одним запросом; last-write-wins (DS-043).
func (s *Service) Update(ctx context.Context, id string, in Input) (*SupplierResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrSupplierNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	// Архив поставщика с незакрытыми закупками запрещён (purchase-orders §6.18, DS-043).
	if !in.IsActive {
		if err := s.repo.CheckNoOpenPurchases(ctx, id); err != nil {
			return nil, err
		}
	}
	sup, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := toResponse(*sup)
	return &resp, nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return ErrSupplierNotFound
	case errors.Is(err, errNameDuplicate):
		return errorspkg.Validation(errorspkg.Field{Name: "name", Message: msgDuplicate})
	}
	return err
}

func toResponse(s Supplier) SupplierResponse {
	return SupplierResponse{
		ID: s.ID, Name: s.Name, ContactName: s.ContactName, Phone: s.Phone, Address: s.Address,
		Note: s.Note, IsActive: s.IsActive, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}
