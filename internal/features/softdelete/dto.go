package softdelete

import "time"

// DeletedBy — кто удалил.
type DeletedBy struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Item — строка раздела «Удалённые» (§6.3), одна форма для всех видов.
type Item struct {
	Kind         string    `json:"kind"`
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Subtitle     string    `json:"subtitle"`
	DeletedAt    time.Time `json:"deleted_at"`
	DeletedBy    DeletedBy `json:"deleted_by"`
	DeleteReason string    `json:"delete_reason"`
	// DeletedWith — у оборудования, удалённого вместе с клиентом, — "client"; иначе null.
	DeletedWith *string `json:"deleted_with"`
	IsArchived  bool    `json:"is_archived"`
}

// ListResponse — конверт списка §API-1.
type ListResponse struct {
	Items  []Item `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// SummaryResponse — счётчики вкладок: ключ — Kind.CountKey().
type SummaryResponse struct {
	Counts map[string]int `json:"counts"`
}

// ListQuery — сырые query-параметры GET /api/v1/deleted. Limit/Offset — строки: «не число»
// отличается от «не передано».
type ListQuery struct {
	Kind, Q, DeletedFrom, DeletedTo, DeletedBy, Limit, Offset string
}
