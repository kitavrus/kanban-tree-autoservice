//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление колонок доски (docs/features/soft-delete-board-columns/draft-spec.md §11,
// часть 3 мастер-спеки soft-delete) — методы OrderCardLiveSuite (§TD-8: боевой бинарь,
// реальный логин; сьюта запущена с TEST_PURGE_ENABLED=true): TS-012 (CHECK миграции columns),
// TS-019 (удаление колонки, причина, удалённая колонка для обычных адресов не существует),
// TS-020 (восстановление заказа удалённой колонки), TS-021 (восстановление колонки), TS-033
// (права), служебная уборка вида columns (§6.3, DS-027), гонки §3. «Последняя нефинальная →
// 409 LAST_NON_FINAL_COLUMN» — BoardLiveSuite.TestBoardLiveSuite_DeleteColumn_LastNonFinal.

// ---- wire-типы (1:1 с board.ColumnDeleteResponse / ColumnRestoreResponse / ColumnResponse) ----

type colDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type colRestoreDTO struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
}

type colDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Color         string `json:"color"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
	Position      int    `json:"position"`
	TasksCount    int    `json:"tasks_count"`
}

type colListDTO struct {
	Items []colDTO `json:"items"`
	Total int      `json:"total"`
}

// ---- фикстуры ----

func colPath(id, tail string) string { return "/api/v1/columns/" + id + tail }

func (s *OrderCardLiveSuite) deleteColumn(b *testutil.Browser, id string, reason any) (int, colDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, colPath(id, ""), body)
	var out colDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *OrderCardLiveSuite) mustDeleteColumn(id string) colDeleteDTO {
	st, out, raw := s.deleteColumn(s.admin, id, "Создана по ошибке")
	s.Require().Equal(http.StatusOK, st, string(raw))
	return out
}

func (s *OrderCardLiveSuite) restoreColumn(b *testutil.Browser, id string) (int, colRestoreDTO, []byte) {
	st, raw := b.Do(http.MethodPost, colPath(id, "/restore"), nil)
	var out colRestoreDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

// boardColumns — действующие колонки доски в порядке API (position, id).
func (s *OrderCardLiveSuite) boardColumns() colListDTO {
	var l colListDTO
	s.sdGet("/api/v1/columns", &l)
	return l
}

func findCol(l colListDTO, id string) *colDTO {
	for i := range l.Items {
		if l.Items[i].ID == id {
			return &l.Items[i]
		}
	}
	return nil
}

// colState — «удалена|причина|кто удалил» строки колонки («» — строки нет).
func (s *OrderCardLiveSuite) colState(id string) string {
	return s.sdScalar(`
		SELECT coalesce((SELECT (deleted_at IS NOT NULL)::text || '|' || coalesce(delete_reason, '') || '|' || coalesce(deleted_by::text, '')
		                 FROM columns WHERE id = $1), '')`, id)
}

func (s *OrderCardLiveSuite) purgeColumns(ids ...string) {
	for _, id := range ids {
		st, raw := s.purge(s.admin, "columns", id)
		s.Equal(http.StatusOK, st, "уборка колонки %s: %s", id, raw)
	}
}

func (s *OrderCardLiveSuite) purgeTasks(ids ...int64) {
	for _, id := range ids {
		st, raw := s.purge(s.admin, "tasks", fmt.Sprint(id))
		s.Equal(http.StatusOK, st, "уборка заказа %d: %s", id, raw)
	}
}

// firstNonFinalColumn — первая по позиции действующая нефинальная колонка доски.
func (s *OrderCardLiveSuite) firstNonFinalColumn() (id, name string) {
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id::text, name FROM columns WHERE NOT is_final AND deleted_at IS NULL ORDER BY position, id LIMIT 1`).Scan(&id, &name))
	return id, name
}

// ---- TS-012 ----

// TS-012 (DS-022, DS-030): миграция колонок — существующие колонки действующие (три поля
// NULL), частичный индекс на месте; CHECK «три поля вместе» отклоняет частичное заполнение,
// причину из пробелов и длиннее 500 символов. Негатив: только deleted_at без deleted_by → CHECK.
func (s *OrderCardLiveSuite) TestTS012_ColumnsMigrationCheck() {
	ctx := context.Background()
	col := s.sdColumn("SD-Миграция-" + uuid.NewString()[:8])
	defer s.purgeColumns(col)
	adminID := s.sdScalar(`SELECT id::text FROM users WHERE login = 'card-admin'`)

	s.Equal("0", s.sdScalar(`
		SELECT count(*)::text FROM columns
		WHERE name = ANY($1) AND (deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL)`,
		[]string{"Приёмка", "Диагностика", "Согласование", "Ждёт запчасти", "В ремонте", "Готов к выдаче", "Выдан"}), "колонки, существовавшие до миграции, — действующие")
	s.Equal("false||", s.colState(col))
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM pg_indexes WHERE tablename = 'columns' AND indexname = 'columns_deleted_at_idx'`))

	for name, q := range map[string]string{
		"только deleted_at":        `UPDATE columns SET deleted_at = now() WHERE id = $1 AND $2::text <> ''`,
		"только deleted_by":        `UPDATE columns SET deleted_by = $2::uuid WHERE id = $1`,
		"только delete_reason":     `UPDATE columns SET delete_reason = 'x' || left($2::text, 0) WHERE id = $1`,
		"без deleted_by":           `UPDATE columns SET deleted_at = now(), delete_reason = 'x' || left($2::text, 0) WHERE id = $1`,
		"без delete_reason":        `UPDATE columns SET deleted_at = now(), deleted_by = $2::uuid WHERE id = $1`,
		"причина из пробелов":      `UPDATE columns SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = '   ' WHERE id = $1`,
		"причина длиннее 500":      `UPDATE columns SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = repeat('я', 501) WHERE id = $1`,
		"несуществующий удаливший": `UPDATE columns SET deleted_at = now(), deleted_by = gen_random_uuid(), delete_reason = 'x' || left($2::text, 0) WHERE id = $1`,
	} {
		_, err := s.Pool.Exec(ctx, q, col, adminID)
		s.Require().Error(err, name)
		want := "columns_deleted_chk"
		if name == "несуществующий удаливший" {
			want = "columns_deleted_by_fkey"
		}
		s.Contains(err.Error(), want, name)
	}

	_, err := s.Pool.Exec(ctx, `
		UPDATE columns SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = repeat('я', 500) WHERE id = $1`, col, adminID)
	s.Require().NoError(err, "полное заполнение проходит")
}

// ---- TS-019 ----

// TS-019 (DS-019, DS-020, DS-024, DS-026, DS-028): пустая колонка удаляется мягко — 200 {id,
// deleted_at}, строка остаётся с отметкой «кто, когда, почему» (причина обрезана), из списка
// колонка уходит, позиции оставшихся идут подряд 1..N. Причина обязательна (400 required /
// too_long / invalid_text — колонка не удалена). Колонка с действующим заказом → 409
// COLUMN_NOT_EMPTY; только с удалённым заказом → 200 (переходное правило DS-035 снято).
// Удалённая колонка для обычных адресов не существует: повтор удаления и правка → 404.
// Негатив: создание заказа в удалённой колонке и перенос в неё → 404 COLUMN_NOT_FOUND.
func (s *OrderCardLiveSuite) TestTS019_DeleteColumn() {
	c := s.newClient("Клиент-Удаление-Колонки")
	mark := uuid.NewString()[:8]
	empty := s.sdColumn("SD-Пустая-" + mark)
	busy := s.sdColumn("SD-Занятая-" + mark)
	after := s.sdColumn("SD-После-" + mark) // стоит за удаляемыми: её позиция должна уплотниться
	adminID := s.sdScalar(`SELECT id::text FROM users WHERE login = 'card-admin'`)
	taskID, _ := s.newTask(c, busy)
	other, _ := s.newTask(c, s.columnID("Приёмка"))
	defer func() {
		s.purgeTasks(taskID, other)
		s.purgeColumns(empty, busy, after)
	}()

	// причина обязательна; при отказе колонка не удалена.
	for name, tc := range map[string]struct {
		reason any
		want   string
	}{
		"без причины":   {nil, "required"},
		"пустая":        {"", "required"},
		"из пробелов":   {"  \t ", "required"},
		"501 символ":    {strings.Repeat("я", 501), "too_long"},
		"NUL в причине": {"плохо\x00", "invalid_text"},
	} {
		st, _, raw := s.deleteColumn(s.admin, empty, tc.reason)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal("VALIDATION", s.sdErr(raw).Code, name)
		s.Equal(tc.want, s.sdField(raw, "reason"), name)
	}
	s.Equal("false||", s.colState(empty), "отказы колонку не удалили")

	before := s.boardColumns()
	afterPos := findCol(before, after).Position

	// пустая колонка — 200; причина хранится обрезанной.
	st, out, raw := s.deleteColumn(s.admin, empty, "  Лишний этап  ")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(empty, out.ID)
	s.WithinDuration(time.Now(), out.DeletedAt, time.Minute)
	s.Equal(time.UTC, out.DeletedAt.Location(), "deleted_at — UTC")
	s.Equal("true|Лишний этап|"+adminID, s.colState(empty))

	now := s.boardColumns()
	s.Nil(findCol(now, empty), "удалённой колонки нет в списке")
	s.Equal(before.Total-1, now.Total, "удалённая колонка не входит в total")
	for i, cl := range now.Items {
		s.Equal(i+1, cl.Position, "позиции действующих колонок — подряд 1..N (%s)", cl.Name)
	}
	s.Equal(afterPos-1, findCol(now, after).Position, "колонка за удалённой сдвинулась")

	// удалённая колонка для обычных адресов не существует.
	st, _, raw = s.deleteColumn(s.admin, empty, "Повтор")
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("COLUMN_NOT_FOUND", s.sdErr(raw).Code)
	st, raw = s.sdUpdateColumn(empty, "Правка удалённой", false, false)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("COLUMN_NOT_FOUND", s.sdErr(raw).Code)
	s.Equal("true|Лишний этап|"+adminID, s.colState(empty), "повтор и правка отметку не изменили")

	// негатив: заказ в удалённую колонку не создать и не перенести.
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{"title": "В удалённую", "client_id": c, "column_id": empty}))
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("COLUMN_NOT_FOUND", s.sdErr(raw).Code)
	st, raw = s.admin.Do(http.MethodPost, taskPath(other, "/move"), map[string]any{"column_id": empty})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("COLUMN_NOT_FOUND", s.sdErr(raw).Code)
	s.Equal("Колонка не найдена", s.sdErr(raw).Message)
	s.Equal(s.columnID("Приёмка"), s.sdScalar(`SELECT column_id::text FROM tasks WHERE id = $1`, other), "заказ остался в своей колонке")
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM tasks WHERE column_id = $1`, empty))
	// фильтр доски по удалённой колонке — пустой список.
	var list struct {
		Items []json.RawMessage `json:"items"`
		Total int               `json:"total"`
	}
	s.sdGet("/api/v1/tasks?column_id="+empty, &list)
	s.Empty(list.Items)
	s.Zero(list.Total)

	// колонка с действующим заказом — 409; заказ мягко удалён — колонку больше не удерживает.
	st, _, raw = s.deleteColumn(s.admin, busy, "Занята")
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("COLUMN_NOT_EMPTY", s.sdErr(raw).Code)
	s.Equal("В колонке есть заказы", s.sdErr(raw).Message)
	s.Equal("false||", s.colState(busy))
	s.mustSoftDelete(taskID)
	st, _, raw = s.deleteColumn(s.admin, busy, "Только удалённые заказы")
	s.Equal(http.StatusOK, st, string(raw))
	s.Equal("true|Только удалённые заказы|"+adminID, s.colState(busy))
	s.Equal(busy, s.sdScalar(`SELECT column_id::text FROM tasks WHERE id = $1`, taskID), "удалённый заказ ссылается на удалённую колонку")
}

// ---- TS-020 ----

// sdColumnWith — своя нефинальная колонка теста с заданным consumes_stock; first — поставить
// её первой на доске (position = 1), иначе в конец.
func (s *OrderCardLiveSuite) sdColumnWith(name string, consumes, first bool) string {
	body := map[string]any{"name": name, "color": "#445566", "is_final": false, "consumes_stock": consumes}
	if first {
		body["position"] = 1
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/columns", body)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Column colDTO `json:"column"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	s.Require().NotEmpty(m.Column.ID, string(raw))
	s.Require().Equal(consumes, m.Column.ConsumesStock, string(raw))
	return m.Column.ID
}

// TS-020 (DS-016): заказ, колонку которого удалили, восстанавливается в конец первой
// нефинальной действующей колонки: notices содержит column_replaced с её названием, товар
// списывается по правилу НОВОЙ колонки — оба направления: колонка заказа списывала, новая не
// списывает → товар остаётся на складе; колонка заказа не списывала, новая списывает → товар
// списан. Первая нефинальная колонка — своя (position = 1), её правило задаёт тест. Негатив:
// колонка заказа сама не восстановлена.
func (s *OrderCardLiveSuite) TestTS020_RestoreTaskOfDeletedColumn() {
	c := s.newClient("Клиент-Колонка-Удалена")
	for _, tc := range []struct {
		name                     string
		oldConsumes, newConsumes bool
		inOld, afterRestore      string // остаток товара (было 10, в заказе 3)
	}{
		{"старая списывала, новая не списывает", true, false, "7.000", "10.000"},
		{"старая не списывала, новая списывает", false, true, "10.000", "7.000"},
	} {
		s.Run(tc.name, func() {
			mark := uuid.NewString()[:8]
			frontName := "SD-Первая-" + mark
			front := s.sdColumnWith(frontName, tc.newConsumes, true)
			col := s.sdColumnWith("SD-Уйдёт-"+mark, tc.oldConsumes, false)
			taskID, _, productID := s.sdTaskWithProduct(c, col, "10", 3)
			defer func() {
				s.purgeTasks(taskID)
				s.purgeColumns(col, front)
			}()
			s.Equal(tc.inOld, s.defaultStock(productID), "до удаления — правило колонки заказа")
			s.mustSoftDelete(taskID)
			s.Equal("10.000", s.defaultStock(productID), "удалённый заказ товар не держит")
			s.mustDeleteColumn(col)

			firstID, firstName := s.firstNonFinalColumn()
			s.Require().Equal(front, firstID, "первая нефинальная действующая — колонка теста")
			s.Equal(frontName, firstName)

			out := s.mustRestore(taskID)
			s.Require().Len(out.Notices, 1)
			s.Equal("column_replaced", out.Notices[0].Code)
			s.Equal("Колонка заказа удалена — заказ поставлен в колонку «"+frontName+"»", out.Notices[0].Message)
			s.Equal(front, s.sdScalar(`SELECT column_id::text FROM tasks WHERE id = $1`, taskID), "заказ в первой нефинальной действующей колонке")
			s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM tasks WHERE column_id = $1 AND deleted_at IS NULL`, front), "заказ — единственный, в конце колонки")
			s.Equal(tc.afterRestore, s.defaultStock(productID), "товар — по правилу НОВОЙ колонки (consumes_stock=%v)", tc.newConsumes)
			wantRows := map[bool]string{true: "1", false: "0"}[tc.newConsumes]
			s.Equal(wantRows, s.taskStockRows(taskID), "списание заказа — по правилу новой колонки")
			st, card, _ := s.getCard(s.admin, taskID)
			s.Require().Equal(http.StatusOK, st)
			s.Equal(front, card.Column.ID)

			// негатив: колонка заказа осталась удалённой и на доске не появилась.
			s.True(strings.HasPrefix(s.colState(col), "true|"), "колонка заказа не восстановлена")
			s.Nil(findCol(s.boardColumns(), col))
		})
	}
}

// ---- TS-021 ----

// TS-021 (DS-019, DS-024, DS-030): восстановленная колонка встаёт в конец доски — position =
// максимум среди действующих + 1; название, цвет и признаки прежние; отметка удаления стёрта;
// удалённые заказы колонки остаются удалёнными. Негатив: повтор → 409 NOT_DELETED;
// несуществующая колонка → 404 COLUMN_NOT_FOUND.
func (s *OrderCardLiveSuite) TestTS021_RestoreColumn() {
	c := s.newClient("Клиент-Восстановление-Колонки")
	mark := uuid.NewString()[:8]
	name := "SD-Вернётся-" + mark
	col := s.sdColumn(name)
	tail := s.sdColumn("SD-Хвост-" + mark)
	taskID, _ := s.newTask(c, col)
	defer func() {
		s.purgeTasks(taskID)
		s.purgeColumns(col, tail)
	}()
	st, raw := s.sdUpdateColumn(col, name, true, true) // признаки, которые должны сохраниться
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.mustSoftDelete(taskID)
	s.mustDeleteColumn(col)

	active := s.boardColumns()
	maxPos := 0
	for _, cl := range active.Items {
		maxPos = max(maxPos, cl.Position)
	}
	s.Equal(active.Total, maxPos, "действующие колонки — подряд 1..N")
	s.Equal(maxPos, findCol(active, tail).Position, "колонка-хвост — последняя")

	st, out, raw := s.restoreColumn(s.admin, col)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(col, out.ID)
	s.Equal(maxPos+1, out.Position, "позиция — максимум среди действующих + 1")

	now := s.boardColumns()
	got := findCol(now, col)
	s.Require().NotNil(got, "колонка снова на доске")
	s.Equal(colDTO{ID: col, Name: name, Color: "#445566", IsFinal: true, ConsumesStock: true, Position: maxPos + 1, TasksCount: 0}, *got)
	s.Equal(col, now.Items[len(now.Items)-1].ID, "колонка — в конце доски")
	s.Equal("false||", s.colState(col), "отметка удаления стёрта")
	s.Equal("true", s.sdScalar(`SELECT (deleted_at IS NOT NULL)::text FROM tasks WHERE id = $1`, taskID), "удалённый заказ колонки остался удалённым")

	// негатив: повтор и несуществующие идентификаторы.
	st, _, raw = s.restoreColumn(s.admin, col)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NOT_DELETED", s.sdErr(raw).Code)
	s.Equal("Запись уже восстановлена", s.sdErr(raw).Message)
	for _, id := range []string{uuid.NewString(), "not-uuid"} {
		st, _, raw = s.restoreColumn(s.admin, id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("COLUMN_NOT_FOUND", s.sdErr(raw).Code, id)
	}
	s.Equal(maxPos+1, findCol(s.boardColumns(), col).Position, "повтор позицию не изменил")
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): удаление и восстановление колонки — только администратору:
// исполнитель → 403 «Нет доступа» до поиска записи (включая несуществующий идентификатор) и до
// проверки причины. Негатив: без сессии → 401; отказы ничего не изменили.
func (s *OrderCardLiveSuite) TestTS033_ColumnsExecutorForbidden() {
	mark := uuid.NewString()[:8]
	live := s.sdColumn("SD-Права-Действует-" + mark)
	deleted := s.sdColumn("SD-Права-Удалена-" + mark)
	defer s.purgeColumns(live, deleted)
	s.mustDeleteColumn(deleted)
	exec, _ := s.sdExecutor()
	anon := s.NewBrowser()

	reason := map[string]any{"reason": "Создана по ошибке"}
	missing := uuid.NewString()
	for _, tc := range []struct {
		method, path string
		body         map[string]any
	}{
		{http.MethodDelete, colPath(live, ""), reason},
		{http.MethodDelete, colPath(missing, ""), reason},
		{http.MethodDelete, colPath("abc", ""), reason},
		{http.MethodDelete, colPath(live, ""), nil}, // роль проверяется раньше причины
		{http.MethodPost, colPath(deleted, "/restore"), nil},
		{http.MethodPost, colPath(live, "/restore"), nil},
		{http.MethodPost, colPath(missing, "/restore"), nil},
		{http.MethodDelete, "/api/v1/test/purge/columns/" + live, nil},
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
	s.Equal("false||", s.colState(live), "отказы ничего не изменили")
	s.True(strings.HasPrefix(s.colState(deleted), "true|"), "отказы ничего не изменили")
}

// ---- служебная уборка вида columns ----

// §6.3 (DS-027): DELETE /api/v1/test/purge/columns/:id физически удаляет колонку — и
// действующую, и мягко удалённую; позиции оставшихся идут подряд. Колонка, на которую
// ссылается заказ (в том числе удалённый), → 409 COLUMN_NOT_EMPTY. Негатив: повтор и
// нечитаемый идентификатор → 404 NOT_FOUND.
func (s *OrderCardLiveSuite) TestPurge_Columns() {
	c := s.newClient("Клиент-Уборка-Колонок")
	mark := uuid.NewString()[:8]
	live := s.sdColumn("SD-Уборка-Действует-" + mark)
	deleted := s.sdColumn("SD-Уборка-Удалена-" + mark)
	held := s.sdColumn("SD-Уборка-С-Заказом-" + mark)
	taskID, _ := s.newTask(c, held)
	s.mustSoftDelete(taskID)
	s.mustDeleteColumn(held) // удалённый заказ колонку не удерживает от мягкого удаления…
	s.mustDeleteColumn(deleted)

	st, raw := s.purge(s.admin, "columns", held) // …но физически убрать её нельзя
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("COLUMN_NOT_EMPTY", s.sdErr(raw).Code)
	s.NotEmpty(s.colState(held), "строка колонки на месте")

	for _, id := range []string{live, deleted} {
		st, raw = s.purge(s.admin, "columns", id)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var out struct {
			ID string `json:"id"`
		}
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		s.Equal(id, out.ID)
		s.Empty(s.colState(id), "строка колонки физически удалена")
	}
	for i, cl := range s.boardColumns().Items {
		s.Equal(i+1, cl.Position, "позиции действующих колонок — подряд 1..N (%s)", cl.Name)
	}

	for _, id := range []string{live, "not-uuid"} {
		st, raw = s.purge(s.admin, "columns", id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("NOT_FOUND", s.sdErr(raw).Code, id)
	}

	s.purgeTasks(taskID)
	s.purgeColumns(held)
}

// ---- гонки (§3 «Конкурентность», abuse case 2) ----

// colLockWaiters — число серверных соединений, ждущих блокировку строки.
func (s *OrderCardLiveSuite) colLockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

type colRaceResult struct {
	status int
	raw    []byte
}

// §3 п. 2 (DS-019): создание заказа в колонке идёт (колонка под FOR SHARE, заказ ещё не
// зафиксирован) — в это окно приходит удаление колонки. Удаление обязано дождаться и увидеть
// действующий заказ: 409 COLUMN_NOT_EMPTY, колонка не удалена. Гонка воспроизводится
// детерминированно, без sleep: тест сам держит блокировку и ждёт по pg_stat_activity.
func (s *OrderCardLiveSuite) TestRace_ColumnDeleteWaitsForTaskCreate() {
	ctx := context.Background()
	c := s.newClient("Клиент-Гонка-Создание")
	col := s.sdColumn("SD-Гонка-Создание-" + uuid.NewString()[:8])
	var taskID int64
	defer func() {
		if taskID != 0 {
			s.purgeTasks(taskID)
		}
		s.purgeColumns(col)
	}()

	// «Создание заказа в полёте»: те же блокировка и INSERT, что делает TaskService.Create.
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT id FROM columns WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, col)
	s.Require().NoError(err)
	s.Require().NoError(tx.QueryRow(ctx, `
		WITH car AS (INSERT INTO equipment (client_id, plate_number) VALUES ($2::uuid, 'ГН 001') RETURNING id)
		INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by)
		SELECT $1::uuid, $2::uuid, (SELECT id FROM car), 'Заказ гонки', 1, id FROM users WHERE login = 'card-admin' RETURNING id`, col, c).Scan(&taskID))

	base := s.colLockWaiters()
	done := make(chan colRaceResult, 1)
	go func() {
		st, raw := s.admin.Do(http.MethodDelete, colPath(col, ""), map[string]any{"reason": "Гонка"})
		done <- colRaceResult{st, raw}
	}()
	s.Require().Eventually(func() bool { return s.colLockWaiters() > base }, 10*time.Second, 20*time.Millisecond,
		"удаление колонки ждёт блокировку")
	s.Require().NoError(tx.Commit(ctx))

	res := <-done
	s.Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("COLUMN_NOT_EMPTY", s.sdErr(res.raw).Code)
	s.Equal("false||", s.colState(col), "колонка не удалена")
}

// §3 п. 2 (DS-019, DS-026): удаление колонки идёт (все колонки под FOR UPDATE, отметка ещё не
// зафиксирована) — в это окно приходят создание заказа в этой колонке и перенос в неё. Оба
// обязаны дождаться удаления и получить 404 COLUMN_NOT_FOUND: действующего заказа в удалённой
// колонке не появляется.
func (s *OrderCardLiveSuite) TestRace_TaskWritesWaitForColumnDelete() {
	ctx := context.Background()
	c := s.newClient("Клиент-Гонка-Удаление")
	col := s.sdColumn("SD-Гонка-Удаление-" + uuid.NewString()[:8])
	other, _ := s.newTask(c, s.columnID("Приёмка"))
	defer func() {
		s.purgeTasks(other)
		s.purgeColumns(col)
	}()

	// «Удаление в полёте»: те же блокировка и UPDATE, что делает ColumnService.Delete.
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT id FROM columns ORDER BY id FOR UPDATE`)
	s.Require().NoError(err)
	_, err = tx.Exec(ctx, `
		UPDATE columns SET deleted_at = now(), delete_reason = 'гонка',
		       deleted_by = (SELECT id FROM users WHERE login = 'card-admin')
		WHERE id = $1`, col)
	s.Require().NoError(err)

	writer := s.NewBrowser() // второй клиент: запросы идут параллельно
	writer.Login("card-admin", "password1")
	requests := []func() (int, []byte){
		func() (int, []byte) {
			return s.admin.Do(http.MethodPost, "/api/v1/tasks", withNewCar(map[string]any{"title": "В удаляемую", "client_id": c, "column_id": col}))
		},
		func() (int, []byte) {
			return writer.Do(http.MethodPost, taskPath(other, "/move"), map[string]any{"column_id": col})
		},
	}
	base := s.colLockWaiters()
	done := make(chan colRaceResult, len(requests))
	for _, do := range requests {
		go func() {
			st, raw := do()
			done <- colRaceResult{st, raw}
		}()
	}
	s.Require().Eventually(func() bool { return s.colLockWaiters() >= base+len(requests) }, 10*time.Second, 20*time.Millisecond,
		"создание и перенос ждут блокировку колонки")
	s.Require().NoError(tx.Commit(ctx))

	for range requests {
		res := <-done
		s.Equal(http.StatusNotFound, res.status, string(res.raw))
		s.Equal("COLUMN_NOT_FOUND", s.sdErr(res.raw).Code)
	}
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM tasks WHERE column_id = $1`, col), "заказов в удалённой колонке нет")
	s.Equal(s.columnID("Приёмка"), s.sdScalar(`SELECT column_id::text FROM tasks WHERE id = $1`, other))
}

// ---- инвариант «≥1 действующая нефинальная колонка» (DS-019, abuse case 3) ----

// onlyNonFinal — фикстура (SQL: не предмет теста): все действующие нефинальные колонки доски,
// кроме keep, помечаются удалёнными — на доске остаются действующими нефинальными только
// колонки теста. Общие колонки сьюты заняты заказами других тестов, через API их не удалить.
// Возвращает откат: отметки снимаются, позиции всех колонок возвращаются к исходным (удаление
// внутри теста уплотняет позиции действующих).
func (s *OrderCardLiveSuite) onlyNonFinal(keep ...string) func() {
	ctx := context.Background()
	positions := map[string]int{}
	rows, err := s.Pool.Query(ctx, `SELECT id::text, position FROM columns`)
	s.Require().NoError(err)
	for rows.Next() {
		var id string
		var pos int
		s.Require().NoError(rows.Scan(&id, &pos))
		positions[id] = pos
	}
	rows.Close()
	s.Require().NoError(rows.Err())

	var hidden []string
	rows, err = s.Pool.Query(ctx, `
		UPDATE columns SET deleted_at = now(), delete_reason = 'фикстура инварианта',
		       deleted_by = (SELECT id FROM users WHERE login = 'card-admin')
		WHERE NOT is_final AND deleted_at IS NULL AND NOT (id = ANY($1::uuid[]))
		RETURNING id::text`, keep)
	s.Require().NoError(err)
	for rows.Next() {
		var id string
		s.Require().NoError(rows.Scan(&id))
		hidden = append(hidden, id)
	}
	rows.Close()
	s.Require().NoError(rows.Err())

	return func() {
		_, err := s.Pool.Exec(ctx, `
			UPDATE columns SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = ANY($1::uuid[])`, hidden)
		s.NoError(err)
		for id, pos := range positions {
			_, err := s.Pool.Exec(ctx, `UPDATE columns SET position = $2 WHERE id = $1`, id, pos)
			s.NoError(err)
		}
	}
}

func (s *OrderCardLiveSuite) activeNonFinal() string {
	return s.sdScalar(`SELECT count(*)::text FROM columns WHERE NOT is_final AND deleted_at IS NULL`)
}

// TS-019 (DS-019): мягко удалённые колонки в инварианте не считаются. Остальные нефинальные
// колонки мягко удалены → удаление последней действующей нефинальной → 409
// LAST_NON_FINAL_COLUMN; PUT is_final=true той же колонке → 409 LAST_NON_FINAL_COLUMN. Негатив:
// колонка не удалена и осталась нефинальной.
func (s *OrderCardLiveSuite) TestTS019_LastNonFinal_OthersSoftDeleted() {
	mark := uuid.NewString()[:8]
	gone := s.sdColumn("SD-Инвариант-Уйдёт-" + mark)
	lastName := "SD-Инвариант-Последняя-" + mark
	last := s.sdColumn(lastName)
	defer s.purgeColumns(gone, last)
	defer s.onlyNonFinal(gone, last)() // откат фикстуры — до уборки колонок теста

	s.Equal("2", s.activeNonFinal())
	s.mustDeleteColumn(gone) // предпоследняя нефинальная удаляется через API
	s.Equal("1", s.activeNonFinal())

	st, _, raw := s.deleteColumn(s.admin, last, "Последняя нефинальная")
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("LAST_NON_FINAL_COLUMN", s.sdErr(raw).Code)
	s.Equal("Нужна хотя бы одна нефинальная колонка", s.sdErr(raw).Message)
	s.Equal("false||", s.colState(last), "колонка не удалена")

	st, raw = s.sdUpdateColumn(last, lastName, true, false)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("LAST_NON_FINAL_COLUMN", s.sdErr(raw).Code)
	s.Equal("false", s.sdScalar(`SELECT is_final::text FROM columns WHERE id = $1`, last), "колонка осталась нефинальной")
	s.Equal("1", s.activeNonFinal())

	// восстановленная колонка снова считается: последнюю теперь можно сделать финальной.
	st, _, raw = s.restoreColumn(s.admin, gone)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.sdUpdateColumn(last, lastName, true, false)
	s.Equal(http.StatusOK, st, string(raw))
}

// §3 п. 1, abuse case 3 (DS-019): две последние действующие нефинальные колонки удаляются
// одновременно — ровно одно удаление проходит, второе получает 409 LAST_NON_FINAL_COLUMN: на
// доске остаётся действующая нефинальная колонка. Гонка детерминирована, без sleep: тест держит
// блокировку всех колонок, по pg_stat_activity дожидается, что оба удаления встали в ожидание,
// и только потом отпускает.
func (s *OrderCardLiveSuite) TestRace_TwoLastNonFinalDeletes() {
	ctx := context.Background()
	mark := uuid.NewString()[:8]
	a := s.sdColumn("SD-Гонка-Последние-А-" + mark)
	b := s.sdColumn("SD-Гонка-Последние-Б-" + mark)
	defer s.purgeColumns(a, b)
	defer s.onlyNonFinal(a, b)() // откат фикстуры — до уборки колонок теста
	s.Require().Equal("2", s.activeNonFinal())

	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT id FROM columns ORDER BY id FOR UPDATE`)
	s.Require().NoError(err)

	second := s.NewBrowser() // второй клиент: запросы идут параллельно
	second.Login("card-admin", "password1")
	base := s.colLockWaiters()
	done := make(chan colRaceResult, 2)
	for _, req := range []struct {
		b  *testutil.Browser
		id string
	}{{s.admin, a}, {second, b}} {
		go func() {
			st, raw := req.b.Do(http.MethodDelete, colPath(req.id, ""), map[string]any{"reason": "Гонка последних"})
			done <- colRaceResult{st, raw}
		}()
	}
	s.Require().Eventually(func() bool { return s.colLockWaiters() >= base+2 }, 10*time.Second, 20*time.Millisecond,
		"оба удаления ждут блокировку колонок")
	s.Require().NoError(tx.Commit(ctx))

	ok, refused := 0, 0
	for range 2 {
		res := <-done
		switch res.status {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			refused++
			s.Equal("LAST_NON_FINAL_COLUMN", s.sdErr(res.raw).Code, string(res.raw))
		default:
			s.Failf("неожиданный ответ", "%d: %s", res.status, res.raw)
		}
	}
	s.Equal(1, ok, "ровно одно удаление проходит")
	s.Equal(1, refused, "второе — 409")
	s.Equal("1", s.activeNonFinal(), "на доске осталась действующая нефинальная колонка")
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM columns WHERE id = ANY($1::uuid[]) AND deleted_at IS NOT NULL`, []string{a, b}))
}
