package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/service"
)

// list — GET /api/v1/purchase-orders (§6.1).
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
func (h *Handler) get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// update — PUT /api/v1/purchase-orders/:id (§6.4).
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
func (h *Handler) send(c fiber.Ctx) error {
	return h.versioned(c, func(a service.Actor, id string, r dto.VersionRequest) (*dto.Order, error) {
		return h.svc.Send(c.Context(), a, id, r)
	})
}

// recall — POST /api/v1/purchase-orders/:id/recall (§6.7).
func (h *Handler) recall(c fiber.Ctx) error {
	return h.versioned(c, func(a service.Actor, id string, r dto.VersionRequest) (*dto.Order, error) {
		return h.svc.Recall(c.Context(), a, id, r)
	})
}

// pickUp — POST /api/v1/purchase-orders/:id/pick-up (§6.8).
func (h *Handler) pickUp(c fiber.Ctx) error {
	return h.versioned(c, func(a service.Actor, id string, r dto.VersionRequest) (*dto.Order, error) {
		return h.svc.PickUp(c.Context(), a, id, r)
	})
}

// cancel — POST /api/v1/purchase-orders/:id/cancel (§6.9).
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
func (h *Handler) restore(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.Restore(c.Context(), actor(c), id); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"id": id})
}

// suggestions — GET /api/v1/purchase-orders/suggestions (§6.10).
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
func (h *Handler) supplierGet(c fiber.Ctx) error {
	resp, err := h.svc.SupplierGet(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// supplierAnswer — PUT /api/v1/supplier/purchase-orders/:id/items/:item_id (§6.14).
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
func (h *Handler) supplierReady(c fiber.Ctx) error {
	resp, err := h.svc.Ready(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
