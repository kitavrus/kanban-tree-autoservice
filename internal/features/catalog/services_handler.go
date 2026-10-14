package catalog

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ServiceHandler — JSON API /api/v1/services (чтение — RequireAPI, мутации — +admin).
type ServiceHandler struct {
	svc *ServiceService
}

func NewServiceHandler(svc *ServiceService) *ServiceHandler { return &ServiceHandler{svc: svc} }

// List — GET /api/v1/services?category_id=&q=&include_archived=&limit=&offset=.
func (h *ServiceHandler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeArchived, _ := strconv.ParseBool(c.Query("include_archived"))
	isAdmin := false
	if p := middleware.CurrentPrincipal(c); p != nil {
		isAdmin = p.IsAdmin()
	}
	resp, err := h.svc.List(c.Context(), ServiceListQuery{
		CategoryID: c.Query("category_id"), Q: c.Query("q"), IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	}, isAdmin)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/services/:id.
func (h *ServiceHandler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func productsFromRequest(items []ServiceProductRequest) []ServiceProductInput {
	out := make([]ServiceProductInput, len(items))
	for i, it := range items {
		out[i] = ServiceProductInput{ProductID: it.ProductID, Qty: it.Qty.String()}
	}
	return out
}

// Create — POST /api/v1/services (admin). is_active по умолчанию — true.
func (h *ServiceHandler) Create(c fiber.Ctx) error {
	var req ServiceCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	in := ServiceInput{CategoryID: req.CategoryID, Name: req.Name, Products: productsFromRequest(req.Products)}
	if req.Price != nil {
		in.HasPrice, in.Price = true, req.Price.String()
	}
	resp, err := h.svc.Create(c.Context(), in)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/services/:id (admin; полная карточка+комплект, is_active обязателен).
func (h *ServiceHandler) Update(c fiber.Ctx) error {
	var req ServiceUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	if req.IsActive == nil {
		return errorspkg.Validation(errorspkg.Field{Name: "is_active", Message: msgRequired})
	}
	in := ServiceInput{
		CategoryID: req.CategoryID, Name: req.Name, Products: productsFromRequest(req.Products), IsActive: *req.IsActive,
	}
	if req.Price != nil {
		in.HasPrice, in.Price = true, req.Price.String()
	}
	resp, err := h.svc.Update(c.Context(), c.Params("id"), in)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
