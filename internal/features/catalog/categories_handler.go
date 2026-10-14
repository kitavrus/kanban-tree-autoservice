package catalog

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// CategoryHandler — JSON API /api/v1/catalog-categories (чтение — RequireAPI, мутации —
// +admin, гарды вешает routes.go).
type CategoryHandler struct {
	svc *CategoryService
}

func NewCategoryHandler(svc *CategoryService) *CategoryHandler { return &CategoryHandler{svc: svc} }

// List — GET /api/v1/catalog-categories?kind=&include_archived=&limit=&offset=.
func (h *CategoryHandler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	includeArchived, _ := strconv.ParseBool(c.Query("include_archived"))
	isAdmin := false
	if p := middleware.CurrentPrincipal(c); p != nil {
		isAdmin = p.IsAdmin()
	}
	resp, err := h.svc.List(c.Context(), CategoryListQuery{
		Kind: c.Query("kind"), IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	}, isAdmin)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/catalog-categories/:id.
func (h *CategoryHandler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/catalog-categories (admin).
func (h *CategoryHandler) Create(c fiber.Ctx) error {
	var req CategoryCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), CategoryInput{Kind: req.Kind, Name: req.Name})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/catalog-categories/:id (admin; переименование).
func (h *CategoryHandler) Update(c fiber.Ctx) error {
	var req CategoryUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Rename(c.Context(), c.Params("id"), req.Name)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
