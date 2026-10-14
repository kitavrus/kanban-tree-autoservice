package clients

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

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
	Shell           ssr.Shell
	IsNew           bool
	ID              string
	CompanyName     string
	Address         string
	ContactName     string
	ContactPosition string
	Phones          []PhoneRow
	Version         int64
	EquipmentCount  int
	Equipment       []EquipmentRow
	Flash           string            // flash после PRG-редиректа на карточку (created/saved/eq_*)
	Errors          map[string]string // поле → текст ошибки
	Error           string            // общая ошибка формы
	Conflict        bool              // ADR-005: 409 → ссылка «Открыть актуальную версию»
	// Toast — тост после PRG-перехода (единица удалена, soft-delete-clients §8.1).
	Toast string
	// DeleteReason / DeleteError — отказ удаления по причине: окно «Удаление» открывается
	// заново с введённой причиной и текстом под полем (§8.1).
	DeleteReason string
	DeleteError  string
}

// EquipmentRow — строка блока «Оборудование клиента» в карточке.
type EquipmentRow struct {
	ID           string
	Model        string
	TypeName     string
	TypeIsActive bool
	// TypeIsDeleted — тип единицы мягко удалён: бейдж «удалён» (soft-delete-catalog §8.3).
	TypeIsDeleted bool
	PlateNumber   string
	Serial        string
}

var flashes = map[string]string{
	"created":    "Клиент создан",
	"saved":      "Изменения сохранены",
	"eq_created": "Оборудование добавлено",
	"eq_saved":   "Оборудование сохранено",
}

// Ключи flash мягкого удаления (soft-delete-clients §8.1): после PRG-перехода — тост.
const (
	flashDeleted   = "deleted"
	flashEqDeleted = "eq_deleted"

	toastEquipmentDeleted = "Удалено. Восстановить можно в разделе «Удалённые»"
)

// clientDeletedToast — тост после удаления клиента; n — число единиц, ушедших вместе с ним.
func clientDeletedToast(n int) string {
	return fmt.Sprintf("Удалено вместе с оборудованием: %d. Восстановить можно в разделе «Удалённые»", n)
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
		out[i] = EquipmentRow{ID: e.ID, Model: e.Model, TypeName: e.TypeName, TypeIsActive: e.TypeIsActive, TypeIsDeleted: e.TypeIsDeleted, PlateNumber: e.PlateNumber, Serial: e.Serial}
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
// клиенту (ADR-102 — TypeOptions читает equipment_types напрямую SQL, без Go-импорта
// пакета equipmenttypes). ---

// SpecRow — строка характеристики в form-view (совпадает с SpecDTO, вьюмодель формы).
type SpecRow struct {
	K string
	V string
}

// TypeOptionVM — опция <select> формы единицы; SpecKeysJSON — JSON-массив ключей типа
// для data-spec-keys (JS подставляет шаблон характеристик при смене типа, ADR-002).
type TypeOptionVM struct {
	ID           string
	Name         string
	IsActive     bool
	IsDeleted    bool
	SpecKeysJSON string
}

// EquipmentFormVM — форма единицы оборудования (создание/правка).
type EquipmentFormVM struct {
	Shell       ssr.Shell
	IsNew       bool
	ID          string
	ClientID    string
	ClientName  string
	TypeID      string
	PlateNumber string
	Brand       string
	Model       string
	SKU         string
	Serial      string
	Note        string
	Specs       []SpecRow
	Types       []TypeOptionVM
	// TypeIsDeleted — выбранный тип единицы мягко удалён: бейдж «удалён» у поля
	// (soft-delete-catalog §8.3).
	TypeIsDeleted bool
	Errors        map[string]string
	Error         string
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
	vm := EquipmentFormVM{IsNew: true, ClientID: clientID, ClientName: cl.CompanyName, Specs: []SpecRow{{}}}
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

// equipmentFormFrom — поля формы из тела POST, перерисовываются НОРМАЛИЗОВАННЫМ списком
// характеристик (EquipmentInput.normalize() — та же нормализация, что validate()): индексы
// `specs[i].k` считаются по нормализованному списку (design-errors.md) — vm.Specs должен
// быть тем же списком, иначе ошибка индекса рисуется не под той строкой (code review MEDIUM-2).
func equipmentFormFrom(c fiber.Ctx) EquipmentFormVM {
	in := EquipmentInput{
		TypeID:      c.FormValue("type_id"),
		PlateNumber: c.FormValue("plate_number"),
		Brand:       c.FormValue("brand"),
		Model:       c.FormValue("model"),
		SKU:         c.FormValue("sku"),
		Serial:      c.FormValue("serial"),
		Note:        c.FormValue("note"),
		Specs:       specsFrom(c),
	}.normalize()
	rows := make([]SpecRow, len(in.Specs))
	for i, sp := range in.Specs {
		rows[i] = SpecRow{K: sp.K, V: sp.V}
	}
	if len(rows) == 0 {
		rows = []SpecRow{{}}
	}
	return EquipmentFormVM{
		TypeID:      in.TypeID,
		PlateNumber: in.PlateNumber,
		Brand:       in.Brand,
		Model:       in.Model,
		SKU:         in.SKU,
		Serial:      in.Serial,
		Note:        in.Note,
		Specs:       rows,
	}
}

func specsFrom(c fiber.Ctx) []SpecDTO {
	ks := c.Request().PostArgs().PeekMulti("spec_k")
	vs := c.Request().PostArgs().PeekMulti("spec_v")
	out := make([]SpecDTO, 0, len(ks))
	for i := range ks {
		v := ""
		if i < len(vs) {
			v = string(vs[i])
		}
		out = append(out, SpecDTO{K: string(ks[i]), V: v})
	}
	return out
}

func equipmentInputFrom(vm EquipmentFormVM) EquipmentInput {
	specs := make([]SpecDTO, len(vm.Specs))
	for i, sp := range vm.Specs {
		specs[i] = SpecDTO{K: sp.K, V: sp.V}
	}
	return EquipmentInput{
		TypeID: vm.TypeID, PlateNumber: vm.PlateNumber, Brand: vm.Brand, Model: vm.Model, SKU: vm.SKU, Serial: vm.Serial,
		Note: vm.Note, Specs: specs,
	}
}

func vmFromEquipment(e EquipmentResponse, clientName string) EquipmentFormVM {
	rows := make([]SpecRow, len(e.Specs))
	for i, sp := range e.Specs {
		rows[i] = SpecRow{K: sp.K, V: sp.V}
	}
	if len(rows) == 0 {
		rows = []SpecRow{{}}
	}
	return EquipmentFormVM{
		ID: e.ID, ClientID: e.ClientID, ClientName: clientName,
		TypeID: e.TypeID, PlateNumber: e.PlateNumber, Brand: e.Brand, Model: e.Model, SKU: e.SKU, Serial: e.Serial,
		Note: e.Note, Specs: rows,
	}
}

// typeOptionsVM — TypeOption (repo) → опции <select> с JSON-шаблоном характеристик.
func typeOptionsVM(opts []TypeOption) []TypeOptionVM {
	out := make([]TypeOptionVM, len(opts))
	for i, o := range opts {
		keys := o.SpecKeys
		if keys == nil {
			keys = []string{}
		}
		b, _ := json.Marshal(keys)
		out[i] = TypeOptionVM{ID: o.ID, Name: o.Name, IsActive: o.IsActive, IsDeleted: o.IsDeleted, SpecKeysJSON: string(b)}
	}
	return out
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
		vm.Errors[f.Name] = equipmentFieldText(f.Name, f.Message)
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

// renderEquipmentForm — опции типа читаются заново на каждый рендер (ADR-102): включают
// vm.TypeID, даже если он архивный (правка существующей единицы без смены типа). Мягко
// удалённый тип остаётся текущим значением только у существующей единицы — в форме новой его
// нет (soft-delete-catalog §8.3: в выборе типа удалённых нет).
func (h *SSR) renderEquipmentForm(c fiber.Ctx, status int, vm EquipmentFormVM) error {
	curTypeID := vm.TypeID
	if uuid.Validate(curTypeID) != nil {
		curTypeID = ""
	}
	opts, err := h.svc.TypeOptions(c.Context(), curTypeID)
	if err != nil {
		return err
	}
	vm.Types = vm.Types[:0]
	for _, o := range typeOptionsVM(opts) {
		if o.IsDeleted && vm.IsNew {
			continue
		}
		vm.Types = append(vm.Types, o)
		vm.TypeIsDeleted = vm.TypeIsDeleted || (o.IsDeleted && o.ID == vm.TypeID)
	}

	title := "Оборудование"
	if vm.IsNew {
		title = "Новое оборудование"
	}
	vm.Shell = h.shell.Build(c, title)
	c.Status(status)
	return h.r.Page(c, "equipment-form", vm)
}

// equipmentFieldText — код ошибки поля → текст под полем (design-errors.md §fieldText,
// часть equipment).
func equipmentFieldText(field, code string) string {
	switch code {
	case msgRequired:
		if field == "type_id" {
			return "Выберите тип"
		}
		return "Обязательное поле"
	case msgTooLong:
		return "Слишком длинное значение"
	case msgTooMany:
		return "Не больше 50 характеристик"
	case msgInvalid, "not_found":
		return "Тип не найден"
	case "archived":
		return "Тип в архиве — выберите другой"
	case msgKeyRequired:
		return "Укажите характеристику"
	case msgInvalidFormat:
		if field == "plate_number" {
			return "Только буквы, цифры, пробел и дефис"
		}
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
		CompanyName:     c.FormValue("company_name"),
		Address:         c.FormValue("address"),
		ContactName:     c.FormValue("contact_name"),
		ContactPosition: c.FormValue("contact_position"),
		Phones:          phonesFrom(c),
	}.normalize()
	rows := make([]PhoneRow, len(in.Phones))
	for i, p := range in.Phones {
		rows[i] = PhoneRow{Phone: p.Phone, Note: p.Note}
	}
	if len(rows) == 0 {
		rows = []PhoneRow{{}}
	}
	return FormVM{
		CompanyName:     in.CompanyName,
		Address:         in.Address,
		ContactName:     in.ContactName,
		ContactPosition: in.ContactPosition,
		Phones:          rows,
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
		CompanyName:     vm.CompanyName,
		Address:         vm.Address,
		ContactName:     vm.ContactName,
		ContactPosition: vm.ContactPosition,
		Phones:          phones,
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
		ID: cl.ID, CompanyName: cl.CompanyName, Address: cl.Address,
		ContactName: cl.ContactName, ContactPosition: cl.ContactPosition,
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
