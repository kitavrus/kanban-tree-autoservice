//go:build integration

package equipmenttypes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// TypesLiveSuite (§TD-8): типы оборудования — API, права (ADR-004), уникальность имени
// (ADR-007), удаление используемого типа.
type TypesLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestTypesLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка вида equipment-types (soft-delete-catalog §6.9, DS-027).
	suite.Run(t, &TypesLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *TypesLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("catalog-admin", "password1", "Каталог Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("catalog-admin", "password1")
}

type typeDTO struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SpecKeys       []string `json:"spec_keys"`
	IsActive       bool     `json:"is_active"`
	EquipmentCount int      `json:"equipment_count"`
}

// delReason — тело DELETE мягкого удаления (soft-delete-catalog DS-020, DS-028).
var delReason = map[string]any{"reason": "Создан по ошибке"}

type apiErr struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

func (s *TypesLiveSuite) errOf(body []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *TypesLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *TypesLiveSuite) create(b *testutil.Browser, body map[string]any) (int, typeDTO, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/equipment-types", body)
	var t typeDTO
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &t))
	}
	return st, t, raw
}

func (s *TypesLiveSuite) put(b *testutil.Browser, id string, body map[string]any) (int, []byte) {
	return b.Do(http.MethodPut, "/api/v1/equipment-types/"+id, body)
}

func card(t typeDTO, patch map[string]any) map[string]any {
	m := map[string]any{"name": t.Name, "spec_keys": t.SpecKeys, "is_active": t.IsActive}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *TypesLiveSuite) newType(name string) typeDTO {
	st, tp, raw := s.create(s.admin, map[string]any{"name": name, "spec_keys": []string{"Мощность", "Напряжение"}})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return tp
}

// ---- LV-030: happy path — создание/получение/список ----

func (s *TypesLiveSuite) TestCreateGetList() {
	st, tp, raw := s.create(s.admin, map[string]any{"name": "  Холодильник  ", "spec_keys": []string{" Объём ", "объём", "Тип хладагента"}})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("Холодильник", tp.Name)
	s.Equal([]string{"Объём", "Тип хладагента"}, tp.SpecKeys, "дедуп case-insensitive, порядок первого вхождения")
	s.True(tp.IsActive)
	s.Equal(0, tp.EquipmentCount)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment-types/"+tp.ID, nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), `"name":"Холодильник"`)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment-types?q=холод", nil)
	s.Equal(http.StatusOK, st)
	var list struct {
		Items  []typeDTO `json:"items"`
		Total  int       `json:"total"`
		Limit  int       `json:"limit"`
		Offset int       `json:"offset"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(1, list.Total)
	s.Equal(500, list.Limit, "лимит по умолчанию для lookup-списка — 500")
}

// ---- LV-035 (equipment-types часть): мусор в limit/offset/include_archived — дефолты,
// не 500 ----

func (s *TypesLiveSuite) TestList_GarbageParams_DefaultsNot500() {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment-types?limit=-5&offset=-1&include_archived=maybe", nil)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- LV-040 (equipment-types часть, authz): исполнитель читает только активные типы,
// мутации 403; аноним 401 ----

func (s *TypesLiveSuite) TestAuthz_ExecutorReadOnlyActive_MutationsForbidden_AnonUnauthorized() {
	active := s.newType("Насос")
	archived := s.newType("Списанный компрессор")
	st, raw := s.put(s.admin, archived.ID, card(archived, map[string]any{"is_active": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	s.SeedUser("catalog-exec", "password1", "Каталог Исп", "executor", true)
	exec := s.NewBrowser()
	exec.Login("catalog-exec", "password1")

	st, raw = exec.Do(http.MethodGet, "/api/v1/equipment-types?include_archived=true", nil)
	s.Equal(http.StatusOK, st, "чтение активных типов разрешено исполнителю")
	s.NotContains(string(raw), archived.Name, "исполнитель не видит архив даже с include_archived=true (ADR-004)")
	s.Contains(string(raw), active.Name)

	st, raw = exec.Do(http.MethodGet, "/api/v1/equipment-types/"+active.ID, nil)
	s.Equal(http.StatusOK, st, string(raw))

	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/equipment-types"},
		{http.MethodPut, "/api/v1/equipment-types/" + active.ID},
		{http.MethodDelete, "/api/v1/equipment-types/" + active.ID},
	} {
		st, raw := exec.Do(c.method, c.path, map[string]any{"name": "x"})
		s.Equal(http.StatusForbidden, st, "%s %s", c.method, c.path)
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/equipment-types", nil)
	s.Equal(http.StatusUnauthorized, st)
}

// ---- LV-036 (equipment-types часть): валидация полей / мусорное тело → 400, не 500 ----

func (s *TypesLiveSuite) TestCreate_Validation() {
	st, _, raw := s.create(s.admin, map[string]any{})
	s.Equal(http.StatusBadRequest, st)
	s.Equal(map[string]string{"name": "required"}, s.fields(raw))

	st, _, raw = s.create(s.admin, map[string]any{"name": strings.Repeat("я", 101)})
	s.Equal(http.StatusBadRequest, st)
	s.Equal("too_long", s.fields(raw)["name"])

	longKeys := make([]string, 51)
	for i := range longKeys {
		longKeys[i] = strings.Repeat("k", i+1) // все разные — не схлопнутся дедупом
	}
	st, _, raw = s.create(s.admin, map[string]any{"name": "Дрель", "spec_keys": longKeys})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_many", s.fields(raw)["spec_keys"])

	st, _, raw = s.create(s.admin, map[string]any{"name": "Дрель-2", "spec_keys": []string{strings.Repeat("k", 101)}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("too_long", s.fields(raw)["spec_keys[0]"])

	for _, body := range []string{`{`, `null`, `"str"`, `{"is_active":"yes"}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/equipment-types", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}

	st, tp, raw := s.create(s.admin, map[string]any{"name": "Без is_active"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/equipment-types/"+tp.ID, map[string]any{"name": "Без is_active"})
	s.Equal(http.StatusBadRequest, st)
	s.Equal("required", s.fields(raw)["is_active"])
}

// ---- code review MEDIUM-1: NUL-байт → 400, не 500 (§TD-4) ----

func (s *TypesLiveSuite) TestCreate_NulByte_Not500() {
	st, _, raw := s.create(s.admin, map[string]any{"name": "Тип\u0000"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid_text", s.fields(raw)["name"])

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment-types?q=%00", nil)
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- (equipment-types, аналог LV-008): битый/несуществующий uuid → 404, не 500 ----

func (s *TypesLiveSuite) TestBadOrMissingID_404() {
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000", "1'; DROP TABLE equipment_types;--"} {
		esc := url.PathEscape(id)
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment-types/"+esc, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.put(s.admin, esc, map[string]any{"name": "x", "is_active": true})
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment-types/"+esc, delReason)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// ---- LV-031: дубль имени активного (409 TAKEN) vs архивного (409 ARCHIVED) ----

func (s *TypesLiveSuite) TestCreate_DuplicateName_TakenVsArchived() {
	t1 := s.newType("Кондиционер")
	st, _, raw := s.create(s.admin, map[string]any{"name": "КОНДИЦИОНЕР"})
	s.Equal(http.StatusConflict, st)
	s.Equal("EQUIPMENT_TYPE_NAME_TAKEN", s.errOf(raw).Code)

	st, raw = s.put(s.admin, t1.ID, card(t1, map[string]any{"is_active": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))

	st, _, raw = s.create(s.admin, map[string]any{"name": "кондиционер"})
	s.Equal(http.StatusConflict, st)
	s.Equal("EQUIPMENT_TYPE_NAME_ARCHIVED", s.errOf(raw).Code)

	t2 := s.newType("Обогреватель")
	st, raw = s.put(s.admin, t2.ID, card(t2, map[string]any{"name": "кондиционер"}))
	s.Equal(http.StatusConflict, st, string(raw))
}

// ---- LV-032 → soft-delete-catalog DS-014: используемый тип удаляется мягко (отказ
// EQUIPMENT_TYPE_IN_USE снят) — единица остаётся действующей; свободный тоже удаляется ----

func (s *TypesLiveSuite) TestDelete_UsedAndFreeDelete200() {
	used := s.newType("Компрессор")
	var clientID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO clients (company_name) VALUES ('ООО Клиент') RETURNING id`).Scan(&clientID))
	var equipmentID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO equipment (client_id, type_id, model) VALUES ($1::uuid, $2::uuid, 'M1') RETURNING id`,
		clientID, used.ID).Scan(&equipmentID))

	st, raw := s.admin.Do(http.MethodDelete, "/api/v1/equipment-types/"+used.ID, delReason)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"deleted_at"`)
	var unitLive bool
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`SELECT deleted_at IS NULL FROM equipment WHERE id = $1::uuid AND type_id = $2::uuid`, equipmentID, used.ID).Scan(&unitLive))
	s.True(unitLive, "единица удалённого типа остаётся действующей")

	free := s.newType("Свободный тип")
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/equipment-types/"+free.ID, delReason)
	s.Equal(http.StatusOK, st, string(raw))
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/equipment-types/"+free.ID, nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- LV-034: SSR archive/restore + мягкое удаление используемого типа формой с причиной
// (soft-delete-catalog §6.9, §8.1, DS-032) ----

func (s *TypesLiveSuite) TestSSR_ArchiveRestoreDeleteUsed() {
	st, page := s.admin.Do(http.MethodGet, "/catalog/equipment-types", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="types-table"`)

	form := url.Values{"name": {"SSR Чиллер"}, "spec_keys": {"Мощность, Объём"}}
	resp := s.admin.Request(http.MethodPost, "/catalog/equipment-types", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/equipment-types?flash=created", resp.Header.Get("Location"))

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/equipment-types?q=SSR+Чиллер", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items []typeDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Require().Len(list.Items, 1, string(raw))
	tp := list.Items[0]

	// archive — редирект сохраняет фильтр `archived`, с которым отправлена форма
	// (design-go-layers.md §SSR Routes); форма отправлена без него (активный список) →
	// без archived=1 в редиректе (code review LOW#3 — раньше было наоборот).
	resp = s.admin.Request(http.MethodPost, "/catalog/equipment-types/"+tp.ID+"/archive", url.Values{}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/equipment-types?flash=archived", resp.Header.Get("Location"))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/equipment-types/"+tp.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	var got typeDTO
	s.Require().NoError(json.Unmarshal(raw, &got))
	s.False(got.IsActive)

	// restore — редирект всегда с archived=1 (design-go-layers.md §SSR Routes): тип обычно
	// восстанавливают, просматривая архивный список.
	resp = s.admin.Request(http.MethodPost, "/catalog/equipment-types/"+tp.ID+"/restore", url.Values{}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/equipment-types?archived=1&flash=restored", resp.Header.Get("Location"))

	// архивирование из формы, отправленной с архивным фильтром (скрытое поле `archived`
	// строки списка, list.html) — фильтр сохраняется в редиректе.
	resp = s.admin.Request(http.MethodPost, "/catalog/equipment-types/"+tp.ID+"/archive", url.Values{"archived": {"1"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/equipment-types?archived=1&flash=archived", resp.Header.Get("Location"))
	resp = s.admin.Request(http.MethodPost, "/catalog/equipment-types/"+tp.ID+"/restore", url.Values{}, nil)
	resp.Body.Close()

	// используемый тип: без причины — форма (422) с окном «Удаление» и текстом под полем; с
	// причиной — тип удалён мягко (единица не мешает), переход к списку с тостом.
	var clientID, equipmentID string
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO clients (company_name) VALUES ('ООО СSSR-Клиент') RETURNING id`).Scan(&clientID))
	s.Require().NoError(s.Pool.QueryRow(s.T().Context(),
		`INSERT INTO equipment (client_id, type_id, model) VALUES ($1::uuid, $2::uuid, 'M1') RETURNING id`,
		clientID, tp.ID).Scan(&equipmentID))

	st, page = s.admin.Do(http.MethodPost, "/catalog/equipment-types/"+tp.ID+"/delete", url.Values{"reason": {"   "}})
	s.Equal(http.StatusUnprocessableEntity, st, string(page))
	s.Contains(string(page), `data-delete-error="Укажите причину удаления"`)
	s.Contains(string(page), `data-testid="type-archive"`)

	resp = s.admin.Request(http.MethodPost, "/catalog/equipment-types/"+tp.ID+"/delete", url.Values{"reason": {"Тип не нужен"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/catalog/equipment-types?flash=deleted", resp.Header.Get("Location"))
	st, page = s.admin.Do(http.MethodGet, "/catalog/equipment-types?flash=deleted", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-kt-toast="Удалено. Восстановить можно в разделе «Удалённые»"`)
	s.NotContains(string(page), "SSR Чиллер", "удалённого типа в списке нет")

	// архив, возврат из архива, правка и повторное удаление удалённого типа — страница 404.
	for _, path := range []string{"/archive", "/restore", "/delete", ""} {
		st, _ = s.admin.Do(http.MethodPost, "/catalog/equipment-types/"+tp.ID+path, url.Values{"reason": {"Повтор"}, "name": {"x"}})
		s.Equal(http.StatusNotFound, st, "POST %s", path)
	}
	st, _ = s.admin.Do(http.MethodGet, "/catalog/equipment-types/"+tp.ID, nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- LV-041 (equipment-types часть): SSR — исполнитель 403, аноним редирект на /login ----

func (s *TypesLiveSuite) TestSSR_ExecutorForbidden_AnonRedirect() {
	s.SeedUser("catalog-exec-ssr", "password1", "Каталог Исп СSSR", "executor", true)
	exec := s.NewBrowser()
	exec.Login("catalog-exec-ssr", "password1")

	st, page := exec.Do(http.MethodGet, "/catalog/equipment-types", nil)
	s.Equal(http.StatusForbidden, st)
	s.Contains(string(page), "Нет доступа")

	tp := s.newType("Тип для 403")
	for _, path := range []string{"/catalog/equipment-types/new", "/catalog/equipment-types/" + tp.ID} {
		st, _ := exec.Do(http.MethodGet, path, nil)
		s.Equal(http.StatusForbidden, st, path)
	}
	for _, path := range []string{
		"/catalog/equipment-types",
		"/catalog/equipment-types/" + tp.ID,
		"/catalog/equipment-types/" + tp.ID + "/archive",
		"/catalog/equipment-types/" + tp.ID + "/delete",
	} {
		st, _ := exec.Do(http.MethodPost, path, url.Values{"name": {"x"}})
		s.Equal(http.StatusForbidden, st, path)
	}

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/catalog/equipment-types", nil)
	s.Equal(http.StatusFound, st, "аноним — редирект на /login")
}

// SSR-список отдаёт не больше 500 типов — при большем числе совпадений экран говорит
// «показаны первые N», а не обрезает список молча.
func (s *TypesLiveSuite) TestSSRList_TruncatedNotice() {
	_, err := s.Pool.Exec(context.Background(),
		`INSERT INTO equipment_types (name) SELECT 'Обрезка-Тип-' || g FROM generate_series(1, 501) g`)
	s.Require().NoError(err)

	st, raw := s.admin.Do(http.MethodGet, "/catalog/equipment-types?q="+url.QueryEscape("Обрезка-Тип"), nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), "Типов: 501 — показаны первые 500, уточните поиск")

	st, raw = s.admin.Do(http.MethodGet, "/catalog/equipment-types?q="+url.QueryEscape("Обрезка-Тип-501"), nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), "Типов: 1</p>")
}
