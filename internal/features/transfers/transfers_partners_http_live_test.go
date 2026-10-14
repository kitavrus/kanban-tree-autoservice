//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Направления документа «Перемещение» — клиент и поставщик
// (docs/features/transfers-partners/draft-spec.md §11) — методы TransfersLiveSuite (§TD-8):
// TS-004 (единица из оборудования заказа, клиент — из заказа), TS-006 (хвост: приём, когда
// единица не «у клиента»), TS-007 (движения остатка), TS-008 (приёмка по направлениям),
// TS-015 (права исполнителя), TS-020 (хвост: архивный поставщик), TS-023 (фильтр «период» в
// поясе пользователя), TS-025 («в пути» на «Остатках»), DS-013 (три шага и «принять сразу»).

// ---- фикстуры ----

// newClient — клиент (SQL: фикстура, не предмет теста).
func (s *TransfersLiveSuite) newClient(name string) string {
	s.seq++
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO clients (company_name) VALUES ($1) RETURNING id::text`, fmt.Sprintf("%s %d", name, s.seq)).Scan(&id))
	return id
}

// newTask — заказ клиента в первой колонке доски (SQL: фикстура).
func (s *TransfersLiveSuite) newTask(clientID string) int64 {
	s.seq++
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		INSERT INTO tasks (column_id, client_id, title, position, created_by)
		VALUES ((SELECT id FROM columns ORDER BY position, id LIMIT 1), $1, $2, $3, $4) RETURNING id`,
		clientID, fmt.Sprintf("Заказ партнёров %d", s.seq), s.seq, s.adminID).Scan(&id))
	return id
}

// newUnitOf — единица клиента clientID: warehouseID "" — «у клиента», иначе «на складе».
func (s *TransfersLiveSuite) newUnitOf(clientID, warehouseID string) string {
	s.seq++
	ctx := context.Background()
	var id string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO equipment (client_id, type_id, brand, model, serial)
		VALUES ($1, $2, 'Bodor', $3, $4) RETURNING id::text`,
		clientID, s.typeID, fmt.Sprintf("TP-%d", s.seq), fmt.Sprintf("SN-TP-%d", s.seq)).Scan(&id))
	if warehouseID != "" {
		_, err := s.Pool.Exec(ctx, `UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = $1`, id, warehouseID)
		s.Require().NoError(err)
	}
	return id
}

// attach — единица в оборудовании заказа (SQL: фикстура; местоположение не меняется).
func (s *TransfersLiveSuite) attach(taskID int64, unitIDs ...string) {
	for _, id := range unitIDs {
		_, err := s.Pool.Exec(context.Background(), `INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2)`, taskID, id)
		s.Require().NoError(err)
	}
}

// taskWithUnits — заказ клиента фикстуры с n единицами «у клиента» в его оборудовании.
func (s *TransfersLiveSuite) taskWithUnits(n int) (int64, []string) {
	taskID := s.newTask(s.clientID)
	units := make([]string, n)
	for i := range units {
		units[i] = s.newUnitOf(s.clientID, "")
	}
	s.attach(taskID, units...)
	return taskID, units
}

func (s *TransfersLiveSuite) newSupplier(active bool) string {
	s.seq++
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO suppliers (name, is_active) VALUES ($1, $2) RETURNING id::text`,
		fmt.Sprintf("Поставщик ТП %d", s.seq), active).Scan(&id))
	return id
}

// executor — исполнитель (общий на сьюту), вошедший через реальный логин.
func (s *TransfersLiveSuite) executor() *testutil.Browser {
	if s.exec == nil {
		s.SeedUser("tp-exec", "password1", "Партнёры Исполнитель", "executor", true)
		s.exec = s.NewBrowser()
		s.exec.Login("tp-exec", "password1")
	}
	return s.exec
}

func unitItems(ids ...string) []map[string]any {
	out := make([]map[string]any, len(ids))
	for i, id := range ids {
		out[i] = unitItem(id)
	}
	return out
}

func intakeBody(taskID int64, to string, units ...string) map[string]any {
	return map[string]any{"direction": "client_to_warehouse", "to_warehouse_id": to, "task_id": taskID, "items": unitItems(units...)}
}

func returnBody(taskID int64, from string, units ...string) map[string]any {
	return map[string]any{"direction": "warehouse_to_client", "from_warehouse_id": from, "task_id": taskID, "items": unitItems(units...)}
}

func (s *TransfersLiveSuite) supplyBody(supplierID, productID string, qty any) map[string]any {
	return map[string]any{
		"direction": "supplier_to_warehouse", "to_warehouse_id": s.almaty, "supplier_id": supplierID,
		"items": []map[string]any{productItem(productID, qty)},
	}
}

func with(body map[string]any, kv ...any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// doAs — запрос от имени b; успешный ответ разбирается как документ.
func (s *TransfersLiveSuite) doAs(b *testutil.Browser, method, path string, body any) (int, transferDTO, []byte) {
	st, raw := b.Do(method, path, body)
	var t transferDTO
	if st < 300 {
		s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	}
	return st, t, raw
}

func (s *TransfersLiveSuite) get(id int64) transferDTO {
	st, t, raw := s.do(http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", id), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	return t
}

// listIDs — id документов списка от имени b с фильтром query.
func (s *TransfersLiveSuite) listIDs(b *testutil.Browser, query string) (map[int64]bool, listDTO) {
	st, raw := b.Do(http.MethodGet, "/api/v1/transfers?limit=500"+query, nil)
	s.Require().Equal(http.StatusOK, st, "%s → %s", query, raw)
	var l listDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	ids := map[int64]bool{}
	for _, it := range l.Items {
		ids[it.ID] = true
	}
	return ids, l
}

// productMovementsSum — сумма движений остатка пары товар × склад по всем причинам.
func (s *TransfersLiveSuite) productMovementsSum(productID, warehouseID string) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT coalesce(sum(qty_delta), 0)::numeric(12,3)::text FROM stock_movements WHERE product_id = $1 AND warehouse_id = $2`,
		productID, warehouseID).Scan(&v))
	return v
}

// ---- DS-013: три шага документов с клиентом, «принять сразу» ----

// Приём от клиента: заявка → отправка («в пути», единица занята документом) → приёмка
// (единица на складе назначения, свободна); возврат клиенту: со склада → «в пути» → «у
// клиента». Движений остатка документы с клиентом не создают (TS-007). Ответ несёт
// направление, заказ, клиента заказа; склад «чужой» точки маршрута — null.
func (s *TransfersLiveSuite) TestClientDocuments_ThreeSteps() {
	taskID, units := s.taskWithUnits(2)
	u1, u2 := units[0], units[1]

	st, t, raw := s.do(http.MethodPost, "/api/v1/transfers", with(intakeBody(taskID, s.moscow, u1, u2), "note", " в ремонт "))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("client_to_warehouse", t.Direction)
	s.Equal("requested", t.Status)
	s.Equal("в ремонт", t.Note)
	s.Require().NotNil(t.TaskID)
	s.Equal(taskID, *t.TaskID)
	s.NotEmpty(t.TaskTitle)
	s.Equal(s.clientID, t.ClientID)
	s.Equal("ТОО Перемещаемый", t.ClientName)
	s.Equal(s.moscow, t.ToWarehouseID)
	s.Equal("Москва", t.ToWarehouseName)
	s.Contains(string(raw), `"from_warehouse_id":null`)
	s.Contains(string(raw), `"from_warehouse_name":null`)
	s.Contains(string(raw), `"supplier_id":null`)
	s.Contains(string(raw), `"supplier_name":null`)
	s.Len(t.Items, 2)
	s.Equal(fmt.Sprintf("at_client||%d", t.ID), s.unitLoc(u1), "заявка занимает единицу, местоположение прежнее")

	st, sent, raw := s.action(t, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("in_transit", sent.Status)
	s.NotNil(sent.SentAt)
	s.Equal(fmt.Sprintf("in_transit||%d", t.ID), s.unitLoc(u1))
	s.Equal(fmt.Sprintf("in_transit||%d", t.ID), s.unitLoc(u2))

	st, got, raw := s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", got.Status)
	s.NotNil(got.ReceivedAt)
	s.Equal("at_warehouse|Москва|", s.unitLoc(u1))
	s.Equal("at_warehouse|Москва|", s.unitLoc(u2))
	s.Nil(got.Items[0].QtyReceived, "у документа с клиентом факта нет")
	s.Empty(s.movements(t.ID), "документы с клиентом остаток не двигают")

	// возврат клиенту одной единицы: «Принять» нажимает наш сотрудник.
	st, back, raw := s.do(http.MethodPost, "/api/v1/transfers", with(returnBody(taskID, s.moscow, u1), "send", true))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("warehouse_to_client", back.Direction)
	s.Equal("in_transit", back.Status)
	s.Equal(s.moscow, back.FromWarehouseID)
	s.Equal("Москва", back.FromWarehouseName)
	s.Contains(string(raw), `"to_warehouse_id":null`)
	s.Equal(s.clientID, back.ClientID)
	s.Equal(fmt.Sprintf("in_transit||%d", back.ID), s.unitLoc(u1))
	st, done, raw := s.action(back, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", done.Status)
	s.Equal("at_client||", s.unitLoc(u1), "клиент получил: единица у клиента, склад пуст")
	s.Equal("at_warehouse|Москва|", s.unitLoc(u2), "вторая единица заказа — на складе")
	s.Empty(s.movements(back.ID))

	// PUT: правка заявки с клиентом — склад и строки; направление и заказ — как были.
	st, req, raw := s.do(http.MethodPost, "/api/v1/transfers", returnBody(taskID, s.moscow, u2))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, upd, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", req.ID), map[string]any{
		"from_warehouse_id": s.moscow, "note": "после ремонта", "items": unitItems(u2), "version": req.Version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("warehouse_to_client", upd.Direction)
	s.Equal(taskID, *upd.TaskID)
	s.Equal("после ремонта", upd.Note)
}

// Отмена документов с клиентом: заявка — единица свободна; «в пути» — единица возвращается в
// источник: приём → «у клиента», возврат → склад отправления.
func (s *TransfersLiveSuite) TestClientDocuments_Cancel() {
	taskID, units := s.taskWithUnits(1)
	u := units[0]

	req := s.create(intakeBody(taskID, s.almaty, u))
	st, _, raw := s.action(req, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("at_client||", s.unitLoc(u))

	tr := s.create(with(intakeBody(taskID, s.almaty, u), "send", true))
	s.Equal(fmt.Sprintf("in_transit||%d", tr.ID), s.unitLoc(u))
	st, cancelled, raw := s.action(tr, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", cancelled.Status)
	s.Equal("at_client||", s.unitLoc(u), "отмена приёма в пути: единица снова у клиента")

	s.create(with(intakeBody(taskID, s.almaty, u), "receive", true))
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u))
	back := s.create(with(returnBody(taskID, s.almaty, u), "send", true))
	st, _, raw = s.action(back, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("at_warehouse|Алматы|", s.unitLoc(u), "отмена возврата в пути: единица на складе отправления")
	s.Empty(s.movements(back.ID))
}

// receive: true — создать, отправить и принять одной транзакцией (клиент принёс сам); при
// ошибке не создаётся ничего (§3 п.3).
func (s *TransfersLiveSuite) TestClientToWarehouse_ReceiveNow() {
	taskID, units := s.taskWithUnits(2)
	st, t, raw := s.do(http.MethodPost, "/api/v1/transfers", with(intakeBody(taskID, s.almaty, units...), "receive", true))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("received", t.Status)
	s.NotNil(t.SentAt)
	s.NotNil(t.ReceivedAt)
	s.Equal(3, t.Version, "создание + отправка + приёмка")
	for _, u := range units {
		s.Equal("at_warehouse|Алматы|", s.unitLoc(u))
	}

	// send + receive вместе — то же самое.
	task2, units2 := s.taskWithUnits(1)
	t2 := s.create(with(intakeBody(task2, s.moscow, units2...), "send", true, "receive", true))
	s.Equal("received", t2.Status)

	// одна из единиц уже занята другим документом → 409, документ не создан, вторая не занята.
	task3, units3 := s.taskWithUnits(2)
	busy := s.create(intakeBody(task3, s.almaty, units3[0]))
	before := s.transfersCount()
	st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", with(intakeBody(task3, s.almaty, units3...), "receive", true))
	s.Equal(http.StatusConflict, st, string(raw))
	e := s.errOf(raw)
	s.Equal("EQUIPMENT_IN_TRANSFER", e.Code)
	s.Equal(busy.ID, e.Details.TransferID)
	s.Equal(before, s.transfersCount())
	s.Equal("at_client||", s.unitLoc(units3[1]))
}

// ---- TS-004 ----

// TS-004: единица документа с клиентом обязана входить в оборудование заказа документа и
// принадлежать его клиенту — иначе 400 items[i].equipment_id: not_in_task; client_id в теле
// игнорируется — клиент берётся из заказа (DS-012, abuse cases 3–4).
func (s *TransfersLiveSuite) TestTS004_EquipmentNotInTask() {
	taskID, units := s.taskWithUnits(1)
	inTask := units[0]
	loose := s.newUnitOf(s.clientID, "")           // того же клиента, но не в заказе
	otherTask, otherUnits := s.taskWithUnits(1)    // единица другого заказа того же клиента
	stranger := s.newClient("ТОО Чужой клиент ТП") // другой клиент
	foreign := s.newUnitOf(stranger, "")           // его единица, ни в одном заказе
	smuggled := s.newUnitOf(stranger, "")          // чужая единица, подложенная в заказ мимо API
	s.attach(taskID, smuggled)
	before := s.transfersCount()

	for name, tc := range map[string]struct {
		body map[string]any
		want map[string]string
	}{
		"не в заказе":                  {intakeBody(taskID, s.almaty, loose), map[string]string{"items[0].equipment_id": "not_in_task"}},
		"из другого заказа":            {intakeBody(taskID, s.almaty, inTask, otherUnits[0]), map[string]string{"items[1].equipment_id": "not_in_task"}},
		"другого клиента":              {intakeBody(taskID, s.almaty, foreign), map[string]string{"items[0].equipment_id": "not_in_task"}},
		"другого клиента, но в заказе": {intakeBody(taskID, s.almaty, smuggled), map[string]string{"items[0].equipment_id": "not_in_task"}},
		"возврат: не в заказе":         {returnBody(taskID, s.almaty, loose), map[string]string{"items[0].equipment_id": "not_in_task"}},
		"единицы нет вовсе": {intakeBody(taskID, s.almaty, "00000000-0000-0000-0000-000000000000"),
			map[string]string{"items[0].equipment_id": "not_found"}},
		"заказа нет":                {intakeBody(999999999, s.almaty, inTask), map[string]string{"task_id": "not_found"}},
		"заказ ≤ 0":                 {intakeBody(0, s.almaty, inTask), map[string]string{"task_id": "invalid"}},
		"без заказа":                {map[string]any{"direction": "client_to_warehouse", "to_warehouse_id": s.almaty, "items": unitItems(inTask)}, map[string]string{"task_id": "required"}},
		"товар в приёме от клиента": {with(intakeBody(taskID, s.almaty), "items", []map[string]any{productItem(s.newProduct("1"), 1)}), map[string]string{"items[0].kind": "not_allowed"}},
		"лишний склад отправления":  {with(intakeBody(taskID, s.almaty, inTask), "from_warehouse_id", s.moscow), map[string]string{"from_warehouse_id": "not_allowed"}},
		"поставщик в приёме":        {with(intakeBody(taskID, s.almaty, inTask), "supplier_id", s.newSupplier(true)), map[string]string{"supplier_id": "not_allowed"}},
	} {
		for _, send := range []bool{false, true} {
			st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", with(tc.body, "send", send))
			s.Equal(http.StatusBadRequest, st, "%s (send=%v) → %s", name, send, raw)
			s.Equal(tc.want, s.fields(raw), "%s (send=%v)", name, send)
		}
	}
	s.Equal(before, s.transfersCount(), "отклонённые запросы документ не создали")
	s.Equal("at_client||", s.unitLoc(inTask), "единица не занята")
	_ = otherTask

	// негатив: client_id в теле игнорируется — клиент документа выводится из заказа.
	st, t, raw := s.do(http.MethodPost, "/api/v1/transfers", with(intakeBody(taskID, s.almaty, inTask), "client_id", stranger))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(s.clientID, t.ClientID, "клиент — из заказа, не из тела")
	s.Equal("ТОО Перемещаемый", t.ClientName)
	var stored string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT client_id::text FROM transfers WHERE id = $1`, t.ID).Scan(&stored))
	s.Equal(s.clientID, stored)

	// PUT: та же проверка под правкой; direction и task_id не меняются (TS-001).
	put := func(body map[string]any) (int, []byte) {
		st, _, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), with(body, "version", t.Version))
		return st, raw
	}
	st, raw = put(map[string]any{"to_warehouse_id": s.almaty, "items": unitItems(loose)})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].equipment_id": "not_in_task"}, s.fields(raw))
	st, raw = put(map[string]any{"to_warehouse_id": s.almaty, "items": unitItems(inTask), "direction": "client_to_warehouse", "task_id": taskID})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"direction": "not_allowed", "task_id": "not_allowed"}, s.fields(raw))
	st, raw = put(map[string]any{"to_warehouse_id": s.almaty, "items": unitItems(inTask), "task_id": otherTask})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"task_id": "not_allowed"}, s.fields(raw))
	st, raw = put(map[string]any{"to_warehouse_id": s.almaty, "from_warehouse_id": s.moscow, "items": unitItems(inTask)})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"from_warehouse_id": "not_allowed"}, s.fields(raw))
	st, raw = put(map[string]any{"to_warehouse_id": s.moscow, "items": unitItems(inTask), "client_id": stranger})
	s.Equal(http.StatusOK, st, "client_id в PUT игнорируется: %s", raw)
	after := s.get(t.ID)
	s.Equal(s.clientID, after.ClientID)
	s.Equal(s.moscow, after.ToWarehouseID)
	s.Equal(taskID, *after.TaskID)
}

// TS-004 / TS-018 (гонка, DS-035 / DS-026): документ с клиентом создаётся параллельно со
// снятием единицы с заказа либо со сменой клиента заказа. Заказ блокируется документом
// (FOR KEY SHARE) до чтения task_equipment, поэтому исход — строго один из двух: документ
// создан, а единица осталась в заказе и клиент заказа прежний (board → 409: снятие единицы —
// EQUIPMENT_IN_TRANSFER; смена клиента — TASK_CLIENT_LOCKED_BY_TRANSFERS с номером созданного
// документа, D84: клиент заказа с документами не меняется), либо единица убрана / клиент
// сменён, а документ получил not_in_task. «Единица в незавершённом документе, но не в заказе»
// и transfers.client_id ≠ tasks.client_id недостижимы.
func (s *TransfersLiveSuite) TestTS004_ParallelCreateVsTaskEquipmentChange() {
	ctx := context.Background()
	other := s.newClient("ТОО Гонка смены клиента")
	const rounds = 20
	for _, mode := range []string{"remove", "change_client"} {
		for i := 0; i < rounds; i++ {
			taskID, units := s.taskWithUnits(1)
			unit := units[0]
			var columnID string
			var version int
			s.Require().NoError(s.Pool.QueryRow(ctx,
				`SELECT column_id::text, version FROM tasks WHERE id = $1`, taskID).Scan(&columnID, &version))

			var wg sync.WaitGroup
			var createSt, boardSt int
			var createRaw, boardRaw []byte
			wg.Add(2)
			go func() {
				defer wg.Done()
				createSt, createRaw = s.admin.Do(http.MethodPost, "/api/v1/transfers",
					with(intakeBody(taskID, s.almaty, unit), "send", i%2 == 0))
			}()
			go func() {
				defer wg.Done()
				if mode == "remove" {
					boardSt, boardRaw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d/equipment/%s", taskID, unit), nil)
					return
				}
				boardSt, boardRaw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{
					"title": "Заказ гонки", "client_id": other, "column_id": columnID,
					"assignee_id": nil, "due_date": nil, "is_paid": false, "version": version,
				})
			}()
			wg.Wait()
			at := fmt.Sprintf("%s, виток %d: create %d %s / board %d %s", mode, i, createSt, createRaw, boardSt, boardRaw)

			switch createSt {
			case http.StatusCreated:
				s.Equal(http.StatusConflict, boardSt, at)
				if mode == "remove" {
					s.Equal("EQUIPMENT_IN_TRANSFER", s.errOf(boardRaw).Code, at)
				} else {
					var locked struct {
						Code    string `json:"code"`
						Details struct {
							TransferIDs []int64 `json:"transfer_ids"`
						} `json:"details"`
					}
					s.Require().NoError(json.Unmarshal(boardRaw, &locked), at)
					s.Equal("TASK_CLIENT_LOCKED_BY_TRANSFERS", locked.Code, at)
					var created struct {
						ID int64 `json:"id"`
					}
					s.Require().NoError(json.Unmarshal(createRaw, &created), at)
					s.Equal([]int64{created.ID}, locked.Details.TransferIDs, at)
				}
			case http.StatusBadRequest:
				s.Equal(map[string]string{"items[0].equipment_id": "not_in_task"}, s.fields(createRaw), at)
				s.Equal(http.StatusOK, boardSt, at)
			default:
				s.Failf("неожиданный статус создания", "%s", at)
			}

			// инварианты — по БД, независимо от того, кто успел первым.
			var orphans, clientMismatch, docs int
			s.Require().NoError(s.Pool.QueryRow(ctx, `
				SELECT count(*) FROM equipment e
				JOIN transfers t ON t.id = e.active_transfer_id
				WHERE e.id = $1 AND NOT EXISTS (
					SELECT 1 FROM task_equipment te WHERE te.task_id = t.task_id AND te.equipment_id = e.id)`, unit).Scan(&orphans))
			s.Zero(orphans, "единица в незавершённом документе, но не в заказе: %s", at)
			s.Require().NoError(s.Pool.QueryRow(ctx, `
				SELECT count(*) FILTER (WHERE t.client_id IS DISTINCT FROM k.client_id), count(*)
				FROM transfers t JOIN tasks k ON k.id = t.task_id WHERE t.task_id = $1`, taskID).Scan(&clientMismatch, &docs))
			s.Zero(clientMismatch, "клиент документа ≠ клиенту заказа: %s", at)
			if mode == "change_client" {
				// клиент заказа сменён тогда и только тогда, когда документ не создан.
				var taskClient string
				s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT client_id::text FROM tasks WHERE id = $1`, taskID).Scan(&taskClient))
				if createSt == http.StatusCreated {
					s.Equal(s.clientID, taskClient, "документ создан — клиент заказа прежний: %s", at)
				} else {
					s.Equal(other, taskClient, "документ не создан — клиент сменён: %s", at)
				}
			}
			if createSt == http.StatusCreated {
				s.Equal(1, docs, at)
			} else {
				s.Zero(docs, at)
			}
		}
	}
}

// ---- TS-006 (хвост) ----

// TS-006: отправка «клиент → склад», когда единица не «у клиента», и «склад → клиент», когда
// она не на складе отправления, → 409 TRANSFER_EQUIPMENT_NOT_AT_SOURCE со своим текстом;
// send / receive при создании — тот же 409, документ не создан.
func (s *TransfersLiveSuite) TestTS006_ClientDocuments_NotAtSource() {
	taskID := s.newTask(s.clientID)
	onStock := s.newUnitOf(s.clientID, s.almaty) // уже на складе — принимать от клиента нечего
	atClient := s.newUnitOf(s.clientID, "")
	s.attach(taskID, onStock, atClient)

	req := s.create(intakeBody(taskID, s.moscow, onStock))
	st, _, raw := s.action(req, "send", nil)
	s.Equal(http.StatusConflict, st, string(raw))
	e := s.errOf(raw)
	s.Equal("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", e.Code)
	s.Equal("Оборудование уже не у клиента", e.Message)
	s.Equal(onStock, e.Details.EquipmentID)
	s.Equal("requested", s.get(req.ID).Status)
	s.Equal(fmt.Sprintf("at_warehouse|Алматы|%d", req.ID), s.unitLoc(onStock), "местоположение не изменилось")
	st, _, raw = s.action(req, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))

	before := s.transfersCount()
	for _, flag := range []string{"send", "receive"} {
		st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", with(intakeBody(taskID, s.moscow, atClient, onStock), flag, true))
		s.Equal(http.StatusConflict, st, "%s: %s", flag, raw)
		e = s.errOf(raw)
		s.Equal("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", e.Code, flag)
		s.Equal("Оборудование уже не у клиента", e.Message, flag)
		s.Equal(onStock, e.Details.EquipmentID, flag)
	}
	s.Equal(before, s.transfersCount(), "документ не создан")
	s.Equal("at_client||", s.unitLoc(atClient), "вторая единица не занята и не уехала")
	s.Equal("at_warehouse|Алматы|", s.unitLoc(onStock))

	// возврат клиенту: единица у клиента / на другом складе → текст «не на складе отправления».
	for name, unit := range map[string]string{"у клиента": atClient, "на другом складе": onStock} {
		st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", with(returnBody(taskID, s.moscow, unit), "send", true))
		s.Equal(http.StatusConflict, st, "%s: %s", name, raw)
		e = s.errOf(raw)
		s.Equal("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", e.Code, name)
		s.Equal("Оборудование находится не на складе отправления", e.Message, name)
		s.Equal(unit, e.Details.EquipmentID, name)
	}
	s.Equal(before, s.transfersCount())
}

// ---- TS-007 ----

// TS-007: приход от поставщика — при отправке и отмене движений нет, при приёмке supplier_in
// +факт на складе назначения; остаток product_stock сходится с суммой движений; факт 0 —
// движение не пишется, документ «Получено».
func (s *TransfersLiveSuite) TestTS007_SupplierMovements() {
	sup := s.newSupplier(true)
	p := s.newProduct("0") // остаток 0 без движений — дальше он равен сумме движений
	p0 := s.newProduct("0")

	st, t, raw := s.do(http.MethodPost, "/api/v1/transfers", s.supplyBody(sup, p, 10))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("supplier_to_warehouse", t.Direction)
	s.Equal("requested", t.Status)
	s.Equal(sup, t.SupplierID)
	s.NotEmpty(t.SupplierName)
	s.Equal(s.almaty, t.ToWarehouseID)
	s.Contains(string(raw), `"from_warehouse_id":null`)
	s.Contains(string(raw), `"task_id":null`)
	s.Contains(string(raw), `"client_id":null`)
	s.Empty(t.Warnings)

	st, sent, raw := s.action(t, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("in_transit", sent.Status)
	s.Empty(sent.Warnings, "списания с источника нет — предупреждений нет")
	s.Empty(s.movements(t.ID), "при отправке движений нет: товар ещё не наш")
	s.Equal("0.000", s.pairStock(p, s.almaty))

	st, got, raw := s.action(sent, "receive", map[string]any{"items": []map[string]any{{"id": sent.Items[0].ID, "qty_received": 12}}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", got.Status)
	s.Equal("10.000", got.Items[0].QtySent.String())
	s.Equal("12.000", got.Items[0].QtyReceived.String())
	s.ElementsMatch([]string{"supplier_in:Алматы:12.000"}, s.movements(t.ID))
	s.Equal("12.000", s.pairStock(p, s.almaty))
	s.Equal("0", s.pairStock(p, s.moscow), "на другом складе — ничего")
	s.Equal(s.pairStock(p, s.almaty), s.productMovementsSum(p, s.almaty), "остаток = сумма движений")

	// отмена заявки и отмена «в пути» — движений нет.
	req := s.create(s.supplyBody(sup, p, 5))
	st, _, raw = s.action(req, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Empty(s.movements(req.ID))
	tr := s.create(with(s.supplyBody(sup, p, 5), "send", true))
	s.Empty(s.movements(tr.ID))
	st, cancelled, raw := s.action(tr, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", cancelled.Status)
	s.Empty(s.movements(tr.ID), "отмена прихода в пути — без движений")
	s.Equal("12.000", s.pairStock(p, s.almaty))

	// негатив: факт 0 — документ «Получено», движение не пишется.
	zero := s.create(with(s.supplyBody(sup, p0, 3), "send", true))
	st, zr, raw := s.action(zero, "receive", map[string]any{"items": []map[string]any{{"id": zero.Items[0].ID, "qty_received": 0}}})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", zr.Status)
	s.Equal("0.000", zr.Items[0].QtyReceived.String())
	s.Empty(s.movements(zero.ID))
	s.Equal("0.000", s.pairStock(p0, s.almaty))
	s.Equal(s.pairStock(p0, s.almaty), s.productMovementsSum(p0, s.almaty))

	// единица в приходе от поставщика → not_allowed (TS-002, abuse case 5).
	unit := s.newUnit(s.almaty)
	st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", with(s.supplyBody(sup, p, 1), "items", unitItems(unit)))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].kind": "not_allowed"}, s.fields(raw))
}

// ---- TS-008 ----

// TS-008: приёмка по направлениям — склад → склад: факт > отправленного → 400 exceeds_sent;
// приход от поставщика: факт больше / меньше заказанного → 200, остаток +факт; items у
// документов с клиентом → 400 not_allowed; receive: true вне приёма от клиента → 400;
// отрицательный факт → 400; параллельный receive прихода — остаток вырос один раз.
func (s *TransfersLiveSuite) TestTS008_ReceiveByDirection() {
	sup := s.newSupplier(true)
	fact := func(t transferDTO, qty any) map[string]any {
		return map[string]any{"items": []map[string]any{{"id": t.Items[0].ID, "qty_received": qty}}}
	}

	// склад → склад: факт больше отправленного запрещён.
	pw := s.newProduct("10")
	w2w := s.create(with(s.body(productItem(pw, 4)), "send", true))
	st, _, raw := s.action(w2w, "receive", fact(w2w, 4.001))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].qty_received": "exceeds_sent"}, s.fields(raw))
	s.Equal("0", s.pairStock(pw, s.moscow))

	// приход от поставщика: больше заказанного.
	pm := s.newProduct("0")
	more := s.create(with(s.supplyBody(sup, pm, 10), "send", true))
	st, _, raw = s.action(more, "receive", fact(more, -1))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].qty_received": "negative"}, s.fields(raw))
	st, _, raw = s.action(more, "receive", fact(more, 1000000000))
	s.Equal(http.StatusBadRequest, st, "верхняя граница NUMERIC(12,3): %s", raw)
	s.Contains(s.fields(raw), "items[0].qty_received")
	st, _, raw = s.action(more, "receive", map[string]any{"items": []map[string]any{{"id": more.Items[0].ID, "qty_received": "12"}}})
	s.Equal(http.StatusBadRequest, st, "количество — JSON-число, строка → 400: %s", raw)
	s.Equal("0.000", s.pairStock(pm, s.almaty), "неудачная приёмка ничего не меняет")
	st, got, raw := s.action(more, "receive", fact(more, 12.5))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("12.500", got.Items[0].QtyReceived.String())
	s.Equal("12.500", s.pairStock(pm, s.almaty), "остаток +факт, а не +заказано")

	// меньше заказанного: недопоставка закрывает документ, движения «потери» нет.
	pl := s.newProduct("0")
	less := s.create(with(s.supplyBody(sup, pl, 10), "send", true))
	st, _, raw = s.action(less, "receive", fact(less, 7))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("7.000", s.pairStock(pl, s.almaty))
	s.ElementsMatch([]string{"supplier_in:Алматы:7.000"}, s.movements(less.ID))

	// без items — факт = заказанное.
	pd := s.newProduct("0")
	def := s.create(with(s.supplyBody(sup, pd, 3), "send", true))
	st, _, raw = s.action(def, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("3.000", s.pairStock(pd, s.almaty))

	// документы с клиентом: факт не вводится.
	taskID, units := s.taskWithUnits(1)
	intake := s.create(with(intakeBody(taskID, s.almaty, units[0]), "send", true))
	for _, items := range []any{
		[]map[string]any{{"id": intake.Items[0].ID, "qty_received": 1}},
		[]map[string]any{{}},
	} {
		st, _, raw = s.action(intake, "receive", map[string]any{"items": items})
		s.Equal(http.StatusBadRequest, st, string(raw))
		s.Equal(map[string]string{"items": "not_allowed"}, s.fields(raw))
	}
	s.Equal(fmt.Sprintf("in_transit||%d", intake.ID), s.unitLoc(units[0]), "единица не принята")
	st, _, raw = s.action(intake, "receive", map[string]any{"items": []any{}})
	s.Require().Equal(http.StatusOK, st, "пустой items допустим: %s", raw)
	back := s.create(with(returnBody(taskID, s.almaty, units[0]), "send", true))
	st, _, raw = s.action(back, "receive", map[string]any{"items": []map[string]any{{"id": back.Items[0].ID, "qty_received": 1}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items": "not_allowed"}, s.fields(raw))

	// receive: true — только приём от клиента.
	before := s.transfersCount()
	for name, body := range map[string]map[string]any{
		"склад → склад":        s.body(productItem(pw, 1)),
		"приход от поставщика": s.supplyBody(sup, pm, 1),
		"возврат клиенту":      returnBody(taskID, s.almaty, units[0]),
	} {
		st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", with(body, "receive", true))
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal(map[string]string{"receive": "not_allowed"}, s.fields(raw), name)
	}
	s.Equal(before, s.transfersCount())

	// негатив: двойной параллельный receive прихода — один успех, остаток вырос один раз.
	pp := s.newProduct("0")
	par := s.create(with(s.supplyBody(sup, pp, 6), "send", true))
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", par.ID), map[string]any{"version": par.Version})
		}(i)
	}
	wg.Wait()
	okCount := 0
	for _, c := range codes {
		if c == http.StatusOK {
			okCount++
		} else {
			s.Equal(http.StatusConflict, c)
		}
	}
	s.Equal(1, okCount, "ровно одна приёмка")
	s.Equal("6.000", s.pairStock(pp, s.almaty))
	s.ElementsMatch([]string{"supplier_in:Алматы:6.000"}, s.movements(par.ID))
}

// ---- TS-015 ----

// TS-015: исполнитель создаёт и проводит документы с клиентом (201 / 200); создание «склад →
// склад» и «поставщик → склад» → 403; GET / PUT / send / receive / cancel документа чужого
// направления → 404 (существование не раскрывается); список не содержит чужих направлений
// даже с фильтром direction (abuse cases 1–2).
func (s *TransfersLiveSuite) TestTS015_ExecutorScope() {
	exec := s.executor()
	sup := s.newSupplier(true)
	p := s.newProduct("10")

	// чужие направления создаёт администратор.
	w2w := s.create(s.body(productItem(p, 1)))
	supply := s.create(with(s.supplyBody(sup, p, 2), "send", true))

	for name, body := range map[string]map[string]any{
		"склад → склад":        s.w2wBody(productItem(p, 1)),
		"приход от поставщика": s.supplyBody(sup, p, 1),
		"склад → склад, send":  with(s.w2wBody(productItem(p, 1)), "send", true),
		// права раньше валидации: тело с ошибками полей чужого направления — тоже 403.
		"чужое направление с мусором": {"direction": "supplier_to_warehouse", "items": []any{}},
	} {
		st, raw := exec.Do(http.MethodPost, "/api/v1/transfers", body)
		s.Equal(http.StatusForbidden, st, "%s → %s", name, raw)
		s.Equal("FORBIDDEN", s.errOf(raw).Code, name)
	}
	st, raw := exec.Do(http.MethodPost, "/api/v1/transfers", map[string]any{"direction": "sideways"})
	s.Equal(http.StatusBadRequest, st, "некорректное направление — 400, не 403: %s", raw)

	for _, doc := range []transferDTO{w2w, supply} {
		base := fmt.Sprintf("/api/v1/transfers/%d", doc.ID)
		for _, cs := range []struct {
			method, path string
			body         any
		}{
			{http.MethodGet, base, nil},
			{http.MethodPut, base, map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{productItem(p, 1)}, "version": doc.Version}},
			{http.MethodPut, base, map[string]any{}},
			{http.MethodPost, base + "/send", map[string]any{"version": doc.Version}},
			{http.MethodPost, base + "/receive", map[string]any{"version": doc.Version}},
			{http.MethodPost, base + "/cancel", map[string]any{"version": doc.Version}},
			{http.MethodPost, base + "/cancel", map[string]any{}}, // и без version — 404, не 400
		} {
			st, raw := exec.Do(cs.method, cs.path, cs.body)
			s.Equal(http.StatusNotFound, st, "%s %s (%s) → %s", cs.method, cs.path, doc.Direction, raw)
			s.Equal("TRANSFER_NOT_FOUND", s.errOf(raw).Code)
		}
		after := s.get(doc.ID)
		s.Equal(doc.Status, after.Status, "документ не тронут")
		s.Equal(doc.Version, after.Version)
	}
	// несуществующий документ отвечает тем же 404 — отличить «нет» от «не твоё» нельзя.
	st, raw = exec.Do(http.MethodGet, "/api/v1/transfers/999999999", nil)
	s.Equal(http.StatusNotFound, st, string(raw))

	// негатив: исполнитель создаёт и проводит приём от клиента и возврат.
	taskID, units := s.taskWithUnits(1)
	st, mine, raw := s.doAs(exec, http.MethodPost, "/api/v1/transfers", intakeBody(taskID, s.almaty, units[0]))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("Партнёры Исполнитель", mine.CreatedByName)
	st, mine, raw = s.doAs(exec, http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", mine.ID),
		map[string]any{"to_warehouse_id": s.moscow, "items": unitItems(units[0]), "version": mine.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, mine, raw = s.doAs(exec, http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/send", mine.ID), map[string]any{"version": mine.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, mine, raw = s.doAs(exec, http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", mine.ID), map[string]any{"version": mine.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", mine.Status)
	s.Equal("at_warehouse|Москва|", s.unitLoc(units[0]))
	st, back, raw := s.doAs(exec, http.MethodPost, "/api/v1/transfers", with(returnBody(taskID, s.moscow, units[0]), "send", true))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, _, raw = s.doAs(exec, http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/cancel", back.ID), map[string]any{"version": back.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	// документ с клиентом, созданный администратором, исполнителю доступен.
	adminDoc := s.create(intakeBody(taskID, s.almaty, s.attachNew(taskID)))
	st, _, raw = s.doAs(exec, http.MethodGet, fmt.Sprintf("/api/v1/transfers/%d", adminDoc.ID), nil)
	s.Equal(http.StatusOK, st, string(raw))

	// список исполнителя: только документы с клиентом, даже с фильтром чужого направления.
	ids, list := s.listIDs(exec, "")
	s.True(ids[mine.ID])
	s.True(ids[back.ID])
	s.True(ids[adminDoc.ID])
	s.False(ids[w2w.ID], "склад → склад исполнителю не виден")
	s.False(ids[supply.ID], "приход от поставщика исполнителю не виден")
	for _, it := range list.Items {
		s.Contains([]string{"client_to_warehouse", "warehouse_to_client"}, it.Direction)
	}
	for _, d := range []string{"supplier_to_warehouse", "warehouse_to_warehouse"} {
		_, l := s.listIDs(exec, "&direction="+d)
		s.Empty(l.Items, "фильтр %s у исполнителя — пустой список", d)
		s.Zero(l.Total)
	}
	_, l := s.listIDs(exec, "&supplier_id="+sup)
	s.Empty(l.Items)
	_, l = s.listIDs(exec, "&warehouse_id="+s.almaty+"&status=in_transit")
	for _, it := range l.Items {
		s.NotEqual(supply.ID, it.ID)
	}
	// администратор видит всё.
	adminIDs, _ := s.listIDs(s.admin, "")
	for _, id := range []int64{w2w.ID, supply.ID, mine.ID, back.ID} {
		s.True(adminIDs[id])
	}
}

// attachNew — новая единица клиента фикстуры «у клиента», сразу в оборудовании заказа.
func (s *TransfersLiveSuite) attachNew(taskID int64) string {
	u := s.newUnitOf(s.clientID, "")
	s.attach(taskID, u)
	return u
}

// TS-015 / TS-034 (SSR): экран и пункт меню «Перемещения» доступны исполнителю; меню
// направлений и фильтр «Направление» — два значения, фильтра «Поставщик» нет. Администратору
// — все четыре направления и фильтр «Поставщик».
func (s *TransfersLiveSuite) TestTS015_SSR_ExecutorScreen() {
	page := func(b *testutil.Browser, path string) (int, string) {
		resp := b.Request(http.MethodGet, path, nil, nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.Require().NoError(err)
		return resp.StatusCode, string(body)
	}
	st, body := page(s.executor(), "/transfers")
	s.Require().Equal(http.StatusOK, st)
	s.Contains(body, `data-testid="st-root" data-is-admin="false"`)
	s.Contains(body, `href="/transfers"`, "пункт меню виден исполнителю")
	s.Contains(body, `data-testid="st-new-direction" data-direction="client_to_warehouse"`)
	s.Contains(body, `data-testid="st-new-direction" data-direction="warehouse_to_client"`)
	s.NotContains(body, `data-direction="warehouse_to_warehouse"`)
	s.NotContains(body, `data-direction="supplier_to_warehouse"`)
	s.NotContains(body, `value="supplier_to_warehouse"`, "фильтр «Направление» — два значения")
	s.NotContains(body, `value="warehouse_to_warehouse"`)
	s.Contains(body, `<option value="warehouse_to_client">Возврат клиенту</option>`)
	s.NotContains(body, `st-filter-supplier`, "фильтра «Поставщик» у исполнителя нет")
	for _, id := range []string{"st-filter-task", "st-filter-from", "st-filter-to", "st-filter-period-error", "st-filter-direction"} {
		s.Contains(body, `data-testid="`+id+`"`)
	}
	_, home := page(s.executor(), "/")
	s.Contains(home, `href="/transfers"`)

	st, body = page(s.admin, "/transfers")
	s.Require().Equal(http.StatusOK, st)
	s.Contains(body, `data-testid="st-root" data-is-admin="true"`)
	for _, d := range []string{"client_to_warehouse", "warehouse_to_client", "warehouse_to_warehouse", "supplier_to_warehouse"} {
		s.Contains(body, `data-testid="st-new-direction" data-direction="`+d+`"`)
		s.Contains(body, `<option value="`+d+`">`)
	}
	s.Contains(body, `data-testid="st-filter-supplier"`)
}

// ---- TS-020 (хвост) ----

// TS-020: архивный поставщик в новом документе → 400 supplier_id: archived; в уже созданном
// остаётся — заявка пересохраняется и проводится; неизвестный → not_found.
func (s *TransfersLiveSuite) TestTS020_ArchivedSupplierInDocument() {
	p := s.newProduct("0")
	archived := s.newSupplier(false)
	active := s.newSupplier(true)

	for name, tc := range map[string]struct {
		supplier any
		want     string
	}{
		"архивный":   {archived, "archived"},
		"неизвестен": {"00000000-0000-0000-0000-000000000000", "not_found"},
		"не uuid":    {"поставщик", "invalid"},
		"пусто":      {"", "required"},
	} {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", with(s.supplyBody(active, p, 1), "supplier_id", tc.supplier))
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal(map[string]string{"supplier_id": tc.want}, s.fields(raw), name)
	}
	st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"direction": "supplier_to_warehouse", "to_warehouse_id": s.almaty, "items": []map[string]any{productItem(p, 1)},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"supplier_id": "required"}, s.fields(raw))

	// поставщик ушёл в архив после создания документа — в документе он остаётся.
	t := s.create(s.supplyBody(active, p, 4))
	_, err := s.Pool.Exec(context.Background(), `UPDATE suppliers SET is_active = false WHERE id = $1`, active)
	s.Require().NoError(err)
	st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", s.supplyBody(active, p, 1))
	s.Equal(http.StatusBadRequest, st, "новый документ на архивного — нельзя: %s", raw)
	s.Equal(map[string]string{"supplier_id": "archived"}, s.fields(raw))

	put := func(supplier string, version int) (int, transferDTO, []byte) {
		return s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
			"to_warehouse_id": s.moscow, "supplier_id": supplier, "items": []map[string]any{productItem(p, 5)}, "version": version,
		})
	}
	st, _, raw = put(archived, t.Version)
	s.Equal(http.StatusBadRequest, st, "сменить на ДРУГОГО архивного нельзя: %s", raw)
	s.Equal(map[string]string{"supplier_id": "archived"}, s.fields(raw))
	st, upd, raw := put(active, t.Version)
	s.Require().Equal(http.StatusOK, st, "архивный поставщик уже в заявке: %s", raw)
	s.Equal(active, upd.SupplierID)
	s.NotEmpty(upd.SupplierName)
	s.Equal(s.moscow, upd.ToWarehouseID)
	st, sent, raw := s.action(upd, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("5.000", s.pairStock(p, s.moscow))

	// PUT без поставщика / со складом отправления — ошибки по полям направления.
	t2 := s.create(s.supplyBody(s.newSupplier(true), p, 1))
	st, _, raw = s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t2.ID), map[string]any{
		"to_warehouse_id": s.almaty, "from_warehouse_id": s.moscow, "items": []map[string]any{productItem(p, 1)}, "version": t2.Version,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"supplier_id": "required", "from_warehouse_id": "not_allowed"}, s.fields(raw))
}

// ---- TS-023 ----

// TS-023: фильтр «период», граница суток. Документ создан 2026-10-06T19:30:00Z — 00:30
// 7 октября по Алматы и 22:30 6 октября по Москве. Пользователь с поясом Asia/Almaty находит
// его 7 октября и не находит 6-го; пользователь с Europe/Moscow — наоборот. Негатив: «по»
// раньше «с» и несуществующая дата → 400 (abuse case 6).
func (s *TransfersLiveSuite) TestTS023_PeriodFilter_UserTimezone() {
	taskID, units := s.taskWithUnits(1)
	doc := s.create(intakeBody(taskID, s.almaty, units[0]))
	// момент создания — фикстура времени TS-F5 (SQL: не предмет теста).
	_, err := s.Pool.Exec(context.Background(), `UPDATE transfers SET created_at = '2026-10-06T19:30:00Z' WHERE id = $1`, doc.ID)
	s.Require().NoError(err)

	tzUser := func(login, tz string) *testutil.Browser {
		s.SeedUser(login, "password1", "Пояс "+login, "admin", true)
		b := s.NewBrowser()
		b.Login(login, "password1")
		st, raw := b.Do(http.MethodPut, "/api/v1/users/me/timezone", map[string]any{"timezone": tz})
		s.Require().Equal(http.StatusOK, st, string(raw))
		return b
	}
	almaty := tzUser("tp-tz-almaty", "Asia/Almaty")
	moscow := tzUser("tp-tz-moscow", "Europe/Moscow")
	found := func(b *testutil.Browser, from, to string) bool {
		q := ""
		if from != "" {
			q += "&created_from=" + from
		}
		if to != "" {
			q += "&created_to=" + to
		}
		ids, _ := s.listIDs(b, q)
		return ids[doc.ID]
	}

	s.True(found(almaty, "2026-10-07", "2026-10-07"), "Алматы: 7 октября — найден")
	s.False(found(almaty, "2026-10-06", "2026-10-06"), "Алматы: 6 октября — нет")
	s.True(found(moscow, "2026-10-06", "2026-10-06"), "Москва: 6 октября — найден")
	s.False(found(moscow, "2026-10-07", "2026-10-07"), "Москва: 7 октября — нет")

	// одна граница и диапазон: «по» включительно, «с» включительно.
	s.True(found(almaty, "2026-10-07", ""))
	s.False(found(almaty, "2026-10-08", ""))
	s.True(found(almaty, "", "2026-10-07"))
	s.False(found(almaty, "", "2026-10-06"))
	s.True(found(moscow, "", "2026-10-06"))
	s.False(found(moscow, "2026-10-07", ""))
	s.True(found(moscow, "2026-10-01", "2026-10-31"))
	// исполнитель с тем же поясом — те же границы (скоуп не мешает: документ с клиентом).
	s.True(found(s.executor(), "2026-10-07", "2026-10-07"), "пояс исполнителя по умолчанию — Алматы")

	// период комбинируется с остальными фильтрами по «И».
	ids, _ := s.listIDs(almaty, fmt.Sprintf("&created_from=2026-10-07&created_to=2026-10-07&direction=client_to_warehouse&task_id=%d&status=requested", taskID))
	s.True(ids[doc.ID])
	ids, _ = s.listIDs(almaty, "&created_from=2026-10-07&created_to=2026-10-07&direction=warehouse_to_client")
	s.False(ids[doc.ID])

	// негатив.
	for q, want := range map[string]map[string]string{
		"created_from=2026-10-07&created_to=2026-10-06": {"created_to": "before_from"},
		"created_from=2026-13-40":                       {"created_from": "invalid"},
		"created_to=2026-02-30":                         {"created_to": "invalid"},
		"created_from=07.10.2026&created_to=вчера":      {"created_from": "invalid", "created_to": "invalid"},
		"created_from=2026-10-07T00:00:00Z":             {"created_from": "invalid"},
	} {
		st, raw := almaty.Do(http.MethodGet, "/api/v1/transfers?"+q, nil)
		s.Equal(http.StatusBadRequest, st, "%s → %s", q, raw)
		s.Equal(want, s.fields(raw), q)
	}
}

// Фильтры списка §6.1: направление, заказ, поставщик — формат, комбинация по «И», поля строки.
func (s *TransfersLiveSuite) TestListFilters_DirectionTaskSupplier() {
	sup, sup2 := s.newSupplier(true), s.newSupplier(true)
	p := s.newProduct("0")
	taskID, units := s.taskWithUnits(1)
	intake := s.create(with(intakeBody(taskID, s.almaty, units[0]), "receive", true))
	back := s.create(returnBody(taskID, s.almaty, units[0]))
	supply := s.create(s.supplyBody(sup, p, 1))
	supply2 := s.create(s.supplyBody(sup2, p, 1))
	w2w := s.create(s.body(productItem(p, 1)))

	check := func(query string, want []int64, not []int64) listDTO {
		ids, l := s.listIDs(s.admin, query)
		for _, id := range want {
			s.True(ids[id], "%s: документ %d должен быть в списке", query, id)
		}
		for _, id := range not {
			s.False(ids[id], "%s: документа %d быть не должно", query, id)
		}
		return l
	}
	check("&direction=client_to_warehouse", []int64{intake.ID}, []int64{back.ID, supply.ID, w2w.ID})
	check("&direction=warehouse_to_client", []int64{back.ID}, []int64{intake.ID, supply.ID, w2w.ID})
	check("&direction=supplier_to_warehouse", []int64{supply.ID, supply2.ID}, []int64{intake.ID, back.ID, w2w.ID})
	check("&direction=warehouse_to_warehouse", []int64{w2w.ID}, []int64{intake.ID, back.ID, supply.ID})
	check(fmt.Sprintf("&task_id=%d", taskID), []int64{intake.ID, back.ID}, []int64{supply.ID, w2w.ID})
	check(fmt.Sprintf("&task_id=%d&status=received", taskID), []int64{intake.ID}, []int64{back.ID})
	check("&supplier_id="+sup, []int64{supply.ID}, []int64{supply2.ID, intake.ID, w2w.ID})
	check("&supplier_id="+sup+"&direction=client_to_warehouse", nil, []int64{supply.ID, intake.ID})
	// склад: отправления ИЛИ назначения — и для документов с одной складской точкой.
	l := check("&warehouse_id="+s.almaty, []int64{intake.ID, back.ID, supply.ID, w2w.ID}, nil)
	check("&warehouse_id="+s.moscow, []int64{w2w.ID}, []int64{intake.ID, back.ID, supply.ID})

	// поля строки списка.
	for _, it := range l.Items {
		switch it.ID {
		case intake.ID:
			s.Equal("client_to_warehouse", it.Direction)
			s.Require().NotNil(it.TaskID)
			s.Equal(taskID, *it.TaskID)
			s.Equal("ТОО Перемещаемый", it.ClientName)
			s.Empty(it.SupplierID)
		case supply.ID:
			s.Equal("supplier_to_warehouse", it.Direction)
			s.Nil(it.TaskID)
			s.Equal(sup, it.SupplierID)
			s.NotEmpty(it.SupplierName)
			s.Empty(it.ClientName)
		case w2w.ID:
			s.Equal("warehouse_to_warehouse", it.Direction)
			s.Nil(it.TaskID)
			s.Empty(it.SupplierID)
		}
	}
	// конверт {items,total,limit,offset} (§API-1).
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/transfers?direction=supplier_to_warehouse&limit=1&offset=1", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var page listDTO
	s.Require().NoError(json.Unmarshal(raw, &page))
	s.Len(page.Items, 1)
	s.Equal(1, page.Limit)
	s.Equal(1, page.Offset)
	s.GreaterOrEqual(page.Total, 2)

	for q, field := range map[string]string{
		"direction=sideways": "direction", "direction=Client_To_Warehouse": "direction",
		"task_id=abc": "task_id", "task_id=0": "task_id", "task_id=-3": "task_id", "task_id=9223372036854775808": "task_id",
		"supplier_id=x": "supplier_id", "supplier_id=%7B" + sup + "%7D": "supplier_id",
	} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/transfers?"+q, nil)
		s.Equal(http.StatusBadRequest, st, "%s → %s", q, raw)
		s.Equal(map[string]string{field: "invalid"}, s.fields(raw), q)
	}
}

// ---- TS-025 ----

// TS-025: «Остатки» — «в пути» включает приход от поставщика в статусе «В пути» для склада
// назначения; заявка (requested), полученный и отменённый документ в «в пути» не входят.
func (s *TransfersLiveSuite) TestTS025_StockInTransit_IncludesSupplier() {
	sup := s.newSupplier(true)
	p := s.newProduct("1")

	t := s.create(s.supplyBody(sup, p, 10))
	s.Equal("0", s.inTransit(p, ""), "заявка от поставщика — не в пути")
	s.Equal("0", s.inTransit(p, s.almaty))

	st, sent, raw := s.action(t, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("10.000", s.inTransit(p, ""))
	s.Equal("10.000", s.inTransit(p, s.almaty), "склад назначения")
	s.Equal("0", s.inTransit(p, s.moscow), "другой склад")

	// вместе со «склад → склад» в тот же склад — суммируются.
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, 5)`, p, s.moscow)
	s.Require().NoError(err)
	w2w := s.create(map[string]any{
		"direction": "warehouse_to_warehouse", "from_warehouse_id": s.moscow, "to_warehouse_id": s.almaty,
		"items": []map[string]any{productItem(p, 2)}, "send": true,
	})
	s.Equal("12.000", s.inTransit(p, s.almaty))
	s.Equal("12.000", s.inTransit(p, ""))
	st, _, raw = s.action(w2w, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, _, raw = s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0", s.inTransit(p, ""), "после приёмки — не в пути")

	cancelled := s.create(with(s.supplyBody(sup, p, 4), "send", true))
	s.Equal("4.000", s.inTransit(p, s.almaty))
	st, _, raw = s.action(cancelled, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("0", s.inTransit(p, s.almaty), "отменённый — не в пути")
}
