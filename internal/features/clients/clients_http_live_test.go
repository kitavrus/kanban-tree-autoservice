//go:build integration

package clients_test

import (
	"context"
	"encoding/json"
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
	ID             string     `json:"id"`
	CompanyName    string     `json:"company_name"`
	Phones         []phoneDTO `json:"phones"`
	EquipmentCount int        `json:"equipment_count"`
	Version        int64      `json:"version"`
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
		"company_name": c.CompanyName,
		"phones":       c.Phones, "version": c.Version,
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
		"company_name": "  ООО Ромашка  ",
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
	for _, k := range []string{`"address"`, `"contact_name"`, `"contact_position"`} {
		s.NotContains(string(raw), k, "поля адреса/контакта убраны")
	}
}

// ---- LV-004: список и поиск (компания/телефон, в т.ч. по цифрам) ----

func (s *ClientsLiveSuite) TestList_Search() {
	c1 := s.newClient("Холодмаш", map[string]any{"phone": "+7 (999) 111-22-33", "note": ""})
	s.newClient("Электросервис", map[string]any{"phone": "+7 (888) 000-00-00", "note": ""})
	st, _, raw := s.create(s.admin, map[string]any{
		"company_name": "Холодов Пётр",
		"phones":       []map[string]any{{"phone": "111", "note": ""}},
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
	s.Equal(2, list.Total, "по компаниям 'Холодмаш' и 'Холодов Пётр'")
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

// ---- LV-040 (clients API часть, authz): исполнитель читает и заводит клиента, правка/
// удаление 403; аноним 401 ----

func (s *ClientsLiveSuite) TestAuthz_ExecutorReadsCreates_EditDeleteForbidden_AnonUnauthorized() {
	c := s.newClient("Права-Клиент")

	s.SeedUser("clients-exec", "password1", "Клиенты Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec", "password1")

	st, _ := exec.Do(http.MethodGet, "/api/v1/clients", nil)
	s.Equal(http.StatusOK, st, "чтение списка разрешено исполнителю")
	st, _ = exec.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Equal(http.StatusOK, st, "чтение карточки разрешено исполнителю")

	// создание исполнителю разрешено: проходит гард и упирается в валидацию (не 403).
	st, raw := exec.Do(http.MethodPost, "/api/v1/clients", map[string]any{"company_name": "x"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["phones"])
	st, created, raw := s.create(exec, map[string]any{
		"company_name": "Права-Клиент Исполнителя", "phones": []map[string]any{{"phone": "123"}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("Права-Клиент Исполнителя", created.CompanyName)

	for _, cs := range []struct{ method, path string }{
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

	var equipmentID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO equipment (client_id, plate_number) VALUES ($1::uuid, 'У001УУ 77') RETURNING id`,
		c.ID).Scan(&equipmentID))

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

// ---- Фикстуры заказов (минимальные строки `tasks`/`subtasks` напрямую SQL:
// заказ не предмет тестов пакета clients). Отказы удаления по ссылкам (CLIENT_HAS_TASKS,
// EQUIPMENT_USED_IN_TASKS) сняты soft-delete-clients DS-014 — см.
// clients_soft_delete_http_live_test.go (TS-022, TS-024). ----

// insertTaskDirect — минимальная строка `tasks` клиента clientID с автомобилем carID
// (one-car-order: tasks.car_id NOT NULL; миграция 0005 board уже создала columns/tasks).
func (s *ClientsLiveSuite) insertTaskDirect(clientID, carID string) int64 {
	var columnID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT id FROM columns ORDER BY position LIMIT 1`).Scan(&columnID))
	var createdBy string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT id FROM users WHERE login = 'clients-admin'`).Scan(&createdBy))
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by)
		 VALUES ($1::uuid, $2::uuid, $4::uuid, 'Guard task', 1024, $3::uuid) RETURNING id`,
		columnID, clientID, createdBy, carID).Scan(&taskID))
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
	form := url.Values{"company_name": {"SSR Конфликт Клиент"}}
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

// equipmentDTO — автомобиль клиента: только госномер (EquipmentResponse).
type equipmentDTO struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	PlateNumber string `json:"plate_number"`
}

// carBody — тело создания/правки автомобиля.
func carBody(plate string) map[string]any { return map[string]any{"plate_number": plate} }

func (s *ClientsLiveSuite) createEquipment(b *testutil.Browser, clientID string, body map[string]any) (int, equipmentDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/clients/"+clientID+"/equipment", body)
	var e equipmentDTO
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &e))
	}
	return st, e, raw
}

// ---- LV-010/LV-011: happy path — создание автомобиля (госномер нормализуется), список
// (envelope), счётчик автомобилей клиента ----

func (s *ClientsLiveSuite) TestEquipment_CreateAndGet_HappyPath() {
	c := s.newClient("Оборудование-Клиент")

	st, e, raw := s.createEquipment(s.admin, c.ID, carBody("  к100кк   77 "))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal(c.ID, e.ClientID)
	s.Equal("К100КК 77", e.PlateNumber, "госномер нормализован")
	for _, k := range []string{`"brand"`, `"model"`, `"serial"`, `"specs"`, `"type_id"`} {
		s.NotContains(string(raw), k, "у автомобиля только госномер")
	}

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal(e.ID, got.ID)
	s.Equal("К100КК 77", got.PlateNumber)

	// список автомобилей клиента отражает созданный.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items []equipmentDTO `json:"items"`
		Total int            `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(1, list.Total)

	// счётчик автомобилей клиента отражает реальные данные.
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
	st, e, raw := s.createEquipment(s.admin, c.ID, carBody("А201АА 77"))
	s.Require().Equal(http.StatusCreated, st, string(raw))

	s.SeedUser("clients-exec-eq", "password1", "Клиенты Исп Обор", "executor", true)
	exec := s.NewBrowser()
	exec.Login("clients-exec-eq", "password1")

	st, _ = exec.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Equal(http.StatusOK, st, "исполнитель читает список оборудования")
	st, _ = exec.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Equal(http.StatusOK, st, "исполнитель читает единицу оборудования")

	st, execEq, raw := s.createEquipment(exec, c.ID, carBody("А202АА 77"))
	s.Equal(http.StatusCreated, st, "D19: исполнитель создаёт оборудование — %s", string(raw))
	s.NotEmpty(execEq.ID)

	for _, cs := range []struct{ method, path string }{
		{http.MethodPut, "/api/v1/equipment/" + e.ID},
		{http.MethodDelete, "/api/v1/equipment/" + e.ID},
	} {
		st, raw := exec.Do(cs.method, cs.path, carBody("А203АА 77"))
		s.Equal(http.StatusForbidden, st, "%s %s → %s", cs.method, cs.path, string(raw))
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/clients/"+c.ID+"/equipment", nil)
	s.Equal(http.StatusUnauthorized, st)
	st, _ = anon.Do(http.MethodPost, "/api/v1/clients/"+c.ID+"/equipment", carBody("А204АА 77"))
	s.Equal(http.StatusUnauthorized, st)
}

// ---- LV-008 (equipment часть, IDOR-подобное): оборудование чужого/несуществующего
// клиента → 404 ----

func (s *ClientsLiveSuite) TestEquipment_IDOR_UnknownOrForeignClient404() {
	other := s.newClient("Чужой-Клиент-Обор")
	_, e, raw := s.createEquipment(s.admin, other.ID, carBody("Б301ББ 77"))
	s.Require().NotEmpty(e.ID, string(raw))

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients/"+id+"/equipment", nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, _, raw = s.createEquipment(s.admin, id, carBody("Б302ББ 77"))
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

// ---- LV-012/LV-014/LV-036 (equipment часть, валидация/adversarial, TD-4): госномер
// обязателен; мусор в теле → 400, не 500 ----

func (s *ClientsLiveSuite) TestEquipment_Validation400() {
	c := s.newClient("Валидация-Оборудование-Клиент")

	for _, tc := range []struct {
		body map[string]any
		msg  string
	}{
		{map[string]any{}, "required"},
		{carBody(""), "required"},
		{carBody("   "), "required"},
		{carBody("А1#"), "invalid_format"},
	} {
		st, _, raw := s.createEquipment(s.admin, c.ID, tc.body)
		s.Equal(http.StatusBadRequest, st, string(raw))
		s.Equal(tc.msg, s.fields(raw)["plate_number"], string(raw))
	}

	for _, body := range []string{`{`, `null`, `"str"`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/clients/"+c.ID+"/equipment", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}
}

// ---- code review MEDIUM-1 (equipment часть): NUL-байт в госномере → 400, не 500 ----

func (s *ClientsLiveSuite) TestEquipment_NulByte_400() {
	c := s.newClient("NUL-Оборудование-Клиент")

	st, _, raw := s.createEquipment(s.admin, c.ID, carBody("А1\u0000"))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["plate_number"])
}

// ---- LV-008 (equipment часть, TD-4, гонка клиента): несуществующий/удалённый параллельно
// клиент на POST → 404 CLIENT_NOT_FOUND ----

func (s *ClientsLiveSuite) TestEquipment_Create_ClientGoneOrMissing_404() {
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, _, raw := s.createEquipment(s.admin, id, carBody("В401ВВ 77"))
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Equal("CLIENT_NOT_FOUND", s.errOf(raw).Code)
	}

	// клиент удалён параллельно между валидацией и вставкой — FK 23503 на client_id
	// маппится в 404 CLIENT_NOT_FOUND, не 500.
	c := s.newClient("Удаляемый-Параллельно-Клиент")
	_, err := s.Pool.Exec(s.T().Context(), `DELETE FROM clients WHERE id = $1::uuid`, c.ID)
	s.Require().NoError(err)
	st, _, raw := s.createEquipment(s.admin, c.ID, carBody("В402ВВ 77"))
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("CLIENT_NOT_FOUND", s.errOf(raw).Code)
}

// ---- LV-013 (equipment часть): правка — LWW, version клиента не меняется ----

func (s *ClientsLiveSuite) TestEquipment_Update_LWW_ClientVersionUnchanged() {
	c := s.newClient("Правка-Оборудование-Клиент")

	st, e, raw := s.createEquipment(s.admin, c.ID, carBody("Г501ГГ 77"))
	s.Require().Equal(http.StatusCreated, st, string(raw))

	st, rawGet := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var before clientDTO
	s.Require().NoError(json.Unmarshal(rawGet, &before))

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, carBody("г502гг 77"))
	s.Require().Equal(http.StatusOK, st, string(raw))
	var updated equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &updated))
	s.Equal("Г502ГГ 77", updated.PlateNumber)

	// LWW: второй PUT без учёта версии всё равно применяется (нет version у equipment).
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, carBody("Г503ГГ 77"))
	s.Require().Equal(http.StatusOK, st, string(raw))

	// правка в пустой госномер → 400 required.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, carBody(" "))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["plate_number"])

	// version клиента не изменился после правок оборудования.
	st, rawAfter := s.admin.Do(http.MethodGet, "/api/v1/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var after clientDTO
	s.Require().NoError(json.Unmarshal(rawAfter, &after))
	s.Equal(before.Version, after.Version, "сохранение оборудования не меняет version клиента")
}

// ---- LV-015/LV-032: удаление автомобиля; повторный DELETE → 404 ----

func (s *ClientsLiveSuite) TestEquipment_Delete_RepeatDelete404() {
	c := s.newClient("Удаление-Оборудование-Клиент")

	st, e, raw := s.createEquipment(s.admin, c.ID, carBody("Д601ДД 77"))
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

// ---- admin PRG: создание/правка/удаление автомобиля через форму ----

func (s *ClientsLiveSuite) TestSSR_Equipment_CreateEditDelete_PRG() {
	c := s.newClient("SSR Оборудование Клиент")

	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID+"/equipment/new", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="equipment-form"`)

	form := url.Values{"plate_number": {"е701ее 77"}}
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
	s.Equal("Е701ЕЕ 77", e.PlateNumber)

	// правка — меняем госномер.
	st, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), "Е701ЕЕ 77")

	editForm := url.Values{"plate_number": {"Е702ЕЕ 77"}}
	resp = s.admin.Request(http.MethodPost, "/equipment/"+e.ID, editForm, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/clients/"+c.ID+"?flash=eq_saved", resp.Header.Get("Location"))

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal("Е702ЕЕ 77", got.PlateNumber)

	// удаление — PRG на карточку клиента, оборудование пропадает.
	resp = s.admin.Request(http.MethodPost, "/equipment/"+e.ID+"/delete",
		url.Values{"_csrf": {s.admin.CSRFToken()}, "reason": {"Списано клиентом"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/clients/"+c.ID+"?flash=eq_deleted", resp.Header.Get("Location"))
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- валидация формы → 422 с текстом ошибки под госномером, ввод сохранён ----

func (s *ClientsLiveSuite) TestSSR_Equipment_ValidationRerendersForm() {
	c := s.newClient("SSR Валидация Оборудование")

	st, page := s.admin.Do(http.MethodPost, "/clients/"+c.ID+"/equipment", url.Values{})
	s.Equal(http.StatusUnprocessableEntity, st)
	s.Contains(string(page), "Укажите госномер")
	s.Contains(string(page), `data-testid="error-plate_number"`)

	st, page = s.admin.Do(http.MethodPost, "/clients/"+c.ID+"/equipment", url.Values{"plate_number": {"А1#Б2"}})
	s.Equal(http.StatusUnprocessableEntity, st)
	body := string(page)
	s.Contains(body, "Только буквы, цифры, пробел и дефис")
	s.Contains(body, "А1#Б2", "введённый госномер сохранён в форме")
}

// ---- CSRF: POST без токена → 403; аноним → редирект /login; исполнитель → 403-страница ----

func (s *ClientsLiveSuite) TestSSR_Equipment_Guards() {
	c := s.newClient("SSR Права Оборудование")
	st, e, raw := s.createEquipment(s.admin, c.ID, carBody("Ж801ЖЖ 77"))
	s.Require().Equal(http.StatusCreated, st, string(raw))

	resp := s.admin.Request(http.MethodPost, "/clients/"+c.ID+"/equipment", url.Values{"plate_number": {"Ж802ЖЖ 77"}}, map[string]string{"X-Csrf-Token": ""})
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

// ---- карточка клиента отражает реальный блок «Автомобили клиента» ----

func (s *ClientsLiveSuite) TestSSR_ClientCard_ShowsEquipmentBlock() {
	c := s.newClient("SSR Карточка Оборудование")
	_, e, raw := s.createEquipment(s.admin, c.ID, carBody("З901ЗЗ 777"))
	s.Require().NotEmpty(e.ID, string(raw))

	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	body := string(page)
	s.Contains(body, `data-testid="equipment-table"`)
	s.Contains(body, "З901ЗЗ 777")
	s.Contains(body, `href="/equipment/`+e.ID+`"`)
}

// ---- История автомобиля: его заказы (tasks.car_id — one-car-order), все услуги заказа, их
// товары и журнал; заказ другого автомобиля не попадает. Комплектности/состояния и их правки нет.
func (s *ClientsLiveSuite) TestSSR_EquipmentHistory() {
	ctx := s.T().Context()
	c := s.newClient("Клиент История Оборудования")
	_, e, raw := s.createEquipment(s.admin, c.ID, carBody("И111ИИ 77"))
	s.Require().NotEmpty(e.ID, string(raw))
	_, other, raw := s.createEquipment(s.admin, c.ID, carBody("И112ИИ 77"))
	s.Require().NotEmpty(other.ID, string(raw))

	st, page := s.admin.Do(http.MethodGet, "/clients/"+c.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `href="/equipment/`+e.ID+`/history"`)
	s.Contains(string(page), `data-testid="equipment-edit"`)

	st, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="history-empty"`)
	s.NotContains(string(page), `data-testid="history-location"`, "местоположения единицы больше нет")
	s.NotContains(string(page), `data-testid="history-transfers"`, "блока перемещений больше нет")

	taskID := s.insertTaskDirect(c.ID, e.ID)
	otherTaskID := s.insertTaskDirect(c.ID, other.ID)
	svcID := s.newServiceDirect("История Ремонт платы")
	otherSvcID := s.newServiceDirect("История Чужая услуга")
	var subID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO subtasks (task_id, service_id, status, price)
		VALUES ($1, $2::uuid, 'done', 1500) RETURNING id`, taskID, svcID).Scan(&subID))
	_, err := s.Pool.Exec(ctx, `INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2::uuid)`,
		otherTaskID, otherSvcID)
	s.Require().NoError(err)
	var prodID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (name, unit) VALUES ('Конденсатор 100мкФ', 'шт') RETURNING id`).Scan(&prodID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1::uuid, $2::uuid, 2)`, subID, prodID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_logs (subtask_id, user_id, text)
		SELECT $1::uuid, id, 'Заменил конденсатор' FROM users WHERE login = 'clients-admin'`, subID)
	s.Require().NoError(err)

	st, page = s.admin.Do(http.MethodGet, "/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st)
	body := string(page)
	s.Contains(body, `href="/?task=`+strconv.FormatInt(taskID, 10)+`"`)
	s.NotContains(body, `href="/?task=`+strconv.FormatInt(otherTaskID, 10)+`"`, "заказ другого автомобиля не попадает")
	s.NotContains(body, `data-testid="history-intake`, "комплектности/состояния и их правки в истории нет")
	s.NotContains(body, "equipment-history.js")
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

// ---- GET /api/v1/equipment/:id/history — JSON для «Истории работ» в карточке заказа
// (§API-1: {items, total, limit, offset}, snake_case, qty/price — числа).
func (s *ClientsLiveSuite) TestAPI_EquipmentHistory() {
	ctx := s.T().Context()
	c := s.newClient("Клиент API История")
	_, e, raw := s.createEquipment(s.admin, c.ID, carBody("К121КК 77"))
	s.Require().NotEmpty(e.ID, string(raw))

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.JSONEq(`{"items":[],"total":0,"limit":0,"offset":0}`, string(raw),
		"новая единица — история пуста; полей location/transfers нет (модуль перемещений удалён)")

	taskID := s.insertTaskDirect(c.ID, e.ID)
	var subID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO subtasks (task_id, service_id, status)
		VALUES ($1, $2::uuid, 'in_progress') RETURNING id`, taskID, s.newServiceDirect("API История Чистка")).Scan(&subID))
	var prodID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (name, unit) VALUES ('Линза', 'шт') RETURNING id`).Scan(&prodID))
	_, err := s.Pool.Exec(ctx, `INSERT INTO subtask_products (subtask_id, product_id, qty, price) VALUES ($1::uuid, $2::uuid, 1.5, 300)`, subID, prodID)
	s.Require().NoError(err)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var resp struct {
		Items []struct {
			TaskID   int64 `json:"task_id"`
			Services []struct {
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
	s.NotContains(string(raw), `"accepted"`, "комплектность/состояние автомобиля в заказе не хранятся")
	s.NotContains(string(raw), `"completeness"`)
	s.NotContains(string(raw), `"condition"`)
	s.NotContains(string(raw), `"equipment_status"`, "склад единицы «в каждом заказе» убран (DS-046)")
	s.Require().Len(o.Services, 1)
	s.Equal("API История Чистка", o.Services[0].Name)
	s.Equal("in_progress", o.Services[0].Status)
	s.Nil(o.Services[0].Price)
	s.NotNil(o.Services[0].Logs, "logs — пустой массив, не null")
	s.Require().Len(o.Services[0].Products, 1)
	s.Equal(1.5, o.Services[0].Products[0].Qty)
	s.Equal(300.0, o.Services[0].Products[0].Price)
	s.NotContains(string(raw), `"location"`, "местоположения единицы больше нет")
	s.NotContains(string(raw), `"transfers"`, "документов перемещения больше нет")

	// история доступна исполнителю (чтение, как GET единицы).
	s.SeedUser("hist-api-exec", "password1", "История Исполнитель", "executor", true)
	exec := s.NewBrowser()
	exec.Login("hist-api-exec", "password1")
	st, raw = exec.Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"total":1`)

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, _ = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+id+"/history", nil)
		s.Equal(http.StatusNotFound, st, id)
	}
	st, _ = s.NewBrowser().Do(http.MethodGet, "/api/v1/equipment/"+e.ID+"/history", nil)
	s.Equal(http.StatusUnauthorized, st, "аноним")
}

// ---- Госномер: create/update нормализуют (trim, схлопывание пробелов, верхний регистр),
// недопустимое значение → 400 details.fields[plate_number]; поиск клиентов по госномеру
// без учёта регистра/пробелов/дефисов ----

func (s *ClientsLiveSuite) TestEquipment_PlateNumber_CreateUpdateValidate() {
	c := s.newClient("Госномер-Клиент")
	st, e, raw := s.createEquipment(s.admin, c.ID, carBody("  а123вс   777 "))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("А123ВС 777", e.PlateNumber)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal("А123ВС 777", got.PlateNumber)

	// без plate_number — госномер обязателен (автомобиль = госномер).
	st, _, raw = s.createEquipment(s.admin, c.ID, map[string]any{})
	s.Require().Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("required", s.fields(raw)["plate_number"])

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, carBody("b-777-xx"))
	s.Require().Equal(http.StatusOK, st, string(raw))
	var upd equipmentDTO
	s.Require().NoError(json.Unmarshal(raw, &upd))
	s.Equal("B-777-XX", upd.PlateNumber)

	for _, tc := range []struct{ plate, msg string }{
		{strings.Repeat("А", 21), "too_long"},
		{"  ", "required"},
		{"А123#ВС", "invalid_format"},
		{"А1_23", "invalid_format"},
	} {
		st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment/"+e.ID, carBody(tc.plate))
		s.Require().Equal(http.StatusBadRequest, st, string(raw))
		s.Equal(tc.msg, s.fields(raw)["plate_number"], "plate=%q", tc.plate)
	}
	// неудачные PUT не изменили госномер.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment/"+e.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.Equal("B-777-XX", got.PlateNumber)
}

func (s *ClientsLiveSuite) TestList_Search_ByPlateNumber() {
	c := s.newClient("Госномер-Поиск ООО", map[string]any{"phone": "+7 (501) 000-00-01", "note": ""})
	other := s.newClient("Госномер-Другой ООО", map[string]any{"phone": "+7 (502) 000-00-02", "note": ""})
	st, _, raw := s.createEquipment(s.admin, c.ID, carBody("Т456ОР 750"))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, eDel, raw := s.createEquipment(s.admin, other.ID, carBody("Т456ОР 751"))
	s.Require().Equal(http.StatusCreated, st, string(raw))
	// мягко удалённая единица в поиск клиентов не попадает.
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment/"+eDel.ID, reasonBody("Машина продана"))
	s.Require().True(st == http.StatusOK || st == http.StatusNoContent, "%d %s", st, raw)

	search := func(q string) []string {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/clients?q="+url.QueryEscape(q), nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var list struct {
			Items []clientDTO `json:"items"`
		}
		s.Require().NoError(json.Unmarshal(raw, &list))
		ids := make([]string, 0, len(list.Items))
		for _, it := range list.Items {
			ids = append(ids, it.ID)
		}
		return ids
	}
	for _, q := range []string{"т456ор750", "Т456ОР 750", "456 ор-750"} {
		ids := search(q)
		s.Contains(ids, c.ID, "q=%q", q)
		s.NotContains(ids, other.ID, "q=%q", q)
	}
	s.NotContains(search("т456ор751"), other.ID, "удалённая единица не ищется")
}
