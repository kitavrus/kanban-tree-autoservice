package equipmenttypes

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API /api/v1/equipment-types (чтение — RequireAPI, мутации — +admin,
// гарды вешает routes.go).
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List — GET /api/v1/equipment-types?q=&include_archived=&limit=&offset=.
// include_archived для не-admin форсируется в false сервисом (ADR-004).
func (h *Handler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeArchived, _ := strconv.ParseBool(c.Query("include_archived"))
	isAdmin := false
	if p := middleware.CurrentPrincipal(c); p != nil {
		isAdmin = p.IsAdmin()
	}
	resp, err := h.svc.List(c.Context(), ListQuery{
		Q: c.Query("q"), IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	}, isAdmin)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/equipment-types/:id.
func (h *Handler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/equipment-types (admin). is_active по умолчанию — true.
func (h *Handler) Create(c fiber.Ctx) error {
	var req TypeCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), Input{Name: req.Name, SpecKeys: req.SpecKeys, IsActive: true})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/equipment-types/:id (admin; полная карточка, is_active обязателен).
func (h *Handler) Update(c fiber.Ctx) error {
	var req TypeUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	if req.IsActive == nil {
		return errorspkg.Validation(errorspkg.Field{Name: "is_active", Message: msgRequired})
	}
	resp, err := h.svc.Update(c.Context(), c.Params("id"), Input{Name: req.Name, SpecKeys: req.SpecKeys, IsActive: *req.IsActive})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
