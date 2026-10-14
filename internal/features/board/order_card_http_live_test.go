//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// OrderCardLiveSuite (§TD-8): карточка заказа (GET) + оборудование заказа. Реальный
// бинарь + TCP + реальный логин; auth-стабы и app.Test запрещены. Услуги/товары/журнал —
// фикстуры напрямую SQL (репозитории записи появятся в фазе 03); оборудование — через API
// этой фазы.
type OrderCardLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestOrderCardLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка заказов (soft-delete-core TS-013); сьюта без
	// переменной (BoardLiveSuite) проверяет, что адреса нет.
	suite.Run(t, &OrderCardLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *OrderCardLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("card-admin", "password1", "Карточка Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("card-admin", "password1")
}

// ---- DTO wire-типы (1:1 с internal/features/board/card_dto.go, §TD-6.2) ----

type cardColumnDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
}

type equipmentHistoryDTO struct {
	TaskID    int64  `json:"task_id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type cardEquipmentDTO struct {
	EquipmentID  string                `json:"equipment_id"`
	TypeName     string                `json:"type_name"`
	Brand        string                `json:"brand"`
	Model        string                `json:"model"`
	Serial       string                `json:"serial"`
	Completeness string                `json:"completeness"`
	Condition    string                `json:"condition"`
	History      []equipmentHistoryDTO `json:"history"`
}

type cardSubtaskProductDTO struct {
	ID        string       `json:"id"`
	ProductID string       `json:"product_id"`
	Name      string       `json:"name"`
	Unit      string       `json:"unit"`
	Position  int          `json:"position"`
	Qty       json.Number  `json:"qty"`
	Price     *json.Number `json:"price"`
	LineTotal json.Number  `json:"line_total"`
}

type cardSubtaskLogDTO struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	UserID     string `json:"user_id"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	CanEdit    bool   `json:"can_edit"`
}

type cardSubtaskDTO struct {
	ID               string       `json:"id"`
	ServiceID        string       `json:"service_id"`
	ServiceName      string       `json:"service_name"`
	Status           string       `json:"status"`
	AssigneeID       *string      `json:"assignee_id"`
	AssigneeName     *string      `json:"assignee_name"`
	AssigneeIsActive *bool        `json:"assignee_is_active"`
	DueDate          *string      `json:"due_date"`
	Price            *json.Number `json:"price"`
	EquipmentID      *string      `json:"equipment_id"`
	Position         int          `json:"position"`
	Total            json.Number  `json:"total"`
	// executor-payouts (D62–D65): выплата исполнителю.
	IsExecutorPaid bool                    `json:"is_executor_paid"`
	ExecutorPaidAt *string                 `json:"executor_paid_at"`
	ExecutorPayout *json.Number            `json:"executor_payout"`
	Products       []cardSubtaskProductDTO `json:"products"`
	Logs           []cardSubtaskLogDTO     `json:"logs"`
}

type cardStockDTO struct {
	ProductID    string      `json:"product_id"`
	Stock        json.Number `json:"stock"`
	MinStock     json.Number `json:"min_stock"`
	TaskReserved json.Number `json:"task_reserved"`
}

// cardTaskDTO — только поля, нужные этому сьюту (не весь TaskResponse — §TD-6.2 допускает
// частичное декодирование).
type cardTaskDTO struct {
	ID              int64       `json:"id"`
	Title           string      `json:"title"`
	ClientID        string      `json:"client_id"`
	ColumnID        string      `json:"column_id"`
	Version         int64       `json:"version"`
	IsPaid          bool        `json:"is_paid"`
	DescriptionHTML string      `json:"description_html"`
	TotalAmount     json.Number `json:"total_amount"`
}

type orderCardDTO struct {
	Task      cardTaskDTO        `json:"task"`
	Column    cardColumnDTO      `json:"column"`
	Equipment []cardEquipmentDTO `json:"equipment"`
	Subtasks  []cardSubtaskDTO   `json:"subtasks"`
	Stock     []cardStockDTO     `json:"stock"`
}

type orderCardMutationDTO struct {
	Card     orderCardDTO      `json:"card"`
	Warnings []stockWarningDTO `json:"warnings"`
}

type equipmentCreateDTO struct {
	ID string `json:"id"`
}

func (s *OrderCardLiveSuite) errOf(body []byte) apiErrDTO {
	var e apiErrDTO
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *OrderCardLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

// ---- фикстуры ----

func (s *OrderCardLiveSuite) newClient(company string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO clients (company_name) VALUES ($1) RETURNING id`, company).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) columnID(name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM columns WHERE name = $1`, name).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) newTask(clientID, columnID string) (int64, int64) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title": "Заказ карточки", "client_id": clientID, "column_id": columnID,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Task struct {
			ID      int64 `json:"id"`
			Version int64 `json:"version"`
		} `json:"task"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	return m.Task.ID, m.Task.Version
}

func (s *OrderCardLiveSuite) newServiceCategory() string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'CardLive Svc ' || gen_random_uuid()) RETURNING id`).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) newService(price string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO services (category_id, name, price) VALUES ($1, 'Услуга', $2::numeric) RETURNING id`,
		s.newServiceCategory(), price).Scan(&id))
	return id
}

// newServiceNoPrice — услуга «по факту» (price=NULL, AC-011/design-sql.md §Totals
// Expression: работа не входит в subtask_total, только товары).
func (s *OrderCardLiveSuite) newServiceNoPrice() string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO services (category_id, name, price) VALUES ($1, 'Услуга по факту', NULL) RETURNING id`,
		s.newServiceCategory()).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) addSubtask(taskID int64, serviceID, price string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO subtasks (task_id, service_id, status, position, price) VALUES ($1, $2, 'new', 1, $3::numeric) RETURNING id`,
		taskID, serviceID, price).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) attachSubtaskEquipment(subtaskID, equipmentID string) {
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE subtasks SET equipment_id = $2 WHERE id = $1`, subtaskID, equipmentID)
	s.Require().NoError(err)
}

func (s *OrderCardLiveSuite) addLog(subtaskID, text string) {
	var userID string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE login = 'card-admin'`).Scan(&userID))
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO subtask_logs (subtask_id, user_id, text) VALUES ($1, $2, $3)`, subtaskID, userID, text)
	s.Require().NoError(err)
}

func (s *OrderCardLiveSuite) newEquipmentType() string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO equipment_types (name) VALUES ('CardLive Type ' || gen_random_uuid()) RETURNING id`).Scan(&id))
	return id
}

// newEquipment — единица напрямую SQL (клиентская фича — не в API этой фазы).
func (s *OrderCardLiveSuite) newEquipment(clientID string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO equipment (client_id, type_id, brand, model, serial) VALUES ($1, $2, 'Brand', 'Model', 'SN') RETURNING id`,
		clientID, s.newEquipmentType()).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) getCard(b *testutil.Browser, id int64) (int, orderCardDTO, []byte) {
	st, raw := b.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", id), nil)
	var c orderCardDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &c), string(raw))
	}
	return st, c, raw
}

func (s *OrderCardLiveSuite) doEquipment(b *testutil.Browser, method, path string, body map[string]any) (int, orderCardMutationDTO, []byte) {
	st, raw := b.Do(method, path, body)
	var m orderCardMutationDTO
	if st == http.StatusOK || st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	}
	return st, m, raw
}

// doCard — тот же конверт card-мутации (услуги/товары/журнал этой фазы, не только
// оборудование) — doEquipment реализует ровно тот же паттерн, алиас для читаемости тестов.
func (s *OrderCardLiveSuite) doCard(b *testutil.Browser, method, path string, body map[string]any) (int, orderCardMutationDTO, []byte) {
	return s.doEquipment(b, method, path, body)
}

// ---- фикстуры фазы 03: товары/комплекты услуг ----

func (s *OrderCardLiveSuite) newProductCategory() string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'CardLive Prod ' || gen_random_uuid()) RETURNING id`).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) newProduct(price, stock, minStock string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO products (category_id, name, unit, price, stock, min_stock) VALUES ($1, 'Товар', 'шт', $2::numeric, $3::numeric, $4::numeric) RETURNING id`,
		s.newProductCategory(), price, stock, minStock).Scan(&id))
	return id
}

// addServiceKit — комплект товаров услуги (service_products), копируется CreateSubtask
// (AC-008).
func (s *OrderCardLiveSuite) addServiceKit(serviceID, productID, qty string, position int) {
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO service_products (service_id, product_id, qty, position) VALUES ($1, $2, $3::numeric, $4)`,
		serviceID, productID, qty, position)
	s.Require().NoError(err)
}

func (s *OrderCardLiveSuite) setServiceActive(id string, active bool) {
	_, err := s.Pool.Exec(context.Background(), `UPDATE services SET is_active = $2 WHERE id = $1`, id, active)
	s.Require().NoError(err)
}

func (s *OrderCardLiveSuite) setProductActive(id string, active bool) {
	_, err := s.Pool.Exec(context.Background(), `UPDATE products SET is_active = $2 WHERE id = $1`, id, active)
	s.Require().NoError(err)
}

// ---- Happy Path ----

// LV-101/AC-001: GET /tasks/:id/card — одним запросом полное дерево карточки.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_GetCard_FullTree() {
	c := s.newClient("Клиент-Карточка-Дерево")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	svcID := s.newService("500")
	subtaskID := s.addSubtask(taskID, svcID, "500")
	s.addLog(subtaskID, "Проверили заказ")

	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(taskID, card.Task.ID)
	s.Equal(col, card.Column.ID)
	s.Require().Len(card.Subtasks, 1)
	s.Equal(svcID, card.Subtasks[0].ServiceID)
	s.Equal("500.00", string(card.Subtasks[0].Total))
	s.Require().Len(card.Subtasks[0].Logs, 1)
	s.Equal("Проверили заказ", card.Subtasks[0].Logs[0].Text)
	s.True(card.Subtasks[0].Logs[0].CanEdit)
	s.Equal("500.00", string(card.Task.TotalAmount))
}

// LV-106/AC-001: заказ без услуг/оборудования — массивы пустые ([]), не null.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_GetCard_EmptyArraysNotNull() {
	c := s.newClient("Клиент-Карточка-Пусто")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"equipment":[]`)
	s.Contains(string(raw), `"subtasks":[]`)
	s.Contains(string(raw), `"stock":[]`)
}

// LV-102/AC-002: description экранируется на сервере, PUT с description меняет, PUT с
// description=null — не трогает сохранённое значение.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Description_EscapeAndUpdate() {
	c := s.newClient("Клиент-Описание")
	col := s.columnID("Новые")

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title": "Заказ с описанием", "client_id": c, "column_id": col,
		"description": "<b>bold</b>\n\nВторой абзац",
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Task struct {
			ID      int64 `json:"id"`
			Version int64 `json:"version"`
		} `json:"task"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m))

	st, card, raw := s.getCard(s.admin, m.Task.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("<p>&lt;b&gt;bold&lt;/b&gt;</p><p>Второй абзац</p>", card.Task.DescriptionHTML)

	// PUT description=null — не меняет сохранённый HTML.
	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", m.Task.ID), map[string]any{
		"title": "Заказ с описанием", "client_id": c, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": m.Task.Version, "description": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, card, raw = s.getCard(s.admin, m.Task.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("<p>&lt;b&gt;bold&lt;/b&gt;</p><p>Второй абзац</p>", card.Task.DescriptionHTML, "nil — не менять")

	// PUT с новым description — переписывает.
	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", m.Task.ID), map[string]any{
		"title": "Заказ с описанием", "client_id": c, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": int64(2), "description": "Новый текст",
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, card, raw = s.getCard(s.admin, m.Task.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("<p>Новый текст</p>", card.Task.DescriptionHTML)
}

// LV-111: deep-link на доску с ?task=<id> — SSR-страница не падает (модалка — фронтенд,
// фаза 05).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_DeepLink_SSR() {
	c := s.newClient("Клиент-DeepLink")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	resp := s.admin.Request(http.MethodGet, fmt.Sprintf("/?task=%d", taskID), nil, nil)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
}

// LV-108/AC-004: добавить/поправить/удалить оборудование заказа.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_AddUpdateRemove() {
	c := s.newClient("Клиент-Оборудование-CRUD")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	eqID := s.newEquipment(c)

	st, m, raw := s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": eqID, "completeness": "Полный комплект", "condition": "Хорошее",
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Equipment, 1)
	s.Equal(eqID, m.Card.Equipment[0].EquipmentID)
	s.Equal("Полный комплект", m.Card.Equipment[0].Completeness)

	st, m, raw = s.doEquipment(s.admin, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eqID), map[string]any{
		"completeness": "Без шнура", "condition": "Царапина на корпусе",
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("Без шнура", m.Card.Equipment[0].Completeness)
	s.Equal("Царапина на корпусе", m.Card.Equipment[0].Condition)

	st, m, raw = s.doEquipment(s.admin, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eqID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(m.Card.Equipment)

	// повторный POST той же единицы — идемпотентно, 201 без ошибки (design-errors.md).
	st, _, raw = s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": eqID,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, _, raw = s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": eqID,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
}

// LV-109/AC-005: новое оборудование прямо из карточки — ADR-004, два запроса
// (POST /clients/:id/equipment, затем POST /tasks/:id/equipment).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_NewFromCard() {
	c := s.newClient("Клиент-Оборудование-Новое")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	typeID := s.newEquipmentType()

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients/"+c+"/equipment", map[string]any{
		"type_id": typeID, "brand": "Brand", "model": "Model-New", "serial": "SN-New",
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var eq equipmentCreateDTO
	s.Require().NoError(json.Unmarshal(raw, &eq))

	st, m, raw := s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": eq.ID,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Equipment, 1)
	s.Equal("Model-New", m.Card.Equipment[0].Model)
}

// LV-110/AC-006: история других заказов с той же единицей оборудования.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_History() {
	c := s.newClient("Клиент-Оборудование-История")
	col := s.columnID("Новые")
	taskA, _ := s.newTask(c, col)
	taskB, _ := s.newTask(c, col)
	eqID := s.newEquipment(c)

	st, _, raw := s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskA), map[string]any{"equipment_id": eqID})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, card, raw := s.getCard(s.admin, taskA)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(card.Equipment, 1)
	s.Empty(card.Equipment[0].History, "единственный заказ с единицей — история пуста")

	st, _, raw = s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskB), map[string]any{"equipment_id": eqID})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, card, raw = s.getCard(s.admin, taskA)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(card.Equipment[0].History, 1)
	s.Equal(taskB, card.Equipment[0].History[0].TaskID)

	st, card, raw = s.getCard(s.admin, taskB)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(card.Equipment[0].History, 1)
	s.Equal(taskA, card.Equipment[0].History[0].TaskID)
}

// ---- Business Rules / Conflicts ----

// LV-142/AC-004: единица выбрана в открытой услуге заказа — DELETE → 409
// TASK_EQUIPMENT_IN_USE + details.subtasks.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_InUseConflict() {
	c := s.newClient("Клиент-Оборудование-Занято")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	eqID := s.newEquipment(c)

	st, _, raw := s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{"equipment_id": eqID})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	svcID := s.newService("300")
	subtaskID := s.addSubtask(taskID, svcID, "300")
	s.attachSubtaskEquipment(subtaskID, eqID)

	st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eqID), nil)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	e := s.errOf(raw)
	s.Equal("TASK_EQUIPMENT_IN_USE", e.Code)
	s.Require().NotNil(e.Details)
	s.Require().Len(e.Details.Subtasks, 1)
	s.Equal(subtaskID, e.Details.Subtasks[0].ID)

	// единица осталась в заказе.
	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(card.Equipment, 1)
}

// LV-143/AC-004: смена клиента заказа — оборудование ПРЕЖНЕГО клиента, выбранное в
// открытой услуге, блокирует смену; свободное (не выбранное) — тихо убирается.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_ClientChange() {
	c1 := s.newClient("Клиент-Смена-Раз")
	c2 := s.newClient("Клиент-Смена-Два")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c1, col)
	eqID := s.newEquipment(c1)

	st, _, raw := s.doEquipment(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{"equipment_id": eqID})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	svcID := s.newService("100")
	subtaskID := s.addSubtask(taskID, svcID, "100")
	s.attachSubtaskEquipment(subtaskID, eqID)

	// занято в открытой услуге — смена клиента отбивается 409.
	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
		"title": "Заказ", "client_id": c2, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
	})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TASK_EQUIPMENT_IN_USE", s.errOf(raw).Code)

	// освобождаем услугу от оборудования — теперь смена клиента проходит, чужое
	// оборудование тихо убирается из task_equipment.
	_, err := s.Pool.Exec(context.Background(), `UPDATE subtasks SET equipment_id = NULL WHERE id = $1`, subtaskID)
	s.Require().NoError(err)

	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
		"title": "Заказ", "client_id": c2, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(card.Equipment, "оборудование прежнего клиента убрано из заказа")
}

// LV-144/AC-004: оборудование другого клиента — 400 VALIDATION field equipment_id.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Equipment_ForeignClient400() {
	c1 := s.newClient("Клиент-Чужое-Раз")
	c2 := s.newClient("Клиент-Чужое-Два")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c1, col)
	foreignEqID := s.newEquipment(c2)

	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": foreignEqID,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("foreign_client", s.fields(raw)["equipment_id"])

	unknown := "00000000-0000-0000-0000-000000000000"
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": unknown,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["equipment_id"])

	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": "",
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["equipment_id"])
}

// ---- Invalid Path IDs (TD-4) ----

// LV-170 (часть — card/equipment пути): невалидные/несуществующие id в пути → 404.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_InvalidPathIDs404() {
	for _, id := range []string{"abc", "-1", "0", "99999999999999999999"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks/"+id+"/card", nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("TASK_NOT_FOUND", s.errOf(raw).Code, id)
	}

	c := s.newClient("Клиент-Невалидные-Пути")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks/999999999/card", nil)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TASK_NOT_FOUND", s.errOf(raw).Code)

	for _, eqID := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eqID), map[string]any{
			"completeness": "x", "condition": "y",
		})
		s.Equal(http.StatusNotFound, st, "%s → %s", eqID, raw)
		s.Equal("TASK_EQUIPMENT_NOT_FOUND", s.errOf(raw).Code, eqID)

		st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, eqID), nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", eqID, raw)
		s.Equal("TASK_EQUIPMENT_NOT_FOUND", s.errOf(raw).Code, eqID)
	}

	// equipment/:id несуществующего заказа — 404 TASK_NOT_FOUND (LockTask раньше).
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/tasks/999999999/equipment", map[string]any{
		"equipment_id": "11111111-1111-1111-1111-111111111111",
	})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TASK_NOT_FOUND", s.errOf(raw).Code)

	// LV-170 (дополнение фазы 03) — subtasks/subtask-products/subtask-logs.
	unknownUUID := "00000000-0000-0000-0000-000000000000"
	for _, id := range []string{"not-a-uuid", unknownUUID} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+id, map[string]any{
			"status": "new", "assignee_id": nil, "due_date": nil, "price": nil, "equipment_id": nil,
		})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/subtasks/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+id+"/products", map[string]any{
			"product_id": unknownUUID, "qty": 1,
		})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+id+"/logs", map[string]any{"text": "x"})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtask-products/"+id, map[string]any{"qty": 1})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_PRODUCT_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/subtask-products/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_PRODUCT_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtask-logs/"+id, map[string]any{"text": "x"})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_LOG_NOT_FOUND", s.errOf(raw).Code, id)

		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/subtask-logs/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("SUBTASK_LOG_NOT_FOUND", s.errOf(raw).Code, id)
	}
}

// ---- Subtasks/Products/Logs (фаза 03) ----

// LV-103/AC-007/AC-008: POST /tasks/:id/subtasks в НЕ списывающей колонке — цена из
// каталога, комплект товаров скопирован (qty/цена/порядок), позиция в конец, tasks.version
// не тронут.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_CreateWithKit() {
	c := s.newClient("Клиент-Услуга-Комплект")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c, col)

	svcID := s.newService("500")
	p1 := s.newProduct("100", "10", "1")
	p2 := s.newProduct("50", "5", "1")
	s.addServiceKit(svcID, p1, "2", 1)
	s.addServiceKit(svcID, p2, "1", 2)

	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks, 1)
	sub := m.Card.Subtasks[0]
	s.Equal(svcID, sub.ServiceID)
	s.Require().NotNil(sub.Price)
	s.Equal(json.Number("500.00"), *sub.Price)
	s.Equal(1, sub.Position)
	s.Require().Len(sub.Products, 2)
	s.Equal(p1, sub.Products[0].ProductID)
	s.Equal(json.Number("2.000"), sub.Products[0].Qty)
	s.Require().NotNil(sub.Products[0].Price)
	s.Equal(json.Number("100.00"), *sub.Products[0].Price)
	s.Equal(p2, sub.Products[1].ProductID)
	s.Empty(m.Warnings)
	s.Equal(version, m.Card.Task.Version, "row-мутация не трогает tasks.version")
}

// LV-104/AC-007/AC-011: PUT /subtasks/:id — status/assignee/due/price(null)/equipment_id.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_UpdateFields() {
	c := s.newClient("Клиент-Услуга-Правка")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	eqID := s.newEquipment(c)
	st, _, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{"equipment_id": eqID})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	svcID := s.newService("300")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	var adminID string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT id FROM users WHERE login = 'card-admin'`).Scan(&adminID))

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "in_progress", "assignee_id": adminID, "due_date": "2026-05-01", "price": nil, "equipment_id": eqID,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	sub := m.Card.Subtasks[0]
	s.Equal("in_progress", sub.Status)
	s.Require().NotNil(sub.AssigneeID)
	s.Equal(adminID, *sub.AssigneeID)
	s.Require().NotNil(sub.DueDate)
	s.Equal("2026-05-01", *sub.DueDate)
	s.Nil(sub.Price, "price=null — «по факту»")
	s.Require().NotNil(sub.EquipmentID)
	s.Equal(eqID, *sub.EquipmentID)
	s.Equal(json.Number("0.00"), sub.Total, "«по факту» без товаров — 0")
	s.Equal(json.Number("0.00"), m.Card.Task.TotalAmount)
}

// LV-105/AC-008: POST/PUT/DELETE /subtask-products — line_total корректен на каждом шаге.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_SubtaskProduct_CRUD() {
	c := s.newClient("Клиент-Товар-CRUD")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID
	productID := s.newProduct("20", "100", "1")

	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{
		"product_id": productID, "qty": 0.5,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Products, 1)
	prod := m.Card.Subtasks[0].Products[0]
	s.Equal(json.Number("0.500"), prod.Qty)
	s.Require().NotNil(prod.Price)
	s.Equal(json.Number("20.00"), *prod.Price)
	s.Equal(json.Number("10.00"), prod.LineTotal)
	prodRowID := prod.ID

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+prodRowID, map[string]any{"qty": 2, "price": 15})
	s.Require().Equal(http.StatusOK, st, string(raw))
	prod = m.Card.Subtasks[0].Products[0]
	s.Equal(json.Number("2.000"), prod.Qty)
	s.Require().NotNil(prod.Price)
	s.Equal(json.Number("15.00"), *prod.Price)
	s.Equal(json.Number("30.00"), prod.LineTotal)

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+prodRowID, map[string]any{"qty": 1, "price": nil})
	s.Require().Equal(http.StatusOK, st, string(raw))
	prod = m.Card.Subtasks[0].Products[0]
	s.Nil(prod.Price, "price=null — без цены")
	s.Equal(json.Number("0.00"), prod.LineTotal)

	st, m, raw = s.doCard(s.admin, http.MethodDelete, "/api/v1/subtask-products/"+prodRowID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(m.Card.Subtasks[0].Products)
}

// LV-106/AC-013: итоги — subtask_total аддитивен из работы+товаров, task_total = Σ,
// GET /tasks total_amount — то же значение.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Totals() {
	c := s.newClient("Клиент-Итоги")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	svc1 := s.newService("1000")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svc1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	sub1 := m.Card.Subtasks[0].ID

	p1 := s.newProduct("150.5", "100", "1")
	p2 := s.newProduct("10", "100", "1")
	p3 := s.newProduct("999", "100", "1")

	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+sub1+"/products", map[string]any{"product_id": p1, "qty": 2})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+sub1+"/products", map[string]any{"product_id": p2, "qty": 0.333})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+sub1+"/products", map[string]any{"product_id": p3, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	prod3ID := m.Card.Subtasks[0].Products[2].ID
	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+prod3ID, map[string]any{"qty": 1, "price": nil})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// 1000 + 2*150.50 + round(0.333*10,2) + 0 = 1304.33
	s.Equal(json.Number("1304.33"), m.Card.Subtasks[0].Total)

	svc2 := s.newServiceNoPrice()
	st, m, raw = s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svc2})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var sub2 string
	for _, sub := range m.Card.Subtasks {
		if sub.ServiceID == svc2 {
			sub2 = sub.ID
		}
	}
	s.Require().NotEmpty(sub2)
	p4 := s.newProduct("100", "100", "1")
	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+sub2+"/products", map[string]any{"product_id": p4, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	var sub2Total json.Number
	for _, sub := range m.Card.Subtasks {
		if sub.ID == sub2 {
			sub2Total = sub.Total
		}
	}
	s.Equal(json.Number("100.00"), sub2Total, "«по факту» + 1*100")
	s.Equal(json.Number("1404.33"), m.Card.Task.TotalAmount, "1304.33 + 100.00")

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list struct {
		Items []struct {
			ID          int64       `json:"id"`
			TotalAmount json.Number `json:"total_amount"`
		} `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	found := false
	for _, it := range list.Items {
		if it.ID == taskID {
			s.Equal(json.Number("1404.33"), it.TotalAmount)
			found = true
		}
	}
	s.True(found, "заказ должен быть в списке")
}

// LV-107/AC-012: журнал работ — CRUD + GET /tasks last_log (обрезка 120 + «…»).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Logs_CRUD() {
	c := s.newClient("Клиент-Журнал-CRUD")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": "Начали осмотр"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Logs, 1)
	logRow := m.Card.Subtasks[0].Logs[0]
	s.Equal("Начали осмотр", logRow.Text)
	s.True(logRow.CanEdit)
	logID := logRow.ID

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-logs/"+logID, map[string]any{"text": "Осмотр завершён"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("Осмотр завершён", m.Card.Subtasks[0].Logs[0].Text)

	longText := strings.Repeat("я", 130)
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": longText})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list struct {
		Items []struct {
			ID      int64 `json:"id"`
			LastLog *struct {
				Text string `json:"text"`
			} `json:"last_log"`
		} `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	var lastLog *struct {
		Text string `json:"text"`
	}
	for _, it := range list.Items {
		if it.ID == taskID {
			lastLog = it.LastLog
		}
	}
	s.Require().NotNil(lastLog)
	s.True(strings.HasSuffix(lastLog.Text, "…"), lastLog.Text)
	s.LessOrEqual(len([]rune(lastLog.Text)), 121)

	st, m, raw = s.doCard(s.admin, http.MethodDelete, "/api/v1/subtask-logs/"+logID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Logs, 1, "осталась только длинная запись")
}

// LV-130/AC-002/AC-014: row-мутации НЕ трогают tasks.version.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Version_UnaffectedByRowMutations() {
	c := s.newClient("Клиент-Версия-Устойчива")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c, col)

	svcID := s.newService("100")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID
	s.Equal(version, m.Card.Task.Version)

	productID := s.newProduct("10", "50", "1")
	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(version, m.Card.Task.Version)

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "in_progress", "assignee_id": nil, "due_date": nil, "price": nil, "equipment_id": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(version, m.Card.Task.Version)

	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
		"title": "Заказ карточки", "client_id": c, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw), "version, захваченный ДО row-мутаций, всё ещё актуален")
}

// LV-131/AC-002: два PUT /tasks/:id с одним version — второй 409, данные первого сохранены.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Version_ConflictOnHeader() {
	c := s.newClient("Клиент-Версия-Конфликт")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c, col)

	st, raw := s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
		"title": "Первая правка", "client_id": c, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
		"title": "Вторая правка", "client_id": c, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
	})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TASK_VERSION_CONFLICT", s.errOf(raw).Code)

	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("Первая правка", card.Task.Title)
}

// LV-140/AC-011: финальная колонка — POST /tasks/:id/subtasks → 409, услуг не прибавилось.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_FinalColumn_BlocksCreate() {
	c := s.newClient("Клиент-Финал-Услуга")
	done := s.columnID("Готово")
	taskID, _ := s.newTask(c, done)

	svcID := s.newService("100")
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("SUBTASK_FINAL_COLUMN", s.errOf(raw).Code)

	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(card.Subtasks)
}

// LV-141/AC-011: финальная — done→in_progress 409, done→done с правкой цены — 200.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_FinalColumn_BlocksUndone() {
	c := s.newClient("Клиент-Финал-Статус")
	newCol, done := s.columnID("Новые"), s.columnID("Готово")
	taskID, _ := s.newTask(c, newCol)

	svcID := s.newService("100")
	subtaskID := s.addSubtask(taskID, svcID, "100")
	_, err := s.Pool.Exec(context.Background(), `UPDATE subtasks SET status = 'done' WHERE id = $1`, subtaskID)
	s.Require().NoError(err)

	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/move", taskID), map[string]any{"column_id": done})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subtaskID, map[string]any{
		"status": "in_progress", "assignee_id": nil, "due_date": nil, "price": nil, "equipment_id": nil,
	})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("SUBTASK_FINAL_COLUMN", s.errOf(raw).Code)

	st, m, raw := s.doCard(s.admin, http.MethodPut, "/api/v1/subtasks/"+subtaskID, map[string]any{
		"status": "done", "assignee_id": nil, "due_date": nil, "price": 150, "equipment_id": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NotNil(m.Card.Subtasks[0].Price)
	s.Equal(json.Number("150.00"), *m.Card.Subtasks[0].Price)
}

// LV-145/AC-007: equipment_id не из оборудования ЭТОГО заказа — 400.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_EquipmentNotInOrder400() {
	c := s.newClient("Клиент-Оборудование-НеЗаказа")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	eqID := s.newEquipment(c) // НЕ добавлена в task_equipment

	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "new", "assignee_id": nil, "due_date": nil, "price": nil, "equipment_id": eqID,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_in_order", s.fields(raw)["equipment_id"])
}

// LV-146/AC-007: assignee_id заблокированного — 400 только при СМЕНЕ значения (SD-007).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_InactiveAssignee400() {
	c := s.newClient("Клиент-Исполнитель-Неактивен")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	inactiveID := s.SeedUser("card-inactive-"+subID[:8], "password1", "Неактивный", "executor", false)
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "new", "assignee_id": inactiveID, "due_date": nil, "price": nil, "equipment_id": nil,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["assignee_id"])

	activeID := s.SeedUser("card-active-"+subID[:8], "password1", "Активный", "executor", true)
	st, _, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "new", "assignee_id": activeID, "due_date": nil, "price": nil, "equipment_id": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	_, err := s.Pool.Exec(context.Background(), `UPDATE users SET is_active = false WHERE id = $1`, activeID)
	s.Require().NoError(err)

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "new", "assignee_id": activeID, "due_date": nil, "price": 200, "equipment_id": nil,
	})
	s.Require().Equal(http.StatusOK, st, string(raw), "тот же assignee_id — SD-007 без повторной проверки active")
	s.Require().NotNil(m.Card.Subtasks[0].AssigneeIsActive)
	s.False(*m.Card.Subtasks[0].AssigneeIsActive)
}

// LV-147/AC-007/AC-008: услуга/товар в архиве — 400.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Subtask_ArchivedServiceOrProduct400() {
	c := s.newClient("Клиент-Архив")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	svcID := s.newService("100")
	s.setServiceActive(svcID, false)
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("archived", s.fields(raw)["service_id"])

	activeSvc := s.newService("50")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": activeSvc})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	productID := s.newProduct("10", "50", "1")
	s.setProductActive(productID, false)
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("archived", s.fields(raw)["product_id"])
}

// LV-148/AC-007/AC-012: DELETE /subtasks/:id — CASCADE товары/журнал.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_DeleteSubtask_CascadesLogsProducts() {
	c := s.newClient("Клиент-Каскад-Удаление")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("100")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	productID := s.newProduct("10", "50", "1")
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": "Запись"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, m, raw = s.doCard(s.admin, http.MethodDelete, "/api/v1/subtasks/"+subID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(m.Card.Subtasks)

	var productsCount, logsCount int
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM subtask_products WHERE subtask_id = $1`, subID).Scan(&productsCount))
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM subtask_logs WHERE subtask_id = $1`, subID).Scan(&logsCount))
	s.Zero(productsCount)
	s.Zero(logsCount)
}

// LV-160/AC-017/AC-008: executor — все row-эндпоинты, видит цену товара.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Access_ExecutorRowEndpoints() {
	s.SeedUser("card-exec-row", "password1", "Исполнитель Карточка", "executor", true)
	exec := s.NewBrowser()
	exec.Login("card-exec-row", "password1")

	c := s.newClient("Клиент-Исполнитель-Строки")
	col := s.columnID("Новые")
	taskID, version := s.newTask(c, col)

	svcID := s.newService("100")
	st, m, raw := s.doCard(exec, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	productID := s.newProduct("10", "50", "1")
	st, m, raw = s.doCard(exec, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().NotNil(m.Card.Subtasks[0].Products[0].Price, "executor видит цену товара")

	eqID := s.newEquipment(c)
	st, _, raw = s.doCard(exec, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{"equipment_id": eqID})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, _, raw = s.doCard(exec, http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": "Заметка исполнителя"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, raw = exec.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
		"title": "Заказ карточки", "client_id": c, "column_id": col,
		"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
	})
	s.Equal(http.StatusOK, st, string(raw), "правка без смены is_paid — доступна executor")
}

// LV-163/AC-012: чужая запись журнала — 403 SUBTASK_LOG_FORBIDDEN, admin — 200.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Logs_ForbiddenForOthers() {
	s.SeedUser("card-exec-a", "password1", "Исполнитель А", "executor", true)
	execA := s.NewBrowser()
	execA.Login("card-exec-a", "password1")
	s.SeedUser("card-exec-b", "password1", "Исполнитель Б", "executor", true)
	execB := s.NewBrowser()
	execB.Login("card-exec-b", "password1")

	c := s.newClient("Клиент-Журнал-Чужой")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(execA, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	st, m, raw = s.doCard(execA, http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": "Запись А"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	logID := m.Card.Subtasks[0].Logs[0].ID
	s.True(m.Card.Subtasks[0].Logs[0].CanEdit)

	st, card, raw := s.getCard(execB, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(card.Subtasks[0].Logs[0].CanEdit, "чужая запись — can_edit=false")

	st, raw = execB.Do(http.MethodPut, "/api/v1/subtask-logs/"+logID, map[string]any{"text": "Правка Б"})
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("SUBTASK_LOG_FORBIDDEN", s.errOf(raw).Code)

	st, raw = execB.Do(http.MethodDelete, "/api/v1/subtask-logs/"+logID, nil)
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("SUBTASK_LOG_FORBIDDEN", s.errOf(raw).Code)

	st, card, raw = s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(card.Subtasks[0].Logs[0].CanEdit, "admin can_edit=true")

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtask-logs/"+logID, map[string]any{"text": "Правка админа"})
	s.Equal(http.StatusOK, st, string(raw))
}

// LV-164/AC-017: без сессии — все новые эндпоинты фазы 03 → 401.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_NoSession401() {
	noSession := s.NewBrowser()
	c := s.newClient("Клиент-НетСессии")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	for _, req := range []struct{ method, path string }{
		{http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID)},
		{http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID)},
		{http.MethodPut, "/api/v1/subtasks/" + subID},
		{http.MethodDelete, "/api/v1/subtasks/" + subID},
		{http.MethodPost, "/api/v1/subtasks/" + subID + "/products"},
		{http.MethodPost, "/api/v1/subtasks/" + subID + "/logs"},
	} {
		st, raw := noSession.Do(req.method, req.path, map[string]any{})
		s.Equal(http.StatusUnauthorized, st, "%s %s: %s", req.method, req.path, raw)
	}
}

// LV-171/AC-008/AC-012: границы qty/price/text/completeness/description/status/due_date.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_InvalidInput400() {
	c := s.newClient("Клиент-Невалидный-Ввод")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID
	productID := s.newProduct("10", "50", "1")

	for _, qty := range []any{0, -1, 1.2345} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": qty})
		s.Equal(http.StatusBadRequest, st, "%v → %s", qty, raw)
		s.NotEmpty(s.fields(raw)["qty"], "%v", qty)
	}

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": "5"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("BAD_REQUEST", s.errOf(raw).Code)

	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	prodRowID := m.Card.Subtasks[0].Products[0].ID

	for _, price := range []any{-1, 1.001} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/subtask-products/"+prodRowID, map[string]any{"qty": 1, "price": price})
		s.Equal(http.StatusBadRequest, st, "%v → %s", price, raw)
		s.NotEmpty(s.fields(raw)["price"], "%v", price)
	}

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": "   "})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["text"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": strings.Repeat("я", 4001)})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_long", s.fields(raw)["text"])

	eqID := s.newEquipment(c)
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": eqID, "completeness": strings.Repeat("a", 1001),
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_long", s.fields(raw)["completeness"])

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title": "x", "client_id": c, "column_id": col, "description": strings.Repeat("d", 10001),
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_long", s.fields(raw)["description"])

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID, map[string]any{
		"status": "closed", "assignee_id": nil, "due_date": "2026-02-30", "price": nil, "equipment_id": nil,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	fields := s.fields(raw)
	s.Equal("invalid", fields["status"])
	s.Equal("invalid_date", fields["due_date"])
}

// LV-172 (TD-4): мусорное тело — 400 BAD_REQUEST, не 500; лишнее поле в PUT /subtasks.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_GarbageBody400() {
	c := s.newClient("Клиент-Мусор-Карточка")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	for _, body := range []string{
		`not json`, `[]`, `null`,
		`{"status":"new","task_id":"11111111-1111-1111-1111-111111111111"}`,
	} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subID, body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}
}

// LV-173/AC-014: повторный DELETE /subtask-products/:id — 404, идемпотентность.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_IdempotentDelete404() {
	c := s.newClient("Клиент-Идемпотентность")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID
	productID := s.newProduct("10", "50", "1")
	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products", map[string]any{"product_id": productID, "qty": 1})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	prodRowID := m.Card.Subtasks[0].Products[0].ID

	st, _, raw = s.doCard(s.admin, http.MethodDelete, "/api/v1/subtask-products/"+prodRowID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/subtask-products/"+prodRowID, nil)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("SUBTASK_PRODUCT_NOT_FOUND", s.errOf(raw).Code)
}

// LV-174/AC-014: POST /subtasks/:id/logs удалённой услуги — 404 SUBTASK_NOT_FOUND.
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_LogOnDeletedSubtask404() {
	c := s.newClient("Клиент-Лог-Удалённая-Услуга")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	svcID := s.newService("0")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	st, _, raw = s.doCard(s.admin, http.MethodDelete, "/api/v1/subtasks/"+subID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, raw = s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subID+"/logs", map[string]any{"text": "Опоздал"})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("SUBTASK_NOT_FOUND", s.errOf(raw).Code)
}

// ---- Access Control ----

// D19: оборудование заказа доступно и executor (не только admin).
func (s *OrderCardLiveSuite) TestOrderCardLiveSuite_Executor_EquipmentAllowed() {
	s.SeedUser("card-exec", "password1", "Карточка Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("card-exec", "password1")

	c := s.newClient("Клиент-Исполнитель-Оборудование")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)
	eqID := s.newEquipment(c)

	st, _, raw := s.doEquipment(exec, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID), map[string]any{
		"equipment_id": eqID,
	})
	s.Equal(http.StatusCreated, st, string(raw))

	st, card, raw := s.getCard(exec, taskID)
	s.Equal(http.StatusOK, st, string(raw))
	s.Require().Len(card.Equipment, 1)
}
