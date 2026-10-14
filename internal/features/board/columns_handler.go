package board

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ColumnsHandler — JSON API /api/v1/columns (чтение — RequireAPI, мутации — +admin,
// гарды вешает routes.go, эталон internal/features/clients/handler.go).
type ColumnsHandler struct {
	svc *ColumnService
}

func NewColumnsHandler(svc *ColumnService) *ColumnsHandler { return &ColumnsHandler{svc: svc} }

// List — GET /api/v1/columns?limit=&offset=. limit/offset — тот же паттерн явной 400-
// валидации, что у tasks_filter.go/stock_handler.go (code review mobile M-01: мусор/выход
// за диапазон → 400 field, а не тихий clamp к дефолту).
func (h *ColumnsHandler) List(c fiber.Ctx) error {
	var fields []errorspkg.Field
	add := func(name string) { fields = append(fields, errorspkg.Field{Name: name, Message: msgInvalid}) }

	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > columnsMaxLimit {
			add("limit")
		} else {
			limit = n
		}
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			add("offset")
		} else {
			offset = n
		}
	}
	if len(fields) > 0 {
		return errorspkg.Validation(fields...)
	}

	resp, err := h.svc.List(c.Context(), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/columns (admin).
func (h *ColumnsHandler) Create(c fiber.Ctx) error {
	var req ColumnInput
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), currentUserID(c), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/columns/:id (admin); смена consumes_stock пересчитывает склад.
func (h *ColumnsHandler) Update(c fiber.Ctx) error {
	var req ColumnInput
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Update(c.Context(), c.Params("id"), currentUserID(c), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Delete — DELETE /api/v1/columns/:id (admin), тело {reason} — мягкое удаление
// (soft-delete-board-columns §6.1): 200 + id, deleted_at; склад не затрагивается. Тело без
// reason либо пустое → 400 reason: required (проверяет сервис).
func (h *ColumnsHandler) Delete(c fiber.Ctx) error {
	// Тело разбирается как JSON независимо от Content-Type; нечитаемое тело — то же, что
	// отсутствие причины: единый ответ 400 VALIDATION с details.fields[] (reason: required).
	var req softdelete.DeleteRequest
	if body := bytes.TrimSpace(c.Body()); len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			req = softdelete.DeleteRequest{}
		}
	}
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Restore — POST /api/v1/columns/:id/restore (admin), тела нет (soft-delete-board-columns §6.2).
func (h *ColumnsHandler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Purge — служебная физическая уборка колонки (вид columns адреса
// DELETE /api/v1/test/purge/:kind/:id, DS-027); подключается TasksHandler.WithPurger.
func (h *ColumnsHandler) Purge(ctx context.Context, id string) error { return h.svc.Purge(ctx, id) }

// currentUserID — Actor.ID (design-go-layers.md §Service Interface: middleware.CurrentPrincipal,
// эталон catalog/products_handler.go). Роут закрыт RequireAPI — principal всегда не nil.
func currentUserID(c fiber.Ctx) string {
	return middleware.CurrentPrincipal(c).ID
}
