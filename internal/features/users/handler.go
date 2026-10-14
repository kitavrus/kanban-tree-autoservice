package users

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API /api/v1/users (только admin — гард вешает routes.go; исключение —
// SetMyTimezone, доступный любому авторизованному).
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List — GET /api/v1/users?q=&role=&is_active=&sort=&order=&limit=&offset=.
func (h *Handler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	resp, err := h.svc.List(c.Context(), ListQuery{
		Q: c.Query("q"), Role: c.Query("role"), IsActive: parseBoolPtr(c.Query("is_active")),
		Sort: c.Query("sort"), Order: c.Query("order"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/users/:id.
func (h *Handler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/users.
func (h *Handler) Create(c fiber.Ctx) error {
	var req CreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("invalid body")
	}
	active := req.IsActive == nil || *req.IsActive
	resp, err := h.svc.Create(c.Context(), Input{
		Login: req.Login, Name: req.Name, Role: req.Role, Phone: req.Phone, Password: req.Password, IsActive: active,
		Timezone: req.Timezone,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/users/:id (полная карточка; is_active обязателен).
func (h *Handler) Update(c fiber.Ctx) error {
	var req UpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("invalid body")
	}
	if req.IsActive == nil {
		return errorspkg.Validation(errorspkg.Field{Name: "is_active", Message: msgRequired})
	}
	resp, err := h.svc.Update(c.Context(), actorOf(c), c.Params("id"), Input{
		Login: req.Login, Name: req.Name, Role: req.Role, Phone: req.Phone, Password: req.Password, IsActive: *req.IsActive,
		Timezone: req.Timezone,
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// SetMyTimezone — PUT /api/v1/users/me/timezone: любой авторизованный меняет СВОЙ пояс
// (DS-033). id — из сессии; user_id и прочие ключи тела игнорируются.
func (h *Handler) SetMyTimezone(c fiber.Ctx) error {
	// encoding/json напрямую, не c.Bind(): binder Fiber отклоняет тело с посторонними
	// ключами, а чужой user_id по спеке (abuse case 2) должен молча игнорироваться.
	var req TimezoneRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return errorspkg.BadRequest("invalid body")
	}
	resp, err := h.svc.SetTimezone(c.Context(), actorOf(c).UserID, req.Timezone)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func actorOf(c fiber.Ctx) Actor {
	uid, _ := c.Locals(middleware.UserIDKey).(string)
	sid, _ := c.Locals(middleware.SessionIDKey).(string)
	return Actor{UserID: uid, SessionID: sid}
}

// parseBoolPtr — пусто/мусор = «любой» (nil).
func parseBoolPtr(raw string) *bool {
	if v, err := strconv.ParseBool(raw); err == nil {
		return &v
	}
	return nil
}
