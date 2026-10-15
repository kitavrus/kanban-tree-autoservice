package board

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Handlers — группировка хендлеров фичи (design-go-layers.md §Package Layout).
type Handlers struct {
	Columns   *ColumnsHandler
	Tasks     *TasksHandler
	Assignees *AssigneesHandler
	Stock     *StockHandler
	Card      *CardHandler
	// Attachments — вложения заказа (фаза 7); nil — роуты вложений не регистрируются.
	Attachments *AttachmentsHandler
	// PurgeEnabled — регистрировать служебную уборку DELETE /api/v1/test/purge/:kind/:id
	// (config.TestPurgeEnabled; рабочие конфигурации — false).
	PurgeEnabled bool
}

// Register — JSON API + SSR-страницы `/` и `/stock` (доска занимает корень, ADR-100 —
// home снят в этой фазе). s == nil — SSR-группа не регистрируется (эталон
// clients/routes.go; используется, где рендерер не нужен, напр. FuzzMoveBody).
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handlers, s *SSR) {
	admin := middleware.RequireAdmin(nil)
	col := api.Group("/columns", a.RequireAPI())
	col.Get("", h.Columns.List)
	col.Post("", admin, h.Columns.Create)
	col.Put("/:id", admin, h.Columns.Update)
	// soft-delete-board-columns §6.1, §6.2: удаление — мягкое, с причиной; восстановление — только
	// admin. Гард softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()
	col.Delete("/:id", sdAdmin, h.Columns.Delete)
	col.Post("/:id/restore", sdAdmin, h.Columns.Restore)

	// D19 — заказы создаёт/правит/переносит и executor; Delete — только admin.
	t := api.Group("/tasks", a.RequireAPI())
	t.Get("", h.Tasks.List)
	t.Post("", h.Tasks.Create)
	t.Get("/:id", h.Tasks.Get)
	t.Put("/:id", h.Tasks.Update)
	t.Post("/:id/move", h.Tasks.Move)
	// soft-delete-core §6.1, §6.2: удаление — мягкое, с причиной; восстановление — только admin.
	// Гард softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	t.Delete("/:id", sdAdmin, h.Tasks.Delete)
	t.Post("/:id/restore", sdAdmin, h.Tasks.Restore)

	// Служебная физическая уборка заказов автотестами (soft-delete-core §6.5, DS-027): адрес
	// существует только при TEST_PURGE_ENABLED=true; по умолчанию его нет (404).
	if h.PurgeEnabled {
		api.Delete("/test/purge/:kind/:id", a.RequireAPI(), sdAdmin, h.Tasks.Purge)
	}

	// Карточка заказа (фаза 6, AC-001) — D19: доступно и executor, admin-гардов нет.
	// Автомобиль заказа задаётся при создании и не меняется (one-car-order): отдельных
	// адресов оборудования заказа нет.
	t.Get("/:id/card", h.Card.Card)
	t.Post("/:id/subtasks", h.Card.CreateSubtask)

	// Услуги/товары/журнал заказа (фаза 6, AC-007/AC-008/AC-012) — §API-1 kebab-сегменты,
	// D19: доступно и executor.
	sub := api.Group("/subtasks", a.RequireAPI())
	sub.Put("/:id", h.Card.UpdateSubtask)
	sub.Delete("/:id", h.Card.DeleteSubtask)
	sub.Post("/:id/products", h.Card.CreateSubtaskProduct)
	sub.Post("/:id/logs", h.Card.CreateLog)

	sp := api.Group("/subtask-products", a.RequireAPI())
	sp.Put("/:id", h.Card.UpdateSubtaskProduct)
	sp.Delete("/:id", h.Card.DeleteSubtaskProduct)

	sl := api.Group("/subtask-logs", a.RequireAPI())
	sl.Put("/:id", h.Card.UpdateLog)
	sl.Delete("/:id", h.Card.DeleteLog)

	// Вложения заказа (фаза 7, D21) — D19: доступно и executor; удаление — автор или admin
	// (проверка в сервисе, ATTACHMENT_FORBIDDEN). Лимит тела загрузки — middleware.BodyLimit
	// по IsAttachmentUpload (router.go), остальные роуты — прежний 4 МБ.
	if h.Attachments != nil {
		t.Post("/:id/attachments", h.Attachments.Upload)
		at := api.Group("/attachments", a.RequireAPI())
		at.Get("/:id/file", h.Attachments.File)
		at.Delete("/:id", h.Attachments.Delete)
	}

	api.Get("/assignees", a.RequireAPI(), h.Assignees.List)

	// Остатки — чтение всем авторизованным (в т.ч. executor), правка вручную — admin (D44).
	st := api.Group("/stock", a.RequireAPI())
	st.Get("", h.Stock.List)
	st.Get("/summary", h.Stock.Summary)
	st.Put("/:product_id", admin, h.Stock.Adjust)

	if s == nil {
		return
	}
	app.Get("/", a.RequireSSR(), s.board)
	app.Get("/stock", a.RequireSSR(), s.stock)
	// Печать актов приёма/выдачи (фаза 04, ADR-102) — D19: доступно и executor.
	app.Get("/tasks/:id/print/:kind", a.RequireSSR(), s.print)
	// PDF-файл акта (пока только handover) — тот же контур доступа, что и HTML-печать.
	app.Get("/tasks/:id/print/:kind/pdf", a.RequireSSR(), s.printPDF)
}
