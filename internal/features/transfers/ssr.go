package transfers

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны экрана «Перемещения» относительно корня модуля.
const Root = "web/ssr/transfers/templates"

// SSR — экран «Перемещения» /transfers (admin и executor, transfers-partners §8.1):
// master-detail; список, документ и действия рисует js/transfers.js поверх JSON API, открытый
// документ — ?id=N, новый из карточки заказа — ?new=1&direction=…&task_id=N. Склады фильтра и
// шапки документа — SSR-списком (все, архивные с пометкой). Исполнителю доступны только
// направления с клиентом (DS-018): меню направлений и фильтры урезаны шаблоном по IsAdmin.
type SSR struct {
	svc    *Service
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(svc *Service, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{svc: svc, r: r, shell: shell, status: status}
}

// WarehouseOption — склад select'ов экрана (архивный — только в фильтре, с пометкой).
type WarehouseOption struct {
	ID       string
	Name     string
	IsActive bool
}

// PageVM — оболочка + склады + роль; документы грузит JS.
type PageVM struct {
	Shell      ssr.Shell
	Warehouses []WarehouseOption
	IsAdmin    bool
}

// Warehouses — все склады по position (вкл. архивные).
func (s *Service) Warehouses(ctx context.Context) ([]WarehouseOption, error) {
	rows, err := s.repo.pool.Query(ctx, `SELECT id::text, name, is_active FROM warehouses WHERE deleted_at IS NULL ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WarehouseOption{}
	for rows.Next() {
		var w WarehouseOption
		if err := rows.Scan(&w.ID, &w.Name, &w.IsActive); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// page — GET /transfers.
func (h *SSR) page(c fiber.Ctx) error {
	warehouses, err := h.svc.Warehouses(c.Context())
	if err != nil {
		return err
	}
	sh := h.shell.Build(c, "Перемещения")
	return h.r.Page(c, "list", PageVM{Shell: sh, Warehouses: warehouses, IsAdmin: sh.IsAdmin})
}
