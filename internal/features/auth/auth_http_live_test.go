//go:build integration

package auth_test

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

// AuthLiveSuite (§TD-8): вход/выход, сессия, CSRF и гарды — боевой бинарь, реальная сеть.
type AuthLiveSuite struct {
	testutil.LiveSuite
}

func TestAuthLive(t *testing.T) {
	suite.Run(t, &AuthLiveSuite{LiveSuite: testutil.LiveSuite{
		BinaryPkg: "./cmd/server",
		ExtraEnv:  []string{"LOGIN_MAX_FAILURES=3"},
	}})
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

func decodeErr(s *AuthLiveSuite, body []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(body, &e), string(body))
	return e
}

func (s *AuthLiveSuite) login(b *testutil.Browser, login, password string) (int, []byte) {
	return b.Do(http.MethodPost, "/api/v1/auth/login", map[string]string{"login": login, "password": password})
}

func (s *AuthLiveSuite) TestLogin_SetsHardenedCookie_AndMe() {
	s.SeedUser("a-ok", "password1", "Админ Окей", "admin", true)
	b := s.NewBrowser()
	resp := b.Request(http.MethodPost, "/api/v1/auth/login", map[string]string{"login": "A-OK ", "password": "password1"}, nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	var me map[string]string
	s.Require().NoError(json.Unmarshal(body, &me))
	s.NotEmpty(me["id"])
	delete(me, "id")
	// timezone — пояс пользователя (user-timezone TS-021, DS-031): умолчание Asia/Almaty.
	s.Equal(map[string]string{"login": "a-ok", "name": "Админ Окей", "role": "admin", "timezone": "Asia/Almaty"}, me, "логин нормализован")

	var sid *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "sid" {
			sid = c
		}
	}
	s.Require().NotNil(sid)
	s.True(sid.HttpOnly)
	s.Equal(http.SameSiteStrictMode, sid.SameSite)
	s.Equal(7*24*3600, sid.MaxAge, "срок cookie = абсолютный потолок сессии (D35)")

	status, meBody := b.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusOK, status)
	s.Contains(string(meBody), `"role":"admin"`)
	s.Contains(string(meBody), `"timezone":"Asia/Almaty"`)

	// TS-021: /auth/me читает пояс из БД на каждый запрос — смена видна без перелогина.
	_, err := s.Pool.Exec(s.T().Context(), `UPDATE users SET timezone = 'Europe/Moscow' WHERE login = 'a-ok'`)
	s.Require().NoError(err)
	status, meBody = b.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusOK, status)
	s.Contains(string(meBody), `"timezone":"Europe/Moscow"`)
}

func (s *AuthLiveSuite) TestLogin_WrongPasswordAndUnknownLoginLookTheSame() {
	s.SeedUser("a-wrong", "password1", "X", "executor", true)
	b := s.NewBrowser()
	st1, body1 := s.login(b, "a-wrong", "nope-nope")
	st2, body2 := s.login(b, "a-nobody", "nope-nope")
	s.Equal(http.StatusUnauthorized, st1)
	s.Equal(st1, st2)
	s.Equal(string(body1), string(body2), "ответ не выдаёт, существует ли логин")
	s.Equal("INVALID_CREDENTIALS", decodeErr(s, body1).Code)
}

func (s *AuthLiveSuite) TestLogin_Blocked() {
	s.SeedUser("a-blocked", "password1", "X", "executor", false)
	b := s.NewBrowser()
	st, body := s.login(b, "a-blocked", "password1")
	s.Equal(http.StatusForbidden, st)
	s.Equal("USER_BLOCKED", decodeErr(s, body).Code)
	st, body = s.login(b, "a-blocked", "wrong-pass")
	s.Equal(http.StatusUnauthorized, st, "без верного пароля о блокировке не сообщаем")
	s.Equal("INVALID_CREDENTIALS", decodeErr(s, body).Code)
}

func (s *AuthLiveSuite) TestLogin_BruteForceLimit() {
	s.SeedUser("a-brute", "password1", "X", "executor", true)
	s.SeedUser("a-brute2", "password1", "X", "executor", true)
	b := s.NewBrowser()
	for i := 0; i < 3; i++ {
		st, _ := s.login(b, "a-brute", "wrong-pass")
		s.Equal(http.StatusUnauthorized, st)
	}
	st, body := s.login(b, "A-Brute", "password1")
	s.Equal(http.StatusTooManyRequests, st, "лимит по логину, регистр не важен")
	s.Equal("TOO_MANY_ATTEMPTS", decodeErr(s, body).Code)

	st, _ = s.login(b, "a-brute2", "password1")
	s.Equal(http.StatusOK, st, "другой логин не затронут")
}

// Параллельный перебор не обходит лимит: счётчик атомарно увеличивается ДО проверки
// пароля (D41) — из 20 одновременных попыток пароль проверяется ровно у LOGIN_MAX_FAILURES.
func (s *AuthLiveSuite) TestLogin_BruteForceLimit_Parallel() {
	s.SeedUser("a-par", "password1", "X", "executor", true)
	codes := make([]int, 20)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = s.login(s.NewBrowser(), "a-par", "wrong-pass")
		}(i)
	}
	wg.Wait()
	count := map[int]int{}
	for _, c := range codes {
		count[c]++
	}
	s.Equal(map[int]int{http.StatusUnauthorized: 3, http.StatusTooManyRequests: 17}, count)
	st, _ := s.login(s.NewBrowser(), "a-par", "password1")
	s.Equal(http.StatusTooManyRequests, st, "верный пароль тоже ждёт окончания окна")
}

// Мусор на входе — 401, не 500: пароль > 72 байт (предел bcrypt), логин длиннее допустимого.
func (s *AuthLiveSuite) TestLogin_OversizedInput() {
	s.SeedUser("a-big", "password1", "X", "executor", true)
	b := s.NewBrowser()
	st, _ := s.login(b, "a-big", strings.Repeat("п", 100))
	s.Equal(http.StatusUnauthorized, st)
	st, _ = s.login(b, strings.Repeat("x", 5000), "password1")
	s.Equal(http.StatusUnauthorized, st)
}

// Вход под другим пользователем без выхода — прежняя сессия браузера убита.
func (s *AuthLiveSuite) TestRelogin_KillsPreviousSession() {
	s.SeedUser("a-first", "password1", "X", "executor", true)
	s.SeedUser("a-second", "password1", "Y", "executor", true)
	b := s.NewBrowser()
	b.Login("a-first", "password1")
	u, _ := url.Parse(s.BaseURL)
	var first string
	for _, c := range b.Client.Jar.Cookies(u) {
		if c.Name == "sid" {
			first = c.Value
		}
	}
	s.Require().NotEmpty(first)
	b.Login("a-second", "password1")

	resp := s.NewBrowser().Request(http.MethodGet, "/api/v1/auth/me", nil, map[string]string{"Cookie": "sid=" + first})
	resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	st, me := b.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(me), `"login":"a-second"`)
}

func (s *AuthLiveSuite) TestLogin_SuccessResetsFailures() {
	s.SeedUser("a-reset", "password1", "X", "executor", true)
	b := s.NewBrowser()
	for i := 0; i < 2; i++ {
		st, _ := s.login(b, "a-reset", "wrong-pass")
		s.Equal(http.StatusUnauthorized, st)
	}
	st, _ := s.login(b, "a-reset", "password1")
	s.Equal(http.StatusOK, st)
	for i := 0; i < 2; i++ {
		st, _ := s.login(s.NewBrowser(), "a-reset", "wrong-pass")
		s.Equal(http.StatusUnauthorized, st, "счётчик сброшен успешным входом")
	}
}

func (s *AuthLiveSuite) TestLogin_BadInput400() {
	b := s.NewBrowser()
	st, body := b.Do(http.MethodPost, "/api/v1/auth/login", map[string]string{})
	s.Equal(http.StatusBadRequest, st)
	e := decodeErr(s, body)
	s.Require().NotNil(e.Details)
	s.Len(e.Details.Fields, 2)

	for _, raw := range []string{`{`, `null`, `[]`, `{"login":"x","password":"y","role":"admin"}`, `{"login":1}`, `{"login":"x","password":"y"} {}`} {
		st, body := b.Do(http.MethodPost, "/api/v1/auth/login", raw)
		s.Equal(http.StatusBadRequest, st, "%s → %s", raw, body)
	}
}

func (s *AuthLiveSuite) TestCSRF_RequiredOnMutations() {
	s.SeedUser("a-csrf", "password1", "X", "executor", true)
	b := s.NewBrowser()
	creds := map[string]string{"login": "a-csrf", "password": "password1"}

	resp := b.Request(http.MethodPost, "/api/v1/auth/login", creds, map[string]string{"X-Csrf-Token": ""})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode)
	s.Equal("CSRF_INVALID", decodeErr(s, body).Code)

	resp = b.Request(http.MethodPost, "/api/v1/auth/login", creds, map[string]string{"X-Csrf-Token": strings.Repeat("a", 64)})
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode, "токен не совпадает с cookie")

	resp = b.Request(http.MethodPost, "/api/v1/auth/login", creds, map[string]string{"Origin": "http://evil.example"})
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode, "чужой Origin")

	resp = b.Request(http.MethodPost, "/login", url.Values{"login": {"a-csrf"}, "password": {"password1"}}, map[string]string{"X-Csrf-Token": ""})
	resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode, "SSR-форма без _csrf")

	st, _ := s.login(b, "a-csrf", "password1")
	s.Equal(http.StatusOK, st, "с токеном — вход проходит")
}

func (s *AuthLiveSuite) TestGuards_Anonymous() {
	b := s.NewBrowser()
	resp := b.Request(http.MethodGet, "/", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusFound, resp.StatusCode)
	s.Equal("/login?redirect=%2F", resp.Header.Get("Location"))

	resp = b.Request(http.MethodGet, "/users?q=x", nil, nil)
	resp.Body.Close()
	s.Equal("/login?redirect=%2Fusers%3Fq%3Dx", resp.Header.Get("Location"))

	st, body := b.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusUnauthorized, st)
	s.Equal("UNAUTHORIZED", decodeErr(s, body).Code)

	resp = b.Request(http.MethodGet, "/", nil, map[string]string{"Cookie": "sid=forged"})
	resp.Body.Close()
	s.Equal(http.StatusFound, resp.StatusCode, "поддельный sid — как без сессии")
}

func (s *AuthLiveSuite) TestSSRLoginForm() {
	s.SeedUser("a-form", "password1", "Форма", "executor", true)
	b := s.NewBrowser()

	st, page := b.Do(http.MethodGet, "/login?redirect=%2Fusers", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), `name="redirect" value="/users"`)

	st, page = b.Do(http.MethodPost, "/login", url.Values{"login": {"a-form"}, "password": {"bad-bad-bad"}, "redirect": {"/users"}})
	s.Equal(http.StatusUnauthorized, st)
	s.Contains(string(page), "Неверный логин или пароль")
	s.Contains(string(page), `value="a-form"`, "логин остаётся в форме")

	st, page = b.Do(http.MethodPost, "/login", url.Values{"login": {""}, "password": {""}})
	s.Equal(http.StatusBadRequest, st)
	s.Contains(string(page), "Введите логин и пароль")

	resp := b.Request(http.MethodPost, "/login", url.Values{"login": {"a-form"}, "password": {"password1"}, "redirect": {"https://evil.example/x"}}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/", resp.Header.Get("Location"), "open redirect закрыт")

	resp = b.Request(http.MethodGet, "/login?redirect=%2Fusers", nil, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode, "вошедшего /login отправляет дальше")
	s.Equal("/users", resp.Header.Get("Location"))

	st, page = b.Do(http.MethodGet, "/", nil)
	s.Equal(http.StatusOK, st)
	s.Contains(string(page), "Форма")
	s.Contains(string(page), `action="/logout"`)
}

func (s *AuthLiveSuite) TestLogout_KillsSession() {
	s.SeedUser("a-out", "password1", "X", "executor", true)
	b := s.NewBrowser()
	b.Login("a-out", "password1")
	u, _ := url.Parse(s.BaseURL)
	var sid string
	for _, c := range b.Client.Jar.Cookies(u) {
		if c.Name == "sid" {
			sid = c.Value
		}
	}
	s.Require().NotEmpty(sid)

	resp := b.Request(http.MethodPost, "/logout", url.Values{}, nil)
	resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Equal("/login", resp.Header.Get("Location"))

	st, _ := b.Do(http.MethodGet, "/api/v1/auth/me", nil)
	s.Equal(http.StatusUnauthorized, st)

	thief := s.NewBrowser()
	resp = thief.Request(http.MethodGet, "/api/v1/auth/me", nil, map[string]string{"Cookie": "sid=" + sid})
	resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode, "старый sid после выхода недействителен")
}
