package warehouses

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Root — шаблоны экрана «Склады» относительно корня модуля.
const Root = "web/ssr/warehouses/templates"

// SSR — экран «Склады» (admin, DS-001): список + форма добавления на той же странице,
// правка названия, архив/возврат, мягкое удаление с причиной (soft-delete-warehouses-suppliers
// §8.1). PRG, как equipmenttypes.
type SSR struct {
	svc    *Service
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(svc *Service, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{svc: svc, r: r, shell: shell, status: status}
}

// ListVM — список складов + форма добавления (NewName/NewError — после неудачного POST).
type ListVM struct {
	Shell    ssr.Shell
	Archived bool
	Flash    string
	Error    string
	Items    []WarehouseResponse
	NewName  string
	NewError string
	// Toast — тост после PRG-перехода (склад удалён, §8.1).
	Toast string
	// Delete — отказ удаления по причине: окно «Удаление» строки склада ID открывается заново с
	// текстом под полем (§8.1).
	Delete DeleteVM
}

// DeleteVM — отказ удаления склада ID по причине (введённая причина и текст ошибки).
type DeleteVM struct {
	ID, Reason, Error string
}

// flashDeleted — ключ flash мягкого удаления: после PRG-перехода — тост (§8.1).
const flashDeleted = "deleted"

// listErrors — ключ ?error= → красный блок над списком (отказы архивирования и удаления).
var listErrors = map[string]string{
	"last_active":        ErrLastActive.Message,
	"in_use":             ErrInUse.Message,
	"delete_stock":       ErrDeleteHasStock.Message,
	"delete_equipment":   ErrDeleteHasEquipment.Message,
	"delete_transfers":   ErrDeleteHasTransfers.Message,
	"delete_last_active": ErrDeleteLastActive.Message,
}

// FormVM — правка склада.
type FormVM struct {
	Shell    ssr.Shell
	ID       string
	Name     string
	IsActive bool
	Errors   map[string]string
	Error    string
}

var flashes = map[string]string{
	"created":  "Склад добавлен",
	"saved":    "Изменения сохранены",
	"archived": "Склад отправлен в архив",
	"restored": "Склад возвращён из архива",
}

func (h *SSR) loadList(c fiber.Ctx, vm *ListVM) error {
	resp, err := h.svc.List(c.Context(), ListQuery{IncludeArchived: vm.Archived, Limit: defaultLimit})
	if err != nil {
		return err
	}
	vm.Items = resp.Items
	vm.Shell = h.shell.Build(c, "Склады")
	return nil
}

// list — GET /warehouses.
func (h *SSR) list(c fiber.Ctx) error {
	vm := ListVM{Flash: flashes[c.Query("flash")]}
	vm.Archived, _ = strconv.ParseBool(c.Query("archived"))
	vm.Error = listErrors[c.Query("error")]
	if c.Query("flash") == flashDeleted {
		vm.Toast = softdelete.ToastDeleted
	}
	if err := h.loadList(c, &vm); err != nil {
		return err
	}
	return h.r.Page(c, "list", vm)
}

// create — POST /warehouses.
func (h *SSR) create(c fiber.Ctx) error {
	name := c.FormValue("name")
	_, err := h.svc.Create(c.Context(), Input{Name: name, IsActive: true})
	if err == nil {
		return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?flash=created")
	}
	var app *errorspkg.AppError
	if !errors.As(err, &app) || app.HTTP >= fiber.StatusInternalServerError {
		return err
	}
	vm := ListVM{NewName: name, NewError: app.Message}
	for _, f := range app.Fields {
		if f.Name == "name" {
			vm.NewError = fieldText(f.Message)
		}
	}
	if err := h.loadList(c, &vm); err != nil {
		return err
	}
	c.Status(fiber.StatusUnprocessableEntity)
	return h.r.Page(c, "list", vm)
}

// formEdit — GET /warehouses/:id.
func (h *SSR) formEdit(c fiber.Ctx) error {
	w, err := h.svc.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrWarehouseNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	return h.renderForm(c, fiber.StatusOK, FormVM{ID: w.ID, Name: w.Name, IsActive: w.IsActive})
}

// update — POST /warehouses/:id (переименование; is_active не меняется).
func (h *SSR) update(c fiber.Ctx) error {
	id := c.Params("id")
	cur, err := h.svc.Get(c.Context(), id)
	if errors.Is(err, ErrWarehouseNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm := FormVM{ID: id, Name: c.FormValue("name"), IsActive: cur.IsActive}
	if _, err := h.svc.Update(c.Context(), id, Input{Name: vm.Name, IsActive: cur.IsActive}); err != nil {
		var app *errorspkg.AppError
		if !errors.As(err, &app) || app.HTTP >= fiber.StatusInternalServerError {
			return err
		}
		vm.Errors = map[string]string{}
		for _, f := range app.Fields {
			vm.Errors[f.Name] = fieldText(f.Message)
		}
		if len(app.Fields) == 0 {
			vm.Error = app.Message
		}
		return h.renderForm(c, fiber.StatusUnprocessableEntity, vm)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?flash=saved")
}

func (h *SSR) archive(c fiber.Ctx) error { return h.setActive(c, false, "archived") }
func (h *SSR) restore(c fiber.Ctx) error { return h.setActive(c, true, "restored") }

func (h *SSR) setActive(c fiber.Ctx, active bool, flash string) error {
	q := ""
	if c.FormValue("archived") != "" {
		q = "archived=1&"
	}
	err := h.svc.SetActive(c.Context(), c.Params("id"), active)
	switch {
	case errors.Is(err, ErrWarehouseNotFound):
		return h.status.NotFound(c)
	case errors.Is(err, ErrLastActive):
		return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?" + q + "error=last_active")
	case errors.Is(err, ErrInUse):
		return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?" + q + "error=in_use")
	case err != nil:
		return err
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?" + q + "flash=" + flash)
}

// remove — POST /warehouses/:id/delete, поле формы reason — мягкое удаление склада
// (soft-delete-warehouses-suppliers §6.5, §8.1, DS-032). Отказ по причине — список с окном
// «Удаление» и текстом под полем (422); отказы 409 — красным блоком над списком (как ошибки
// архивирования); удалённый либо несуществующий склад — страница 404; успех — список с тостом.
func (h *SSR) remove(c fiber.Ctx) error {
	id := c.Params("id")
	reason := c.FormValue("reason")
	archived := c.FormValue("archived") != ""
	q := ""
	if archived {
		q = "archived=1&"
	}
	_, err := h.svc.Delete(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	key := ""
	switch {
	case err == nil:
		return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?" + q + "flash=" + flashDeleted)
	case errors.Is(err, ErrWarehouseNotFound):
		return h.status.NotFound(c)
	case errors.Is(err, ErrDeleteHasStock):
		key = "delete_stock"
	case errors.Is(err, ErrDeleteHasEquipment):
		key = "delete_equipment"
	case errors.Is(err, ErrDeleteHasTransfers):
		key = "delete_transfers"
	case errors.Is(err, ErrDeleteLastActive):
		key = "delete_last_active"
	}
	if key != "" {
		return c.Redirect().Status(fiber.StatusSeeOther).To("/warehouses?" + q + "error=" + key)
	}
	text := softdelete.ReasonError(err)
	if text == "" {
		return err
	}
	vm := ListVM{Archived: archived, Delete: DeleteVM{ID: id, Reason: reason, Error: text}}
	if err := h.loadList(c, &vm); err != nil {
		return err
	}
	c.Status(fiber.StatusUnprocessableEntity)
	return h.r.Page(c, "list", vm)
}

func (h *SSR) renderForm(c fiber.Ctx, status int, vm FormVM) error {
	vm.Shell = h.shell.Build(c, "Склад")
	c.Status(status)
	return h.r.Page(c, "form", vm)
}

// fieldText — код ошибки поля → текст под полем.
func fieldText(code string) string {
	switch code {
	case msgRequired:
		return "Обязательное поле"
	case msgTooLong:
		return "Слишком длинное значение"
	case msgTaken:
		return "Такой склад уже есть"
	case msgArchived:
		return "Такой склад уже есть в архиве — верните его из архива"
	}
	return "Недопустимое значение"
}
