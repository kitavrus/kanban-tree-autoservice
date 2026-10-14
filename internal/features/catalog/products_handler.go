package catalog

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ProductHandler — JSON API /api/v1/products (чтение — RequireAPI, мутации — +admin).
type ProductHandler struct {
	svc *ProductService
}

func NewProductHandler(svc *ProductService) *ProductHandler { return &ProductHandler{svc: svc} }

// List — GET /api/v1/products?q=&include_archived=&limit=&offset=.
func (h *ProductHandler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeArchived, _ := strconv.ParseBool(c.Query("include_archived"))
	isAdmin := false
	if p := middleware.CurrentPrincipal(c); p != nil {
		isAdmin = p.IsAdmin()
	}
	resp, err := h.svc.List(c.Context(), ProductListQuery{
		Q: c.Query("q"), IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	}, isAdmin)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/products/:id.
func (h *ProductHandler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func inputFromCreate(req ProductCreateRequest) ProductInput {
	return ProductInput{
		Name: req.Name, SKU: req.SKU, Unit: req.Unit,
		Price: req.Price.String(), Stock: req.Stock.String(), MinStock: req.MinStock.String(),
		Box: req.Box, Place: req.Place,
	}
}

// Create — POST /api/v1/products (admin). is_active по умолчанию — true; остаток при
// создании в stock_movements не пишется (нет «старого» значения для дельты).
func (h *ProductHandler) Create(c fiber.Ctx) error {
	var req ProductCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), inputFromCreate(req))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/products/:id (admin; полная карточка, is_active обязателен).
func (h *ProductHandler) Update(c fiber.Ctx) error {
	var req ProductUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	if req.IsActive == nil {
		return errorspkg.Validation(errorspkg.Field{Name: "is_active", Message: msgRequired})
	}
	userID := ""
	if p := middleware.CurrentPrincipal(c); p != nil {
		userID = p.ID
	}
	in := ProductInput{
		Name: req.Name, SKU: req.SKU, Unit: req.Unit,
		Price: req.Price.String(), Stock: req.Stock.String(), MinStock: req.MinStock.String(),
		Box: req.Box, Place: req.Place, IsActive: *req.IsActive,
	}
	resp, err := h.svc.Update(c.Context(), c.Params("id"), userID, in)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
