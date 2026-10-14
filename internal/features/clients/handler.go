package clients

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API /api/v1/clients (чтение — RequireAPI, мутации — +admin, гарды
// вешает routes.go). Equipment-методы (ListEquipment/CreateEquipment/GetEquipment/
// UpdateEquipment/DeleteEquipment) — Phase 7.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List — GET /api/v1/clients?q=&limit=&offset=.
func (h *Handler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	resp, err := h.svc.List(c.Context(), ListQuery{Q: c.Query("q"), Limit: limit, Offset: offset})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/clients/:id.
func (h *Handler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/clients (admin).
func (h *Handler) Create(c fiber.Ctx) error {
	var req ClientCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), ClientInput{
		CompanyName: req.CompanyName, Address: req.Address,
		ContactName: req.ContactName, ContactPosition: req.ContactPosition, Phones: req.Phones,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/clients/:id (admin; полная карточка + телефоны, version обязателен).
func (h *Handler) Update(c fiber.Ctx) error {
	var req ClientUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Update(c.Context(), c.Params("id"), req.Version, ClientInput{
		CompanyName: req.CompanyName, Address: req.Address,
		ContactName: req.ContactName, ContactPosition: req.ContactPosition, Phones: req.Phones,
	})
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI/RequireSSR —
// principal всегда не nil).
func currentActor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin()}
}

// deleteRequest — тело DELETE разбирается как JSON независимо от Content-Type; нечитаемое либо
// пустое тело — то же, что отсутствие причины: 400 VALIDATION reason: required (сервис).
func deleteRequest(c fiber.Ctx) softdelete.DeleteRequest {
	var req softdelete.DeleteRequest
	if body := bytes.TrimSpace(c.Body()); len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			req = softdelete.DeleteRequest{}
		}
	}
	return req
}

// Delete — DELETE /api/v1/clients/:id (admin), тело {reason} — мягкое удаление клиента вместе
// с его оборудованием (soft-delete-clients §6.1): 200 + id, deleted_at, equipment_deleted.
func (h *Handler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), deleteRequest(c))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Restore — POST /api/v1/clients/:id/restore (admin), тела нет (§6.2).
func (h *Handler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// ListEquipment — GET /api/v1/clients/:id/equipment (RequireAPI, D19: без admin-guard).
func (h *Handler) ListEquipment(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	resp, err := h.svc.ListEquipment(c.Context(), c.Params("id"), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// CreateEquipment — POST /api/v1/clients/:id/equipment (RequireAPI, D19: исполнитель
// допущен создавать оборудование).
func (h *Handler) CreateEquipment(c fiber.Ctx) error {
	var req EquipmentRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.CreateEquipment(c.Context(), c.Params("id"), equipmentInputOf(req))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// GetEquipment — GET /api/v1/equipment/:id (RequireAPI).
func (h *Handler) GetEquipment(c fiber.Ctx) error {
	resp, err := h.svc.GetEquipment(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// UpdateEquipment — PUT /api/v1/equipment/:id (admin; полная замена, LWW).
func (h *Handler) UpdateEquipment(c fiber.Ctx) error {
	var req EquipmentRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.UpdateEquipment(c.Context(), c.Params("id"), equipmentInputOf(req))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// DeleteEquipment — DELETE /api/v1/equipment/:id (admin), тело {reason} — мягкое удаление
// единицы (§6.3): 200 + id, deleted_at.
func (h *Handler) DeleteEquipment(c fiber.Ctx) error {
	resp, _, err := h.svc.DeleteEquipment(c.Context(), currentActor(c), c.Params("id"), deleteRequest(c))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// RestoreEquipment — POST /api/v1/equipment/:id/restore (admin), тела нет (§6.4).
func (h *Handler) RestoreEquipment(c fiber.Ctx) error {
	resp, err := h.svc.RestoreEquipment(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func equipmentInputOf(req EquipmentRequest) EquipmentInput {
	return EquipmentInput{
		TypeID: req.TypeID, Brand: req.Brand, Model: req.Model, SKU: req.SKU,
		Serial: req.Serial, Note: req.Note, Specs: req.Specs,
	}
}
