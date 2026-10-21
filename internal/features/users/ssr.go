package users

import (
	"errors"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// Root — шаблоны экрана «Сотрудники» относительно корня модуля.
const Root = "web/ssr/users/templates"

// SSR — экран «Сотрудники» (admin): список + формы создания/правки (PRG).
type SSR struct {
	svc    *Service
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(svc *Service, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{svc: svc, r: r, shell: shell, status: status}
}

// ListVM — список сотрудников. Фильтры — в URL (reload/назад/ссылка сохраняют состояние).
type ListVM struct {
	Shell  ssr.Shell
	Q      string
	Role   string
	Status string // "" | "active" | "blocked"
	Flash  string
	Items  []UserResponse
	Total  int
}

// FormVM — форма сотрудника (создание/правка).
type FormVM struct {
	Shell    ssr.Shell
	IsNew    bool
	ID       string
	Login    string
	Name     string
	Role     string
	Phone    string
	IsActive bool
	// Timezone — выбранный пояс (IANA); Timezones — пункты select'а (DS-056).
	Timezone  string
	Timezones []timezone.Option
	// SupplierID / Suppliers — поставщик учётки роли supplier и пункты выпадающего списка
	// (purchase-orders §8.5).
	SupplierID string
	Suppliers  []SupplierOption
	Errors     map[string]string // поле → текст ошибки
	Error      string            // общая ошибка формы
}

var flashes = map[string]string{
	"created": "Сотрудник создан",
	"saved":   "Изменения сохранены",
}

// list — GET /users.
func (h *SSR) list(c fiber.Ctx) error {
	vm := ListVM{Q: c.Query("q"), Role: c.Query("role"), Status: c.Query("status"), Flash: flashes[c.Query("flash")]}
	var active *bool
	switch vm.Status {
	case "active":
		active = ptr(true)
	case "blocked":
		active = ptr(false)
	default:
		vm.Status = ""
	}
	resp, err := h.svc.List(c.Context(), ListQuery{Q: vm.Q, Role: vm.Role, IsActive: active, Limit: maxLimit})
	if err != nil {
		return err
	}
	vm.Items, vm.Total = resp.Items, resp.Total
	vm.Shell = h.shell.Build(c, "Сотрудники")
	return h.r.Page(c, "list", vm)
}

// formNew — GET /users/new.
func (h *SSR) formNew(c fiber.Ctx) error {
	return h.renderForm(c, fiber.StatusOK, FormVM{IsNew: true, Role: RoleExecutor, IsActive: true, Timezone: timezone.Default})
}

// create — POST /users.
func (h *SSR) create(c fiber.Ctx) error {
	vm := formFrom(c)
	vm.IsNew = true
	_, err := h.svc.Create(c.Context(), Input{
		Login: vm.Login, Name: vm.Name, Role: vm.Role, Phone: vm.Phone, Password: c.FormValue("password"), IsActive: vm.IsActive,
		Timezone: formTimezone(c), SupplierID: vm.SupplierID,
	})
	if err != nil {
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/users?flash=created")
}

// formEdit — GET /users/:id.
func (h *SSR) formEdit(c fiber.Ctx) error {
	u, err := h.svc.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrUserNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm := FormVM{ID: u.ID, Login: u.Login, Name: u.Name, Role: u.Role, Phone: u.Phone, IsActive: u.IsActive, Timezone: u.Timezone}
	if u.SupplierID != nil {
		vm.SupplierID = *u.SupplierID
	}
	return h.renderForm(c, fiber.StatusOK, vm)
}

// update — POST /users/:id.
func (h *SSR) update(c fiber.Ctx) error {
	vm := formFrom(c)
	vm.ID = c.Params("id")
	_, err := h.svc.Update(c.Context(), actorOf(c), vm.ID, Input{
		Login: vm.Login, Name: vm.Name, Role: vm.Role, Phone: vm.Phone, Password: c.FormValue("password"), IsActive: vm.IsActive,
		Timezone: formTimezone(c), SupplierID: vm.SupplierID,
	})
	if errors.Is(err, ErrUserNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/users?flash=saved")
}

func formFrom(c fiber.Ctx) FormVM {
	return FormVM{
		Login:    c.FormValue("login"),
		Name:     c.FormValue("name"),
		Role:     c.FormValue("role"),
		Phone:    c.FormValue("phone"),
		IsActive: c.FormValue("is_active") == "on",
		Timezone: c.FormValue("timezone"),
		// Поле «Поставщик» имеет смысл только у роли supplier (скрыто у прочих ролей).
		SupplierID: c.FormValue("supplier_id"),
	}
}

// formTimezone — пояс из формы; поля нет/пусто (форма без select'а) — nil: при создании
// пояс по умолчанию, при правке колонка не трогается (DS-056).
func formTimezone(c fiber.Ctx) *string {
	if tz := c.FormValue("timezone"); tz != "" {
		return &tz
	}
	return nil
}

// formError — ошибки сервиса → форма с текстами (422 валидация, 409 конфликты).
func (h *SSR) formError(c fiber.Ctx, vm FormVM, err error) error {
	var app *errorspkg.AppError
	if !errors.As(err, &app) || app.HTTP >= fiber.StatusInternalServerError {
		return err
	}
	vm.Errors = map[string]string{}
	for _, f := range app.Fields {
		vm.Errors[f.Name] = fieldText(f.Name, f.Message)
	}
	switch {
	case errors.Is(err, ErrLoginTaken):
		vm.Errors["login"] = ErrLoginTaken.Message
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
	title := "Сотрудник"
	if vm.IsNew {
		title = "Новый сотрудник"
	}
	vm.Shell = h.shell.Build(c, title)
	if vm.Timezone == "" {
		vm.Timezone = timezone.Default
	}
	vm.Timezones = timezone.Options(time.Now(), vm.Timezone)
	sups, err := h.svc.ActiveSuppliers(c.Context())
	if err != nil {
		return err
	}
	vm.Suppliers = sups
	c.Status(status)
	return h.r.Page(c, "form", vm)
}

// fieldText — код ошибки поля → текст под полем.
func fieldText(field, code string) string {
	if field == "supplier_id" {
		switch code {
		case msgRequired:
			return "Выберите поставщика"
		case msgNotAllowed:
			return "Поставщик указывается только для роли «Поставщик»"
		}
		return "Поставщик недоступен"
	}
	if field == "timezone" {
		if code == msgRequired {
			return "Выберите часовой пояс"
		}
		return "Неизвестный часовой пояс"
	}
	switch code {
	case msgRequired:
		return "Обязательное поле"
	case msgTooLong:
		if field == "password" {
			return "Слишком длинный пароль: до 72 латинских символов (кириллица — до 36)"
		}
		return "Слишком длинное значение"
	case msgTooShort:
		if field == "password" {
			return "Минимум 8 символов"
		}
		return "Минимум 3 символа"
	case msgInvalidFormat:
		if field == "login" {
			return "Только латинские буквы, цифры, точка, дефис и подчёркивание"
		}
		return "Только цифры, пробелы и символы + ( ) -"
	}
	return "Недопустимое значение"
}

// FilterURL — ссылка списка с фильтрами (для шаблона: сброс одного фильтра и т.п.).
func (vm ListVM) FilterURL(q, role, status string) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if role != "" {
		v.Set("role", role)
	}
	if status != "" {
		v.Set("status", status)
	}
	if len(v) == 0 {
		return "/users"
	}
	return "/users?" + v.Encode()
}

func ptr[T any](v T) *T { return &v }
