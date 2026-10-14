package warehouses

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API /api/v1/warehouses (только admin, DS-010/TS-014; гарды — routes.go).
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List — GET /api/v1/warehouses?include_archived=&limit=&offset=.
func (h *Handler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeArchived, _ := strconv.ParseBool(c.Query("include_archived"))
	resp, err := h.svc.List(c.Context(), ListQuery{IncludeArchived: includeArchived, Limit: limit, Offset: offset})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/warehouses.
func (h *Handler) Create(c fiber.Ctx) error {
	var req WarehouseCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), Input{Name: req.Name, IsActive: true})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/warehouses/:id (переименование/архив; is_active обязателен).
func (h *Handler) Update(c fiber.Ctx) error {
	var req WarehouseUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	if req.IsActive == nil {
		return errorspkg.Validation(errorspkg.Field{Name: "is_active", Message: msgRequired})
	}
	resp, err := h.svc.Update(c.Context(), c.Params("id"), Input{Name: req.Name, IsActive: *req.IsActive})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
