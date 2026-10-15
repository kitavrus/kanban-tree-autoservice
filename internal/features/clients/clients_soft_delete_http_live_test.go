//go:build integration

package clients_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление клиентов и единиц оборудования (docs/features/soft-delete-clients/draft-spec.md,
// §11): TS-022, TS-023, TS-024, TS-025, TS-026, TS-033, вкладки раздела
// «Удалённые», служебная уборка, SSR-формы удаления и гонки §3. Сьюта — ClientsLiveSuite
// (§TD-8: боевой бинарь, реальный логин); заказы создаются через API того же сервера.

// ---- wire-типы (§TD-6.2) ----

type sdDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type clientDeleteDTO struct {
	ID               string    `json:"id"`
	DeletedAt        time.Time `json:"deleted_at"`
	EquipmentDeleted int       `json:"equipment_deleted"`
}

type clientRestoreDTO struct {
	ID                string `json:"id"`
	EquipmentRestored int    `json:"equipment_restored"`
}

type unitDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type sdTaskDTO struct {
	ID              int64  `json:"id"`
	ClientID        string `json:"client_id"`
	ClientIsDeleted bool   `json:"client_is_deleted"`
	Version         int64  `json:"version"`
	Title           string `json:"title"`
	ColumnID        string `json:"column_id"`
}

type sdCardDTO struct {
	Task      sdTaskDTO `json:"task"`
	Equipment []struct {
		EquipmentID string `json:"equipment_id"`
		IsDeleted   bool   `json:"is_deleted"`
	} `json:"equipment"`
	Subtasks []struct {
		ID          string  `json:"id"`
		EquipmentID *string `json:"equipment_id"`
	} `json:"subtasks"`
}

type sdDeletedItem struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	DeletedBy struct {
		Name string `json:"name"`
	} `json:"deleted_by"`
	DeleteReason string  `json:"delete_reason"`
	DeletedWith  *string `json:"deleted_with"`
}

// ---- помощники ----

func (s *ClientsLiveSuite) sdErr(raw []byte) sdDetails {
	var e sdDetails
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *ClientsLiveSuite) sdScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func mark() string { return uuid.NewString()[:8] }

func (s *ClientsLiveSuite) deleteClient(b *testutil.Browser, id string, reason any) (int, clientDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, "/api/v1/clients/"+id, body)
	var out clientDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *ClientsLiveSuite) mustDeleteClient(id, reason string) clientDeleteDTO {
	st, out, raw := s.deleteClient(s.admin, id, reason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	return out
}

func (s *ClientsLiveSuite) restoreClient(b *testutil.Browser, id string) (int, clientRestoreDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/clients/"+id+"/restore", nil)
	var out clientRestoreDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *ClientsLiveSuite) deleteUnit(b *testutil.Browser, id string, reason any) (int, unitDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, "/api/v1/equipment/"+id, body)
	var out unitDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *ClientsLiveSuite) mustDeleteUnit(id, reason string) {
	st, _, raw := s.deleteUnit(s.admin, id, reason)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *ClientsLiveSuite) restoreUnit(b *testutil.Browser, id string) (int, []byte) {
	return b.Do(http.MethodPost, "/api/v1/equipment/"+id+"/restore", nil)
}

// unit — действующий автомобиль клиента с уникальным госномером (label — только для
// читаемости теста).
func (s *ClientsLiveSuite) unit(clientID, _ string) string {
	return s.unitPlate(clientID, uniquePlate())
}

// unitPlate — действующий автомобиль клиента с заданным госномером.
func (s *ClientsLiveSuite) unitPlate(clientID, plate string) string {
	st, e, raw := s.createEquipment(s.admin, clientID, carBody(plate))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return e.ID
}

// uniquePlate — уникальный допустимый госномер (латиница/цифры из uuid, ≤20 символов).
func uniquePlate() string { return "SD " + strings.ToUpper(uuid.NewString()[:8]) }

// clientState — «удалён|кто|причина» клиента; пустая строка — строки нет.
func (s *ClientsLiveSuite) clientState(id string) string {
	return s.sdScalar(`
		SELECT coalesce((SELECT (deleted_at IS NOT NULL)::text || '|' || coalesce(u.login, '') || '|' || coalesce(c.delete_reason, '')
		                 FROM clients c LEFT JOIN users u ON u.id = c.deleted_by WHERE c.id = $1::uuid), '')`, id)
}

// unitState — «удалена|кто|причина|вместе с клиентом» единицы; пустая строка — строки нет.
func (s *ClientsLiveSuite) unitState(id string) string {
	return s.sdScalar(`
		SELECT coalesce((SELECT (e.deleted_at IS NOT NULL)::text || '|' || coalesce(u.login, '') || '|' || coalesce(e.delete_reason, '')
		                        || '|' || e.deleted_with_client::text
		                 FROM equipment e LEFT JOIN users u ON u.id = e.deleted_by WHERE e.id = $1::uuid), '')`, id)
}

const liveState = "false|||false"

func (s *ClientsLiveSuite) columnID() string {
	return s.sdScalar(`SELECT id::text FROM columns WHERE NOT is_final AND deleted_at IS NULL ORDER BY position, id LIMIT 1`)
}

// apiTask — заказ клиента через API доски; unitIDs принимаются в заказ.
func (s *ClientsLiveSuite) apiTask(b *testutil.Browser, clientID, title string, unitIDs ...string) sdTaskDTO {
	st, raw := b.Do(http.MethodPost, "/api/v1/tasks", map[string]any{"client_id": clientID, "title": title, "column_id": s.columnID()})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var out struct {
		Task sdTaskDTO `json:"task"`
	}
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	for _, u := range unitIDs {
		st, raw = s.addUnit(b, out.Task.ID, u)
		s.Require().Equal(http.StatusCreated, st, string(raw))
	}
	return out.Task
}

func (s *ClientsLiveSuite) addUnit(b *testutil.Browser, taskID int64, unitID string) (int, []byte) {
	return b.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/equipment", taskID),
		map[string]any{"equipment_id": unitID, "completeness": "", "condition": ""})
}

func (s *ClientsLiveSuite) taskCard(taskID int64) sdCardDTO {
	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var c sdCardDTO
	s.Require().NoError(json.Unmarshal(raw, &c), string(raw))
	return c
}

func (s *ClientsLiveSuite) deletedList(kind, q string) []sdDeletedItem {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/deleted?kind="+kind+"&limit=200&q="+url.QueryEscape(q), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var out struct {
		Items []sdDeletedItem `json:"items"`
		Total int             `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	s.Require().Equal(len(out.Items), out.Total, "выборка помещается в страницу")
	return out.Items
}

func (s *ClientsLiveSuite) executor(login string) *testutil.Browser {
	s.SeedUser(login, "password1", "Исполнитель "+login, "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b
}

// ---- TS-022 ----

// TS-022 (DS-014, DS-015): клиент с действующим заказом удаляется —
// 200; он и ВСЕ его действующие единицы помечены одной причиной и одним моментом,
// deleted_with_client = true, equipment_deleted = числу действующих единиц (удалённая раньше
// отдельно в счёт не входит и свою причину сохраняет); заказ остаётся на доске с
// client_is_deleted = true; телефоны не тронуты. Причина обязательна.
// Негатив: отказа 409 CLIENT_HAS_TASKS нет.
func (s *ClientsLiveSuite) TestTS022_DeleteClientWithTasksAndDocuments() {
	m := mark()
	c := s.newClient("SD-022 Клиент " + m)
	u1, u2 := s.unit(c.ID, "M-022-1"), s.unit(c.ID, "M-022-2")
	early := s.unit(c.ID, "M-022-раньше")
	task := s.apiTask(s.admin, c.ID, "SD-022 заказ "+m, u1, u2, early)
	s.mustDeleteUnit(early, "Списана раньше клиента")

	// причина обязательна (DS-020): клиент не удалён.
	for name, tc := range map[string]struct {
		reason any
		code   string
	}{
		"нет тела": {nil, "required"}, "пробелы": {"   ", "required"},
		"501 символ": {strings.Repeat("я", 501), "too_long"}, "NUL": {"a\x00b", "invalid_text"},
	} {
		st, _, raw := s.deleteClient(s.admin, c.ID, tc.reason)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal(tc.code, s.fields(raw)["reason"], name)
	}
	s.Equal("false||", s.clientState(c.ID))
	s.Equal(liveState, s.unitState(u1), "при отказе единицы не помечены")

	before := time.Now().Add(-time.Minute)
	out := s.mustDeleteClient(c.ID, "  Клиент закрылся  ")
	s.Equal(c.ID, out.ID)
	s.Equal(2, out.EquipmentDeleted, "только действующие единицы")
	s.True(out.DeletedAt.After(before), "deleted_at — момент удаления")
	s.Equal(time.UTC, out.DeletedAt.Location())

	s.Equal("true|clients-admin|Клиент закрылся", s.clientState(c.ID), "причина обрезана")
	for _, u := range []string{u1, u2} {
		s.Equal("true|clients-admin|Клиент закрылся|true", s.unitState(u), "единица ушла вместе с клиентом")
	}
	s.Equal("true|clients-admin|Списана раньше клиента|false", s.unitState(early), "удалённая отдельно сохраняет свою причину")
	s.Equal("2", s.sdScalar(`
		SELECT count(*)::text FROM equipment e JOIN clients c ON c.id = e.client_id
		WHERE c.id = $1::uuid AND e.deleted_with_client AND e.deleted_at = c.deleted_at`, c.ID), "момент удаления — как у клиента")
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM client_phones WHERE client_id = $1::uuid`, c.ID), "телефоны остаются")
	s.Equal(fmt.Sprint(c.Version+1), s.sdScalar(`SELECT version::text FROM clients WHERE id = $1::uuid`, c.ID), "version + 1")

	// заказ клиента остаётся действующим и отдаёт признак и в списке доски, и по номеру.
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+c.ID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list struct {
		Items []sdTaskDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	s.Require().Len(list.Items, 1, "заказ удалённого клиента остаётся на доске")
	s.Equal(task.ID, list.Items[0].ID)
	s.True(list.Items[0].ClientIsDeleted)
	st, raw = s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", task.ID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var one sdTaskDTO
	s.Require().NoError(json.Unmarshal(raw, &one), string(raw))
	s.True(one.ClientIsDeleted)

	// повтор удаления → 404 (DS-024).
	st, _, raw = s.deleteClient(s.admin, c.ID, "Повтор")
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("CLIENT_NOT_FOUND", s.sdErr(raw).Code)
}

// ---- TS-023 ----

// TS-023 (DS-015, DS-024): восстановление клиента возвращает его и единицы с
// deleted_with_client = true (отметка и флаг стёрты); единица, удалённая раньше отдельно,
// остаётся удалённой и восстанавливается сама. Негатив: пока клиент удалён, его единицу — и
// ушедшую с ним, и удалённую отдельно — восстановить нельзя: 409 CLIENT_DELETED; повтор
// восстановления → 409 NOT_DELETED; неизвестный идентификатор → 404.
func (s *ClientsLiveSuite) TestTS023_RestoreClient() {
	m := mark()
	c := s.newClient("SD-023 Клиент " + m)
	u1, u2 := s.unit(c.ID, "M-023-1"), s.unit(c.ID, "M-023-2")
	early := s.unit(c.ID, "M-023-раньше")
	s.mustDeleteUnit(early, "Списана раньше")
	s.Equal(2, s.mustDeleteClient(c.ID, "Удалён по ошибке").EquipmentDeleted)

	for name, u := range map[string]string{"ушла с клиентом": u1, "удалена отдельно": early} {
		st, raw := s.restoreUnit(s.admin, u)
		s.Equal(http.StatusConflict, st, "%s: %s", name, raw)
		e := s.sdErr(raw)
		s.Equal("CLIENT_DELETED", e.Code, name)
		s.Equal("Сначала восстановите клиента", e.Message, name)
	}
	s.Equal("true|clients-admin|Удалён по ошибке|true", s.unitState(u1), "при отказе единица остаётся удалённой")

	st, out, raw := s.restoreClient(s.admin, c.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(c.ID, out.ID)
	s.Equal(2, out.EquipmentRestored, "только ушедшие вместе с клиентом")
	s.Equal("false||", s.clientState(c.ID), "отметка удаления стёрта (DS-030)")
	for _, u := range []string{u1, u2} {
		s.Equal(liveState, s.unitState(u))
	}
	s.Equal("true|clients-admin|Списана раньше|false", s.unitState(early), "удалённая отдельно остаётся удалённой")
	s.Equal(fmt.Sprint(c.Version+2), s.sdScalar(`SELECT version::text FROM clients WHERE id = $1::uuid`, c.ID), "удаление и восстановление — по +1")

	// клиент и его единицы снова существуют для обычных адресов.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var got clientDTO
	s.Require().NoError(json.Unmarshal(raw, &got), string(raw))
	s.Equal(2, got.EquipmentCount, "equipment_count — только действующие единицы")

	// теперь единица, удалённая отдельно, восстанавливается сама.
	st, raw = s.restoreUnit(s.admin, early)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(liveState, s.unitState(early))

	st, _, raw = s.restoreClient(s.admin, c.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NOT_DELETED", s.sdErr(raw).Code)
	st, raw = s.restoreUnit(s.admin, early)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NOT_DELETED", s.sdErr(raw).Code)

	for _, id := range []string{uuid.NewString(), "not-uuid"} {
		st, _, raw = s.restoreClient(s.admin, id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("CLIENT_NOT_FOUND", s.sdErr(raw).Code)
		st, raw = s.restoreUnit(s.admin, id)
		s.Equal(http.StatusNotFound, st, "%s: %s", id, raw)
		s.Equal("EQUIPMENT_NOT_FOUND", s.sdErr(raw).Code)
	}
}

// ---- TS-024 ----

// TS-024 (DS-014): единица, бывшая в заказе и в услуге заказа, удаляется — 200 (отказа
// EQUIPMENT_USED_IN_TASKS нет), deleted_with_client = false. Причина обязательна. Удалённая единица
// для обычных адресов не существует. Негатив: добавление удалённой единицы в заказ и назначение
// её услуге → 400 equipment_id: not_found; уже назначенная услуге остаётся.
func (s *ClientsLiveSuite) TestTS024_DeleteEquipment() {
	m := mark()
	c := s.newClient("SD-024 Клиент " + m)
	used, spare := s.unit(c.ID, "M-024-в-заказе"), s.unit(c.ID, "M-024-свободна")
	task := s.apiTask(s.admin, c.ID, "SD-024 заказ "+m, used, spare)

	// услуга заказа с этой единицей (subtasks.equipment_id).
	svc := s.newServiceDirect("SD-024 услуга " + m)
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", task.ID), map[string]any{"service_id": svc})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", task.ID), map[string]any{"service_id": svc})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	card := s.taskCard(task.ID)
	s.Require().Len(card.Subtasks, 2)
	assigned, other := card.Subtasks[0].ID, card.Subtasks[1].ID
	putSub := func(id string, unitID any) (int, []byte) {
		return s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+id, map[string]any{
			"status": "new", "assignee_id": nil, "due_date": nil, "price": nil, "equipment_id": unitID})
	}
	st, raw = putSub(assigned, used)
	s.Require().Equal(http.StatusOK, st, string(raw))

	// причина обязательна.
	st, _, raw = s.deleteUnit(s.admin, used, " ")
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["reason"])
	s.Equal(liveState, s.unitState(used))

	st, out, raw := s.deleteUnit(s.admin, used, " Списана клиентом ")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(used, out.ID)
	s.False(out.DeletedAt.IsZero())
	s.Equal("true|clients-admin|Списана клиентом|false", s.unitState(used), "удалена отдельно — без пометки «вместе с клиентом»")
	s.Equal("false||", s.clientState(c.ID), "клиент не затронут")

	// для обычных адресов удалённая единица не существует (DS-026).
	for _, rq := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/equipment/" + used, nil},
		{http.MethodGet, "/api/v1/equipment/" + used + "/history", nil},
		{http.MethodPut, "/api/v1/equipment/" + used, carBody("Х000ХХ 77")},
		{http.MethodDelete, "/api/v1/equipment/" + used, reasonBody("Повтор")},
	} {
		st, raw = s.admin.Do(rq.method, rq.path, rq.body)
		s.Equal(http.StatusNotFound, st, "%s %s: %s", rq.method, rq.path, raw)
		s.Equal("EQUIPMENT_NOT_FOUND", s.sdErr(raw).Code, rq.path)
	}
	for _, page := range []string{"/equipment/" + used, "/equipment/" + used + "/history"} {
		st, _ = s.admin.Do(http.MethodGet, page, nil)
		s.Equal(http.StatusNotFound, st, "SSR %s", page)
	}

	// негатив: удалённую единицу нельзя ни принять в заказ, ни назначить услуге заново.
	second := s.apiTask(s.admin, c.ID, "SD-024 второй заказ "+m)
	st, raw = s.addUnit(s.admin, second.ID, used)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["equipment_id"])
	st, raw = putSub(other, used)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["equipment_id"])
	// уже назначенная услуге единица остаётся: услуга правится без смены единицы.
	st, raw = putSub(assigned, used)
	s.Equal(http.StatusOK, st, string(raw))
	// действующую единицу услуге назначить по-прежнему можно.
	st, raw = putSub(other, spare)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- TS-025 ----

// TS-025 (DS-014, DS-026): удалённые не выбираются и не видны. Новый заказ и смена клиента
// заказа на удалённого → 400 client_id: not_found; списки клиентов и единиц клиента
// удалённых не содержат; equipment_count — только действующие. Негатив: чтение и правка
// удалённого клиента, список и создание его оборудования (доступно исполнителю) → 404.
func (s *ClientsLiveSuite) TestTS025_DeletedNotSelectable() {
	m := mark()
	gone := s.newClient("SD-025 Удалённый " + m)
	live := s.newClient("SD-025 Действующий " + m)
	s.unit(gone.ID, "M-025-ушла-"+m)
	keep, dropped := s.unit(live.ID, "M-025-живая-"+m), s.unit(live.ID, "M-025-удалена-"+m)
	task := s.apiTask(s.admin, live.ID, "SD-025 заказ "+m)
	s.mustDeleteUnit(dropped, "Списана")
	s.mustDeleteClient(gone.ID, "Закрылся")

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{"client_id": gone.ID, "title": "Заказ удалённого", "column_id": s.columnID()})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["client_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM tasks WHERE client_id = $1::uuid`, gone.ID), "заказ не создан")

	st, raw = s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title": task.Title, "client_id": gone.ID, "column_id": task.ColumnID, "assignee_id": nil, "due_date": nil,
		"is_paid": false, "version": task.Version})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["client_id"])
	s.Equal(live.ID, s.sdScalar(`SELECT client_id::text FROM tasks WHERE id = $1`, task.ID), "клиент заказа не сменился")

	// списки.
	var clients struct {
		Items []clientDTO `json:"items"`
	}
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?limit=200&q="+url.QueryEscape("SD-025")+"+"+m, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?limit=200&q="+url.QueryEscape(m), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &clients), string(raw))
	s.Require().Len(clients.Items, 1, "в списке только действующий клиент: %s", raw)
	s.Equal(live.ID, clients.Items[0].ID)
	s.Equal(1, clients.Items[0].EquipmentCount, "equipment_count — только действующие единицы")

	var units struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+live.ID+"/equipment", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &units), string(raw))
	s.Require().Len(units.Items, 1)
	s.Equal(1, units.Total)
	s.Equal(keep, units.Items[0].ID)

	// негатив: удалённый клиент для обычных адресов не существует.
	exec := s.executor("sd-025-exec-" + m)
	for _, rq := range []struct {
		b            *testutil.Browser
		method, path string
		body         any
	}{
		{s.admin, http.MethodGet, "/api/v1/clients/" + gone.ID, nil},
		{s.admin, http.MethodPut, "/api/v1/clients/" + gone.ID, card(gone, nil)},
		{s.admin, http.MethodGet, "/api/v1/clients/" + gone.ID + "/equipment", nil},
		{exec, http.MethodPost, "/api/v1/clients/" + gone.ID + "/equipment", carBody(uniquePlate())},
	} {
		st, raw = rq.b.Do(rq.method, rq.path, rq.body)
		s.Equal(http.StatusNotFound, st, "%s %s: %s", rq.method, rq.path, raw)
		s.Equal("CLIENT_NOT_FOUND", s.sdErr(raw).Code, rq.path)
	}
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM equipment WHERE client_id = $1::uuid`, gone.ID), "единица удалённому клиенту не добавлена")
	for _, page := range []string{"/clients/" + gone.ID, "/clients/" + gone.ID + "/equipment/new"} {
		st, _ = s.admin.Do(http.MethodGet, page, nil)
		s.Equal(http.StatusNotFound, st, "SSR %s", page)
	}
	st, page := s.admin.Do(http.MethodGet, "/clients?q="+url.QueryEscape(m), nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), live.CompanyName)
	s.NotContains(string(page), gone.CompanyName, "удалённого клиента в справочнике нет")
}

// ---- TS-026 ----

// TS-026 (DS-014, часть 4): карточка действующего заказа с удалёнными клиентом и единицей
// отдаёт client_is_deleted и is_deleted = true; заказ удалённого клиента правится без смены
// клиента. Восстановление такого заказа из удалённых даёт notices client_deleted и
// equipment_deleted (DS-016). Негатив: у действующих записей оба признака = false.
func (s *ClientsLiveSuite) TestTS026_CardMarks() {
	m := mark()
	c := s.newClient("SD-026 Клиент " + m)
	gone, live := s.unit(c.ID, "M-026-удалена"), s.unit(c.ID, "M-026-живая")
	task := s.apiTask(s.admin, c.ID, "SD-026 заказ "+m, gone, live)

	flags := func() (bool, map[string]bool) {
		card := s.taskCard(task.ID)
		units := map[string]bool{}
		for _, e := range card.Equipment {
			units[e.EquipmentID] = e.IsDeleted
		}
		s.Require().Len(units, 2, "обе единицы остаются в заказе")
		return card.Task.ClientIsDeleted, units
	}

	clientDeleted, units := flags()
	s.False(clientDeleted, "негатив: клиент действует")
	s.False(units[gone])
	s.False(units[live])

	s.mustDeleteUnit(gone, "Списана")
	clientDeleted, units = flags()
	s.False(clientDeleted)
	s.True(units[gone], "единица удалена — пометка в карточке")
	s.False(units[live], "соседняя единица не помечена")

	s.mustDeleteClient(c.ID, "Закрылся")
	clientDeleted, units = flags()
	s.True(clientDeleted)
	s.True(units[gone])
	s.True(units[live], "ушла вместе с клиентом")

	// §8.3 п.3: заказ удалённого клиента правится как обычно, без смены клиента.
	cur := s.taskCard(task.ID).Task
	st, raw := s.admin.Do(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", task.ID), map[string]any{
		"title": "SD-026 переименован " + m, "client_id": c.ID, "column_id": cur.ColumnID, "assignee_id": nil, "due_date": nil,
		"is_paid": false, "version": cur.Version})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// DS-016: заказ восстанавливается и с удалёнными клиентом и оборудованием — с предупреждениями.
	st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", task.ID), reasonBody("Дубль"))
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", task.ID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var restored struct {
		Notices []struct{ Code, Message string } `json:"notices"`
	}
	s.Require().NoError(json.Unmarshal(raw, &restored), string(raw))
	notices := map[string]string{}
	for _, n := range restored.Notices {
		notices[n.Code] = n.Message
	}
	s.Equal("Клиент заказа в удалённых", notices["client_deleted"])
	s.Equal("Автомобиль заказа в удалённых", notices["equipment_deleted"])

	// клиента восстановили — пометки ушли; единица, удалённая отдельно, остаётся помеченной.
	_, _, raw = s.restoreClient(s.admin, c.ID)
	clientDeleted, units = flags()
	s.False(clientDeleted, string(raw))
	s.True(units[gone])
	s.False(units[live])
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): четыре адреса удаления и восстановления клиента и единицы —
// только администратору: исполнитель → 403 «Нет доступа» ДО поиска записи (в том числе по
// несуществующему и по битому идентификатору, с причиной и без), запись не тронута. Негатив:
// без сессии → 401.
func (s *ClientsLiveSuite) TestTS033_SoftDeleteAdminOnly() {
	m := mark()
	c := s.newClient("SD-033 Клиент " + m)
	u := s.unit(c.ID, "M-033")
	deletedClient := s.newClient("SD-033 Удалённый " + m)
	s.mustDeleteClient(deletedClient.ID, "Фикстура")
	exec := s.executor("sd-033-exec-" + m)
	anon := s.NewBrowser()

	for _, id := range []string{c.ID, u, deletedClient.ID, uuid.NewString(), "not-uuid"} {
		for _, rq := range []struct{ method, path string }{
			{http.MethodDelete, "/api/v1/clients/" + id},
			{http.MethodPost, "/api/v1/clients/" + id + "/restore"},
			{http.MethodDelete, "/api/v1/equipment/" + id},
			{http.MethodPost, "/api/v1/equipment/" + id + "/restore"},
		} {
			for _, body := range []any{nil, reasonBody("Попытка исполнителя")} {
				st, raw := exec.Do(rq.method, rq.path, body)
				s.Equal(http.StatusForbidden, st, "%s %s: %s", rq.method, rq.path, raw)
				e := s.sdErr(raw)
				s.Equal("FORBIDDEN", e.Code, rq.path)
				s.Equal("Нет доступа", e.Message, rq.path)
			}
			st, _ := anon.Do(rq.method, rq.path, reasonBody("Без сессии"))
			s.Equal(http.StatusUnauthorized, st, "%s %s без сессии", rq.method, rq.path)
		}
	}
	s.Equal("false||", s.clientState(c.ID), "клиент не удалён")
	s.Equal(liveState, s.unitState(u), "единица не удалена")
	s.Equal("true|clients-admin|Фикстура", s.clientState(deletedClient.ID), "удалённый клиент не восстановлен")
}

// ---- раздел «Удалённые» (§6.5, §8.2) ----

// Виды clients и equipment раздела «Удалённые» (DS-011, DS-015): клиент — title = название
// компании, subtitle пуст, deleted_with = null; автомобиль — title = госномер, subtitle —
// название клиента, deleted_with = "client" только у ушедшей с клиентом; причина и «кто удалил»
// — как у записи; поиск по названию и причине; счётчики summary совпадают с total списков.
// Негатив: действующие и восстановленные записи в списки не входят.
func (s *ClientsLiveSuite) TestDeletedTabs_ClientsAndEquipment() {
	m := mark()
	c := s.newClient("SD-tab Клиент " + m)
	other := s.newClient("SD-tab Сосед " + m)
	plateWith, plateAlone := uniquePlate(), uniquePlate()
	with := s.unitPlate(c.ID, plateWith)
	alone := s.unitPlate(other.ID, plateAlone)
	live := s.unit(other.ID, "живая")
	s.mustDeleteUnit(alone, "Списана "+m)
	s.mustDeleteClient(c.ID, "Закрылся "+m)

	clients := s.deletedList("clients", m)
	s.Require().Len(clients, 1, "только удалённый клиент")
	s.Equal(sdDeletedItem{Kind: "clients", ID: c.ID, Title: c.CompanyName, Subtitle: "", DeleteReason: "Закрылся " + m,
		DeletedBy: clients[0].DeletedBy}, clients[0])
	s.Equal("Клиенты Админ", clients[0].DeletedBy.Name)
	s.Nil(clients[0].DeletedWith)
	s.Len(s.deletedList("clients", "закрылся "+m), 1, "поиск по причине без учёта регистра")

	units := s.deletedList("equipment", m)
	s.Require().Len(units, 2, "две удалённые единицы; действующая не входит")
	byID := map[string]sdDeletedItem{}
	for _, it := range units {
		byID[it.ID] = it
	}
	s.NotContains(byID, live)
	s.Equal(plateWith, byID[with].Title, "заголовок — госномер")
	s.Equal(c.CompanyName, byID[with].Subtitle)
	s.Equal("Закрылся "+m, byID[with].DeleteReason, "причина наследуется от клиента")
	s.Require().NotNil(byID[with].DeletedWith)
	s.Equal("client", *byID[with].DeletedWith)
	s.Equal(plateAlone, byID[alone].Title)
	s.Equal(other.CompanyName, byID[alone].Subtitle)
	s.Nil(byID[alone].DeletedWith, "удалена отдельно — deleted_with = null")
	s.Len(s.deletedList("equipment", plateAlone), 1, "поиск по госномеру")
	s.Len(s.deletedList("equipment", c.CompanyName), 1, "поиск по названию клиента")

	// счётчики вкладок = total своих списков.
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/deleted/summary", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var sum struct {
		Counts map[string]int `json:"counts"`
	}
	s.Require().NoError(json.Unmarshal(raw, &sum), string(raw))
	s.Equal(s.sdScalar(`SELECT count(*)::text FROM clients WHERE deleted_at IS NOT NULL`), fmt.Sprint(sum.Counts["clients"]))
	s.Equal(s.sdScalar(`SELECT count(*)::text FROM equipment WHERE deleted_at IS NOT NULL`), fmt.Sprint(sum.Counts["equipment"]))
	s.Positive(sum.Counts["clients"])
	s.GreaterOrEqual(sum.Counts["equipment"], 2)

	// восстановленные из списков уходят.
	_, _, _ = s.restoreClient(s.admin, c.ID)
	_, _ = s.restoreUnit(s.admin, alone)
	s.Empty(s.deletedList("clients", m))
	s.Empty(s.deletedList("equipment", m))
}

// ---- служебная уборка (§6.5, DS-027, TS-F9) ----

// Виды clients и equipment адреса DELETE /api/v1/test/purge/:kind/:id: действующая и мягко
// удалённая запись стираются физически (клиент — вместе с телефонами и единицами); запись, на
// которую ссылаются заказы, → 409 CONFLICT «Запись используется» и
// остаётся; нет записи либо битый идентификатор → 404 NOT_FOUND. Негатив: исполнитель → 403.
func (s *ClientsLiveSuite) TestPurge_ClientsAndEquipment() {
	m := mark()
	purge := func(b *testutil.Browser, kind, id string) (int, []byte) {
		return b.Do(http.MethodDelete, "/api/v1/test/purge/"+kind+"/"+id, nil)
	}

	live := s.newClient("SD-purge Действует " + m)
	liveUnit := s.unit(live.ID, "M-purge-1")
	gone := s.newClient("SD-purge Удалён " + m)
	s.unit(gone.ID, "M-purge-2")
	s.mustDeleteClient(gone.ID, "Фикстура")
	held := s.newClient("SD-purge С заказом " + m)
	heldUnit := s.unit(held.ID, "M-purge-в-заказе")
	freeUnit := s.unit(held.ID, "M-purge-свободна")
	s.apiTask(s.admin, held.ID, "SD-purge заказ "+m, heldUnit)
	s.mustDeleteUnit(freeUnit, "Фикстура")

	exec := s.executor("sd-purge-exec-" + m)
	st, raw := purge(exec, "clients", live.ID)
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("false||", s.clientState(live.ID))

	for kind, id := range map[string]string{"clients": held.ID, "equipment": heldUnit} {
		st, raw = purge(s.admin, kind, id)
		s.Equal(http.StatusConflict, st, "%s: %s", kind, raw)
		e := s.sdErr(raw)
		s.Equal("CONFLICT", e.Code, kind)
		s.Equal("Запись используется", e.Message, kind)
	}
	s.NotEmpty(s.clientState(held.ID), "клиент с заказом на месте")
	s.NotEmpty(s.unitState(heldUnit), "единица заказа на месте")

	// единицы: действующая и мягко удалённая.
	for _, id := range []string{liveUnit, freeUnit} {
		st, raw = purge(s.admin, "equipment", id)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Empty(s.unitState(id), "строка единицы физически удалена")
	}
	// клиенты: действующий и мягко удалённый — вместе с телефонами и единицами.
	for _, id := range []string{live.ID, gone.ID} {
		st, raw = purge(s.admin, "clients", id)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var out struct {
			ID string `json:"id"`
		}
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		s.Equal(id, out.ID)
		s.Empty(s.clientState(id), "строка клиента физически удалена")
		s.Equal("0|0", s.sdScalar(`
			SELECT (SELECT count(*) FROM client_phones WHERE client_id = $1::uuid)::text || '|' ||
			       (SELECT count(*) FROM equipment WHERE client_id = $1::uuid)::text`, id))
	}
	for _, kind := range []string{"clients", "equipment"} {
		for _, id := range []string{live.ID, "not-uuid"} {
			st, raw = purge(s.admin, kind, id)
			s.Equal(http.StatusNotFound, st, "%s %s: %s", kind, id, raw)
			s.Equal("NOT_FOUND", s.sdErr(raw).Code)
		}
	}
}

// ---- SSR-формы удаления (§6.5, §8.1, DS-032) ----

func (s *ClientsLiveSuite) postForm(path string, form url.Values) (int, string, string) {
	form.Set("_csrf", s.admin.CSRFToken())
	resp := s.admin.Request(http.MethodPost, path, form, nil)
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Require().NoError(err)
	return resp.StatusCode, resp.Header.Get("Location"), string(body)
}

// SSR POST /clients/:id/delete и /equipment/:id/delete: поле reason обязательно — без причины
// форма возвращается (422) с окном «Удаление» и текстом под полем, запись не удалена; успех — PRG и тост с числом единиц. Удалённая
// запись — страница 404. Негатив: длинная причина — свой текст, введённая причина сохранена.
func (s *ClientsLiveSuite) TestSSR_SoftDeleteForms() {
	m := mark()
	c := s.newClient("SD-ssr Клиент " + m)
	plate2 := uniquePlate()
	u1, u2 := s.unit(c.ID, "M-ssr-1"), s.unitPlate(c.ID, plate2)
	s.apiTask(s.admin, c.ID, "SD-ssr заказ "+m, u1, u2)

	// форма клиента: окно «Удаление» с пояснением и числом единиц.
	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="client-delete-form" data-delete-dialog`)
	s.Contains(string(page), "Автомобили клиента (2 шт.) будут удалены вместе с ним")
	s.NotContains(string(page), "data-delete-error")

	// причина не указана / слишком длинная.
	status, _, body := s.postForm("/clients/"+c.ID+"/delete", url.Values{})
	s.Equal(http.StatusUnprocessableEntity, status)
	s.Contains(body, `data-delete-error="Укажите причину удаления"`)
	long := strings.Repeat("я", 501)
	status, _, body = s.postForm("/clients/"+c.ID+"/delete", url.Values{"reason": {long}})
	s.Equal(http.StatusUnprocessableEntity, status)
	s.Contains(body, `data-delete-error="Причина — не длиннее 500 символов"`)
	s.Contains(body, `name="reason" value="`+long+`"`, "введённая причина сохранена")
	status, _, body = s.postForm("/equipment/"+u1+"/delete", url.Values{"reason": {"  "}})
	s.Equal(http.StatusUnprocessableEntity, status)
	s.Contains(body, `data-delete-error="Укажите причину удаления"`)
	s.Equal("false||", s.clientState(c.ID))
	s.Equal(liveState, s.unitState(u1))

	// успех: единица → карточка клиента с тостом; клиент → список с тостом и числом единиц.
	status, loc, _ := s.postForm("/equipment/"+u2+"/delete", url.Values{"reason": {"Списана клиентом"}})
	s.Equal(http.StatusSeeOther, status)
	s.Equal("/clients/"+c.ID+"?flash=eq_deleted", loc)
	st, page = s.admin.Do(http.MethodGet, loc, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-kt-toast="Удалено. Восстановить можно в разделе «Удалённые»"`)
	s.NotContains(string(page), plate2, "удалённой единицы в списке клиента нет")
	s.Equal("true|clients-admin|Списана клиентом|false", s.unitState(u2))

	status, loc, _ = s.postForm("/clients/"+c.ID+"/delete", url.Values{"reason": {"Закрылся"}})
	s.Equal(http.StatusSeeOther, status)
	s.Equal("/clients?flash=deleted&n=1", loc)
	st, page = s.admin.Do(http.MethodGet, loc, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-kt-toast="Удалено вместе с автомобилями: 1. Восстановить можно в разделе «Удалённые»"`)
	s.Equal("true|clients-admin|Закрылся", s.clientState(c.ID))
	s.Equal("true|clients-admin|Закрылся|true", s.unitState(u1))

	// запись удалил другой администратор, пока форма открыта, — страница 404.
	status, _, _ = s.postForm("/clients/"+c.ID+"/delete", url.Values{"reason": {"Повтор"}})
	s.Equal(http.StatusNotFound, status)
	status, _, _ = s.postForm("/equipment/"+u2+"/delete", url.Values{"reason": {"Повтор"}})
	s.Equal(http.StatusNotFound, status)
}

// ---- гонки (§3 «Конкурентность») ----

// lockWaiters — число серверных соединений, ждущих блокировку строки.
func (s *ClientsLiveSuite) lockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

type raceResult struct {
	status int
	raw    []byte
}

// race — запрос req уходит, пока тест держит блокировку в tx; по pg_stat_activity тест
// дожидается, что запрос встал в ожидание, затем фиксирует tx и возвращает ответ. Без sleep.
func (s *ClientsLiveSuite) race(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) raceResult {
	ctx := context.Background()
	commit := hold(ctx)
	base := s.lockWaiters()
	done := make(chan raceResult, 1)
	go func() {
		st, raw := req()
		done <- raceResult{st, raw}
	}()
	s.Require().Eventually(func() bool { return s.lockWaiters() > base }, 10*time.Second, 20*time.Millisecond,
		"запрос ждёт блокировку")
	select {
	case res := <-done:
		s.FailNow("запрос не дождался блокировки", "%d %s", res.status, res.raw)
	default:
	}
	commit()
	return <-done
}

// holdClientDelete — «удаление клиента в полёте»: те же блокировки и UPDATE, что делает
// Repo.SoftDeleteClient (клиент FOR UPDATE → его действующие единицы FOR NO KEY UPDATE по id →
// отметки), транзакция не зафиксирована. Возвращает фиксацию.
func (s *ClientsLiveSuite) holdClientDelete(clientID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		adminID := s.sdScalar(`SELECT id::text FROM users WHERE login = 'clients-admin'`)
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM clients WHERE id = $1::uuid AND deleted_at IS NULL FOR UPDATE`, clientID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM equipment WHERE client_id = $1::uuid AND deleted_at IS NULL ORDER BY id FOR NO KEY UPDATE`, clientID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE clients SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'Гонка' WHERE id = $1::uuid`, clientID, adminID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `
			UPDATE equipment SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'Гонка', deleted_with_client = true
			WHERE client_id = $1::uuid AND deleted_at IS NULL`, clientID, adminID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 п. 5 (DS-014, DS-024): удаление клиента идёт — в это окно приходит создание заказа для
// него. Заказ обязан дождаться (клиент читается FOR KEY SHARE с условием «не удалён») и получить
// 400 client_id: not_found; заказ не создан.
func (s *ClientsLiveSuite) TestRace_TaskCreateWaitsForClientDelete() {
	m := mark()
	c := s.newClient("SD-race Клиент-заказ " + m)
	column := s.columnID()

	res := s.race(s.holdClientDelete(c.ID), func() (int, []byte) {
		return s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{"client_id": c.ID, "title": "Заказ гонки " + m, "column_id": column})
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["client_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM tasks WHERE client_id = $1::uuid`, c.ID), "заказ не создан")
}

// §3 п. 1 (DS-015, abuse case 3): удаление клиента идёт — в это окно исполнитель добавляет ему
// единицу. Создание читает клиента FOR KEY SHARE с условием «не удалён»: дожидается удаления и
// получает 404 CLIENT_NOT_FOUND; действующей единицы у удалённого клиента не появляется
// (инвариант «у удалённого клиента нет действующих единиц»).
func (s *ClientsLiveSuite) TestRace_EquipmentCreateWaitsForClientDelete() {
	m := mark()
	c := s.newClient("SD-race Клиент-единица " + m)
	s.unit(c.ID, "M-race-была")
	exec := s.executor("sd-race-exec-" + m)

	res := s.race(s.holdClientDelete(c.ID), func() (int, []byte) {
		st, _, raw := s.createEquipment(exec, c.ID, carBody(uniquePlate()))
		return st, raw
	})
	s.Require().Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("CLIENT_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("1|0", s.sdScalar(`
		SELECT count(*)::text || '|' || (count(*) FILTER (WHERE deleted_at IS NULL))::text
		FROM equipment WHERE client_id = $1::uuid`, c.ID), "новая единица не добавлена, действующих нет")
}

// §3 п. 4 (DS-015): удаление клиента идёт — в это окно приходит восстановление его единицы,
// удалённой раньше отдельно. Восстановление читает клиента FOR KEY SHARE: дожидается удаления
// и получает 409 CLIENT_DELETED; единица остаётся удалённой — действующей единицы у удалённого
// клиента не появляется.
func (s *ClientsLiveSuite) TestRace_UnitRestoreWaitsForClientDelete() {
	m := mark()
	c := s.newClient("SD-race Клиент-восстановление " + m)
	u := s.unit(c.ID, "M-race-удалена-раньше")
	s.mustDeleteUnit(u, "Списана раньше")

	res := s.race(s.holdClientDelete(c.ID), func() (int, []byte) { return s.restoreUnit(s.admin, u) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("CLIENT_DELETED", s.sdErr(res.raw).Code)
	s.Equal("true|clients-admin|Списана раньше|false", s.unitState(u), "единица остаётся удалённой")
}

// holdUnitDelete — «удаление единицы в полёте»: та же блокировка и UPDATE, что делает
// Repo.SoftDeleteEquipment, транзакция не зафиксирована. Возвращает фиксацию.
func (s *ClientsLiveSuite) holdUnitDelete(unitID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		adminID := s.sdScalar(`SELECT id::text FROM users WHERE login = 'clients-admin'`)
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM equipment WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`, unitID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE equipment SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'Гонка' WHERE id = $1::uuid`, unitID, adminID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 (DS-014, ревью): удаление единицы идёт — в это окно её принимают в заказ
// (POST /tasks/:id/equipment). Добавление читает единицу FOR SHARE с условием «не удалена»:
// обязано дождаться удаления и получить 400 equipment_id: not_found — строки заказа с
// только что удалённой единицей не появляется.
func (s *ClientsLiveSuite) TestRace_TaskEquipmentAddWaitsForUnitDelete() {
	m := mark()
	c := s.newClient("SD-race Клиент-в-заказ " + m)
	u := s.unit(c.ID, "M-race-в-заказ")
	task := s.apiTask(s.admin, c.ID, "SD-race заказ "+m)

	res := s.race(s.holdUnitDelete(u), func() (int, []byte) { return s.addUnit(s.admin, task.ID, u) })
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["equipment_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM task_equipment WHERE task_id = $1`, task.ID), "единица в заказ не принята")
}

// Обратный порядок той же гонки: единицу принимают в заказ (заказ под FOR UPDATE, единица под
// FOR SHARE, строка task_equipment не зафиксирована) — в это окно приходит удаление единицы.
// Удаление обязано дождаться добавления; заказ единицу не удерживает (DS-014) — удаление
// проходит, единица остаётся в заказе уже удалённой (с бейджем). Ровно один порядок, без 500.
func (s *ClientsLiveSuite) TestRace_UnitDeleteWaitsForTaskEquipmentAdd() {
	m := mark()
	c := s.newClient("SD-race Клиент-из-заказа " + m)
	u := s.unit(c.ID, "M-race-из-заказа")
	task := s.apiTask(s.admin, c.ID, "SD-race заказ "+m)

	res := s.race(func(ctx context.Context) func() {
		// «добавление в заказ в полёте»: те же блокировки и INSERT, что в CardService.AddEquipment.
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM tasks WHERE id = $1 FOR UPDATE`, task.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT client_id FROM equipment WHERE id = $1::uuid AND deleted_at IS NULL FOR SHARE`, u)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2::uuid)`, task.ID, u)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) {
		st, _, raw := s.deleteUnit(s.admin, u, "Гонка")
		return st, raw
	})
	s.Require().Equal(http.StatusOK, res.status, string(res.raw))
	s.Equal("true|clients-admin|Гонка|false", s.unitState(u))
	card := s.taskCard(task.ID)
	s.Require().Len(card.Equipment, 1, "единица осталась в заказе")
	s.True(card.Equipment[0].IsDeleted, "с пометкой «удалён»")
}

// §3 (DS-014, ревью): удаление единицы идёт — в это окно её назначают услуге заказа
// (PUT /subtasks/:id с equipment_id). Назначение читает единицу FOR SHARE: обязано дождаться
// удаления и получить 400 equipment_id: not_found — услуга остаётся без единицы.
func (s *ClientsLiveSuite) TestRace_SubtaskAssignWaitsForUnitDelete() {
	m := mark()
	c := s.newClient("SD-race Клиент-услуга " + m)
	u := s.unit(c.ID, "M-race-услуга")
	task := s.apiTask(s.admin, c.ID, "SD-race заказ "+m, u)
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", task.ID),
		map[string]any{"service_id": s.newServiceDirect("SD-race услуга " + m)})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	card := s.taskCard(task.ID)
	s.Require().Len(card.Subtasks, 1)
	sub := card.Subtasks[0].ID

	res := s.race(s.holdUnitDelete(u), func() (int, []byte) {
		return s.admin.Do(http.MethodPut, "/api/v1/subtasks/"+sub, map[string]any{
			"status": "new", "assignee_id": nil, "due_date": nil, "price": nil, "equipment_id": u})
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["equipment_id"])
	s.Equal("", s.sdScalar(`SELECT coalesce(equipment_id::text, '') FROM subtasks WHERE id = $1::uuid`, sub), "единица услуге не назначена")
}

// §3 п. 2, 4 (DS-015, ревью): восстановление клиента идёт (клиент под FOR UPDATE, отметки
// клиента и его единиц сняты, транзакция не зафиксирована).
//   - Операция с его единицей, ушедшей вместе с ним, — принять в заказ — блокировку не ждёт:
//     до фиксации единица для всех остаётся удалённой, запрос сразу получает 400 not_found (частично восстановленного состояния никто не видит).
//   - Восстановление его единицы, удалённой раньше отдельно, читает клиента FOR KEY SHARE:
//     обязано дождаться восстановления клиента и пройти (200) — без ожидания получило бы 409
//     CLIENT_DELETED по уже неактуальному состоянию.
//
// После фиксации те же операции с восстановленной единицей проходят.
func (s *ClientsLiveSuite) TestRace_ClientRestoreVsUnitOperations() {
	m := mark()
	c := s.newClient("SD-race Клиент-возврат " + m)
	with, alone := s.unit(c.ID, "M-race-с-клиентом"), s.unit(c.ID, "M-race-отдельно")
	s.apiTask(s.admin, c.ID, "SD-race заказ "+m, with)
	second := s.apiTask(s.admin, c.ID, "SD-race второй заказ "+m)
	s.mustDeleteUnit(alone, "Списана раньше")
	s.mustDeleteClient(c.ID, "Удалён по ошибке")

	var inFlight bool
	res := s.race(func(ctx context.Context) func() {
		// «восстановление клиента в полёте»: те же блокировки и UPDATE, что в Repo.RestoreClient.
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM clients WHERE id = $1::uuid FOR UPDATE`, c.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE clients SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1::uuid`, c.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM equipment WHERE client_id = $1::uuid AND deleted_with_client ORDER BY id FOR NO KEY UPDATE`, c.ID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `
			UPDATE equipment SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, deleted_with_client = false
			WHERE client_id = $1::uuid AND deleted_with_client`, c.ID)
		s.Require().NoError(err)
		inFlight = true

		// операции с единицей, ушедшей с клиентом, отвечают сразу — не ждут и не видят
		// незафиксированного восстановления.
		type quick struct {
			status int
			raw    []byte
		}
		for name, req := range map[string]func() (int, []byte){
			"в заказ": func() (int, []byte) { return s.addUnit(s.admin, second.ID, with) },
		} {
			done := make(chan quick, 1)
			go func() {
				st, raw := req()
				done <- quick{st, raw}
			}()
			select {
			case q := <-done:
				s.Equal(http.StatusBadRequest, q.status, "%s: %s", name, q.raw)
				s.Contains(string(q.raw), "not_found", name)
			case <-time.After(15 * time.Second):
				s.FailNow("операция с удалённой единицей ждёт блокировку восстановления клиента", name)
			}
		}
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.restoreUnit(s.admin, alone) })
	s.Require().True(inFlight)
	s.Require().Equal(http.StatusOK, res.status, "восстановление единицы дождалось клиента: %s", res.raw)
	s.Equal(liveState, s.unitState(alone))
	s.Equal(liveState, s.unitState(with))
	s.Equal("false||", s.clientState(c.ID))

	// после фиксации единица действует — операции проходят.
	st, raw := s.addUnit(s.admin, second.ID, with)
	s.Equal(http.StatusCreated, st, string(raw))
}
