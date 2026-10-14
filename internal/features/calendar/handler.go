package calendar

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// Handler — JSON API /api/v1/calendar: смотреть — все авторизованные, настраивать — admin.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func viewerTZ(c fiber.Ctx) string {
	if p := middleware.CurrentPrincipal(c); p != nil && p.Timezone != "" {
		return p.Timezone
	}
	return timezone.Default
}

// Calendar — GET /api/v1/calendar?from=YYYY-MM-DD&to=YYYY-MM-DD (по умолчанию — текущий месяц).
func (h *Handler) Calendar(c fiber.Ctx) error {
	tz := viewerTZ(c)
	from, to, err := ParseRange(c.Query("from"), c.Query("to"), h.svc.Today(tz))
	if err != nil {
		return err
	}
	resp, err := h.svc.Calendar(c.Context(), tz, from, to)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// GetSettings — GET /api/v1/calendar/settings.
func (h *Handler) GetSettings(c fiber.Ctx) error {
	s, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(s)
}

// PutSettings — PUT /api/v1/calendar/settings {max_cars} (только admin).
func (h *Handler) PutSettings(c fiber.Ctx) error {
	var req SettingsRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	s, err := h.svc.SaveSettings(c.Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(s)
}
