package board

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// cardService — узкий интерфейс зависимости CardHandler (эталон taskService,
// tasks_handler.go): карточка + row-методы услуг/товаров/журнала.
type cardService interface {
	Card(ctx context.Context, a Actor, taskID string) (*OrderCardResponse, error)
	CreateSubtask(ctx context.Context, a Actor, taskID string, in SubtaskCreateRequest) (*OrderCardMutationResponse, error)
	UpdateSubtask(ctx context.Context, a Actor, subtaskID string, in SubtaskUpdateRequest) (*OrderCardMutationResponse, error)
	DeleteSubtask(ctx context.Context, a Actor, subtaskID string) (*OrderCardMutationResponse, error)
	CreateSubtaskProduct(ctx context.Context, a Actor, subtaskID string, in SubtaskProductCreateRequest) (*OrderCardMutationResponse, error)
	UpdateSubtaskProduct(ctx context.Context, a Actor, id string, in SubtaskProductUpdateRequest) (*OrderCardMutationResponse, error)
	DeleteSubtaskProduct(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
	CreateLog(ctx context.Context, a Actor, subtaskID string, in SubtaskLogRequest) (*OrderCardMutationResponse, error)
	UpdateLog(ctx context.Context, a Actor, id string, in SubtaskLogRequest) (*OrderCardMutationResponse, error)
	DeleteLog(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
}

// CardHandler — карточка заказа и её строки (D19 — доступно и executor, admin-гардов нет).
type CardHandler struct {
	svc cardService
}

func NewCardHandler(svc cardService) *CardHandler { return &CardHandler{svc: svc} }

// Card — GET /api/v1/tasks/:id/card.
func (h *CardHandler) Card(c fiber.Ctx) error {
	resp, err := h.svc.Card(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// CreateSubtask — POST /api/v1/tasks/:id/subtasks.
func (h *CardHandler) CreateSubtask(c fiber.Ctx) error {
	var req SubtaskCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.CreateSubtask(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// UpdateSubtask — PUT /api/v1/subtasks/:id.
func (h *CardHandler) UpdateSubtask(c fiber.Ctx) error {
	var req SubtaskUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.UpdateSubtask(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// DeleteSubtask — DELETE /api/v1/subtasks/:id.
func (h *CardHandler) DeleteSubtask(c fiber.Ctx) error {
	resp, err := h.svc.DeleteSubtask(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// CreateSubtaskProduct — POST /api/v1/subtasks/:id/products.
func (h *CardHandler) CreateSubtaskProduct(c fiber.Ctx) error {
	var req SubtaskProductCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.CreateSubtaskProduct(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// UpdateSubtaskProduct — PUT /api/v1/subtask-products/:id.
func (h *CardHandler) UpdateSubtaskProduct(c fiber.Ctx) error {
	var req SubtaskProductUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.UpdateSubtaskProduct(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// DeleteSubtaskProduct — DELETE /api/v1/subtask-products/:id.
func (h *CardHandler) DeleteSubtaskProduct(c fiber.Ctx) error {
	resp, err := h.svc.DeleteSubtaskProduct(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// CreateLog — POST /api/v1/subtasks/:id/logs.
func (h *CardHandler) CreateLog(c fiber.Ctx) error {
	var req SubtaskLogRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.CreateLog(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// UpdateLog — PUT /api/v1/subtask-logs/:id.
func (h *CardHandler) UpdateLog(c fiber.Ctx) error {
	var req SubtaskLogRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.UpdateLog(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// DeleteLog — DELETE /api/v1/subtask-logs/:id.
func (h *CardHandler) DeleteLog(c fiber.Ctx) error {
	resp, err := h.svc.DeleteLog(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
