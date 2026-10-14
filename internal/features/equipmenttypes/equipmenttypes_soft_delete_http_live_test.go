//go:build integration

package equipmenttypes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление типов оборудования (docs/features/soft-delete-catalog/draft-spec.md §4.4,
// §6.7, §6.8, §11): TS-028, TS-029 (тип), TS-030 (тип), TS-033, TS-061, причина удаления,
// служебная уборка и гонки §3. Сьюта — TypesLiveSuite (§TD-8: боевой бинарь, реальный логин);
// клиент и единицы оборудования создаются через API того же сервера.

// ---- wire-типы (§TD-6.2) ----

type sdErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type sdDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type sdUnitDTO struct {
	ID            string `json:"id"`
	TypeID        string `json:"type_id"`
	TypeName      string `json:"type_name"`
	TypeIsActive  bool   `json:"type_is_active"`
	TypeIsDeleted bool   `json:"type_is_deleted"`
}

// ---- помощники ----

func sdMark() string { return uuid.NewString()[:8] }

func (s *TypesLiveSuite) sdErr(raw []byte) sdErrDTO {
	var e sdErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *TypesLiveSuite) sdScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

// typeState — «удалён|кто|причина|is_active» типа; нет строки — "".
func (s *TypesLiveSuite) typeState(id string) string {
	var v string
	err := s.Pool.QueryRow(context.Background(), `
		SELECT (t.deleted_at IS NOT NULL)::text || '|' || coalesce(u.login, '') || '|' || coalesce(t.delete_reason, '') || '|' || t.is_active::text
		FROM equipment_types t LEFT JOIN users u ON u.id = t.deleted_by WHERE t.id = $1::uuid`, id).Scan(&v)
	if err != nil {
		return ""
	}
	return v
}

const (
	typeLiveActive   = "false|||true"
	typeLiveArchived = "false|||false"
)

func (s *TypesLiveSuite) deleteType(b *testutil.Browser, id string, reason any) (int, sdDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, "/api/v1/equipment-types/"+id, body)
	var out sdDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *TypesLiveSuite) mustDeleteType(id, reason string) {
	st, _, raw := s.deleteType(s.admin, id, reason)
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *TypesLiveSuite) restoreType(b *testutil.Browser, id string) (int, []byte) {
	return b.Do(http.MethodPost, "/api/v1/equipment-types/"+id+"/restore", nil)
}

func (s *TypesLiveSuite) getType(id string) (int, typeDTO, []byte) {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment-types/"+id, nil)
	var t typeDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	}
	return st, t, raw
}

// listed — тип из списка GET /api/v1/equipment-types (с архивом) по id; ok = false — нет в списке.
func (s *TypesLiveSuite) listed(id string) (typeDTO, bool) {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment-types?include_archived=true&limit=500", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []typeDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l))
	for _, t := range l.Items {
		if t.ID == id {
			return t, true
		}
	}
	return typeDTO{}, false
}

func (s *TypesLiveSuite) sdClient(name string) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients", map[string]any{
		"company_name": name, "phones": []map[string]any{{"phone": "+7 700 000-00-00"}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var c struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &c))
	return c.ID
}

func (s *TypesLiveSuite) unitReq(method, path, typeID, model string) (int, sdUnitDTO, []byte) {
	var body any
	if method != http.MethodGet {
		body = map[string]any{"type_id": typeID, "model": model}
	}
	st, raw := s.admin.Do(method, path, body)
	var u sdUnitDTO
	if st == http.StatusOK || st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &u), string(raw))
	}
	return st, u, raw
}

func (s *TypesLiveSuite) sdUnit(clientID, typeID, model string) string {
	st, u, raw := s.unitReq(http.MethodPost, "/api/v1/clients/"+clientID+"/equipment", typeID, model)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return u.ID
}

func (s *TypesLiveSuite) sdExecutor(login string) *testutil.Browser {
	s.SeedUser(login, "password1", "Исполнитель "+login, "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b
}

// ---- TS-028 ----

// TS-028 (DS-009): архив и удаление типа независимы — архивный тип удаляется и
// восстанавливается архивным, активный — активным; список с фильтром «архив» удалённых не
// содержит; удалённый тип для обычных адресов не существует (GET, PUT, повтор удаления — 404,
// DS-026). Восстановление: 200 {id}; повтор — 409 NOT_DELETED; типа нет — 404.
func (s *TypesLiveSuite) TestTS028_TypeArchiveAndDeleteIndependent() {
	m := sdMark()
	for _, archived := range []bool{true, false} {
		live := typeLiveActive
		tp := s.newType("SD-028 Тип " + m + map[bool]string{true: " архивный", false: " активный"}[archived])
		if archived {
			live = typeLiveArchived
			st, raw := s.put(s.admin, tp.ID, card(tp, map[string]any{"is_active": false}))
			s.Require().Equal(http.StatusOK, st, string(raw))
		}
		_, ok := s.listed(tp.ID)
		s.Require().True(ok, "до удаления тип в списке с архивом")

		st, del, raw := s.deleteType(s.admin, tp.ID, "  Создан по ошибке ")
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Equal(tp.ID, del.ID)
		s.WithinDuration(time.Now(), del.DeletedAt, time.Minute)
		s.Equal("true|catalog-admin|Создан по ошибке|"+live[len("false|||"):], s.typeState(tp.ID), "удаление архив не меняет, причина обрезана")
		_, ok = s.listed(tp.ID)
		s.False(ok, "список с фильтром «архив» удалённых не содержит")

		st, _, raw = s.getType(tp.ID)
		s.Equal(http.StatusNotFound, st, string(raw))
		s.Equal("EQUIPMENT_TYPE_NOT_FOUND", s.sdErr(raw).Code)
		st, raw = s.put(s.admin, tp.ID, card(tp, map[string]any{"name": "SD-028 Правка " + m}))
		s.Equal(http.StatusNotFound, st, string(raw))
		st, _, raw = s.deleteType(s.admin, tp.ID, "Повтор")
		s.Equal(http.StatusNotFound, st, string(raw))
		s.Equal("EQUIPMENT_TYPE_NOT_FOUND", s.sdErr(raw).Code)

		st, raw = s.restoreType(s.admin, tp.ID)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.JSONEq(`{"id":"`+tp.ID+`"}`, string(raw))
		s.Equal(live, s.typeState(tp.ID), "восстановлен с прежним признаком архива, отметка стёрта")
		st, raw = s.restoreType(s.admin, tp.ID)
		s.Equal(http.StatusConflict, st, string(raw))
		e := s.sdErr(raw)
		s.Equal("NOT_DELETED", e.Code)
		s.Equal("Запись уже восстановлена", e.Message)
	}
	for _, missing := range []string{"00000000-0000-0000-0000-000000000000", "not-uuid"} {
		st, raw := s.restoreType(s.admin, missing)
		s.Equal(http.StatusNotFound, st, "%s: %s", missing, raw)
		s.Equal("EQUIPMENT_TYPE_NOT_FOUND", s.sdErr(raw).Code)
	}
}

// ---- TS-029 (тип оборудования) ----

// TS-029 / DS-017: название удалённого типа свободно — создаётся новый с тем же названием;
// восстановление старого → 409 NAME_TAKEN, тип остаётся удалённым; после переименования нового
// — 200. NAME_TAKEN — и когда название занял АРХИВНЫЙ действующий тип. Коды создания
// сохраняются и считаются только по действующим: название занято активным → 409
// EQUIPMENT_TYPE_NAME_TAKEN, архивным → 409 EQUIPMENT_TYPE_NAME_ARCHIVED — в том числе когда
// рядом лежит удалённый АКТИВНЫЙ тёзка (поиск строки-конфликта без условия «не удалён» ответил
// бы TAKEN). Негатив: два действующих типа с одним названием невозможны.
func (s *TypesLiveSuite) TestTS029_TypeNameFreeAfterDelete() {
	m := sdMark()
	name := "SD-029 Лазер " + m
	old := s.newType(name) // активный — и удаляется активным
	s.mustDeleteType(old.ID, "Дубль")

	st, fresh, raw := s.create(s.admin, map[string]any{"name": "  " + strings.ToUpper(name) + " "})
	s.Require().Equal(http.StatusCreated, st, "название удалённого типа свободно: %s", raw)
	st, _, raw = s.create(s.admin, map[string]any{"name": name})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("EQUIPMENT_TYPE_NAME_TAKEN", s.sdErr(raw).Code, "два действующих типа с одним названием невозможны")

	st, raw = s.restoreType(s.admin, old.ID)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	e := s.sdErr(raw)
	s.Equal("NAME_TAKEN", e.Code)
	s.Equal("Название занято — переименуйте действующую запись", e.Message)
	s.Equal("true|catalog-admin|Дубль|true", s.typeState(old.ID), "тип остался удалённым")

	// название занял архивный действующий тип: создание → NAME_ARCHIVED, восстановление → NAME_TAKEN.
	st, raw = s.put(s.admin, fresh.ID, card(fresh, map[string]any{"is_active": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.create(s.admin, map[string]any{"name": name})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("EQUIPMENT_TYPE_NAME_ARCHIVED", s.sdErr(raw).Code, "строка-конфликт — действующий архивный тип, не удалённый активный")
	st, raw = s.restoreType(s.admin, old.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("NAME_TAKEN", s.sdErr(raw).Code)

	// переименование в занятое название — as-is 409; после переименования нового — восстановление.
	spare := s.newType("SD-029 Запасной " + m)
	st, raw = s.put(s.admin, spare.ID, card(spare, map[string]any{"name": name}))
	s.Equal(http.StatusConflict, st, string(raw))
	st, raw = s.put(s.admin, fresh.ID, map[string]any{"name": "SD-029 Переименован " + m, "spec_keys": []string{}, "is_active": false})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.restoreType(s.admin, old.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(typeLiveActive, s.typeState(old.ID))
}

// ---- TS-030 (тип), TS-061 ----

// TS-061 + TS-030 (DS-014): equipment_count типа считает только действующие единицы — у типа с
// двумя единицами после мягкого удаления одной счётчик 1 (счёт всех строк дал бы 2), у типа с
// единственной удалённой единицей — 0; такой тип удаляется (200, без EQUIPMENT_TYPE_IN_USE), и
// тип с действующими единицами — тоже. Существующие единицы не меняются; удалённый тип у них
// показан с type_is_deleted = true (список клиента и чтение единицы). Выбор удалённого типа: при
// создании единицы и при СМЕНЕ типа → 400 type_id: not_found; сохранение единицы без смены уже
// удалённого типа проходит. Негатив: после восстановления единицы и типа счётчик снова 1, у
// действующего типа type_is_deleted = false.
func (s *TypesLiveSuite) TestTS061_EquipmentCountAndDeletedTypeRefs() {
	m := sdMark()
	clientID := s.sdClient("SD-061 Клиент " + m)
	lonely := s.newType("SD-061 С одной единицей " + m)
	pair := s.newType("SD-061 С двумя единицами " + m)
	other := s.newType("SD-061 Действующий " + m)
	only := s.sdUnit(clientID, lonely.ID, "M-061-единственная")
	first := s.sdUnit(clientID, pair.ID, "M-061-первая")
	second := s.sdUnit(clientID, pair.ID, "M-061-вторая")
	elsewhere := s.sdUnit(clientID, other.ID, "M-061-другой-тип")

	count := func(id string) int {
		_, t, raw := s.getType(id)
		inList, ok := s.listed(id)
		s.Require().True(ok, string(raw))
		s.Equal(t.EquipmentCount, inList.EquipmentCount, "счётчик в списке и в карточке совпадают")
		return t.EquipmentCount
	}
	s.Equal(1, count(lonely.ID))
	s.Equal(2, count(pair.ID))

	delUnit := func(id string) {
		st, raw := s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+id, map[string]any{"reason": "Списана"})
		s.Require().Equal(http.StatusOK, st, string(raw))
	}
	delUnit(only)
	delUnit(second)
	s.Equal(0, count(lonely.ID), "единственная единица мягко удалена — счётчик 0")
	s.Equal(1, count(pair.ID), "счётчик — только действующие единицы")

	// тип с единственной мягко удалённой единицей и тип с действующей единицей удаляются.
	for _, id := range []string{lonely.ID, pair.ID} {
		st, _, raw := s.deleteType(s.admin, id, "Тип не нужен")
		s.Require().Equal(http.StatusOK, st, string(raw))
	}
	s.Equal("3", s.sdScalar(`SELECT count(*)::text FROM equipment WHERE id = ANY($1::uuid[]) AND type_id = ANY($2::uuid[])`,
		[]string{only, first, second}, []string{lonely.ID, pair.ID}), "строки единиц и их ссылки на тип не изменились")

	// пометка у единицы: чтение по идентификатору и список клиента.
	st, u, raw := s.unitReq(http.MethodGet, "/api/v1/equipment/"+first, "", "")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(u.TypeIsDeleted)
	s.Equal(pair.ID, u.TypeID)
	s.Equal("SD-061 С двумя единицами "+m, u.TypeName, "название удалённого типа по-прежнему видно")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+clientID+"/equipment", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var units struct {
		Items []sdUnitDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &units))
	flags := map[string]bool{}
	for _, it := range units.Items {
		flags[it.ID] = it.TypeIsDeleted
	}
	s.Equal(map[string]bool{first: true, elsewhere: false}, flags, "в списке клиента — действующие единицы, пометка только у удалённого типа")

	// выбор удалённого типа: новая единица и смена типа → 400; без смены — сохраняется.
	st, _, raw = s.unitReq(http.MethodPost, "/api/v1/clients/"+clientID+"/equipment", pair.ID, "M-061-новая")
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"type_id": "not_found"}, s.fields(raw))
	st, _, raw = s.unitReq(http.MethodPut, "/api/v1/equipment/"+elsewhere, pair.ID, "M-061-другой-тип")
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"type_id": "not_found"}, s.fields(raw))
	st, u, raw = s.unitReq(http.MethodPut, "/api/v1/equipment/"+first, pair.ID, "M-061-первая-правка")
	s.Require().Equal(http.StatusOK, st, "сохранение без смены уже удалённого типа: %s", raw)
	s.True(u.TypeIsDeleted)
	// с удалённого типа на действующий перейти можно.
	st, u, raw = s.unitReq(http.MethodPut, "/api/v1/equipment/"+first, other.ID, "M-061-первая-правка")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.False(u.TypeIsDeleted)

	// негатив: единица и тип восстановлены — счётчик снова 1, у единицы тип без пометки.
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/equipment/"+only+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, u, raw = s.unitReq(http.MethodGet, "/api/v1/equipment/"+only, "", "")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(u.TypeIsDeleted, "единица восстановлена, её тип по-прежнему в удалённых")
	st, raw = s.restoreType(s.admin, lonely.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(1, count(lonely.ID))
	_, u, _ = s.unitReq(http.MethodGet, "/api/v1/equipment/"+only, "", "")
	s.False(u.TypeIsDeleted)
}

// ---- TS-033 ----

// TS-033 (DS-003): удаление и восстановление типа — только администратору: исполнитель → 403
// FORBIDDEN «Нет доступа» ДО поиска записи и проверки причины (существующий, удалённый,
// несуществующий и битый идентификатор, с причиной и без); тип не изменился. Негатив: без
// сессии → 401.
func (s *TypesLiveSuite) TestTS033_TypeSoftDeleteAdminOnly() {
	m := sdMark()
	live := s.newType("SD-033 Тип " + m)
	gone := s.newType("SD-033 Удалённый " + m)
	s.mustDeleteType(gone.ID, "Фикстура")
	exec := s.sdExecutor("sd033-types-exec-" + m)
	anon := s.NewBrowser()

	for _, id := range []string{live.ID, gone.ID, "00000000-0000-0000-0000-000000000000", "not-uuid"} {
		for _, body := range []any{nil, map[string]any{"reason": "Попытка"}} {
			st, raw := exec.Do(http.MethodDelete, "/api/v1/equipment-types/"+id, body)
			s.Equal(http.StatusForbidden, st, "DELETE %s: %s", id, raw)
			e := s.sdErr(raw)
			s.Equal("FORBIDDEN", e.Code)
			s.Equal("Нет доступа", e.Message)
		}
		st, raw := s.restoreType(exec, id)
		s.Equal(http.StatusForbidden, st, "restore %s: %s", id, raw)
		s.Equal("Нет доступа", s.sdErr(raw).Message)

		st, _ = anon.Do(http.MethodDelete, "/api/v1/equipment-types/"+id, map[string]any{"reason": "Аноним"})
		s.Equal(http.StatusUnauthorized, st, id)
		st, _ = s.restoreType(anon, id)
		s.Equal(http.StatusUnauthorized, st, id)
	}
	s.Equal(typeLiveActive, s.typeState(live.ID), "тип не удалён")
	s.Equal("true|catalog-admin|Фикстура|true", s.typeState(gone.ID), "тип не восстановлен")
}

// ---- причина удаления (DS-020) ----

// Причина обязательна: нет тела, нет поля, пробелы → 400 reason: required; 501 символ →
// too_long; NUL → invalid_text; тип не удалён. Граница: 500 символов — допустимо.
func (s *TypesLiveSuite) TestTypeDeleteReasonValidation() {
	tp := s.newType("SD-reason Тип " + sdMark())
	for _, tc := range []struct {
		name string
		body any
		code string
	}{
		{"нет тела", nil, "required"},
		{"нет поля", map[string]any{"note": "x"}, "required"},
		{"null", map[string]any{"reason": nil}, "required"},
		{"пробелы", map[string]any{"reason": " \t\n "}, "required"},
		{"501 символ", map[string]any{"reason": strings.Repeat("я", 501)}, "too_long"},
		{"NUL", map[string]any{"reason": "Причина\u0000"}, "invalid_text"},
		{"не JSON", `{`, "required"},
	} {
		st, raw := s.admin.Do(http.MethodDelete, "/api/v1/equipment-types/"+tp.ID, tc.body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", tc.name, raw)
		s.Equal(tc.code, s.fields(raw)["reason"], tc.name)
	}
	s.Equal(typeLiveActive, s.typeState(tp.ID), "тип не удалён")

	long := strings.Repeat("я", 500)
	st, _, raw := s.deleteType(s.admin, tp.ID, long)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("true|catalog-admin|"+long+"|true", s.typeState(tp.ID))
}

// ---- служебная уборка (§6.9, DS-027, TS-F10) ----

// Вид equipment-types адреса DELETE /api/v1/test/purge/:kind/:id: действующий и мягко удалённый
// тип стираются физически; тип, на который ссылается единица (в том числе мягко удалённая), →
// 409 EQUIPMENT_TYPE_IN_USE (код остался только здесь) и остаётся; нет типа либо битый
// идентификатор → 404 NOT_FOUND. Негатив: исполнитель → 403.
func (s *TypesLiveSuite) TestPurge_EquipmentTypes() {
	m := sdMark()
	purge := func(b *testutil.Browser, id string) (int, []byte) {
		return b.Do(http.MethodDelete, "/api/v1/test/purge/equipment-types/"+id, nil)
	}
	held := s.newType("SD-purge С единицей " + m)
	unit := s.sdUnit(s.sdClient("SD-purge Клиент "+m), held.ID, "M-purge")
	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+unit, map[string]any{"reason": "Фикстура"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.mustDeleteType(held.ID, "Фикстура") // мягкое удаление типа и единицы ссылку не снимает

	exec := s.sdExecutor("sd-purge-types-exec-" + m)
	st, raw = purge(exec, held.ID)
	s.Equal(http.StatusForbidden, st, string(raw))

	st, raw = purge(s.admin, held.ID)
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("EQUIPMENT_TYPE_IN_USE", s.sdErr(raw).Code)
	s.NotEmpty(s.typeState(held.ID), "тип с единицей на месте")

	live := s.newType("SD-purge Свободный " + m)
	gone := s.newType("SD-purge Удалённый " + m)
	s.mustDeleteType(gone.ID, "Фикстура")
	for _, id := range []string{live.ID, gone.ID} {
		st, raw = purge(s.admin, id)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Empty(s.typeState(id), "строка типа физически удалена")
		st, raw = purge(s.admin, id)
		s.Equal(http.StatusNotFound, st, string(raw))
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
	}
	st, raw = purge(s.admin, "not-uuid")
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("NOT_FOUND", s.sdErr(raw).Code)
}

// ---- гонки (§3 «Конкурентность») ----

func (s *TypesLiveSuite) lockWaiters() int {
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
func (s *TypesLiveSuite) race(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) raceResult {
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

// holdTypeDelete — «удаление типа в полёте»: тот же UPDATE, что делает Repo.SoftDelete,
// транзакция не зафиксирована. Возвращает фиксацию.
func (s *TypesLiveSuite) holdTypeDelete(typeID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		adminID := s.sdScalar(`SELECT id::text FROM users WHERE login = 'catalog-admin'`)
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE equipment_types SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'Гонка' WHERE id = $1::uuid AND deleted_at IS NULL`,
			typeID, adminID)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 п. 5 (DS-014): удаление типа идёт — в это окно приходит создание единицы этого типа.
// Создание обязано дождаться удаления (строка типа FOR SHARE) и получить 400 type_id: not_found;
// единица не создана. Реализация, проверяющая тип до вставки без блокировки, создала бы единицу с
// новой ссылкой на удалённый тип.
func (s *TypesLiveSuite) TestRace_EquipmentCreateWaitsForTypeDelete() {
	m := sdMark()
	tp := s.newType("SD-race Тип-уходит " + m)
	clientID := s.sdClient("SD-race Клиент " + m)

	res := s.race(s.holdTypeDelete(tp.ID), func() (int, []byte) {
		st, _, raw := s.unitReq(http.MethodPost, "/api/v1/clients/"+clientID+"/equipment", tp.ID, "M-race-опоздала")
		return st, raw
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["type_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM equipment WHERE client_id = $1::uuid`, clientID), "единица не создана")
}

// §3 п. 5: то же для СМЕНЫ типа единицы — правка ждёт удаление типа-цели и получает 400
// type_id: not_found; тип единицы не изменился.
func (s *TypesLiveSuite) TestRace_EquipmentTypeChangeWaitsForTypeDelete() {
	m := sdMark()
	from := s.newType("SD-race Тип-исходный " + m)
	to := s.newType("SD-race Тип-цель " + m)
	unit := s.sdUnit(s.sdClient("SD-race Клиент-смена "+m), from.ID, "M-race-смена")

	res := s.race(s.holdTypeDelete(to.ID), func() (int, []byte) {
		st, _, raw := s.unitReq(http.MethodPut, "/api/v1/equipment/"+unit, to.ID, "M-race-смена")
		return st, raw
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["type_id"])
	s.Equal(from.ID, s.sdScalar(`SELECT type_id::text FROM equipment WHERE id = $1::uuid`, unit))
}

// §3 п. 4 (DS-017): тип с названием удалённого создаётся в незавершённой транзакции — в это окно
// приходит восстановление удалённого. Исход определяет частичный уникальный индекс:
// восстановление ждёт и после фиксации получает 409 NAME_TAKEN, тип остаётся удалённым.
func (s *TypesLiveSuite) TestRace_TypeRestoreWaitsForCreateSameName() {
	m := sdMark()
	name := "SD-race Название " + m
	old := s.newType(name)
	s.mustDeleteType(old.ID, "Гонка")

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `INSERT INTO equipment_types (name) VALUES ($1)`, name)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.restoreType(s.admin, old.ID) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("NAME_TAKEN", s.sdErr(res.raw).Code)
	s.Equal("true|catalog-admin|Гонка|true", s.typeState(old.ID), "тип остался удалённым")
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM equipment_types WHERE lower(btrim(name)) = lower($1) AND deleted_at IS NULL`, name))
}

// DS-024: два одновременных удаления одного типа — второе ждёт первое и получает 404; отметка
// первого не перезаписана.
func (s *TypesLiveSuite) TestRace_SecondTypeDeleteGets404() {
	tp := s.newType("SD-race Двойное " + sdMark())
	res := s.race(s.holdTypeDelete(tp.ID), func() (int, []byte) {
		st, _, raw := s.deleteType(s.admin, tp.ID, "Второй запрос")
		return st, raw
	})
	s.Require().Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("EQUIPMENT_TYPE_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("true|catalog-admin|Гонка|true", s.typeState(tp.ID), "причина первого удаления не перезаписана")
}
