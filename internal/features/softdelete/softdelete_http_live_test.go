//go:build integration

package softdelete_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SoftDeleteLiveSuite (§TD-8: боевой бинарь, реальный логин) — раздел «Удалённые»
// (docs/features/soft-delete-core/draft-spec.md §6.3, §6.4, §8.1): TS-009 (список: вид,
// фильтры по «И», конверт, сортировка), TS-010 (период в поясе пользователя), TS-011
// (счётчики вкладок), TS-033 (права на адресах раздела), SSR-оболочка экрана /deleted и пункт
// меню. Заказы создаются и удаляются через API доски того же сервера.
type SoftDeleteLiveSuite struct {
	testutil.LiveSuite
	admin   *testutil.Browser
	adminID string
}

func TestSoftDeleteLiveSuite(t *testing.T) {
	suite.Run(t, &SoftDeleteLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *SoftDeleteLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.adminID = s.SeedUser("sd-admin", "password1", "Удалённые Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("sd-admin", "password1")
}

// ---- wire-типы (1:1 с softdelete.Item / ListResponse / SummaryResponse, §TD-6.2) ----

type itemDTO struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Subtitle  string    `json:"subtitle"`
	DeletedAt time.Time `json:"deleted_at"`
	DeletedBy struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"deleted_by"`
	DeleteReason string  `json:"delete_reason"`
	DeletedWith  *string `json:"deleted_with"`
	IsArchived   bool    `json:"is_archived"`
}

type listDTO struct {
	Items  []itemDTO `json:"items"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

type summaryDTO struct {
	Counts map[string]int `json:"counts"`
}

type errDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *struct {
		Fields []struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"details"`
}

const api = "/api/v1/deleted"

func (s *SoftDeleteLiveSuite) errOf(raw []byte) errDTO {
	var e errDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *SoftDeleteLiveSuite) fields(raw []byte) map[string]string {
	e := s.errOf(raw)
	s.Require().NotNil(e.Details, string(raw))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

// list — GET /api/v1/deleted от имени b; query — пары ключ, значение.
func (s *SoftDeleteLiveSuite) list(b *testutil.Browser, kv ...string) (int, listDTO, []byte) {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		q.Set(kv[i], kv[i+1])
	}
	st, raw := b.Do(http.MethodGet, api+"?"+q.Encode(), nil)
	var out listDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		s.Require().NotNil(out.Items, "items — массив, не null: %s", raw)
	}
	return st, out, raw
}

func (s *SoftDeleteLiveSuite) mustList(kv ...string) listDTO {
	st, out, raw := s.list(s.admin, kv...)
	s.Require().Equal(http.StatusOK, st, string(raw))
	return out
}

func ids(l listDTO) []string {
	out := make([]string, len(l.Items))
	for i, it := range l.Items {
		out[i] = it.ID
	}
	return out
}

// ---- фикстуры ----

func (s *SoftDeleteLiveSuite) newClient(company string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO clients (company_name) VALUES ($1) RETURNING id`, company).Scan(&id))
	return id
}

func (s *SoftDeleteLiveSuite) columnID() string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id FROM columns WHERE NOT is_final AND deleted_at IS NULL ORDER BY position, id LIMIT 1`).Scan(&id))
	return id
}

func (s *SoftDeleteLiveSuite) newTask(b *testutil.Browser, clientID, title string) int64 {
	st, raw := b.Do(http.MethodPost, "/api/v1/tasks", map[string]any{"title": title, "client_id": clientID, "column_id": s.columnID()})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Task struct {
			ID int64 `json:"id"`
		} `json:"task"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	return m.Task.ID
}

// deleted — заказ, мягко удалённый b с причиной reason; at != "" — момент удаления
// (фикстура времени, SQL: не предмет теста). Возвращает номер заказа строкой.
func (s *SoftDeleteLiveSuite) deleted(b *testutil.Browser, clientID, title, reason, at string) string {
	id := s.newTask(b, clientID, title)
	st, raw := b.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", id), map[string]any{"reason": reason})
	s.Require().Equal(http.StatusOK, st, string(raw))
	if at != "" {
		_, err := s.Pool.Exec(context.Background(), `UPDATE tasks SET deleted_at = $2::timestamptz WHERE id = $1`, id, at)
		s.Require().NoError(err)
	}
	return fmt.Sprint(id)
}

func (s *SoftDeleteLiveSuite) user(login, name, role string) (*testutil.Browser, string) {
	id := s.SeedUser(login, "password1", name, role, true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b, id
}

func (s *SoftDeleteLiveSuite) setTimezone(b *testutil.Browser, tz string) {
	st, raw := b.Do(http.MethodPut, "/api/v1/users/me/timezone", map[string]any{"timezone": tz})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// ---- TS-009 ----

// TS-009 (DS-011, DS-028): kind обязателен и из реестра; параметры валидируются (400 с
// details.fields[]). Негатив: kind=orders → invalid, limit=201 → invalid.
func (s *SoftDeleteLiveSuite) TestTS009_Validation() {
	for _, tc := range []struct {
		name string
		kv   []string
		want map[string]string
	}{
		{"без kind", nil, map[string]string{"kind": "required"}},
		{"kind=orders", []string{"kind", "orders"}, map[string]string{"kind": "invalid"}},
		{"q длиннее 100", []string{"kind", "tasks", "q", strings.Repeat("я", 101)}, map[string]string{"q": "too_long"}},
		{"не дата", []string{"kind", "tasks", "deleted_from", "2026-02-30", "deleted_to", "завтра"},
			map[string]string{"deleted_from": "invalid", "deleted_to": "invalid"}},
		{"deleted_by не uuid", []string{"kind", "tasks", "deleted_by", "admin"}, map[string]string{"deleted_by": "invalid"}},
		{"limit=201", []string{"kind", "tasks", "limit", "201"}, map[string]string{"limit": "invalid"}},
		{"limit=0", []string{"kind", "tasks", "limit", "0"}, map[string]string{"limit": "invalid"}},
		{"offset=-1", []string{"kind", "tasks", "offset", "-1"}, map[string]string{"offset": "invalid"}},
		{"offset не число", []string{"kind", "tasks", "offset", "x"}, map[string]string{"offset": "invalid"}},
	} {
		st, _, raw := s.list(s.admin, tc.kv...)
		s.Equal(http.StatusBadRequest, st, "%s: %s", tc.name, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code, tc.name)
		s.Equal(tc.want, s.fields(raw), tc.name)
	}
}

// TS-009: строка списка и конверт {items, total, limit, offset}; сортировка — deleted_at по
// убыванию, при равенстве — id; пагинация; фильтры q (название, причина, номер — без учёта
// регистра), deleted_by, период — сочетаются по «И». Виды, чья часть не смержена, — пустой
// список без ошибки.
func (s *SoftDeleteLiveSuite) TestTS009_ListFiltersEnvelopeOrder() {
	mark := "sd9" + uuid.NewString()[:8]
	c := s.newClient("Клиент " + mark)
	second, secondID := s.user("sd-admin-2-"+mark, "Второй Админ "+mark, "admin")

	// порядок по времени удаления: c (новее) → a и b (один момент — по id) → d (старее).
	a := s.deleted(s.admin, c, "Ремонт КОФЕмашины "+mark, "Создан по ошибке "+mark, "2026-10-05T10:00:00Z")
	b := s.deleted(s.admin, c, "Диагностика "+mark, "Дубль заказа "+mark, "2026-10-05T10:00:00Z")
	cNew := s.deleted(second, c, "Чистка "+mark, "Клиент отказался "+mark, "2026-10-07T10:00:00Z")
	d := s.deleted(s.admin, c, "Замена фильтра "+mark, "Создан по ошибке повторно "+mark, "2026-10-01T10:00:00Z")
	live := s.newTask(s.admin, c, "Действующий "+mark) // не удалён — в разделе его нет

	all := s.mustList("kind", "tasks", "q", mark)
	s.Equal([]string{cNew, a, b, d}, ids(all), "deleted_at по убыванию, затем id")
	s.Equal(4, all.Total)
	s.Equal(50, all.Limit, "limit по умолчанию")
	s.Equal(0, all.Offset)
	s.NotContains(ids(all), fmt.Sprint(live))

	it := all.Items[1]
	s.Equal("tasks", it.Kind)
	s.Equal(a, it.ID)
	s.Equal("#"+a+" Ремонт КОФЕмашины "+mark, it.Title)
	s.Equal("Клиент "+mark, it.Subtitle)
	s.True(it.DeletedAt.Equal(time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)), "deleted_at: %s", it.DeletedAt)
	s.Equal(s.adminID, it.DeletedBy.ID)
	s.Equal("Удалённые Админ", it.DeletedBy.Name)
	s.Equal("Создан по ошибке "+mark, it.DeleteReason)
	s.Nil(it.DeletedWith)
	s.False(it.IsArchived)
	s.Equal(secondID, all.Items[0].DeletedBy.ID)

	// пагинация: total не зависит от страницы.
	page := s.mustList("kind", "tasks", "q", mark, "limit", "2", "offset", "1")
	s.Equal([]string{a, b}, ids(page))
	s.Equal(4, page.Total)
	s.Equal(2, page.Limit)
	s.Equal(1, page.Offset)
	beyond := s.mustList("kind", "tasks", "q", mark, "limit", "2", "offset", "10")
	s.Empty(beyond.Items)
	s.Equal(4, beyond.Total)

	// поиск: название и причина без учёта регистра, номер заказа («N» и «#N»).
	s.Equal([]string{a}, ids(s.mustList("kind", "tasks", "q", "кофемашины "+mark)), "по названию")
	s.Equal([]string{a, d}, ids(s.mustList("kind", "tasks", "q", "создан ПО ошибке")), "по причине")
	s.Equal([]string{b}, ids(s.mustList("kind", "tasks", "q", "#"+b)), "по номеру с решёткой")
	s.Contains(ids(s.mustList("kind", "tasks", "q", b)), b, "по номеру")
	s.Empty(s.mustList("kind", "tasks", "q", "100%_"+mark).Items, "спецсимволы LIKE ищутся буквально")

	// «Кто удалил»; несуществующий сотрудник — пустой список.
	s.Equal([]string{cNew}, ids(s.mustList("kind", "tasks", "q", mark, "deleted_by", secondID)))
	s.Empty(s.mustList("kind", "tasks", "q", mark, "deleted_by", uuid.NewString()).Items)

	// период (пояс администратора — по умолчанию Asia/Almaty, +05:00): границы включительно.
	s.Equal([]string{a, b}, ids(s.mustList("kind", "tasks", "q", mark, "deleted_from", "2026-10-05", "deleted_to", "2026-10-05")))
	s.Equal([]string{cNew, a, b}, ids(s.mustList("kind", "tasks", "q", mark, "deleted_from", "2026-10-05")))
	s.Equal([]string{a, b, d}, ids(s.mustList("kind", "tasks", "q", mark, "deleted_to", "2026-10-05")))

	// сочетание по «И».
	s.Equal([]string{d}, ids(s.mustList("kind", "tasks", "q", "создан по ошибке", "deleted_by", s.adminID, "deleted_to", "2026-10-04")))
	s.Empty(s.mustList("kind", "tasks", "q", "создан по ошибке", "deleted_by", secondID).Items)

	// восстановленный заказ из раздела уходит.
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+d+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal([]string{cNew, a, b}, ids(s.mustList("kind", "tasks", "q", mark)))

	// виды, чья часть мастер-спеки не смержена, — пустой список без ошибки.
	for _, k := range softdelete.Kinds {
		if k == softdelete.KindTasks || k == softdelete.KindColumns { // части 1, 3 смержены
			continue
		}
		l := s.mustList("kind", string(k))
		s.Empty(l.Items, string(k))
		s.Equal(0, l.Total, string(k))
	}
}

// ---- TS-009: вид columns (часть 3, soft-delete-board-columns §6.3, §8.2) ----

// newColumn — колонка доски, созданная b через API доски того же сервера.
func (s *SoftDeleteLiveSuite) newColumn(b *testutil.Browser, name string, isFinal bool) string {
	st, raw := b.Do(http.MethodPost, "/api/v1/columns", map[string]any{
		"name": name, "color": "#445566", "is_final": isFinal, "consumes_stock": false,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Column struct {
			ID string `json:"id"`
		} `json:"column"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	s.Require().NotEmpty(m.Column.ID, string(raw))
	return m.Column.ID
}

// TS-009 (DS-011, soft-delete-board-columns §6.3): вид columns отдаёт удалённые колонки —
// title — название, subtitle «финальная» либо пустая строка, is_archived = false, deleted_with =
// null; сортировка — deleted_at по убыванию; поиск по названию и причине без учёта регистра,
// фильтр «кто удалил»; counts.columns = total списка. Восстановленная колонка из списка
// уходит. Негатив: действующая колонка в список не входит; исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS009_ColumnsKind() {
	mark := "sdcol" + uuid.NewString()[:8]
	other, otherID := s.user("sd-col-"+mark, "Второй Удаляющий", "admin")
	del := func(b *testutil.Browser, id, reason string) {
		st, raw := b.Do(http.MethodDelete, "/api/v1/columns/"+id, map[string]any{"reason": reason})
		s.Require().Equal(http.StatusOK, st, string(raw))
	}

	plain := s.newColumn(s.admin, "Обычная "+mark, false)
	final := s.newColumn(s.admin, "Итог "+mark, true)
	live := s.newColumn(s.admin, "Действующая "+mark, false) // в список не входит
	// уборка фикстур (SQL: сьюта без служебного адреса purge) — колонки теста пустые.
	defer func() {
		_, err := s.Pool.Exec(context.Background(), `DELETE FROM columns WHERE id = ANY($1::uuid[])`, []string{plain, final, live})
		s.NoError(err)
	}()
	del(s.admin, plain, "Лишний этап "+mark)
	del(other, final, "Дубль "+mark)

	l := s.mustList("kind", "columns", "q", mark)
	s.Equal(2, l.Total)
	s.Equal([]string{final, plain}, ids(l), "новые удаления сверху")
	s.NotContains(ids(s.mustList("kind", "columns", "limit", "200")), live)
	top := l.Items[0]
	s.Equal("columns", top.Kind)
	s.Equal("Итог "+mark, top.Title)
	s.Equal("финальная", top.Subtitle)
	s.Equal("Дубль "+mark, top.DeleteReason)
	s.Equal(otherID, top.DeletedBy.ID)
	s.Equal("Второй Удаляющий", top.DeletedBy.Name)
	s.False(top.IsArchived)
	s.Nil(top.DeletedWith)
	s.WithinDuration(time.Now(), top.DeletedAt, time.Minute)
	s.Equal("Обычная "+mark, l.Items[1].Title)
	s.Equal("", l.Items[1].Subtitle)

	// фильтры сочетаются по «И»: название и причина без учёта регистра, кто удалил.
	s.Equal([]string{plain}, ids(s.mustList("kind", "columns", "q", strings.ToUpper("обычная "+mark))))
	s.Equal([]string{plain}, ids(s.mustList("kind", "columns", "q", strings.ToUpper("лишний этап "+mark))))
	s.Equal([]string{final}, ids(s.mustList("kind", "columns", "q", mark, "deleted_by", otherID)))
	s.Empty(ids(s.mustList("kind", "columns", "q", "Обычная "+mark, "deleted_by", otherID)))
	// пагинация в конверте.
	page := s.mustList("kind", "columns", "q", mark, "limit", "1", "offset", "1")
	s.Equal([]string{plain}, ids(page))
	s.Equal(2, page.Total)
	s.Equal(1, page.Limit)
	s.Equal(1, page.Offset)

	// счётчик вкладки = total списка; восстановление уменьшает оба.
	count := func() int {
		st, raw := s.admin.Do(http.MethodGet, api+"/summary", nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var out summaryDTO
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		return out.Counts["columns"]
	}
	before := count()
	s.Equal(s.mustList("kind", "columns", "limit", "1").Total, before)
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/columns/"+plain+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(before-1, count())
	s.Equal([]string{final}, ids(s.mustList("kind", "columns", "q", mark)))

	exec, _ := s.user("sd-col-exec-"+mark, "Исполнитель", "executor")
	st, _, raw = s.list(exec, "kind", "columns")
	s.Equal(http.StatusForbidden, st, string(raw))
}

// ---- TS-009: виды каталога (часть 5, soft-delete-catalog §6.9, §8.2) ----

// postID — POST path с телом body от имени администратора, 201 → id созданной записи.
func (s *SoftDeleteLiveSuite) postID(path string, body map[string]any) string {
	st, raw := s.admin.Do(http.MethodPost, path, body)
	s.Require().Equal(http.StatusCreated, st, "%s: %s", path, raw)
	var out struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	s.Require().NotEmpty(out.ID, string(raw))
	return out.ID
}

// TS-009 (DS-011, DS-009, soft-delete-catalog §6.9): виды products, services,
// catalog-categories отдают удалённые записи — title — название; subtitle — у товара артикул,
// у услуги название категории, у категории (только услуг) «услуги»;
// is_archived = true только у записи, удалённой из архива (TS-028 негатив); deleted_with = null;
// поиск по названию и причине, фильтр «кто удалил»; counts вкладки = total списка;
// восстановленная запись из списка уходит. Негатив: действующая запись в список не входит; у
// записи, удалённой активной, is_archived = false; исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS009_CatalogKinds() {
	ctx := context.Background()
	mark := "sdcat" + uuid.NewString()[:8]
	other, otherID := s.user("sd-cat-"+mark, "Второй Удаляющий", "admin")
	del := func(b *testutil.Browser, path, reason string) {
		st, raw := b.Do(http.MethodDelete, path, map[string]any{"reason": reason})
		s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
	}

	svcCat := s.postID("/api/v1/catalog-categories", map[string]any{"kind": "service", "name": "Работы " + mark})
	goneCat := s.postID("/api/v1/catalog-categories", map[string]any{"kind": "service", "name": "Лишняя " + mark})
	// товары без остатка (stock не передан → 0): удаление разрешено.
	archivedProduct := s.postID("/api/v1/products", map[string]any{"sku": "ARC-" + mark, "name": "Архивный товар " + mark, "price": 10})
	activeProduct := s.postID("/api/v1/products", map[string]any{"sku": "ACT-" + mark, "name": "Активный товар " + mark, "price": 10})
	liveProduct := s.postID("/api/v1/products", map[string]any{"sku": "LIVE-" + mark, "name": "Действующий товар " + mark, "price": 10})
	service := s.postID("/api/v1/services", map[string]any{"category_id": svcCat, "name": "Услуга " + mark, "price": 100})
	// уборка фикстур (SQL: сьюта без служебного адреса purge); счётчики видов каталога остаются
	// нулевыми для TS-011.
	defer func() {
		for _, q := range []string{
			`DELETE FROM products WHERE id = ANY($1::uuid[])`, `DELETE FROM services WHERE id = ANY($1::uuid[])`,
			`DELETE FROM catalog_categories WHERE id = ANY($1::uuid[])`,
		} {
			_, err := s.Pool.Exec(ctx, q, []string{svcCat, goneCat, archivedProduct, activeProduct, liveProduct, service})
			s.NoError(err)
		}
	}()

	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+archivedProduct, map[string]any{
		"sku": "ARC-" + mark, "name": "Архивный товар " + mark, "price": 10, "is_active": false})
	s.Require().Equal(http.StatusOK, st, string(raw))

	del(s.admin, "/api/v1/products/"+archivedProduct, "Снят "+mark)
	del(other, "/api/v1/products/"+activeProduct, "Дубль "+mark)
	del(s.admin, "/api/v1/services/"+service, "Не оказываем "+mark)
	del(s.admin, "/api/v1/catalog-categories/"+goneCat, "Лишняя категория "+mark)

	// ---- товары: новые удаления сверху, артикул в подзаголовке, признак архива ----
	l := s.mustList("kind", "products", "q", mark)
	s.Equal(2, l.Total)
	s.Equal([]string{activeProduct, archivedProduct}, ids(l), "новые удаления сверху")
	s.NotContains(ids(s.mustList("kind", "products", "limit", "200")), liveProduct, "действующий товар в список не входит")
	top, second := l.Items[0], l.Items[1]
	s.Equal("products", top.Kind)
	s.Equal("Активный товар "+mark, top.Title)
	s.Equal("ACT-"+mark, top.Subtitle, "подзаголовок товара — артикул")
	s.Equal("Дубль "+mark, top.DeleteReason)
	s.Equal(otherID, top.DeletedBy.ID)
	s.Equal("Второй Удаляющий", top.DeletedBy.Name)
	s.False(top.IsArchived, "удалён активным")
	s.Nil(top.DeletedWith)
	s.WithinDuration(time.Now(), top.DeletedAt, time.Minute)
	s.True(second.IsArchived, "удалён из архива (TS-028)")
	// фильтры по «И»: название и причина без учёта регистра, кто удалил.
	s.Equal([]string{archivedProduct}, ids(s.mustList("kind", "products", "q", strings.ToUpper("архивный товар "+mark))))
	s.Equal([]string{archivedProduct}, ids(s.mustList("kind", "products", "q", strings.ToUpper("снят "+mark))))
	s.Equal([]string{activeProduct}, ids(s.mustList("kind", "products", "q", mark, "deleted_by", otherID)))
	s.Empty(ids(s.mustList("kind", "products", "q", "Архивный товар "+mark, "deleted_by", otherID)))

	// ---- услуги, категории ----
	sv := s.mustList("kind", "services", "q", mark)
	s.Require().Equal([]string{service}, ids(sv))
	s.Equal("services", sv.Items[0].Kind)
	s.Equal("Услуга "+mark, sv.Items[0].Title)
	s.Equal("Работы "+mark, sv.Items[0].Subtitle)
	s.False(sv.Items[0].IsArchived)

	cats := s.mustList("kind", "catalog-categories", "q", mark)
	s.Require().Equal([]string{goneCat}, ids(cats), "действующие категории в список не входят")
	s.Equal("catalog-categories", cats.Items[0].Kind)
	s.Equal("Лишняя "+mark, cats.Items[0].Title)
	s.Equal("услуги", cats.Items[0].Subtitle)

	// ---- счётчики вкладок = total списка; восстановление уменьшает оба ----
	counts := func() map[string]int {
		st, raw := s.admin.Do(http.MethodGet, api+"/summary", nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var out summaryDTO
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		return out.Counts
	}
	before := counts()
	for kind, key := range map[string]string{"products": "products", "services": "services",
		"catalog-categories": "catalog_categories"} {
		s.Equal(s.mustList("kind", kind, "limit", "1").Total, before[key], "счётчик %s = total списка", key)
		s.Positive(before[key], key)
	}
	for path, key := range map[string]string{
		"/api/v1/products/" + activeProduct: "products", "/api/v1/services/" + service: "services",
		"/api/v1/catalog-categories/" + goneCat: "catalog_categories",
	} {
		st, raw := s.admin.Do(http.MethodPost, path+"/restore", nil)
		s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
		s.Equal(before[key]-1, counts()[key], key)
	}
	s.Equal([]string{archivedProduct}, ids(s.mustList("kind", "products", "q", mark)), "восстановленный товар из списка ушёл")
	s.Empty(ids(s.mustList("kind", "services", "q", mark)))
	s.Empty(ids(s.mustList("kind", "catalog-categories", "q", mark)))

	exec, _ := s.user("sd-cat-exec-"+mark, "Исполнитель", "executor")
	for _, kind := range []string{"products", "services", "catalog-categories"} {
		st, _, raw := s.list(exec, "kind", kind)
		s.Equal(http.StatusForbidden, st, "%s: %s", kind, raw)
		s.Equal("Нет доступа", s.errOf(raw).Message)
	}
}

// ---- TS-009: вид suppliers (часть 6, soft-delete-suppliers §6.5, §8.2) ----

// TS-009 (DS-011, DS-009, soft-delete-suppliers §6.5): вид suppliers отдаёт удалённые записи —
// title — название; subtitle — контактное лицо; is_archived = true только у записи, удалённой из архива; deleted_with = null;
// новые удаления сверху; поиск по названию и причине без учёта регистра, фильтр «кто удалил»;
// counts вкладки = total списка; восстановленная запись из списка уходит. Негатив: действующие
// (в том числе архивные) поставщики в список не входят; исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS009_SuppliersKind() {
	ctx := context.Background()
	mark := "sdwh" + uuid.NewString()[:8]
	other, otherID := s.user("sd-wh-"+mark, "Второй Удаляющий", "admin")
	del := func(b *testutil.Browser, path, reason string) {
		st, raw := b.Do(http.MethodDelete, path, map[string]any{"reason": reason})
		s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
	}
	archive := func(path string, body map[string]any) {
		st, raw := s.admin.Do(http.MethodPut, path, body)
		s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
	}

	supArchived := s.postID("/api/v1/suppliers", map[string]any{"name": "Архивный поставщик " + mark})
	supActive := s.postID("/api/v1/suppliers", map[string]any{"name": "Активный поставщик " + mark, "contact_name": "Иван Петров"})
	supLive := s.postID("/api/v1/suppliers", map[string]any{"name": "Действующий поставщик " + mark})
	// уборка фикстур (SQL: сьюта без служебного адреса purge); счётчики видов остаются нулевыми
	// для TS-011.
	defer func() {
		_, err := s.Pool.Exec(ctx, `DELETE FROM suppliers WHERE id = ANY($1::uuid[])`, []string{supArchived, supActive, supLive})
		s.NoError(err)
	}()
	archive("/api/v1/suppliers/"+supArchived, map[string]any{"name": "Архивный поставщик " + mark, "is_active": false})

	del(s.admin, "/api/v1/suppliers/"+supArchived, "Не работаем "+mark)
	del(other, "/api/v1/suppliers/"+supActive, "Ошибка "+mark)

	// ---- поставщики ----
	sp := s.mustList("kind", "suppliers", "q", mark)
	s.Require().Equal([]string{supActive, supArchived}, ids(sp), "новые удаления сверху")
	s.NotContains(ids(s.mustList("kind", "suppliers", "limit", "200")), supLive, "действующий поставщик в список не входит")
	s.Equal("suppliers", sp.Items[0].Kind)
	s.Equal("Активный поставщик "+mark, sp.Items[0].Title)
	s.Equal("Иван Петров", sp.Items[0].Subtitle, "подзаголовок — контактное лицо")
	s.Equal("Ошибка "+mark, sp.Items[0].DeleteReason)
	s.Equal(otherID, sp.Items[0].DeletedBy.ID)
	s.False(sp.Items[0].IsArchived)
	s.Nil(sp.Items[0].DeletedWith)
	s.True(sp.Items[1].IsArchived, "удалён из архива (DS-009)")
	s.Equal("", sp.Items[1].Subtitle, "контактного лица нет — пустая строка")
	s.Equal([]string{supArchived}, ids(s.mustList("kind", "suppliers", "q", strings.ToUpper("не работаем "+mark))))
	s.Equal([]string{supActive}, ids(s.mustList("kind", "suppliers", "q", mark, "deleted_by", otherID)))

	// ---- счётчики вкладок = total списка; восстановление уменьшает оба ----
	counts := func() map[string]int {
		st, raw := s.admin.Do(http.MethodGet, api+"/summary", nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		var out summaryDTO
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		return out.Counts
	}
	before := counts()
	s.Equal(s.mustList("kind", "suppliers", "limit", "1").Total, before["suppliers"], "счётчик = total списка")
	s.Equal(2, before["suppliers"])
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/suppliers/"+supActive+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(before["suppliers"]-1, counts()["suppliers"])
	s.Equal([]string{supArchived}, ids(s.mustList("kind", "suppliers", "q", mark)), "восстановленный поставщик из списка ушёл")

	exec, _ := s.user("sd-wh-exec-"+mark, "Исполнитель", "executor")
	st, _, raw = s.list(exec, "kind", "suppliers")
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("Нет доступа", s.errOf(raw).Message)
}

// ---- TS-010 ----

// TS-010 (DS-025): заказ удалён 2026-10-06T23:30:00Z — это 6 октября для пользователя с
// поясом UTC и 7 октября для пользователя с поясом Europe/Moscow (+03:00): каждый находит его
// в «свой» день и не находит в соседний. Негатив: deleted_to раньше deleted_from → 400.
func (s *SoftDeleteLiveSuite) TestTS010_PeriodInUserTimezone() {
	mark := "sd10" + uuid.NewString()[:8]
	c := s.newClient("Клиент " + mark)
	id := s.deleted(s.admin, c, "Заказ "+mark, "Причина "+mark, "2026-10-06T23:30:00Z")

	utc, _ := s.user("sd-tz-utc-"+mark, "Пояс UTC", "admin")
	s.setTimezone(utc, "UTC")
	moscow, _ := s.user("sd-tz-msk-"+mark, "Пояс Москва", "admin")
	s.setTimezone(moscow, "Europe/Moscow")

	found := func(b *testutil.Browser, day string) bool {
		st, l, raw := s.list(b, "kind", "tasks", "q", mark, "deleted_from", day, "deleted_to", day)
		s.Require().Equal(http.StatusOK, st, string(raw))
		return len(l.Items) == 1 && l.Items[0].ID == id
	}
	s.True(found(utc, "2026-10-06"), "UTC: этот день")
	s.False(found(utc, "2026-10-07"), "UTC: не следующий день")
	s.True(found(moscow, "2026-10-07"), "+03:00: следующий день")
	s.False(found(moscow, "2026-10-06"), "+03:00: не этот день")

	// время в ответе — UTC независимо от пояса пользователя.
	_, l, _ := s.list(moscow, "kind", "tasks", "q", mark)
	s.Require().Len(l.Items, 1)
	s.True(l.Items[0].DeletedAt.Equal(time.Date(2026, 10, 6, 23, 30, 0, 0, time.UTC)))

	st, _, raw := s.list(utc, "kind", "tasks", "deleted_from", "2026-10-07", "deleted_to", "2026-10-06")
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"deleted_to": "invalid"}, s.fields(raw))
}

// ---- TS-011 ----

// TS-011 (DS-011): счётчики вкладок — по всем 9 видам реестра (ключи snake_case), каждый
// совпадает с total своего списка; удаление и восстановление заказа меняют counts.tasks.
// Негатив: исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS011_Summary() {
	summary := func() summaryDTO {
		st, raw := s.admin.Do(http.MethodGet, api+"/summary?kind=tasks&whatever=1", nil) // лишние параметры игнорируются
		s.Require().Equal(http.StatusOK, st, string(raw))
		var out summaryDTO
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
		return out
	}
	mark := "sd11" + uuid.NewString()[:8]
	c := s.newClient("Клиент " + mark)
	before := summary().Counts["tasks"]
	id := s.deleted(s.admin, c, "Заказ "+mark, "Причина "+mark, "")
	s.deleted(s.admin, c, "Заказ 2 "+mark, "Причина "+mark, "")

	sum := summary()
	s.Len(sum.Counts, 8)
	s.Equal(before+2, sum.Counts["tasks"])
	for _, k := range softdelete.Kinds {
		n, ok := sum.Counts[k.CountKey()]
		s.True(ok, "нет счётчика %s", k.CountKey())
		s.Equal(s.mustList("kind", string(k), "limit", "1").Total, n, "счётчик %s = total списка", k)
	}
	for _, key := range []string{"clients", "equipment", "products", "services",
		"catalog_categories", "suppliers"} {
		s.Equal(0, sum.Counts[key], key)
	}

	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks/"+id+"/restore", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(before+1, summary().Counts["tasks"])
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): раздел «Удалённые» — только администратору: исполнитель →
// 403 «Нет доступа» на списке (при любых параметрах, включая некорректные) и счётчиках; без
// сессии → 401.
func (s *SoftDeleteLiveSuite) TestTS033_AdminOnly() {
	exec, _ := s.user("sd-exec-"+uuid.NewString()[:8], "Исполнитель", "executor")
	anon := s.NewBrowser()
	for _, path := range []string{api + "?kind=tasks", api, api + "?kind=orders&limit=999", api + "/summary"} {
		st, raw := exec.Do(http.MethodGet, path, nil)
		s.Equal(http.StatusForbidden, st, "%s: %s", path, raw)
		s.Equal("FORBIDDEN", s.errOf(raw).Code, path)
		s.Equal("Нет доступа", s.errOf(raw).Message, path)

		st, _ = anon.Do(http.MethodGet, path, nil)
		s.Equal(http.StatusUnauthorized, st, "без сессии: %s", path)
	}
}

// ---- SSR ----

// Экран /deleted (§8.1): администратору — 200, оболочка экрана и пункт меню «Удалённые»;
// исполнителю — страница отказа «Нет доступа» (403), пункта меню нет.
func (s *SoftDeleteLiveSuite) TestSSR_PageAndMenu() {
	st, raw := s.admin.Do(http.MethodGet, "/deleted?kind=tasks", nil)
	s.Require().Equal(http.StatusOK, st)
	page := string(raw)
	for _, want := range []string{
		`data-testid="deleted-root"`, `data-testid="deleted-tabs"`, `data-testid="deleted-filter-q"`,
		`data-testid="deleted-filter-from"`, `data-testid="deleted-filter-to"`, `data-testid="deleted-filter-by"`,
		`data-testid="deleted-filter-reset"`, `data-testid="deleted-warnings"`, `data-testid="deleted-table"`,
		`/assets/ssr-shared/js/deleted.js`, `href="/deleted"`, "Удалённые",
	} {
		s.Contains(page, want)
	}
	st, raw = s.admin.Do(http.MethodGet, "/assets/ssr-shared/js/deleted.js", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), "/api/v1/deleted")

	exec, _ := s.user("sd-exec-ssr-"+uuid.NewString()[:8], "Исполнитель", "executor")
	st, raw = exec.Do(http.MethodGet, "/deleted", nil)
	s.Equal(http.StatusForbidden, st)
	s.Contains(string(raw), "Нет доступа")
	s.NotContains(string(raw), `data-testid="deleted-root"`)

	st, raw = exec.Do(http.MethodGet, "/", nil)
	s.Require().Equal(http.StatusOK, st)
	s.NotContains(string(raw), `href="/deleted"`, "пункта меню у исполнителя нет")
}
