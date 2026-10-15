// Package calendar — календарь загрузки сервиса: сколько машин одновременно стоит в ремонте
// по дням и есть ли свободные места при заданной вместимости (настройка 5–12 машин).
//
// Машина (заказ) занимает место с дня начала работ (tasks.start_date; заказ с будущим началом —
// только с этого дня):
//   - выданный заказ (финальная колонка) — по день выдачи (finished_at в поясе пользователя)
//     включительно;
//   - не выданный с плановым окончанием (due_date = start_date + duration_days − 1) сегодня или
//     позже — по плановое окончание включительно;
//   - не выданный с прошедшим плановым окончанием — бессрочно: машина стоит, пока её не
//     выдадут, поэтому занимает место во всех будущих днях (open_ended).
//
// Заказ, выданный раньше плановой даты начала (отмена/ошибка записи), места не занимает — ни в
// сетке, ни на диаграмме Ганта (одно правило для любого окна просмотра).
package calendar

// Settings — настройки календаря (GET/PUT /api/v1/calendar/settings).
type Settings struct {
	MaxCars int `json:"max_cars"`
}

// SettingsRequest — тело PUT /api/v1/calendar/settings. Указатель — отличить «не передано».
type SettingsRequest struct {
	MaxCars *int `json:"max_cars"`
}

// Day — загрузка одного дня.
type Day struct {
	Date     string  `json:"date"`
	Occupied int     `json:"occupied"`
	Free     int     `json:"free"` // 0, если мест нет или перегруз
	TaskIDs  []int64 `json:"task_ids"`
}

// Order — заказ, занимающий место хотя бы в одном дне окна.
type Order struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	ClientName  string  `json:"client_name"`
	ColumnName  string  `json:"column_name"`
	ColumnColor string  `json:"column_color"`
	StartDate   string  `json:"start_date"`
	DueDate     *string `json:"due_date"`
	EndDate     *string `json:"end_date"` // nil — бессрочно (open_ended)
	OpenEnded   bool    `json:"open_ended"`
	IsFinished  bool    `json:"is_finished"`
}

// CalendarResponse — GET /api/v1/calendar?from=&to=.
type CalendarResponse struct {
	Capacity int     `json:"capacity"`
	Today    string  `json:"today"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Days     []Day   `json:"days"`
	Orders   []Order `json:"orders"`
}
