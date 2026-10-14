package board

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

const (
	assigneesDefaultLimit = 200
	assigneesMaxLimit     = 1000
)

// AssigneeResponse/AssigneeListResponse — design-go-layers.md §DTO Assignee. Порядок:
// name, id. Только is_active (форма заказа предлагает только активных пользователей).
type AssigneeResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type AssigneeListResponse struct {
	Items  []AssigneeResponse `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// AssigneesHandler — GET /api/v1/assignees (design-sql.md §Assignees Query). Однострочный
// запрос — отдельный repo/service не заводится (единственный метод, эталон design-go-
// layers.md §Package Layout: только assignees_handler.go в списке файлов фазы).
type AssigneesHandler struct {
	pool *pgxpool.Pool
}

func NewAssigneesHandler(pool *pgxpool.Pool) *AssigneesHandler { return &AssigneesHandler{pool: pool} }

// List — GET /api/v1/assignees?limit=&offset=. limit/offset — тот же паттерн явной 400-
// валидации, что у tasks_filter.go/stock_handler.go (code review mobile M-01: мусор/выход
// за диапазон → 400 field, а не тихий clamp к дефолту).
func (h *AssigneesHandler) List(c fiber.Ctx) error {
	var fields []errorspkg.Field
	add := func(name string) { fields = append(fields, errorspkg.Field{Name: name, Message: msgInvalid}) }

	limit := assigneesDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > assigneesMaxLimit {
			add("limit")
		} else {
			limit = n
		}
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			add("offset")
		} else {
			offset = n
		}
	}
	if len(fields) > 0 {
		return errorspkg.Validation(fields...)
	}

	resp, err := h.list(c.Context(), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (h *AssigneesHandler) list(ctx context.Context, limit, offset int) (*AssigneeListResponse, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id, name, role, count(*) OVER () FROM users WHERE is_active ORDER BY name, id LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var total int
	items := []AssigneeResponse{}
	for rows.Next() {
		var a AssigneeResponse
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &total); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &AssigneeListResponse{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}
