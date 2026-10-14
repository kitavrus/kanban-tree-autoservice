package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeRedirect(t *testing.T) {
	cases := map[string]string{
		"":                          "/",
		"/":                         "/",
		"/users":                    "/users",
		"/users?q=%D0%B8%D0%B2&x=1": "/users?q=%D0%B8%D0%B2&x=1",
		"/users/abc#top":            "/users/abc#top",
		"//evil.example":            "/",
		"///evil.example":           "/",
		"/\\evil.example":           "/",
		"https://evil.example/":     "/",
		"http:/evil":                "/",
		"javascript:alert(1)":       "/",
		"evil.example":              "/",
		"/login":                    "/", // петля на вход
		"/login?redirect=/users":    "/",
		"/users\r\nSet-Cookie: x=1": "/",
		"/%2F%2Fevil.example":       "/%2F%2Fevil.example", // остаётся путём этого сайта
	}
	for in, want := range cases {
		assert.Equal(t, want, SafeRedirect(in), "%q", in)
	}
}

func TestNormalizeLogin(t *testing.T) {
	assert.Equal(t, "admin", NormalizeLogin("  Admin "))
	assert.Equal(t, "sidorov.p", NormalizeLogin("SIDOROV.P"))
}
