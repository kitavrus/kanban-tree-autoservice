//go:build integration

package warehouses_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// WarehousesLiveSuite (§TD-8): справочник складов (docs/features/warehouses/draft-spec.md
// DS-001/DS-010/DS-011) — TS-002 (CRUD, дубль имени вкл. архивные → 400 fields[name],
// переименование, архив), TS-014 (executor → 403 API и не видит «Склады» в сайдбаре), SSR.
type WarehousesLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestWarehousesLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка вида warehouses (soft-delete-warehouses-suppliers
	// DS-027, TestPurge_Warehouses).
	suite.Run(t, &WarehousesLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *WarehousesLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("wh-admin", "password1", "Склады Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("wh-admin", "password1")
}

type warehouseDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	IsActive bool   `json:"is_active"`
}

type warehouseListDTO struct {
	Items  []warehouseDTO `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func (s *WarehousesLiveSuite) fields(body []byte) map[string]string {
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

func (s *WarehousesLiveSuite) list(query string) warehouseListDTO {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/warehouses"+query, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l warehouseListDTO
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	return l
}

func (s *WarehousesLiveSuite) create(name string) warehouseDTO {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": name})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var w warehouseDTO
	s.Require().NoError(json.Unmarshal(raw, &w), string(raw))
	return w
}

func (s *WarehousesLiveSuite) byName(l warehouseListDTO, name string) (warehouseDTO, bool) {
	for _, w := range l.Items {
		if w.Name == name {
			return w, true
		}
	}
	return warehouseDTO{}, false
}

// Сид миграции 0008: «Алматы» (1), «Москва» (2); конверт списка §API-1.
func (s *WarehousesLiveSuite) TestSeed_AlmatyMoscow_Envelope() {
	l := s.list("")
	s.GreaterOrEqual(l.Total, 2)
	s.Equal(500, l.Limit)
	s.Equal(0, l.Offset)
	s.Require().GreaterOrEqual(len(l.Items), 2)
	s.Equal("Алматы", l.Items[0].Name)
	s.Equal(1, l.Items[0].Position)
	s.Equal("Москва", l.Items[1].Name)
	s.Equal(2, l.Items[1].Position)
}

// TS-002: создать, дубль имени (регистр/пробелы, вкл. архивные) → 400 fields[name];
// переименовать; архивировать.
func (s *WarehousesLiveSuite) TestTS002_CRUD_DuplicateRenameArchive() {
	w := s.create("  Астана  ")
	s.Equal("Астана", w.Name, "trim")
	s.True(w.IsActive)
	s.Greater(w.Position, 2, "новый склад — в конец списка")

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": " астана "})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "taken"}, s.fields(raw))

	// переименование.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": "Астана-2", "is_active": true})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var upd warehouseDTO
	s.Require().NoError(json.Unmarshal(raw, &upd))
	s.Equal("Астана-2", upd.Name)

	// архив: из активного списка пропадает, в include_archived — есть.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": "Астана-2", "is_active": false})
	s.Require().Equal(http.StatusOK, st, string(raw))
	_, ok := s.byName(s.list(""), "Астана-2")
	s.False(ok, "архивный склад не в активном списке")
	arch, ok := s.byName(s.list("?include_archived=true"), "Астана-2")
	s.True(ok)
	s.False(arch.IsActive)

	// дубль архивного имени — тоже 400, свой код.
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": "АСТАНА-2"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "archived"}, s.fields(raw))

	// переименование в чужое имя — 400.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": "москва", "is_active": false})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"name": "taken"}, s.fields(raw))
}

// Валидация, 404, мусор → 400 (не 500); удаление — только с причиной (DS-020).
func (s *WarehousesLiveSuite) TestValidation_NotFound_NoDelete() {
	for _, body := range []any{map[string]any{}, map[string]any{"name": "   "}, `{`, `null`, `{"name":"X","extra":1}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", body)
		s.Equal(http.StatusBadRequest, st, "%v → %s", body, raw)
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/warehouses", map[string]any{"name": string(make([]byte, 0)) + "Склад\u0000"})
	s.Equal(http.StatusBadRequest, st, string(raw))

	w := s.create("Склад-валидация")
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": "Склад-валидация"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"is_active": "required"}, s.fields(raw))

	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000"} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+id, map[string]any{"name": "X", "is_active": true})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}

	// soft-delete-warehouses-suppliers §6.1: удаление появилось, но только с причиной — без тела
	// склад остаётся на месте (полный цикл — warehouses_soft_delete_http_live_test.go).
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/warehouses/"+w.ID, nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"reason": "required"}, s.fields(raw))
	_, ok := s.byName(s.list(""), "Склад-валидация")
	s.True(ok, "склад на месте")
}

// D47: последний активный склад архивировать нельзя.
func (s *WarehousesLiveSuite) TestLastActive_CannotArchive() {
	l := s.list("")
	for i, w := range l.Items {
		if i == 0 {
			continue
		}
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": false})
		s.Require().Equal(http.StatusOK, st, string(raw))
		defer func(w warehouseDTO) {
			st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": true})
			s.Require().Equal(http.StatusOK, st, string(raw))
		}(w)
	}
	last := l.Items[0]
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+last.ID, map[string]any{"name": last.Name, "is_active": false})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Contains(string(raw), "WAREHOUSE_LAST_ACTIVE")
}

// Код-ревью MEDIUM-3 (D47 под гонкой): два последних активных склада архивируются
// параллельно — проверка и запись в одной tx под FOR UPDATE, ровно один 200 и один 409,
// активный склад остаётся.
func (s *WarehousesLiveSuite) TestLastActive_ConcurrentArchive() {
	l := s.list("")
	s.Require().GreaterOrEqual(len(l.Items), 2)
	for i, w := range l.Items {
		if i < 2 {
			continue
		}
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": false})
		s.Require().Equal(http.StatusOK, st, string(raw))
		defer func(w warehouseDTO) {
			st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": true})
			s.Require().Equal(http.StatusOK, st, string(raw))
		}(w)
	}
	pair := l.Items[:2]
	for round := 0; round < 5; round++ {
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i, w := range pair {
			wg.Add(1)
			go func(i int, w warehouseDTO) {
				defer wg.Done()
				codes[i], _ = s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": false})
			}(i, w)
		}
		wg.Wait()
		s.ElementsMatch([]int{http.StatusOK, http.StatusConflict}, codes, "раунд %d", round)
		s.Len(s.list("").Items, 1, "активный склад остался")
		for _, w := range pair { // вернуть оба в актив перед следующим раундом
			st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": true})
			s.Require().Equal(http.StatusOK, st, string(raw))
		}
	}
}

// TS-014 (часть подфазы A): executor не видит «Склады» — 403 на API и SSR, нет пункта меню.
func (s *WarehousesLiveSuite) TestTS014_ExecutorForbidden() {
	s.SeedUser("wh-exec", "password1", "Склады Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("wh-exec", "password1")

	for _, cs := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/warehouses", nil},
		{http.MethodPost, "/api/v1/warehouses", map[string]any{"name": "Чужой"}},
		{http.MethodPut, "/api/v1/warehouses/00000000-0000-0000-0000-000000000000", map[string]any{"name": "X", "is_active": true}},
	} {
		st, raw := exec.Do(cs.method, cs.path, cs.body)
		s.Equal(http.StatusForbidden, st, "%s %s → %s", cs.method, cs.path, raw)
	}

	resp := exec.Request(http.MethodGet, "/warehouses", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)

	resp = exec.Request(http.MethodGet, "/", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.NotContains(string(body), `href="/warehouses"`, "executor не видит пункт «Склады»")

	anon := s.NewBrowser()
	st, raw := anon.Do(http.MethodGet, "/api/v1/warehouses", nil)
	s.Equal(http.StatusUnauthorized, st, string(raw))
}

// SSR: список, пункт сайдбара, PRG-добавление, ошибка дубля под полем, правка, архив.
func (s *WarehousesLiveSuite) TestSSR_ListCreateEditArchive() {
	resp := s.admin.Request(http.MethodGet, "/warehouses", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Contains(string(body), `data-testid="warehouses-table"`)
	s.Contains(string(body), `href="/warehouses"`, "пункт «Склады» в сайдбаре admin")
	s.Contains(string(body), "Алматы")

	resp = s.admin.Request(http.MethodPost, "/warehouses", url.Values{"name": {"SSR-Склад"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/warehouses?flash=created", resp.Header.Get("Location"))

	resp = s.admin.Request(http.MethodPost, "/warehouses", url.Values{"name": {"ssr-склад"}}, nil)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusUnprocessableEntity, resp.StatusCode)
	s.Contains(string(body), "Такой склад уже есть")

	w, ok := s.byName(s.list(""), "SSR-Склад")
	s.Require().True(ok)
	resp = s.admin.Request(http.MethodPost, "/warehouses/"+w.ID, url.Values{"name": {"SSR-Склад-2"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	resp = s.admin.Request(http.MethodPost, "/warehouses/"+w.ID+"/archive", url.Values{}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	arch, ok := s.byName(s.list("?include_archived=true"), "SSR-Склад-2")
	s.Require().True(ok)
	s.False(arch.IsActive)

	resp = s.admin.Request(http.MethodGet, "/warehouses/00000000-0000-0000-0000-000000000000", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusNotFound, resp.StatusCode)
}
