//go:build integration

package catalog_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление товаров, услуг и категорий каталога (docs/features/soft-delete-catalog/
// draft-spec.md §11): TS-027, TS-028, TS-029, TS-033, пометки category_is_deleted /
// product_is_deleted и выбор без удалённых (DS-014), SSR-формы удаления (DS-032), служебная
// уборка (DS-027) и гонки §3. Сьюта — CatalogLiveSuite (§TD-8: боевой бинарь, реальный логин).

// ---- wire-типы (§TD-6.2) ----

type sdErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type sdDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type sdRestoreDTO struct {
	ID string `json:"id"`
}

type sdProductDTO struct {
	ID       string `json:"id"`
	IsActive bool   `json:"is_active"`
}

type sdServiceDTO struct {
	ID                string `json:"id"`
	CategoryID        string `json:"category_id"`
	CategoryIsDeleted bool   `json:"category_is_deleted"`
	IsActive          bool   `json:"is_active"`
	Products          []struct {
		ProductID        string `json:"product_id"`
		ProductIsDeleted bool   `json:"product_is_deleted"`
	} `json:"products"`
}

// ---- помощники ----

func sdMark() string { return uuid.NewString()[:8] }

func (s *CatalogLiveSuite) sdErr(raw []byte) sdErrDTO {
	var e sdErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *CatalogLiveSuite) sdScalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *CatalogLiveSuite) sdExec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *CatalogLiveSuite) sdAdminID() string {
	return s.sdScalar(`SELECT id::text FROM users WHERE login = 'catalog4-admin'`)
}

// sdState — «удалена|кто|причина|is_active» строки таблицы table; нет строки — "".
func (s *CatalogLiveSuite) sdState(table, id string) string {
	var v string
	err := s.Pool.QueryRow(context.Background(), `
		SELECT (r.deleted_at IS NOT NULL)::text || '|' || coalesce(u.login, '') || '|' || coalesce(r.delete_reason, '') || '|' || r.is_active::text
		FROM `+table+` r LEFT JOIN users u ON u.id = r.deleted_by WHERE r.id = $1::uuid`, id).Scan(&v)
	if err != nil {
		return ""
	}
	return v
}

const (
	sdLiveActive   = "false|||true"
	sdLiveArchived = "false|||false"
)

func (s *CatalogLiveSuite) sdDelete(b *testutil.Browser, path string, reason any) (int, sdDeleteDTO, []byte) {
	var body any
	if reason != nil {
		body = map[string]any{"reason": reason}
	}
	st, raw := b.Do(http.MethodDelete, path, body)
	var out sdDeleteDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	}
	return st, out, raw
}

func (s *CatalogLiveSuite) sdMustDelete(path, reason string) {
	st, _, raw := s.sdDelete(s.admin, path, reason)
	s.Require().Equal(http.StatusOK, st, "%s: %s", path, raw)
}

func (s *CatalogLiveSuite) sdRestore(b *testutil.Browser, path string) (int, []byte) {
	return b.Do(http.MethodPost, path+"/restore", nil)
}

// sdProduct — товар с остатком stock (0 — нулевой остаток); категорий у товаров нет.
func (s *CatalogLiveSuite) sdProduct(name string, stock float64) string {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/products", map[string]any{
		"name": name, "sku": "SD-" + sdMark(), "unit": "шт", "price": 100, "stock": stock,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var p sdProductDTO
	s.Require().NoError(json.Unmarshal(raw, &p))
	return p.ID
}

func (s *CatalogLiveSuite) sdGetProduct(id string) (int, sdProductDTO, []byte) {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/products/"+id, nil)
	var p sdProductDTO
	if st == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &p), string(raw))
	}
	return st, p, raw
}

func (s *CatalogLiveSuite) sdArchiveProduct(id, name string) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/products/"+id, map[string]any{
		"name": name, "price": 100, "is_active": false,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *CatalogLiveSuite) sdServiceReq(method, path, categoryID, name string, extra map[string]any) (int, sdServiceDTO, []byte) {
	var body any
	if method != http.MethodGet {
		m := map[string]any{"category_id": categoryID, "name": name, "price": 500}
		for k, v := range extra {
			m[k] = v
		}
		body = m
	}
	st, raw := s.admin.Do(method, path, body)
	var sv sdServiceDTO
	if st == http.StatusOK || st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &sv), string(raw))
	}
	return st, sv, raw
}

func (s *CatalogLiveSuite) sdService(categoryID, name string, productIDs ...string) string {
	kit := []map[string]any{}
	for _, id := range productIDs {
		kit = append(kit, map[string]any{"product_id": id, "qty": 1})
	}
	st, sv, raw := s.sdServiceReq(http.MethodPost, "/api/v1/services", categoryID, name, map[string]any{"products": kit})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return sv.ID
}

func (s *CatalogLiveSuite) sdExecutor(login string) *testutil.Browser {
	s.SeedUser(login, "password1", "Исполнитель "+login, "executor", true)
	b := s.NewBrowser()
	b.Login(login, "password1")
	return b
}

// sdSubtask — заказ с услугой serviceID и (если задан) товаром productID в ней: минимальные
// строки SQL (board-сервис в этой сьюте не предмет теста). Возвращает id услуги заказа.
func (s *CatalogLiveSuite) sdSubtask(serviceID, productID string) string {
	ctx := context.Background()
	var clientID, columnID, subtaskID string
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('SD-каталог клиент ' || $1) RETURNING id`, sdMark()).Scan(&clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns WHERE deleted_at IS NULL ORDER BY position LIMIT 1`).Scan(&columnID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		WITH car AS (INSERT INTO equipment (client_id, plate_number) VALUES ($2::uuid, 'КТ 001') RETURNING id)
		INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by)
		VALUES ($1::uuid, $2::uuid, (SELECT id FROM car), 'SD-каталог заказ', 1024, $3::uuid) RETURNING id`, columnID, clientID, s.sdAdminID()).Scan(&taskID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2::uuid) RETURNING id`, taskID, serviceID).Scan(&subtaskID))
	if productID != "" {
		s.sdExec(`INSERT INTO subtask_products (subtask_id, product_id, qty, price, position) VALUES ($1::uuid, $2::uuid, 2, 100, 1)`,
			subtaskID, productID)
	}
	return subtaskID
}

// ---- TS-027 ----

// TS-027 (DS-021, DS-014): товар с нулевым остатком удаляется (200, тело id + deleted_at,
// «кто, когда, почему» на строке, причина обрезана); ненулевой остаток — 409 PRODUCT_HAS_STOCK,
// строка не тронута. Данные подобраны против неверных реализаций: ОТРИЦАТЕЛЬНЫЙ остаток
// (проверка «> 0» прошла бы) и дробный остаток меньше единицы (проверка «>= 1» / округление до
// целого прошли бы). Ссылки из услуг каталога, заказов и журнала склада удалению не мешают. Удалённый товар для обычных адресов не существует (DS-026).
func (s *CatalogLiveSuite) TestTS027_ProductDelete_StockGuard() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-027 Работы "+m)

	positive := s.sdProduct("SD-027 Остаток "+m, 3)
	negative := s.sdProduct("SD-027 Долг "+m, -2)
	fraction := s.sdProduct("SD-027 Дробный "+m, 0.001)
	for name, id := range map[string]string{"положительный": positive, "отрицательный": negative, "дробный": fraction} {
		st, _, raw := s.sdDelete(s.admin, "/api/v1/products/"+id, "Создан по ошибке")
		s.Require().Equal(http.StatusConflict, st, "%s: %s", name, raw)
		e := s.sdErr(raw)
		s.Equal("PRODUCT_HAS_STOCK", e.Code, name)
		s.Equal("У товара есть остаток на складе — сначала спишите его", e.Message, name)
		s.Equal(sdLiveActive, s.sdState("products", id), "%s: товар не помечен", name)
	}

	// товар в комплекте услуги, в заказе и в журнале склада, остаток нулевой — удаляется.
	used := s.sdProduct("SD-027 В заказах "+m, 0)
	svc := s.sdService(svcCat.ID, "SD-027 Услуга "+m, used)
	subtaskID := s.sdSubtask(svc, used)
	s.sdExec(`INSERT INTO stock_movements (product_id, qty_delta, reason, user_id) VALUES ($1::uuid, 1, 'manual', $2::uuid)`,
		used, s.sdAdminID())
	st, del, raw := s.sdDelete(s.admin, "/api/v1/products/"+used, "  Создан по ошибке  ")
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(used, del.ID)
	s.WithinDuration(time.Now(), del.DeletedAt, time.Minute)
	s.Equal("true|catalog4-admin|Создан по ошибке|true", s.sdState("products", used), "причина обрезана, архив не тронут")
	s.Equal("1|1|1", s.sdScalar(`
		SELECT (SELECT count(*) FROM service_products WHERE product_id = $1::uuid)::text || '|' ||
		       (SELECT count(*) FROM subtask_products WHERE subtask_id = $2::uuid AND product_id = $1::uuid)::text || '|' ||
		       (SELECT count(*) FROM stock_movements WHERE product_id = $1::uuid)::text`, used, subtaskID),
		"строки комплекта, заказа и журнала склада остались")

	// DS-026: удалённого товара для обычных адресов нет; повтор удаления — 404.
	st, _, raw = s.sdGetProduct(used)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(raw).Code)
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+used, map[string]any{"name": "Правка", "price": 1, "is_active": true})
	s.Equal(http.StatusNotFound, st, string(raw))
	st, _, raw = s.sdDelete(s.admin, "/api/v1/products/"+used, "Повтор")
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(raw).Code)
	for _, q := range []string{"", "&include_archived=true"} {
		st, raw = s.admin.Do(http.MethodGet, "/api/v1/products?q="+m+"&limit=500"+q, nil)
		s.Require().Equal(http.StatusOK, st)
		s.NotContains(string(raw), used, "удалённого товара в списке нет (%q)", q)
		s.Contains(string(raw), positive)
	}

	// остаток списан до нуля — товар удаляется.
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/products/"+positive, map[string]any{
		"name": "SD-027 Остаток " + m, "price": 100, "is_active": true, "stock": 0,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.sdDelete(s.admin, "/api/v1/products/"+positive, "Списан")
	s.Equal(http.StatusOK, st, string(raw))
}

// ---- TS-028 ----

// TS-028 (DS-009): архив и удаление независимы — архивная запись удаляется и восстанавливается
// архивной, активная — активной; список справочника с фильтром «архив» удалённых не содержит.
// Восстановление: 200 {id}; повтор — 409 NOT_DELETED; записи нет — 404 своего вида. Проверяются
// товар, услуга и категория.
func (s *CatalogLiveSuite) TestTS028_ArchiveAndDeleteIndependent() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-028 Работы "+m)

	type rec struct{ table, path, list, notFound string }
	mk := func(archived bool) []rec {
		suffix := " активная " + m
		if archived {
			suffix = " архивная " + m
		}
		p := s.sdProduct("SD-028 Товар"+suffix, 0)
		sv := s.sdService(svcCat.ID, "SD-028 Услуга"+suffix)
		c := s.newCategory(s.admin, "service", "SD-028 Категория"+suffix)
		if archived {
			s.sdArchiveProduct(p, "SD-028 Товар"+suffix)
			st, _, raw := s.sdServiceReq(http.MethodPut, "/api/v1/services/"+sv, svcCat.ID, "SD-028 Услуга"+suffix, map[string]any{"is_active": false})
			s.Require().Equal(http.StatusOK, st, string(raw))
			st, raw = s.admin.Do(http.MethodPost, "/catalog/categories/"+c.ID+"/archive", url.Values{"kind": {"service"}})
			s.Require().Equal(http.StatusSeeOther, st, string(raw))
		}
		return []rec{
			{"products", "/api/v1/products/" + p, "/api/v1/products?include_archived=true&limit=2000", "PRODUCT_NOT_FOUND"},
			{"services", "/api/v1/services/" + sv, "/api/v1/services?include_archived=true&limit=2000", "SERVICE_NOT_FOUND"},
			{"catalog_categories", "/api/v1/catalog-categories/" + c.ID, "/api/v1/catalog-categories?include_archived=true&limit=2000", "CATEGORY_NOT_FOUND"},
		}
	}
	for _, archived := range []bool{true, false} {
		live := sdLiveActive
		if archived {
			live = sdLiveArchived
		}
		for _, r := range mk(archived) {
			id := r.path[strings.LastIndex(r.path, "/")+1:]
			s.Require().Equal(live, s.sdState(r.table, id), r.path)
			st, raw := s.admin.Do(http.MethodGet, r.list, nil)
			s.Require().Equal(http.StatusOK, st)
			s.Contains(string(raw), id, "до удаления запись в списке с архивом: %s", r.path)

			s.sdMustDelete(r.path, "Причина")
			s.Equal("true|catalog4-admin|Причина|"+live[len("false|||"):], s.sdState(r.table, id), "удаление архив не меняет: %s", r.path)
			st, raw = s.admin.Do(http.MethodGet, r.list, nil)
			s.Require().Equal(http.StatusOK, st)
			s.NotContains(string(raw), id, "список с фильтром «архив» удалённых не содержит: %s", r.path)

			st, raw = s.sdRestore(s.admin, r.path)
			s.Require().Equal(http.StatusOK, st, "%s: %s", r.path, raw)
			var out sdRestoreDTO
			s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
			s.Equal(id, out.ID)
			s.Equal(live, s.sdState(r.table, id), "восстановлена с прежним признаком архива, отметка стёрта: %s", r.path)

			st, raw = s.sdRestore(s.admin, r.path)
			s.Equal(http.StatusConflict, st, "%s: %s", r.path, raw)
			e := s.sdErr(raw)
			s.Equal("NOT_DELETED", e.Code, r.path)
			s.Equal("Запись уже восстановлена", e.Message, r.path)

			for _, missing := range []string{"00000000-0000-0000-0000-000000000000", "not-uuid"} {
				st, raw = s.sdRestore(s.admin, r.path[:strings.LastIndex(r.path, "/")+1]+missing)
				s.Equal(http.StatusNotFound, st, "%s %s: %s", r.table, missing, raw)
				s.Equal(r.notFound, s.sdErr(raw).Code)
			}
		}
	}
}

// ---- TS-029 ----

// TS-029 (DS-017): после удаления категории создаётся новая с тем же названием и видом;
// восстановление старой → 409 NAME_TAKEN, она остаётся удалённой; после переименования новой —
// восстановление 200 (категории только услуг — вид один). Негатив: две действующие
// категории одного вида с одним названием по-прежнему невозможны — и созданием, и
// переименованием (409 CATEGORY_NAME_TAKEN).
func (s *CatalogLiveSuite) TestTS029_CategoryNameFreeAfterDelete() {
	m := sdMark()
	name := "SD-029 Расходники " + m
	old := s.newCategory(s.admin, "service", name)
	s.sdMustDelete("/api/v1/catalog-categories/"+old.ID, "Дубль")

	fresh := s.newCategory(s.admin, "service", "  "+strings.ToUpper(name)+" ") // регистр и пробелы не различают
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/catalog-categories", map[string]any{"kind": "service", "name": name})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CATEGORY_NAME_TAKEN", s.sdErr(raw).Code, "две действующие с одним названием невозможны")
	spare := s.newCategory(s.admin, "service", "SD-029 Запасная "+m)
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+spare.ID, map[string]any{"name": name})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("CATEGORY_NAME_TAKEN", s.sdErr(raw).Code, "переименование в занятое название")

	st, raw = s.sdRestore(s.admin, "/api/v1/catalog-categories/"+old.ID)
	s.Require().Equal(http.StatusConflict, st, string(raw))
	e := s.sdErr(raw)
	s.Equal("NAME_TAKEN", e.Code)
	s.Equal("Название занято — переименуйте действующую запись", e.Message)
	s.Equal("true|catalog4-admin|Дубль|true", s.sdState("catalog_categories", old.ID), "категория осталась удалённой")

	st, raw = s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+fresh.ID, map[string]any{"name": "SD-029 Переименована " + m})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.sdRestore(s.admin, "/api/v1/catalog-categories/"+old.ID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(sdLiveActive, s.sdState("catalog_categories", old.ID))

	// удалённую категорию нельзя переименовать, отправить в архив, прочитать (DS-026).
	s.sdMustDelete("/api/v1/catalog-categories/"+spare.ID, "Лишняя")
	st, raw = s.admin.Do(http.MethodPut, "/api/v1/catalog-categories/"+spare.ID, map[string]any{"name": "SD-029 Новое имя " + m})
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("CATEGORY_NOT_FOUND", s.sdErr(raw).Code)
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+spare.ID, nil)
	s.Equal(http.StatusNotFound, st)
}

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): шесть адресов удаления и восстановления каталога — только
// администратору: исполнитель → 403 FORBIDDEN «Нет доступа» ДО поиска записи и проверки причины
// (существующий, несуществующий и битый идентификатор, с причиной и без); запись не изменилась.
// Негатив: без сессии → 401.
func (s *CatalogLiveSuite) TestTS033_CatalogSoftDeleteAdminOnly() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-033 Работы "+m)
	live := map[string]string{
		"products":           s.sdProduct("SD-033 Товар "+m, 0),
		"services":           s.sdService(svcCat.ID, "SD-033 Услуга "+m),
		"catalog_categories": s.newCategory(s.admin, "service", "SD-033 Категория "+m).ID,
	}
	gone := map[string]string{
		"products":           s.sdProduct("SD-033 Удалённый товар "+m, 0),
		"services":           s.sdService(svcCat.ID, "SD-033 Удалённая услуга "+m),
		"catalog_categories": s.newCategory(s.admin, "service", "SD-033 Удалённая категория "+m).ID,
	}
	paths := map[string]string{"products": "/api/v1/products/", "services": "/api/v1/services/", "catalog_categories": "/api/v1/catalog-categories/"}
	for table, id := range gone {
		s.sdMustDelete(paths[table]+id, "Фикстура")
	}

	exec := s.sdExecutor("sd033-exec-" + m)
	anon := s.NewBrowser()
	for table, base := range paths {
		for _, id := range []string{live[table], gone[table], "00000000-0000-0000-0000-000000000000", "not-uuid"} {
			for _, body := range []any{nil, map[string]any{"reason": "Попытка"}} {
				st, raw := exec.Do(http.MethodDelete, base+id, body)
				s.Equal(http.StatusForbidden, st, "DELETE %s%s: %s", base, id, raw)
				e := s.sdErr(raw)
				s.Equal("FORBIDDEN", e.Code)
				s.Equal("Нет доступа", e.Message)
			}
			st, raw := exec.Do(http.MethodPost, base+id+"/restore", nil)
			s.Equal(http.StatusForbidden, st, "restore %s%s: %s", base, id, raw)
			s.Equal("Нет доступа", s.sdErr(raw).Message)
		}
		s.Equal(sdLiveActive, s.sdState(table, live[table]), "%s: запись не удалена", table)
		s.Equal("true|catalog4-admin|Фикстура|true", s.sdState(table, gone[table]), "%s: запись не восстановлена", table)

		st, _ := anon.Do(http.MethodDelete, base+live[table], map[string]any{"reason": "Аноним"})
		s.Equal(http.StatusUnauthorized, st, table)
		st, _ = anon.Do(http.MethodPost, base+gone[table]+"/restore", nil)
		s.Equal(http.StatusUnauthorized, st, table)
	}
}

// ---- причина удаления (DS-020, abuse case 5) ----

// Причина обязательна для товара, услуги и категории: нет тела, нет поля, пробелы → 400
// reason: required; 501 символ → too_long; NUL → invalid_text; запись не удалена. Границы: 500
// символов (символов, не байт) — допустимо.
func (s *CatalogLiveSuite) TestDeleteReasonValidation() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-reason Работы "+m)
	recs := map[string]string{
		"products":           "/api/v1/products/" + s.sdProduct("SD-reason Товар "+m, 0),
		"services":           "/api/v1/services/" + s.sdService(svcCat.ID, "SD-reason Услуга "+m),
		"catalog_categories": "/api/v1/catalog-categories/" + s.newCategory(s.admin, "service", "SD-reason Категория "+m).ID,
	}
	for table, path := range recs {
		id := path[strings.LastIndex(path, "/")+1:]
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
			st, raw := s.admin.Do(http.MethodDelete, path, tc.body)
			s.Equal(http.StatusBadRequest, st, "%s %s: %s", table, tc.name, raw)
			s.Equal(tc.code, s.fields(raw)["reason"], "%s %s", table, tc.name)
		}
		s.Equal(sdLiveActive, s.sdState(table, id), "%s: запись не удалена", table)

		long := strings.Repeat("я", 500)
		st, _, raw := s.sdDelete(s.admin, path, " "+long+" ")
		s.Require().Equal(http.StatusOK, st, "%s: %s", table, raw)
		s.Equal("true|catalog4-admin|"+long+"|true", s.sdState(table, id))
	}
}

// ---- пометки и выбор без удалённых (DS-014, §6.9, abuse case 2) ----

// Категория удалена — её услуги остаются действующими с category_is_deleted = true; у
// остальных признак false. Счётчик item_count категории — только действующие записи: в
// категории активная, архивная и удалённая услуга → 2 (счёт «только активные» дал бы 1, «все» —
// 3). Удалённая категория в новой услуге и при СМЕНЕ категории → 400 category_id: not_found;
// сохранение записи без смены уже удалённой категории проходит. (Категории есть только у услуг.)
func (s *CatalogLiveSuite) TestDeletedCategory_FlagCountAndSelection() {
	m := sdMark()
	gone := s.newCategory(s.admin, "service", "SD-cat Удаляемая "+m)
	live := s.newCategory(s.admin, "service", "SD-cat Действующая "+m)
	active := s.sdService(gone.ID, "SD-cat Активная "+m)
	archived := s.sdService(gone.ID, "SD-cat Архивная "+m)
	st, _, raw := s.sdServiceReq(http.MethodPut, "/api/v1/services/"+archived, gone.ID, "SD-cat Архивная "+m, map[string]any{"is_active": false})
	s.Require().Equal(http.StatusOK, st, string(raw))
	deleted := s.sdService(gone.ID, "SD-cat Удалённая "+m)
	s.sdMustDelete("/api/v1/services/"+deleted, "Фикстура")
	elsewhere := s.sdService(live.ID, "SD-cat В другой категории "+m)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories/"+gone.ID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var c categoryDTO
	s.Require().NoError(json.Unmarshal(raw, &c))
	s.Equal(2, c.ItemCount, "item_count — действующие услуги: активная и архивная")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/catalog-categories?kind=service&include_archived=true&limit=2000", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct {
		Items []categoryDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	found := false
	for _, it := range list.Items {
		if it.ID == gone.ID {
			found = true
			s.Equal(2, it.ItemCount, "item_count в списке")
		}
	}
	s.True(found, "категория есть в списке")

	get := func(id string) sdServiceDTO {
		st, sv, raw := s.sdServiceReq(http.MethodGet, "/api/v1/services/"+id, "", "", nil)
		s.Require().Equal(http.StatusOK, st, string(raw))
		return sv
	}
	s.False(get(active).CategoryIsDeleted, "до удаления категории признак false")
	s.sdMustDelete("/api/v1/catalog-categories/"+gone.ID, "Категория не нужна")

	// услуги остались действующими, с пометкой.
	sv := get(active)
	s.True(sv.CategoryIsDeleted)
	s.Equal(gone.ID, sv.CategoryID)
	s.False(get(elsewhere).CategoryIsDeleted, "услуга действующей категории — без пометки")
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/services?include_archived=true&limit=2000&q="+url.QueryEscape("SD-cat"), nil)
	s.Require().Equal(http.StatusOK, st)
	var slist struct {
		Items []sdServiceDTO `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &slist))
	flags := map[string]bool{}
	for _, it := range slist.Items {
		flags[it.ID] = it.CategoryIsDeleted
	}
	s.Equal(true, flags[active])
	s.Equal(true, flags[archived])
	s.Contains(flags, elsewhere)
	s.False(flags[elsewhere])

	// выбор удалённой категории: новая запись и смена категории → 400 not_found.
	st, _, raw = s.sdServiceReq(http.MethodPost, "/api/v1/services", gone.ID, "SD-cat Новая услуга "+m, nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["category_id"])
	st, _, raw = s.sdServiceReq(http.MethodPut, "/api/v1/services/"+elsewhere, gone.ID, "SD-cat В другой категории "+m, map[string]any{"is_active": true})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["category_id"])
	// несуществующая категория — как раньше invalid (не «удалена»).
	st, _, raw = s.sdServiceReq(http.MethodPost, "/api/v1/services", "00000000-0000-0000-0000-000000000000", "SD-cat Мимо "+m, nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["category_id"])

	// сохранение без смены уже удалённой категории — разрешено.
	st, _, raw = s.sdServiceReq(http.MethodPut, "/api/v1/services/"+active, gone.ID, "SD-cat Активная правка "+m, map[string]any{"is_active": true})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"category_is_deleted":true`)
}

// Товар удалён из каталога — строка состава услуги остаётся с product_is_deleted = true (у
// действующего товара — false); услугу можно пересохранить с этой же строкой. В НОВОЙ строке
// состава удалённый товар → 400 products[i].product_id: not_found (индекс — исходной строки);
// архивный — по-прежнему invalid.
func (s *CatalogLiveSuite) TestDeletedProduct_ServiceKit() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-kit Работы "+m)
	keep := s.sdProduct("SD-kit Остаётся "+m, 0)
	inKit := s.sdProduct("SD-kit Удалят из каталога "+m, 0)
	other := s.sdProduct("SD-kit Удалён заранее "+m, 0)
	archived := s.sdProduct("SD-kit Архивный "+m, 0)
	s.sdArchiveProduct(archived, "SD-kit Архивный "+m)
	svc := s.sdService(svcCat.ID, "SD-kit Услуга "+m, keep, inKit)
	s.sdMustDelete("/api/v1/products/"+inKit, "Снят с продажи")
	s.sdMustDelete("/api/v1/products/"+other, "Снят с продажи")

	st, sv, raw := s.sdServiceReq(http.MethodGet, "/api/v1/services/"+svc, "", "", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Require().Len(sv.Products, 2, "строка удалённого товара осталась в составе")
	s.Equal(keep, sv.Products[0].ProductID)
	s.False(sv.Products[0].ProductIsDeleted)
	s.Equal(inKit, sv.Products[1].ProductID)
	s.True(sv.Products[1].ProductIsDeleted)

	kit := func(ids ...string) map[string]any {
		rows := []map[string]any{}
		for _, id := range ids {
			rows = append(rows, map[string]any{"product_id": id, "qty": 2})
		}
		return map[string]any{"products": rows}
	}
	put := func(ids ...string) map[string]any {
		body := kit(ids...)
		body["is_active"] = true // PUT — полная карточка
		return body
	}
	// пересохранение с уже стоящей строкой удалённого товара — проходит.
	st, sv, raw = s.sdServiceReq(http.MethodPut, "/api/v1/services/"+svc, svcCat.ID, "SD-kit Услуга "+m, put(keep, inKit))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.True(sv.Products[1].ProductIsDeleted)
	// новая строка с ДРУГИМ удалённым товаром — 400 по её индексу.
	st, _, raw = s.sdServiceReq(http.MethodPut, "/api/v1/services/"+svc, svcCat.ID, "SD-kit Услуга "+m, put(keep, inKit, other))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"products[2].product_id": "not_found"}, s.fields(raw))
	// новая услуга с удалённым товаром — 400; с архивным — as-is invalid.
	st, _, raw = s.sdServiceReq(http.MethodPost, "/api/v1/services", svcCat.ID, "SD-kit Новая "+m, kit(inKit))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("not_found", s.fields(raw)["products[0].product_id"])
	st, _, raw = s.sdServiceReq(http.MethodPost, "/api/v1/services", svcCat.ID, "SD-kit Новая "+m, kit(keep, archived))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("invalid", s.fields(raw)["products[1].product_id"])

	// удаление услуги состав не трогает; удалённая услуга для обычных адресов не существует.
	s.sdMustDelete("/api/v1/services/"+svc, "Не оказываем")
	s.Equal("2", s.sdScalar(`SELECT count(*)::text FROM service_products WHERE service_id = $1::uuid`, svc))
	st, _, raw = s.sdServiceReq(http.MethodPut, "/api/v1/services/"+svc, svcCat.ID, "SD-kit Правка "+m, put(keep))
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("SERVICE_NOT_FOUND", s.sdErr(raw).Code)
}

// ---- служебная уборка (§6.9, DS-027, TS-F10) ----

// Виды products, services, catalog-categories адреса DELETE /api/v1/test/purge/:kind/:id:
// действующая и мягко удалённая запись стираются физически; запись, на которую есть ссылки, →
// 409 с прежним кодом своего вида (PRODUCT_IN_USE, SERVICE_IN_USE, CATEGORY_IN_USE — коды
// остались только здесь) и остаётся; нет записи либо битый идентификатор → 404 NOT_FOUND.
// Журнал склада товара ссылкой не считается: уходит вместе с товаром, а при отказе остаётся.
// Негатив: исполнитель → 403.
func (s *CatalogLiveSuite) TestPurge_CatalogKinds() {
	m := sdMark()
	purge := func(b *testutil.Browser, kind, id string) (int, []byte) {
		return b.Do(http.MethodDelete, "/api/v1/test/purge/"+kind+"/"+id, nil)
	}
	journal := func(productID string) string {
		return s.sdScalar(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1::uuid`, productID)
	}
	addJournal := func(productID string) {
		s.sdExec(`INSERT INTO stock_movements (product_id, qty_delta, reason, user_id) VALUES ($1::uuid, 1, 'manual', $2::uuid)`,
			productID, s.sdAdminID())
	}
	svcCat := s.newCategory(s.admin, "service", "SD-purge Работы "+m)
	heldProduct := s.sdProduct("SD-purge В комплекте "+m, 0)
	heldService := s.sdService(svcCat.ID, "SD-purge В заказе "+m, heldProduct)
	s.sdSubtask(heldService, "")
	addJournal(heldProduct)
	s.sdMustDelete("/api/v1/products/"+heldProduct, "Фикстура") // мягкое удаление ссылок не снимает

	exec := s.sdExecutor("sd-purge-exec-" + m)
	st, raw := purge(exec, "products", heldProduct)
	s.Equal(http.StatusForbidden, st, string(raw))

	for _, tc := range []struct{ kind, table, id, code string }{
		{"products", "products", heldProduct, "PRODUCT_IN_USE"},
		{"services", "services", heldService, "SERVICE_IN_USE"},
		{"catalog-categories", "catalog_categories", svcCat.ID, "CATEGORY_IN_USE"},
	} {
		st, raw = purge(s.admin, tc.kind, tc.id)
		s.Equal(http.StatusConflict, st, "%s: %s", tc.kind, raw)
		s.Equal(tc.code, s.sdErr(raw).Code, tc.kind)
		s.NotEmpty(s.sdState(tc.table, tc.id), "%s: запись на месте", tc.kind)
	}
	s.Equal("1", journal(heldProduct), "отказ уборки журнал товара не трогает")

	freeCat := s.newCategory(s.admin, "service", "SD-purge Свободная "+m)
	goneCat := s.newCategory(s.admin, "service", "SD-purge Удалённая "+m)
	s.sdMustDelete("/api/v1/catalog-categories/"+goneCat.ID, "Фикстура")
	freeProduct := s.sdProduct("SD-purge Свободный "+m, 4) // остаток уборке не мешает
	goneProduct := s.sdProduct("SD-purge Удалённый "+m, 0)
	addJournal(freeProduct) // журнал склада уборке не мешает — уходит вместе с товаром
	addJournal(goneProduct)
	s.sdMustDelete("/api/v1/products/"+goneProduct, "Фикстура")
	freeService := s.sdService(svcCat.ID, "SD-purge Свободная услуга "+m, freeProduct) // состав уходит каскадом
	goneService := s.sdService(svcCat.ID, "SD-purge Удалённая услуга "+m)
	s.sdMustDelete("/api/v1/services/"+goneService, "Фикстура")
	for _, tc := range []struct{ kind, table, id string }{
		{"services", "services", freeService}, {"services", "services", goneService},
		{"products", "products", freeProduct}, {"products", "products", goneProduct},
		{"catalog-categories", "catalog_categories", freeCat.ID}, {"catalog-categories", "catalog_categories", goneCat.ID},
	} {
		st, raw = purge(s.admin, tc.kind, tc.id)
		s.Require().Equal(http.StatusOK, st, "%s %s: %s", tc.kind, tc.id, raw)
		s.Empty(s.sdState(tc.table, tc.id), "%s: строка физически удалена", tc.kind)
		st, raw = purge(s.admin, tc.kind, tc.id)
		s.Equal(http.StatusNotFound, st, "%s повтор: %s", tc.kind, raw)
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
	}
	s.Equal("0|0", journal(freeProduct)+"|"+journal(goneProduct), "журнал убранных товаров стёрт")
	for _, kind := range []string{"products", "services", "catalog-categories"} {
		st, raw = purge(s.admin, kind, "not-uuid")
		s.Equal(http.StatusNotFound, st, "%s: %s", kind, raw)
		s.Equal("NOT_FOUND", s.sdErr(raw).Code)
	}
}

// ---- SSR-формы удаления (§6.9, §8.1, DS-032) ----

func (s *CatalogLiveSuite) postForm(path string, form url.Values) (int, string, string) {
	form.Set("_csrf", s.admin.CSRFToken())
	resp := s.admin.Request(http.MethodPost, path, form, nil)
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Require().NoError(err)
	return resp.StatusCode, resp.Header.Get("Location"), string(body)
}

func (s *CatalogLiveSuite) page(path string) (int, string) {
	st, raw := s.admin.Do(http.MethodGet, path, nil)
	return st, string(raw)
}

// SSR POST /catalog/products/:id/delete и /catalog/services/:id/delete: поле reason обязательно
// — без причины форма возвращается (422) с окном «Удаление» и текстом под полем, введённая
// причина сохранена, запись не удалена; товар с остатком — 409, форма с красным блоком; успех —
// PRG к списку и тост. Удалённая запись — страница 404 на форме, сохранении, архиве, возврате из
// архива и повторном удалении (DS-026: «Вернуть из архива» не восстанавливает удалённое).
func (s *CatalogLiveSuite) TestSSR_ProductAndServiceDeleteForms() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-ssr Работы "+m)
	withStock := s.sdProduct("SD-ssr С остатком "+m, 2)
	product := s.sdProduct("SD-ssr Товар "+m, 0)
	service := s.sdService(svcCat.ID, "SD-ssr Услуга "+m, product)

	st, page := s.page("/catalog/products/" + product)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(page, `data-testid="product-delete-form" data-delete-dialog`)
	s.Contains(page, "Запись можно восстановить в разделе «Удалённые». Чтобы только скрыть её из выбора, используйте «В архив»")
	s.Contains(page, `data-testid="product-archive"`, "«В архив» — отдельное действие рядом")
	s.NotContains(page, "data-delete-error")

	long := strings.Repeat("я", 501)
	for _, base := range []string{"/catalog/products/" + product, "/catalog/services/" + service} {
		status, _, body := s.postForm(base+"/delete", url.Values{})
		s.Equal(http.StatusUnprocessableEntity, status, base)
		s.Contains(body, `data-delete-error="Укажите причину удаления"`, base)
		status, _, body = s.postForm(base+"/delete", url.Values{"reason": {long}})
		s.Equal(http.StatusUnprocessableEntity, status, base)
		s.Contains(body, `data-delete-error="Причина — не длиннее 500 символов"`, base)
		s.Contains(body, `name="reason" value="`+long+`"`, "введённая причина сохранена: %s", base)
	}
	s.Equal(sdLiveActive, s.sdState("products", product))
	s.Equal(sdLiveActive, s.sdState("services", service))

	// 409: товар с остатком — форма товара с красным блоком.
	status, _, body := s.postForm("/catalog/products/"+withStock+"/delete", url.Values{"reason": {"Не нужен"}})
	s.Equal(http.StatusConflict, status)
	s.Contains(body, `data-testid="form-error"`)
	s.Contains(body, "У товара есть остаток на складе — сначала спишите его")
	s.Contains(body, `data-testid="product-stock"`, "форма товара перерисована целиком")
	s.NotContains(body, "data-delete-error")
	s.Equal(sdLiveActive, s.sdState("products", withStock))

	// успех: список с тостом, записи в списке нет.
	for _, tc := range []struct{ table, id, base, list, flash, name string }{
		{"products", product, "/catalog/products/", "/catalog/products", "deleted", "SD-ssr Товар " + m},
		{"services", service, "/catalog/services/", "/catalog/services", "service_deleted", "SD-ssr Услуга " + m},
	} {
		status, loc, _ := s.postForm(tc.base+tc.id+"/delete", url.Values{"reason": {"Создан по ошибке"}})
		s.Equal(http.StatusSeeOther, status, tc.table)
		s.Equal(tc.list+"?flash="+tc.flash, loc)
		st, page = s.page(loc)
		s.Require().Equal(http.StatusOK, st)
		s.Contains(page, `data-kt-toast="Удалено. Восстановить можно в разделе «Удалённые»"`, tc.table)
		s.NotContains(page, `data-testid="flash"`, "удаление — тост, не зелёный блок")
		s.NotContains(page, tc.name, "%s: удалённой записи в списке нет", tc.table)
		st, page = s.page(tc.list + "?archived=1")
		s.Require().Equal(http.StatusOK, st)
		s.NotContains(page, tc.name, "%s: и в списке с архивом нет", tc.table)
		s.Equal("true|catalog4-admin|Создан по ошибке|true", s.sdState(tc.table, tc.id))

		// запись удалил другой администратор, пока форма открыта, — страница 404.
		st, _ = s.page(tc.base + tc.id)
		s.Equal(http.StatusNotFound, st, "форма удалённой записи: %s", tc.table)
		for _, action := range []string{"", "/archive", "/restore", "/delete"} {
			status, _, _ = s.postForm(tc.base+tc.id+action, url.Values{"reason": {"Повтор"}, "name": {"Правка"}, "category_id": {svcCat.ID}})
			s.Equal(http.StatusNotFound, status, "POST %s%s", tc.base, action)
		}
		s.Equal("true|catalog4-admin|Создан по ошибке|true", s.sdState(tc.table, tc.id), "«Вернуть из архива» удалённое не восстанавливает")
	}
}

// SSR POST /catalog/categories/:id/delete: без причины — тот же экран списка услуг (422) с окном
// «Удаление» у строки этой категории (и только у неё); успех — список без категории и тост; её
// услуга остаётся в списке с бейджем «категория удалена». Форма услуги удалённой категории
// показывает её текущим значением с бейджем «удалена»; в форме новой услуги удалённой категории
// нет. Удалённая категория: переименование, архив, возврат из архива, удаление — страница 404.
// (Категории есть только у услуг.)
func (s *CatalogLiveSuite) TestSSR_CategoryDeleteForm() {
	m := sdMark()
	gone := s.newCategory(s.admin, "service", "SD-ssr Удаляемая "+m)
	stay := s.newCategory(s.admin, "service", "SD-ssr Остаётся "+m)
	service := s.sdService(gone.ID, "SD-ssr Услуга категории "+m)
	s.sdService(stay.ID, "SD-ssr Услуга соседней категории "+m)
	// список услуг этого теста (сьюта общая — фильтр по метке).
	mine := "/catalog/services?q=" + url.QueryEscape(m)

	st, page := s.page(mine)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(page, `data-testid="cat-delete-form" data-delete-dialog`)
	s.NotContains(page, "data-delete-error")
	s.Equal(2, strings.Count(page, `data-testid="service-row"`))
	s.NotContains(page, `data-testid="category-deleted-badge"`)

	status, _, body := s.postForm("/catalog/categories/"+gone.ID+"/delete", url.Values{"kind": {"service"}, "reason": {"  "}})
	s.Equal(http.StatusUnprocessableEntity, status)
	s.Contains(body, `data-testid="category-panel"`, "экран списка услуг")
	s.Equal(1, strings.Count(body, `data-delete-error="Укажите причину удаления"`), "окно — только у строки этой категории")
	s.Equal(sdLiveActive, s.sdState("catalog_categories", gone.ID))

	status, loc, _ := s.postForm("/catalog/categories/"+gone.ID+"/delete", url.Values{"kind": {"service"}, "reason": {"Не нужна"}})
	s.Equal(http.StatusSeeOther, status)
	s.Equal("/catalog/services?flash=category_deleted", loc)
	st, page = s.page(loc)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(page, `data-kt-toast="Удалено. Восстановить можно в разделе «Удалённые»"`)
	s.NotContains(page, `data-cat-id="`+gone.ID+`"`, "удалённой категории в панели нет")
	s.Contains(page, `data-cat-id="`+stay.ID+`"`)
	st, page = s.page(mine)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(page, "SD-ssr Услуга категории "+m, "услуга удалённой категории осталась в списке")
	s.Equal(2, strings.Count(page, `data-testid="service-row"`))
	s.Equal(1, strings.Count(page, `data-testid="category-deleted-badge"`), "бейдж — только у услуги удалённой категории")

	// форма услуги: удалённая категория — текущим значением с бейджем.
	st, page = s.page("/catalog/services/" + service)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(page, `data-testid="category-deleted-badge">удалена</span>`)
	s.Contains(page, `<option value="`+gone.ID+`" selected>SD-ssr Удаляемая `+m+` (удалена)</option>`)
	// форма новой услуги: удалённой категории нет даже по прямой ссылке с её идентификатором.
	st, page = s.page("/catalog/services/new?category=" + gone.ID)
	s.Require().Equal(http.StatusOK, st)
	s.NotContains(page, gone.ID)
	s.NotContains(page, `data-testid="category-deleted-badge"`)
	s.Contains(page, `<option value="`+stay.ID+`"`)

	for _, action := range []string{"/rename", "/archive", "/restore", "/delete"} {
		status, _, _ = s.postForm("/catalog/categories/"+gone.ID+action, url.Values{"kind": {"service"}, "name": {"Новое имя " + m}, "reason": {"Повтор"}})
		s.Equal(http.StatusNotFound, status, "POST %s", action)
	}
	s.Equal("true|catalog4-admin|Не нужна|true", s.sdState("catalog_categories", gone.ID))
}

// ---- гонки (§3 «Конкурентность») ----

// lockWaiters — число серверных соединений, ждущих блокировку.
func (s *CatalogLiveSuite) lockWaiters() int {
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
func (s *CatalogLiveSuite) race(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) raceResult {
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

// holdProductDelete — «удаление товара в полёте»: те же блокировки и UPDATE, что делает
// ProductRepo.SoftDelete (строка товара — с остатком — FOR NO KEY UPDATE → отметка),
// транзакция не зафиксирована. Возвращает фиксацию.
func (s *CatalogLiveSuite) holdProductDelete(productID string) func(ctx context.Context) func() {
	return func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`, productID)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE products SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'Гонка' WHERE id = $1::uuid`, productID, s.sdAdminID())
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}
}

// §3 п. 1 (DS-021, abuse case 3): остаток товара меняется в незавершённой транзакции (приход:
// строка товара под FOR NO KEY UPDATE, products.stock 0 → 5) — в это окно приходит удаление
// товара. Удаление обязано дождаться блокировки строки товара и увидеть остаток: 409
// PRODUCT_HAS_STOCK, товар не помечен. Реализация, читающая остаток без блокировки, увидела бы
// ноль и удалила товар с остатком.
func (s *CatalogLiveSuite) TestRace_ProductDeleteWaitsForStockChange() {
	m := sdMark()
	p := s.sdProduct("SD-race Товар-приход "+m, 0)

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `SELECT 1 FROM products WHERE id = $1::uuid FOR NO KEY UPDATE`, p)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE products SET stock = 5 WHERE id = $1::uuid`, p)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) {
		st, _, raw := s.sdDelete(s.admin, "/api/v1/products/"+p, "Гонка")
		return st, raw
	})
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("PRODUCT_HAS_STOCK", s.sdErr(res.raw).Code)
	s.Equal(sdLiveActive, s.sdState("products", p), "товар не удалён")
}

// §3 п. 1: удаление товара идёт (строка товара заблокирована, отметка поставлена,
// транзакция не зафиксирована) — в это окно приходит правка остатка через карточку товара (PUT).
// Правка обязана дождаться удаления и получить 404 PRODUCT_NOT_FOUND; остаток не изменился,
// движения нет. Реализация без условия «не удалён» в UPDATE карточки записала бы остаток 7
// удалённому товару.
func (s *CatalogLiveSuite) TestRace_ProductStockEditWaitsForDelete() {
	m := sdMark()
	p := s.sdProduct("SD-race Товар-правка "+m, 0)

	res := s.race(s.holdProductDelete(p), func() (int, []byte) {
		return s.admin.Do(http.MethodPut, "/api/v1/products/"+p, map[string]any{
			"name": "SD-race Товар-правка " + m, "price": 100, "is_active": true, "stock": 7,
		})
	})
	s.Require().Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("0.000|0", s.sdScalar(`
		SELECT (SELECT stock::text FROM products WHERE id = $1::uuid) || '|' ||
		       (SELECT count(*) FROM stock_movements WHERE product_id = $1::uuid)::text`, p), "остаток и журнал не тронуты")
	s.Equal("true|catalog4-admin|Гонка|true", s.sdState("products", p))
}

// §3 п. 5 (DS-014): удаление товара идёт — в это окно приходит создание услуги каталога с этим
// товаром в составе. Услуга обязана дождаться удаления (строка товара FOR SHARE) и получить 400
// products[0].product_id: not_found; услуга не создана. Реализация, проверяющая товар без
// блокировки, создала бы услугу с новой ссылкой на удалённый товар.
func (s *CatalogLiveSuite) TestRace_ServiceKitWaitsForProductDelete() {
	m := sdMark()
	svcCat := s.newCategory(s.admin, "service", "SD-race Работы "+m)
	p := s.sdProduct("SD-race Товар-комплект "+m, 0)
	name := "SD-race Услуга-комплект " + m

	res := s.race(s.holdProductDelete(p), func() (int, []byte) {
		st, _, raw := s.sdServiceReq(http.MethodPost, "/api/v1/services", svcCat.ID, name,
			map[string]any{"products": []map[string]any{{"product_id": p, "qty": 1}}})
		return st, raw
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["products[0].product_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM services WHERE name = $1`, name), "услуга не создана")
}

// §3 п. 5 (DS-014): удаление категории идёт (UPDATE строки не зафиксирован) — в это окно
// приходит создание услуги в этой категории. Услуга обязана дождаться удаления (категория FOR
// SHARE) и получить 400 category_id: not_found; услуга не создана. (Категории есть только у услуг.)
func (s *CatalogLiveSuite) TestRace_ServiceCreateWaitsForCategoryDelete() {
	m := sdMark()
	cat := s.newCategory(s.admin, "service", "SD-race Категория-уходит "+m)
	name := "SD-race Услуга-опоздала " + m

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `UPDATE catalog_categories SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = 'Гонка' WHERE id = $1::uuid`,
			cat.ID, s.sdAdminID())
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) {
		return s.admin.Do(http.MethodPost, "/api/v1/services", map[string]any{"category_id": cat.ID, "name": name, "price": 1})
	})
	s.Require().Equal(http.StatusBadRequest, res.status, string(res.raw))
	s.Equal("not_found", s.fields(res.raw)["category_id"])
	s.Equal("0", s.sdScalar(`SELECT count(*)::text FROM services WHERE name = $1`, name), "услуга не создана")
}

// §3 п. 4 (DS-017): категория с названием удалённой создаётся в незавершённой транзакции — в это
// окно приходит восстановление удалённой. Исход определяет частичный уникальный индекс:
// восстановление ждёт и после фиксации получает 409 NAME_TAKEN, категория остаётся удалённой —
// ровно один из двух запросов успешен. Реализация с проверкой названия отдельным SELECT увидела
// бы свободное название и получила бы 500 либо два действующих дубля.
func (s *CatalogLiveSuite) TestRace_CategoryRestoreWaitsForCreateSameName() {
	m := sdMark()
	name := "SD-race Название " + m
	old := s.newCategory(s.admin, "service", name)
	s.sdMustDelete("/api/v1/catalog-categories/"+old.ID, "Гонка")

	res := s.race(func(ctx context.Context) func() {
		tx, err := s.Pool.Begin(ctx)
		s.Require().NoError(err)
		_, err = tx.Exec(ctx, `INSERT INTO catalog_categories (kind, name) VALUES ('service', $1)`, name)
		s.Require().NoError(err)
		return func() { s.Require().NoError(tx.Commit(ctx)) }
	}, func() (int, []byte) { return s.sdRestore(s.admin, "/api/v1/catalog-categories/"+old.ID) })
	s.Require().Equal(http.StatusConflict, res.status, string(res.raw))
	s.Equal("NAME_TAKEN", s.sdErr(res.raw).Code)
	s.Equal("true|catalog4-admin|Гонка|true", s.sdState("catalog_categories", old.ID), "категория осталась удалённой")
	s.Equal("1", s.sdScalar(`SELECT count(*)::text FROM catalog_categories WHERE kind = 'service' AND lower(btrim(name)) = lower($1) AND deleted_at IS NULL`, name))
}

// DS-024: два одновременных удаления одного товара — второе ждёт первое (строка товара под FOR
// NO KEY UPDATE) и получает 404 PRODUCT_NOT_FOUND; отметка первого не перезаписана.
func (s *CatalogLiveSuite) TestRace_SecondProductDeleteGets404() {
	m := sdMark()
	p := s.sdProduct("SD-race Товар-двойное "+m, 0)

	res := s.race(s.holdProductDelete(p), func() (int, []byte) {
		st, _, raw := s.sdDelete(s.admin, "/api/v1/products/"+p, "Второй запрос")
		return st, raw
	})
	s.Require().Equal(http.StatusNotFound, res.status, string(res.raw))
	s.Equal("PRODUCT_NOT_FOUND", s.sdErr(res.raw).Code)
	s.Equal("true|catalog4-admin|Гонка|true", s.sdState("products", p), "причина первого удаления не перезаписана")
}
