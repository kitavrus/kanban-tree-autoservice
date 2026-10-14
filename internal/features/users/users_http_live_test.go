//go:build integration

package users_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// UsersLiveSuite (§TD-8): сотрудники — API и SSR-экран, права, D29, сессии.
type UsersLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestUsersLive(t *testing.T) {
	suite.Run(t, &UsersLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *UsersLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("boss", "password1", "Главный Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("boss", "password1")
}

type user struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Phone    string `json:"phone"`
	IsActive bool   `json:"is_active"`
}

type apiErr struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

func (s *UsersLiveSuite) errOf(body []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *UsersLiveSuite) fields(body []byte) map[string]string {
	e := s.errOf(body)
	s.Require().NotNil(e.Details, string(body))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

func (s *UsersLiveSuite) create(b *testutil.Browser, body map[string]any) (int, user, []byte) {
	st, raw := b.Do(http.MethodPost, "/api/v1/users", body)
	var u user
	if st == http.StatusCreated {
		s.Require().NoError(json.Unmarshal(raw, &u))
	}
	return st, u, raw
}

func (s *UsersLiveSuite) put(b *testutil.Browser, id string, body map[string]any) (int, []byte) {
	return b.Do(http.MethodPut, "/api/v1/users/"+id, body)
}

func card(u user, patch map[string]any) map[string]any {
	m := map[string]any{"login": u.Login, "name": u.Name, "role": u.Role, "phone": u.Phone, "is_active": u.IsActive}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *UsersLiveSuite) newExecutor(login string) user {
	st, u, raw := s.create(s.admin, map[string]any{"login": login, "name": "Исп " + login, "role": "executor", "password": "password1"})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return u
}

// ---- happy path ----

func (s *UsersLiveSuite) TestCreateGetListUpdate() {
	st, u, raw := s.create(s.admin, map[string]any{
		"login": "  Sidorov.P ", "name": " Сидоров Пётр ", "role": "executor", "phone": "+7 (900) 111-22-33", "password": "password1",
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("sidorov.p", u.Login, "логин нормализован")
	s.Equal("Сидоров Пётр", u.Name)
	s.True(u.IsActive, "по умолчанию активен")
	s.NotContains(string(raw), "password")

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users/"+u.ID, nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), `"phone":"+7 (900) 111-22-33"`)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users?q=сидоров&role=executor&is_active=true", nil)
	s.Equal(http.StatusOK, st)
	var list struct {
		Items  []user `json:"items"`
		Total  int    `json:"total"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(1, list.Total)
	s.Equal("sidorov.p", list.Items[0].Login)
	s.Equal(50, list.Limit)

	st, raw = s.put(s.admin, u.ID, card(u, map[string]any{"name": "Сидоров П.", "role": "admin", "phone": ""}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Contains(string(raw), `"role":"admin"`)
	s.Contains(string(raw), `"phone":""`)

	b := s.NewBrowser()
	b.Login("SIDOROV.P", "password1")
}

func (s *UsersLiveSuite) TestListEnvelopeAndPaging() {
	for i := 0; i < 3; i++ {
		s.newExecutor(fmt.Sprintf("page-%d", i))
	}
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/users?q=page-&limit=2&offset=1&sort=login&order=asc", nil)
	s.Equal(http.StatusOK, st)
	var list struct {
		Items []user `json:"items"`
		Total int    `json:"total"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list))
	s.Equal(3, list.Total)
	s.Require().Len(list.Items, 2)
	s.Equal("page-1", list.Items[0].Login)

	// мусор в параметрах списка — дефолты, не 500
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/users?limit=-5&offset=-1&sort=password_hash;drop&order=sideways&role=root&is_active=maybe", nil)
	s.Equal(http.StatusOK, st)

	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users?q=%25", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(raw), `"total":0`, "% ищется буквально")
}

// «Активных заказов» (как в прототипе): вне финальной колонки, сотрудник — ответственный
// заказа или исполнитель незавершённой услуги; заказ считается один раз.
func (s *UsersLiveSuite) TestList_ActiveTasks() {
	ctx := context.Background()
	u := s.newExecutor("active-tasks")
	idle := s.newExecutor("active-idle")

	var clientID, openCol, finalCol, serviceID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('Активные-Клиент') RETURNING id`).Scan(&clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns WHERE NOT is_final ORDER BY position LIMIT 1`).Scan(&openCol))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns WHERE is_final ORDER BY position LIMIT 1`).Scan(&finalCol))
	var categoryID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'Активные ' || gen_random_uuid()) RETURNING id`).Scan(&categoryID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO services (category_id, name) VALUES ($1, 'Активные-услуга') RETURNING id`, categoryID).Scan(&serviceID))

	task := func(column string, assignee *string) int64 {
		var id int64
		s.Require().NoError(s.Pool.QueryRow(ctx,
			`INSERT INTO tasks (column_id, client_id, title, assignee_id, position, created_by)
			 VALUES ($1, $2, 'Активные', $3, 1, $4) RETURNING id`, column, clientID, assignee, u.ID).Scan(&id))
		return id
	}
	subtask := func(taskID int64, status string) {
		_, err := s.Pool.Exec(ctx, `INSERT INTO subtasks (task_id, service_id, assignee_id, status) VALUES ($1, $2, $3, $4)`,
			taskID, serviceID, u.ID, status)
		s.Require().NoError(err)
	}
	task(openCol, &u.ID)                       // ответственный — да
	subtask(task(openCol, nil), "in_progress") // исполнитель незавершённой услуги — да
	subtask(task(openCol, nil), "done")        // услуга завершена — нет
	task(finalCol, &u.ID)                      // финальная колонка — нет
	both := task(openCol, &u.ID)               // и ответственный, и исполнитель — один раз
	subtask(both, "new")

	st, raw := s.admin.Do(http.MethodGet, "/api/v1/users?q=active-", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list struct {
		Items []struct {
			Login       string `json:"login"`
			ActiveTasks *int   `json:"active_tasks"`
		} `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	got := map[string]int{}
	for _, it := range list.Items {
		s.Require().NotNil(it.ActiveTasks, "active_tasks есть в списке: %s", it.Login)
		got[it.Login] = *it.ActiveTasks
	}
	s.Equal(map[string]int{u.Login: 3, idle.Login: 0}, got)

	// Карточка сотрудника — без счётчика (он только в списке).
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/users/"+u.ID, nil)
	s.Require().Equal(http.StatusOK, st)
	s.NotContains(string(raw), "active_tasks")

	// SSR-экран: колонка и значение в строке сотрудника.
	st, raw = s.admin.Do(http.MethodGet, "/users?q=active-tasks", nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), "Активных заказов")
	s.Contains(string(raw), `data-testid="user-active-tasks">3<`)
}

// ---- authz ----

func (s *UsersLiveSuite) TestExecutorAndAnonymousHaveNoAccess() {
	u := s.newExecutor("no-access")
	exec := s.NewBrowser()
	exec.Login("no-access", "password1")

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/users"},
		{http.MethodGet, "/api/v1/users/" + u.ID},
		{http.MethodPost, "/api/v1/users"},
		{http.MethodPut, "/api/v1/users/" + u.ID},
	} {
		st, raw := exec.Do(c.method, c.path, map[string]any{})
		if c.method == http.MethodGet {
			st, raw = exec.Do(c.method, c.path, nil)
		}
		s.Equal(http.StatusForbidden, st, "%s %s", c.method, c.path)
		s.Equal("FORBIDDEN", s.errOf(raw).Code)
	}
	st, page := exec.Do(http.MethodGet, "/users", nil)
	s.Equal(http.StatusForbidden, st)
	s.Contains(string(page), "Нет доступа")
	st, page = exec.Do(http.MethodGet, "/", nil)
	s.Equal(http.StatusOK, st)
	s.NotContains(string(page), `href="/users"`, "исполнитель не видит пункт «Сотрудники»")

	anon := s.NewBrowser()
	st, _ = anon.Do(http.MethodGet, "/api/v1/users", nil)
	s.Equal(http.StatusUnauthorized, st)
}

func (s *UsersLiveSuite) TestRoleChangeAppliesWithoutRelogin() {
	u := s.newExecutor("promote-me")
	b := s.NewBrowser()
	b.Login("promote-me", "password1")
	st, _ := b.Do(http.MethodGet, "/api/v1/users", nil)
	s.Equal(http.StatusForbidden, st)

	st, raw := s.put(s.admin, u.ID, card(u, map[string]any{"role": "admin"}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _ = b.Do(http.MethodGet, "/api/v1/users", nil)
	s.Equal(http.StatusOK, st, "та же сессия — уже admin")
}

// ---- validation / garbage → 400, not 500 ----

func (s *UsersLiveSuite) TestCreate_Validation() {
	st, _, raw := s.create(s.admin, map[string]any{})
	s.Equal(http.StatusBadRequest, st)
	s.Equal(map[string]string{"login": "required", "name": "required", "role": "invalid", "password": "required"}, s.fields(raw))

	st, _, raw = s.create(s.admin, map[string]any{
		"login": "ab", "name": strings.Repeat("я", 201), "role": "root", "phone": "abc", "password": "1234567",
	})
	s.Equal(http.StatusBadRequest, st)
	s.Equal(map[string]string{"login": "too_short", "name": "too_long", "role": "invalid", "phone": "invalid_format", "password": "too_short"}, s.fields(raw))

	// пароль длиннее 72 байт (предел bcrypt) — 400, не 500
	for _, pwd := range []string{strings.Repeat("a", 73), strings.Repeat("п", 37)} {
		st, _, raw = s.create(s.admin, map[string]any{"login": "long-pwd", "name": "x", "role": "executor", "password": pwd})
		s.Equal(http.StatusBadRequest, st, string(raw))
		s.Equal("too_long", s.fields(raw)["password"])
	}
	st, _, raw = s.create(s.admin, map[string]any{"login": "pwd-72", "name": "x", "role": "executor", "password": strings.Repeat("п", 36)})
	s.Equal(http.StatusCreated, st, "ровно 72 байта — можно: %s", raw)

	st, _, raw = s.create(s.admin, map[string]any{"login": "кириллица", "name": "x", "role": "executor", "password": "password1"})
	s.Equal(http.StatusBadRequest, st)
	s.Equal("invalid_format", s.fields(raw)["login"])

	for _, body := range []string{`{`, `null`, `"str"`, `{"login":"x","name":"x","role":"executor","password":"password1","password_hash":"h"}`, `{"is_active":"yes"}`} {
		st, raw := s.admin.Do(http.MethodPost, "/api/v1/users", body)
		s.Equal(http.StatusBadRequest, st, "%s → %s", body, raw)
	}
}

func (s *UsersLiveSuite) TestCreate_DuplicateLogin409() {
	s.newExecutor("dup-login")
	st, _, raw := s.create(s.admin, map[string]any{"login": "DUP-LOGIN", "name": "x", "role": "executor", "password": "password1"})
	s.Equal(http.StatusConflict, st)
	s.Equal("LOGIN_TAKEN", s.errOf(raw).Code)

	other := s.newExecutor("dup-other")
	st, raw = s.put(s.admin, other.ID, card(other, map[string]any{"login": "dup-login"}))
	s.Equal(http.StatusConflict, st)
	s.Equal("LOGIN_TAKEN", s.errOf(raw).Code)
}

func (s *UsersLiveSuite) TestUpdate_BadIDAndBody() {
	u := s.newExecutor("upd-bad")
	for _, id := range []string{"not-a-uuid", "00000000-0000-0000-0000-000000000000", "1'; DROP TABLE users;--"} {
		st, raw := s.put(s.admin, url.PathEscape(id), card(u, nil))
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
		st, _ = s.admin.Do(http.MethodGet, "/api/v1/users/"+url.PathEscape(id), nil)
		s.Equal(http.StatusNotFound, st)
	}
	body := card(u, nil)
	delete(body, "is_active")
	st, raw := s.put(s.admin, u.ID, body)
	s.Equal(http.StatusBadRequest, st)
	s.Equal("required", s.fields(raw)["is_active"])

	st, raw = s.put(s.admin, u.ID, card(u, map[string]any{"password": "short"}))
	s.Equal(http.StatusBadRequest, st)
	s.Equal("too_short", s.fields(raw)["password"])
}

func (s *UsersLiveSuite) TestNoDelete() {
	u := s.newExecutor("no-delete")
	st, _ := s.admin.Do(http.MethodDelete, "/api/v1/users/"+u.ID, nil)
	s.NotEqual(http.StatusOK, st, "удаления нет (D39)")
	st, _ = s.admin.Do(http.MethodGet, "/api/v1/users/"+u.ID, nil)
	s.Equal(http.StatusOK, st)
}

// ---- sessions ----

func (s *UsersLiveSuite) TestBlockKillsSessions_UnblockAllowsLogin() {
	u := s.newExecutor("to-block")
	phone, laptop := s.NewBrowser(), s.NewBrowser()
	phone.Login("to-block", "password1")
	laptop.Login("to-block", "password1")

	st, raw := s.put(s.admin, u.ID, card(u, map[string]any{"is_active": false}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	for _, b := range []*testutil.Browser{phone, laptop} {
		st, _ := b.Do(http.MethodGet, "/api/v1/auth/me", nil)
		s.Equal(http.StatusUnauthorized, st, "все сессии заблокированного убиты")
	}
	st, raw = s.NewBrowser().Do(http.MethodPost, "/api/v1/auth/login", map[string]string{"login": "to-block", "password": "password1"})
	s.Equal(http.StatusForbidden, st)
	s.Equal("USER_BLOCKED", s.errOf(raw).Code)

	st, _ = s.put(s.admin, u.ID, card(u, map[string]any{"is_active": true}))
	s.Require().Equal(http.StatusOK, st)
	s.NewBrowser().Login("to-block", "password1")
}

func (s *UsersLiveSuite) TestPasswordChange_KillsOtherSessions_KeepsOwnCurrent() {
	s.SeedUser("pwd-admin", "password1", "Пароль Админ", "admin", true)
	cur, other := s.NewBrowser(), s.NewBrowser()
	cur.Login("pwd-admin", "password1")
	other.Login("pwd-admin", "password1")

	st, raw := cur.Do(http.MethodGet, "/api/v1/users?q=pwd-admin", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct{ Items []user }
	s.Require().NoError(json.Unmarshal(raw, &list))
	me := list.Items[0]

	st, raw = s.put(cur, me.ID, card(me, map[string]any{"password": "new-password"}))
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _ = cur.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusOK, st, "текущая сессия того, кто сменил себе пароль, жива")
	st, _ = other.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusUnauthorized, st, "остальные сессии убиты")
	s.NewBrowser().Login("pwd-admin", "new-password")

	// админ меняет пароль исполнителю — убиваются все его сессии
	u := s.newExecutor("pwd-exec")
	eb := s.NewBrowser()
	eb.Login("pwd-exec", "password1")
	st, _ = s.put(s.admin, u.ID, card(u, map[string]any{"password": "other-pass"}))
	s.Require().Equal(http.StatusOK, st)
	st, _ = eb.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusUnauthorized, st)
}

// ---- D29 ----

func (s *UsersLiveSuite) TestSelfBlock409() {
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/users?q=boss", nil)
	s.Require().Equal(http.StatusOK, st)
	var list struct{ Items []user }
	s.Require().NoError(json.Unmarshal(raw, &list))
	st, raw = s.put(s.admin, list.Items[0].ID, card(list.Items[0], map[string]any{"is_active": false}))
	s.Equal(http.StatusConflict, st)
	s.Equal("SELF_BLOCK", s.errOf(raw).Code)
}

// ---- SSR-экран ----

func (s *UsersLiveSuite) TestSSR_CreateEditFlow() {
	st, page := s.admin.Do(http.MethodGet, "/users", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `data-testid="users-table"`)

	form := url.Values{"name": {"Волков Олег"}, "login": {"ssr-volkov"}, "role": {"executor"}, "phone": {"8 900"}, "password": {"password1"}, "is_active": {"on"}}
	resp := s.admin.Request(http.MethodPost, "/users", form, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/users?flash=created", resp.Header.Get("Location"))

	st, page = s.admin.Do(http.MethodPost, "/users", url.Values{"name": {""}, "login": {"ssr-volkov"}, "role": {"executor"}, "password": {"1"}})
	s.Equal(http.StatusUnprocessableEntity, st)
	s.Contains(string(page), "Обязательное поле")
	s.Contains(string(page), "Минимум 8 символов")

	st, page = s.admin.Do(http.MethodPost, "/users", url.Values{"name": {"x"}, "login": {"SSR-VOLKOV"}, "role": {"executor"}, "password": {"password1"}})
	s.Equal(http.StatusConflict, st)
	s.Contains(string(page), "Логин уже занят")

	st, page = s.admin.Do(http.MethodGet, "/users?q=ssr-volkov", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), "Волков Олег")

	st, _ = s.admin.Do(http.MethodGet, "/users/not-a-uuid", nil)
	s.Equal(http.StatusNotFound, st)
	st, _ = s.admin.Do(http.MethodPost, "/users/00000000-0000-0000-0000-000000000000", form)
	s.Equal(http.StatusNotFound, st)
}

// LastAdminSuite — отдельная БД: ровно контролируемый набор администраторов.
type LastAdminSuite struct {
	testutil.LiveSuite
}

func TestLastAdminLive(t *testing.T) {
	suite.Run(t, &LastAdminSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *LastAdminSuite) put(b *testutil.Browser, id string, body map[string]any) (int, string) {
	st, raw := b.Do(http.MethodPut, "/api/v1/users/"+id, body)
	return st, string(raw)
}

func adminCard(login string, patch map[string]any) map[string]any {
	m := map[string]any{"login": login, "name": login, "role": "admin", "phone": "", "is_active": true}
	for k, v := range patch {
		m[k] = v
	}
	return m
}

func (s *LastAdminSuite) TestLastActiveAdminProtected_AndRaceKeepsOne() {
	aID := s.SeedUser("adm-a", "password1", "adm-a", "admin", true)
	a := s.NewBrowser()
	a.Login("adm-a", "password1")

	st, raw := s.put(a, aID, adminCard("adm-a", map[string]any{"role": "executor"}))
	s.Equal(http.StatusConflict, st, "единственный админ не снимает с себя роль")
	s.Contains(raw, "LAST_ADMIN")

	bID := s.SeedUser("adm-b", "password1", "adm-b", "admin", true)
	b := s.NewBrowser()
	b.Login("adm-b", "password1")
	st, raw = s.put(a, aID, adminCard("adm-a", map[string]any{"role": "executor"}))
	s.Equal(http.StatusOK, st, "при втором админе — можно: %s", raw)
	st, _ = s.put(b, aID, adminCard("adm-a", nil))
	s.Require().Equal(http.StatusOK, st)
	st, raw = s.put(b, bID, adminCard("adm-b", map[string]any{"is_active": false}))
	s.Equal(http.StatusConflict, st)
	s.Contains(raw, "SELF_BLOCK")

	// гонка: A разжалует B, B разжалует A одновременно — ровно один успех (D29, FOR UPDATE)
	a.Login("adm-a", "password1")
	for round := 0; round < 5; round++ {
		var wg sync.WaitGroup
		codes := make([]int, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			codes[0], _ = s.put(a, bID, adminCard("adm-b", map[string]any{"role": "executor"}))
		}()
		go func() {
			defer wg.Done()
			codes[1], _ = s.put(b, aID, adminCard("adm-a", map[string]any{"role": "executor"}))
		}()
		wg.Wait()
		ok := 0
		for _, c := range codes {
			if c == http.StatusOK {
				ok++
			} else {
				// проигравший: 409 LAST_ADMIN в транзакции или 403, если победитель
				// успел закоммитить до его проверки роли — оба исхода детерминированно безопасны
				s.Contains([]int{http.StatusConflict, http.StatusForbidden}, c)
			}
		}
		s.Equal(1, ok, "раунд %d: %v", round, codes)
		var admins int
		s.Require().NoError(s.Pool.QueryRow(s.T().Context(), `SELECT count(*) FROM users WHERE role='admin' AND is_active`).Scan(&admins))
		s.Equal(1, admins, "раунд %d: остался ровно один админ", round)
		// восстановить обоих админами (победитель — тот, кто остался admin)
		_, err := s.Pool.Exec(s.T().Context(), `UPDATE users SET role='admin' WHERE login IN ('adm-a','adm-b')`)
		s.Require().NoError(err)
	}
}
