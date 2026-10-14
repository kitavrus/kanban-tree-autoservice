//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// CarOrderLiveSuite (§TD-8): новый заказ по госномеру — поиск автомобиля
// GET /api/v1/equipment?plate_number= и атомарное создание POST /api/v1/tasks с
// plate_number + equipment_id | new_equipment (+ new_client). Реальный бинарь + TCP +
// реальный логин; auth-стабы запрещены.
type CarOrderLiveSuite struct {
	testutil.LiveSuite
	admin    *testutil.Browser
	executor *testutil.Browser
	adminID  string
	column   string
	carType  string
}

func TestCarOrderLiveSuite(t *testing.T) {
	suite.Run(t, &CarOrderLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *CarOrderLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.adminID = s.SeedUser("car-admin", "password1", "Авто Админ", "admin", true)
	s.SeedUser("car-exec", "password1", "Авто Мастер", "executor", true)
	s.admin = s.NewBrowser()
	s.admin.Login("car-admin", "password1")
	s.executor = s.NewBrowser()
	s.executor.Login("car-exec", "password1")
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT id FROM columns WHERE name = 'Приёмка'`).Scan(&s.column))
	s.carType = s.newType(true)
}

// ---- wire-типы (1:1 с clients.EquipmentLookupResponse / board.TaskMutationResponse) ----

type lookupItemDTO struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	ClientName  string `json:"client_name"`
	TypeID      string `json:"type_id"`
	TypeName    string `json:"type_name"`
	PlateNumber string `json:"plate_number"`
	Brand       string `json:"brand"`
	Model       string `json:"model"`
	Serial      string `json:"serial"`
}

type lookupListDTO struct {
	Items  []lookupItemDTO `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type carTaskDTO struct {
	Task struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		ClientID string `json:"client_id"`
		ColumnID string `json:"column_id"`
	} `json:"task"`
}

// ---- фикстуры ----

func (s *CarOrderLiveSuite) newType(active bool) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO equipment_types (name, is_active) VALUES ('Легковой ' || gen_random_uuid(), $1) RETURNING id`, active).Scan(&id))
	return id
}

func (s *CarOrderLiveSuite) newClient(name string) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients", map[string]any{
		"company_name": name, "phones": []map[string]string{{"phone": "+7 999 100-20-30", "note": ""}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var c struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &c))
	return c.ID
}

func (s *CarOrderLiveSuite) newCar(clientID, plate, brand, model string) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients/"+clientID+"/equipment", map[string]any{
		"type_id": s.carType, "plate_number": plate, "brand": brand, "model": model,
		"sku": "", "serial": "", "specs": []any{}, "note": "",
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var e struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &e))
	return e.ID
}

func (s *CarOrderLiveSuite) softDeleteCar(id string) {
	_, err := s.Pool.Exec(context.Background(),
		`UPDATE equipment SET deleted_at = now(), deleted_by = $2, delete_reason = 'тест' WHERE id = $1`, id, s.adminID)
	s.Require().NoError(err)
}

func (s *CarOrderLiveSuite) lookup(b *testutil.Browser, plate string) (int, lookupListDTO, []byte) {
	st, raw := b.Do(http.MethodGet, "/api/v1/equipment?plate_number="+url.QueryEscape(plate), nil)
	var l lookupListDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	}
	return st, l, raw
}

func (s *CarOrderLiveSuite) createTask(b *testutil.Browser, body map[string]any) (int, carTaskDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/tasks", body)
	var t carTaskDTO
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	}
	return st, t, raw
}

func (s *CarOrderLiveSuite) fields(raw []byte) map[string]string {
	var e apiErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	s.Require().Equal("VALIDATION", e.Code, string(raw))
	s.Require().NotNil(e.Details, string(raw))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *CarOrderLiveSuite) count(sql string, args ...any) int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(), sql, args...).Scan(&n))
	return n
}

func (s *CarOrderLiveSuite) taskEquipment(taskID int64) []string {
	rows, err := s.Pool.Query(context.Background(), `SELECT equipment_id::text FROM task_equipment WHERE task_id = $1`, taskID)
	s.Require().NoError(err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		s.Require().NoError(rows.Scan(&id))
		out = append(out, id)
	}
	return out
}

// rowCounts — снимок числа строк, которые может создать POST /tasks по автомобилю.
func (s *CarOrderLiveSuite) rowCounts() [5]int {
	return [5]int{
		s.count(`SELECT count(*) FROM clients`),
		s.count(`SELECT count(*) FROM client_phones`),
		s.count(`SELECT count(*) FROM equipment`),
		s.count(`SELECT count(*) FROM tasks`),
		s.count(`SELECT count(*) FROM task_equipment`),
	}
}

// ---- поиск по госномеру ----

func (s *CarOrderLiveSuite) TestLookup_CaseSpaceInsensitive_DeletedExcluded_MultipleMatches() {
	a := s.newClient("Петрова Н. С.")
	b := s.newClient("Сидоров А. А.")
	carA := s.newCar(a, "о718мр 199", "Hyundai", "Solaris")
	carB := s.newCar(b, "О718-МР199", "Kia", "Rio") // перепродан: тот же номер у другого клиента
	gone := s.newCar(b, "О718МР 199", "Lada", "Vesta")
	s.softDeleteCar(gone)
	s.newCar(a, "О718МР 777", "Lada", "Granta") // другой номер — не совпадение

	st, l, raw := s.lookup(s.admin, " о 718-мр199 ")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(2, l.Total)
	s.Equal(20, l.Limit)
	s.Equal(0, l.Offset)
	got := map[string]lookupItemDTO{}
	for _, it := range l.Items {
		got[it.ID] = it
	}
	s.Require().Len(got, 2, string(raw))
	s.Require().Contains(got, carA)
	s.Require().Contains(got, carB)
	s.NotContains(got, gone, "удалённый автомобиль не находится")
	s.Equal(a, got[carA].ClientID)
	s.Equal("Петрова Н. С.", got[carA].ClientName)
	s.Equal("О718МР 199", got[carA].PlateNumber)
	s.Equal("Hyundai", got[carA].Brand)
	s.Equal("Solaris", got[carA].Model)
	s.Equal(s.carType, got[carA].TypeID)
	s.NotEmpty(got[carA].TypeName)

	// исполнитель тоже ищет (диалог «Новый заказ» доступен и ему, D19).
	st, l, raw = s.lookup(s.executor, "О718МР199")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(2, l.Total)

	// точное совпадение, а не подстрока.
	st, l, _ = s.lookup(s.admin, "О718МР19")
	s.Require().Equal(http.StatusOK, st)
	s.Equal(0, l.Total)
	s.Empty(l.Items)
	s.NotNil(l.Items, "items — всегда массив")

	st, _, raw = s.lookup(s.admin, " - ")
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["plate_number"])
}

// ---- создание заказа по автомобилю ----

func (s *CarOrderLiveSuite) TestCreate_ExistingCar_ClientAuto_TitleGenerated_EquipmentAttached() {
	owner := s.newClient("Владелец Существующего")
	other := s.newClient("Чужой Клиент")
	car := s.newCar(owner, "Т111ТТ 77", "Hyundai", "Solaris")

	st, t, raw := s.createTask(s.admin, map[string]any{
		"plate_number": "т111-тт77", "equipment_id": car, "column_id": s.column,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(owner, t.Task.ClientID, "клиент — владелец автомобиля")
	s.Equal("Hyundai Solaris Т111ТТ 77", t.Task.Title)
	s.Equal(s.column, t.Task.ColumnID)
	s.Equal([]string{car}, s.taskEquipment(t.Task.ID), "автомобиль сразу в заказе")

	// client_id другого клиента — автомобиль ему не принадлежит.
	st, _, raw = s.createTask(s.admin, map[string]any{
		"plate_number": "Т111ТТ77", "equipment_id": car, "client_id": other, "column_id": s.column,
	})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("foreign_client", s.fields(raw)["equipment_id"])

	// госномер не тот, что у найденного автомобиля.
	st, _, raw = s.createTask(s.admin, map[string]any{
		"plate_number": "А000АА 77", "equipment_id": car, "column_id": s.column,
	})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("plate_mismatch", s.fields(raw)["equipment_id"])

	// удалённый автомобиль заказу не назначить.
	gone := s.newCar(owner, "Т222ТТ 77", "Kia", "Rio")
	s.softDeleteCar(gone)
	st, _, raw = s.createTask(s.admin, map[string]any{
		"plate_number": "Т222ТТ 77", "equipment_id": gone, "column_id": s.column,
	})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["equipment_id"])
}

func (s *CarOrderLiveSuite) TestCreate_NewCar_ExistingClient() {
	client := s.newClient("Клиент Нового Авто")
	st, t, raw := s.createTask(s.executor, map[string]any{
		"plate_number": " в333вв  50 ", "client_id": client, "column_id": s.column,
		"new_equipment": map[string]any{"type_id": s.carType, "brand": " Toyota ", "model": "Camry", "serial": "JTNB11HK103456789"},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(client, t.Task.ClientID)
	s.Equal("Toyota Camry В333ВВ 50", t.Task.Title)

	te := s.taskEquipment(t.Task.ID)
	s.Require().Len(te, 1)
	var plate, owner, serial, typeID string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT plate_number, client_id::text, serial, type_id::text FROM equipment WHERE id = $1`, te[0]).Scan(&plate, &owner, &serial, &typeID))
	s.Equal("В333ВВ 50", plate, "госномер сохранён нормализованным")
	s.Equal(client, owner)
	s.Equal("JTNB11HK103456789", serial)
	s.Equal(s.carType, typeID)

	// новый автомобиль теперь находится поиском.
	_, l, _ := s.lookup(s.admin, "В333ВВ50")
	s.Equal(1, l.Total)
}

func (s *CarOrderLiveSuite) TestCreate_NewCar_NewClient() {
	before := s.rowCounts()
	st, t, raw := s.createTask(s.admin, map[string]any{
		"plate_number": "е444ее 750", "column_id": s.column,
		"new_equipment": map[string]any{"type_id": s.carType, "brand": "", "model": "Logan"},
		"new_client":    map[string]any{"company_name": " Кузнецов Пётр ", "phones": []map[string]string{{"phone": "8 (912) 000-11-22"}}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("Logan Е444ЕЕ 750", t.Task.Title, "пустая марка пропускается")
	after := s.rowCounts()
	s.Equal([5]int{before[0] + 1, before[1] + 1, before[2] + 1, before[3] + 1, before[4] + 1}, after)

	var name, phone string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT c.company_name, p.phone FROM clients c JOIN client_phones p ON p.client_id = c.id WHERE c.id = $1`, t.Task.ClientID).Scan(&name, &phone))
	s.Equal("Кузнецов Пётр", name)
	s.Equal("8 (912) 000-11-22", phone)
	te := s.taskEquipment(t.Task.ID)
	s.Require().Len(te, 1)
	s.Equal(1, s.count(`SELECT count(*) FROM equipment WHERE id = $1 AND client_id = $2`, te[0], t.Task.ClientID))

	// исполнитель тоже заводит нового клиента (как POST /clients) — клиент, автомобиль и
	// заказ создаются атомарно.
	before = s.rowCounts()
	st, t, raw = s.createTask(s.executor, map[string]any{
		"plate_number": "Е555ЕЕ 750", "column_id": s.column,
		"new_equipment": map[string]any{"type_id": s.carType, "model": "Logan"},
		"new_client":    map[string]any{"company_name": "Исполнитель Создал", "phones": []map[string]string{{"phone": "123"}}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	after = s.rowCounts()
	s.Equal([5]int{before[0] + 1, before[1] + 1, before[2] + 1, before[3] + 1, before[4] + 1}, after)
	s.Equal(1, s.count(`SELECT count(*) FROM clients WHERE id = $1 AND company_name = 'Исполнитель Создал'`, t.Task.ClientID))
	te = s.taskEquipment(t.Task.ID)
	s.Require().Len(te, 1)
	s.Equal(1, s.count(`SELECT count(*) FROM equipment WHERE id = $1 AND client_id = $2`, te[0], t.Task.ClientID))
}

// TestExecutor_CreatesClient_ViaClientsAPI — POST /api/v1/clients исполнителю разрешён (201);
// правка и удаление клиента по-прежнему только admin (403).
func (s *CarOrderLiveSuite) TestExecutor_CreatesClient_ViaClientsAPI() {
	st, raw := s.executor.Do(http.MethodPost, "/api/v1/clients", map[string]any{
		"company_name": "Клиент От Исполнителя", "phones": []map[string]string{{"phone": "+7 999 555-44-33", "note": ""}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var c struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	s.Require().NoError(json.Unmarshal(raw, &c))
	s.Equal(1, s.count(`SELECT count(*) FROM clients WHERE id = $1 AND company_name = 'Клиент От Исполнителя'`, c.ID))

	st, raw = s.executor.Do(http.MethodPut, "/api/v1/clients/"+c.ID, map[string]any{
		"company_name": "Переименован", "version": c.Version, "phones": []map[string]string{{"phone": "+7 999 555-44-33", "note": ""}},
	})
	s.Equal(http.StatusForbidden, st, string(raw))
	st, raw = s.executor.Do(http.MethodDelete, "/api/v1/clients/"+c.ID, map[string]any{"reason": "тест"})
	s.Equal(http.StatusForbidden, st, string(raw))

	// валидация — общая с admin: пустое имя → 400.
	st, raw = s.executor.Do(http.MethodPost, "/api/v1/clients", map[string]any{
		"company_name": " ", "phones": []map[string]string{{"phone": "+7 999 555-44-33", "note": ""}},
	})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["company_name"])
}

// TestLookup_ByClientID — GET /api/v1/equipment?client_id=: автомобили клиента без удалённых,
// конверт {items,total,limit,offset}; client_id + plate_number — фильтр по AND; ни одного
// параметра → 400 plate_number/client_id: required; битый client_id → 400 invalid.
func (s *CarOrderLiveSuite) TestLookup_ByClientID() {
	owner := s.newClient("Владелец Двух Авто")
	other := s.newClient("Другой Владелец")
	car1 := s.newCar(owner, "А101АА 77", "Kia", "Rio")
	car2 := s.newCar(owner, "А102АА 77", "Kia", "Ceed")
	gone := s.newCar(owner, "А103АА 77", "Kia", "Soul")
	s.newCar(other, "А104АА 77", "Lada", "Vesta")
	s.softDeleteCar(gone)

	for _, b := range []*testutil.Browser{s.admin, s.executor} {
		st, raw := b.Do(http.MethodGet, "/api/v1/equipment?client_id="+owner, nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var l lookupListDTO
		s.Require().NoError(json.Unmarshal(raw, &l))
		s.Equal(2, l.Total)
		s.Equal(20, l.Limit)
		s.Equal(0, l.Offset)
		ids := []string{}
		for _, it := range l.Items {
			ids = append(ids, it.ID)
			s.Equal(owner, it.ClientID)
			s.Equal("Владелец Двух Авто", it.ClientName)
		}
		s.ElementsMatch([]string{car1, car2}, ids)
		s.Contains(string(raw), `"items":`)
	}

	// AND с госномером: свой — 1, чужой — 0.
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment?client_id="+owner+"&plate_number="+url.QueryEscape("а101аа77"), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l lookupListDTO
	s.Require().NoError(json.Unmarshal(raw, &l))
	s.Require().Equal(1, l.Total)
	s.Equal(car1, l.Items[0].ID)
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment?client_id="+other+"&plate_number="+url.QueryEscape("А101АА 77"), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &l))
	s.Equal(0, l.Total)
	s.NotNil(l.Items)

	// клиент без автомобилей — пустой конверт.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment?client_id="+s.newClient("Безлошадный"), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"items":[]`)
	s.Contains(string(raw), `"total":0`)

	// валидация.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment", nil)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	f := s.fields(raw)
	s.Equal("required", f["plate_number"])
	s.Equal("required", f["q"])
	s.Equal("required", f["client_id"])
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment?client_id=not-a-uuid", nil)
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["client_id"])
}

// TestLookup_Fragment — GET /api/v1/equipment?q=: подстрока госномера без учёта регистра/
// пробелов/дефисов, без удалённых; совпадения с начала номера — первыми; limit соблюдается;
// q «из одних пробелов/дефисов» → 400 required; %/_ — обычные символы (не LIKE-шаблон).
func (s *CarOrderLiveSuite) TestLookup_Fragment() {
	owner := s.newClient("Автодополнение Клиент")
	mid := s.newCar(owner, "Х9Ч718ЧЧ 99", "Lada", "Granta") // «718» не в начале
	head := s.newCar(owner, "Ч718ЧЧ 199", "Lada", "Vesta")  // «Ч718» с начала
	gone := s.newCar(owner, "ЧЧ718Ч 77", "Lada", "Niva")    // удалён — не находится
	s.newCar(owner, "Ш555ШШ 77", "Lada", "Largus")          // не совпадает
	s.softDeleteCar(gone)

	get := func(b *testutil.Browser, q string) (int, lookupListDTO, []byte) {
		st, raw := b.Do(http.MethodGet, "/api/v1/equipment?q="+url.QueryEscape(q)+"&limit=10", nil)
		var l lookupListDTO
		if st == http.StatusOK {
			s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
		}
		return st, l, raw
	}

	st, l, raw := get(s.executor, "ч7-18")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Equal(2, l.Total, string(raw))
	s.Equal(10, l.Limit)
	s.Equal([]string{head, mid}, []string{l.Items[0].ID, l.Items[1].ID}, "с начала номера — первым")
	s.Equal("Автодополнение Клиент", l.Items[0].ClientName)

	// limit и total.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment?q=718&limit=1", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(json.Unmarshal(raw, &l))
	s.True(l.Total >= 2)
	s.Len(l.Items, 1)

	// LIKE-метасимволы — не шаблон.
	_, l, _ = get(s.admin, "%")
	s.Equal(0, l.Total)

	// пустой после нормализации фрагмент → 400.
	st, _, raw = get(s.admin, " - ")
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["q"])
}

// TestCreate_ValidationErrors_NoPartialRows — ошибки формы и отказы ПОСЛЕ создания клиента/
// автомобиля внутри транзакции: ни одной строки не остаётся (атомарность).
func (s *CarOrderLiveSuite) TestCreate_ValidationErrors_NoPartialRows() {
	newClient := map[string]any{"company_name": "Атомарный Клиент", "phones": []map[string]string{{"phone": "+7 900 000-00-00"}}}
	newCar := func(model string) map[string]any {
		return map[string]any{"type_id": s.carType, "brand": "Ford", "model": model}
	}

	cases := []struct {
		name  string
		body  map[string]any
		field string
		msg   string
	}{
		{"нет госномера", map[string]any{"column_id": s.column, "new_equipment": newCar("Focus"), "new_client": newClient}, "plate_number", "required"},
		{"нет модели", map[string]any{"plate_number": "К100КК 77", "column_id": s.column, "new_equipment": newCar(" "), "new_client": newClient}, "new_equipment.model", "required"},
		{"плохой госномер", map[string]any{"plate_number": "К100*КК", "column_id": s.column, "new_equipment": newCar("Focus"), "new_client": newClient}, "plate_number", "invalid_format"},
		{"телефон нового клиента", map[string]any{"plate_number": "К100КК 77", "column_id": s.column, "new_equipment": newCar("Focus"),
			"new_client": map[string]any{"company_name": "Без Телефона", "phones": []map[string]string{{"phone": "нет"}}}}, "new_client.phones[0].phone", "invalid_format"},
		{"ни автомобиля, ни нового", map[string]any{"plate_number": "К100КК 77", "column_id": s.column, "client_id": s.newClient("Пустой Заказ")}, "equipment_id", "required"},
		// отказы ВНУТРИ транзакции — клиент уже вставлен, всё откатывается:
		{"архивный тип", map[string]any{"plate_number": "К100КК 77", "column_id": s.column, "new_client": newClient,
			"new_equipment": map[string]any{"type_id": s.newType(false), "model": "Focus"}}, "new_equipment.type_id", "archived"},
		{"нет ответственного", map[string]any{"plate_number": "К100КК 77", "column_id": s.column, "new_client": newClient,
			"new_equipment": newCar("Focus"), "assignee_id": "00000000-0000-0000-0000-000000000001"}, "assignee_id", "not_found"},
	}
	before := s.rowCounts() // newClient("Пустой Заказ")/newType выше — до снимка
	for _, c := range cases {
		st, _, raw := s.createTask(s.admin, c.body)
		s.Require().Equal(http.StatusBadRequest, st, "%s: %s", c.name, raw)
		s.Equal(c.msg, s.fields(raw)[c.field], "%s: %s", c.name, raw)
	}
	s.Equal(before, s.rowCounts(), "ни клиента, ни автомобиля, ни заказа не осталось")

	// строгий разбор тела: неизвестное поле во вложенном объекте — 400 BAD_REQUEST.
	st, _, raw := s.createTask(s.admin, map[string]any{
		"plate_number": "К100КК 77", "column_id": s.column, "client_id": s.newClient("Строгий"),
		"new_equipment": map[string]any{"type_id": s.carType, "model": "Focus", "color": "red"},
	})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Contains(string(raw), "BAD_REQUEST")
}
