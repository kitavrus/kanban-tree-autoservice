package transfers

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API /api/v1/transfers: любой авторизованный; права по направлениям
// (исполнитель — только документы с клиентом, DS-018/DS-042) держит сервис.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// actor — пользователь запроса: роль и часовой пояс (границы дня фильтра «период», DS-034).
func actor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin(), Timezone: ssr.ViewerTimezone(c)}
}

func badBody() error { return errorspkg.BadRequest("Некорректное тело запроса") }

// List — GET /api/v1/transfers?status=&warehouse_id=&direction=&task_id=&supplier_id=
// &created_from=&created_to=&limit=&offset=.
func (h *Handler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	resp, err := h.svc.List(c.Context(), actor(c), ListQuery{
		Status: c.Query("status"), WarehouseID: c.Query("warehouse_id"), Direction: c.Query("direction"),
		SupplierID: c.Query("supplier_id"), TaskID: c.Query("task_id"),
		CreatedFrom: c.Query("created_from"), CreatedTo: c.Query("created_to"),
		Limit: limit, Offset: offset,
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/transfers/:id.
func (h *Handler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/transfers.
func (h *Handler) Create(c fiber.Ctx) error {
	var req TransferCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return badBody()
	}
	resp, err := h.svc.Create(c.Context(), actor(c), Input{
		Direction: req.Direction, FromWarehouseID: req.FromWarehouseID, ToWarehouseID: req.ToWarehouseID,
		TaskID: req.TaskID, SupplierID: req.SupplierID, Note: req.Note, Items: req.Items,
	}, req.Send, req.Receive)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/transfers/:id (только заявка).
func (h *Handler) Update(c fiber.Ctx) error {
	var req TransferUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return badBody()
	}
	resp, err := h.svc.Update(c.Context(), actor(c), c.Params("id"), Input{
		FromWarehouseID: req.FromWarehouseID, ToWarehouseID: req.ToWarehouseID,
		SupplierID: req.SupplierID, Note: req.Note, Items: req.Items,
	}, req.Direction, req.TaskID, req.Version)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Send — POST /api/v1/transfers/:id/send.
func (h *Handler) Send(c fiber.Ctx) error {
	var req TransferActionRequest
	if err := c.Bind().Body(&req); err != nil {
		return badBody()
	}
	resp, err := h.svc.Send(c.Context(), actor(c), c.Params("id"), req.Version)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Receive — POST /api/v1/transfers/:id/receive.
func (h *Handler) Receive(c fiber.Ctx) error {
	var req TransferReceiveRequest
	if err := c.Bind().Body(&req); err != nil {
		return badBody()
	}
	resp, err := h.svc.Receive(c.Context(), actor(c), c.Params("id"), req.Version, req.Items)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Cancel — POST /api/v1/transfers/:id/cancel.
func (h *Handler) Cancel(c fiber.Ctx) error {
	var req TransferActionRequest
	if err := c.Bind().Body(&req); err != nil {
		return badBody()
	}
	resp, err := h.svc.Cancel(c.Context(), actor(c), c.Params("id"), req.Version)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Unreceive — POST /api/v1/transfers/:id/unreceive («Отменить проведение», только admin).
func (h *Handler) Unreceive(c fiber.Ctx) error {
	var req TransferUnreceiveRequest
	if err := c.Bind().Body(&req); err != nil {
		return badBody()
	}
	resp, err := h.svc.Unreceive(c.Context(), actor(c), c.Params("id"), req.Version, req.Reason)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Delete — DELETE /api/v1/transfers/:id, тело {reason, version} (только admin; «Заявка» /
// «Отменён»): мягкое удаление (soft-delete-transfers §6.1).
func (h *Handler) Delete(c fiber.Ctx) error {
	// Тело разбирается как JSON независимо от Content-Type; нечитаемое тело — то же, что
	// отсутствие полей: единый ответ 400 VALIDATION с details.fields[] (как у заказа, часть 1).
	var req TransferDeleteRequest
	if body := bytes.TrimSpace(c.Body()); len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			req = TransferDeleteRequest{}
		}
	}
	resp, err := h.svc.Delete(c.Context(), actor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Restore — POST /api/v1/transfers/:id/restore (только admin): вернуть документ из раздела
// «Удалённые». Тела нет.
func (h *Handler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
