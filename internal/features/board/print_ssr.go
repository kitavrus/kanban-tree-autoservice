package board

import (
	"context"
	"errors"
	"html/template"
	"slices"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// printService — узкий интерфейс зависимости SSR-печати (эталон cardService/taskService).
type printService interface {
	PrintData(ctx context.Context, a Actor, taskID string) (*PrintVM, error)
}

// PrintVM — данные шаблонов печати (design-go-layers.md §Handler Print, ADR-102).
// Firm с пустыми полями допустим — каждый реквизит в шаблоне выводится своим `{{ if }}`,
// блок реквизитов целиком не опускается разом (иначе одно заполненное поле потерялось бы).
type PrintVM struct {
	Firm        config.FirmInfo
	Kind        string // intake | handover (Enum PrintKind)
	Card        OrderCardResponse
	ClientName  string
	Phones      string
	Description template.HTML // task.description_html (ADR-006) — хранится уже экранированным
	PrintedAt   time.Time
	// Timezone — пояс смотрящего пользователя (IANA, DS-032): дата печати и дата оплаты
	// выводятся в нём ({{ timeIn .Timezone … }}), а не в поясе сервера.
	Timezone string
}

// print — GET /tasks/:id/print/:kind (RequireSSR — исполнитель и admin, D19; Cache-Control:
// no-store ставит сам guard). kind вне PrintKinds или заказ не найден/чужой — 404-страница
// (StatusPages.NotFound), не JSON-ошибка API (это SSR-экран).
func (s *SSR) print(c fiber.Ctx) error {
	kind := c.Params("kind")
	if !slices.Contains(PrintKinds, kind) {
		return s.status.NotFound(c)
	}

	vm, err := s.card.PrintData(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return s.status.NotFound(c)
		}
		return err
	}
	vm.Kind = kind
	vm.Firm = s.firm
	vm.PrintedAt = time.Now()
	vm.Timezone = ssr.ViewerTimezone(c)

	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return s.r.Render(c.Response().BodyWriter(), "print-"+kind, "print", vm)
}
