package clients

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Root — шаблоны экрана «Клиенты» относительно корня модуля.
const Root = "web/ssr/clients/templates"

// SSR — экран «Клиенты» (/clients) + карточка клиента: список + формы создания/правки
// (PRG), удаление (подтверждение), конфликт одновременной правки (ADR-005) + блок
// «Оборудование клиента» и форма единицы оборудования (Phase 8).
type SSR struct {
	svc    *Service
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(svc *Service, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{svc: svc, r: r, shell: shell, status: status}
}

// ListVM — список клиентов. Фильтр q — в URL (reload/назад/ссылка сохраняют состояние).
type ListVM struct {
	Shell ssr.Shell
	Q     string
	Flash string
	// Toast — тост после PRG-перехода (мягкое удаление клиента, soft-delete-clients §8.1).
	Toast string
	Items []ClientResponse
	Total int
}

// PhoneRow — строка телефона в form-view (совпадает с PhoneDTO, отдельный тип — вьюмодель
// формы, не wire-контракт API).
type PhoneRow struct {
	Phone string
	Note  string
}

// FormVM — форма клиента (создание/правка) + карточка (телефоны 1..N, версия, блок
// «Оборудование клиента»).
type FormVM struct {
	Shell          ssr.Shell
	IsNew          bool
	ID             string
	CompanyName    string
	Phones         []PhoneRow
	Version        int64
	EquipmentCount int
	Equipment      []EquipmentRow
	Flash          string            // flash после PRG-редиректа на карточку (created/saved/eq_*)
	Errors         map[string]string // поле → текст ошибки
	Error          string            // общая ошибка формы
	Conflict       bool              // ADR-005: 409 → ссылка «Открыть актуальную версию»
	// Toast — тост после PRG-перехода (единица удалена, soft-delete-clients §8.1).
	Toast string
	// DeleteReason / DeleteError — отказ удаления по причине: окно «Удаление» открывается
	// заново с введённой причиной и текстом под полем (§8.1).
	DeleteReason string
	DeleteError  string
}

// EquipmentRow — строка блока «Оборудование клиента» в карточке: только госномер.
type EquipmentRow struct {
	ID          string
	PlateNumber string
}

var flashes = map[string]string{
	"created":    "Клиент создан",
	"saved":      "Изменения сохранены",
	"eq_created": "Автомобиль добавлен",
	"eq_saved":   "Автомобиль сохранён",
}

// Ключи flash мягкого удаления (soft-delete-clients §8.1): после PRG-перехода — тост.
const (
	flashDeleted   = "deleted"
	flashEqDeleted = "eq_deleted"

	toastEquipmentDeleted = "Удалено. Восстановить можно в разделе «Удалённые»"
)

// clientDeletedToast — тост после удаления клиента; n — число единиц, ушедших вместе с ним.
func clientDeletedToast(n int) string {
	return fmt.Sprintf("Удалено вместе с автомобилями: %d. Восстановить можно в разделе «Удалённые»", n)
}

// reasonErrorText — код ошибки поля reason → текст под полем «Причина» (§6.1, DS-020).
func reasonErrorText(code string) string {
	switch code {
	case "required":
		return "Укажите причину удаления"
	case "too_long":
		return "Причина — не длиннее 500 символов"
	case "invalid_text":
		return "Недопустимые символы в причине"
	}
	return "Недопустимое значение"
}

// reasonError — текст отказа по полю reason из ошибки сервиса; "" — отказ не по причине.
func reasonError(err error) string {
	var app *errorspkg.AppError
	if !errors.As(err, &app) {
		return ""
	}
	for _, f := range app.Fields {
		if f.Name == "reason" {
			return reasonErrorText(f.Message)
		}
	}
	return ""
}

// list — GET /clients.
func (h *SSR) list(c fiber.Ctx) error {
	vm := ListVM{Q: c.Query("q"), Flash: flashes[c.Query("flash")]}
	if c.Query("flash") == flashDeleted {
		n, _ := strconv.Atoi(c.Query("n"))
		vm.Toast = clientDeletedToast(max(n, 0))
	}
	resp, err := h.svc.List(c.Context(), ListQuery{Q: vm.Q, Limit: maxLimit})
	if err != nil {
		return err
	}
	vm.Items, vm.Total = resp.Items, resp.Total
	vm.Shell = h.shell.Build(c, "Клиенты")
	return h.r.Page(c, "list", vm)
}

// formNew — GET /clients/new.
func (h *SSR) formNew(c fiber.Ctx) error {
	return h.renderForm(c, fiber.StatusOK, FormVM{IsNew: true, Phones: []PhoneRow{{}}})
}

// create — POST /clients. Редирект — на карточку созданного клиента (design-go-layers.md
// §SSR Routes: `/clients/{id}?flash=created`), не на список (code review LOW#3).
func (h *SSR) create(c fiber.Ctx) error {
	vm := formFrom(c)
	vm.IsNew = true
	cl, err := h.svc.Create(c.Context(), inputFrom(vm))
	if err != nil {
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/clients/" + cl.ID + "?flash=created")
}

// formEdit — GET /clients/:id.
func (h *SSR) formEdit(c fiber.Ctx) error {
	cl, err := h.svc.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrClientNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm := vmFromResponse(*cl)
	vm.Flash = flashes[c.Query("flash")]
	if c.Query("flash") == flashEqDeleted {
		vm.Toast = toastEquipmentDeleted
	}
	eq, err := h.svc.ListEquipment(c.Context(), cl.ID, 0, 0)
	if err != nil {
		return err
	}
	vm.Equipment = equipmentRowsOf(eq.Items)
	return h.renderForm(c, fiber.StatusOK, vm)
}

func equipmentRowsOf(items []EquipmentResponse) []EquipmentRow {
	out := make([]EquipmentRow, len(items))
	for i, e := range items {
		out[i] = EquipmentRow{ID: e.ID, PlateNumber: e.PlateNumber}
	}
	return out
}

// update — POST /clients/:id. Version пуст/битый → nil → сервис вернёт 400 version:
// required (ADR-100), как на JSON API. Редирект на успехе — карточка (design-go-layers.md
// §SSR Routes: `/clients/{id}?flash=saved`, code review LOW#3).
func (h *SSR) update(c fiber.Ctx) error {
	vm := formFrom(c)
	vm.ID = c.Params("id")

	var version *int64
	if raw := c.FormValue("version"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			version = &v
			vm.Version = v
		}
	}

	_, err := h.svc.Update(c.Context(), vm.ID, version, inputFrom(vm))
	if errors.Is(err, ErrClientNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		// блок «Оборудование клиента» не должен пропадать при перерисовке формы (422/409) —
		// раньше vm.Equipment оставался пуст на любой ошибке сохранения (code review LOW#4).
		if eq, eqErr := h.svc.ListEquipment(c.Context(), vm.ID, 0, 0); eqErr == nil {
			vm.Equipment = equipmentRowsOf(eq.Items)
			vm.EquipmentCount = eq.Total
		}
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/clients/" + vm.ID + "?flash=saved")
}

// remove — POST /clients/:id/delete, поле формы reason — мягкое удаление клиента вместе с
// оборудованием (soft-delete-clients §6.5, §8.1, DS-032). Прочий отказ — клиент остаётся,
// карточка перерисовывается с красным блоком; отказ по причине — карточка с окном «Удаление» и текстом под полем; успех — список с тостом.
func (h *SSR) remove(c fiber.Ctx) error {
	id := c.Params("id")
	reason := c.FormValue("reason")
	resp, err := h.svc.Delete(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	if errors.Is(err, ErrClientNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		cl, gerr := h.svc.Get(c.Context(), id)
		if gerr != nil {
			return err
		}
		vm := vmFromResponse(*cl)
		if eq, eqErr := h.svc.ListEquipment(c.Context(), id, 0, 0); eqErr == nil {
			vm.Equipment = equipmentRowsOf(eq.Items)
			vm.EquipmentCount = eq.Total
		}
		if text := reasonError(err); text != "" {
			vm.DeleteReason, vm.DeleteError = reason, text
			return h.renderForm(c, fiber.StatusUnprocessableEntity, vm)
		}
		return h.formError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/clients?flash=" + flashDeleted + "&n=" + strconv.Itoa(resp.EquipmentDeleted))
}

// --- Оборудование клиента (Phase 8): форма создания/правки единицы, привязанной к
// клиенту. ---

// EquipmentFormVM — форма единицы оборудования (создание/правка).
type EquipmentFormVM struct {
	Shell       ssr.Shell
	IsNew       bool
	ID          string
	ClientID    string
	ClientName  string
	PlateNumber string
	Errors      map[string]string
	Error       string
	// DeleteReason / DeleteError — отказ удаления по причине (окно «Удаление», §8.1).
	DeleteReason string
	DeleteError  string
}

// eqNew — GET /clients/:id/equipment/new.
func (h *SSR) eqNew(c fiber.Ctx) error {
	clientID := c.Params("id")
	cl, err := h.svc.Get(c.Context(), clientID)
	if errors.Is(err, ErrClientNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm := EquipmentFormVM{IsNew: true, ClientID: clientID, ClientName: cl.CompanyName}
	return h.renderEquipmentForm(c, fiber.StatusOK, vm)
}

// eqCreate — POST /clients/:id/equipment.
func (h *SSR) eqCreate(c fiber.Ctx) error {
	clientID := c.Params("id")
	vm := equipmentFormFrom(c)
	vm.IsNew = true
	vm.ClientID = clientID
	if cl, err := h.svc.Get(c.Context(), clientID); err == nil {
		vm.ClientName = cl.CompanyName
	}

	_, err := h.svc.CreateEquipment(c.Context(), clientID, equipmentInputFrom(vm))
	if errors.Is(err, ErrClientNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return h.equipmentFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/clients/" + clientID + "?flash=eq_created")
}

// eqEdit — GET /equipment/:id.
func (h *SSR) eqEdit(c fiber.Ctx) error {
	e, err := h.svc.GetEquipment(c.Context(), c.Params("id"))
	if errors.Is(err, ErrEquipmentNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	clientName := ""
	if cl, err := h.svc.Get(c.Context(), e.ClientID); err == nil {
		clientName = cl.CompanyName
	}
	return h.renderEquipmentForm(c, fiber.StatusOK, vmFromEquipment(*e, clientName))
}

// eqUpdate — POST /equipment/:id (LWW, без version — D28).
func (h *SSR) eqUpdate(c fiber.Ctx) error {
	id := c.Params("id")
	cur, err := h.svc.GetEquipment(c.Context(), id)
	if errors.Is(err, ErrEquipmentNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}

	vm := equipmentFormFrom(c)
	vm.ID = id
	vm.ClientID = cur.ClientID
	if cl, gerr := h.svc.Get(c.Context(), cur.ClientID); gerr == nil {
		vm.ClientName = cl.CompanyName
	}

	_, err = h.svc.UpdateEquipment(c.Context(), id, equipmentInputFrom(vm))
	if errors.Is(err, ErrEquipmentNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return h.equipmentFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/clients/" + cur.ClientID + "?flash=eq_saved")
}

// eqRemove — POST /equipment/:id/delete, поле формы reason — мягкое удаление единицы
// (soft-delete-clients §6.5, §8.1, DS-032). Прочий отказ — форма единицы перерисовывается с
// красным блоком; отказ по причине — форма с окном
// «Удаление»; успех — карточка клиента с тостом.
func (h *SSR) eqRemove(c fiber.Ctx) error {
	id := c.Params("id")
	reason := c.FormValue("reason")
	_, clientID, err := h.svc.DeleteEquipment(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	if errors.Is(err, ErrEquipmentNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		e, gerr := h.svc.GetEquipment(c.Context(), id)
		if gerr != nil {
			return err
		}
		clientName := ""
		if cl, cerr := h.svc.Get(c.Context(), e.ClientID); cerr == nil {
			clientName = cl.CompanyName
		}
		vm := vmFromEquipment(*e, clientName)
		if text := reasonError(err); text != "" {
			vm.DeleteReason, vm.DeleteError = reason, text
			return h.renderEquipmentForm(c, fiber.StatusUnprocessableEntity, vm)
		}
		return h.equipmentFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/clients/" + clientID + "?flash=" + flashEqDeleted)
}

// equipmentFormFrom — госномер из тела POST, перерисовывается нормализованным.
func equipmentFormFrom(c fiber.Ctx) EquipmentFormVM {
	in := EquipmentInput{PlateNumber: c.FormValue("plate_number")}.normalize()
	return EquipmentFormVM{PlateNumber: in.PlateNumber}
}

func equipmentInputFrom(vm EquipmentFormVM) EquipmentInput {
	return EquipmentInput{PlateNumber: vm.PlateNumber}
}

func vmFromEquipment(e EquipmentResponse, clientName string) EquipmentFormVM {
	return EquipmentFormVM{ID: e.ID, ClientID: e.ClientID, ClientName: clientName, PlateNumber: e.PlateNumber}
}

// equipmentFormError — ошибки сервиса → форма единицы с текстами (422 валидация; 404 —
// уже обработан вызывающей стороной до попадания сюда).
func (h *SSR) equipmentFormError(c fiber.Ctx, vm EquipmentFormVM, err error) error {
	var app *errorspkg.AppError
	if !errors.As(err, &app) || app.HTTP >= fiber.StatusInternalServerError {
		return err
	}
	vm.Errors = map[string]string{}
	for _, f := range app.Fields {
		vm.Errors[f.Name] = equipmentFieldText(f.Message)
	}
	if len(app.Fields) == 0 {
		vm.Error = app.Message
	}
	status := app.HTTP
	if len(app.Fields) > 0 {
		status = fiber.StatusUnprocessableEntity
	}
	return h.renderEquipmentForm(c, status, vm)
}

func (h *SSR) renderEquipmentForm(c fiber.Ctx, status int, vm EquipmentFormVM) error {
	title := "Автомобиль"
	if vm.IsNew {
		title = "Новый автомобиль"
	}
	vm.Shell = h.shell.Build(c, title)
	c.Status(status)
	return h.r.Page(c, "equipment-form", vm)
}

// equipmentFieldText — код ошибки поля → текст под полем (design-errors.md §fieldText,
// часть equipment).
func equipmentFieldText(code string) string {
	switch code {
	case msgRequired:
		return "Укажите госномер"
	case msgTooLong:
		return "Не длиннее 20 символов"
	case msgInvalidFormat:
		return "Только буквы, цифры, пробел и дефис"
	}
	return "Недопустимое значение"
}

// formFrom — поля формы из тела POST, перерисовываются НОРМАЛИЗОВАННЫМ списком телефонов
// (ClientInput.normalize() — та же нормализация, что использует validate()): индексы ошибок
// `phones[i].phone` считаются по нормализованному списку (design-errors.md), поэтому и
// vm.Phones должен быть тем же списком — иначе при отбросе пустых строк (напр.
// ["", "abc"]) ошибка индекса 0 рисуется под неправильной строкой формы (code review MEDIUM-2).
func formFrom(c fiber.Ctx) FormVM {
	in := ClientInput{
		CompanyName: c.FormValue("company_name"),
		Phones:      phonesFrom(c),
	}.normalize()
	rows := make([]PhoneRow, len(in.Phones))
	for i, p := range in.Phones {
		rows[i] = PhoneRow{Phone: p.Phone, Note: p.Note}
	}
	if len(rows) == 0 {
		rows = []PhoneRow{{}}
	}
	return FormVM{
		CompanyName: in.CompanyName,
		Phones:      rows,
	}
}

func phonesFrom(c fiber.Ctx) []PhoneDTO {
	phones := c.Request().PostArgs().PeekMulti("phone")
	notes := c.Request().PostArgs().PeekMulti("phone_note")
	out := make([]PhoneDTO, 0, len(phones))
	for i := range phones {
		note := ""
		if i < len(notes) {
			note = string(notes[i])
		}
		out = append(out, PhoneDTO{Phone: string(phones[i]), Note: note})
	}
	return out
}

func inputFrom(vm FormVM) ClientInput {
	phones := make([]PhoneDTO, len(vm.Phones))
	for i, p := range vm.Phones {
		phones[i] = PhoneDTO{Phone: p.Phone, Note: p.Note}
	}
	return ClientInput{
		CompanyName: vm.CompanyName,
		Phones:      phones,
	}
}

func vmFromResponse(cl ClientResponse) FormVM {
	rows := make([]PhoneRow, len(cl.Phones))
	for i, p := range cl.Phones {
		rows[i] = PhoneRow{Phone: p.Phone, Note: p.Note}
	}
	if len(rows) == 0 {
		rows = []PhoneRow{{}}
	}
	return FormVM{
		ID: cl.ID, CompanyName: cl.CompanyName,
		Phones: rows, Version: cl.Version, EquipmentCount: cl.EquipmentCount,
	}
}

// formError — ошибки сервиса → форма с текстами (422 валидация, 409 конфликт версии —
// эталон users/ssr.go formError; ветка ErrClientVersionConflict — ADR-005).
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
	case errors.Is(err, ErrClientVersionConflict):
		vm.Error = ErrClientVersionConflict.Message
		vm.Conflict = true
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
	title := "Клиент"
	if vm.IsNew {
		title = "Новый клиент"
	}
	vm.Shell = h.shell.Build(c, title)
	c.Status(status)
	return h.r.Page(c, "form", vm)
}

// fieldText — код ошибки поля → текст под полем (design-errors.md §fieldText, часть clients).
func fieldText(field, code string) string {
	switch code {
	case msgRequired:
		if field == "phones" {
			return "Укажите хотя бы один телефон"
		}
		return "Обязательное поле"
	case msgTooLong:
		return "Слишком длинное значение"
	case msgTooMany:
		return "Не больше 20 телефонов"
	case msgInvalidFormat:
		return "Номер должен содержать цифры"
	}
	return "Недопустимое значение"
}
