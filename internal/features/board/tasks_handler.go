package board

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// taskService — узкий интерфейс зависимости TasksHandler (не `*TaskService` напрямую):
// FuzzMoveBody (board_fuzz_test.go, §9.1) собирает app.Test-хендлер с фейковой
// реализацией без БД — TaskService (tasks_service.go) реализует его структурно, без явной
// декларации.
type taskService interface {
	List(ctx context.Context, f TaskFilter) (*TaskListResponse, error)
	Get(ctx context.Context, a Actor, idStr string) (*TaskResponse, error)
	Create(ctx context.Context, a Actor, in TaskCreateRequest) (*TaskMutationResponse, error)
	Update(ctx context.Context, a Actor, idStr string, in TaskUpdateRequest) (*TaskMutationResponse, error)
	Move(ctx context.Context, a Actor, idStr string, in TaskMoveRequest) (*TaskMutationResponse, error)
	Delete(ctx context.Context, a Actor, idStr string, in softdelete.DeleteRequest) (*TaskDeleteResponse, error)
	Restore(ctx context.Context, a Actor, idStr string) (*TaskRestoreResponse, error)
	Purge(ctx context.Context, a Actor, idStr string) error
}

// TasksHandler — JSON API /api/v1/tasks (чтение/мутации — RequireAPI, Delete — +admin;
// PAID_ADMIN_ONLY — сервисная проверка, не роут-гард).
type TasksHandler struct {
	svc taskService
	// purgers — служебная уборка видов других пакетов (WithPurger); nil — только заказы.
	purgers map[string]Purger
}

func NewTasksHandler(svc taskService) *TasksHandler { return &TasksHandler{svc: svc} }

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI — principal
// всегда не nil). Today — единственное место пакета, где вычисляется «сегодня» пользователя.
func currentActor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin(), Today: viewerToday(c)}
}

// viewerToday — текущая календарная дата (YYYY-MM-DD) в часовом поясе пользователя запроса
// (user-timezone DS-032): «сегодня / просрочен» считаются по ней, а не по дате сервера БД.
func viewerToday(c fiber.Ctx) string {
	return timezone.In(time.Now(), ssr.ViewerTimezone(c)).Format(time.DateOnly)
}

// List — GET /api/v1/tasks?<TaskFilter>.
func (h *TasksHandler) List(c fiber.Ctx) error {
	f, err := ParseTaskFilter(c, currentActor(c))
	if err != nil {
		return err
	}
	resp, err := h.svc.List(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Get — GET /api/v1/tasks/:id.
func (h *TasksHandler) Get(c fiber.Ctx) error {
	resp, err := h.svc.Get(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Create — POST /api/v1/tasks (D19 — доступно и executor).
func (h *TasksHandler) Create(c fiber.Ctx) error {
	var req TaskCreateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Create(c.Context(), currentActor(c), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Update — PUT /api/v1/tasks/:id (D19 — доступно и executor; version обязателен).
func (h *TasksHandler) Update(c fiber.Ctx) error {
	var req TaskUpdateRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Update(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Move — POST /api/v1/tasks/:id/move (D19 — доступно и executor).
func (h *TasksHandler) Move(c fiber.Ctx) error {
	var req TaskMoveRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("Некорректное тело запроса")
	}
	resp, err := h.svc.Move(c.Context(), currentActor(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Delete — DELETE /api/v1/tasks/:id (admin), тело {reason} — мягкое удаление (soft-delete-core
// §6.1): 200 + id, deleted_at, warnings (товары вернулись на склад). Тело без reason либо
// пустое → 400 reason: required (проверяет сервис).
func (h *TasksHandler) Delete(c fiber.Ctx) error {
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

// Restore — POST /api/v1/tasks/:id/restore (admin), тела нет (soft-delete-core §6.2).
func (h *TasksHandler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// purgeKindTasks — вид служебной уборки, которым владеет board (soft-delete-core §6.5).
const purgeKindTasks = "tasks"

// Purger — физическая уборка записи одного вида по идентификатору из URL (служебный адрес
// автотестов). Виды других пакетов подключаются WithPurger: board их не импортирует.
type Purger func(ctx context.Context, id string) error

// WithPurger — подключить уборку вида kind (виды других пакетов soft-delete).
func (h *TasksHandler) WithPurger(kind string, p Purger) *TasksHandler {
	if h.purgers == nil {
		h.purgers = map[string]Purger{}
	}
	h.purgers[kind] = p
	return h
}

// Purge — DELETE /api/v1/test/purge/:kind/:id (admin): служебная физическая уборка тестовых
// данных автотестами. Роут регистрируется только при TEST_PURGE_ENABLED=true (DS-027).
// Вид tasks — заказ; прочие виды — подключённые WithPurger; неизвестный
// вид → 404 NOT_FOUND.
func (h *TasksHandler) Purge(c fiber.Ctx) error {
	id := c.Params("id")
	switch kind := c.Params("kind"); {
	case kind == purgeKindTasks:
		if err := h.svc.Purge(c.Context(), currentActor(c), id); err != nil {
			return err
		}
	case h.purgers[kind] != nil:
		if err := h.purgers[kind](c.Context(), id); err != nil {
			return err
		}
	default:
		return errPurgeNotFound()
	}
	return c.JSON(PurgeResponse{ID: id})
}
