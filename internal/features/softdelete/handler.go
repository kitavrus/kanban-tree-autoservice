package softdelete

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Handler — JSON API раздела «Удалённые» (только admin — гард на группе роутов, DS-003).
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List — GET /api/v1/deleted?kind=&q=&deleted_from=&deleted_to=&deleted_by=&limit=&offset=.
func (h *Handler) List(c fiber.Ctx) error {
	resp, err := h.svc.List(c.Context(), ssr.ViewerTimezone(c), ListQuery{
		Kind: c.Query("kind"), Q: c.Query("q"),
		DeletedFrom: c.Query("deleted_from"), DeletedTo: c.Query("deleted_to"),
		DeletedBy: c.Query("deleted_by"), Limit: c.Query("limit"), Offset: c.Query("offset"),
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Summary — GET /api/v1/deleted/summary (лишние параметры игнорируются).
func (h *Handler) Summary(c fiber.Ctx) error {
	resp, err := h.svc.Summary(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
