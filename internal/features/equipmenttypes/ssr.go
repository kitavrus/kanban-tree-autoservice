package equipmenttypes

import (
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/internal/features/catalog"
	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Root — шаблоны экрана «Каталог: типы оборудования» относительно корня модуля.
const Root = "web/ssr/equipmenttypes/templates"

// SSR — экран «Каталог: типы оборудования» (admin): список + формы создания/правки
// (PRG), архив/восстановление, удаление свободного типа (AC-011, AC-012). Делит полосу
// вкладок каталога (catalog-tabs partial) с пакетом catalog (фаза 4) — pool нужен только
// для счётчиков catalog.LoadTabCounts, без обратного импорта catalog → equipmenttypes.
type SSR struct {
	svc    *Service
	pool   *pgxpool.Pool
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(svc *Service, pool *pgxpool.Pool, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{svc: svc, pool: pool, r: r, shell: shell, status: status}
}

// tabs — счётчики полосы вкладок; ошибка НЕ проглатывается молча (code review LOW#2,
// тот же фикс, что в catalog/ssr.go tabs).
func (h *SSR) tabs(c fiber.Ctx) catalog.TabsVM {
	cnt, err := catalog.LoadTabCounts(c.Context(), h.pool)
	if err != nil {
		slog.ErrorContext(c.Context(), "equipmenttypes: не удалось посчитать вкладки", "error", err, "request_id", requestid.FromContext(c))
		return catalog.TabsVM{Active: "equipment-types"}
	}
	return catalog.TabsVM{Active: "equipment-types", Services: cnt.Services, Products: cnt.Products, Types: cnt.Types}
}

// ListVM — список типов. Фильтры (q, archived) — в URL: reload/назад/ссылка сохраняют
// состояние (AC-011).
type ListVM struct {
	Shell    ssr.Shell
	Tabs     catalog.TabsVM
	Q        string
	Archived bool
	Flash    string
	// Toast — тост после PRG-перехода (тип удалён, soft-delete-catalog §8.1).
	Toast string
	Items []TypeResponse
	Total int
}

// SpecKeysText — характеристики через запятую для списка («—», если пусто).
func (t TypeResponse) SpecKeysText() string {
	if len(t.SpecKeys) == 0 {
		return "—"
	}
	return strings.Join(t.SpecKeys, ", ")
}

// FormVM — форма типа оборудования (создание/правка).
type FormVM struct {
	Shell          ssr.Shell
	IsNew          bool
	ID             string
	Name           string
	SpecKeys       string // одна строка через запятую — как вводит пользователь
	IsActive       bool
	EquipmentCount int
	Errors         map[string]string // поле → текст ошибки
	Error          string            // общая ошибка формы
	ArchivedLink   string            // ADR-007: имя занято архивным типом → ссылка «Показать архивные»
	// DeleteReason / DeleteError — отказ удаления по причине: окно «Удаление» открывается
	// заново с введённым текстом и ошибкой под полем (soft-delete-catalog §8.1).
	DeleteReason string
	DeleteError  string
}

var flashes = map[string]string{
	"created":  "Тип оборудования создан",
	"saved":    "Изменения сохранены",
	"archived": "Тип отправлен в архив",
	"restored": "Тип возвращён из архива",
}

// flashDeleted — ключ flash мягкого удаления: после PRG-перехода — тост (§8.1).
const flashDeleted = "deleted"

// list — GET /catalog/equipment-types.
func (h *SSR) list(c fiber.Ctx) error {
	vm := ListVM{Q: c.Query("q"), Flash: flashes[c.Query("flash")]}
	if c.Query("flash") == flashDeleted {
		vm.Toast = softdelete.ToastDeleted
	}
	vm.Archived, _ = strconv.ParseBool(c.Query("archived"))
	// SSR-экран доступен только admin (routes.go) — include_archived читаем как есть.
	resp, err := h.svc.List(c.Context(), ListQuery{Q: vm.Q, IncludeArchived: vm.Archived, Limit: defaultLimit}, true)
	if err != nil {
		return err
	}
	vm.Items, vm.Total = resp.Items, resp.Total
	vm.Shell = h.shell.Build(c, "Каталог: типы оборудования")
	vm.Tabs = h.tabs(c)
	return h.r.Page(c, "list", vm)
}

// formNew — GET /catalog/equipment-types/new.
func (h *SSR) formNew(c fiber.Ctx) error {
	return h.renderForm(c, fiber.StatusOK, FormVM{IsNew: true, IsActive: true})
}

// create — POST /catalog/equipment-types.
func (h *SSR) create(c fiber.Ctx) error {
	vm := formFrom(c)
	vm.IsNew = true
	_, err := h.svc.Create(c.Context(), Input{Name: vm.Name, SpecKeys: ParseSpecKeys(vm.SpecKeys), IsActive: true})
	if err != nil {
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/equipment-types?flash=created")
}

// formEdit — GET /catalog/equipment-types/:id.
func (h *SSR) formEdit(c fiber.Ctx) error {
	t, err := h.svc.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrTypeNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	return h.renderForm(c, fiber.StatusOK, FormVM{
		ID: t.ID, Name: t.Name, SpecKeys: strings.Join(t.SpecKeys, ", "),
		IsActive: t.IsActive, EquipmentCount: t.EquipmentCount,
	})
}

// update — POST /catalog/equipment-types/:id (is_active сохраняется как есть —
// архивирование/восстановление отдельными маршрутами).
func (h *SSR) update(c fiber.Ctx) error {
	vm := formFrom(c)
	vm.ID = c.Params("id")
	cur, err := h.svc.Get(c.Context(), vm.ID)
	if errors.Is(err, ErrTypeNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm.IsActive = cur.IsActive
	vm.EquipmentCount = cur.EquipmentCount
	_, err = h.svc.Update(c.Context(), vm.ID, Input{Name: vm.Name, SpecKeys: ParseSpecKeys(vm.SpecKeys), IsActive: cur.IsActive})
	if err != nil {
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/equipment-types?flash=saved")
}

// archive — POST /catalog/equipment-types/:id/archive. Редирект сохраняет фильтр
// `archived`, с которым была отправлена форма (design-go-layers.md §SSR Routes: «фильтр
// сохраняется: archived из формы») — НЕ форсирует archived=1 (это восстановление, code
// review LOW#3).
func (h *SSR) archive(c fiber.Ctx) error {
	q := ""
	if v := c.FormValue("archived"); v != "" {
		q = "archived=" + url.QueryEscape(v) + "&"
	}
	return h.setActive(c, false, "archived", q)
}

// restore — POST /catalog/equipment-types/:id/restore. Редирект — всегда с archived=1
// (design-go-layers.md §SSR Routes): восстановленный тип виден в архивном списке, откуда
// обычно и вызывается «Вернуть».
func (h *SSR) restore(c fiber.Ctx) error { return h.setActive(c, true, "restored", "archived=1&") }

func (h *SSR) setActive(c fiber.Ctx, active bool, flash, q string) error {
	id := c.Params("id")
	err := h.svc.SetActive(c.Context(), id, active)
	if errors.Is(err, ErrTypeNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/equipment-types?" + q + "flash=" + flash)
}

// remove — POST /catalog/equipment-types/:id/delete, поле формы reason — мягкое удаление типа
// (soft-delete-catalog §6.9, §8.1, DS-032): единицы оборудования не мешают. Отказ по причине —
// форма с окном «Удаление» и текстом под полем (422); удалённый либо несуществующий тип —
// страница 404; успех — список с тостом.
func (h *SSR) remove(c fiber.Ctx) error {
	id := c.Params("id")
	reason := c.FormValue("reason")
	_, err := h.svc.Delete(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	if errors.Is(err, ErrTypeNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		t, gerr := h.svc.Get(c.Context(), id)
		if gerr != nil {
			return err
		}
		vm := FormVM{ID: t.ID, Name: t.Name, SpecKeys: strings.Join(t.SpecKeys, ", "), IsActive: t.IsActive, EquipmentCount: t.EquipmentCount}
		if text := softdelete.ReasonError(err); text != "" {
			vm.DeleteReason, vm.DeleteError = reason, text
			return h.renderForm(c, fiber.StatusUnprocessableEntity, vm)
		}
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/equipment-types?flash=" + flashDeleted)
}

func formFrom(c fiber.Ctx) FormVM {
	return FormVM{Name: c.FormValue("name"), SpecKeys: c.FormValue("spec_keys")}
}

// formError — ошибки сервиса → форма с текстами (422 валидация, 409 конфликты —
// эталон users/ssr.go formError).
func (h *SSR) formError(c fiber.Ctx, vm FormVM, err error) error {
	var app *errorspkg.AppError
	if !errors.As(err, &app) || app.HTTP >= fiber.StatusInternalServerError {
		return err
	}
	vm.Errors = map[string]string{}
	for _, f := range app.Fields {
		name := f.Name
		if strings.HasPrefix(name, "spec_keys[") {
			// SSR-форма — один текстовый инпут (spec_keys через запятую), без построчных
			// индексов: любая ошибка отдельного ключа сворачивается в общую под полем.
			name = "spec_keys"
		}
		vm.Errors[name] = fieldText(name, f.Message)
	}
	switch {
	case errors.Is(err, ErrTypeNameTaken):
		vm.Errors["name"] = ErrTypeNameTaken.Message
	case errors.Is(err, ErrTypeNameArchived):
		vm.Errors["name"] = ErrTypeNameArchived.Message
		vm.ArchivedLink = "/catalog/equipment-types?archived=1&q=" + url.QueryEscape(vm.Name)
	case len(app.Fields) == 0:
		vm.Error = app.Message
	}
	status := app.HTTP
	if len(app.Fields) > 0 {
		status = fiber.StatusUnprocessableEntity
	}
	return h.renderForm(c, status, vm)
}

func (h *SSR) renderForm(c fiber.Ctx, status int, vm FormVM) error {
	title := "Тип оборудования"
	if vm.IsNew {
		title = "Новый тип оборудования"
	}
	vm.Shell = h.shell.Build(c, title)
	c.Status(status)
	return h.r.Page(c, "form", vm)
}

// fieldText — код ошибки поля → текст под полем (design-errors.md §fieldText).
func fieldText(field, code string) string {
	switch code {
	case msgRequired:
		return "Обязательное поле"
	case msgTooLong:
		return "Слишком длинное значение"
	case msgTooMany:
		return "Не больше 50 характеристик"
	}
	_ = field
	return "Недопустимое значение"
}
