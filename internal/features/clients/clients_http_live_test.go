//go:build integration

package clients_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ClientsLiveSuite (§TD-8): клиенты + телефоны + version (оптимистичная блокировка),
// поиск, каскадное удаление.
type ClientsLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestClientsLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка видов clients и equipment (soft-delete-clients
	// §6.5, DS-027).
	suite.Run(t, &ClientsLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *ClientsLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("clients-admin", "password1", "Клиенты Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("clients-admin", "password1")
}

type phoneDTO struct {
	Phone string `json:"phone"`
	Note  string `json:"note"`
}

type clientDTO struct {
	ID              string     `json:"id"`
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []phoneDTO `json:"phones"`
	EquipmentCount  int        `json:"equipment_count"`
	Version         int64      `json:"version"`
}

type apiErr struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

func (s *ClientsLiveSuite) errOf(body []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *ClientsLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *ClientsLiveSuite) create(b *testutil.Browser, body map[string]any) (int, clientDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/clients", body)
	var c clientDTO
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &c))
	}
	return st, c, raw
}

func (s *ClientsLiveSuite) put(b *testutil.Browser, id string, body map[string]any) (int, []byte) {
	return b.Do(http.MethodPut, "/api/v1/clients/"+id, body)
}

func card(c clientDTO, patch map[string]any) map[string]any {
	m := map[string]any{
		"company_name": c.CompanyName, "address": c.Address,
		"contact_name": c.ContactName, "contact_position": c.ContactPosition,
		"phones": c.Phones, "version": c.Version,
	}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *ClientsLiveSuite) newClient(company string, phones ...map[string]any) clientDTO {
	if len(phones) == 0 {
		phones = []map[string]any{{"phone": "9991234567", "note": ""}}
	}
	st, c, raw := s.create(s.admin, map[string]any{"company_name": company, "phones": phones})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return c
}

// ---- LV-001/LV-002: happy path — создание/получение с несколькими телефонами ----

func (s *ClientsLiveSuite) TestCreateAndGet_HappyPath() {
	st, c, raw := s.create(s.admin, map[string]any{
		"company_name":     "  ООО Ромашка  ",
		"address":          "г. Москва, ул. Ленина, 1",
		"contact_name":     "Иванов Иван",
		"contact_position": "Директор",
		"phones": []map[string]any{
			{"phone": "+7 (999) 123-45-67", "note": "основной"},
			{"phone": "8001234567", "note": ""},
		},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("ООО Ромашка", c.CompanyName, "trim")
	s.Require().Len(c.Phones, 2)
	s.Equal("+7 (999) 123-45-67", c.Phones[0].Phone)
	s.Equal("основной", c.Phones[0].Note)
	s.Equal(int64(1), c.Version)
	s.Equal(0, c.EquipmentCount)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Equal(http.StatusOK, st)
	var got clientDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(c.ID, got.ID)
	s.Len(got.Phones, 2)
}

// ---- LV-004: список и поиск (компания/представитель/телефон, в т.ч. по цифрам) ----

func (s *ClientsLiveSuite) TestList_Search() {
	c1 := s.newClient("Холодмаш", map[string]any{"phone": "+7 (999) 111-22-33", "note": ""})
	s.newClient("Электросервис", map[string]any{"phone": "+7 (888) 000-00-00", "note": ""})
	st, _, raw := s.create(s.admin, map[string]any{
		"company_name": "Прочее ООО", "contact_name": "Холодов Пётр",
		"phones": []map[string]any{{"phone": "111", "note": ""}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?q=холод", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items  []clientDTO `json:"items"`
		Total  int         `json:"total"`
		Limit  int         `json:"limit"`
		Offset int         `json:"offset"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(2, list.Total, "по компании 'Холодмаш' и по контакту 'Холодов Пётр'")
	s.Equal(50, list.Limit)

	// поиск по цифрам телефона (ADR-003): "999 111" встречается только у c1.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?q=999+111", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Require().Equal(1, list.Total, string(raw))
	s.Equal(c1.ID, list.Items[0].ID)
}

// ---- LV-004 (code review MEDIUM-3): «8999…» находит «+7 (999)…» — ведущая 7/8 полного
// номера отбрасывается перед сравнением (ADR-003) ----

// Область кодов «70x» — не пересекается с номерами остальных тестов сьюты (общая БД,
// без TRUNCATE между тестами): изолирует Total-проверки от чужих строк.
func (s *ClientsLiveSuite) TestList_Search_LeadingCountryDigit() {
	var list struct {
		Items []clientDTO `json:"items"`
		Total int         `json:"total"`
	}
	search := func(q string) {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients?q="+url.QueryEscape(q), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	}

	// полный 11-значный номер, набранный с "+7" — находится запросом с ведущей 8, и наоборот.
	c := s.newClient("Ведущая-Цифра-Клиент-701", map[string]any{"phone": "+7 (701) 555-11-22", "note": ""})
	search("87015551122")
	s.Require().Equal(1, list.Total, "«8701…» находит «+7 (701)…» (ADR-003): %+v", list)
	s.Equal(c.ID, list.Items[0].ID)

	c2 := s.newClient("Ведущая-Цифра-Клиент-702", map[string]any{"phone": "87024445566", "note": ""})
	search("+7 (702) 444-55-66")
	s.Require().Equal(1, list.Total, "%+v", list)
	s.Equal(c2.ID, list.Items[0].ID)

	// пример ADR-003 буквально: короткий частичный ввод «8<area><начало номера>» находит
	// номер, набранный с "+7" (обрезанные цифры запроса — в обрезанных цифрах номера).
	c3 := s.newClient("Ведущая-Цифра-Клиент-703", map[string]any{"phone": "+7 (703) 123-00-00", "note": ""})
	search("8703123")
	s.Require().Equal(1, list.Total, "короткий частичный ввод «8703123» ~ «+7 (703) 123-…»: %+v", list)
	s.Equal(c3.ID, list.Items[0].ID)

	// регрессия итерации 2: короткий частичный ввод С КОДОМ СТРАНЫ (не только полный
	// 11-значный номер) обязан находить номер, набранный с той же ведущей цифрой — раньше
	// (после MEDIUM-3 итерации 1) БД-сторона стриплась ВСЕГДА, а запрос — только при
	// ≥11 цифрах, из-за чего оба варианта ниже давали 0 результатов.
	c4 := s.newClient("Ведущая-Цифра-Клиент-704", map[string]any{"phone": "+7 (704) 111-99-00", "note": ""})
	search("+7 704 111")
	s.Require().Equal(1, list.Total, "«+7 704 111» находит «+7 (704) 111-99-00» (было 0 после регрессии): %+v", list)
	s.Equal(c4.ID, list.Items[0].ID)

	search("8 (704) 111")
	s.Require().Equal(1, list.Total, "«8 (704) 111» находит «+7 (704) 111-99-00» (было 0 после регрессии): %+v", list)
	s.Equal(c4.ID, list.Items[0].ID)

	// негатив: номер без совпадающих цифр (другая «область») не находится.
	search("+7 (709) 999-99-99")
	s.Equal(0, list.Total, "номер без совпадающих цифр не должен находиться: %+v", list)
}

// ---- LV-040 (clients API часть): мусор в limit/offset — дефолты, не 500; % и _ в поиске
// ищутся буквально (EscapeLike), не как SQL-шаблон.

func (s *ClientsLiveSuite) TestList_GarbageParams_And_LiteralLikeChars() {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients?limit=-5&offset=-100", nil)
	s.Equal(http.StatusOK, st, string(raw))
	var list struct {
		Limit, Offset int
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(50, list.Limit)
	s.Equal(0, list.Offset)

	s.newClient("100% Сервис")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?q=100%25", nil) // "100%" буквально
	s.Require().Equal(http.StatusOK, st, string(raw))
	var withPct struct {
		Total int `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &withPct))
	s.Equal(1, withPct.Total, "'%' в запросе ищется буквально, а не как SQL-шаблон 'любые символы'")
}

// ---- LV-035 (clients часть): пагинация — limit/offset корректно режут страницу ----

func (s *ClientsLiveSuite) TestList_Pagination() {
	for i := 0; i < 5; i++ {
		s.newClient("Пагинация-Клиент-" + string(rune('A'+i)))
	}
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients?q=Пагинация-Клиент&limit=2&offset=0", nil)
	s.Require().Equal(http.StatusOK, st)
	var page1 struct {
		Items []clientDTO `json:"items"`
		Total int         `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &page1))
	s.Equal(5, page1.Total)
	s.Len(page1.Items, 2)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?q=Пагинация-Клиент&limit=2&offset=2", nil)
	s.Require().Equal(http.StatusOK, st)
	var page2 struct {
		Items []clientDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &page2))
	s.Len(page2.Items, 2)
	s.NotEqual(page1.Items[0].ID, page2.Items[0].ID)
}

// SSR-список отдаёт не больше maxLimit (200) клиентов — при большем числе совпадений
// экран говорит «показаны первые N», а не обрезает список молча.
func (s *ClientsLiveSuite) TestSSRList_TruncatedNotice() {
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO clients (company_name) SELECT 'Обрезка-Клиент-' || g FROM generate_series(1, 201) g`)
	s.Require().NoError(err)

	st, raw := s.admin.Do(http.MethodGet, "/clients?q="+url.QueryEscape("Обрезка-Клиент"), nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), "Клиентов: 201 — показаны первые 200, уточните поиск")

	st, raw = s.admin.Do(http.MethodGet, "/clients?q="+url.QueryEscape("Обрезка-Клиент-201"), nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), "Клиентов: 1</p>")
}

// ---- LV-040 (clients API часть, authz): исполнитель читает, мутации 403; аноним 401 ----

func (s *ClientsLiveSuite) TestAuthz_ExecutorReadOnly_MutationsForbidden_AnonUnauthorized() {
	c := s.newClient("Права-Клиент")

	s.SeedUser("clients-exec", "password1", "Клиенты Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec", "password1")

	st, _ := exec.Do(http.MethodGet, "/api/v1/clients", nil)
	s.Equal(http.StatusOK, st, "чтение списка разрешено исполнителю")
	st, _ = exec.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Equal(http.StatusOK, st, "чтение карточки разрешено исполнителю")

	for _, cs := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/clients"},
		{http.MethodPut, "/api/v1/clients/" + c.ID},
		{http.MethodDelete, "/api/v1/clients/" + c.ID},
	} {
		st, raw := exec.Do(cs.method, cs.path, map[string]any{"company_name": "x"})
		s.Equal(http.StatusForbidden, st, "%s %s", cs.method, cs.path)
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/clients", nil)
	s.Equal(http.StatusUnauthorized, st)
}

// ---- LV-005: валидация / мусор в теле → 400, не 500 ----

func (s *ClientsLiveSuite) TestCreate_Validation() {
	st, _, raw := s.create(s.admin, map[string]any{})
	s.Equal(http.StatusBadRequest, st)
	f := s.fields(raw)
	s.Equal("required", f["company_name"])
	s.Equal("required", f["phones"])

	st, _, raw = s.create(s.admin, map[string]any{
		"company_name": strings.Repeat("я", 201),
		"phones":       []map[string]any{{"phone": "123"}},
	})
	s.Equal(http.StatusBadRequest, st)
	s.Equal("too_long", s.fields(raw)["company_name"])

	manyPhones := make([]map[string]any, 21)
	for i := range manyPhones {
		manyPhones[i] = map[string]any{"phone": "123456789"}
	}
	st, _, raw = s.create(s.admin, map[string]any{"company_name": "Много телефонов", "phones": manyPhones})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_many", s.fields(raw)["phones"])

	st, _, raw = s.create(s.admin, map[string]any{"company_name": "Плохой телефон", "phones": []map[string]any{{"phone": "abc"}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_format", s.fields(raw)["phones[0].phone"])

	for _, body := range []string{`{`, `null`, `"str"`, `{"phones":"not-an-array"}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}

	// version отсутствует на PUT → 400 required (ADR-100).
	c := s.newClient("Без version на PUT")
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/clients/"+c.ID, map[string]any{
		"company_name": c.CompanyName, "phones": []phoneDTO{{Phone: "123"}},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["version"])
}

// ---- code review MEDIUM-1: NUL-байт / невалидный UTF-8 → 400, не 500 (§TD-4) ----

func (s *ClientsLiveSuite) TestCreate_NulByteAndQueryGarbage_Not500() {
	st, _, raw := s.create(s.admin, map[string]any{
		"company_name": "Плохое имя\u0000",
		"phones":       []map[string]any{{"phone": "123"}},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["company_name"])

	st, _, raw = s.create(s.admin, map[string]any{
		"company_name": "Клиент с NUL в телефоне",
		"phones":       []map[string]any{{"phone": "1\u000023"}},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["phones[0].phone"])

	// GET /api/v1/clients?q=%00 (доступно и исполнителю, D19/AC-013) — не 500: фильтр
	// молча игнорируется, обычный список.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients?q=%00", nil)
	s.Equal(http.StatusOK, st, string(raw))

	s.SeedUser("clients-exec-nul", "password1", "Клиенты Исп NUL", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec-nul", "password1")
	st, raw = exec.Do(http.MethodGet, "/api/v1/clients?q=%00", nil)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- LV-008: битый/несуществующий uuid → 404, не 500 ----

func (s *ClientsLiveSuite) TestBadOrMissingID_404() {
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000", "1'; DROP TABLE clients;--"} {
		esc := url.PathEscape(id)
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+esc, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.put(s.admin, esc, map[string]any{"company_name": "x", "phones": []phoneDTO{{Phone: "1"}}, "version": 1})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/clients/"+esc, reasonBody("Проверка идентификатора"))
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// ---- LV-006: устаревший version на PUT → 409, карточка не перезаписана ----

func (s *ClientsLiveSuite) TestUpdate_StaleVersion_Conflict_NotOverwritten() {
	c := s.newClient("Версия-Клиент")
	st, raw := s.put(s.admin, c.ID, card(c, map[string]any{"company_name": "Первая правка"}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	// повторный PUT со старым version=1 (карточка уже version=2) → 409, не перезапись.
	st, raw = s.put(s.admin, c.ID, card(c, map[string]any{"company_name": "Устаревшая правка"}))
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CLIENT_VERSION_CONFLICT", s.errOf(raw).Code)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got clientDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal("Первая правка", got.CompanyName, "устаревшая правка не применилась")
	s.Equal(int64(2), got.Version)
}

// ---- LV-020: конкурентная запись — детерминированный исход, ровно один 200, один 409 ----

func (s *ClientsLiveSuite) TestUpdate_Concurrent_ExactlyOneWinner() {
	c := s.newClient("Конкурентный-Клиент")

	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b := s.NewBrowser()
			b.Login("clients-admin", "password1")
			st, _ := s.put(b, c.ID, card(c, map[string]any{"company_name": "Правка от горутины"}))
			statuses[idx] = st
		}(i)
	}
	wg.Wait()

	ok200, ok409 := 0, 0
	for _, st := range statuses {
		switch st {
		case http.StatusOK:
			ok200++
		case http.StatusConflict:
			ok409++
		}
	}
	s.Equal(1, ok200, "ровно один запрос выиграл гонку: %v", statuses)
	s.Equal(1, ok409, "ровно один запрос получил конфликт: %v", statuses)

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got clientDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(int64(2), got.Version, "версия увеличилась ровно один раз")
}

// reasonBody — тело DELETE мягкого удаления (soft-delete DS-028).
func reasonBody(reason string) map[string]any { return map[string]any{"reason": reason} }

// ---- LV-007 (soft-delete-clients TS-F6): DELETE клиента с оборудованием — МЯГКОЕ удаление:
// 200 с телом, строки клиента, телефонов и оборудования остаются в базе помеченными; для
// обычных адресов клиент не существует; повторный DELETE → 404 ----

func (s *ClientsLiveSuite) TestDelete_Soft_And_RepeatDelete404() {
	c := s.newClient("Удаляемый-Клиент", map[string]any{"phone": "111", "note": ""}, map[string]any{"phone": "222", "note": ""})

	var typeID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO equipment_types (name) VALUES ('Тип для удаления') RETURNING id`).Scan(&typeID))
	var equipmentID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO equipment (client_id, type_id, model) VALUES ($1::uuid, $2::uuid, 'M1') RETURNING id`,
		c.ID, typeID).Scan(&equipmentID))

	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/clients/"+c.ID, reasonBody("Создан по ошибке"))
	s.Require().Equal(http.StatusOK, st, string(raw))

	var clientDeleted bool
	var phonesLeft, equipmentRows, equipmentMarked int
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(), `
		SELECT (SELECT deleted_at IS NOT NULL FROM clients WHERE id = $1::uuid),
		       (SELECT count(*) FROM client_phones WHERE client_id = $1::uuid),
		       (SELECT count(*) FROM equipment WHERE client_id = $1::uuid),
		       (SELECT count(*) FROM equipment WHERE client_id = $1::uuid AND deleted_at IS NOT NULL)`, c.ID).
		Scan(&clientDeleted, &phonesLeft, &equipmentRows, &equipmentMarked))
	s.True(clientDeleted, "строка клиента помечена удалённой, а не стёрта")
	s.Equal(2, phonesLeft, "телефоны остаются при мягком удалении")
	s.Equal(1, equipmentRows, "строка оборудования на месте")
	s.Equal(1, equipmentMarked, "оборудование помечено удалённым вместе с клиентом")

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Equal(http.StatusNotFound, st, string(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+equipmentID, nil)
	s.Equal(http.StatusNotFound, st, string(raw))

	// повторное удаление уже удалённого клиента → 404 (DS-024), не 500.
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/clients/"+c.ID, reasonBody("Повтор"))
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("CLIENT_NOT_FOUND", s.errOf(raw).Code)
}

// ---- Фикстуры заказов (минимальные строки `tasks`/`task_equipment`/`subtasks` напрямую SQL:
// заказ не предмет тестов пакета clients). Отказы удаления по ссылкам (CLIENT_HAS_TASKS,
// EQUIPMENT_USED_IN_TASKS, EQUIPMENT_HAS_TRANSFERS) сняты soft-delete-clients DS-014 — см.
// clients_soft_delete_http_live_test.go (TS-022, TS-024). ----

// insertTaskDirect — минимальная строка `tasks`, ссылающаяся на clientID (design-sql.md
// §Client/Equipment Delete Guard Query, миграция 0005 board уже создала columns/tasks).
func (s *ClientsLiveSuite) insertTaskDirect(clientID string) int64 {
	var columnID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT id FROM columns ORDER BY position LIMIT 1`).Scan(&columnID))
	var createdBy string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT id FROM users WHERE login = 'clients-admin'`).Scan(&createdBy))
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO tasks (column_id, client_id, title, position, created_by)
		 VALUES ($1::uuid, $2::uuid, 'Guard task', 1024, $3::uuid) RETURNING id`,
		columnID, clientID, createdBy).Scan(&taskID))
	return taskID
}

// newServiceDirect — категория+услуга напрямую SQL (subtasks.service_id NOT NULL — не
// импортируем catalog, ADR-102-стиль).
func (s *ClientsLiveSuite) newServiceDirect(name string) string {
	var catID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', $1) RETURNING id`, name+" категория").Scan(&catID))
	var svcID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO services (category_id, name) VALUES ($1::uuid, $2) RETURNING id`, catID, name).Scan(&svcID))
	return svcID
}

// ---- SSR-экран (Phase 6) ----

// ---- LV-041 (clients часть): admin — 200, исполнитель — 403-страница, аноним — редирект
// на /login (не JSON).
func (s *ClientsLiveSuite) TestSSR_Guards() {
	st, page := s.admin.Do(http.MethodGet, "/clients", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="clients-table"`)

	s.SeedUser("clients-exec-ssr", "password1", "Клиенты Исп SSR", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec-ssr", "password1")
	st, page = exec.Do(http.MethodGet, "/clients", nil)
	s.Equal(http.StatusForbidden, st)
	s.Contains(string(page), "Нет доступа")
	st, _ = exec.Do(http.MethodGet, "/clients/new", nil)
	s.Equal(http.StatusForbidden, st)

	anon := s.NewBrowser()
	resp := anon.Request(http.MethodGet, "/clients", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusFound, resp.StatusCode)
	s.Contains(resp.Header.Get("Location"), "/login")
}

// ---- LV-021: SSR PRG create + 409 conflict сохраняет ввод (в т.ч. телефоны, ADR-005) ----

func (s *ClientsLiveSuite) TestSSR_CreatePRG_And_ConflictKeepsInput() {
	form := url.Values{"company_name": {"SSR Конфликт Клиент"}, "address": {"г. Тест"}}
	form.Add("phone", "9990000000")
	form.Add("phone_note", "")
	resp := s.admin.Request(http.MethodPost, "/clients", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	createLocation := resp.Header.Get("Location")

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients?q=SSR+Конфликт+Клиент", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items []clientDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Require().Len(list.Items, 1)
	c := list.Items[0]
	s.Equal(int64(1), c.Version)
	// PRG — на карточку созданного клиента, не на список (design-go-layers.md §SSR Routes).
	s.Equal("/clients/"+c.ID+"?flash=created", createLocation)

	// кто-то другой обновляет карточку раньше нас (version становится 2).
	st, raw = s.put(s.admin, c.ID, card(c, map[string]any{"company_name": "Уже изменено другим"}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	// SSR-правка со старым version=1 → 409, форма перерисована с нашим вводом.
	staleForm := url.Values{"company_name": {"Мои несохранённые правки"}, "version": {"1"}}
	staleForm.Add("phone", "1112223344")
	staleForm.Add("phone_note", "мой ввод")
	st, page := s.admin.Do(http.MethodPost, "/clients/"+c.ID, staleForm)
	s.Equal(http.StatusConflict, st)
	body := string(page)
	s.Contains(body, "Карточку уже изменил другой пользователь — ваши правки не сохранены")
	s.Contains(body, "Мои несохранённые правки")
	s.Contains(body, "1112223344")
	s.Contains(body, "мой ввод")
	s.Contains(body, `href="/clients/`+c.ID+`"`, "ссылка «Открыть актуальную версию»")

	// карточка в БД не перезаписана устаревшей правкой.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got clientDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal("Уже изменено другим", got.CompanyName)
}

// ---- LV-022: SSR delete без CSRF-токена → 403, с токеном — PRG-редирект ----

func (s *ClientsLiveSuite) TestSSR_Delete_CSRFRequired() {
	c := s.newClient("SSR Удаляемый Клиент")

	resp := s.admin.Request(http.MethodPost, "/clients/"+c.ID+"/delete", url.Values{}, map[string]string{"X-Csrf-Token": ""})
	defer resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode, "без CSRF-токена (ни заголовка, ни _csrf в теле) — 403")

	resp2 := s.admin.Request(http.MethodPost, "/clients/"+c.ID+"/delete",
		url.Values{"_csrf": {s.admin.CSRFToken()}, "reason": {"Создан по ошибке"}}, nil)
	resp2.Body.Close()
	s.Equal(http.StatusSeeOther, resp2.StatusCode)
	s.Equal("/clients?flash=deleted&n=0", resp2.Header.Get("Location"))

	st, _ := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- code review MEDIUM-2: SSR-форма перерисовывается НОРМАЛИЗОВАННЫМ списком телефонов
// — индекс ошибки соответствует строке, которую ввёл пользователь, а не сырому индексу
// POST (пример из отчёта: ["", "abc"] — пустая строка отбрасывается, ошибка должна быть
// под "abc", не под пустой строкой) ----

func (s *ClientsLiveSuite) TestSSR_Create_PhoneErrorIndexMatchesNormalizedRow() {
	form := url.Values{"company_name": {"Индекс-Телефон-Клиент"}}
	form.Add("phone", "")
	form.Add("phone_note", "")
	form.Add("phone", "abc")
	form.Add("phone_note", "")
	st, page := s.admin.Do(http.MethodPost, "/clients", form)
	s.Equal(http.StatusUnprocessableEntity, st)
	body := string(page)

	firstRow := strings.Index(body, `data-testid="phone-row"`)
	templateStart := strings.Index(body, "<template data-row-template>")
	s.Require().True(firstRow >= 0 && templateStart > firstRow, body)
	rowSegment := body[firstRow:templateStart]

	s.Equal(1, strings.Count(rowSegment, `data-testid="phone-row"`),
		"пустая строка отброшена normalize() — остаётся только строка 'abc'")
	s.Contains(rowSegment, `value="abc"`, "нормализованная (не сырая) строка формы")
	s.Contains(rowSegment, "Номер должен содержать цифры", "ошибка под строкой 'abc', не под отброшенной пустой")
}

// ---- Equipment API (Phase 7) ----

type specDTO struct {
	K string `json:"k"`
	V string `json:"v"`
}

type equipmentDTO struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	TypeID       string    `json:"type_id"`
	TypeName     string    `json:"type_name"`
	TypeIsActive bool      `json:"type_is_active"`
	Brand        string    `json:"brand"`
	Model        string    `json:"model"`
	SKU          string    `json:"sku"`
	Serial       string    `json:"serial"`
	Specs        []specDTO `json:"specs"`
	Note         string    `json:"note"`
}

// newType — активный тип оборудования напрямую в БД (equipmenttypes — отдельный пакет,
// ADR-102: связь только через SQL).
func (s *ClientsLiveSuite) newType(name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO equipment_types (name) VALUES ($1) RETURNING id`, name).Scan(&id))
	return id
}

// archiveType — тип в архиве (is_active=false) напрямую в БД.
func (s *ClientsLiveSuite) archiveType(name string) string {
	id := s.newType(name)
	_, err := s.Pool.Exec(s.T().Context(), `UPDATE equipment_types SET is_active = false WHERE id = $1::uuid`, id)
	s.Require().NoError(err)
	return id
}

func (s *ClientsLiveSuite) createEquipment(b *testutil.Browser, clientID string, body map[string]any) (int, equipmentDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/clients/"+clientID+"/equipment", body)
	var e equipmentDTO
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &e))
	}
	return st, e, raw
}

// ---- LV-010/LV-011: happy path — создание единицы оборудования с характеристиками,
// список (envelope), счётчик оборудования клиента ----

func (s *ClientsLiveSuite) TestEquipment_CreateAndGet_HappyPath() {
	c := s.newClient("Оборудование-Клиент")
	typeID := s.newType("Компрессор")

	st, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{
		"type_id": typeID, "brand": "  Danfoss  ", "model": "M100", "sku": "SKU1", "serial": "SN1",
		"specs": []map[string]any{{"k": "мощность", "v": "5 кВт"}}, "note": "примечание",
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(c.ID, e.ClientID)
	s.Equal(typeID, e.TypeID)
	s.Equal("Компрессор", e.TypeName)
	s.True(e.TypeIsActive)
	s.Equal("Danfoss", e.Brand, "trim")
	s.Require().Len(e.Specs, 1)
	s.Equal("мощность", e.Specs[0].K)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(e.ID, got.ID)

	// список оборудования клиента отражает созданную единицу.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items []equipmentDTO `json:"items"`
		Total int            `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(1, list.Total)

	// счётчик оборудования клиента отражает реальные данные.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var cGot clientDTO
	s.Require().NoError(json.Unmarshal(raw, &cGot))
	s.Equal(1, cGot.EquipmentCount)
}

// ---- LV-040 (equipment часть, D19): исполнитель — GET/POST 200/201, PUT/DELETE 403;
// аноним 401 ----

func (s *ClientsLiveSuite) TestEquipment_Authz_ExecutorCreateAllowed_MutationsForbidden() {
	c := s.newClient("Права-Оборудование-Клиент")
	typeID := s.newType("Тип для прав")
	st, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	s.SeedUser("clients-exec-eq", "password1", "Клиенты Исп Обор", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec-eq", "password1")

	st, _ = exec.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Equal(http.StatusOK, st, "исполнитель читает список оборудования")
	st, _ = exec.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Equal(http.StatusOK, st, "исполнитель читает единицу оборудования")

	st, execEq, raw := s.createEquipment(exec, c.ID, map[string]any{"type_id": typeID, "model": "M2"})
	s.Equal(http.StatusCreated, st, "D19: исполнитель создаёт оборудование — %s", string(raw))
	s.NotEmpty(execEq.ID)

	for _, cs := range []struct{ method, path string }{
		{http.MethodPut, "/api/v1/equipment/" + e.ID},
		{http.MethodDelete, "/api/v1/equipment/" + e.ID},
	} {
		st, raw := exec.Do(cs.method, cs.path, map[string]any{"type_id": typeID, "model": "x"})
		s.Equal(http.StatusForbidden, st, "%s %s → %s", cs.method, cs.path, string(raw))
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Equal(http.StatusUnauthorized, st)
	st, _ = anon.Do(http.MethodPost, "/api/v1/clients/"+c.ID+"/equipment", map[string]any{"type_id": typeID, "model": "x"})
	s.Equal(http.StatusUnauthorized, st)
}

// ---- LV-008 (equipment часть, IDOR-подобное): оборудование чужого/несуществующего
// клиента → 404 ----

func (s *ClientsLiveSuite) TestEquipment_IDOR_UnknownOrForeignClient404() {
	other := s.newClient("Чужой-Клиент-Обор")
	typeID := s.newType("Тип IDOR")
	_, e, raw := s.createEquipment(s.admin, other.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Require().NotEmpty(e.ID, string(raw))

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+id+"/equipment", nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, _, raw = s.createEquipment(s.admin, id, map[string]any{"type_id": typeID, "model": "M1"})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}

	// оборудование существует у ДРУГОГО клиента — прямой GET по /equipment/:id не скрыт
	// (нет child-scoping по клиенту в контракте), но список ЭТОГО клиента его не содержит.
	c := s.newClient("Свой-Клиент-Обор")
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Total int `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(0, list.Total)

	// битый/несуществующий uuid единицы оборудования → 404.
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// ---- LV-012/LV-013/LV-014/LV-036 (equipment часть, валидация/adversarial, TD-4):
// обязательные поля, битый uuid type_id, архивный тип на create → 400 archived, не 500;
// specs key_required; мусор в теле → 400 ----

func (s *ClientsLiveSuite) TestEquipment_Validation_And_ArchivedType400() {
	c := s.newClient("Валидация-Оборудование-Клиент")
	archivedType := s.archiveType("Архивный тип на создание")

	st, _, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": archivedType, "model": "M1"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("archived", s.fields(raw)["type_id"])

	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{"model": "M1"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["type_id"])

	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{"type_id": "not-a-uuid", "model": "M1"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["type_id"])

	typeID := s.newType("Тип валидации")
	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["model"])

	// specs: значение без ключа → key_required.
	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{
		"type_id": typeID, "model": "M1", "specs": []map[string]any{{"k": "", "v": "5"}},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("key_required", s.fields(raw)["specs[0].k"])

	// несуществующий type_id (валидный uuid, нет строки) → not_found.
	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{
		"type_id": "00000000-0000-0000-0000-000000000000", "model": "M1",
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["type_id"])

	for _, body := range []string{`{`, `null`, `"str"`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients/"+c.ID+"/equipment", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}
}

// ---- code review MEDIUM-1 (equipment часть): NUL-байт в полях единицы → 400, не 500 ----

func (s *ClientsLiveSuite) TestEquipment_NulByte_400() {
	c := s.newClient("NUL-Оборудование-Клиент")
	typeID := s.newType("NUL Тип")

	st, _, raw := s.createEquipment(s.admin, c.ID, map[string]any{
		"type_id": typeID, "model": "M1\u0000",
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["model"])

	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{
		"type_id": typeID, "model": "M1", "specs": []map[string]any{{"k": "к\u0000люч", "v": "5"}},
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["specs[0].k"])
}

// ---- LV-008 (equipment часть, TD-4, гонка клиента): несуществующий/удалённый параллельно
// клиент на POST → 404 CLIENT_NOT_FOUND ----

func (s *ClientsLiveSuite) TestEquipment_Create_ClientGoneOrMissing_404() {
	typeID := s.newType("Тип для гонки клиента")

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, _, raw := s.createEquipment(s.admin, id, map[string]any{"type_id": typeID, "model": "M1"})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("CLIENT_NOT_FOUND", s.errOf(raw).Code)
	}

	// клиент удалён параллельно между валидацией и вставкой — FK 23503 на client_id
	// маппится в 404 CLIENT_NOT_FOUND, не 500.
	c := s.newClient("Удаляемый-Параллельно-Клиент")
	_, err := s.Pool.Exec(s.T().Context(), `DELETE FROM clients WHERE id = $1::uuid`, c.ID)
	s.Require().NoError(err)
	st, _, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("CLIENT_NOT_FOUND", s.errOf(raw).Code)
}

// ---- LV-013 (equipment часть): правка — LWW, смена типа на активный ok, version клиента
// не меняется ----

func (s *ClientsLiveSuite) TestEquipment_Update_LWW_ClientVersionUnchanged() {
	c := s.newClient("Правка-Оборудование-Клиент")
	typeID := s.newType("Тип правки")
	typeID2 := s.newType("Тип правки 2")

	st, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, rawGet := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var before clientDTO
	s.Require().NoError(json.Unmarshal(rawGet, &before))

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, map[string]any{
		"type_id": typeID2, "brand": "NewBrand", "model": "M2", "sku": "", "serial": "", "note": "",
		"specs": []map[string]any{{"k": "k1", "v": "v1"}},
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var updated equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &updated))
	s.Equal(typeID2, updated.TypeID)
	s.Equal("M2", updated.Model)

	// LWW: второй PUT без учёта версии всё равно применяется (нет version у equipment).
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, map[string]any{
		"type_id": typeID2, "model": "M3",
	})
	s.Require().Equal(http.StatusOK, st, string(raw))

	// version клиента не изменился после правок оборудования.
	st, rawAfter := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var after clientDTO
	s.Require().NoError(json.Unmarshal(rawAfter, &after))
	s.Equal(before.Version, after.Version, "сохранение оборудования не меняет version клиента")
}

// ---- LV-013 (equipment часть, TD-4): смена на архивный тип → 400 archived; правка БЕЗ
// смены уже архивного типа разрешена ----

func (s *ClientsLiveSuite) TestEquipment_Update_ArchivedTypeRules() {
	c := s.newClient("Архивный-Тип-Правка-Клиент")
	typeID := s.newType("Активный тип правки")
	archivedType := s.archiveType("Архивный тип для смены")

	st, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	// смена на архивный тип → 400 archived.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, map[string]any{
		"type_id": archivedType, "model": "M1",
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("archived", s.fields(raw)["type_id"])

	// заводим единицу сразу с типом, который потом архивируем — правка БЕЗ смены типа
	// (тип уже архивный) разрешена.
	typeToArchive := s.newType("Тип, который заархивируют")
	st, e2, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeToArchive, "model": "M2"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	_, err := s.Pool.Exec(s.T().Context(), `UPDATE equipment_types SET is_active = false WHERE id = $1::uuid`, typeToArchive)
	s.Require().NoError(err)

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e2.ID, map[string]any{
		"type_id": typeToArchive, "model": "M2-правка", "brand": "brand",
	})
	s.Equal(http.StatusOK, st, string(raw), "правка без смены уже архивного типа разрешена")
}

// ---- LV-015/LV-032: удаление единицы оборудования; повторный DELETE → 404; тип, чья
// единственная единица мягко удалена, удаляется (soft-delete-catalog TS-061: отказ
// EQUIPMENT_TYPE_IN_USE снят) ----

func (s *ClientsLiveSuite) TestEquipment_Delete_ThenTypeDeletes() {
	c := s.newClient("Удаление-Оборудование-Клиент")
	typeID := s.newType("Тип для in-use")

	st, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+e.ID, reasonBody("Списано"))
	s.Equal(http.StatusOK, st, string(raw))

	// оборудование больше не найдено, счётчик клиента обновился.
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Equal(http.StatusNotFound, st)
	st, rawC := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got clientDTO
	s.Require().NoError(json.Unmarshal(rawC, &got))
	s.Equal(0, got.EquipmentCount)

	// повторный DELETE → 404.
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+e.ID, reasonBody("Повтор"))
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("EQUIPMENT_NOT_FOUND", s.errOf(raw).Code)

	// единица удалена МЯГКО — её строка по-прежнему ссылается на тип, но удалению типа это не
	// мешает: удаление типа — UPDATE, внешний ключ equipment.type_id не срабатывает
	// (soft-delete-catalog §4.4, TS-061).
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment-types/"+typeID, reasonBody("Тип больше не нужен"))
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- LV-035 (equipment часть): мусор в limit/offset списка оборудования — дефолты,
// не 500 ----

func (s *ClientsLiveSuite) TestEquipment_List_GarbageParams() {
	c := s.newClient("Мусор-Параметры-Оборудование")
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment?limit=-5&offset=-100", nil)
	s.Equal(http.StatusOK, st, string(raw))
	var list struct {
		Limit, Offset int
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(200, list.Limit)
	s.Equal(0, list.Offset)
}

// ---- SSR equipment (Phase 8) ----

// ---- admin PRG: создание/правка/удаление единицы через форму ----

func (s *ClientsLiveSuite) TestSSR_Equipment_CreateEditDelete_PRG() {
	c := s.newClient("SSR Оборудование Клиент")
	typeID := s.newType("SSR Тип оборудования")

	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID+"/equipment/new", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="equipment-form"`)

	form := url.Values{"type_id": {typeID}, "model": {"M100"}, "brand": {"Danfoss"}}
	form.Add("spec_k", "мощность")
	form.Add("spec_v", "5 кВт")
	resp := s.admin.Request(http.MethodPost, "/clients/"+c.ID+"/equipment", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/clients/"+c.ID+"?flash=eq_created", resp.Header.Get("Location"))

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items []equipmentDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Require().Len(list.Items, 1, string(raw))
	e := list.Items[0]
	s.Equal("M100", e.Model)

	// правка — обновляем модель.
	st, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), "M100")

	editForm := url.Values{"type_id": {typeID}, "model": {"M200"}}
	resp = s.admin.Request(http.MethodPost, "/equipment/"+e.ID, editForm, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/clients/"+c.ID+"?flash=eq_saved", resp.Header.Get("Location"))

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal("M200", got.Model)

	// удаление — PRG на карточку клиента, оборудование пропадает.
	resp = s.admin.Request(http.MethodPost, "/equipment/"+e.ID+"/delete",
		url.Values{"_csrf": {s.admin.CSRFToken()}, "reason": {"Списано клиентом"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/clients/"+c.ID+"?flash=eq_deleted", resp.Header.Get("Location"))
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- валидация формы → 422 с текстом ошибки; архивный тип на создание → 422 с текстом ----

func (s *ClientsLiveSuite) TestSSR_Equipment_ValidationRerendersForm() {
	c := s.newClient("SSR Валидация Оборудование")
	archivedType := s.archiveType("SSR Архивный тип формы")

	st, page := s.admin.Do(http.MethodPost, "/clients/"+c.ID+"/equipment", url.Values{})
	s.Equal(http.StatusUnprocessableEntity, st)
	s.Contains(string(page), "Выберите тип")
	s.Contains(string(page), "Обязательное поле")

	// архивный тип на создание → 422 "Тип в архиве — выберите другой", ввод сохранён
	// (ADR-004: TypeActive-проверка выполняется отдельно от валидации полей).
	archivedForm := url.Values{"type_id": {archivedType}, "model": {"Мой ввод"}}
	st, page = s.admin.Do(http.MethodPost, "/clients/"+c.ID+"/equipment", archivedForm)
	s.Equal(http.StatusUnprocessableEntity, st)
	body := string(page)
	s.Contains(body, "Тип в архиве — выберите другой")
	s.Contains(body, "Мой ввод", "введённая модель сохранена в форме")

	// значение характеристики без ключа → key_required, ввод сохранён.
	activeType := s.newType("SSR Активный тип формы")
	specForm := url.Values{"type_id": {activeType}, "model": {"Мой ввод 2"}}
	specForm.Add("spec_k", "")
	specForm.Add("spec_v", "5")
	st, page = s.admin.Do(http.MethodPost, "/clients/"+c.ID+"/equipment", specForm)
	s.Equal(http.StatusUnprocessableEntity, st)
	body = string(page)
	s.Contains(body, "Укажите характеристику")
	s.Contains(body, "Мой ввод 2", "введённая модель сохранена в форме")
}

// ---- code review MEDIUM-2 (equipment часть): SSR-форма единицы перерисовывается
// НОРМАЛИЗОВАННЫМ списком характеристик — тот же баг, что и с телефонами ----

func (s *ClientsLiveSuite) TestSSR_Equipment_SpecErrorIndexMatchesNormalizedRow() {
	c := s.newClient("Индекс-Спека-Клиент")
	typeID := s.newType("Индекс Спека Тип")

	form := url.Values{"type_id": {typeID}, "model": {"M1"}}
	form.Add("spec_k", "")
	form.Add("spec_v", "")
	form.Add("spec_k", "")
	form.Add("spec_v", "значение без ключа")
	st, page := s.admin.Do(http.MethodPost, "/clients/"+c.ID+"/equipment", form)
	s.Equal(http.StatusUnprocessableEntity, st)
	body := string(page)

	firstRow := strings.Index(body, `data-testid="spec-row"`)
	templateStart := strings.Index(body, "<template data-row-template>")
	s.Require().True(firstRow >= 0 && templateStart > firstRow, body)
	rowSegment := body[firstRow:templateStart]

	s.Equal(1, strings.Count(rowSegment, `data-testid="spec-row"`),
		"полностью пустая пара отброшена normalize() — остаётся только 'значение без ключа'")
	s.Contains(rowSegment, `value="значение без ключа"`, "нормализованная (не сырая) строка формы")
	s.Contains(rowSegment, "Укажите характеристику", "ошибка под строкой со значением, не под отброшенной пустой парой")
}

// ---- CSRF: POST без токена → 403; аноним → редирект /login; исполнитель → 403-страница ----

func (s *ClientsLiveSuite) TestSSR_Equipment_Guards() {
	c := s.newClient("SSR Права Оборудование")
	typeID := s.newType("SSR Тип прав")
	st, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	resp := s.admin.Request(http.MethodPost, "/clients/"+c.ID+"/equipment", url.Values{"type_id": {typeID}, "model": {"M2"}}, map[string]string{"X-Csrf-Token": ""})
	defer resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode, "без CSRF-токена — 403")

	s.SeedUser("clients-exec-eq-ssr", "password1", "Клиенты Исп Обор SSR", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec-eq-ssr", "password1")
	st, _ = exec.Do(http.MethodGet, "/clients/"+c.ID+"/equipment/new", nil)
	s.Equal(http.StatusForbidden, st)
	st, page := exec.Do(http.MethodGet, "/equipment/"+e.ID, nil)
	s.Equal(http.StatusForbidden, st)
	s.Contains(string(page), "Нет доступа")

	anon := s.NewBrowser()
	resp2 := anon.Request(http.MethodGet, "/equipment/"+e.ID, nil, nil)
	resp2.Body.Close()
	s.Equal(http.StatusFound, resp2.StatusCode)
	s.Contains(resp2.Header.Get("Location"), "/login")
}

// ---- карточка клиента отражает реальный блок «Оборудование клиента» ----

func (s *ClientsLiveSuite) TestSSR_ClientCard_ShowsEquipmentBlock() {
	c := s.newClient("SSR Карточка Оборудование")
	typeID := s.newType("SSR Тип карточки")
	_, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "КартаМодель", "serial": "SN-CARD"})
	s.Require().NotEmpty(e.ID, string(raw))

	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	body := string(page)
	s.Contains(body, `data-testid="equipment-table"`)
	s.Contains(body, "КартаМодель")
	s.Contains(body, "SN-CARD")
	s.Contains(body, `href="/equipment/`+e.ID+`"`)
}

// ---- История единицы оборудования: заказы (task_equipment / subtasks.equipment_id),
// услуги по единице, их товары и журнал; услуги по другой единице не попадают.
func (s *ClientsLiveSuite) TestSSR_EquipmentHistory() {
	ctx := s.T().Context()
	c := s.newClient("Клиент История Оборудования")
	typeID := s.newType("Тип для истории")
	_, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M-hist"})
	s.Require().NotEmpty(e.ID, string(raw))
	_, other, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M-hist-other"})
	s.Require().NotEmpty(other.ID, string(raw))

	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `href="/equipment/`+e.ID+`/history"`)
	s.Contains(string(page), `data-testid="equipment-edit"`)

	st, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="history-empty"`)

	taskID := s.insertTaskDirect(c.ID)
	_, err := s.Pool.Exec(ctx, `INSERT INTO task_equipment (task_id, equipment_id, completeness, condition)
		VALUES ($1, $2::uuid, 'с кабелем', 'царапины')`, taskID, e.ID)
	s.Require().NoError(err)
	svcID := s.newServiceDirect("История Ремонт платы")
	otherSvcID := s.newServiceDirect("История Чужая услуга")
	var subID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO subtasks (task_id, service_id, equipment_id, status, price)
		VALUES ($1, $2::uuid, $3::uuid, 'done', 1500) RETURNING id`, taskID, svcID, e.ID).Scan(&subID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtasks (task_id, service_id, equipment_id) VALUES ($1, $2::uuid, $3::uuid)`,
		taskID, otherSvcID, other.ID)
	s.Require().NoError(err)
	var catID, prodID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'История товары') RETURNING id`).Scan(&catID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (category_id, name, unit) VALUES ($1::uuid, 'Конденсатор 100мкФ', 'шт') RETURNING id`, catID).Scan(&prodID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1::uuid, $2::uuid, 2)`, subID, prodID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_logs (subtask_id, user_id, text)
		SELECT $1::uuid, id, 'Заменил конденсатор' FROM users WHERE login = 'clients-admin'`, subID)
	s.Require().NoError(err)

	st, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st)
	body := string(page)
	s.Contains(body, `href="/?task=`+strconv.FormatInt(taskID, 10)+`"`)
	s.Contains(body, "с кабелем")
	s.Contains(body, `data-testid="history-intake-edit"`)
	s.Contains(body, `data-task-id="`+strconv.FormatInt(taskID, 10)+`" data-equipment-id="`+e.ID+`"`)
	s.Contains(body, "/assets/ssr-shared/js/equipment-history.js")
	s.Contains(body, "История Ремонт платы")
	s.Contains(body, "Выполнена")
	s.Contains(body, "1500 ₽")
	s.Contains(body, "Конденсатор 100мкФ")
	s.Contains(body, "2 шт")
	s.Contains(body, "Заменил конденсатор")
	s.NotContains(body, "История Чужая услуга")

	// TS-041 (user-timezone, DS-032): дата заказа и время записи журнала — в поясе смотрящего
	// (фикстура TS-F5: 2026-10-06T19:30Z = 00:30 7 октября по Алматы, 22:30 6 октября по Москве).
	_, err = s.Pool.Exec(ctx, `UPDATE tasks SET created_at = '2026-10-06T19:30:00Z' WHERE id = $1`, taskID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `UPDATE subtask_logs SET created_at = '2026-10-06T19:30:00Z' WHERE subtask_id = $1::uuid`, subID)
	s.Require().NoError(err)
	_, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Contains(string(page), `data-testid="history-order-date">07.10.2026</span>`)
	s.Contains(string(page), `data-testid="history-log-time">07.10.2026 00:30</span>`)
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/me/timezone", map[string]any{"timezone": "Europe/Moscow"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Contains(string(page), `data-testid="history-order-date">06.10.2026</span>`)
	s.Contains(string(page), `data-testid="history-log-time">06.10.2026 22:30</span>`)
	s.NotContains(string(page), "07.10.2026")
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/users/me/timezone", map[string]any{"timezone": "Asia/Almaty"})
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, _ = s.admin.Do(http.MethodGet, "/equipment/not-a-uuid/history", nil)
	s.Equal(http.StatusNotFound, st)
	st, _ = s.admin.Do(http.MethodGet, "/equipment/00000000-0000-0000-0000-000000000000/history", nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- История единицы: текущее местоположение и документы «Перемещение» (transfers-core
// DS-046) — вместо склада единицы «в каждом заказе». Документ двигается реальным API
// /api/v1/transfers; история — JSON и SSR.
type historyLocationDTO struct {
	Location struct {
		LocationStatus   string  `json:"location_status"`
		WarehouseID      *string `json:"warehouse_id"`
		WarehouseName    *string `json:"warehouse_name"`
		ActiveTransferID *int64  `json:"active_transfer_id"`
		TransitToName    *string `json:"transit_to_name"`
	} `json:"location"`
	Transfers []struct {
		ID                int64  `json:"id"`
		Status            string `json:"status"`
		FromWarehouseName string `json:"from_warehouse_name"`
		ToWarehouseName   string `json:"to_warehouse_name"`
		CreatedAt         string `json:"created_at"`
	} `json:"transfers"`
}

func (s *ClientsLiveSuite) historyLocation(equipmentID string) historyLocationDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment/"+equipmentID+"/history", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var h historyLocationDTO
	s.Require().NoError(json.Unmarshal(raw, &h), string(raw))
	return h
}

// transferOf — документ склад → склад с одной единицей через реальный API перемещений.
func (s *ClientsLiveSuite) transferOf(from, to, equipmentID string, send bool) (id int64, version int) {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"direction": "warehouse_to_warehouse", "from_warehouse_id": from, "to_warehouse_id": to, "send": send,
		"items": []map[string]any{{"kind": "equipment", "equipment_id": equipmentID}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var t struct {
		ID      int64 `json:"id"`
		Version int   `json:"version"`
	}
	s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	return t.ID, t.Version
}

func (s *ClientsLiveSuite) warehouseByName(name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(), `SELECT id::text FROM warehouses WHERE name = $1`, name).Scan(&id))
	return id
}

func (s *ClientsLiveSuite) TestEquipmentHistory_LocationAndTransfers() {
	ctx := s.T().Context()
	almaty, moscow := s.warehouseByName("Алматы"), s.warehouseByName("Москва")
	c := s.newClient("Клиент История Перемещений")
	_, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": s.newType("Тип для истории перемещений"), "model": "M-move"})
	s.Require().NotEmpty(e.ID, string(raw))

	// SSR: новая единица — «у клиента», блока документов нет.
	st, page := s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="history-location" data-location-status="at_client">у клиента</span>`)
	s.NotContains(string(page), `data-testid="history-transfers"`)
	s.NotContains(string(page), `data-testid="history-active-transfer"`)

	// фикстура местоположения: единица принята на «Алматы» (в продукте — добавлением в заказ).
	_, err := s.Pool.Exec(ctx, `UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2::uuid WHERE id = $1::uuid`, e.ID, almaty)
	s.Require().NoError(err)
	h := s.historyLocation(e.ID)
	s.Equal("at_warehouse", h.Location.LocationStatus)
	s.Require().NotNil(h.Location.WarehouseName)
	s.Equal("Алматы", *h.Location.WarehouseName)
	s.Nil(h.Location.ActiveTransferID)
	s.Empty(h.Transfers)

	// документ «в пути» Алматы → Москва.
	docID, version := s.transferOf(almaty, moscow, e.ID, true)
	h = s.historyLocation(e.ID)
	s.Equal("in_transit", h.Location.LocationStatus)
	s.Nil(h.Location.WarehouseID)
	s.Require().NotNil(h.Location.ActiveTransferID)
	s.Equal(docID, *h.Location.ActiveTransferID)
	s.Require().NotNil(h.Location.TransitToName)
	s.Equal("Москва", *h.Location.TransitToName)
	s.Require().Len(h.Transfers, 1)
	s.Equal(docID, h.Transfers[0].ID)
	s.Equal("in_transit", h.Transfers[0].Status)
	s.Equal("Алматы", h.Transfers[0].FromWarehouseName)
	s.Equal("Москва", h.Transfers[0].ToWarehouseName)
	s.NotEmpty(h.Transfers[0].CreatedAt)

	_, err = s.Pool.Exec(ctx, `UPDATE transfers SET created_at = '2026-10-06T19:30:00Z' WHERE id = $1`, docID)
	s.Require().NoError(err)
	_, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	body := string(page)
	s.Contains(body, `data-location-status="in_transit">в пути → Москва</span>`)
	s.Contains(body, fmt.Sprintf(`href="/transfers?id=%d" data-testid="history-active-transfer">перемещение #%d</a>`, docID, docID))
	s.Contains(body, fmt.Sprintf(`data-testid="history-transfer" data-transfer-id="%d"`, docID))
	s.Contains(body, "Алматы → Москва")
	s.Contains(body, `data-testid="history-transfer-status">В пути</span>`)
	s.Contains(body, `data-testid="history-transfer-date">07.10.2026</span>`, "дата документа — в поясе пользователя (Asia/Almaty)")

	// приёмка: единица на складе назначения, документ — «Получено»; затем второй документ обратно.
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", docID), map[string]any{"version": version})
	s.Require().Equal(http.StatusOK, st, string(raw))
	backID, _ := s.transferOf(moscow, almaty, e.ID, false)
	h = s.historyLocation(e.ID)
	s.Equal("at_warehouse", h.Location.LocationStatus)
	s.Require().NotNil(h.Location.WarehouseName)
	s.Equal("Москва", *h.Location.WarehouseName)
	s.Require().NotNil(h.Location.ActiveTransferID, "заявка занимает единицу")
	s.Equal(backID, *h.Location.ActiveTransferID)
	s.Nil(h.Location.TransitToName, "склад назначения — только пока единица в пути")
	s.Require().Len(h.Transfers, 2, "новые сверху")
	s.Equal(backID, h.Transfers[0].ID)
	s.Equal("requested", h.Transfers[0].Status)
	s.Equal(docID, h.Transfers[1].ID)
	s.Equal("received", h.Transfers[1].Status)
	_, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Contains(string(page), `data-location-status="at_warehouse">на складе Москва</span>`)
	s.Contains(string(page), `data-testid="history-transfer-status">Получено</span>`)
	s.Contains(string(page), `data-testid="history-transfer-status">Заявка</span>`)

	// история доступна исполнителю (чтение, как GET единицы).
	s.SeedUser("hist-move-exec", "password1", "История Исполнитель", "executor", true)
	exec := s.NewBrowser()
	exec.Login("hist-move-exec", "password1")
	st, raw = exec.Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Equal(http.StatusOK, st, string(raw))
}

// История единицы и документы с клиентом (transfers-partners DS-002, DS-013): приём от клиента
// и возврат клиенту видны в истории единицы — точка маршрута без склада подписана «клиент»;
// единица в пути к клиенту — «в пути → клиент».
func (s *ClientsLiveSuite) TestEquipmentHistory_ClientDocuments() {
	ctx := s.T().Context()
	almaty := s.warehouseByName("Алматы")
	c := s.newClient("Клиент История Приём Возврат")
	_, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": s.newType("Тип для истории приёма"), "model": "M-intake"})
	s.Require().NotEmpty(e.ID, string(raw))
	// фикстура: заказ клиента с этой единицей (SQL — заказ не предмет теста).
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO tasks (column_id, client_id, title, position, created_by)
		VALUES ((SELECT id FROM columns ORDER BY position, id LIMIT 1), $1::uuid, 'Заказ истории приёма', 1,
		        (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)) RETURNING id`, c.ID).Scan(&taskID))
	_, err := s.Pool.Exec(ctx, `INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2::uuid)`, taskID, e.ID)
	s.Require().NoError(err)

	clientDoc := func(body map[string]any) (id int64, version int) {
		body["task_id"] = taskID
		body["items"] = []map[string]any{{"kind": "equipment", "equipment_id": e.ID}}
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/transfers", body)
		s.Require().Equal(http.StatusCreated, st, string(raw))
		var t struct {
			ID      int64 `json:"id"`
			Version int   `json:"version"`
		}
		s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
		return t.ID, t.Version
	}
	intakeID, _ := clientDoc(map[string]any{"direction": "client_to_warehouse", "to_warehouse_id": almaty, "receive": true})
	h := s.historyLocation(e.ID)
	s.Equal("at_warehouse", h.Location.LocationStatus)
	s.Require().Len(h.Transfers, 1, "приём от клиента виден в истории единицы")
	s.Equal(intakeID, h.Transfers[0].ID)
	s.Equal("received", h.Transfers[0].Status)
	s.Equal("клиент", h.Transfers[0].FromWarehouseName)
	s.Equal("Алматы", h.Transfers[0].ToWarehouseName)

	backID, backVersion := clientDoc(map[string]any{"direction": "warehouse_to_client", "from_warehouse_id": almaty, "send": true})
	h = s.historyLocation(e.ID)
	s.Equal("in_transit", h.Location.LocationStatus)
	s.Require().NotNil(h.Location.TransitToName)
	s.Equal("клиент", *h.Location.TransitToName)
	s.Require().Len(h.Transfers, 2)
	s.Equal(backID, h.Transfers[0].ID)
	s.Equal("Алматы", h.Transfers[0].FromWarehouseName)
	s.Equal("клиент", h.Transfers[0].ToWarehouseName)
	_, page := s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Contains(string(page), `data-location-status="in_transit">в пути → клиент</span>`)
	s.Contains(string(page), "Алматы → клиент")
	s.Contains(string(page), "клиент → Алматы")

	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", backID), map[string]any{"version": backVersion})
	s.Require().Equal(http.StatusOK, st, string(raw))
	h = s.historyLocation(e.ID)
	s.Equal("at_client", h.Location.LocationStatus)
	s.Nil(h.Location.TransitToName)
	s.Nil(h.Location.ActiveTransferID)

	// soft-delete-clients DS-014: заказ и завершённые документы клиента не удерживают —
	// оборудование вернулось клиенту, удаление проходит (раньше — 409).
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/clients/"+c.ID, reasonBody("Клиент ушёл"))
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- GET /api/v1/equipment/:id/history — JSON для «Истории работ» в карточке заказа
// (§API-1: {items, total, limit, offset}, snake_case, qty/price — числа).
func (s *ClientsLiveSuite) TestAPI_EquipmentHistory() {
	ctx := s.T().Context()
	c := s.newClient("Клиент API История")
	typeID := s.newType("Тип для API истории")
	_, e, raw := s.createEquipment(s.admin, c.ID, map[string]any{"type_id": typeID, "model": "M-api-hist"})
	s.Require().NotEmpty(e.ID, string(raw))

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.JSONEq(`{"items":[],"total":0,"limit":0,"offset":0,"transfers":[],
		"location":{"location_status":"at_client","warehouse_id":null,"warehouse_name":null,"active_transfer_id":null,"transit_to_name":null}}`,
		string(raw), "новая единица — «у клиента», документов нет (transfers-core DS-008, DS-046)")

	taskID := s.insertTaskDirect(c.ID)
	_, err := s.Pool.Exec(ctx, `INSERT INTO task_equipment (task_id, equipment_id, condition) VALUES ($1, $2::uuid, 'вмятина')`, taskID, e.ID)
	s.Require().NoError(err)
	var subID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO subtasks (task_id, service_id, equipment_id, status)
		VALUES ($1, $2::uuid, $3::uuid, 'in_progress') RETURNING id`, taskID, s.newServiceDirect("API История Чистка"), e.ID).Scan(&subID))
	var catID, prodID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'API История товары') RETURNING id`).Scan(&catID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (category_id, name, unit) VALUES ($1::uuid, 'Линза', 'шт') RETURNING id`, catID).Scan(&prodID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_products (subtask_id, product_id, qty, price) VALUES ($1::uuid, $2::uuid, 1.5, 300)`, subID, prodID)
	s.Require().NoError(err)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var resp struct {
		Items []struct {
			TaskID    int64  `json:"task_id"`
			Accepted  bool   `json:"accepted"`
			Condition string `json:"condition"`
			Services  []struct {
				Name     string   `json:"name"`
				Status   string   `json:"status"`
				Price    *float64 `json:"price"`
				Products []struct {
					Name  string  `json:"name"`
					Qty   float64 `json:"qty"`
					Price float64 `json:"price"`
				} `json:"products"`
				Logs []any `json:"logs"`
			} `json:"services"`
		} `json:"items"`
		Total int `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &resp), string(raw))
	s.Require().Equal(1, resp.Total)
	o := resp.Items[0]
	s.Equal(taskID, o.TaskID)
	s.True(o.Accepted)
	s.Equal("вмятина", o.Condition)
	s.NotContains(string(raw), `"equipment_status"`, "склад единицы «в каждом заказе» убран (DS-046)")
	s.Require().Len(o.Services, 1)
	s.Equal("API История Чистка", o.Services[0].Name)
	s.Equal("in_progress", o.Services[0].Status)
	s.Nil(o.Services[0].Price)
	s.NotNil(o.Services[0].Logs, "logs — пустой массив, не null")
	s.Require().Len(o.Services[0].Products, 1)
	s.Equal(1.5, o.Services[0].Products[0].Qty)
	s.Equal(300.0, o.Services[0].Products[0].Price)

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, _ = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+id+"/history", nil)
		s.Equal(http.StatusNotFound, st, id)
	}
	st, _ = s.NewBrowser().Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Equal(http.StatusUnauthorized, st, "аноним")
}
