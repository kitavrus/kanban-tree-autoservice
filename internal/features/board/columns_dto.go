package board

import "time"

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}
// (design-go-layers.md §DTO Column).

// ColumnResponse — карточка колонки доски. Порядок в списке — position, id.
type ColumnResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Color         string `json:"color"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
	Position      int    `json:"position"`
	TasksCount    int    `json:"tasks_count"`
}

type ColumnListResponse struct {
	Items  []ColumnResponse `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// ColumnInput — POST/PUT /api/v1/columns[/:id]. Position: POST nil → в конец,
// PUT nil → без перемещения (сервис клэмпит явное значение к 1..N).
type ColumnInput struct {
	Name          string `json:"name"`
	Color         string `json:"color"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
	Position      *int   `json:"position"`
}

// ColumnMutationResponse — ответ Create/Update: Warnings — не nil (пустой массив, не
// null, TS без null — §TD-6.2), непусто только при смене consumes_stock (AC-007).
type ColumnMutationResponse struct {
	Column   ColumnResponse `json:"column"`
	Warnings []StockWarning `json:"warnings"`
}

// ColumnDeleteResponse — ответ DELETE /api/v1/columns/:id (soft-delete-board-columns §6.1):
// мягкое удаление, deleted_at — UTC (RFC 3339).
type ColumnDeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// ColumnRestoreResponse — ответ POST /api/v1/columns/:id/restore (§6.2): position — новая
// позиция колонки, последняя на доске.
type ColumnRestoreResponse struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}
