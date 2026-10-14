//go:build integration

package suppliers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SuppliersLiveSuite (§TD-8): справочник «Поставщики» (docs/features/suppliers/draft-spec.md
// DS-007/DS-043) — TS-020: валидация полей, уникальность названия (регистр/пробелы, вкл.
// архивные), is_active обязателен в PUT, параллельное создание, executor → 403, фильтры
// списка и конверт {items,total,limit,offset}, 404 SUPPLIER_NOT_FOUND; SSR-оболочка экрана.
type SuppliersLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestSuppliersLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка вида suppliers (soft-delete-warehouses-suppliers
	// DS-027, TestPurge_Suppliers).
	suite.Run(t, &SuppliersLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *SuppliersLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("sup-admin", "password1", "Поставщики Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("sup-admin", "password1")
}

type supplierDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	Note        string `json:"note"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type supplierListDTO struct {
	Items  []supplierDTO `json:"items"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

const supAPI = "/api/v1/suppliers"

func (s *SuppliersLiveSuite) fields(body []byte) map[string]string {
	var e struct {
		Code    string `json:"code"`
		Details *struct {
			Fields []struct{ Name, Message string } `json:"fields"`
		} `json:"details"`
	}
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *SuppliersLiveSuite) list(query url.Values) supplierListDTO {
	path := supAPI
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	st, raw := s.admin.Do(http.MethodGet, path, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l supplierListDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	return l
}

func (s *SuppliersLiveSuite) create(body map[string]any) supplierDTO {
	st, raw := s.admin.Do(http.MethodPost, supAPI, body)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var d supplierDTO
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	return d
}

func (s *SuppliersLiveSuite) update(id string, body map[string]any) supplierDTO {
	st, raw := s.admin.Do(http.MethodPut, supAPI+"/"+id, body)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var d supplierDTO
	s.Require().NoError(json.Unmarshal(raw, &d), string(raw))
	return d
}

func names(l supplierListDTO) []string {
	out := make([]string, len(l.Items))
	for i, it := range l.Items {
		out[i] = it.Name
	}
	return out
}

func rep(n int) string { return strings.Repeat("я", n) }

// TS-020: создание со всеми полями (trim), правка, архив и возврат одним PUT; уникальность
// названия без регистра и с пробелами по краям, включая архивные и кроме самого себя.
func (s *SuppliersLiveSuite) TestTS020_CRUD_DuplicateArchiveRestore() {
	d := s.create(map[string]any{
		"name": "  ООО Картридж  ", "contact_name": " Иван Петров ", "phone": " +7 700 123-45-67 ",
		"address": " Алматы, Абая 1 ", "note": " оптовик ",
	})
	s.NotEmpty(d.ID)
	s.Equal("ООО Картридж", d.Name, "trim")
	s.Equal("Иван Петров", d.ContactName)
	s.Equal("+7 700 123-45-67", d.Phone)
	s.Equal("Алматы, Абая 1", d.Address)
	s.Equal("оптовик", d.Note)
	s.True(d.IsActive, "новый поставщик активен")
	s.NotEmpty(d.CreatedAt)
	s.NotEmpty(d.UpdatedAt)

	// только обязательное поле — остальные пустые строки.
	bare := s.create(map[string]any{"name": "Минимальный поставщик"})
	s.Equal("", bare.ContactName)
	s.Equal("", bare.Phone)
	s.Equal("", bare.Address)
	s.Equal("", bare.Note)

	// дубль: другой регистр + пробелы по краям → 400 name: duplicate, запись не создана.
	before := s.list(nil).Total
	st, raw := s.admin.Do(http.MethodPost, supAPI, map[string]any{"name": "  ооо КАРТРИДЖ  "})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "duplicate"}, s.fields(raw))
	s.Equal(before, s.list(nil).Total, "дубль не создан")

	// правка всех полей; своё же название (в другом регистре) — не дубль.
	upd := s.update(d.ID, map[string]any{
		"name": "ооо картридж", "contact_name": "Пётр", "phone": "87001234567", "address": "Астана", "note": "", "is_active": true,
	})
	s.Equal(d.ID, upd.ID)
	s.Equal("ооо картридж", upd.Name)
	s.Equal("Пётр", upd.ContactName)
	s.Equal("87001234567", upd.Phone)
	s.Equal("Астана", upd.Address)
	s.Equal("", upd.Note)
	s.True(upd.IsActive)

	// повтор того же PUT безопасен (то же итоговое состояние).
	again := s.update(d.ID, map[string]any{
		"name": "ооо картридж", "contact_name": "Пётр", "phone": "87001234567", "address": "Астана", "note": "", "is_active": true,
	})
	s.Equal(upd.Name, again.Name)
	s.Equal(upd.IsActive, again.IsActive)

	// архив.
	arch := s.update(d.ID, map[string]any{"name": "ооо картридж", "is_active": false})
	s.False(arch.IsActive)
	s.Equal("", arch.ContactName, "необязательные поля, не переданные в PUT, очищаются")

	// дубль архивного названия — тоже 400 (уникальность включает архивные).
	st, raw = s.admin.Do(http.MethodPost, supAPI, map[string]any{"name": "ООО КАРТРИДЖ"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "duplicate"}, s.fields(raw))

	// переименование в чужое название (в т.ч. архивное) — 400.
	st, raw = s.admin.Do(http.MethodPut, supAPI+"/"+bare.ID, map[string]any{"name": " ООО картридж ", "is_active": true})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "duplicate"}, s.fields(raw))

	// возврат из архива.
	back := s.update(d.ID, map[string]any{"name": "ооо картридж", "is_active": true})
	s.True(back.IsActive)
}

// TS-020: name required / 201 символ; phone без цифр и 51 символ; длины contact_name (200),
// address (500), note (1000); мусор → 400 (не 500), запись не создаётся.
func (s *SuppliersLiveSuite) TestTS020_Validation() {
	before := s.list(nil).Total

	for _, c := range []struct {
		why  string
		body map[string]any
		want map[string]string
	}{
		{"нет name", map[string]any{}, map[string]string{"name": "required"}},
		{"name из пробелов", map[string]any{"name": "   "}, map[string]string{"name": "required"}},
		{"name 201", map[string]any{"name": rep(201)}, map[string]string{"name": "too_long"}},
		{"name NUL", map[string]any{"name": "Поставщик\u0000"}, map[string]string{"name": "invalid_text"}},
		{"phone без цифр", map[string]any{"name": "V1", "phone": "нет телефона"}, map[string]string{"phone": "invalid"}},
		{"phone 51", map[string]any{"name": "V1", "phone": strings.Repeat("1", 51)}, map[string]string{"phone": "too_long"}},
		{"contact_name 201", map[string]any{"name": "V1", "contact_name": rep(201)}, map[string]string{"contact_name": "too_long"}},
		{"address 501", map[string]any{"name": "V1", "address": rep(501)}, map[string]string{"address": "too_long"}},
		{"note 1001", map[string]any{"name": "V1", "note": rep(1001)}, map[string]string{"note": "too_long"}},
		{"всё сразу", map[string]any{"name": "", "contact_name": rep(201), "phone": "abc", "address": rep(501), "note": rep(1001)},
			map[string]string{"name": "required", "contact_name": "too_long", "phone": "invalid", "address": "too_long", "note": "too_long"}},
	} {
		st, raw := s.admin.Do(http.MethodPost, supAPI, c.body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", c.why, raw)
		s.Equal(c.want, s.fields(raw), c.why)
	}

	// мусорное тело, null, неизвестное поле, неверный тип — 400, не 500.
	for _, body := range []any{`{`, `null`, `[]`, `{"name":"X","extra":1}`, `{"name":123}`, `{"name":"X","phone":7}`} {
		st, raw := s.admin.Do(http.MethodPost, supAPI, body)
		s.Equal(http.StatusBadRequest, st, "%v → %s", body, raw)
	}
	s.Equal(before, s.list(nil).Total, "невалидные запросы ничего не создали")

	// границы валидны: 200 / 200 / 50 / 500 / 1000.
	ok := s.create(map[string]any{
		"name": rep(200), "contact_name": rep(200), "phone": strings.Repeat("1", 50), "address": rep(500), "note": rep(1000),
	})
	s.Equal(rep(200), ok.Name)
	s.Equal(rep(1000), ok.Note)

	// те же правила в PUT.
	st, raw := s.admin.Do(http.MethodPut, supAPI+"/"+ok.ID, map[string]any{"name": " ", "phone": "---", "is_active": true})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "required", "phone": "invalid"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodPut, supAPI+"/"+ok.ID, map[string]any{"name": rep(201), "note": rep(1001), "is_active": true})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "too_long", "note": "too_long"}, s.fields(raw))
}

// TS-020: is_active обязателен в PUT; несуществующий и битый id → 404 SUPPLIER_NOT_FOUND
// (не 500); удаление — только с причиной (soft-delete-warehouses-suppliers DS-020).
func (s *SuppliersLiveSuite) TestTS020_PutIsActiveRequired_NotFound_NoDelete() {
	d := s.create(map[string]any{"name": "Поставщик-PUT"})

	st, raw := s.admin.Do(http.MethodPut, supAPI+"/"+d.ID, map[string]any{"name": "Поставщик-PUT"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"is_active": "required"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodPut, supAPI+"/"+d.ID, `{"name":"Поставщик-PUT","is_active":null}`)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"is_active": "required"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodPut, supAPI+"/"+d.ID, `{"name":"Поставщик-PUT","is_active":"yes"}`)
	s.Equal(http.StatusBadRequest, st, string(raw))

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodPut, supAPI+"/"+id, map[string]any{"name": "X", "is_active": true})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		s.Contains(string(raw), "SUPPLIER_NOT_FOUND", id)
	}

	// soft-delete-warehouses-suppliers §6.3: удаление появилось, но только с причиной — без тела
	// поставщик остаётся на месте (полный цикл удаления — suppliers_soft_delete_http_live_test.go).
	st, raw = s.admin.Do(http.MethodDelete, supAPI+"/"+d.ID, nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"reason": "required"}, s.fields(raw))
	l := s.list(url.Values{"q": {"Поставщик-PUT"}})
	s.Equal(1, l.Total, "поставщик на месте")
}

// TS-020 (DS-007, §3 К-3): параллельное создание с одним названием → ровно один 201,
// остальные — 400 name: duplicate (уникальный индекс), в БД одна запись.
func (s *SuppliersLiveSuite) TestTS020_ConcurrentCreate_ExactlyOne201() {
	const n = 8
	for round := 0; round < 3; round++ {
		name := "Гонка-" + string(rune('A'+round))
		codes := make([]int, n)
		bodies := make([][]byte, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				// варианты регистра/пробелов — одно и то же название для индекса.
				v := name
				if i%2 == 1 {
					v = "  " + strings.ToLower(name) + " "
				}
				codes[i], bodies[i] = s.admin.Do(http.MethodPost, supAPI, map[string]any{"name": v})
			}(i)
		}
		wg.Wait()

		created := 0
		for i, c := range codes {
			switch c {
			case http.StatusCreated:
				created++
			case http.StatusBadRequest:
				s.Equal(map[string]string{"name": "duplicate"}, s.fields(bodies[i]), "раунд %d", round)
			default:
				s.Failf("неожиданный статус", "раунд %d: %d %s", round, c, bodies[i])
			}
		}
		s.Equal(1, created, "раунд %d: ровно один 201", round)
		s.Equal(1, s.list(url.Values{"q": {name}}).Total, "раунд %d: одна запись", round)
	}
}

// TS-020: фильтры списка q (подстрока без регистра, спецсимволы LIKE — буквально) и
// is_active; сортировка «активные раньше архивных, затем по названию»; конверт
// {items,total,limit,offset}, limit по умолчанию 50, максимум 500; is_active=maybe → 400.
func (s *SuppliersLiveSuite) TestTS020_ListFilters_Envelope() {
	b := s.create(map[string]any{"name": "Фильтр-Бета"})
	a := s.create(map[string]any{"name": "фильтр-альфа"})
	g := s.create(map[string]any{"name": "ФИЛЬТР-Гамма"})
	s.create(map[string]any{"name": "Фильтр100%_скидка"})
	s.update(a.ID, map[string]any{"name": a.Name, "is_active": false})

	// конверт и дефолты.
	st, raw := s.admin.Do(http.MethodGet, supAPI, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var env map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &env))
	s.Len(env, 4, "конверт — ровно items,total,limit,offset: %s", raw)
	for _, k := range []string{"items", "total", "limit", "offset"} {
		s.Contains(env, k)
	}
	all := s.list(nil)
	s.Equal(50, all.Limit, "limit по умолчанию")
	s.Equal(0, all.Offset)
	s.GreaterOrEqual(all.Total, 4)

	// q: подстрока без регистра; активные раньше архивных, затем по названию.
	l := s.list(url.Values{"q": {"фИЛЬТР-"}})
	s.Equal(3, l.Total)
	s.Equal([]string{b.Name, g.Name, a.Name}, names(l), "активные (Бета, Гамма) раньше архивного (альфа)")

	// is_active.
	l = s.list(url.Values{"q": {"фильтр-"}, "is_active": {"true"}})
	s.Equal([]string{b.Name, g.Name}, names(l))
	s.Equal(2, l.Total)
	l = s.list(url.Values{"q": {"фильтр-"}, "is_active": {"false"}})
	s.Equal([]string{a.Name}, names(l))
	s.False(l.Items[0].IsActive)

	// спецсимволы LIKE ищутся буквально.
	s.Equal([]string{"Фильтр100%_скидка"}, names(s.list(url.Values{"q": {"100%_"}})))
	s.Equal(0, s.list(url.Values{"q": {"Фильтр_Бета"}}).Total, "_ — не «любой символ»")
	s.Equal(0, s.list(url.Values{"q": {"нет-такого-поставщика"}}).Total)
	s.Empty(s.list(url.Values{"q": {"нет-такого-поставщика"}}).Items)

	// limit / offset: total — под фильтрами, а не размер страницы.
	l = s.list(url.Values{"q": {"фильтр-"}, "limit": {"2"}, "offset": {"1"}})
	s.Equal(3, l.Total)
	s.Equal(2, l.Limit)
	s.Equal(1, l.Offset)
	s.Equal([]string{g.Name, a.Name}, names(l))
	s.Equal(500, s.list(url.Values{"limit": {"100000"}}).Limit, "limit не больше 500")

	// негатив: is_active=maybe → 400 is_active: invalid; q длиннее 200 → 400.
	for _, v := range []string{"maybe", "1", "TRUE"} {
		st, raw = s.admin.Do(http.MethodGet, supAPI+"?is_active="+v, nil)
		s.Equal(http.StatusBadRequest, st, "is_active=%s → %s", v, raw)
		s.Equal(map[string]string{"is_active": "invalid"}, s.fields(raw), v)
	}
	st, raw = s.admin.Do(http.MethodGet, supAPI+"?"+url.Values{"q": {rep(201)}}.Encode(), nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"q": "too_long"}, s.fields(raw))
}

// TS-020 (DS-007, abuse case 1): executor → 403 на все три endpoint'а и на SSR-экран, пункта
// меню нет; аноним → 401.
func (s *SuppliersLiveSuite) TestTS020_ExecutorForbidden() {
	target := s.create(map[string]any{"name": "Поставщик-для-прав"})
	s.SeedUser("sup-exec", "password1", "Поставщики Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("sup-exec", "password1")

	for _, cs := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, supAPI, nil},
		{http.MethodPost, supAPI, map[string]any{"name": "Чужой поставщик"}},
		{http.MethodPut, supAPI + "/" + target.ID, map[string]any{"name": "Взлом", "is_active": false}},
		{http.MethodPut, supAPI + "/00000000-0000-0000-0000-000000000000", map[string]any{"name": "X", "is_active": true}},
	} {
		st, raw := exec.Do(cs.method, cs.path, cs.body)
		s.Equal(http.StatusForbidden, st, "%s %s → %s", cs.method, cs.path, raw)
	}
	l := s.list(url.Values{"q": {"Поставщик-для-прав"}})
	s.Require().Len(l.Items, 1)
	s.True(l.Items[0].IsActive, "executor ничего не изменил")
	s.Equal(0, s.list(url.Values{"q": {"Чужой поставщик"}}).Total, "executor ничего не создал")

	resp := exec.Request(http.MethodGet, "/suppliers", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)

	resp = exec.Request(http.MethodGet, "/", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.NotContains(string(body), `href="/suppliers"`, "executor не видит пункт «Поставщики»")

	anon := s.NewBrowser()
	st, raw := anon.Do(http.MethodGet, supAPI, nil)
	s.Equal(http.StatusUnauthorized, st, string(raw))
}

// SSR: оболочка экрана «Поставщики» для admin — пункт сайдбара, корень экрана, форма и
// скрипт; данные грузит js/suppliers.js через API.
func (s *SuppliersLiveSuite) TestSSR_PageShell() {
	for _, path := range []string{"/suppliers", "/suppliers?new=1", "/suppliers?id=00000000-0000-0000-0000-000000000000"} {
		resp := s.admin.Request(http.MethodGet, path, nil, nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode, path)
		html := string(body)
		s.Contains(html, `data-testid="suppliers-root"`, path)
		s.Contains(html, `data-testid="supplier-form"`, path)
		s.Contains(html, `data-testid="supplier-add"`, path)
		s.Contains(html, `/assets/ssr-shared/js/suppliers.js`, path)
		s.Contains(html, `href="/suppliers"`, "пункт «Поставщики» в сайдбаре admin")
		s.Contains(html, "Поставщики")
	}

	resp := s.admin.Request(http.MethodGet, "/assets/ssr-shared/js/suppliers.js", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
}
