package board

import (
	"context"
	"errors"
	"math"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// FuzzColumnInputValidate — TD-7/design-tests.md §Fuzz Targets: normalize()+validate() не
// должны паниковать на произвольном вводе (мусорный цвет, экстремальная позиция).
func FuzzColumnInputValidate(f *testing.F) {
	f.Add("Приёмка", "#94A3B8", false, false, 1)
	f.Add("", "#GGGGGG", false, false, 0)
	f.Add("x", "#fff", true, true, math.MinInt)
	f.Add("y", "", false, true, math.MaxInt)

	f.Fuzz(func(t *testing.T, name, color string, isFinal, consumesStock bool, position int) {
		in := ColumnInput{Name: name, Color: color, IsFinal: isFinal, ConsumesStock: consumesStock, Position: &position}
		_ = in.normalize().validate()
	})
}

// FuzzParseTaskFilter — design-tests.md §Fuzz Targets: одна и та же произвольная строка
// подставляется во ВСЕ query-параметры ParseTaskFilter (url.Values-вход, безопасное
// percent-кодирование через Values.Encode — сырые байты в URL не ломают http.Request).
// Инвариант: без паники; единственная допустимая ошибка — *errorspkg.AppError 400.
func FuzzParseTaskFilter(f *testing.F) {
	for _, s := range []string{"#", "#0", "#999999999999999999999", "%", "_\\", "me", "all", validUUID, "\x00abc", "日本語ラベル"} {
		f.Add(s)
	}

	app := fiber.New()
	var gotErr error
	app.Get("/x", func(c fiber.Ctx) error {
		_, gotErr = ParseTaskFilter(c, admin)
		return c.SendStatus(fiber.StatusOK)
	})

	f.Fuzz(func(t *testing.T, s string) {
		// Кап по БАЙТАМ (не рунам) — невалидный UTF-8 после []rune()/percent-encoding может
		// раздуться до x9 (replacement-рута U+FFFD → 3 байта → "%EF%BF%BD"); тестовый
		// транспорт (fasthttp app.Test) читает запрос через буфер 4096 байт. ParseTaskFilter
		// сама кэпит длину q до 100 рун (taskFilterQMax) — инвариант не теряется.
		if len(s) > 120 {
			s = s[:120]
		}
		v := url.Values{}
		for _, key := range []string{"q", "client_id", "category_id", "paid", "assignee", "due", "limit", "offset"} {
			v.Set(key, s)
		}
		req := httptest.NewRequest(fiber.MethodGet, "/x?"+v.Encode(), nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		_ = resp.Body.Close()
		if gotErr == nil {
			return
		}
		var appErr *errorspkg.AppError
		if !errors.As(gotErr, &appErr) || appErr.HTTP != fiber.StatusBadRequest {
			t.Fatalf("ParseTaskFilter(%q): ошибка не 400 *errorspkg.AppError: %v", s, gotErr)
		}
	})
}

// FuzzTaskInputValidate — design-tests.md §Fuzz Targets: normalize()+validate() не должны
// паниковать; поля ошибок — подмножество {title, client_id, column_id, assignee_id,
// due_date} (version — только у TaskUpdateRequest, здесь не участвует).
func FuzzTaskInputValidate(f *testing.F) {
	f.Add("Заказ", validUUID, validUUID, "", "", "А123ВС 77", validUUID, false)
	f.Add("", "", "", "\x00", "2026-02-30", "", "", true)
	f.Add(strings.Repeat("я", 250), "not-a-uuid", "not-a-uuid", "bad", "bad-date", "А*1", "nope", false)
	f.Add(strings.Repeat("a", 201), validUUID, validUUID, validUUID, "2026-01-15", strings.Repeat("Б", 30), "\x00", true)

	allowed := map[string]bool{"title": true, "client_id": true, "column_id": true, "assignee_id": true, "due_date": true,
		"plate_number": true, "equipment_id": true}

	// one-car-order: клиент и автомобиль — только в TaskCreateRequest (prepare), не в шапке.
	f.Fuzz(func(t *testing.T, title, clientID, columnID, assigneeID, dueDate, plate, equipmentID string, isPaid bool) {
		in := TaskCreateRequest{
			TaskHeader: TaskHeader{
				Title: title, ColumnID: columnID,
				AssigneeID: &assigneeID, DueDate: &dueDate, IsPaid: &isPaid,
			},
			ClientID: clientID, PlateNumber: plate, EquipmentID: equipmentID,
		}
		_, fields := in.normalize().prepare()
		for _, fld := range fields {
			if !allowed[fld.Name] {
				t.Fatalf("неожиданное поле в ошибках валидации: %s", fld.Name)
			}
		}
	})
}

// fakeMoveTaskService — реализация taskService для FuzzMoveBody (§9.1: app.Test —
// единственное допустимое исключение из запрета для новых тестов, т.к. фаззинг без БД
// требует чистой in-process сборки; фейк реализует полный интерфейс структурно).
type fakeMoveTaskService struct{}

func (fakeMoveTaskService) List(context.Context, TaskFilter) (*TaskListResponse, error) {
	return nil, nil
}
func (fakeMoveTaskService) Get(context.Context, Actor, string) (*TaskResponse, error) {
	return nil, nil
}
func (fakeMoveTaskService) Create(context.Context, Actor, TaskCreateRequest) (*TaskMutationResponse, error) {
	return nil, nil
}
func (fakeMoveTaskService) Update(context.Context, Actor, string, TaskUpdateRequest) (*TaskMutationResponse, error) {
	return nil, nil
}
func (fakeMoveTaskService) Delete(context.Context, Actor, string, softdelete.DeleteRequest) (*TaskDeleteResponse, error) {
	return nil, nil
}
func (fakeMoveTaskService) Restore(context.Context, Actor, string) (*TaskRestoreResponse, error) {
	return nil, nil
}
func (fakeMoveTaskService) Purge(context.Context, Actor, string) error { return nil }

// Move — покрывает диапазон {200,400,404,409} по форме входа, без обращения к БД.
func (fakeMoveTaskService) Move(_ context.Context, _ Actor, _ string, in TaskMoveRequest) (*TaskMutationResponse, error) {
	switch {
	case in.ColumnID == "":
		return nil, errorspkg.Validation(errorspkg.Field{Name: "column_id", Message: msgRequired})
	case uuid.Validate(in.ColumnID) != nil:
		return nil, errorspkg.Validation(errorspkg.Field{Name: "column_id", Message: msgInvalid})
	case in.ColumnID == "00000000-0000-0000-0000-000000000000":
		return nil, ErrTaskNotFound
	case in.PrevID != nil && in.NextID != nil && *in.PrevID == *in.NextID:
		return nil, ErrSubtasksNotDone
	default:
		return &TaskMutationResponse{Warnings: []StockWarning{}}, nil
	}
}

// FuzzMoveBody — design-tests.md §Fuzz Targets: POST move через app.Test (исключение
// §9.1) — статус ∈ {200,400,404,409}, никогда 5xx, вне зависимости от формы тела.
func FuzzMoveBody(f *testing.F) {
	f.Add(`{}`)
	f.Add(`{"prev_id":1e99}`)
	f.Add(`[]`)
	f.Add(`null`)
	f.Add(`{"column_id":"` + validUUID + `"}`)
	f.Add(`{"column_id":"` + validUUID + `","prev_id":1,"next_id":1}`)
	f.Add(`{"column_id":"not-a-uuid"}`)

	app := fiber.New(fiber.Config{ErrorHandler: errorspkg.FiberErrorHandler})
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.PrincipalKey, &middleware.Principal{ID: validUUID, Role: middleware.RoleAdmin, IsActive: true})
		return c.Next()
	})
	h := NewTasksHandler(fakeMoveTaskService{})
	app.Post("/tasks/:id/move", h.Move)

	f.Fuzz(func(t *testing.T, body string) {
		req := httptest.NewRequest(fiber.MethodPost, "/tasks/1/move", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			t.Fatalf("5xx для тела %q: %d", body, resp.StatusCode)
		}
	})
}

// fakeCardService — реализация cardService для фаззинга §9.1 (app.Test — без БД):
// каждый метод прогоняет реальные normalize()/validate() DTO этой фазы, чтобы фаззить
// именно bind-парсер + валидатор, не сеть/БД.
type fakeCardService struct{}

func (fakeCardService) Card(context.Context, Actor, string) (*OrderCardResponse, error) {
	return nil, nil
}

func (fakeCardService) CreateSubtask(context.Context, Actor, string, SubtaskCreateRequest) (*OrderCardMutationResponse, error) {
	return nil, nil
}

func (fakeCardService) UpdateSubtask(_ context.Context, _ Actor, _ string, in SubtaskUpdateRequest) (*OrderCardMutationResponse, error) {
	if f := in.normalize().validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	return &OrderCardMutationResponse{Warnings: []StockWarning{}}, nil
}

func (fakeCardService) DeleteSubtask(context.Context, Actor, string) (*OrderCardMutationResponse, error) {
	return nil, nil
}

func (fakeCardService) CreateSubtaskProduct(_ context.Context, _ Actor, _ string, in SubtaskProductCreateRequest) (*OrderCardMutationResponse, error) {
	if f := in.normalize().validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	return &OrderCardMutationResponse{Warnings: []StockWarning{}}, nil
}

func (fakeCardService) UpdateSubtaskProduct(_ context.Context, _ Actor, _ string, in SubtaskProductUpdateRequest) (*OrderCardMutationResponse, error) {
	if f := in.normalize().validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	return &OrderCardMutationResponse{Warnings: []StockWarning{}}, nil
}

func (fakeCardService) DeleteSubtaskProduct(context.Context, Actor, string) (*OrderCardMutationResponse, error) {
	return nil, nil
}

func (fakeCardService) CreateLog(_ context.Context, _ Actor, _ string, in SubtaskLogRequest) (*OrderCardMutationResponse, error) {
	if f := in.normalize().validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	return &OrderCardMutationResponse{Warnings: []StockWarning{}}, nil
}

func (fakeCardService) UpdateLog(context.Context, Actor, string, SubtaskLogRequest) (*OrderCardMutationResponse, error) {
	return nil, nil
}

func (fakeCardService) DeleteLog(context.Context, Actor, string) (*OrderCardMutationResponse, error) {
	return nil, nil
}

func newFakeCardApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: errorspkg.FiberErrorHandler})
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.PrincipalKey, &middleware.Principal{ID: validUUID, Role: middleware.RoleAdmin, IsActive: true})
		return c.Next()
	})
	return app
}

// assertNo5xx — общий инвариант FZ-001…003 (design-tests.md §Fuzzing): no panic, только
// 400/404/409/2xx, никогда 5xx.
func assertNo5xx(t *testing.T, app *fiber.App, method, path, body string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Fatalf("5xx для %s %s тела %q: %d", method, path, body, resp.StatusCode)
	}
}

// FuzzSubtaskUpdateBody — FZ-001: PUT /subtasks/:id (SubtaskUpdateRequest).
func FuzzSubtaskUpdateBody(f *testing.F) {
	for _, s := range []string{
		`{}`, `null`, `[]`, `{"qty":1e309}`, `{"status":"new"}`,
		`{"status":"new","price":1e309}`, `{"status":"new","price":"NaN"}`,
		`{"status":"new","price":-0}`, `{"status":"` + strings.Repeat("я", 4001) + `"}`,
		`{"status":"new","due_date":"\u0000"}`, `{"status":"new","assignee_id":{"nested":true}}`,
	} {
		f.Add(s)
	}

	app := newFakeCardApp()
	h := NewCardHandler(fakeCardService{})
	app.Put("/subtasks/:id", h.UpdateSubtask)

	f.Fuzz(func(t *testing.T, body string) {
		assertNo5xx(t, app, fiber.MethodPut, "/subtasks/"+validUUID, body)
	})
}

// FuzzSubtaskProductBody — FZ-002: POST /subtasks/:id/products, PUT /subtask-products/:id.
func FuzzSubtaskProductBody(f *testing.F) {
	for _, s := range []string{
		`{}`, `null`, `[]`, `{"qty":1e309}`, `{"qty":"NaN"}`, `{"qty":-0}`,
		`{"product_id":"` + validUUID + `","qty":0.5}`, `{"qty":1,"price":1e309}`,
		`{"qty":{"nested":true}}`, `\u0000`,
	} {
		f.Add(s)
	}

	app := newFakeCardApp()
	h := NewCardHandler(fakeCardService{})
	app.Post("/subtasks/:id/products", h.CreateSubtaskProduct)
	app.Put("/subtask-products/:id", h.UpdateSubtaskProduct)

	f.Fuzz(func(t *testing.T, body string) {
		assertNo5xx(t, app, fiber.MethodPost, "/subtasks/"+validUUID+"/products", body)
		assertNo5xx(t, app, fiber.MethodPut, "/subtask-products/"+validUUID, body)
	})
}

// FuzzSubtaskLogBody — FZ-003: POST /subtasks/:id/logs.
func FuzzSubtaskLogBody(f *testing.F) {
	for _, s := range []string{
		`{}`, `null`, `[]`, `{"text":""}`, `{"text":"` + strings.Repeat("я", 4001) + `"}`,
		`{"text":"\u0000"}`, `{"text":"Запись","extra":1}`, `{"text":{"nested":true}}`,
	} {
		f.Add(s)
	}

	app := newFakeCardApp()
	h := NewCardHandler(fakeCardService{})
	app.Post("/subtasks/:id/logs", h.CreateLog)

	f.Fuzz(func(t *testing.T, body string) {
		assertNo5xx(t, app, fiber.MethodPost, "/subtasks/"+validUUID+"/logs", body)
	})
}
