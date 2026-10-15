//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление заказа (docs/features/soft-delete-core/draft-spec.md §11) — методы
// OrderCardLiveSuite (§TD-8: боевой бинарь, реальный логин; сьюта запущена с
// TEST_PURGE_ENABLED=true): TS-001 (причина), TS-002 (удаление и возврат товара), TS-003
// (повтор и гонка), TS-004 / TS-005 (восстановление, нехватка товара), TS-006 (читатели),
// TS-007 (мутации удалённого заказа), TS-012 (CHECK миграции), TS-013 (служебная уборка),
// TS-033 (права), TS-049 (удалённый заказ удерживает клиента и единицу; колонку — уже нет,
// soft-delete-board-columns TS-019), TS-052
// (колонка стала финальной). TS-008 — AttachmentsLiveSuite.

// ---- wire-типы (1:1 с board.TaskDeleteResponse / TaskRestoreResponse / PurgeResponse) ----

type sdWarningDTO struct {
	ProductID string      `json:"product_id"`
	Name      string      `json:"name"`
	Stock     json.Number `json:"stock"`
	Kind      string      `json:"kind"`
}

type sdDeleteDTO struct {
	ID        int64          `json:"id"`
	DeletedAt time.Time      `json:"deleted_at"`
	Warnings  []sdWarningDTO `json:"warnings"`
}

type sdNoticeDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type sdRestoreDTO struct {
	ID       int64          `json:"id"`
	Warnings []sdWarningDTO `json:"warnings"`
	Notices  []sdNoticeDTO  `json:"notices"`
}

type sdErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *struct {
		Fields []struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"details"`
}

func (s *OrderCardLiveSuite) sdErr(raw []byte) sdErrDTO {
	var e sdErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

// sdField — код ошибки поля name из details.fields[] («» — поля нет).
func (s *OrderCardLiveSuite) sdField(raw []byte, name string) string {
	e := s.sdErr(raw)
	if e.Details == nil {
		return ""
	}
	for _, f := range e.Details.Fields {
		if f.Name == name {
			return f.Message
		}
	}
	return ""
}

// ---- фикстуры ----

func taskPath(id int64, tail string) string { return fmt.Sprintf("/api/v1/tasks/%d%s", id, tail) }

func (s *OrderCardLiveSuite) softDelete(b *testutil.Browser, taskID int64, reason string) (int, sdDeleteDTO, []byte) {
	st, raw := b.Do(http.MethodDelete, taskPath(taskID, ""), map[string]any{"reason": reason})
	var out sdDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *OrderCardLiveSuite) mustSoftDelete(taskID int64) sdDeleteDTO {
	st, out, raw := s.softDelete(s.admin, taskID, "Создан по ошибке")
	s.Require().Equal(http.StatusOK, st, string(raw))
	return out
}

func (s *OrderCardLiveSuite) restoreTask(b *testutil.Browser, taskID int64) (int, sdRestoreDTO, []byte) {
	st, raw := b.Do(http.MethodPost, taskPath(taskID, "/restore"), nil)
	var out sdRestoreDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *OrderCardLiveSuite) mustRestore(taskID int64) sdRestoreDTO {
	st, out, raw := s.restoreTask(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	return out
}

// sdExecutor — исполнитель с уникальным логином, вошедший через реальный логин; id — users.id.
func (s *OrderCardLiveSuite) sdExecutor() (*testutil.Browser, string) {
	login := "sd-exec-" + uuid.NewString()[:8]
	id := s.SeedUser(login, "password1", "Исполнитель "+login, "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b, id
}

func (s *OrderCardLiveSuite) sdScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

// defaultStock — остаток товара.
func (s *OrderCardLiveSuite) defaultStock(productID string) string {
	return s.sdScalar(`SELECT stock::text FROM products WHERE id = $1`, productID)
}

// sdTaskWithProduct — заказ клиента clientID в колонке columnName с одной услугой (new) и
// товаром qty (остаток товара — stock). Возвращает заказ, услугу,
// товар. Товар и услуга добавляются через API карточки.
func (s *OrderCardLiveSuite) sdTaskWithProduct(clientID, columnID, stock string, qty int) (taskID int64, subtaskID, productID string) {
	taskID, _ = s.newTask(clientID, columnID)
	subtaskID = s.addSubtask(taskID, s.newService("1000"), "1000")
	productID = s.newProduct("50", stock, "0")
	st, _, raw := s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/products", map[string]any{"product_id": productID, "qty": qty})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return taskID, subtaskID, productID
}

func (s *OrderCardLiveSuite) sdMove(taskID int64, columnID string) {
	st, raw := s.admin.Do(http.MethodPost, taskPath(taskID, "/move"), map[string]any{"column_id": columnID})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// taskMovements — движения остатка заказа: «reason:qty_delta», по порядку записи.
func (s *OrderCardLiveSuite) taskMovements(taskID int64) string {
	return s.sdScalar(`
		SELECT coalesce(string_agg(reason || ':' || qty_delta::text, ',' ORDER BY created_at, id), '')
		FROM stock_movements WHERE task_id = $1`, taskID)
}

func (s *OrderCardLiveSuite) taskStockRows(taskID int64) string {
	return s.sdScalar(`SELECT count(*)::text FROM task_stock WHERE task_id = $1`, taskID)
}

// sdColumn — своя колонка теста (не трогаем общие колонки доски): нефинальная, без списания.
func (s *OrderCardLiveSuite) sdColumn(name string) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/columns", map[string]any{
		"name": name, "color": "#445566", "is_final": false, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Column struct {
			ID string `json:"id"`
		} `json:"column"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	s.Require().NotEmpty(m.Column.ID, string(raw))
	return m.Column.ID
}

func (s *OrderCardLiveSuite) sdUpdateColumn(id, name string, isFinal, consumes bool) (int, []byte) {
	return s.admin.Do(http.MethodPut, "/api/v1/columns/"+id, map[string]any{
		"name": name, "color": "#445566", "is_final": isFinal, "consumes_stock": consumes,
	})
}

func (s *OrderCardLiveSuite) purge(b *testutil.Browser, kind, id string) (int, []byte) {
	return b.Do(http.MethodDelete, "/api/v1/test/purge/"+kind+"/"+id, nil)
}

// ---- TS-001 ----

// TS-001 (DS-020): причина обязательна — тело без reason, пустая и из пробелов → 400
// required; 501 символ → too_long; NUL → invalid_text; заказ при отказе не удалён. 500
// символов с пробелами по краям принимаются, причина хранится обрезанной.
func (s *OrderCardLiveSuite) TestTS001_DeleteReasonValidation() {
	c := s.newClient("Клиент-Причина-Удаления")
	taskID, _ := s.newTask(c, s.columnID("Приёмка"))

	for _, tc := range []struct {
		name string
		body any
		want string
	}{
		{"тела нет", nil, "required"},
		{"тело без reason", map[string]any{}, "required"},
		{"reason null", map[string]any{"reason": nil}, "required"},
		{"пусто", map[string]any{"reason": ""}, "required"},
		{"одни пробелы", map[string]any{"reason": "  \t "}, "required"},
		{"501 символ", map[string]any{"reason": strings.Repeat("я", 501)}, "too_long"},
		{"NUL", map[string]any{"reason": "при\u0000чина"}, "invalid_text"},
	} {
		st, raw := s.admin.Do(http.MethodDelete, taskPath(taskID, ""), tc.body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", tc.name, raw)
		s.Equal("VALIDATION", s.sdErr(raw).Code, tc.name)
		s.Equal(tc.want, s.sdField(raw, "reason"), "%s: %s", tc.name, raw)
		s.Equal("false", s.sdScalar(`SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $1`, taskID), "%s: заказ не удалён", tc.name)
	}

	// тело не JSON (форма, text/plain) — тот же единый формат: 400 VALIDATION с details.fields[].
	resp := s.admin.Request(http.MethodDelete, taskPath(taskID, ""), "reason=x", map[string]string{"Content-Type": "text/plain"})
	plain, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Equal(http.StatusBadRequest, resp.StatusCode, string(plain))
	s.Equal("VALIDATION", s.sdErr(plain).Code, string(plain))
	s.Equal("required", s.sdField(plain, "reason"), string(plain))
	s.Equal("false", s.sdScalar(`SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $1`, taskID))

	long := strings.Repeat("я", 500)
	st, out, raw := s.softDelete(s.admin, taskID, "  "+long+" \n")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(taskID, out.ID)
	s.Equal(long, s.sdScalar(`SELECT delete_reason FROM tasks WHERE id = $1`, taskID), "крайние пробелы обрезаны")
}

// ---- TS-002 ----

// TS-002 (DS-004): удаление заказа в списывающей колонке — 200 {id, deleted_at, warnings},
// три поля удаления заполнены (кто — из сессии), version + 1, task_stock пуст, остаток вырос
// на количество заказа, движения — списание и возврат; строка, услуги и товары услуг на
// месте. Негатив: исполнитель → 403 «Нет доступа», остатки не изменились.
func (s *OrderCardLiveSuite) TestTS002_SoftDeleteReturnsStock() {
	c := s.newClient("Клиент-Мягкое-Удаление")
	taskID, _, productID := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
	s.sdMove(taskID, s.columnID("В ремонте"))
	s.Equal("6.000", s.defaultStock(productID))
	s.Equal("1", s.taskStockRows(taskID))
	versionBefore := s.sdScalar(`SELECT version::text FROM tasks WHERE id = $1`, taskID)

	exec, _ := s.sdExecutor()
	st, _, raw := s.softDelete(exec, taskID, "Создан по ошибке")
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("FORBIDDEN", s.sdErr(raw).Code)
	s.Equal("Нет доступа", s.sdErr(raw).Message)
	s.Equal("6.000", s.defaultStock(productID), "отказ остаток не меняет")
	s.Equal("1", s.taskStockRows(taskID))

	before := time.Now().Add(-time.Minute)
	st, out, raw := s.softDelete(s.admin, taskID, "Создан по ошибке")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(taskID, out.ID)
	s.NotNil(out.Warnings, "warnings — массив, не null: %s", raw)
	s.True(out.DeletedAt.After(before) && out.DeletedAt.Before(time.Now().Add(time.Minute)), "deleted_at — момент удаления: %s", out.DeletedAt)
	s.Equal(time.UTC, out.DeletedAt.Location(), "deleted_at в UTC: %s", raw)

	s.Equal("card-admin|Создан по ошибке|true", s.sdScalar(`
		SELECT u.login || '|' || t.delete_reason || '|' || (t.deleted_at IS NOT NULL)::text
		FROM tasks t JOIN users u ON u.id = t.deleted_by WHERE t.id = $1`, taskID))
	s.Equal("1", s.sdScalar(`SELECT (version - $2::bigint)::text FROM tasks WHERE id = $1`, taskID, versionBefore), "version + 1")
	s.Equal("0", s.taskStockRows(taskID), "удалённый заказ товар не держит")
	s.Equal("10.000", s.defaultStock(productID), "товар вернулся на склад")
	s.Equal("task_consume:-4.000,task_return:4.000", s.taskMovements(taskID))
	s.Equal("1|1", s.sdScalar(`
		SELECT (SELECT count(*) FROM subtasks WHERE task_id = $1)::text || '|' ||
		       (SELECT count(*) FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = $1)::text`, taskID),
		"части заказа остаются")
}

// ---- TS-003 ----

// TS-003 (DS-024): повтор удаления → 404 TASK_NOT_FOUND; два параллельных удаления — ровно
// один 200 и один 404, возврат товара выполнен один раз, причина проигравшего не записана.
func (s *OrderCardLiveSuite) TestTS003_RepeatAndParallelDelete() {
	c := s.newClient("Клиент-Гонка-Удаления")
	wip := s.columnID("В ремонте")

	taskID, _, productID := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
	s.sdMove(taskID, wip)
	s.mustSoftDelete(taskID)
	st, _, raw := s.softDelete(s.admin, taskID, "Повтор")
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TASK_NOT_FOUND", s.sdErr(raw).Code)
	s.Equal("Создан по ошибке", s.sdScalar(`SELECT delete_reason FROM tasks WHERE id = $1`, taskID), "причина повтора не записана")
	s.Equal("10.000", s.defaultStock(productID))

	second := s.NewBrowser()
	second.Login("card-admin", "password1")
	for i := 0; i < 5; i++ {
		raceID, _, raceProduct := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
		s.sdMove(raceID, wip)

		browsers := []*testutil.Browser{s.admin, second}
		reasons := []string{"Причина первого", "Причина второго"}
		statuses := make([]int, 2)
		bodies := make([][]byte, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for j := range browsers {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				<-start
				statuses[j], bodies[j] = browsers[j].Do(http.MethodDelete, taskPath(raceID, ""), map[string]any{"reason": reasons[j]})
			}(j)
		}
		close(start)
		wg.Wait()

		winner := -1
		for j, code := range statuses {
			switch code {
			case http.StatusOK:
				s.Require().Equal(-1, winner, "итерация %d: два успешных удаления", i)
				winner = j
			case http.StatusNotFound:
				s.Equal("TASK_NOT_FOUND", s.sdErr(bodies[j]).Code)
			default:
				s.Failf("неожиданный ответ", "итерация %d: %d %s", i, code, bodies[j])
			}
		}
		s.Require().NotEqual(-1, winner, "итерация %d: ни одного 200: %v", i, statuses)
		s.Equal(reasons[winner], s.sdScalar(`SELECT delete_reason FROM tasks WHERE id = $1`, raceID), "записана причина победителя")
		s.Equal("task_consume:-4.000,task_return:4.000", s.taskMovements(raceID), "возврат товара выполнен один раз")
		s.Equal("10.000", s.defaultStock(raceProduct))
	}
}

// ---- TS-004 ----

// TS-004 (DS-005): восстановление заказа в списывающей колонке — 200 {id, warnings, notices},
// три поля удаления очищены, version + 1, task_stock и остаток как до удаления, колонка и
// позиция прежние. Негатив: повтор восстановления → 409 NOT_DELETED, остатки не изменились;
// восстановление несуществующего заказа → 404.
func (s *OrderCardLiveSuite) TestTS004_RestoreConsumesAgain() {
	c := s.newClient("Клиент-Восстановление")
	wip := s.columnID("В ремонте")
	taskID, _, productID := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
	s.sdMove(taskID, wip)
	place := s.sdScalar(`SELECT column_id::text || '|' || position::text FROM tasks WHERE id = $1`, taskID)
	s.mustSoftDelete(taskID)
	s.Equal("10.000", s.defaultStock(productID))
	versionDeleted := s.sdScalar(`SELECT version::text FROM tasks WHERE id = $1`, taskID)

	st, out, raw := s.restoreTask(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(taskID, out.ID)
	s.NotNil(out.Warnings, string(raw))
	s.Empty(out.Warnings, "товара хватает: %s", raw)
	s.NotNil(out.Notices, "notices — массив, не null: %s", raw)
	s.Empty(out.Notices)

	s.Equal("true", s.sdScalar(`
		SELECT (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)::text FROM tasks WHERE id = $1`, taskID),
		"три поля удаления очищены")
	s.Equal("1", s.sdScalar(`SELECT (version - $2::bigint)::text FROM tasks WHERE id = $1`, taskID, versionDeleted), "version + 1")
	s.Equal(place, s.sdScalar(`SELECT column_id::text || '|' || position::text FROM tasks WHERE id = $1`, taskID), "колонка и позиция прежние")
	s.Equal("6.000", s.defaultStock(productID), "товар списан снова")
	s.Equal("4.000", s.sdScalar(`SELECT qty::text FROM task_stock WHERE task_id = $1`, taskID))
	s.Equal("task_consume:-4.000,task_return:4.000,task_consume:-4.000", s.taskMovements(taskID))
	st, card, _ := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, "заказ снова открывается")
	s.Len(card.Subtasks, 1)

	st, _, raw = s.restoreTask(s.admin, taskID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NOT_DELETED", s.sdErr(raw).Code)
	s.Equal("Запись уже восстановлена", s.sdErr(raw).Message)
	s.Equal("6.000", s.defaultStock(productID), "повтор остаток не меняет")

	st, _, raw = s.restoreTask(s.admin, 999999999)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TASK_NOT_FOUND", s.sdErr(raw).Code)
}

// ---- TS-005 ----

// TS-005 (DS-005): нехватка товара при восстановлении — не отказ: 200, warnings содержит товар
// с kind = negative, остаток ушёл в минус, заказ восстановлен.
func (s *OrderCardLiveSuite) TestTS005_RestoreWithShortage() {
	c := s.newClient("Клиент-Нехватка")
	taskID, _, productID := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
	s.sdMove(taskID, s.columnID("В ремонте"))
	s.mustSoftDelete(taskID)

	// товар ушёл со склада, пока заказ был удалён (фикстура остатка).
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE products SET stock = 1 WHERE id = $1`, productID)
	s.Require().NoError(err)

	st, out, raw := s.restoreTask(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, "восстановление не отклоняется: %s", raw)
	s.Require().Len(out.Warnings, 1, string(raw))
	w := out.Warnings[0]
	s.Equal(productID, w.ProductID)
	s.Equal("negative", w.Kind)
	s.Equal("-3.000", s.defaultStock(productID))
	s.Equal("false", s.sdScalar(`SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $1`, taskID), "заказ восстановлен")
}

// ---- TS-006 ----

type sdStockListDTO struct {
	Items []struct {
		ProductID string      `json:"product_id"`
		Pending   json.Number `json:"pending"`
	} `json:"items"`
}

type sdPayoutListDTO struct {
	Items []struct {
		SubtaskID string `json:"subtask_id"`
		TaskID    int64  `json:"task_id"`
	} `json:"items"`
	Total int `json:"total"`
}

type sdPayoutSummaryDTO struct {
	Items []struct {
		AssigneeID string      `json:"assignee_id"`
		PaidCount  int         `json:"paid_count"`
		Paid       json.Number `json:"paid"`
	} `json:"items"`
	Total int `json:"total"`
}

type sdHistoryDTO struct {
	Items []struct {
		TaskID int64 `json:"task_id"`
	} `json:"items"`
	Total int `json:"total"`
}

type sdUsersDTO struct {
	Items []struct {
		ID          string `json:"id"`
		ActiveTasks *int   `json:"active_tasks"`
	} `json:"items"`
}

type sdColumnsDTO struct {
	Items []struct {
		ID         string `json:"id"`
		TasksCount int    `json:"tasks_count"`
	} `json:"items"`
}

// sdReaders — как заказ виден читателям (true / число — «виден»).
type sdReaders struct {
	board, search, history bool
	pending                string
	payouts, paidCount     int
	paid                   string
	activeTasks            int
	columnCount            int
	otherCardHistory       int
}

// sdOnBoard — заказ taskID есть в выдаче доски GET /api/v1/tasks с фильтром query.
func (s *OrderCardLiveSuite) sdOnBoard(query string, taskID int64) bool {
	var l struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	s.sdGet("/api/v1/tasks?assignee=all&limit=1000&"+query, &l)
	for _, it := range l.Items {
		if it.ID == taskID {
			return true
		}
	}
	return false
}

func (s *OrderCardLiveSuite) sdGet(path string, out any) {
	st, raw := s.admin.Do(http.MethodGet, path, nil)
	s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
	s.Require().NoError(json.Unmarshal(raw, out), string(raw))
}

// TS-006 (DS-010, DS-029): удалённый заказ не виден читателям — доска и поиск, «Остатки»
// (потребность «в заказах»), «Выплаты» и их итоги (включая уже выплаченную услугу), история
// единицы (экран единицы и блок истории в карточке другого заказа), счётчик «Активных
// заказов» сотрудника, счётчик заказов колонки. После восстановления — снова виден. Негатив:
// GET заказа, карточки и печатной формы удалённого заказа → 404.
func (s *OrderCardLiveSuite) TestTS006_Readers() {
	ctx := context.Background()
	c := s.newClient("Клиент-Читатели")
	col := s.sdColumn("SD-Читатели-" + uuid.NewString()[:8])
	_, execID := s.sdExecutor()
	title := "SD-Заказ-" + uuid.NewString()[:8]

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{
		"title": title, "client_id": c, "column_id": col, "assignee_id": execID,
	}))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var created struct {
		Task struct {
			ID int64 `json:"id"`
		} `json:"task"`
	}
	s.Require().NoError(json.Unmarshal(raw, &created), string(raw))
	taskID := created.Task.ID

	// услуга с исполнителем, выполнена и выплачена (фикстура выплаты); товар услуги — потребность.
	subtaskID := s.addSubtask(taskID, s.newService("1000"), "1000")
	productName := "SD-Товар-" + uuid.NewString()[:8]
	var productID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO products (name, unit, price, stock, min_stock) VALUES ($1, 'шт', 50, 10, 0) RETURNING id`, productName).Scan(&productID))
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/products", map[string]any{"product_id": productID, "qty": 3})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	_, err := s.Pool.Exec(ctx, `
		UPDATE subtasks SET assignee_id = $2, executor_payout = 500, is_executor_paid = true, executor_paid_at = now()
		WHERE id = $1`, subtaskID, execID)
	s.Require().NoError(err)

	// автомобиль заказа (one-car-order) и другой (действующий) заказ того же автомобиля — «История
	// работ» в его карточке (GET /equipment/:id/history без текущего заказа).
	eq := s.taskCar(taskID)
	otherID := s.newTaskWithCar(eq, s.sdScalar(`SELECT plate_number FROM equipment WHERE id = $1`, eq), s.columnID("Приёмка"))

	read := func() sdReaders {
		var r sdReaders
		r.board = s.sdOnBoard("client_id="+c, taskID)
		r.search = s.sdOnBoard("q="+title, taskID)

		var stock sdStockListDTO
		s.sdGet("/api/v1/stock?scope=all&q="+productName, &stock)
		s.Require().Len(stock.Items, 1, "товар в «Остатках»")
		pending, err := stock.Items[0].Pending.Float64()
		s.Require().NoError(err)
		r.pending = fmt.Sprintf("%.3f", pending)

		var payouts sdPayoutListDTO
		s.sdGet("/api/v1/executor-payouts?assignee="+execID, &payouts)
		r.payouts = payouts.Total
		var summary sdPayoutSummaryDTO
		s.sdGet("/api/v1/executor-payouts/summary?assignee="+execID, &summary)
		r.paid = "0.00"
		if len(summary.Items) > 0 {
			r.paidCount, r.paid = summary.Items[0].PaidCount, summary.Items[0].Paid.String()
		}

		var hist sdHistoryDTO
		s.sdGet("/api/v1/equipment/"+eq+"/history", &hist)
		for _, it := range hist.Items {
			if it.TaskID == taskID {
				r.history = true
			}
		}

		var users sdUsersDTO
		s.sdGet("/api/v1/users?limit=200", &users)
		for _, u := range users.Items {
			if u.ID == execID && u.ActiveTasks != nil {
				r.activeTasks = *u.ActiveTasks
			}
		}

		var cols sdColumnsDTO
		s.sdGet("/api/v1/columns", &cols)
		for _, cl := range cols.Items {
			if cl.ID == col {
				r.columnCount = cl.TasksCount
			}
		}

		for _, it := range hist.Items {
			if it.TaskID != otherID {
				r.otherCardHistory++
			}
		}
		return r
	}

	visible := sdReaders{board: true, search: true, history: true, pending: "3.000", payouts: 1, paidCount: 1, paid: "500.00",
		activeTasks: 1, columnCount: 1, otherCardHistory: 1}
	s.Equal(visible, read(), "до удаления заказ виден всем читателям")

	s.mustSoftDelete(taskID)
	s.Equal(sdReaders{pending: "0.000", paid: "0.00"}, read(), "удалённый заказ не виден ни одному читателю")

	// негатив: чтение удалённого заказа по номеру — 404, как несуществующего; исполнителю тоже.
	exec, _ := s.sdExecutor()
	for _, b := range []*testutil.Browser{s.admin, exec} {
		for _, path := range []string{taskPath(taskID, ""), taskPath(taskID, "/card")} {
			st, raw := b.Do(http.MethodGet, path, nil)
			s.Equal(http.StatusNotFound, st, "%s: %s", path, raw)
			s.Equal("TASK_NOT_FOUND", s.sdErr(raw).Code, path)
		}
		for _, kind := range []string{"intake", "handover"} {
			st, _ := b.Do(http.MethodGet, fmt.Sprintf("/tasks/%d/print/%s", taskID, kind), nil)
			s.Equal(http.StatusNotFound, st, "печатная форма %s удалённого заказа", kind)
		}
	}

	s.mustRestore(taskID)
	s.Equal(visible, read(), "после восстановления заказ снова виден")
	st, _ = s.admin.Do(http.MethodGet, fmt.Sprintf("/tasks/%d/print/intake", taskID), nil)
	s.Equal(http.StatusOK, st, "печатная форма восстановленного заказа открывается")

	// уборка своей колонки: заказ → в общую колонку, колонка убирается служебным адресом.
	s.sdMove(taskID, s.columnID("Приёмка"))
	st, raw = s.purge(s.admin, "columns", col)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- TS-007 ----

// TS-007 (DS-026): любая правка удалённого заказа и его частей → 404 своего вида; данные
// заказа не изменились. Заодно — отметка и сумма выплаты услуги удалённого заказа (§6.6).
func (s *OrderCardLiveSuite) TestTS007_MutationsOfDeletedTask() {
	ctx := context.Background()
	c := s.newClient("Клиент-Мутации-Удалённого")
	newCol, wip := s.columnID("Приёмка"), s.columnID("В ремонте")
	taskID, subtaskID, productID := s.sdTaskWithProduct(c, newCol, "10", 2)
	_, execID := s.sdExecutor()
	_, err := s.Pool.Exec(ctx, `UPDATE subtasks SET assignee_id = $2, status = 'done' WHERE id = $1`, subtaskID, execID)
	s.Require().NoError(err)
	s.addLog(subtaskID, "Запись до удаления")
	productRowID := s.sdScalar(`SELECT id::text FROM subtask_products WHERE subtask_id = $1`, subtaskID)
	logID := s.sdScalar(`SELECT id::text FROM subtask_logs WHERE subtask_id = $1`, subtaskID)
	svc := s.newService("300")

	snapshot := func() string {
		return s.sdScalar(`
			SELECT t.title || '|' || t.column_id::text || '|' ||
			       (SELECT count(*) FROM subtasks WHERE task_id = t.id)::text || '|' ||
			       (SELECT string_agg(status || ':' || is_executor_paid::text || ':' || coalesce(executor_payout::text, '-'), ',') FROM subtasks WHERE task_id = t.id) || '|' ||
			       (SELECT string_agg(qty::text, ',') FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = t.id) || '|' ||
			       (SELECT string_agg(l.text, ',') FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id WHERE s.task_id = t.id) || '|' ||
			       t.car_id::text
			FROM tasks t WHERE t.id = $1`, taskID)
	}
	before := snapshot()
	s.mustSoftDelete(taskID)
	version := s.sdScalar(`SELECT version::text FROM tasks WHERE id = $1`, taskID)

	for _, tc := range []struct {
		name, method, path string
		body               map[string]any
		code               string
	}{
		{"правка заказа", http.MethodPut, taskPath(taskID, ""), withPlan(map[string]any{"title": "Новое название", "column_id": newCol, "version": json.Number(version)}), "TASK_NOT_FOUND"},
		{"перенос заказа", http.MethodPost, taskPath(taskID, "/move"), map[string]any{"column_id": wip}, "TASK_NOT_FOUND"},
		{"добавление услуги", http.MethodPost, taskPath(taskID, "/subtasks"), map[string]any{"service_id": svc}, "TASK_NOT_FOUND"},
		{"правка услуги", http.MethodPut, "/api/v1/subtasks/" + subtaskID, map[string]any{"status": "in_progress"}, "SUBTASK_NOT_FOUND"},
		{"удаление услуги", http.MethodDelete, "/api/v1/subtasks/" + subtaskID, nil, "SUBTASK_NOT_FOUND"},
		{"добавление товара", http.MethodPost, "/api/v1/subtasks/" + subtaskID + "/products", map[string]any{"product_id": productID, "qty": 1}, "SUBTASK_NOT_FOUND"},
		{"правка товара", http.MethodPut, "/api/v1/subtask-products/" + productRowID, map[string]any{"qty": 5, "price": 50}, "SUBTASK_PRODUCT_NOT_FOUND"},
		{"удаление товара", http.MethodDelete, "/api/v1/subtask-products/" + productRowID, nil, "SUBTASK_PRODUCT_NOT_FOUND"},
		{"добавление записи", http.MethodPost, "/api/v1/subtasks/" + subtaskID + "/logs", map[string]any{"text": "Запись после удаления"}, "SUBTASK_NOT_FOUND"},
		{"правка записи", http.MethodPut, "/api/v1/subtask-logs/" + logID, map[string]any{"text": "Исправлено"}, "SUBTASK_LOG_NOT_FOUND"},
		{"удаление записи", http.MethodDelete, "/api/v1/subtask-logs/" + logID, nil, "SUBTASK_LOG_NOT_FOUND"},
		{"сумма выплаты", http.MethodPut, "/api/v1/executor-payouts/" + subtaskID, map[string]any{"executor_payout": 700}, "EXECUTOR_PAYOUT_NOT_FOUND"},
	} {
		st, raw := s.admin.Do(tc.method, tc.path, tc.body)
		s.Equal(http.StatusNotFound, st, "%s: %s", tc.name, raw)
		s.Equal(tc.code, s.sdErr(raw).Code, "%s: %s", tc.name, raw)
	}

	// отметка выплаты — массовая операция: услуга удалённого заказа «не найдена» по индексу.
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/executor-payouts/mark", map[string]any{"subtask_ids": []string{subtaskID}, "is_executor_paid": true})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.sdField(raw, "subtask_ids[0]"), string(raw))

	s.Equal(before, snapshot(), "данные удалённого заказа не изменились")
	s.Equal(version, s.sdScalar(`SELECT version::text FROM tasks WHERE id = $1`, taskID))
	s.Equal("10.000", s.defaultStock(productID))

	// после восстановления те же адреса снова работают.
	s.mustRestore(taskID)
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/logs", map[string]any{"text": "Запись после восстановления"})
	s.Equal(http.StatusCreated, st, string(raw))
}

// ---- TS-012 ----

// TS-012 (DS-022, DS-030): миграция 0018 — три поля удаления по умолчанию NULL (заказ
// действует); CHECK tasks_deleted_chk отклоняет частичное заполнение, пустую и слишком длинную
// причину; полное заполнение проходит; частичный индекс раздела «Удалённые» создан.
func (s *OrderCardLiveSuite) TestTS012_MigrationCheck() {
	ctx := context.Background()
	c := s.newClient("Клиент-Миграция-Удаления")
	taskID, _ := s.newTask(c, s.columnID("Приёмка"))
	adminID := s.sdScalar(`SELECT id::text FROM users WHERE login = 'card-admin'`)

	s.Equal("true", s.sdScalar(`
		SELECT (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)::text FROM tasks WHERE id = $1`, taskID))
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM pg_indexes WHERE tablename = 'tasks' AND indexname = 'tasks_deleted_at_idx'`))

	for name, q := range map[string]string{
		"только deleted_at":        `UPDATE tasks SET deleted_at = now() WHERE id = $1 AND $2::text <> ''`,
		"только deleted_by":        `UPDATE tasks SET deleted_by = $2::uuid WHERE id = $1`,
		"только delete_reason":     `UPDATE tasks SET delete_reason = 'x' || left($2::text, 0) WHERE id = $1`,
		"без deleted_by":           `UPDATE tasks SET deleted_at = now(), delete_reason = 'x' || left($2::text, 0) WHERE id = $1`,
		"без delete_reason":        `UPDATE tasks SET deleted_at = now(), deleted_by = $2::uuid WHERE id = $1`,
		"причина из пробелов":      `UPDATE tasks SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = '   ' WHERE id = $1`,
		"причина длиннее 500":      `UPDATE tasks SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = repeat('я', 501) WHERE id = $1`,
		"несуществующий удаливший": `UPDATE tasks SET deleted_at = now(), deleted_by = gen_random_uuid(), delete_reason = 'x' || left($2::text, 0) WHERE id = $1`,
	} {
		_, err := s.Pool.Exec(ctx, q, taskID, adminID)
		s.Require().Error(err, name)
		want := "tasks_deleted_chk"
		if name == "несуществующий удаливший" {
			want = "tasks_deleted_by_fkey"
		}
		s.Contains(err.Error(), want, name)
	}

	_, err := s.Pool.Exec(ctx, `
		UPDATE tasks SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = repeat('я', 500) WHERE id = $1`, taskID, adminID)
	s.Require().NoError(err, "полное заполнение проходит")
}

// ---- TS-013 ----

// TS-013 (DS-027, DS-036): служебная уборка DELETE /api/v1/test/purge/tasks/:id (сьюта
// запущена с TEST_PURGE_ENABLED=true) — прежнее безвозвратное удаление: заказ физически удалён
// с частями, товар возвращён на склад; работает и для мягко удалённого заказа.
// Повтор, неизвестный вид, нечисловой номер → 404 NOT_FOUND. Негатив: исполнитель → 403.
// Файлы вложений — AttachmentsLiveSuite; «без переменной адреса нет» — BoardLiveSuite.
func (s *OrderCardLiveSuite) TestTS013_Purge() {
	c := s.newClient("Клиент-Уборка")
	taskID, subtaskID, productID := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
	s.sdMove(taskID, s.columnID("В ремонте"))
	s.addLog(subtaskID, "Запись")
	eq := s.taskCar(taskID)
	id := fmt.Sprint(taskID)

	exec, _ := s.sdExecutor()
	for _, target := range []string{id, "999999999"} {
		st, raw := s.purge(exec, "tasks", target)
		s.Equal(http.StatusForbidden, st, string(raw))
		s.Equal("Нет доступа", s.sdErr(raw).Message)
	}

	s.Equal("6.000", s.defaultStock(productID))

	for _, tc := range []struct{ kind, id string }{{"suppliers", c}, {"orders", id}, {"tasks", "abc"}, {"tasks", "0"}, {"tasks", "999999999"}} {
		st, raw := s.purge(s.admin, tc.kind, tc.id)
		s.Equal(http.StatusNotFound, st, "%s/%s: %s", tc.kind, tc.id, raw)
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
		s.Equal("Не найдено", s.sdErr(raw).Message)
	}

	st, raw := s.purge(s.admin, "tasks", id)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var out struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	s.Equal(id, out.ID)
	s.Equal("0|0|0", s.sdScalar(`
		SELECT (SELECT count(*) FROM tasks WHERE id = $1)::text || '|' ||
		       (SELECT count(*) FROM subtasks WHERE task_id = $1)::text || '|' ||
		       (SELECT count(*) FROM task_stock WHERE task_id = $1)::text`, taskID),
		"заказ и его части удалены физически")
	s.Equal("10.000", s.defaultStock(productID), "товар возвращён на склад")
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM equipment WHERE id = $1`, eq), "единица остаётся в справочнике")

	st, raw = s.purge(s.admin, "tasks", id)
	s.Equal(http.StatusNotFound, st, "повтор: %s", raw)
	s.Equal("NOT_FOUND", s.sdErr(raw).Code)

	// мягко удалённый заказ убирается так же.
	softID, _, softProduct := s.sdTaskWithProduct(c, s.columnID("Приёмка"), "10", 4)
	s.sdMove(softID, s.columnID("В ремонте"))
	s.mustSoftDelete(softID)
	st, raw = s.purge(s.admin, "tasks", fmt.Sprint(softID))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM tasks WHERE id = $1`, softID))
	s.Equal("10.000", s.defaultStock(softProduct), "товар удалённого заказа уже возвращён — второй раз не возвращается")
}

// ---- TS-033 ----

// TS-033 (DS-003): адреса удаления, восстановления и уборки заказа — исполнителю 403 «Нет
// доступа» до поиска записи (и для существующего, и для несуществующего номера), запись не
// меняется. Негатив: без сессии → 401.
func (s *OrderCardLiveSuite) TestTS033_ExecutorForbidden() {
	c := s.newClient("Клиент-Права-Удаления")
	live, _ := s.newTask(c, s.columnID("Приёмка"))
	deleted, _ := s.newTask(c, s.columnID("Приёмка"))
	s.mustSoftDelete(deleted)
	exec, _ := s.sdExecutor()
	anon := s.NewBrowser()

	reason := map[string]any{"reason": "Создан по ошибке"}
	for _, tc := range []struct {
		method, path string
		body         map[string]any
	}{
		{http.MethodDelete, taskPath(live, ""), reason},
		{http.MethodDelete, taskPath(999999999, ""), reason},
		{http.MethodDelete, "/api/v1/tasks/abc", reason},
		{http.MethodDelete, taskPath(live, ""), nil}, // роль проверяется раньше причины
		{http.MethodPost, taskPath(deleted, "/restore"), nil},
		{http.MethodPost, taskPath(live, "/restore"), nil},
		{http.MethodPost, taskPath(999999999, "/restore"), nil},
		{http.MethodDelete, fmt.Sprintf("/api/v1/test/purge/tasks/%d", live), nil},
		{http.MethodDelete, "/api/v1/test/purge/clients/x", nil},
	} {
		var body any
		if tc.body != nil {
			body = tc.body
		}
		st, raw := exec.Do(tc.method, tc.path, body)
		s.Equal(http.StatusForbidden, st, "%s %s: %s", tc.method, tc.path, raw)
		s.Equal("FORBIDDEN", s.sdErr(raw).Code, tc.path)
		s.Equal("Нет доступа", s.sdErr(raw).Message, tc.path)

		st, _ = anon.Do(tc.method, tc.path, body)
		s.Equal(http.StatusUnauthorized, st, "без сессии: %s %s", tc.method, tc.path)
	}
	s.Equal("false|true", s.sdScalar(`
		SELECT (SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $1) || '|' ||
		       (SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $2)`, live, deleted), "отказы ничего не изменили")
}

// ---- TS-049 ----

// TS-049: переходное правило DS-035 снято полностью. Для клиента и единицы — частью 4
// (soft-delete-clients DS-014, сценарий заменён TS-022): мягко удалённый заказ их не удерживает,
// удаление мягкое и проходит — 200, отказов CLIENT_HAS_TASKS / EQUIPMENT_USED_IN_TASKS нет. Для
// колонки — частью 3 (TestTS019_DeleteColumn). Негатив: строки заказа, клиента, единицы и
// колонки физически на месте (клиент и единица — с отметкой удаления), а служебная уборка
// колонки, на которую ссылается удалённый заказ, — по-прежнему 409 COLUMN_NOT_EMPTY (DS-027).
func (s *OrderCardLiveSuite) TestTS049_DeletedTaskNoLongerHoldsClientEquipment() {
	c := s.newClient("Клиент-Только-Удалённый-Заказ")
	col := s.sdColumn("SD-Держит-" + uuid.NewString()[:8])
	taskID, _ := s.newTask(c, col)
	eq := s.taskCar(taskID)
	s.mustSoftDelete(taskID)

	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/test/purge/columns/"+col, nil)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("COLUMN_NOT_EMPTY", s.sdErr(raw).Code)

	// единица, затем клиент: удалённый заказ не удерживает ни ту, ни другого.
	for _, path := range []string{"/api/v1/equipment/" + eq, "/api/v1/clients/" + c} {
		st, raw = s.admin.Do(http.MethodDelete, path, map[string]any{"reason": "Больше не нужен"})
		s.Equal(http.StatusOK, st, "%s: %s", path, raw)
	}
	s.Equal("1|true|true|1", s.sdScalar(`
		SELECT (SELECT count(*) FROM tasks WHERE id = $1)::text || '|' ||
		       coalesce((SELECT (deleted_at IS NOT NULL)::text FROM clients WHERE id = $2), 'нет') || '|' ||
		       coalesce((SELECT (deleted_at IS NOT NULL)::text FROM equipment WHERE id = $3), 'нет') || '|' ||
		       (SELECT count(*) FROM columns WHERE id = $4)::text`,
		taskID, c, eq, col), "строки на месте, клиент и единица помечены удалёнными")

	// уборка: служебное удаление заказа освобождает колонку.
	st, raw = s.purge(s.admin, "tasks", fmt.Sprint(taskID))
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.purge(s.admin, "columns", col)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- TS-052 ----

// TS-052 (DS-039, DS-004): пока заказ удалён, правка его колонки заказ не трогает — включение
// consumes_stock товар по нему не списывает, а колонку можно сделать финальной, хотя у заказа
// есть незавершённая услуга. Восстановление такого заказа ставит его в конец первой
// нефинальной колонки: notices содержит column_replaced с её названием, товар списывается по
// правилу НОВОЙ колонки. Негатив: заказ без незавершённых услуг восстанавливается в свою
// финальную колонку, notices пуст.
func (s *OrderCardLiveSuite) TestTS052_RestoreWhenColumnBecameFinal() {
	ctx := context.Background()
	c := s.newClient("Клиент-Колонка-Стала-Финальной")
	colName := "SD-Финал-" + uuid.NewString()[:8]
	col := s.sdColumn(colName)

	open, _, productID := s.sdTaskWithProduct(c, col, "10", 3) // услуга new — незавершённая
	closed, closedSub, closedProduct := s.sdTaskWithProduct(c, col, "10", 2)
	_, err := s.Pool.Exec(ctx, `UPDATE subtasks SET status = 'done' WHERE id = $1`, closedSub) // фикстура: услуга выполнена
	s.Require().NoError(err)
	s.mustSoftDelete(open)
	s.mustSoftDelete(closed)

	// колонка начинает списывать товар — удалённые заказы не пересчитываются.
	st, raw := s.sdUpdateColumn(col, colName, false, true)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("10.000|10.000", s.defaultStock(productID)+"|"+s.defaultStock(closedProduct), "удалённые заказы товар не списывают")
	s.Equal("0|0", s.taskStockRows(open)+"|"+s.taskStockRows(closed))

	// колонка становится финальной: незавершённая услуга удалённого заказа не мешает.
	st, raw = s.sdUpdateColumn(col, colName, true, true)
	s.Require().Equal(http.StatusOK, st, string(raw))

	var firstID, firstName string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT id::text, name FROM columns WHERE NOT is_final AND deleted_at IS NULL ORDER BY position, id LIMIT 1`).Scan(&firstID, &firstName))
	lastPos := s.sdScalar(`SELECT coalesce(max(position), 0)::text FROM tasks WHERE column_id = $1 AND deleted_at IS NULL`, firstID)

	out := s.mustRestore(open)
	s.Require().Len(out.Notices, 1)
	s.Equal("column_replaced", out.Notices[0].Code)
	s.Equal("Колонка заказа стала финальной — заказ поставлен в колонку «"+firstName+"»", out.Notices[0].Message)
	s.Equal(firstID, s.sdScalar(`SELECT column_id::text FROM tasks WHERE id = $1`, open), "заказ в первой нефинальной колонке")
	s.Equal("true", s.sdScalar(`SELECT (position > $2::numeric)::text FROM tasks WHERE id = $1`, open, lastPos), "в конце колонки")
	consumes := s.sdScalar(`SELECT consumes_stock::text FROM columns WHERE id = $1`, firstID)
	wantStock := map[string]string{"true": "7.000", "false": "10.000"}[consumes]
	s.Equal(wantStock, s.defaultStock(productID), "товар списан по правилу новой колонки (consumes_stock=%s)", consumes)
	st, card, _ := s.getCard(s.admin, open)
	s.Require().Equal(http.StatusOK, st)
	s.Equal(firstID, card.Column.ID)

	// негатив: без незавершённых услуг заказ остаётся в своей (теперь финальной) колонке.
	out = s.mustRestore(closed)
	s.Empty(out.Notices)
	s.Equal(col, s.sdScalar(`SELECT column_id::text FROM tasks WHERE id = $1`, closed))
	s.Equal("8.000", s.defaultStock(closedProduct), "колонка списывает товар — списан снова")

	// уборка своей колонки.
	for _, id := range []int64{open, closed} {
		st, raw = s.purge(s.admin, "tasks", fmt.Sprint(id))
		s.Require().Equal(http.StatusOK, st, string(raw))
	}
	st, raw = s.purge(s.admin, "columns", col)
	s.Equal(http.StatusOK, st, string(raw))
}
