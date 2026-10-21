package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/service"
)

// list — GET /api/v1/purchase-orders (§6.1).
//
// @Summary  Список закупок (§6.1)
// @Tags     purchase-orders
// @Produce  json
// @Param    status query string false "Статус" Enums(draft, sent, ready, picked_up, cancelled)
// @Param    limit  query int    false "1–100, по умолчанию 50"
// @Param    offset query int    false ">= 0"
// @Success  200 {object} dto.ListResponse
// @Failure  400 {object} ErrorResponse
// @Failure  403 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders [get]
func (h *Handler) list(c fiber.Ctx) error {
	resp, err := h.svc.List(c.Context(), actor(c), service.ListQuery{
		Status: c.Query("status"), Limit: c.Query("limit"), Offset: c.Query("offset"),
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// create — POST /api/v1/purchase-orders (§6.2): 201 + карточка.
//
// @Summary  Создать черновик (§6.2)
// @Tags     purchase-orders
// @Accept   json
// @Produce  json
// @Param    body body dto.CreateRequest true "Черновик"
// @Success  201 {object} dto.Order
// @Failure  400 {object} ErrorResponse
// @Failure  403 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders [post]
func (h *Handler) create(c fiber.Ctx) error {
	var req dto.CreateRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	resp, err := h.svc.Create(c.Context(), actor(c), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// get — GET /api/v1/purchase-orders/:id (§6.3).
//
// @Summary  Карточка закупки (§6.3)
// @Tags     purchase-orders
// @Produce  json
// @Param    id path string true "id заказа"
// @Success  200 {object} dto.Order
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id} [get]
func (h *Handler) get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// update — PUT /api/v1/purchase-orders/:id (§6.4).
//
// @Summary  Правка черновика/отправленного заказа (§6.4)
// @Tags     purchase-orders
// @Accept   json
// @Produce  json
// @Param    id   path string            true "id заказа"
// @Param    body body dto.UpdateRequest true "Примечание и строки"
// @Success  200 {object} dto.Order
// @Failure  400 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id} [put]
func (h *Handler) update(c fiber.Ctx) error {
	var req dto.UpdateRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	resp, err := h.svc.Update(c.Context(), actor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// delete — DELETE /api/v1/purchase-orders/:id (§6.5): 204.
//
// @Summary  Мягко удалить черновик (§6.5, только admin)
// @Tags     purchase-orders
// @Accept   json
// @Param    id   path string            true "id заказа"
// @Param    body body dto.ReasonRequest true "Версия и причина"
// @Success  204
// @Failure  400 {object} ErrorResponse
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id} [delete]
func (h *Handler) delete(c fiber.Ctx) error {
	var req dto.ReasonRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if err := h.svc.Delete(c.Context(), actor(c), c.Params("id"), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) versioned(c fiber.Ctx, run func(service.Actor, string, dto.VersionRequest) (*dto.Order, error)) error {
	var req dto.VersionRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	resp, err := run(actor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// send — POST /api/v1/purchase-orders/:id/send (§6.6).
//
// @Summary  Отправить поставщику (§6.6)
// @Tags     purchase-orders
// @Accept   json
// @Produce  json
// @Param    id   path string             true "id заказа"
// @Param    body body dto.VersionRequest true "Версия"
// @Success  200 {object} dto.Order
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id}/send [post]
func (h *Handler) send(c fiber.Ctx) error {
	return h.versioned(c, func(a service.Actor, id string, r dto.VersionRequest) (*dto.Order, error) {
		return h.svc.Send(c.Context(), a, id, r)
	})
}

// recall — POST /api/v1/purchase-orders/:id/recall (§6.7).
//
// @Summary  Отозвать (§6.7)
// @Tags     purchase-orders
// @Accept   json
// @Produce  json
// @Param    id   path string             true "id заказа"
// @Param    body body dto.VersionRequest true "Версия"
// @Success  200 {object} dto.Order
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id}/recall [post]
func (h *Handler) recall(c fiber.Ctx) error {
	return h.versioned(c, func(a service.Actor, id string, r dto.VersionRequest) (*dto.Order, error) {
		return h.svc.Recall(c.Context(), a, id, r)
	})
}

// pickUp — POST /api/v1/purchase-orders/:id/pick-up (§6.8).
//
// @Summary  «Забрали» (§6.8)
// @Tags     purchase-orders
// @Accept   json
// @Produce  json
// @Param    id   path string             true "id заказа"
// @Param    body body dto.VersionRequest true "Версия"
// @Success  200 {object} dto.Order
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id}/pick-up [post]
func (h *Handler) pickUp(c fiber.Ctx) error {
	return h.versioned(c, func(a service.Actor, id string, r dto.VersionRequest) (*dto.Order, error) {
		return h.svc.PickUp(c.Context(), a, id, r)
	})
}

// cancel — POST /api/v1/purchase-orders/:id/cancel (§6.9).
//
// @Summary  Отменить с причиной (§6.9)
// @Tags     purchase-orders
// @Accept   json
// @Produce  json
// @Param    id   path string            true "id заказа"
// @Param    body body dto.ReasonRequest true "Версия и причина"
// @Success  200 {object} dto.Order
// @Failure  400 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id}/cancel [post]
func (h *Handler) cancel(c fiber.Ctx) error {
	var req dto.ReasonRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	resp, err := h.svc.Cancel(c.Context(), actor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// restore — POST /api/v1/purchase-orders/:id/restore (раздел «Удалённые», admin, DS-072).
//
// @Summary  Восстановить удалённый черновик (DS-072, только admin)
// @Tags     purchase-orders
// @Produce  json
// @Param    id path string true "id заказа"
// @Success  200 {object} RestoreResponse
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/{id}/restore [post]
func (h *Handler) restore(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.Restore(c.Context(), actor(c), id); err != nil {
		return err
	}
	return c.JSON(RestoreResponse{ID: id})
}

// suggestions — GET /api/v1/purchase-orders/suggestions (§6.10).
//
// @Summary  Строки дефицита для «Новый заказ» (§6.10)
// @Tags     purchase-orders
// @Produce  json
// @Param    reason query string false "Тип строки" Enums(for_orders, below_min)
// @Param    limit  query int    false "1–500, по умолчанию 500"
// @Param    offset query int    false ">= 0"
// @Success  200 {object} dto.SuggestionsResponse
// @Failure  400 {object} ErrorResponse
// @Failure  403 {object} ErrorResponse
// @Security CookieSession
// @Router   /purchase-orders/suggestions [get]
func (h *Handler) suggestions(c fiber.Ctx) error {
	resp, err := h.svc.Suggestions(c.Context(), actor(c), service.SuggestionsQuery{
		Reason: c.Query("reason"), Limit: c.Query("limit"), Offset: c.Query("offset"),
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// ---- кабинет поставщика ----

// supplierList — GET /api/v1/supplier/purchase-orders (§6.12).
//
// @Summary  Кабинет поставщика: список заказов (§6.12)
// @Tags     supplier-cabinet
// @Produce  json
// @Param    status query string false "Статус" Enums(sent, ready, picked_up)
// @Param    limit  query int    false "1–100, по умолчанию 50"
// @Param    offset query int    false ">= 0"
// @Success  200 {object} dto.SupplierListResponse
// @Failure  400 {object} ErrorResponse
// @Failure  403 {object} ErrorResponse
// @Security CookieSession
// @Router   /supplier/purchase-orders [get]
func (h *Handler) supplierList(c fiber.Ctx) error {
	resp, err := h.svc.SupplierList(c.Context(), actor(c), service.ListQuery{
		Status: c.Query("status"), Limit: c.Query("limit"), Offset: c.Query("offset"),
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// supplierGet — GET /api/v1/supplier/purchase-orders/:id (§6.13).
//
// @Summary  Кабинет поставщика: карточка заказа (§6.13)
// @Tags     supplier-cabinet
// @Produce  json
// @Param    id path string true "id заказа"
// @Success  200 {object} dto.SupplierOrder
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Security CookieSession
// @Router   /supplier/purchase-orders/{id} [get]
func (h *Handler) supplierGet(c fiber.Ctx) error {
	resp, err := h.svc.SupplierGet(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// supplierAnswer — PUT /api/v1/supplier/purchase-orders/:id/items/:item_id (§6.14).
//
// @Summary  Кабинет поставщика: ответ по строке (§6.14)
// @Tags     supplier-cabinet
// @Accept   json
// @Produce  json
// @Param    id      path string            true "id заказа"
// @Param    item_id path string            true "id строки"
// @Param    body    body dto.AnswerRequest true "Ответ"
// @Success  200 {object} dto.AnswerResponse
// @Failure  400 {object} ErrorResponse
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /supplier/purchase-orders/{id}/items/{item_id} [put]
func (h *Handler) supplierAnswer(c fiber.Ctx) error {
	var req dto.AnswerRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	resp, err := h.svc.Answer(c.Context(), actor(c), c.Params("id"), c.Params("item_id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// supplierReady — POST /api/v1/supplier/purchase-orders/:id/ready (§6.15).
//
// @Summary  Кабинет поставщика: «Собрано, можно забирать» (§6.15)
// @Tags     supplier-cabinet
// @Produce  json
// @Param    id path string true "id заказа"
// @Success  200 {object} dto.SupplierOrder
// @Failure  403 {object} ErrorResponse
// @Failure  404 {object} ErrorResponse
// @Failure  409 {object} ErrorResponse
// @Security CookieSession
// @Router   /supplier/purchase-orders/{id}/ready [post]
func (h *Handler) supplierReady(c fiber.Ctx) error {
	resp, err := h.svc.Ready(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// ErrorResponse — wire-формат ошибки pkg/errorspkg (только для swagger): 400 VALIDATION несёт
// details.fields[], 409 «Собрано» — details.item_ids.
type ErrorResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// RestoreResponse — ответ POST /api/v1/purchase-orders/:id/restore.
type RestoreResponse struct {
	ID string `json:"id"`
}
