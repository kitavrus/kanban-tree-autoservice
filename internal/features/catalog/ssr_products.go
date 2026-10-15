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
	Shell    ssr.Shell
	Tabs     TabsVM
	Q        string
	Archived bool
	Flash    string
	// Toast — тост после PRG-перехода (запись удалена, soft-delete-catalog §8.1).
	Toast string
	Items []ProductResponse
	Total int
	Pager PagerVM // pagination DS-005: страница по ssrPageSize позиций
}

// ProductFormVM — /catalog/products/new и /catalog/products/:id.
type ProductFormVM struct {
	Shell    ssr.Shell
	Tabs     TabsVM
	IsNew    bool
	ID       string
	Name     string
	SKU      string
	Unit     string
	Price    string
	Stock    string
	MinStock string
	Box      string
	Place    string
	IsActive bool
	Units    []string
	Errors   map[string]string
	Error    string
	// DeleteReason / DeleteError — отказ удаления по причине: окно «Удаление» открывается
	// заново с введённым текстом и ошибкой под полем (§8.1).
	DeleteReason string
	DeleteError  string
}

// IsLow — остаток на грани минимума (выделение в таблице, task.md).
func (p ProductResponse) IsLow() bool {
	stock, err1 := strconv.ParseFloat(p.Stock.String(), 64)
	min, err2 := strconv.ParseFloat(p.MinStock.String(), 64)
	return err1 == nil && err2 == nil && stock <= min
}

// IsDebt — отрицательный остаток («долг», task.md).
func (p ProductResponse) IsDebt() bool {
	stock, err := strconv.ParseFloat(p.Stock.String(), 64)
	return err == nil && stock < 0
}

// list — GET /catalog/products.
func (h *SSR) productsList(c fiber.Ctx) error {
	vm := ProductsListVM{Q: c.Query("q"), Flash: flashes[c.Query("flash")], Toast: toastOf(c.Query("flash"))}
	vm.Archived, _ = strconv.ParseBool(c.Query("archived"))
	offset := parseSSROffset(c.Query("offset")) // pagination DS-005

	resp, err := h.prodSvc.List(c.Context(), ProductListQuery{
		Q: vm.Q, IncludeArchived: vm.Archived, Limit: ssrPageSize, Offset: offset,
	}, true)
	if err != nil {
		return err
	}
	vm.Items, vm.Total = resp.Items, resp.Total
	vm.Pager = buildPager("/catalog/products", vm.Q, "", vm.Archived, offset, len(resp.Items), resp.Total)
	vm.Shell = h.shell.Build(c, "Каталог: запчасти")
	vm.Tabs = h.tabs(c, "products")
	return h.r.Page(c, "products-list", vm)
}

// productVM — форма существующего товара из ответа сервиса.
func productVM(p *ProductResponse) ProductFormVM {
	return ProductFormVM{
		ID: p.ID, Name: p.Name, SKU: p.SKU, Unit: p.Unit,
		Price: p.Price.String(), Stock: p.Stock.String(), MinStock: p.MinStock.String(),
		Box: p.Box, Place: p.Place, IsActive: p.IsActive,
	}
}

func (h *SSR) productFormNew(c fiber.Ctx) error {
	return h.renderProductForm(c, fiber.StatusOK, ProductFormVM{IsNew: true, IsActive: true})
}

func productFormFrom(c fiber.Ctx) ProductFormVM {
	return ProductFormVM{
		Name: c.FormValue("name"), SKU: c.FormValue("sku"),
		Unit: c.FormValue("unit"), Price: c.FormValue("price"), Stock: c.FormValue("stock"),
		MinStock: c.FormValue("min_stock"), Box: c.FormValue("box"), Place: c.FormValue("place"),
	}
}

func inputFromForm(vm ProductFormVM) ProductInput {
	return ProductInput{
		Name: vm.Name, SKU: vm.SKU, Unit: vm.Unit,
		Price: vm.Price, Stock: vm.Stock, MinStock: vm.MinStock, Box: vm.Box, Place: vm.Place, IsActive: vm.IsActive,
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
	return h.renderProductForm(c, fiber.StatusOK, productVM(p))
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
		vm := productVM(p)
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
	return h.renderProductForm(c, status, vm)
}

func (h *SSR) renderProductForm(c fiber.Ctx, status int, vm ProductFormVM) error {
	title := "Запчасть"
	if vm.IsNew {
		title = "Новая запчасть"
	}
	vm.Shell = h.shell.Build(c, title)
	vm.Tabs = h.tabs(c, "products")
	vm.Units = Units
	c.Status(status)
	return h.r.Page(c, "products-form", vm)
}

// fieldText — код ошибки поля → текст под полем.
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
	}
	return "Недопустимое значение"
}
