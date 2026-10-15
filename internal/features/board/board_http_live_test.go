//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// BoardLiveSuite (§TD-8): доска — колонки, заказы, move, склад, фильтры, права,
// конкурентность. Реальный бинарь + TCP + реальный логин; auth-стабы и app.Test (кроме
// *_fuzz_test.go) запрещены.
type BoardLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestBoardLiveSuite(t *testing.T) {
	suite.Run(t, &BoardLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *BoardLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("board-admin", "password1", "Доска Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("board-admin", "password1")
}

// ---- DTO wire-типы (1:1 с internal/features/board/*_dto.go, §TD-6.2) ----

type columnDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Color         string `json:"color"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
	Position      int    `json:"position"`
	TasksCount    int    `json:"tasks_count"`
}

type columnListDTO struct {
	Items  []columnDTO `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type stockWarningDTO struct {
	ProductID string      `json:"product_id"`
	Name      string      `json:"name"`
	Stock     json.Number `json:"stock"` // numeric.Amount на wire — JSON-число без кавычек
	MinStock  json.Number `json:"min_stock"`
	Kind      string      `json:"kind"`
}

type taskCardDTO struct {
	ID            int64   `json:"id"`
	ColumnID      string  `json:"column_id"`
	ClientID      string  `json:"client_id"`
	ClientName    string  `json:"client_name"`
	Title         string  `json:"title"`
	AssigneeID    *string `json:"assignee_id"`
	AssigneeName  *string `json:"assignee_name"`
	StartDate     string  `json:"start_date"`
	DurationDays  int     `json:"duration_days"`
	DueDate       *string `json:"due_date"`
	IsPaid        bool    `json:"is_paid"`
	Position      string  `json:"position"`
	Version       int64   `json:"version"`
	SubtasksTotal int     `json:"subtasks_total"`
	SubtasksDone  int     `json:"subtasks_done"`
	SubtasksLate  int     `json:"subtasks_late"`
}

type taskDTO struct {
	taskCardDTO
	PaidAt    *string `json:"paid_at"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type taskListDTO struct {
	Items  []taskCardDTO `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type taskMutationDTO struct {
	Task     taskDTO           `json:"task"`
	Warnings []stockWarningDTO `json:"warnings"`
}

type apiErrSubtaskDTO struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
}

type apiErrDTO struct {
	Code    string `json:"code"`
	Details *struct {
		Fields   []struct{ Name, Message string } `json:"fields"`
		Subtasks []apiErrSubtaskDTO               `json:"subtasks"`
	} `json:"details"`
}

func (s *BoardLiveSuite) errOf(body []byte) apiErrDTO {
	var e apiErrDTO
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *BoardLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

// ---- фикстуры (клиент/товар/услуга — напрямую SQL: не входят в API этой фазы) ----

func (s *BoardLiveSuite) newClient(company string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO clients (company_name) VALUES ($1) RETURNING id`, company).Scan(&id))
	return id
}

// columnID — id одной из 5 сидовых колонок D32 по имени.
func (s *BoardLiveSuite) columnID(name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM columns WHERE name = $1`, name).Scan(&id))
	return id
}

func (s *BoardLiveSuite) createProduct(stock, minStock string) string {
	ctx := context.Background()
	var productID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (name, stock, min_stock) VALUES ('Товар', $1::numeric, $2::numeric) RETURNING id`,
		stock, minStock).Scan(&productID))
	return productID
}

func (s *BoardLiveSuite) productStock(id string) string {
	var stock string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT stock::text FROM products WHERE id = $1`, id).Scan(&stock))
	return stock
}

func (s *BoardLiveSuite) newServiceWithCategory() (categoryID, serviceID string) {
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'BoardLive Svc ' || gen_random_uuid()) RETURNING id`).Scan(&categoryID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO services (category_id, name) VALUES ($1, 'Услуга') RETURNING id`, categoryID).Scan(&serviceID))
	return categoryID, serviceID
}

// addSubtask — подзадача-услуга заказа (subtasks вне API фазы 5, появится в фазе 6).
func (s *BoardLiveSuite) addSubtask(taskID int64, serviceID, status string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO subtasks (task_id, service_id, status) VALUES ($1, $2, $3) RETURNING id`,
		taskID, serviceID, status).Scan(&id))
	return id
}

func (s *BoardLiveSuite) addSubtaskProduct(subtaskID, productID, qty string) {
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1, $2, $3::numeric)`, subtaskID, productID, qty)
	s.Require().NoError(err)
}

// ---- HTTP-хелперы ----

// createTask — POST /tasks; для негативных сценариев (400) тело ответа — errorBody, не
// taskMutationDTO, поэтому Task не парсится (raw остаётся источником для s.fields/s.errOf).
func (s *BoardLiveSuite) createTask(b *testutil.Browser, body map[string]any) (int, taskDTO, []byte) {
	st, m, raw := s.mutate(b, http.MethodPost, "/api/v1/tasks", body)
	return st, m.Task, raw
}

func (s *BoardLiveSuite) mutate(b *testutil.Browser, method, path string, body map[string]any) (int, taskMutationDTO, []byte) {
	st, raw := b.Do(method, path, body)
	var m taskMutationDTO
	if st == http.StatusOK || st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	}
	return st, m, raw
}

func (s *BoardLiveSuite) getTask(b *testutil.Browser, id int64) (int, taskDTO, []byte) {
	st, raw := b.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), nil)
	var t taskDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	}
	return st, t, raw
}

func (s *BoardLiveSuite) newTask(clientID, columnID string) taskDTO {
	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{
		"title": "Заказ", "client_id": clientID, "column_id": columnID,
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return m.Task
}

func taskCard(t taskDTO, patch map[string]any) map[string]any {
	m := map[string]any{
		"title": t.Title, "column_id": t.ColumnID,
		"assignee_id": t.AssigneeID, "start_date": t.StartDate, "duration_days": t.DurationDays,
		"is_paid": t.IsPaid, "version": t.Version,
	}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *BoardLiveSuite) createColumn(b *testutil.Browser, body map[string]any) (int, columnDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/columns", body)
	var m struct {
		Column columnDTO `json:"column"`
	}
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	}
	return st, m.Column, raw
}

// ---- Happy Path ----

// LV-001.
func (s *BoardLiveSuite) TestBoardLiveSuite_Columns_SeedPresent() {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list columnListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Require().GreaterOrEqual(len(list.Items), 7)

	byName := map[string]columnDTO{}
	for _, c := range list.Items {
		byName[c.Name] = c
	}
	for _, name := range []string{"Приёмка", "Диагностика", "Согласование", "Ждёт запчасти", "В ремонте", "Готов к выдаче", "Выдан"} {
		_, ok := byName[name]
		s.True(ok, "колонка %q должна присутствовать (D32)", name)
	}
	s.True(byName["Выдан"].IsFinal)
	s.True(byName["Выдан"].ConsumesStock)
	s.False(byName["Приёмка"].IsFinal)
	s.False(byName["Приёмка"].ConsumesStock)
	s.True(byName["В ремонте"].ConsumesStock)
	s.False(byName["Диагностика"].ConsumesStock)
	s.True(byName["Готов к выдаче"].ConsumesStock)
	s.False(byName["Готов к выдаче"].IsFinal)

	// порядок по position.
	prev := -1
	for _, c := range list.Items {
		s.Greater(c.Position, prev)
		prev = c.Position
	}
}

// LV-002.
func (s *BoardLiveSuite) TestBoardLiveSuite_CreateTask() {
	c := s.newClient("Клиент-Создание-Заказа")
	col := s.columnID("Приёмка")

	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{
		"title": "Первый заказ", "client_id": c, "column_id": col,
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Greater(m.Task.ID, int64(0))
	s.Equal("Первый заказ", m.Task.Title)
	s.Equal(int64(1), m.Task.Version)
	s.False(m.Task.IsPaid)
	s.Equal(c, m.Task.ClientID)
	s.NotEmpty(m.Task.ClientName)

	st, got, raw := s.getTask(s.admin, m.Task.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(m.Task.ID, got.ID)

	// второй заказ в ту же колонку — id растёт (bigserial, D37).
	st2, m2, raw2 := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{
		"title": "Второй заказ", "client_id": c, "column_id": col,
	}))
	s.Require().Equal(http.StatusCreated, st2, string(raw2))
	s.Greater(m2.Task.ID, m.Task.ID)
	s.Greater(m2.Task.Position, m.Task.Position, "в конец колонки")
}

// LV-003.
func (s *BoardLiveSuite) TestBoardLiveSuite_UpdateTask_VersionIncrements() {
	c := s.newClient("Клиент-Правка-Заказа")
	col := s.columnID("Приёмка")
	t := s.newTask(c, col)

	st, m, raw := s.mutate(s.admin, http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"title": "Правка"}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("Правка", m.Task.Title)
	s.Equal(int64(2), m.Task.Version)
}

// LV-004.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_BetweenColumns() {
	c := s.newClient("Клиент-Move-Между-Колонками")
	newCol, soglCol := s.columnID("Приёмка"), s.columnID("Согласование")
	t := s.newTask(c, newCol)

	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": soglCol})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(soglCol, m.Task.ColumnID)
	s.Equal(int64(2), m.Task.Version)

	st, got, raw := s.getTask(s.admin, t.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(soglCol, got.ColumnID, "GET /tasks/:id отражает новую колонку")
}

// LV-005.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_WithinColumn_PrevNext() {
	c := s.newClient("Клиент-Move-Внутри-Колонки")
	col := s.columnID("Приёмка")
	t1 := s.newTask(c, col)
	t2 := s.newTask(c, col)
	t3 := s.newTask(c, col)
	s.Require().Less(t1.Position, t2.Position)
	s.Require().Less(t2.Position, t3.Position)

	// t3 между t1 и t2.
	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t3.ID)+"/move",
		map[string]any{"column_id": col, "prev_id": t1.ID, "next_id": t2.ID})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Greater(parseAmount(m.Task.Position), parseAmount(t1.Position))
	s.Require().Less(parseAmount(m.Task.Position), parseAmount(t2.Position))

	// верх колонки (только next).
	st, m, raw = s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t2.ID)+"/move",
		map[string]any{"column_id": col, "next_id": t1.ID})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Less(parseAmount(m.Task.Position), parseAmount(t1.Position))
}

// LV-036.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_ForeignNeighborIgnored() {
	c := s.newClient("Клиент-Move-Чужой-Сосед")
	colA, colB := s.columnID("Приёмка"), s.columnID("Согласование")
	t := s.newTask(c, colA)
	foreign := s.newTask(c, colB) // сосед из другой колонки

	// prev_id из чужой колонки — игнорируется, позиция как «конец»/«новая колонка».
	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move",
		map[string]any{"column_id": colA, "prev_id": foreign.ID})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(colA, m.Task.ColumnID)

	// несуществующий сосед — тоже игнорируется, не 404.
	st, _, raw = s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move",
		map[string]any{"column_id": colA, "prev_id": int64(999999999)})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// сосед = сам заказ — игнорируется.
	st, _, raw = s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move",
		map[string]any{"column_id": colA, "prev_id": t.ID})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// LV-006/AC-004.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_ConsumesStock() {
	c := s.newClient("Клиент-Списание")
	newCol, wipCol, waitCol := s.columnID("Приёмка"), s.columnID("В ремонте"), s.columnID("Ждёт запчасти")
	productID := s.createProduct("10", "1")
	t := s.newTask(c, newCol)
	_, svcID := s.newServiceWithCategory()
	subtaskID := s.addSubtask(t.ID, svcID, "new")
	s.addSubtaskProduct(subtaskID, productID, "3")

	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": wipCol})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("7.000", s.productStock(productID))
	s.Empty(m.Warnings)

	var movements int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE task_id = $1 AND reason = 'task_consume'`, t.ID).Scan(&movements))
	s.Equal(1, movements)

	// назад в «Согласование» (не consumes) — возврат.
	sogl := s.columnID("Согласование")
	st, _, raw = s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": sogl})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("10.000", s.productStock(productID))

	// «В ремонте»→«Ждёт запчасти» (обе consumes) — 0 новых движений.
	st, _, raw = s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": wipCol})
	s.Require().Equal(http.StatusOK, st, string(raw))
	before := 0
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE task_id = $1`, t.ID).Scan(&before))
	st, _, raw = s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": waitCol})
	s.Require().Equal(http.StatusOK, st, string(raw))
	after := 0
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE task_id = $1`, t.ID).Scan(&after))
	s.Equal(before, after, "переход между двумя consumes-колонками не создаёт новых движений — спрос не изменился")
	assertStockInvariant(s.T(), s.Pool)
}

// LV-007/AC-005.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_NegativeStockWarning() {
	c := s.newClient("Клиент-Недостача")
	newCol, wipCol := s.columnID("Приёмка"), s.columnID("В ремонте")
	productID := s.createProduct("1", "5")
	t := s.newTask(c, newCol)
	_, svcID := s.newServiceWithCategory()
	subtaskID := s.addSubtask(t.ID, svcID, "new")
	s.addSubtaskProduct(subtaskID, productID, "3")

	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": wipCol})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("-2.000", s.productStock(productID))
	s.Require().Len(m.Warnings, 1)
	s.Equal("negative", m.Warnings[0].Kind)
	assertStockInvariant(s.T(), s.Pool)
}

// LV-008/AC-007. Собственная колонка (не сидовая «В ремонте») — тест мутирует
// consumes_stock туда-обратно и не должен влиять на общий стенд остальных тестов сьюты.
func (s *BoardLiveSuite) TestBoardLiveSuite_ColumnToggleConsumes_RecalcAll() {
	st, col, raw := s.createColumn(s.admin, map[string]any{
		"name": "Toggle-Колонка", "color": "#2563EB", "is_final": false, "consumes_stock": true,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	c := s.newClient("Клиент-Toggle-Колонки")
	p1, p2 := s.createProduct("10", "1"), s.createProduct("10", "1")
	_, svcID := s.newServiceWithCategory()

	t1 := s.newTask(c, col.ID)
	sub1 := s.addSubtask(t1.ID, svcID, "new")
	s.addSubtaskProduct(sub1, p1, "2")
	t2 := s.newTask(c, col.ID)
	sub2 := s.addSubtask(t2.ID, svcID, "new")
	s.addSubtaskProduct(sub2, p2, "3")

	// подзадачи/товары добавлены НАПРЯМУЮ SQL уже ПОСЛЕ Create — Reconcile при создании
	// заказа отработал по нулевому спросу (единообразие, no-op); склад пока не тронут.
	s.Equal("10.000", s.productStock(p1))
	s.Equal("10.000", s.productStock(p2))

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/columns/"+col.ID, map[string]any{
		"name": "Toggle-Колонка", "color": "#2563EB", "is_final": false, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("10.000", s.productStock(p1))
	s.Equal("10.000", s.productStock(p2))

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/columns/"+col.ID, map[string]any{
		"name": "Toggle-Колонка", "color": "#2563EB", "is_final": false, "consumes_stock": true,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("8.000", s.productStock(p1))
	s.Equal("7.000", s.productStock(p2))
	assertStockInvariant(s.T(), s.Pool)
}

// LV-009/AC-006.
func (s *BoardLiveSuite) TestBoardLiveSuite_ColumnsCRUD() {
	st, col, raw := s.createColumn(s.admin, map[string]any{
		"name": "CRUD-Колонка", "color": "#112233", "is_final": false, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.NotEmpty(col.ID)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/columns/"+col.ID, map[string]any{
		"name": "CRUD-Переименовано", "color": "#445566", "is_final": false, "consumes_stock": false, "position": 1,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, st)
	var list columnListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	var found columnDTO
	for _, c := range list.Items {
		if c.ID == col.ID {
			found = c
		}
	}
	s.Equal("CRUD-Переименовано", found.Name)
	s.Equal("#445566", found.Color)
	s.Equal(1, found.Position)

	// soft-delete-board-columns §6.1: удаление — мягкое, с причиной: 200 {id, deleted_at};
	// строка остаётся с отметкой, из списка колонка уходит. Без причины — 400.
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/columns/"+col.ID, nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["reason"])
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/columns/"+col.ID, map[string]any{"reason": "Создана по ошибке"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var del struct {
		ID        string `json:"id"`
		DeletedAt string `json:"deleted_at"`
	}
	s.Require().NoError(json.Unmarshal(raw, &del), string(raw))
	s.Equal(col.ID, del.ID)
	s.NotEmpty(del.DeletedAt)
	var marked bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT deleted_at IS NOT NULL FROM columns WHERE id = $1`, col.ID).Scan(&marked))
	s.True(marked, "строка колонки на месте, с отметкой удаления")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.NotContains(string(raw), col.ID, "удалённой колонки нет в списке")
}

// LV-010/AC-012.
func (s *BoardLiveSuite) TestBoardLiveSuite_DeleteTask_ReturnsStock() {
	c := s.newClient("Клиент-Удаление-Заказа")
	newCol, wipCol := s.columnID("Приёмка"), s.columnID("В ремонте")
	productID := s.createProduct("10", "1")
	t := s.newTask(c, newCol)
	_, svcID := s.newServiceWithCategory()
	subtaskID := s.addSubtask(t.ID, svcID, "new")
	s.addSubtaskProduct(subtaskID, productID, "4")

	// move в consumes-колонку — только теперь Reconcile видит спрос и списывает.
	st, _, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": wipCol})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("6.000", s.productStock(productID))

	// soft-delete-core DS-004: удаление мягкое, с причиной — товар возвращён на склад, строка
	// заказа, его услуги и движения остатка остаются (подробно — soft_delete_http_live_test.go).
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/tasks/"+itoa(t.ID), map[string]any{"reason": "Создан по ошибке"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("10.000", s.productStock(productID))

	var stillReferencing int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE task_id = $1`, t.ID).Scan(&stillReferencing))
	s.Equal(2, stillReferencing, "движения заказа сохраняют ссылку: списание и возврат")

	var subtasksLeft int
	var marked bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM subtasks WHERE task_id = $1), deleted_at IS NOT NULL FROM tasks WHERE id = $1`, t.ID).
		Scan(&subtasksLeft, &marked))
	s.Equal(1, subtasksLeft, "услуги удалённого заказа остаются")
	s.True(marked, "строка заказа помечена удалённой")

	st, _, raw = s.getTask(s.admin, t.ID)
	s.Equal(http.StatusNotFound, st, string(raw))
	assertStockInvariant(s.T(), s.Pool)
}

// LV-011/AC-011.
func (s *BoardLiveSuite) TestBoardLiveSuite_AdminTogglesPaid() {
	c := s.newClient("Клиент-Оплата")
	col := s.columnID("Приёмка")
	t := s.newTask(c, col)
	s.False(t.IsPaid)
	s.Nil(t.PaidAt)

	st, m, raw := s.mutate(s.admin, http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"is_paid": true}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(m.Task.IsPaid)
	s.NotNil(m.Task.PaidAt)

	st, m, raw = s.mutate(s.admin, http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(m.Task, map[string]any{"is_paid": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(m.Task.IsPaid)
	s.Nil(m.Task.PaidAt)
}

// ---- Filters ----

// LV-020/AC-013.
func (s *BoardLiveSuite) TestBoardLiveSuite_Filters_Combination() {
	c1 := s.newClient("Фильтр-Клиент-Раз")
	c2 := s.newClient("Фильтр-Клиент-Два")
	col := s.columnID("Приёмка")
	t1 := s.newTask(c1, col)
	s.newTask(c2, col)

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c1, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list taskListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	for _, it := range list.Items {
		s.Equal(c1, it.ClientID)
	}

	// q по #N.
	st, raw = s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks?q=%%23%d", t1.ID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Require().GreaterOrEqual(list.Total, 1)
	found := false
	for _, it := range list.Items {
		if it.ID == t1.ID {
			found = true
		}
	}
	s.True(found)

	// paid=paid / unpaid.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?paid=paid&client_id="+c1, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(0, list.Total, "новый заказ не оплачен")
}

// plan.md §Доска/§Оборудование клиента: q ищет по описанию, услуге, товару и госномеру
// автомобиля заказа (не только по названию/клиенту/#N).
func (s *BoardLiveSuite) TestBoardLiveSuite_Filters_SearchAllFields() {
	ctx := context.Background()
	client := s.newClient("Поиск-Клиент")
	target := s.newTask(client, s.columnID("Приёмка"))
	other := s.newTask(client, s.columnID("Приёмка"))

	// Описание хранится экранированным (ADR-006) — ищем по исходному тексту с & и кавычкой.
	_, err := s.Pool.Exec(ctx, `UPDATE tasks SET description_html = $2 WHERE id = $1`,
		target.ID, `<p>Стучит &amp; &#34;греется&#34; двигатель</p>`)
	s.Require().NoError(err)

	_, serviceID := s.newServiceWithCategory()
	_, err = s.Pool.Exec(ctx, `UPDATE services SET name = 'Замена-турбины-QX7' WHERE id = $1`, serviceID)
	s.Require().NoError(err)
	subID := s.addSubtask(target.ID, serviceID, "new")
	productID := s.createProduct("10", "0")
	_, err = s.Pool.Exec(ctx, `UPDATE products SET name = 'Фильтр-ZK42' WHERE id = $1`, productID)
	s.Require().NoError(err)
	s.addSubtaskProduct(subID, productID, "1")

	// автомобиль заказа (one-car-order: tasks.car_id) — госномер для поиска.
	_, err = s.Pool.Exec(ctx,
		`UPDATE equipment SET plate_number = 'Р900ХК 177' WHERE id = (SELECT car_id FROM tasks WHERE id = $1)`, target.ID)
	s.Require().NoError(err)

	search := func(q string) []int64 {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+client+"&q="+url.QueryEscape(q), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var list taskListDTO
		s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
		ids := make([]int64, 0, len(list.Items))
		for _, it := range list.Items {
			ids = append(ids, it.ID)
		}
		return ids
	}

	for _, q := range []string{`стучит & "греется"`, "турбины-qx7", "фильтр-zk42", "р900хк"} {
		ids := search(q)
		s.Contains(ids, target.ID, "q=%q", q)
		s.NotContains(ids, other.ID, "q=%q", q)
	}
	s.Empty(search("нет-такого-текста-9f3"), "лишние совпадения")
	// Теги хранения описания не участвуют в поиске как текст.
	s.NotContains(search("<p>"), target.ID)
}

// Госномер: поиск доски находит заказ по госномеру его автомобиля (tasks.car_id) без учёта
// регистра, пробелов и дефисов («а123вс777» → «А123ВС 777»); карточка заказа отдаёт car.plate_number.
func (s *BoardLiveSuite) TestBoardLiveSuite_Filters_SearchByPlateNumber() {
	ctx := context.Background()
	client := s.newClient("Госномер-Поиск-Клиент")
	target := s.newTask(client, s.columnID("Приёмка"))
	other := s.newTask(client, s.columnID("Приёмка"))

	_, err := s.Pool.Exec(ctx,
		`UPDATE equipment SET plate_number = 'Х987КМ 199' WHERE id = (SELECT car_id FROM tasks WHERE id = $1)`, target.ID)
	s.Require().NoError(err)

	search := func(q string) []int64 {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+client+"&q="+url.QueryEscape(q), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var list taskListDTO
		s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
		ids := make([]int64, 0, len(list.Items))
		for _, it := range list.Items {
			ids = append(ids, it.ID)
		}
		return ids
	}
	for _, q := range []string{"Х987КМ 199", "х987км199", "987 км", "км-199"} {
		ids := search(q)
		s.Contains(ids, target.ID, "q=%q", q)
		s.NotContains(ids, other.ID, "q=%q", q)
	}
	s.Empty(search("Х000ХХ"), "лишние совпадения")

	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", target.ID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var card struct {
		Car struct {
			PlateNumber string `json:"plate_number"`
		} `json:"car"`
	}
	s.Require().NoError(json.Unmarshal(raw, &card), string(raw))
	s.Equal("Х987КМ 199", card.Car.PlateNumber)
}

// LV-021/AC-014.
func (s *BoardLiveSuite) TestBoardLiveSuite_Filters_AssigneeDefaultByRole() {
	s.SeedUser("board-exec-assignee", "password1", "Доска Исп Ответственный", "executor", true)
	exec := s.NewBrowser()
	exec.Login("board-exec-assignee", "password1")

	var execID string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE login = 'board-exec-assignee'`).Scan(&execID))

	c := s.newClient("Клиент-Ответственный")
	col := s.columnID("Приёмка")
	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{
		"title": "Мой заказ", "client_id": c, "column_id": col, "assignee_id": execID,
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	other := s.newTask(c, col) // без ответственного — чужой для executor
	_ = other

	// executor без assignee — видит только свои.
	st, raw = exec.Do(http.MethodGet, "/api/v1/tasks?client_id="+c, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list taskListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	for _, it := range list.Items {
		s.Require().NotNil(it.AssigneeID)
		s.Equal(execID, *it.AssigneeID)
	}
	s.Require().Contains(idsOf(list.Items), m.Task.ID)

	// assignee=all — видит все.
	st, raw = exec.Do(http.MethodGet, "/api/v1/tasks?assignee=all&client_id="+c, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(2, list.Total)

	// admin без параметра — все.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(2, list.Total)
}

// LV-022/AC-013.
func (s *BoardLiveSuite) TestBoardLiveSuite_Filters_LimitOffsetTotal() {
	c := s.newClient("Клиент-Лимит-Оффсет")
	col := s.columnID("Приёмка")
	for i := 0; i < 3; i++ {
		s.newTask(c, col)
	}

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c+"&limit=2&offset=0", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list taskListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(3, list.Total)
	s.Len(list.Items, 2)
	s.Equal(2, list.Limit)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?limit=1001", nil)
	s.Equal(http.StatusBadRequest, st, string(raw))

	// дефолты.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(500, list.Limit)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, st)
	var clist columnListDTO
	s.Require().NoError(json.Unmarshal(raw, &clist))
	s.Equal(100, clist.Limit)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/assignees", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var alist struct {
		Limit int `json:"limit"`
	}
	s.Require().NoError(json.Unmarshal(raw, &alist))
	s.Equal(200, alist.Limit)
}

// ---- Invalid Input (TD-4) ----

// LV-030.
func (s *BoardLiveSuite) TestBoardLiveSuite_Auth_NoSession401() {
	anon := s.NewBrowser()
	for _, cs := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/columns"},
		{http.MethodGet, "/api/v1/tasks"},
		{http.MethodGet, "/api/v1/assignees"},
	} {
		st, _ := anon.Do(cs.method, cs.path, nil)
		s.Equal(http.StatusUnauthorized, st, "%s %s", cs.method, cs.path)
	}
}

// LV-031.
func (s *BoardLiveSuite) TestBoardLiveSuite_Auth_NoCSRF403() {
	c := s.newClient("Клиент-Без-CSRF")
	col := s.columnID("Приёмка")
	resp := s.admin.Request(http.MethodPost, "/api/v1/tasks",
		withNewCar(map[string]any{"title": "x", "client_id": c, "column_id": col}), map[string]string{"X-Csrf-Token": ""})
	defer resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)
}

// LV-032.
func (s *BoardLiveSuite) TestBoardLiveSuite_GarbageBody400() {
	for _, body := range []string{`{`, `null`, `"str"`, `[]`, `{"prev_id":"x"}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}
	c := s.newClient("Клиент-Мусор-Move")
	col := s.columnID("Приёмка")
	t := s.newTask(c, col)
	for _, body := range []string{`{"prev_id":"x"}`, `{"stock":"5"}`, `[]`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}
}

// LV-033/AC-009.
func (s *BoardLiveSuite) TestBoardLiveSuite_TaskValidation400() {
	c := s.newClient("Клиент-Валидация-Заказа")
	col := s.columnID("Приёмка")

	// one-car-order: заказа без автомобиля нет — прежний режим «title + client_id» → 400.
	st, _, raw := s.createTask(s.admin, withPlan(map[string]any{"title": "Без машины", "client_id": c, "column_id": col}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["plate_number"])
	s.Equal("required", s.fields(raw)["equipment_id"])

	// title не обязателен — генерируется из госномера.
	st, created, raw := s.createTask(s.admin, withPlan(map[string]any{
		"client_id": c, "column_id": col, "plate_number": "ЮТЕСТ", "new_equipment": map[string]any{},
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("ЮТЕСТ", created.Title)

	st, _, raw = s.createTask(s.admin, withNewCar(map[string]any{
		"title": stringsRepeat("а", 201), "client_id": c, "column_id": col,
	}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_long", s.fields(raw)["title"])

	st, _, raw = s.createTask(s.admin, withNewCar(map[string]any{"title": "x", "column_id": col}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["client_id"])

	unknown := "00000000-0000-0000-0000-000000000000"
	st, _, raw = s.createTask(s.admin, withNewCar(map[string]any{"title": "x", "client_id": unknown, "column_id": col}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["client_id"])

	st, _, raw = s.createTask(s.admin, withNewCar(map[string]any{"title": "x", "client_id": c, "column_id": unknown}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["column_id"])

	st, _, raw = s.createTask(s.admin, withNewCar(map[string]any{"title": "x", "client_id": c, "column_id": col, "assignee_id": unknown}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["assignee_id"])

	st, _, raw = s.createTask(s.admin, withNewCar(map[string]any{"title": "x", "client_id": c, "column_id": col, "start_date": "2026-13-40"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_date", s.fields(raw)["start_date"])
}

// order-start-duration (TD-4 валидация): POST без плана работ → 400 по каждому полю; формат/окно
// даты, границы 1..60; due_date во входе не принимается (неизвестное поле → 400, не 500);
// мусорный тип duration_days → 400.
func (s *BoardLiveSuite) TestBoardLiveSuite_TaskPlanValidation400() {
	c := s.newClient("Клиент-План-Валидация")
	col := s.columnID("Приёмка")
	body := func(patch map[string]any) map[string]any {
		m := map[string]any{"title": "План", "client_id": c, "column_id": col}
		for k, v := range patch {
			m[k] = v
		}
		return withNewCar(m)
	}

	st, _, raw := s.createTask(s.admin, body(map[string]any{"start_date": nil, "duration_days": nil}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"start_date": "required", "duration_days": "required"}, s.fields(raw))

	st, _, raw = s.createTask(s.admin, body(map[string]any{"start_date": "  "}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["start_date"])

	for _, tc := range []struct {
		patch       map[string]any
		field, code string
	}{
		{map[string]any{"start_date": "15.10.2026"}, "start_date", "invalid_date"},
		{map[string]any{"start_date": "2026-02-30"}, "start_date", "invalid_date"},
		{map[string]any{"start_date": "0000-01-01"}, "start_date", "out_of_range"},
		{map[string]any{"start_date": "2100-01-01"}, "start_date", "out_of_range"},
		{map[string]any{"duration_days": 0}, "duration_days", "out_of_range"},
		{map[string]any{"duration_days": 61}, "duration_days", "out_of_range"},
		{map[string]any{"duration_days": -1}, "duration_days", "out_of_range"},
	} {
		st, _, raw = s.createTask(s.admin, body(tc.patch))
		s.Equal(http.StatusBadRequest, st, "%v → %s", tc.patch, raw)
		s.Equal(tc.code, s.fields(raw)[tc.field], "%v", tc.patch)
	}

	for _, patch := range []map[string]any{
		{"due_date": "2026-10-20"}, // срок больше не вводится
		{"duration_days": "3"},     // строка вместо числа
		{"duration_days": 1.5},     // не целое
		{"start_date": 20261015},   // число вместо строки
	} {
		st, _, raw = s.createTask(s.admin, body(patch))
		s.Equal(http.StatusBadRequest, st, "%v → %s", patch, raw)
	}
}

// order-start-duration: POST сохраняет план и возвращает вычисленное плановое окончание
// (start_date + duration_days − 1); границы 1 и 60 дней; PUT меняет оба поля — due_date в ответе,
// GET и списке доски пересчитан; PUT без плана → 400, с due_date → 400; фильтр due — по плану.
func (s *BoardLiveSuite) TestBoardLiveSuite_TaskPlan_CreateUpdate() {
	c := s.newClient("Клиент-План-Заказа")
	col := s.columnID("Приёмка")

	st, t, raw := s.createTask(s.admin, withNewCar(map[string]any{
		"title": "План 3 дня", "client_id": c, "column_id": col, "start_date": "2031-03-30", "duration_days": 3,
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("2031-03-30", t.StartDate)
	s.Equal(3, t.DurationDays)
	s.Require().NotNil(t.DueDate)
	s.Equal("2031-04-01", *t.DueDate, "через границу месяца")

	for _, tc := range []struct {
		days int
		due  string
	}{{1, "2031-03-30"}, {60, "2031-05-28"}} {
		st, b, raw := s.createTask(s.admin, withNewCar(map[string]any{
			"title": "План граница", "client_id": c, "column_id": col, "start_date": "2031-03-30", "duration_days": tc.days,
		}))
		s.Require().Equal(http.StatusCreated, st, string(raw))
		s.Equal(tc.due, *b.DueDate, "days=%d", tc.days)
	}

	// PUT: новое начало и дни → плановое окончание пересчитано в ответе, GET и списке.
	st, m, raw := s.mutate(s.admin, http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{
		"start_date": "2031-04-10", "duration_days": 5,
	}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("2031-04-10", m.Task.StartDate)
	s.Equal(5, m.Task.DurationDays)
	s.Equal("2031-04-14", *m.Task.DueDate)
	st, got, raw := s.getTask(s.admin, t.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("2031-04-10|5|2031-04-14", got.StartDate+"|"+itoa(int64(got.DurationDays))+"|"+*got.DueDate)
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?assignee=all&client_id="+c, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list taskListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	found := false
	for _, it := range list.Items {
		if it.ID == t.ID {
			found = true
			s.Equal("2031-04-10|5|2031-04-14", it.StartDate+"|"+itoa(int64(it.DurationDays))+"|"+*it.DueDate)
		}
	}
	s.True(found, "заказ в списке доски")

	// PUT — полная замена шапки: без плана → 400 по полям; due_date во входе → 400.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(m.Task, map[string]any{"start_date": nil, "duration_days": nil}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"start_date": "required", "duration_days": "required"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(m.Task, map[string]any{"due_date": "2031-05-01"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(m.Task, map[string]any{"duration_days": 61}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("out_of_range", s.fields(raw)["duration_days"])
	st, got, _ = s.getTask(s.admin, t.ID)
	s.Require().Equal(http.StatusOK, st)
	s.Equal(m.Task.Version, got.Version, "отклонённые PUT заказ не меняли")

	// фильтр «Горит/просрочено» — по плановому окончанию: план прошёл → overdue; кончается
	// сегодня → soon; длинный план в будущем — ни там, ни там.
	fc := s.newClient("Клиент-План-Фильтр")
	today, err := time.Parse(time.DateOnly, testToday())
	s.Require().NoError(err)
	mk := func(title string, startOffset, days int) int64 {
		st, x, raw := s.createTask(s.admin, withNewCar(map[string]any{"title": title, "client_id": fc, "column_id": col,
			"start_date": today.AddDate(0, 0, startOffset).Format(time.DateOnly), "duration_days": days}))
		s.Require().Equal(http.StatusCreated, st, string(raw))
		return x.ID
	}
	overdue, soon, later := mk("План прошёл", -5, 2), mk("План сегодня", -2, 3), mk("План потом", 0, 10)
	ids := func(due string) map[int64]bool {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?assignee=all&client_id="+fc+"&due="+due, nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var l taskListDTO
		s.Require().NoError(json.Unmarshal(raw, &l))
		out := map[int64]bool{}
		for _, it := range l.Items {
			out[it.ID] = true
		}
		return out
	}
	s.Equal(map[int64]bool{overdue: true}, ids("overdue"))
	s.Equal(map[int64]bool{overdue: true, soon: true}, ids("soon"))
	s.NotContains(ids("soon"), later)
}

// LV-034/AC-006.
func (s *BoardLiveSuite) TestBoardLiveSuite_ColumnValidation400() {
	st, _, raw := s.createColumn(s.admin, map[string]any{"name": "", "color": "#112233"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["name"])

	st, _, raw = s.createColumn(s.admin, map[string]any{"name": "x", "color": "red"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_format", s.fields(raw)["color"])

	st, _, raw = s.createColumn(s.admin, map[string]any{"name": "x", "color": "#112233", "position": 0})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["position"])
}

// LV-035.
func (s *BoardLiveSuite) TestBoardLiveSuite_NotFound404() {
	for _, id := range []string{"abc", "-1", "99999999999999999999"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
	for _, id := range []string{"not-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/columns/"+id, map[string]any{"name": "x", "color": "#112233"})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/columns/"+id, map[string]any{"reason": "Создана по ошибке"})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// LV-037/AC-013.
func (s *BoardLiveSuite) TestBoardLiveSuite_QueryEnumValidation400() {
	for _, q := range []string{"paid=maybe", "due=x", "assignee=bad-uuid-ish", "limit=-1"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?"+q, nil)
		s.Equal(http.StatusBadRequest, st, "%s → %s", q, raw)
	}
}

// Mobile M-01 (code review): GET /api/v1/columns и /api/v1/assignees — мусорный или
// выходящий за диапазон limit/offset теперь 400 field, как у /tasks и /stock (унификация;
// раньше тихо подставлялся дефолт).
func (s *BoardLiveSuite) TestBoardLiveSuite_ColumnsAssignees_InvalidLimitOffset400() {
	for _, q := range []string{"limit=abc", "limit=-5", "limit=0", "offset=-1", "offset=abc"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/columns?"+q, nil)
		s.Equal(http.StatusBadRequest, st, "columns %s → %s", q, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code, "columns %s", q)

		st, raw = s.admin.Do(http.MethodGet, "/api/v1/assignees?"+q, nil)
		s.Equal(http.StatusBadRequest, st, "assignees %s → %s", q, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code, "assignees %s", q)
	}
}

// ---- Access Control ----

func (s *BoardLiveSuite) newExecutor(login string) *testutil.Browser {
	s.SeedUser(login, "password1", "Доска Исп", "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b
}

// LV-040/AC-006.
func (s *BoardLiveSuite) TestBoardLiveSuite_Executor_ColumnsForbidden() {
	exec := s.newExecutor("board-exec-columns")
	for _, cs := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/columns"},
		{http.MethodPut, "/api/v1/columns/" + s.columnID("Приёмка")},
		{http.MethodDelete, "/api/v1/columns/" + s.columnID("Приёмка")},
	} {
		st, raw := exec.Do(cs.method, cs.path, map[string]any{"name": "x", "color": "#112233"})
		s.Equal(http.StatusForbidden, st, "%s %s → %s", cs.method, cs.path, raw)
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}
	st, _ := exec.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Equal(http.StatusOK, st, "чтение колонок разрешено executor")
}

// LV-041/AC-012.
func (s *BoardLiveSuite) TestBoardLiveSuite_Executor_DeleteTaskForbidden() {
	exec := s.newExecutor("board-exec-delete")
	c := s.newClient("Клиент-Исполнитель-Удаление")
	t := s.newTask(c, s.columnID("Приёмка"))

	st, raw := exec.Do(http.MethodDelete, "/api/v1/tasks/"+itoa(t.ID), map[string]any{"reason": "Создан по ошибке"})
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("FORBIDDEN", s.errOf(raw).Code)

	st, _, raw = s.getTask(s.admin, t.ID)
	s.Equal(http.StatusOK, st, "заказ жив: %s", raw)
}

// soft-delete-core TS-013 (DS-027, abuse case 4): сьюта запущена БЕЗ TEST_PURGE_ENABLED —
// служебной уборки нет: адрес не зарегистрирован, 404 и для существующего заказа; заказ цел.
func (s *BoardLiveSuite) TestBoardLiveSuite_PurgeDisabledByDefault() {
	c := s.newClient("Клиент-Уборка-Выключена")
	t := s.newTask(c, s.columnID("Приёмка"))

	for _, b := range []*testutil.Browser{s.admin, s.newExecutor("board-exec-purge")} {
		for _, path := range []string{"/api/v1/test/purge/tasks/" + itoa(t.ID)} {
			st, raw := b.Do(http.MethodDelete, path, nil)
			s.Equal(http.StatusNotFound, st, "%s: %s", path, raw)
		}
	}
	st, _, raw := s.getTask(s.admin, t.ID)
	s.Equal(http.StatusOK, st, "заказ жив: %s", raw)
}

// LV-042/AC-011.
func (s *BoardLiveSuite) TestBoardLiveSuite_Executor_PaidAdminOnly() {
	exec := s.newExecutor("board-exec-paid")
	c := s.newClient("Клиент-Исполнитель-Оплата")
	col := s.columnID("Приёмка")

	st, _, raw := s.createTask(exec, withNewCar(map[string]any{"title": "x", "client_id": c, "column_id": col, "is_paid": true}))
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("PAID_ADMIN_ONLY", s.errOf(raw).Code)

	t := s.newTask(c, col)
	stCode, rawBody := exec.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"is_paid": true}))
	s.Equal(http.StatusForbidden, stCode, string(rawBody))
	s.Equal("PAID_ADMIN_ONLY", s.errOf(rawBody).Code)

	// = текущему значению (false) — разрешено.
	stCode, rawBody = exec.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"is_paid": false}))
	s.Equal(http.StatusOK, stCode, string(rawBody))
}

// LV-044/AC-009/AC-002 (D19).
func (s *BoardLiveSuite) TestBoardLiveSuite_Executor_TaskWriteAllowed() {
	exec := s.newExecutor("board-exec-write")
	c := s.newClient("Клиент-Исполнитель-Пишет")
	col1, col2 := s.columnID("Приёмка"), s.columnID("Согласование")

	st, m, raw := s.mutate(exec, http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{"title": "x", "client_id": c, "column_id": col1}))
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, _, raw = s.mutate(exec, http.MethodPut, "/api/v1/tasks/"+itoa(m.Task.ID), taskCard(m.Task, map[string]any{"title": "y"}))
	s.Equal(http.StatusOK, st, string(raw))

	st, _, raw = s.mutate(exec, http.MethodPost, "/api/v1/tasks/"+itoa(m.Task.ID)+"/move", map[string]any{"column_id": col2})
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- Business Rules ----

// LV-050/AC-003.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_ToFinal_SubtasksNotDone() {
	c := s.newClient("Клиент-Финал-Незавершено")
	col, done := s.columnID("Приёмка"), s.columnID("Выдан")
	t := s.newTask(c, col)
	_, svcID := s.newServiceWithCategory()
	s.addSubtask(t.ID, svcID, "in_progress")

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": done})
	s.Equal(http.StatusConflict, st, string(raw))
	e := s.errOf(raw)
	s.Equal("SUBTASKS_NOT_DONE", e.Code)
	s.Require().NotNil(e.Details)
	s.Require().Len(e.Details.Subtasks, 1)
	s.Equal("in_progress", e.Details.Subtasks[0].Status)

	st, got, raw := s.getTask(s.admin, t.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(col, got.ColumnID, "колонка не изменилась")
}

// LV-051/AC-003.
func (s *BoardLiveSuite) TestBoardLiveSuite_Update_ToFinal_SubtasksNotDone() {
	c := s.newClient("Клиент-Финал-PUT-Незавершено")
	col, done := s.columnID("Приёмка"), s.columnID("Выдан")
	t := s.newTask(c, col)
	_, svcID := s.newServiceWithCategory()
	s.addSubtask(t.ID, svcID, "new")

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"column_id": done}))
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("SUBTASKS_NOT_DONE", s.errOf(raw).Code)
}

// LV-052/AC-003.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_ToFinal_AllDone() {
	c := s.newClient("Клиент-Финал-Готово")
	col, done := s.columnID("Приёмка"), s.columnID("Выдан")
	t := s.newTask(c, col)
	_, svcID := s.newServiceWithCategory()
	s.addSubtask(t.ID, svcID, "done")

	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": done})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(done, m.Task.ColumnID)
}

// LV-053/AC-006.
func (s *BoardLiveSuite) TestBoardLiveSuite_DeleteColumn_NotEmpty() {
	c := s.newClient("Клиент-Колонка-Не-Пуста")
	col := s.columnID("Приёмка")
	s.newTask(c, col)

	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/columns/"+col, map[string]any{"reason": "Создана по ошибке"})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("COLUMN_NOT_EMPTY", s.errOf(raw).Code)
}

// LV-054/AC-006.
func (s *BoardLiveSuite) TestBoardLiveSuite_DeleteColumn_LastNonFinal() {
	// делаем пустую тестовую колонку единственной нефинальной, обнулив остальные пустые.
	st, col, raw := s.createColumn(s.admin, map[string]any{
		"name": "LastNonFinal-Единственная", "color": "#334455", "is_final": false, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	// сделать все остальные нефинальные колонки финальными (пустые — допустимо технически;
	// сценарий изолирован от общего стенда именованием, чтобы не задеть остальные тесты).
	stR, raw := s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, stR)
	var list columnListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	var switched []columnDTO
	for _, c := range list.Items {
		if c.ID == col.ID || c.IsFinal {
			continue
		}
		// is_final — флаг колонки, не требует пустоты (COLUMN_NOT_EMPTY проверяется
		// только на DELETE); переключаем все остальные нефинальные, включая занятые.
		// Закрыть чужие незавершённые услуги в колонке (остатки других тестов сьюты, общий
		// стенд без TRUNCATE между тестами) — иначе PUT is_final=true отобьётся SUBTASKS_
		// NOT_DONE (MEDIUM-3, code review), а не тем инвариантом, который проверяет этот тест.
		_, _ = s.Pool.Exec(context.Background(),
			`UPDATE subtasks SET status = 'done' WHERE task_id IN (SELECT id FROM tasks WHERE column_id = $1)`, c.ID)
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/columns/"+c.ID, map[string]any{
			"name": c.Name, "color": c.Color, "is_final": true, "consumes_stock": c.ConsumesStock,
		})
		s.Require().Equal(http.StatusOK, st, string(raw))
		switched = append(switched, c)
	}

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/columns/"+col.ID, map[string]any{"reason": "Создана по ошибке"})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("LAST_NON_FINAL_COLUMN", s.errOf(raw).Code)

	// восстановить стенд для остальных тестов сьюты.
	for _, c := range switched {
		_, _ = s.admin.Do(http.MethodPut, "/api/v1/columns/"+c.ID, map[string]any{
			"name": c.Name, "color": c.Color, "is_final": false, "consumes_stock": c.ConsumesStock,
		})
	}
	_, _ = s.admin.Do(http.MethodDelete, "/api/v1/columns/"+col.ID, map[string]any{"reason": "Создана по ошибке"})
}

// AC-006 (code review HIGH-1): PUT is_final=true на последней нефинальной колонке
// запрещён так же, как её DELETE (LV-054 выше) — иначе на доске не остаётся места для
// незавершённых заказов.
func (s *BoardLiveSuite) TestBoardLiveSuite_Update_LastNonFinal_Rejected() {
	st, col, raw := s.createColumn(s.admin, map[string]any{
		"name": "LastNonFinal-PUT-Единственная", "color": "#334455", "is_final": false, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	stR, raw := s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, stR)
	var list columnListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))
	var switched []columnDTO
	for _, c := range list.Items {
		if c.ID == col.ID || c.IsFinal {
			continue
		}
		// закрыть чужие незавершённые услуги в колонке (остатки других тестов сьюты, общий
		// стенд без TRUNCATE между тестами) — иначе PUT is_final=true отобьётся SUBTASKS_
		// NOT_DONE (MEDIUM-3), а не тем инвариантом, который проверяет этот тест.
		_, _ = s.Pool.Exec(context.Background(),
			`UPDATE subtasks SET status = 'done' WHERE task_id IN (SELECT id FROM tasks WHERE column_id = $1)`, c.ID)
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/columns/"+c.ID, map[string]any{
			"name": c.Name, "color": c.Color, "is_final": true, "consumes_stock": c.ConsumesStock,
		})
		s.Require().Equal(http.StatusOK, st, string(raw))
		switched = append(switched, c)
	}

	// последняя нефинальная → PUT is_final=true → 409, не 200.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/columns/"+col.ID, map[string]any{
		"name": col.Name, "color": col.Color, "is_final": true, "consumes_stock": col.ConsumesStock,
	})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("LAST_NON_FINAL_COLUMN", s.errOf(raw).Code)

	// колонка осталась нефинальной (PUT отбит транзакцией целиком).
	stG, rawG := s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, stG)
	var list2 columnListDTO
	s.Require().NoError(json.Unmarshal(rawG, &list2))
	for _, c := range list2.Items {
		if c.ID == col.ID {
			s.False(c.IsFinal, "PUT отбит — is_final не должен был смениться")
		}
	}

	// восстановить стенд для остальных тестов сьюты.
	for _, c := range switched {
		_, _ = s.admin.Do(http.MethodPut, "/api/v1/columns/"+c.ID, map[string]any{
			"name": c.Name, "color": c.Color, "is_final": false, "consumes_stock": c.ConsumesStock,
		})
	}
	_, _ = s.admin.Do(http.MethodDelete, "/api/v1/columns/"+col.ID, map[string]any{"reason": "Создана по ошибке"})
}

// AC-003 (code review MEDIUM-3, эскалация design): PUT is_final=true на колонке, где есть
// заказ с незавершённой услугой, запрещён тем же SUBTASKS_NOT_DONE, что и Move/PUT заказа
// в финальную колонку (LV-050/LV-051) — иначе заказ с открытыми услугами молча оказывается
// в финальной колонке.
func (s *BoardLiveSuite) TestBoardLiveSuite_Update_ToFinal_ColumnHasOpenSubtasks() {
	st, col, raw := s.createColumn(s.admin, map[string]any{
		"name": "ToFinal-Открытые-Услуги", "color": "#445566", "is_final": false, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	c := s.newClient("Клиент-Колонка-Финал-Незавершено")
	t := s.newTask(c, col.ID)
	_, svcID := s.newServiceWithCategory()
	s.addSubtask(t.ID, svcID, "new")

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/columns/"+col.ID, map[string]any{
		"name": col.Name, "color": col.Color, "is_final": true, "consumes_stock": col.ConsumesStock,
	})
	s.Equal(http.StatusConflict, st, string(raw))
	e := s.errOf(raw)
	s.Equal("SUBTASKS_NOT_DONE", e.Code)
	s.Require().NotNil(e.Details)
	s.Require().Len(e.Details.Subtasks, 1)

	stG, rawG := s.admin.Do(http.MethodGet, "/api/v1/columns", nil)
	s.Require().Equal(http.StatusOK, stG)
	var list columnListDTO
	s.Require().NoError(json.Unmarshal(rawG, &list))
	for _, cc := range list.Items {
		if cc.ID == col.ID {
			s.False(cc.IsFinal, "PUT отбит — is_final не должен был смениться")
		}
	}
}

// LV-055/AC-010.
func (s *BoardLiveSuite) TestBoardLiveSuite_Update_StaleVersion() {
	c := s.newClient("Клиент-Устаревшая-Версия")
	col := s.columnID("Приёмка")
	t := s.newTask(c, col)

	st, m, raw := s.mutate(s.admin, http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"title": "Первая правка"}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(int64(2), m.Task.Version)

	// повторный PUT со старым version=1.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"title": "Устаревшая правка"}))
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TASK_VERSION_CONFLICT", s.errOf(raw).Code)

	st, got, raw := s.getTask(s.admin, t.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("Первая правка", got.Title)
}

// AC-010/AC-011 (code review MEDIUM-2): version-конфликт проверяется РАНЬШЕ paid-гарда —
// executor со устаревшей формой должен получить 409 TASK_VERSION_CONFLICT (клиент по коду
// перезагружает карточку), а не ложный 403 PAID_ADMIN_ONLY из-за того, что is_paid успели
// сменить с момента открытия формы.
func (s *BoardLiveSuite) TestBoardLiveSuite_Update_StaleVersion_PaidChanged_VersionConflictWins() {
	exec := s.newExecutor("board-exec-stale-paid")
	c := s.newClient("Клиент-Устаревшая-Версия-Оплата")
	col := s.columnID("Приёмка")
	t := s.newTask(c, col) // t.Version == 1, is_paid == false

	// admin помечает оплату — version растёт до 2, is_paid → true.
	st, m, raw := s.mutate(s.admin, http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"is_paid": true}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(int64(2), m.Task.Version)
	s.True(m.Task.IsPaid)

	// executor отправляет форму со старым version=1 (захвачен ДО правки admin'а) и
	// is_paid=false, как было в его форме — но реальный is_paid уже true. Ожидание: 409
	// (форма устарела), а не 403 (executor формально не пытается сменить оплату своей
	// стороной — просто не знает о новом состоянии).
	stCode, rawBody := exec.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"is_paid": false}))
	s.Equal(http.StatusConflict, stCode, string(rawBody))
	s.Equal("TASK_VERSION_CONFLICT", s.errOf(rawBody).Code)
}

// ---- Race Condition ----

// LV-060: две колонки, N горутин переносят одну карточку в разные колонки — все 200
// (LWW), итог = одна из целей, инвариант склада.
func (s *BoardLiveSuite) TestBoardLiveSuite_Race_ConcurrentMoveDifferentColumns() {
	c := s.newClient("Клиент-Race-Move-Разные-Колонки")
	colA, colB := s.columnID("Приёмка"), s.columnID("Согласование")
	t := s.newTask(c, colA)

	const n = 20
	var wg sync.WaitGroup
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		target := colA
		if i%2 == 0 {
			target = colB
		}
		go func(idx int, colID string) {
			defer wg.Done()
			// s.admin переиспользуется всеми горутинами (общая сессия/CSRF-cookie,
			// http.Client и cookiejar безопасны для конкурентного использования) — логин
			// на каждую горутину упирается в rate-limit неудачных попыток входа.
			st, _ := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": colID})
			statuses[idx] = st
		}(i, target)
	}
	wg.Wait()

	for _, st := range statuses {
		s.Equal(http.StatusOK, st, "%v", statuses)
	}
	st, got, raw := s.getTask(s.admin, t.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains([]string{colA, colB}, got.ColumnID)
}

// LV-061: PUT со старой version (форма) vs move — move 200, PUT 409. Move Task Query
// (design-sql.md) НЕ проверяет version (LWW «порядок ≠ правка заказа») — Move ВСЕГДА
// успевает и увеличивает version, поэтому детерминированный тест сценария «форма открыта
// со старым version, за это время кто-то перенёс карточку» — move синхронно первым (200,
// version+1), затем форма отправляет СТАРЫЙ захваченный version (409), а не два реально
// параллельных запроса с одинаковым version (тот случай зависит от того, чья транзакция
// первой возьмёт FOR UPDATE — сам Move всё равно не проверяет version, поэтому оба могли
// бы получить 200; это не баг, а описанное поведение LWW для Move).
func (s *BoardLiveSuite) TestBoardLiveSuite_Race_FormVsMove() {
	c := s.newClient("Клиент-Race-Form-Vs-Move")
	col1, col2 := s.columnID("Приёмка"), s.columnID("Согласование")
	t := s.newTask(c, col1)

	st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": col2})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(int64(2), m.Task.Version)

	// форма отправляет version, захваченный ДО move (t.Version == 1) — устарел.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/tasks/"+itoa(t.ID), taskCard(t, map[string]any{"title": "Гонка"}))
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TASK_VERSION_CONFLICT", s.errOf(raw).Code)
}

// LV-062: toggle consumes_stock колонки ‖ move заказа в/из неё — нет deadlock-500, инвариант
// склада, task_stock согласован с итоговой колонкой.
func (s *BoardLiveSuite) TestBoardLiveSuite_Race_ToggleVsMove() {
	c := s.newClient("Клиент-Race-Toggle-Vs-Move")
	newCol, wipCol := s.columnID("Приёмка"), s.columnID("В ремонте")
	productID := s.createProduct("50", "1")
	t := s.newTask(c, newCol)
	_, svcID := s.newServiceWithCategory()
	subtaskID := s.addSubtask(t.ID, svcID, "new")
	s.addSubtaskProduct(subtaskID, productID, "2")

	var wg sync.WaitGroup
	var moveStatus, toggleStatus int
	wg.Add(2)
	go func() {
		defer wg.Done()
		st, _ := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": wipCol})
		moveStatus = st
	}()
	go func() {
		defer wg.Done()
		st, _ := s.admin.Do(http.MethodPut, "/api/v1/columns/"+wipCol, map[string]any{
			"name": "В ремонте", "color": "#2563EB", "is_final": false, "consumes_stock": true,
		})
		toggleStatus = st
	}()
	wg.Wait()

	s.Less(moveStatus, 500)
	s.Less(toggleStatus, 500)
	assertStockInvariant(s.T(), s.Pool)

	// восстановить consumes_stock=true (idempotent PUT выше уже задал true).
}

// LV-063: параллельные move двух заказов с общим товаром — stock согласован, без потерь.
func (s *BoardLiveSuite) TestBoardLiveSuite_Race_ConcurrentMoveSharedProduct() {
	c := s.newClient("Клиент-Race-Общий-Товар")
	newCol, wipCol := s.columnID("Приёмка"), s.columnID("В ремонте")
	productID := s.createProduct("100", "1")
	_, svcID := s.newServiceWithCategory()

	t1 := s.newTask(c, newCol)
	sub1 := s.addSubtask(t1.ID, svcID, "new")
	s.addSubtaskProduct(sub1, productID, "5")
	t2 := s.newTask(c, newCol)
	sub2 := s.addSubtask(t2.ID, svcID, "new")
	s.addSubtaskProduct(sub2, productID, "7")

	var wg sync.WaitGroup
	statuses := make([]int, 2)
	ids := []int64{t1.ID, t2.ID}
	for i, id := range ids {
		wg.Add(1)
		go func(idx int, taskID int64) {
			defer wg.Done()
			st, _ := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(taskID)+"/move", map[string]any{"column_id": wipCol})
			statuses[idx] = st
		}(i, id)
	}
	wg.Wait()

	for _, st := range statuses {
		s.Equal(http.StatusOK, st, "%v", statuses)
	}
	s.Equal("88.000", s.productStock(productID), "100 - 5 - 7, без потерянных обновлений")
	assertStockInvariant(s.T(), s.Pool)
}

// LV-064: 70 вставок между одними соседями → ребалансировка, порядок сохранён, позиции
// уникальны. Порог поднят с 50 до 70 (code review MEDIUM-1): guard needs_rebalance теперь
// срабатывает при scale(pos) >= 16 (PG ограничивает scale деления NUMERIC этим потолком —
// прежний scale(pos) > 20 был недостижим), и с binary-halving от 1024 нужно ~54 вставки,
// чтобы дойти до scale=16; 70 — с запасом.
func (s *BoardLiveSuite) TestBoardLiveSuite_Race_PositionRebalance50Inserts() {
	c := s.newClient("Клиент-Race-Ребалансировка")
	col := s.columnID("Приёмка")
	t1 := s.newTask(c, col)
	t2 := s.newTask(c, col)

	// каждая новая карточка вставляется СРАЗУ после t1, перед предыдущей вставленной —
	// расстояние (t1, next) прогрессивно делится пополам, пока scale(pos) не достигнет 16
	// и не сработает ребалансировка колонки (design-sql.md §Position Query).
	next := t2.ID
	for i := 0; i < 70; i++ {
		t := s.newTask(c, col)
		st, m, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move",
			map[string]any{"column_id": col, "prev_id": t1.ID, "next_id": next})
		s.Require().Equal(http.StatusOK, st, string(raw))
		next = m.Task.ID
	}

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c+"&limit=1000", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list taskListDTO
	s.Require().NoError(json.Unmarshal(raw, &list))

	seen := map[string]bool{}
	var prevPos float64
	first := true
	for _, it := range list.Items {
		s.False(seen[it.Position], "позиции уникальны: дубль %s", it.Position)
		seen[it.Position] = true
		p := parseAmount(it.Position)
		if !first {
			s.GreaterOrEqual(p, prevPos)
		}
		prevPos, first = p, false
	}
}

// LV-065 (soft-delete-clients DS-014, DS-024): DELETE клиента ‖ POST заказа на этого клиента.
// Удаление клиента мягкое и заказами не удерживается — оно проходит всегда (200); заказ либо
// успел раньше (201 — остаётся у удалённого клиента с пометкой), либо получает 400 field
// client_id: not_found. Ни 409 CLIENT_HAS_TASKS, ни 500. Детерминированные исходы гонки —
// clients.TestRace_TaskCreateWaitsForClientDelete.
func (s *BoardLiveSuite) TestBoardLiveSuite_Race_DeleteClientVsCreateTask() {
	c := s.newClient("Клиент-Race-Удаление-Vs-Создание")
	col := s.columnID("Приёмка")

	var wg sync.WaitGroup
	var deleteStatus, createStatus int
	var createRaw []byte
	wg.Add(2)
	go func() {
		defer wg.Done()
		st, _ := s.admin.Do(http.MethodDelete, "/api/v1/clients/"+c, map[string]any{"reason": "Гонка с созданием заказа"})
		deleteStatus = st
	}()
	go func() {
		defer wg.Done()
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{"title": "Race заказ", "client_id": c, "column_id": col}))
		createStatus, createRaw = st, raw
	}()
	wg.Wait()

	s.Equal(http.StatusOK, deleteStatus, "заказы клиента мягкому удалению не мешают")
	var deleted bool
	var tasks int
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT (SELECT deleted_at IS NOT NULL FROM clients WHERE id = $1::uuid),
		       (SELECT count(*) FROM tasks WHERE client_id = $1::uuid)`, c).Scan(&deleted, &tasks))
	s.True(deleted, "клиент помечен удалённым")
	if createStatus == http.StatusCreated {
		s.Equal(1, tasks, "заказ успел раньше — остаётся у удалённого клиента")
	} else {
		s.Equal(http.StatusBadRequest, createStatus, string(createRaw))
		s.Contains(string(createRaw), `"client_id"`)
		s.Zero(tasks, "заказ не создан")
	}
}

// live-сверка enum'ов с CHECK constraint'ом БД (design-go-layers.md §Enum SubtaskStatus).
func (s *BoardLiveSuite) TestBoardLiveSuite_Enums_SubtaskStatusMatchesCheckConstraint() {
	var def string
	err := s.Pool.QueryRow(context.Background(), `
		SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'subtasks'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%status%'`).Scan(&def)
	s.Require().NoError(err)
	for _, v := range []string{"new", "in_progress", "done"} {
		s.Contains(def, v, "CHECK constraint должен перечислять %q: %s", v, def)
	}
}

// TestBoardLiveSuite_Move_ToFinal_WithAPISubtask — LV-150 (регресс фазы 5, AC-011): услуга
// создана через API этой фазы (POST /tasks/:id/subtasks), не прямой SQL-вставкой (эталон
// TestBoardLiveSuite_Move_ToFinal_AllDone) — перенос в финальную колонку по-прежнему
// требует done-статуса всех услуг.
func (s *BoardLiveSuite) TestBoardLiveSuite_Move_ToFinal_WithAPISubtask() {
	c := s.newClient("Клиент-Финал-API-Услуга")
	col, done := s.columnID("Приёмка"), s.columnID("Выдан")
	t := s.newTask(c, col)
	_, svcID := s.newServiceWithCategory()

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/subtasks", map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var created struct {
		Card struct {
			Subtasks []struct {
				ID string `json:"id"`
			} `json:"subtasks"`
		} `json:"card"`
	}
	s.Require().NoError(json.Unmarshal(raw, &created), string(raw))
	s.Require().Len(created.Card.Subtasks, 1)
	subID := created.Card.Subtasks[0].ID

	// пока услуга не done — перенос в финальную колонку блокируется (AC-011, as-is).
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": done})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("SUBTASKS_NOT_DONE", s.errOf(raw).Code)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "done", "assignee_id": nil, "due_date": nil, "price": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, got, raw := s.mutate(s.admin, http.MethodPost, "/api/v1/tasks/"+itoa(t.ID)+"/move", map[string]any{"column_id": done})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(done, got.Task.ColumnID)
}

// ---- утилиты ----

func itoa(id int64) string { return fmt.Sprintf("%d", id) }

func idsOf(items []taskCardDTO) []int64 {
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func stringsRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

// parseAmount — примитивный парсер NUMERIC-строки позиции в float64 (только для сравнения
// порядка в тестах, не для бизнес-логики).
func parseAmount(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}
