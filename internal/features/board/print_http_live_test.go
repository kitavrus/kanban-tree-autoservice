//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// PrintLiveSuite (§TD-8, TD-4): печать актов приёма/выдачи (design-go-layers.md §Handler
// Print, ADR-102). Реальный бинарь + TCP + реальный логин — auth-стабы и app.Test запрещены.
// FIRM_* не заданы (дефолт окружения LiveSuite) — сьюта тем самым проверяет и «полное
// содержимое» акта, и «пустые реквизиты не рендерятся» на одном и том же процессе.
type PrintLiveSuite struct {
	testutil.LiveSuite
	admin    *testutil.Browser
	executor *testutil.Browser
}

func TestPrintLiveSuite(t *testing.T) {
	suite.Run(t, &PrintLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *PrintLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("print-admin", "password1", "Печать Админ", "admin", true)
	s.SeedUser("print-exec", "password1", "Печать Исполнитель", "executor", true)
	s.admin = s.NewBrowser()
	s.admin.Login("print-admin", "password1")
	s.executor = s.NewBrowser()
	s.executor.Login("print-exec", "password1")
}

// ---- фикстуры (самодостаточные — не переиспользуют OrderCardLiveSuite: другой сьют) ----

func (s *PrintLiveSuite) newClient(company, phone string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO clients (company_name) VALUES ($1) RETURNING id`, company).Scan(&id))
	if phone != "" {
		_, err := s.Pool.Exec(context.Background(),
			`INSERT INTO client_phones (client_id, position, phone) VALUES ($1, 1, $2)`, id, phone)
		s.Require().NoError(err)
	}
	return id
}

func (s *PrintLiveSuite) columnID(name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM columns WHERE name = $1`, name).Scan(&id))
	return id
}

type printTaskCreated struct {
	Task struct {
		ID      int64 `json:"id"`
		Version int64 `json:"version"`
	} `json:"task"`
}

func (s *PrintLiveSuite) newTask(clientID, columnID, description string) (int64, int64) {
	return s.newTaskWithPlate(clientID, columnID, description, testPlate())
}

// newTaskWithPlate — заказ с новым автомобилем клиента с госномером plate (one-car-order).
func (s *PrintLiveSuite) newTaskWithPlate(clientID, columnID, description, plate string) (int64, int64) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", withPlan(map[string]any{
		"title": "Заказ печати", "client_id": clientID, "column_id": columnID, "description": description,
		"plate_number": plate, "new_equipment": map[string]any{},
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m printTaskCreated
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	return m.Task.ID, m.Task.Version
}

func (s *PrintLiveSuite) markPaid(taskID, clientID, columnID string, version int64) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/tasks/"+taskID, map[string]any{
		"title": "Заказ печати", "column_id": columnID,
		"assignee_id": nil, "start_date": testToday(), "duration_days": 1, "is_paid": true, "version": version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *PrintLiveSuite) newServiceCategory() string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'PrintLive Svc ' || gen_random_uuid()) RETURNING id`).Scan(&id))
	return id
}

func (s *PrintLiveSuite) newService(name, price string) string {
	var priceArg any
	if price == "" {
		priceArg = nil
	} else {
		priceArg = price
	}
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO services (category_id, name, price) VALUES ($1, $2, $3::numeric) RETURNING id`,
		s.newServiceCategory(), name, priceArg).Scan(&id))
	return id
}

func (s *PrintLiveSuite) newProduct(name, price, stock, minStock string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO products (name, unit, price, stock, min_stock) VALUES ($1, 'шт', $2::numeric, $3::numeric, $4::numeric) RETURNING id`,
		name, price, stock, minStock).Scan(&id))
	return id
}

type printSubtaskCreated struct {
	Card struct {
		Subtasks []struct {
			ID        string `json:"id"`
			ServiceID string `json:"service_id"`
		} `json:"subtasks"`
	} `json:"card"`
}

// addSubtask — POST /tasks/:id/subtasks, возвращает id только что созданной строки
// (последняя услуга в списке карточки — заказ этой фазы, position — в конец).
func (s *PrintLiveSuite) addSubtask(taskID int64, serviceID string) string {
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": serviceID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m printSubtaskCreated
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	for _, sub := range m.Card.Subtasks {
		if sub.ServiceID == serviceID {
			return sub.ID
		}
	}
	s.Require().Fail("подзадача не найдена в ответе", string(raw))
	return ""
}

// setSubtaskStatus — PUT /subtasks/:id полностью заменяет изменяемые поля строки (LWW,
// design-go-layers.md §DTO Row Requests) — price передаётся явно, иначе status=done
// незаметно обнулил бы цену услуги (price=nil = «по факту»).
func (s *PrintLiveSuite) setSubtaskStatus(subtaskID, status string, price any) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+subtaskID, map[string]any{
		"status": status, "assignee_id": nil, "due_date": nil, "price": price,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *PrintLiveSuite) addProduct(subtaskID, productID string, qty float64) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/products", map[string]any{
		"product_id": productID, "qty": qty,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
}

// ---- LV-180/AC-015: акт приёма — реквизиты (пустые FIRM_* — блок опущен), #N, дата,
// клиент+телефоны, автомобиль (только госномер — one-car-order), описание (экранировано),
// подписи «Сдал»/«Принял». Доступно
// и executor (D19), не только admin. ----

func (s *PrintLiveSuite) TestPrintLiveSuite_Intake_FullContent() {
	c := s.newClient("Клиент-Печать-Приём", "8 900 000-00-01")
	col := s.columnID("Приёмка")
	taskID, _ := s.newTaskWithPlate(c, col, "<b>Не включается</b>\n\nВторой абзац", "П123РН 777")

	resp := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/intake", taskID), nil, nil)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Contains(resp.Header.Get("Content-Type"), "text/html")

	body := s.readBody(resp)
	s.Contains(body, fmt.Sprintf("Акт приёма №%d", taskID))
	s.Contains(body, "Клиент-Печать-Приём")
	s.Contains(body, "8 900 000-00-01")
	s.Contains(body, `data-testid="print-plate-number">П123РН 777<`, "госномер автомобиля в акте")
	s.NotContains(body, "Комплектность", "комплектность/состояние в актах не печатаются")
	s.NotContains(body, "Состояние")
	s.Contains(body, "&lt;b&gt;Не включается&lt;/b&gt;", "описание — серверно-экранированный HTML (ADR-006)")
	s.NotContains(body, "<b>Не включается</b>")
	s.Contains(body, "Сдал:")
	s.Contains(body, "Принял:")
	// FIRM_* не заданы этому сьюту (дефолт LiveSuite) — блок реквизитов пуст (LV-182 добивает
	// явными негативными проверками), здесь достаточно не упасть.

	// D19: доступно исполнителю, не только admin.
	respExec := s.executor.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/intake", taskID), nil, nil)
	defer respExec.Body.Close()
	s.Equal(http.StatusOK, respExec.StatusCode)
}

// ---- LV-181/AC-016 (SD-009): акт выдачи — ВСЕ услуги со статусом и ценой (не только
// выполненные), «по факту» — прочерк, товары qty×цена, итог = task.total_amount, отметка
// оплаты, подписи «Выдал»/«Получил». ----

func (s *PrintLiveSuite) TestPrintLiveSuite_Handover_FullContent() {
	c := s.newClient("Клиент-Печать-Выдача", "8 900 000-00-02")
	col := s.columnID("Приёмка")
	taskID, version := s.newTask(c, col, "")

	svcDone := s.newService("Диагностика", "500")
	subDone := s.addSubtask(taskID, svcDone)
	s.setSubtaskStatus(subDone, "done", 500)
	prodID := s.newProduct("Термопаста", "20", "100", "1")
	s.addProduct(subDone, prodID, 2)

	svcFact := s.newService("Чистка по факту", "")
	s.addSubtask(taskID, svcFact)

	s.markPaid(fmt.Sprintf("%d", taskID), c, col, version)

	resp := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/handover", taskID), nil, nil)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Contains(resp.Header.Get("Content-Type"), "text/html")

	body := s.readBody(resp)
	s.Contains(body, fmt.Sprintf("Акт выдачи №%d", taskID))
	s.Contains(body, "Диагностика")
	s.Contains(body, "Выполнена")
	s.Contains(body, "Термопаста")
	s.Contains(body, "Чистка по факту")
	s.Contains(body, "Новая", "вторая услуга осталась в статусе new — акт печатает ВСЕ услуги (SD-009)")
	s.Contains(body, ">—<", "цена работы «по факту» — прочерк")
	// итог: 500.00 (услуга) + 2*20.00 (товар) = 540.00 — совпадает с task.total_amount (AC-013).
	s.Contains(body, "540.00")
	// работы и запчасти считаются отдельно: 500.00 (цена услуги) + 40.00 (запчасти).
	s.Contains(body, "Работы: 500.00")
	s.Contains(body, "Запчасти: 40.00")
	s.Contains(body, `data-testid="print-subtask-parts-total">40.00<`)
	s.Contains(body, "Оплачено")
	s.Contains(body, "Выдал:")
	s.Contains(body, "Получил:")
}

// ---- LV-182: FIRM_* пустые → блок реквизитов не рендерится; неизвестный kind/несуществующий
// заказ → 404; XSS в описании экранирован. ----

func (s *PrintLiveSuite) TestPrintLiveSuite_EmptyFirmAndEdgeCases() {
	c := s.newClient("Клиент-Печать-Грани", "")
	col := s.columnID("Приёмка")
	taskID, _ := s.newTask(c, col, "<script>alert(1)</script>")

	resp := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/intake", taskID), nil, nil)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	body := s.readBody(resp)

	// FIRM_NAME/FIRM_ADDRESS/FIRM_PHONE не заданы (дефолт LiveSuite) — ни один блок
	// реквизитов не отрисован (design-go-layers.md §Handler Print: `{{ if }}` на КАЖДОЕ поле).
	s.NotContains(body, `class="print-firm-name"`)
	s.NotContains(body, `class="print-firm-address"`)
	s.NotContains(body, `class="print-firm-phone"`)

	// XSS в описании — экранирован сервером (ADR-006), сырой тег в разметку не попадает.
	s.NotContains(body, "<script>alert(1)</script>")
	s.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;")

	// kind вне PrintKinds → 404-страница (не JSON-ошибка API).
	resp2 := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/other", taskID), nil, nil)
	defer resp2.Body.Close()
	s.Equal(http.StatusNotFound, resp2.StatusCode)

	// несуществующий заказ → 404.
	resp3 := s.admin.Request(http.MethodGet, "/tasks/999999999/print/intake", nil, nil)
	defer resp3.Body.Close()
	s.Equal(http.StatusNotFound, resp3.StatusCode)

	// аноним → редирект на /login (RequireSSR, cache-control no-store — as-is каркас).
	anon := s.NewBrowser()
	resp4 := anon.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/intake", taskID), nil, nil)
	defer resp4.Body.Close()
	s.Equal(http.StatusFound, resp4.StatusCode)
	s.True(strings.HasPrefix(resp4.Header.Get("Location"), "/login"), resp4.Header.Get("Location"))
	s.Equal("no-store, must-revalidate", resp4.Header.Get("Cache-Control"))
}

// ---- PDF-версия акта выдачи: /tasks/:id/print/handover/pdf — файл application/pdf;
// иной kind/чужой заказ — 404, аноним — редирект на /login. ----

func (s *PrintLiveSuite) TestPrintLiveSuite_HandoverPDF() {
	c := s.newClient("Клиент-Печать-PDF", "8 900 000-00-03")
	col := s.columnID("Приёмка")
	taskID, _ := s.newTask(c, col, "")
	svc := s.newService("Диагностика PDF", "500")
	sub := s.addSubtask(taskID, svc)
	prodID := s.newProduct("Термопаста PDF", "20", "100", "1")
	s.addProduct(sub, prodID, 2)
	s.addSubtask(taskID, s.newService("Чистка PDF по факту", ""))

	for _, b := range []*testutil.Browser{s.admin, s.executor} {
		resp := b.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/handover/pdf", taskID), nil, nil)
		s.Require().Equal(http.StatusOK, resp.StatusCode)
		s.Equal("application/pdf", resp.Header.Get("Content-Type"))
		s.Equal(fmt.Sprintf(`attachment; filename="akt-vydachi-%d.pdf"`, taskID), resp.Header.Get("Content-Disposition"))
		body := s.readBody(resp)
		resp.Body.Close()
		s.True(strings.HasPrefix(body, "%PDF-"), "тело — PDF-документ")
	}

	// HTML-страница акта выдачи ведёт на PDF-версию.
	respHTML := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/handover", taskID), nil, nil)
	defer respHTML.Body.Close()
	s.Contains(s.readBody(respHTML), fmt.Sprintf(`href="/tasks/%d/print/handover/pdf"`, taskID))

	resp2 := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/intake/pdf", taskID), nil, nil)
	defer resp2.Body.Close()
	s.Equal(http.StatusNotFound, resp2.StatusCode)

	resp3 := s.admin.Request(http.MethodGet, "/tasks/999999999/print/handover/pdf", nil, nil)
	defer resp3.Body.Close()
	s.Equal(http.StatusNotFound, resp3.StatusCode)

	anon := s.NewBrowser()
	resp4 := anon.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/handover/pdf", taskID), nil, nil)
	defer resp4.Body.Close()
	s.Equal(http.StatusFound, resp4.StatusCode)
	s.True(strings.HasPrefix(resp4.Header.Get("Location"), "/login"), resp4.Header.Get("Location"))
}

func (s *PrintLiveSuite) readBody(resp *http.Response) string {
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return string(raw)
}
