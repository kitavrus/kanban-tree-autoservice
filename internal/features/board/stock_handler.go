package board

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

const (
	stockDefaultLimit = 200
	stockMaxLimit     = 1000
)

// stockService — узкий интерфейс зависимости StockHandler (эталон taskService).
type stockService interface {
	List(ctx context.Context, p StockListParams) (*StockListResponse, error)
	Summary(ctx context.Context) (*StockSummaryResponse, error)
	Adjust(ctx context.Context, productID, userID string, in StockAdjustRequest) (*StockRow, error)
}

// StockHandler — JSON API /api/v1/stock (чтение — RequireAPI, Adjust — +admin).
type StockHandler struct {
	svc stockService
}

func NewStockHandler(svc stockService) *StockHandler { return &StockHandler{svc: svc} }

// List — GET /api/v1/stock?scope=&q=&limit=&offset=. scope/limit/offset — enum/диапазон (design-errors.md §Validation Fields), иначе 400 field.
func (h *StockHandler) List(c fiber.Ctx) error {
	var fields []errorspkg.Field
	add := func(name string) { fields = append(fields, errorspkg.Field{Name: name, Message: msgInvalid}) }

	scope := c.Query("scope", StockScopeRisky)
	if !isValidStockScope(scope) {
		add("scope")
	}

	limit := stockDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > stockMaxLimit {
			add("limit")
		} else {
			limit = n
		}
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			add("offset")
		} else {
			offset = n
		}
	}
	if len(fields) > 0 {
		return errorspkg.Validation(fields...)
	}

	resp, err := h.svc.List(c.Context(), StockListParams{
		Scope: scope, Q: c.Query("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Summary — GET /api/v1/stock/summary.
func (h *StockHandler) Summary(c fiber.Ctx) error {
	resp, err := h.svc.Summary(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Adjust — PUT /api/v1/stock/:product_id (admin, D44).
func (h *StockHandler) Adjust(c fiber.Ctx) error {
	var req StockAdjustRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Adjust(c.Context(), c.Params("product_id"), currentUserID(c), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func isValidStockScope(s string) bool {
	for _, v := range StockScopes {
		if v == s {
			return true
		}
	}
	return false
}
