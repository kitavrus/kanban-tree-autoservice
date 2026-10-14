//go:build integration

package users_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Часовой пояс пользователя (docs/features/user-timezone/draft-spec.md §6, TS-021/TS-022).
// HTTP — только через живой бинарь и реальный вход (§TD-8).

const (
	tzAlmaty = "Asia/Almaty"
	tzMoscow = "Europe/Moscow"
)

type tzBody struct {
	Timezone string `json:"timezone"`
}

func (s *UsersLiveSuite) setMyTimezone(b *testutil.Browser, body any) (int, []byte) {
	return b.Do(http.MethodPut, "/api/v1/users/me/timezone", body)
}

// meTimezone — пояс из GET /api/v1/auth/me (читается из БД на каждый запрос).
func (s *UsersLiveSuite) meTimezone(b *testutil.Browser) string {
	st, raw := b.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var me tzBody
	s.Require().NoError(json.Unmarshal(raw, &me), string(raw))
	return me.Timezone
}

// userTimezone — пояс сотрудника глазами администратора (GET /api/v1/users/:id).
func (s *UsersLiveSuite) userTimezone(id string) string {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/users/"+id, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var u tzBody
	s.Require().NoError(json.Unmarshal(raw, &u), string(raw))
	return u.Timezone
}

func (s *UsersLiveSuite) userVersion(id string) (version int64) {
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT version FROM users WHERE id = $1::uuid`, id).Scan(&version))
	return version
}

func (s *UsersLiveSuite) userUpdatedAt(id string) (t time.Time) {
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT updated_at FROM users WHERE id = $1::uuid`, id).Scan(&t))
	return t
}

func (s *UsersLiveSuite) userCreatedAt(id string) (t time.Time) {
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT created_at FROM users WHERE id = $1::uuid`, id).Scan(&t))
	return t
}

// TS-021: свой пояс — PUT /api/v1/users/me/timezone доступен исполнителю; /auth/me и
// SSR-layout отдают новое значение сразу следующим запросом (DS-031, DS-033).
func (s *UsersLiveSuite) TestTS021_OwnTimezone_ExecutorChangesOwn() {
	u := s.newExecutor("tz-own")
	exec := s.NewBrowser()

	// Ответ входа и /auth/me несут пояс; умолчание — Алматы.
	st, raw := exec.Do(http.MethodPost, "/api/v1/auth/login", map[string]string{"login": "tz-own", "password": "password1"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	var login tzBody
	s.Require().NoError(json.Unmarshal(raw, &login))
	s.Equal(tzAlmaty, login.Timezone)
	s.Equal(tzAlmaty, s.meTimezone(exec))

	st, page := exec.Do(http.MethodGet, "/", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `<body data-timezone="Asia/Almaty">`)
	s.Contains(string(page), `<option value="Asia/Almaty" selected>Алматы (UTC&#43;5)</option>`)
	s.Contains(string(page), `<option value="Europe/Moscow">Москва (UTC&#43;3)</option>`)

	version := s.userVersion(u.ID)
	updatedAt, createdAt := s.userUpdatedAt(u.ID), s.userCreatedAt(u.ID)
	st, raw = s.setMyTimezone(exec, map[string]any{"timezone": tzMoscow})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.JSONEq(`{"timezone":"Europe/Moscow"}`, string(raw))

	s.Equal(tzMoscow, s.meTimezone(exec), "новое значение — сразу следующим запросом, без перелогина")
	s.Equal(tzMoscow, s.userTimezone(u.ID))
	s.Equal(version, s.userVersion(u.ID), "last-write-wins: version не увеличивается")
	// D82: смена своего пояса обновляет updated_at пользователя (created_at не трогается).
	s.True(s.userUpdatedAt(u.ID).After(updatedAt), "updated_at стал позже: было %s, стало %s", updatedAt, s.userUpdatedAt(u.ID))
	s.Equal(createdAt, s.userCreatedAt(u.ID), "created_at не изменился")

	st, page = exec.Do(http.MethodGet, "/", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `<body data-timezone="Europe/Moscow">`)
	s.Contains(string(page), `<option value="Europe/Moscow" selected>Москва (UTC&#43;3)</option>`)

	// Повтор безопасен; пояс переживает повторный вход (хранится в БД, не в сессии).
	st, _ = s.setMyTimezone(exec, map[string]any{"timezone": tzMoscow})
	s.Equal(http.StatusOK, st)
	again := s.NewBrowser()
	again.Login("tz-own", "password1")
	s.Equal(tzMoscow, s.meTimezone(again))

	// Пояс вне перечня select'а валиден (любое имя IANA) и показывается в select'е.
	st, raw = s.setMyTimezone(exec, map[string]any{"timezone": "Asia/Kolkata"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, page = exec.Do(http.MethodGet, "/", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `<option value="Asia/Kolkata" selected>Asia/Kolkata (UTC&#43;5:30)</option>`)
}

// TS-021: администратор — тоже «любой авторизованный»: свой пояс меняет тем же endpoint'ом.
func (s *UsersLiveSuite) TestTS021_OwnTimezone_AdminChangesOwn() {
	s.SeedUser("tz-admin", "password1", "Админ Пояс", "admin", true)
	adm := s.NewBrowser()
	adm.Login("tz-admin", "password1")
	st, raw := s.setMyTimezone(adm, map[string]any{"timezone": tzMoscow})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(tzMoscow, s.meTimezone(adm))
}

// TS-021: невалидный пояс → 400 VALIDATION details.fields[] (timezone: invalid / required),
// запись не изменена (abuse cases §3: обход каталога, 10 000 символов, Mars/Olympus).
func (s *UsersLiveSuite) TestTS021_OwnTimezone_Validation() {
	u := s.newExecutor("tz-bad")
	exec := s.NewBrowser()
	exec.Login("tz-bad", "password1")
	st, _ := s.setMyTimezone(exec, map[string]any{"timezone": tzMoscow})
	s.Require().Equal(http.StatusOK, st)

	for name, c := range map[string]struct {
		body any
		want string
	}{
		"неизвестный":          {map[string]any{"timezone": "Mars/Olympus"}, "invalid"},
		"обход каталога":       {map[string]any{"timezone": "../../etc/passwd"}, "invalid"},
		"абсолютный путь":      {map[string]any{"timezone": "/etc/passwd"}, "invalid"},
		"65 символов":          {map[string]any{"timezone": strings.Repeat("a", 65)}, "invalid"},
		"10 000 символов":      {map[string]any{"timezone": strings.Repeat("A", 10000)}, "invalid"},
		"пояс сервера":         {map[string]any{"timezone": "Local"}, "invalid"},
		"posix-копия zoneinfo": {map[string]any{"timezone": "posix/Asia/Almaty"}, "invalid"},
		"пробелы по краям":     {map[string]any{"timezone": " Europe/Moscow "}, "invalid"},
		"sql-инъекция":         {map[string]any{"timezone": "Asia/Almaty'; DROP TABLE users;--"}, "invalid"},
		"пустая строка":        {map[string]any{"timezone": ""}, "required"},
		"поле не передано":     {map[string]any{}, "required"},
		"null":                 {map[string]any{"timezone": nil}, "required"},
		"только чужой id":      {map[string]any{"user_id": u.ID}, "required"},
		"несуществующий город": {map[string]any{"timezone": "Europe/Atlantis"}, "invalid"},
	} {
		st, raw := s.setMyTimezone(exec, c.body)
		s.Equal(http.StatusBadRequest, st, "%s: %s", name, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code, name)
		s.Equal(map[string]string{"timezone": c.want}, s.fields(raw), name)
	}

	// Мусор вместо JSON / не тот тип → 400, не 500.
	for _, body := range []string{`{"timezone":`, `[]`, `{"timezone":123}`, `{"timezone":{"a":1}}`} {
		resp := exec.Request(http.MethodPut, "/api/v1/users/me/timezone", body,
			map[string]string{"Content-Type": "application/json"})
		_ = resp.Body.Close()
		s.Equal(http.StatusBadRequest, resp.StatusCode, body)
	}

	s.Equal(tzMoscow, s.meTimezone(exec), "после отказов запись не изменена")
	s.Equal(tzMoscow, s.userTimezone(u.ID))
}

// TS-021: чужой user_id в теле игнорируется — меняется пояс владельца сессии (abuse case 2).
func (s *UsersLiveSuite) TestTS021_OwnTimezone_ForeignUserIDIgnored() {
	victim := s.newExecutor("tz-victim")
	attacker := s.newExecutor("tz-attacker")
	b := s.NewBrowser()
	b.Login("tz-attacker", "password1")

	st, raw := s.setMyTimezone(b, map[string]any{"timezone": tzMoscow, "user_id": victim.ID, "id": victim.ID})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(tzMoscow, s.userTimezone(attacker.ID), "изменён пояс владельца сессии")
	s.Equal(tzAlmaty, s.userTimezone(victim.ID), "чужой пояс не тронут")

	// И через query — тоже нет.
	st, _ = b.Do(http.MethodPut, "/api/v1/users/me/timezone?user_id="+victim.ID, map[string]any{"timezone": "Asia/Tashkent"})
	s.Equal(http.StatusOK, st)
	s.Equal("Asia/Tashkent", s.userTimezone(attacker.ID))
	s.Equal(tzAlmaty, s.userTimezone(victim.ID))
}

// TS-021 (негатив): без сессии → 401; маршрут не перехватывается admin-группой и /:id.
func (s *UsersLiveSuite) TestTS021_OwnTimezone_Anonymous401_AndRouting() {
	anon := s.NewBrowser()
	st, raw := s.setMyTimezone(anon, map[string]any{"timezone": tzMoscow})
	s.Equal(http.StatusUnauthorized, st, string(raw))
	s.Equal("UNAUTHORIZED", s.errOf(raw).Code)

	// Соседние пути по-прежнему под admin-гардом: исполнителю 403, а не «свой профиль».
	s.newExecutor("tz-route")
	exec := s.NewBrowser()
	exec.Login("tz-route", "password1")
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/users/me"},
		{http.MethodPut, "/api/v1/users/me"},
		{http.MethodGet, "/api/v1/users/me/timezone"},
	} {
		var body any
		if c.method != http.MethodGet {
			body = map[string]any{"timezone": tzMoscow}
		}
		st, _ := exec.Do(c.method, c.path, body)
		s.Contains([]int{http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed}, st, "%s %s", c.method, c.path)
	}
	s.Equal(tzAlmaty, s.meTimezone(exec))
}

// TS-022: администратор задаёт пояс другому — POST/PUT/GET /api/v1/users (DS-056).
func (s *UsersLiveSuite) TestTS022_AdminSetsTimezone() {
	// POST без поля → Asia/Almaty; ответ содержит timezone.
	st, u, raw := s.create(s.admin, map[string]any{"login": "tz-adm-default", "name": "Пояс Умолчание", "role": "executor", "password": "password1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Contains(string(raw), `"timezone":"Asia/Almaty"`)

	// POST с полем.
	st, m, raw := s.create(s.admin, map[string]any{"login": "tz-adm-moscow", "name": "Пояс Москва", "role": "executor", "password": "password1", "timezone": tzMoscow})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Contains(string(raw), `"timezone":"Europe/Moscow"`)
	s.Equal(tzMoscow, s.userTimezone(m.ID))

	// POST с невалидным → 400 details.fields[], пользователь не создан.
	for _, bad := range []any{"Mars/Olympus", "", "../../etc/passwd", strings.Repeat("a", 65), "Local"} {
		st, _, raw = s.create(s.admin, map[string]any{"login": "tz-adm-bad", "name": "Плохой Пояс", "role": "executor", "password": "password1", "timezone": bad})
		s.Equal(http.StatusBadRequest, st, "%v: %s", bad, raw)
		s.Equal("VALIDATION", s.errOf(raw).Code)
		s.Equal(map[string]string{"timezone": "invalid"}, s.fields(raw), "%v", bad)
	}
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users?q=tz-adm-bad", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), `"total":0`)

	// PUT с полем → 200, значение в ответе.
	st, raw = s.put(s.admin, u.ID, card(u, map[string]any{"timezone": tzMoscow}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"timezone":"Europe/Moscow"`)
	s.Equal(tzMoscow, s.userTimezone(u.ID))

	// PUT без поля → пояс не меняется (остальные поля — меняются).
	st, raw = s.put(s.admin, u.ID, card(u, map[string]any{"name": "Пояс Переименован"}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"name":"Пояс Переименован"`)
	s.Contains(string(raw), `"timezone":"Europe/Moscow"`)

	// PUT с невалидным → 400, ни пояс, ни имя не изменены.
	st, raw = s.put(s.admin, u.ID, card(u, map[string]any{"name": "Не Должно Сохраниться", "timezone": "Mars/Olympus"}))
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"timezone": "invalid"}, s.fields(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users/"+u.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), `"name":"Пояс Переименован"`)
	s.Contains(string(raw), `"timezone":"Europe/Moscow"`)

	// GET /users (список) содержит timezone в каждом элементе.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users?q=tz-adm-&sort=login&order=asc", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list struct {
		Items []struct {
			Login    string `json:"login"`
			Timezone string `json:"timezone"`
		} `json:"items"`
		Total int `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(2, list.Total)
	got := map[string]string{}
	for _, it := range list.Items {
		got[it.Login] = it.Timezone
	}
	s.Equal(map[string]string{"tz-adm-default": tzMoscow, "tz-adm-moscow": tzMoscow}, got)

	// Сотрудник видит пояс, заданный администратором, без перелогина — и сам его меняет.
	b := s.NewBrowser()
	b.Login("tz-adm-default", "password1")
	s.Equal(tzMoscow, s.meTimezone(b))
	st, _ = s.setMyTimezone(b, map[string]any{"timezone": "Asia/Tashkent"})
	s.Require().Equal(http.StatusOK, st)

	// Устаревшая форма администратора (без поля timezone) не затирает пояс, который
	// сотрудник только что сменил сам (§1 «Логика» п.6).
	st, raw = s.put(s.admin, u.ID, card(u, map[string]any{"phone": "+7 900"}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"timezone":"Asia/Tashkent"`)
	// С полем — last-write-wins: сохраняется последняя запись.
	st, _ = s.put(s.admin, u.ID, card(u, map[string]any{"timezone": tzAlmaty}))
	s.Require().Equal(http.StatusOK, st)
	s.Equal(tzAlmaty, s.meTimezone(b))
}

// TS-022 (негатив): исполнитель чужой пояс через PUT /api/v1/users/:id не меняет → 403.
func (s *UsersLiveSuite) TestTS022_ExecutorCannotSetForeignTimezone() {
	victim := s.newExecutor("tz-403-victim")
	s.newExecutor("tz-403-exec")
	exec := s.NewBrowser()
	exec.Login("tz-403-exec", "password1")

	st, raw := s.put(exec, victim.ID, card(victim, map[string]any{"timezone": tzMoscow}))
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("FORBIDDEN", s.errOf(raw).Code)
	s.Equal(tzAlmaty, s.userTimezone(victim.ID))

	st, raw = exec.Do(http.MethodPost, "/api/v1/users", map[string]any{"login": "tz-403-new", "name": "x", "role": "executor", "password": "password1", "timezone": tzMoscow})
	s.Equal(http.StatusForbidden, st, string(raw))

	st, _ = exec.Do(http.MethodPost, "/users/"+victim.ID, url.Values{"name": {victim.Name}, "login": {victim.Login}, "role": {"executor"}, "is_active": {"on"}, "timezone": {tzMoscow}})
	s.Equal(http.StatusForbidden, st)
	s.Equal(tzAlmaty, s.userTimezone(victim.ID))
}

// TS-022: SSR-форма сотрудника — поле «Часовой пояс» (экран §8.1, DS-056).
func (s *UsersLiveSuite) TestTS022_SSRForm_Timezone() {
	// Форма создания: по умолчанию «Алматы (UTC+5)».
	st, page := s.admin.Do(http.MethodGet, "/users/new", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="user-timezone"`)
	s.Contains(string(page), `<option value="Asia/Almaty" selected>Алматы (UTC&#43;5)</option>`)

	form := func(tz string) url.Values {
		v := url.Values{"name": {"Форма Пояс"}, "login": {"tz-ssr"}, "role": {"executor"}, "phone": {""}, "password": {"password1"}, "is_active": {"on"}}
		if tz != "" {
			v.Set("timezone", tz)
		}
		return v
	}

	// Невалидный пояс подменой значения → 422, текст под полем, сотрудник не создан.
	st, page = s.admin.Do(http.MethodPost, "/users", form("Mars/Olympus"))
	s.Equal(http.StatusUnprocessableEntity, st)
	s.Contains(string(page), `data-testid="error-timezone">Неизвестный часовой пояс<`)
	s.Contains(string(page), `value="Форма Пояс"`, "введённое не теряется")

	resp := s.admin.Request(http.MethodPost, "/users", form(tzMoscow), nil)
	_ = resp.Body.Close()
	s.Require().Equal(http.StatusSeeOther, resp.StatusCode)

	var id, tz string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT id::text, timezone FROM users WHERE login = 'tz-ssr'`).Scan(&id, &tz))
	s.Equal(tzMoscow, tz)

	// Форма правки показывает сохранённый пояс.
	st, page = s.admin.Do(http.MethodGet, "/users/"+id, nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(page), `<option value="Europe/Moscow" selected>Москва (UTC&#43;3)</option>`)
	s.Contains(string(page), `<option value="Asia/Almaty">Алматы (UTC&#43;5)</option>`, "выбран пояс сотрудника, а не умолчание")

	// Правка: новый пояс сохраняется.
	edit := form("Asia/Tashkent")
	edit.Del("password")
	resp = s.admin.Request(http.MethodPost, "/users/"+id, edit, nil)
	_ = resp.Body.Close()
	s.Require().Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("Asia/Tashkent", s.userTimezone(id))

	// Правка невалидным → 422 под полем, пояс прежний.
	st, page = s.admin.Do(http.MethodPost, "/users/"+id, func() url.Values { v := form("../../etc/passwd"); v.Del("password"); return v }())
	s.Equal(http.StatusUnprocessableEntity, st)
	s.Contains(string(page), `data-testid="error-timezone">Неизвестный часовой пояс<`)
	s.Equal("Asia/Tashkent", s.userTimezone(id))

	// Форма без поля (устаревшая вкладка до релиза) — пояс не трогается.
	stale := form("")
	stale.Del("password")
	resp = s.admin.Request(http.MethodPost, "/users/"+id, stale, nil)
	_ = resp.Body.Close()
	s.Require().Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("Asia/Tashkent", s.userTimezone(id))
}
