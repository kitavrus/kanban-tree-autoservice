package catalog

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ServicesListVM — /catalog/services.
type ServicesListVM struct {
	Shell          ssr.Shell
	Tabs           TabsVM
	Q              string
	Archived       bool
	ActiveCategory string
	Flash          string
	// Toast — тост после PRG-перехода (запись удалена, soft-delete-catalog §8.1).
	Toast    string
	CatError string
	// CatDelete — отказ удаления категории по причине (окно «Удаление» строки категории).
	CatDelete  CatDeleteVM
	Categories []CategoryResponse
	CatTrunc   TruncVM // pagination DS-006: пометка «показаны N из M» в панели категорий
	Items      []ServiceResponse
	Total      int
	Pager      PagerVM // pagination DS-005: страница по ssrPageSize позиций
}

// ServiceProductRowVM — строка комплекта в form-view (совпадает с ServiceProductInput,
// отдельный тип — вьюмодель формы, не wire-контракт API).
type ServiceProductRowVM struct {
	ProductID string
	Qty       string
	// IsDeleted — товар строки мягко удалён из каталога: бейдж «удалён» (§8.1).
	IsDeleted bool
}

// ProductOptionVM — опция combobox товара в комплекте: подпись под названием
// (data-hint="артикул · цена / ед.", task.md).
type ProductOptionVM struct {
	ID       string
	Name     string
	Hint     string
	IsActive bool
	// IsDeleted — товар мягко удалён: опция показывается только в строке, где он уже выбран.
	IsDeleted bool
}

// ServiceFormVM — /catalog/services/new и /catalog/services/:id.
type ServiceFormVM struct {
	Shell      ssr.Shell
	Tabs       TabsVM
	IsNew      bool
	ID         string
	CategoryID string
	Name       string
	Price      string // "" — «по факту»
	IsActive   bool
	Products   []ServiceProductRowVM
	Categories []CategoryOptionVM
	// CategoryIsDeleted — категория услуги мягко удалена: бейдж «удалена» у поля (§8.1).
	CategoryIsDeleted bool
	CatTrunc          TruncVM // pagination DS-006: пометка «показаны N из M» под select категории
	ProductOptions    []ProductOptionVM
	Errors            map[string]string
	Error             string
	// DeleteReason / DeleteError — отказ удаления по причине (окно «Удаление», §8.1).
	DeleteReason string
	DeleteError  string
}

// PriceLabel — «по факту» при NULL (task.md: таблица услуг).
func (s ServiceResponse) PriceLabel() string {
	if s.Price == nil {
		return "по факту"
	}
	return s.Price.String() + " ₽"
}

// ProductsLabel — «Название ×qty, Название2 ×qty2» (task.md: таблица услуг).
func (s ServiceResponse) ProductsLabel() string {
	if len(s.Products) == 0 {
		return "—"
	}
	parts := make([]string, len(s.Products))
	for i, p := range s.Products {
		parts[i] = p.Name + " ×" + p.Qty.String()
	}
	return strings.Join(parts, ", ")
}

func productOptionsVM(items []ProductResponse) []ProductOptionVM {
	out := make([]ProductOptionVM, len(items))
	for i, p := range items {
		out[i] = ProductOptionVM{ID: p.ID, Name: p.Name, Hint: productHint(p.SKU, p.Unit, p.Price.String()), IsActive: p.IsActive}
	}
	return out
}

// list — GET /catalog/services.
func (h *SSR) servicesList(c fiber.Ctx) error {
	vm := ServicesListVM{Q: c.Query("q"), ActiveCategory: c.Query("category"), Flash: flashes[c.Query("flash")], Toast: toastOf(c.Query("flash")),
		CatError: catErrorText(c.Query("cat_error")), CatDelete: catDeleteFrom(c)}
	vm.Archived, _ = strconv.ParseBool(c.Query("archived"))

	cats, trunc, err := h.cat.list(c, KindService)
	if err != nil {
		return err
	}
	vm.Categories, vm.CatTrunc = cats, trunc
	offset := parseSSROffset(c.Query("offset")) // pagination DS-005

	resp, err := h.svcSvc.List(c.Context(), ServiceListQuery{
		CategoryID: vm.ActiveCategory, Q: vm.Q, IncludeArchived: vm.Archived, Limit: ssrPageSize, Offset: offset,
	}, true)
	if err != nil {
		return err
	}
	vm.Items, vm.Total = resp.Items, resp.Total
	vm.Pager = buildPager("/catalog/services", vm.Q, vm.ActiveCategory, vm.Archived, offset, len(resp.Items), resp.Total)
	vm.Shell = h.shell.Build(c, "Каталог: услуги")
	vm.Tabs = h.tabs(c, "services")
	return h.r.Page(c, "services-list", vm)
}

// serviceCategoryOptions — активные категории вида service + текущая категория услуги,
// даже архивная (иначе выбор при редактировании теряется — эталон productCategoryOptions);
// удалённая текущая — только в форме существующей услуги (categoryOptions).
func (h *SSR) serviceCategoryOptions(c fiber.Ctx, currentID string, keepDeleted bool) ([]CategoryOptionVM, TruncVM, error) {
	return h.categoryOptions(c, KindService, currentID, keepDeleted)
}

// serviceProductOptions — активные товары (комбобокс строки комплекта не предлагает
// архивные, task.md) + товары уже выбранных строк, даже архивные (иначе форма потеряет
// отображение уже выбранного — «архивные товары не предлагаются, но у существующей
// строки показываются», task.md — эталон productCategoryOptions). Мягко удалённый товар уже
// выбранной строки тоже остаётся виден — с признаком IsDeleted: шаблон показывает его только в
// своей строке с бейджем «удалён» и не предлагает в новых (soft-delete-catalog §8.1).
func (h *SSR) serviceProductOptions(c fiber.Ctx, existingIDs []string) ([]ProductOptionVM, error) {
	resp, err := h.prodSvc.List(c.Context(), ProductListQuery{IncludeArchived: false, Limit: defaultLimit}, true)
	if err != nil {
		return nil, err
	}
	opts := productOptionsVM(resp.Items)
	have := map[string]bool{}
	for _, o := range opts {
		have[o.ID] = true
	}
	var missing []string
	for _, id := range existingIDs {
		if !isUUID(id) || have[id] {
			continue
		}
		have[id] = true
		missing = append(missing, id)
	}
	extra, err := h.prodSvc.repo.OptionsByIDs(c.Context(), missing)
	if err != nil {
		return nil, err
	}
	return append(opts, extra...), nil
}

// markDeletedRows — признак «товар удалён» у строк состава по набору опций.
func markDeletedRows(rows []ServiceProductRowVM, opts []ProductOptionVM) {
	deleted := map[string]bool{}
	for _, o := range opts {
		if o.IsDeleted {
			deleted[o.ID] = true
		}
	}
	for i := range rows {
		rows[i].IsDeleted = deleted[rows[i].ProductID]
	}
}

// serviceVM — форма существующей услуги из ответа сервиса.
func (h *SSR) serviceVM(c fiber.Ctx, sv *ServiceResponse) (ServiceFormVM, error) {
	catOpts, catTrunc, err := h.serviceCategoryOptions(c, sv.CategoryID, true)
	if err != nil {
		return ServiceFormVM{}, err
	}
	ids := make([]string, len(sv.Products))
	for i, p := range sv.Products {
		ids[i] = p.ProductID
	}
	prodOpts, err := h.serviceProductOptions(c, ids)
	if err != nil {
		return ServiceFormVM{}, err
	}
	price := ""
	if sv.Price != nil {
		price = sv.Price.String()
	}
	return ServiceFormVM{
		ID: sv.ID, CategoryID: sv.CategoryID, Name: sv.Name, Price: price, IsActive: sv.IsActive,
		Products: serviceRowsFrom(sv), Categories: catOpts, CatTrunc: catTrunc, ProductOptions: prodOpts,
	}, nil
}

func (h *SSR) serviceFormNew(c fiber.Ctx) error {
	catOpts, catTrunc, err := h.serviceCategoryOptions(c, c.Query("category"), false)
	if err != nil {
		return err
	}
	prodOpts, err := h.serviceProductOptions(c, nil)
	if err != nil {
		return err
	}
	return h.renderServiceForm(c, fiber.StatusOK, ServiceFormVM{
		IsNew: true, IsActive: true, CategoryID: c.Query("category"),
		Categories: catOpts, CatTrunc: catTrunc, ProductOptions: prodOpts, Products: []ServiceProductRowVM{{}},
	})
}

// serviceProductsFrom — строки комплекта из тела POST (row-list.js: одинаковые имена
// повторяются N раз — читаются через PostArgs().PeekMulti, эталон clients/ssr.go phonesFrom).
func serviceProductsFrom(c fiber.Ctx) []ServiceProductInput {
	ids := c.Request().PostArgs().PeekMulti("product_id")
	qtys := c.Request().PostArgs().PeekMulti("qty")
	out := make([]ServiceProductInput, 0, len(ids))
	for i := range ids {
		qty := ""
		if i < len(qtys) {
			qty = string(qtys[i])
		}
		out = append(out, ServiceProductInput{ProductID: string(ids[i]), Qty: qty})
	}
	return out
}

// serviceFormFrom — поля формы из тела POST, перерисовываются НОРМАЛИЗОВАННЫМ списком
// строк комплекта (ServiceInput.normalize() — та же нормализация, что validate()):
// индексы ошибок `products[i].*` считаются по нормализованному списку (design-errors.md;
// после объединения дублей — тоже, эталон clients/ssr.go formFrom).
func serviceFormFrom(c fiber.Ctx) ServiceFormVM {
	in := ServiceInput{
		CategoryID: c.FormValue("category_id"),
		Name:       c.FormValue("name"),
		Products:   serviceProductsFrom(c),
	}
	if price := c.FormValue("price"); price != "" {
		in.HasPrice, in.Price = true, price
	}
	in = in.normalize()

	rows := make([]ServiceProductRowVM, len(in.Products))
	for i, p := range in.Products {
		rows[i] = ServiceProductRowVM{ProductID: p.ProductID, Qty: p.Qty}
	}
	if len(rows) == 0 {
		rows = []ServiceProductRowVM{{}}
	}
	price := ""
	if in.HasPrice {
		price = in.Price
	}
	return ServiceFormVM{CategoryID: in.CategoryID, Name: in.Name, Price: price, Products: rows}
}

func serviceInputFromForm(vm ServiceFormVM) ServiceInput {
	products := make([]ServiceProductInput, len(vm.Products))
	for i, p := range vm.Products {
		products[i] = ServiceProductInput{ProductID: p.ProductID, Qty: p.Qty}
	}
	in := ServiceInput{CategoryID: vm.CategoryID, Name: vm.Name, Products: products, IsActive: vm.IsActive}
	if vm.Price != "" {
		in.HasPrice, in.Price = true, vm.Price
	}
	return in
}

func (h *SSR) serviceCreate(c fiber.Ctx) error {
	vm := serviceFormFrom(c)
	vm.IsNew, vm.IsActive = true, true
	_, err := h.svcSvc.Create(c.Context(), serviceInputFromForm(vm))
	if err != nil {
		return h.serviceFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/services?flash=service_created")
}

func serviceRowsFrom(resp *ServiceResponse) []ServiceProductRowVM {
	rows := make([]ServiceProductRowVM, len(resp.Products))
	for i, p := range resp.Products {
		rows[i] = ServiceProductRowVM{ProductID: p.ProductID, Qty: p.Qty.String()}
	}
	if len(rows) == 0 {
		rows = []ServiceProductRowVM{{}}
	}
	return rows
}

func (h *SSR) serviceFormEdit(c fiber.Ctx) error {
	sv, err := h.svcSvc.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrServiceNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm, err := h.serviceVM(c, sv)
	if err != nil {
		return err
	}
	return h.renderServiceForm(c, fiber.StatusOK, vm)
}

func (h *SSR) serviceUpdate(c fiber.Ctx) error {
	vm := serviceFormFrom(c)
	vm.ID = c.Params("id")
	cur, err := h.svcSvc.Get(c.Context(), vm.ID)
	if errors.Is(err, ErrServiceNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm.IsActive = cur.IsActive
	_, err = h.svcSvc.Update(c.Context(), vm.ID, serviceInputFromForm(vm))
	if err != nil {
		return h.serviceFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/services?flash=service_saved")
}

func (h *SSR) serviceArchive(c fiber.Ctx) error {
	return h.serviceSetActive(c, false, "service_archived")
}
func (h *SSR) serviceRestore(c fiber.Ctx) error {
	return h.serviceSetActive(c, true, "service_restored")
}

func (h *SSR) serviceSetActive(c fiber.Ctx, active bool, flash string) error {
	id := c.Params("id")
	if err := h.svcSvc.SetActive(c.Context(), id, active); err != nil {
		if errors.Is(err, ErrServiceNotFoundErr) {
			return h.status.NotFound(c)
		}
		return err
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/services?flash=" + flash)
}

// serviceRemove — POST /catalog/services/:id/delete, поле формы reason — мягкое удаление
// услуги каталога (soft-delete-catalog §6.9, §8.1, DS-032): заказы не мешают. Отказ по причине
// — форма с окном «Удаление» и текстом под полем (422); удалённая либо несуществующая услуга —
// страница 404; успех — список с тостом.
func (h *SSR) serviceRemove(c fiber.Ctx) error {
	id := c.Params("id")
	reason := c.FormValue("reason")
	_, err := h.svcSvc.Delete(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	if errors.Is(err, ErrServiceNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		sv, gerr := h.svcSvc.Get(c.Context(), id)
		if gerr != nil {
			return err
		}
		vm, verr := h.serviceVM(c, sv)
		if verr != nil {
			return err
		}
		if text := softdelete.ReasonError(err); text != "" {
			vm.DeleteReason, vm.DeleteError = reason, text
			return h.renderServiceForm(c, fiber.StatusUnprocessableEntity, vm)
		}
		return h.serviceFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/services?flash=service_deleted")
}

// serviceFormError — ошибки сервиса → форма с текстами (422 валидация, 409 конфликты).
func (h *SSR) serviceFormError(c fiber.Ctx, vm ServiceFormVM, err error) error {
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
	status := app.HTTP
	if len(app.Fields) > 0 {
		status = fiber.StatusUnprocessableEntity
	}
	catOpts, catTrunc, _ := h.serviceCategoryOptions(c, vm.CategoryID, !vm.IsNew)
	vm.Categories, vm.CatTrunc = catOpts, catTrunc
	ids := make([]string, len(vm.Products))
	for i, p := range vm.Products {
		ids[i] = p.ProductID
	}
	prodOpts, _ := h.serviceProductOptions(c, ids)
	vm.ProductOptions = prodOpts
	return h.renderServiceForm(c, status, vm)
}

func (h *SSR) renderServiceForm(c fiber.Ctx, status int, vm ServiceFormVM) error {
	title := "Услуга"
	if vm.IsNew {
		title = "Новая услуга"
	}
	vm.Shell = h.shell.Build(c, title)
	vm.Tabs = h.tabs(c, "services")
	vm.CategoryIsDeleted = currentCategoryDeleted(vm.Categories, vm.CategoryID)
	markDeletedRows(vm.Products, vm.ProductOptions)
	c.Status(status)
	return h.r.Page(c, "services-form", vm)
}
