package catalog

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ProductsListVM — /catalog/products.
type ProductsListVM struct {
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
	Items      []ProductResponse
	Total      int
	Pager      PagerVM // pagination DS-005: страница по ssrPageSize позиций
}

// ProductFormVM — /catalog/products/new и /catalog/products/:id.
type ProductFormVM struct {
	Shell      ssr.Shell
	Tabs       TabsVM
	IsNew      bool
	ID         string
	CategoryID string
	Name       string
	SKU        string
	Unit       string
	Price      string
	Stocks     []ProductStockVM // блок «Склады» (DS-003, D46)
	IsActive   bool
	Units      []string
	Categories []CategoryOptionVM
	// CategoryIsDeleted — категория товара мягко удалена: бейдж «удалена» у поля (§8.1).
	CategoryIsDeleted bool
	CatTrunc          TruncVM // pagination DS-006: пометка «показаны N из M» под select категории
	Errors            map[string]string
	Error             string
	// DeleteReason / DeleteError — отказ удаления по причине: окно «Удаление» открывается
	// заново с введённым текстом и ошибкой под полем (§8.1).
	DeleteReason string
	DeleteError  string
}

// ProductStockVM — строка блока «Склады» формы товара. Поля формы повторяются
// (wh_id/wh_name/wh_stock/wh_min_stock/wh_box/wh_place) в порядке строк.
type ProductStockVM struct {
	WarehouseID   string
	WarehouseName string
	Stock         string
	MinStock      string
	Box           string
	Place         string
}

// IsLow — остаток пары товар×склад на грани минимума (DS-004). Пара без строки
// product_stock (Tracked=false) — не учитывается (D48).
func (s ProductStockResponse) IsLow() bool {
	stock, err1 := strconv.ParseFloat(s.Stock.String(), 64)
	min, err2 := strconv.ParseFloat(s.MinStock.String(), 64)
	return s.Tracked && err1 == nil && err2 == nil && stock <= min
}

// IsDebt — отрицательный остаток пары товар×склад («долг», task.md).
func (s ProductStockResponse) IsDebt() bool {
	stock, err := strconv.ParseFloat(s.Stock.String(), 64)
	return err == nil && stock < 0
}

// IsLow — хоть на одном складе остаток на грани минимума (выделение в таблице).
func (p ProductResponse) IsLow() bool {
	for _, s := range p.Stocks {
		if s.IsLow() {
			return true
		}
	}
	return false
}

// IsDebt — хоть на одном складе отрицательный остаток.
func (p ProductResponse) IsDebt() bool {
	for _, s := range p.Stocks {
		if s.IsDebt() {
			return true
		}
	}
	return false
}

func stockVMsFromResponse(rows []ProductStockResponse) []ProductStockVM {
	out := make([]ProductStockVM, len(rows))
	for i, s := range rows {
		out[i] = ProductStockVM{WarehouseID: s.WarehouseID, WarehouseName: s.WarehouseName,
			Stock: s.Stock.String(), MinStock: s.MinStock.String(), Box: s.Box, Place: s.Place}
	}
	return out
}

func stockVMsFromForm(c fiber.Ctx) []ProductStockVM {
	args := c.Request().PostArgs()
	ids := args.PeekMulti("wh_id")
	at := func(key string, i int) string {
		v := args.PeekMulti(key)
		if i < len(v) {
			return string(v[i])
		}
		return ""
	}
	out := make([]ProductStockVM, len(ids))
	for i, id := range ids {
		out[i] = ProductStockVM{WarehouseID: string(id), WarehouseName: at("wh_name", i),
			Stock: at("wh_stock", i), MinStock: at("wh_min_stock", i), Box: at("wh_box", i), Place: at("wh_place", i)}
	}
	return out
}

// list — GET /catalog/products.
func (h *SSR) productsList(c fiber.Ctx) error {
	vm := ProductsListVM{Q: c.Query("q"), ActiveCategory: c.Query("category"), Flash: flashes[c.Query("flash")], Toast: toastOf(c.Query("flash")),
		CatError: catErrorText(c.Query("cat_error")), CatDelete: catDeleteFrom(c)}
	vm.Archived, _ = strconv.ParseBool(c.Query("archived"))

	cats, trunc, err := h.cat.list(c, KindProduct)
	if err != nil {
		return err
	}
	vm.Categories, vm.CatTrunc = cats, trunc
	offset := parseSSROffset(c.Query("offset")) // pagination DS-005

	resp, err := h.prodSvc.List(c.Context(), ProductListQuery{
		CategoryID: vm.ActiveCategory, Q: vm.Q, IncludeArchived: vm.Archived, Limit: ssrPageSize, Offset: offset,
	}, true)
	if err != nil {
		return err
	}
	vm.Items, vm.Total = resp.Items, resp.Total
	vm.Pager = buildPager("/catalog/products", vm.Q, vm.ActiveCategory, vm.Archived, offset, len(resp.Items), resp.Total)
	vm.Shell = h.shell.Build(c, "Каталог: товары")
	vm.Tabs = h.tabs(c, "products")
	return h.r.Page(c, "products-list", vm)
}

// productCategoryOptions — активные категории вида product + текущая категория товара,
// даже если она в архиве (иначе выбор при редактировании теряется, task.md: «архивный
// товар/категория не предлагается ПРИ ДОБАВЛЕНИИ», но уже выбранная должна остаться видна);
// удалённая текущая — только в форме существующего товара (categoryOptions).
func (h *SSR) productCategoryOptions(c fiber.Ctx, currentID string, keepDeleted bool) ([]CategoryOptionVM, TruncVM, error) {
	return h.categoryOptions(c, KindProduct, currentID, keepDeleted)
}

// productVM — форма существующего товара из ответа сервиса.
func (h *SSR) productVM(c fiber.Ctx, p *ProductResponse) (ProductFormVM, error) {
	opts, catTrunc, err := h.productCategoryOptions(c, p.CategoryID, true)
	if err != nil {
		return ProductFormVM{}, err
	}
	return ProductFormVM{
		ID: p.ID, CategoryID: p.CategoryID, Name: p.Name, SKU: p.SKU, Unit: p.Unit,
		Price: p.Price.String(), Stocks: stockVMsFromResponse(p.Stocks),
		IsActive: p.IsActive, Categories: opts, CatTrunc: catTrunc,
	}, nil
}

func (h *SSR) productFormNew(c fiber.Ctx) error {
	opts, catTrunc, err := h.productCategoryOptions(c, c.Query("category"), false)
	if err != nil {
		return err
	}
	whs, err := h.prodSvc.ActiveWarehouses(c.Context())
	if err != nil {
		return err
	}
	stocks := make([]ProductStockVM, len(whs))
	for i, w := range whs {
		stocks[i] = ProductStockVM{WarehouseID: w.ID, WarehouseName: w.Name}
	}
	return h.renderProductForm(c, fiber.StatusOK, ProductFormVM{IsNew: true, IsActive: true, CategoryID: c.Query("category"), Categories: opts, CatTrunc: catTrunc, Stocks: stocks})
}

func productFormFrom(c fiber.Ctx) ProductFormVM {
	return ProductFormVM{
		CategoryID: c.FormValue("category_id"), Name: c.FormValue("name"), SKU: c.FormValue("sku"),
		Unit: c.FormValue("unit"), Price: c.FormValue("price"), Stocks: stockVMsFromForm(c),
	}
}

func inputFromForm(vm ProductFormVM) ProductInput {
	stocks := make([]ProductStockInput, len(vm.Stocks))
	for i, s := range vm.Stocks {
		stocks[i] = ProductStockInput{WarehouseID: s.WarehouseID, Stock: s.Stock, MinStock: s.MinStock, Box: s.Box, Place: s.Place}
	}
	return ProductInput{
		CategoryID: vm.CategoryID, Name: vm.Name, SKU: vm.SKU, Unit: vm.Unit,
		Price: vm.Price, Stocks: stocks, IsActive: vm.IsActive,
	}
}

func (h *SSR) productCreate(c fiber.Ctx) error {
	vm := productFormFrom(c)
	vm.IsNew, vm.IsActive = true, true
	_, err := h.prodSvc.Create(c.Context(), inputFromForm(vm))
	if err != nil {
		return h.productFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/products?flash=created")
}

func (h *SSR) productFormEdit(c fiber.Ctx) error {
	p, err := h.prodSvc.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrProductNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm, err := h.productVM(c, p)
	if err != nil {
		return err
	}
	return h.renderProductForm(c, fiber.StatusOK, vm)
}

func (h *SSR) productUpdate(c fiber.Ctx) error {
	vm := productFormFrom(c)
	vm.ID = c.Params("id")
	cur, err := h.prodSvc.Get(c.Context(), vm.ID)
	if errors.Is(err, ErrProductNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm.IsActive = cur.IsActive
	userID := ""
	if p := middleware.CurrentPrincipal(c); p != nil {
		userID = p.ID
	}
	_, err = h.prodSvc.Update(c.Context(), vm.ID, userID, inputFromForm(vm))
	if err != nil {
		return h.productFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/products?flash=saved")
}

func (h *SSR) productArchive(c fiber.Ctx) error { return h.productSetActive(c, false, "archived") }
func (h *SSR) productRestore(c fiber.Ctx) error { return h.productSetActive(c, true, "restored") }

func (h *SSR) productSetActive(c fiber.Ctx, active bool, flash string) error {
	id := c.Params("id")
	if err := h.prodSvc.SetActive(c.Context(), id, active); err != nil {
		if errors.Is(err, ErrProductNotFoundErr) {
			return h.status.NotFound(c)
		}
		return err
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/products?flash=" + flash)
}

// productRemove — POST /catalog/products/:id/delete, поле формы reason — мягкое удаление
// товара (soft-delete-catalog §6.9, §8.1, DS-032). Отказ 409 PRODUCT_HAS_STOCK — форма товара
// с красным блоком; отказ по причине — форма с окном «Удаление» и текстом под полем (422);
// удалённый либо несуществующий товар — страница 404; успех — список с тостом.
func (h *SSR) productRemove(c fiber.Ctx) error {
	id := c.Params("id")
	reason := c.FormValue("reason")
	_, err := h.prodSvc.Delete(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	if errors.Is(err, ErrProductNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		p, gerr := h.prodSvc.Get(c.Context(), id)
		if gerr != nil {
			return err
		}
		vm, verr := h.productVM(c, p)
		if verr != nil {
			return err
		}
		if text := softdelete.ReasonError(err); text != "" {
			vm.DeleteReason, vm.DeleteError = reason, text
			return h.renderProductForm(c, fiber.StatusUnprocessableEntity, vm)
		}
		return h.productFormError(c, vm, err)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/products?flash=deleted")
}

// productFormError — ошибки сервиса → форма с текстами (422 валидация, 409 конфликты).
func (h *SSR) productFormError(c fiber.Ctx, vm ProductFormVM, err error) error {
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
	opts, catTrunc, _ := h.productCategoryOptions(c, vm.CategoryID, !vm.IsNew)
	vm.Categories, vm.CatTrunc = opts, catTrunc
	return h.renderProductForm(c, status, vm)
}

func (h *SSR) renderProductForm(c fiber.Ctx, status int, vm ProductFormVM) error {
	title := "Товар"
	if vm.IsNew {
		title = "Новый товар"
	}
	vm.Shell = h.shell.Build(c, title)
	vm.Tabs = h.tabs(c, "products")
	vm.Units = Units
	vm.CategoryIsDeleted = currentCategoryDeleted(vm.Categories, vm.CategoryID)
	c.Status(status)
	return h.r.Page(c, "products-form", vm)
}

// fieldText — код ошибки поля → текст под полем (эталон equipmenttypes/ssr.go).
func fieldText(code string) string {
	switch code {
	case msgRequired:
		return "Обязательное поле"
	case msgTooLong:
		return "Слишком длинное значение"
	case msgInvalidText:
		return "Недопустимые символы"
	case msgInvalidFormat:
		return "Введите число"
	case msgTooPrecise:
		return "Слишком много знаков после запятой"
	case msgTooLarge:
		return "Слишком большое число"
	case msgInvalid:
		return "Недопустимое значение"
	case msgNotFound:
		return "Запись удалена — выберите другую"
	case msgDuplicate:
		return "Склад указан дважды"
	}
	return "Недопустимое значение"
}
