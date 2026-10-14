package executorpayouts

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API /api/v1/executor-payouts: доступно и admin, и executor (D62).
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actorOf(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin()}
}

func listQuery(c fiber.Ctx) ListQuery {
	return ListQuery{
		Assignee: c.Query("assignee"), Status: c.Query("status"), OnlyDone: c.Query("only_done"),
		Limit: c.Query("limit"), Offset: c.Query("offset"),
	}
}

// List — GET /api/v1/executor-payouts?assignee=&status=&only_done=&limit=&offset=.
func (h *Handler) List(c fiber.Ctx) error {
	resp, err := h.svc.List(c.Context(), actorOf(c), listQuery(c))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Summary — GET /api/v1/executor-payouts/summary (те же фильтры).
func (h *Handler) Summary(c fiber.Ctx) error {
	resp, err := h.svc.Summary(c.Context(), actorOf(c), listQuery(c))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Mark — POST /api/v1/executor-payouts/mark.
func (h *Handler) Mark(c fiber.Ctx) error {
	var req MarkRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Mark(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// UpdatePayout — PUT /api/v1/executor-payouts/:id (id — uuid услуги).
func (h *Handler) UpdatePayout(c fiber.Ctx) error {
	var req PayoutUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.UpdatePayout(c.Context(), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
