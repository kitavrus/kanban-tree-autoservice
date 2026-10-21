//go:build integration

package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// defaultExtraEnv — session-режим: обязательного auth-env нет.
func defaultExtraEnv() []string { return nil }

// csrfCookie / csrfHeader — double-submit CSRF (internal/server/middleware/csrf.go).
const (
	csrfCookie = "kt_csrf"
	csrfHeader = "X-Csrf-Token"
)

// Browser — отдельный «браузер»: свой cookie jar (своя сессия и CSRF-cookie),
// редиректы НЕ следуются (тест видит 302/303 и Location).
type Browser struct {
	s      *LiveSuite
	Client *http.Client
}

// NewBrowser — новый браузер с пустым jar (другой пользователь / другое устройство).
func (s *LiveSuite) NewBrowser() *Browser {
	jar, err := cookiejar.New(nil)
	s.Require().NoError(err)
	return &Browser{s: s, Client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// browser — браузер по умолчанию поверх s.Client (его jar).
func (s *LiveSuite) browser() *Browser {
	return &Browser{s: s, Client: &http.Client{
		Jar:           s.Client.Jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// CSRFToken — токен из cookie kt_csrf (при отсутствии — получить его GET /login).
func (b *Browser) CSRFToken() string {
	u, _ := url.Parse(b.s.BaseURL)
	for i := 0; i < 2; i++ {
		for _, c := range b.Client.Jar.Cookies(u) {
			if c.Name == csrfCookie {
				return c.Value
			}
		}
		resp, err := b.Client.Get(b.s.BaseURL + "/login")
		b.s.Require().NoError(err)
		_ = resp.Body.Close()
	}
	b.s.FailNow("нет cookie " + csrfCookie)
	return ""
}

// Request — запрос как из браузера: X-Csrf-Token на мутациях (если не задан в headers).
// body: nil | string/[]byte (как есть) | url.Values (форма) | значение (JSON).
func (b *Browser) Request(method, path string, body any, headers map[string]string) *http.Response {
	var r io.Reader
	ct := ""
	switch v := body.(type) {
	case nil:
	case string:
		r, ct = strings.NewReader(v), "application/json"
	case []byte:
		r, ct = bytes.NewReader(v), "application/json"
	case url.Values:
		r, ct = strings.NewReader(v.Encode()), "application/x-www-form-urlencoded"
	default:
		raw, err := json.Marshal(v)
		b.s.Require().NoError(err)
		r, ct = bytes.NewReader(raw), "application/json"
	}
	req, err := http.NewRequest(method, b.s.BaseURL+path, r)
	b.s.Require().NoError(err)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if method != http.MethodGet && method != http.MethodHead {
		if _, ok := headers[csrfHeader]; !ok {
			req.Header.Set(csrfHeader, b.CSRFToken())
		}
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := b.Client.Do(req)
	b.s.Require().NoError(err)
	return resp
}

// Do — Request + прочитанное тело.
func (b *Browser) Do(method, path string, body any) (int, []byte) {
	resp := b.Request(method, path, body, nil)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	b.s.Require().NoError(err)
	return resp.StatusCode, out
}

// Login — реальный вход через POST /api/v1/auth/login (§TD-8, auth-стабы запрещены).
func (b *Browser) Login(login, password string) {
	status, body := b.Do(http.MethodPost, "/api/v1/auth/login", map[string]string{"login": login, "password": password})
	b.s.Require().Equal(http.StatusOK, status, "реальный логин обязан пройти (§TD-8): %s", body)
}

// Do / LoginAs — то же через браузер по умолчанию (s.Client).
func (s *LiveSuite) Do(method, path string, body any) (int, []byte) {
	return s.browser().Do(method, path, body)
}

func (s *LiveSuite) LoginAs(login, password string) { s.browser().Login(login, password) }

// SeedUser — пользователь прямо в БД (bcrypt MinCost — быстро); возвращает id.
func (s *BaseSuite) SeedUser(login, password, name, role string, active bool) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(s.T(), err)
	var id string
	err = s.Pool.QueryRow(context.Background(),
		`INSERT INTO users (login, password_hash, name, role, is_active) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		login, string(hash), name, role, active).Scan(&id)
	require.NoError(s.T(), err)
	return id
}

// SeedSupplierUser — учётка роли supplier, привязанная к поставщику supplierID (purchase-orders
// DS-007, TS-F3): CHECK users_supplier_chk требует supplier_id у этой роли.
func (s *BaseSuite) SeedSupplierUser(login, password, name, supplierID string, active bool) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(s.T(), err)
	var id string
	err = s.Pool.QueryRow(context.Background(),
		`INSERT INTO users (login, password_hash, name, role, is_active, supplier_id)
		 VALUES ($1, $2, $3, 'supplier', $4, $5::uuid) RETURNING id`,
		login, string(hash), name, active, supplierID).Scan(&id)
	require.NoError(s.T(), err)
	return id
}
