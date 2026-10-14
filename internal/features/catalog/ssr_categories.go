package catalog

import (
	"errors"
	"net/url"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// ssrCategories — управление категориями встроено в экран «Услуги» (левая панель, task.md):
// создать/переименовать/архив/восстановить/удалить.
type ssrCategories struct {
	svc    *CategoryService
	status *ssr.StatusPages
}

func redirectTab(string) string { return "/catalog/services" }

// listCategoryPanel — категории вида kind (активные и архивные — панель управления
// показывает все, «Показать архивные» списка товаров/услуг на панель не влияет).
// pagination DS-006: lookup с лимитом defaultLimit, TruncVM — для пометки «показаны N из M».
func (h *ssrCategories) list(c fiber.Ctx, kind string) ([]CategoryResponse, TruncVM, error) {
	resp, err := h.svc.List(c.Context(), CategoryListQuery{Kind: kind, IncludeArchived: true, Limit: defaultLimit}, true)
	if err != nil {
		return nil, TruncVM{}, err
	}
	return resp.Items, TruncVM{Shown: len(resp.Items), Total: resp.Total}, nil
}

// createCategory — POST /catalog/categories.
func (h *ssrCategories) create(c fiber.Ctx) error {
	kind := c.FormValue("kind")
	to := redirectTab(kind)
	_, err := h.svc.Create(c.Context(), CategoryInput{Kind: kind, Name: c.FormValue("name")})
	if err != nil {
		return c.Redirect().Status(fiber.StatusSeeOther).To(to + "?cat_error=" + url.QueryEscape(categoryErrCode(err)))
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(to + "?flash=category_created")
}

// rename — POST /catalog/categories/:id/rename.
func (h *ssrCategories) rename(c fiber.Ctx) error {
	kind := c.FormValue("kind")
	to := redirectTab(kind)
	_, err := h.svc.Rename(c.Context(), c.Params("id"), c.FormValue("name"))
	// категории нет либо она мягко удалена (её удалил другой администратор, пока экран открыт)
	// — страница 404 (soft-delete-catalog §8.1 «Состояния», DS-026).
	if errors.Is(err, ErrCategoryNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return c.Redirect().Status(fiber.StatusSeeOther).To(to + "?cat_error=" + url.QueryEscape(categoryErrCode(err)))
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(to + "?flash=category_saved")
}

func (h *ssrCategories) archive(c fiber.Ctx) error { return h.setActive(c, false, "category_archived") }
func (h *ssrCategories) restore(c fiber.Ctx) error { return h.setActive(c, true, "category_restored") }

func (h *ssrCategories) setActive(c fiber.Ctx, active bool, flash string) error {
	to := redirectTab(c.FormValue("kind"))
	err := h.svc.SetActive(c.Context(), c.Params("id"), active)
	// архив и возврат из архива удалённой категории — страница 404 (§6.9, DS-009, DS-026).
	if errors.Is(err, ErrCategoryNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return c.Redirect().Status(fiber.StatusSeeOther).To(to + "?cat_error=" + url.QueryEscape(categoryErrCode(err)))
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(to + "?flash=" + flash)
}

// categoryRemove — POST /catalog/categories/:id/delete, поле формы reason — мягкое удаление
// категории (soft-delete-catalog §6.9, §8.1, DS-032): товары и услуги категории не мешают и
// остаются в каталоге. Отказ по причине — тот же экран списка (422) с окном «Удаление» у строки
// категории и текстом под полем; удалённая либо несуществующая категория — страница 404; успех
// — список с тостом.
func (h *SSR) categoryRemove(c fiber.Ctx) error {
	kind := c.FormValue("kind")
	id := c.Params("id")
	reason := c.FormValue("reason")
	_, err := h.catSvc.Delete(c.Context(), currentActor(c), id, softdelete.DeleteRequest{Reason: &reason})
	if errors.Is(err, ErrCategoryNotFoundErr) {
		return h.status.NotFound(c)
	}
	if err != nil {
		text := softdelete.ReasonError(err)
		if text == "" {
			return err
		}
		c.Locals(catDeleteLocal, CatDeleteVM{ID: id, Reason: reason, Error: text})
		c.Status(fiber.StatusUnprocessableEntity)
		return h.servicesList(c)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(redirectTab(kind) + "?flash=category_deleted")
}

// categoryErrCode — err → КОД (не готовый текст, code review LOW#1: cat_error раньше
// уходил в URL и рендерился как есть — админ по чужой ссылке видел произвольный текст в
// alert; теперь в query — только код из закрытого набора, текст — catErrorText() на
// сервере, эталон flashes). Ошибка валидации name (required/too_long/invalid_text,
// code review LOW#3) — код самого поля, не общее «unknown».
func categoryErrCode(err error) string {
	var app *errorspkg.AppError
	if errors.As(err, &app) {
		for _, f := range app.Fields {
			if f.Name == "name" {
				return f.Message
			}
		}
	}
	switch {
	case errors.Is(err, ErrCategoryNameTaken):
		return "name_taken"
	case errors.Is(err, ErrCategoryNotFoundErr):
		return "not_found"
	default:
		return "unknown"
	}
}

// catErrorText — код cat_error (см. categoryErrCode) → текст под alert на странице
// списка (ssr_products.go/ssr_services.go). Пустая строка на входе — пустая строка на
// выходе (нет ошибки — нет alert).
func catErrorText(code string) string {
	switch code {
	case "":
		return ""
	case msgRequired:
		return "Укажите название"
	case msgTooLong:
		return "Слишком длинное название"
	case msgInvalidText:
		return "Недопустимые символы в названии"
	case "name_taken":
		return ErrCategoryNameTaken.Message
	case "not_found":
		return ErrCategoryNotFoundErr.Message
	default:
		return "Не удалось выполнить действие"
	}
}
