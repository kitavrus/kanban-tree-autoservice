package config

import (
	"fmt"
	"strconv"
	"time"
)

// AuthConfig — session-режим (auth.mode=session, /project-bootstrap):
// HttpOnly cookie `sid`, store — Redis. Сроки — D35: sliding TTL + абсолютный потолок.
type AuthConfig struct {
	SessionTTL         time.Duration // бездействие дольше — сессия истекает (sliding)
	SessionAbsoluteMax time.Duration // потолок жизни сессии с момента входа
	CookieSecure       bool          // Secure-флаг cookie (true за HTTPS)
	LoginMaxFailures   int           // неудачных входов подряд по логину до блокировки попыток
	LoginFailureWindow time.Duration // окно счётчика неудачных входов
}

func loadAuth() (AuthConfig, error) {
	var a AuthConfig
	var err error
	if a.SessionTTL, err = duration("SESSION_TTL", "12h"); err != nil {
		return a, err
	}
	if a.SessionAbsoluteMax, err = duration("SESSION_ABSOLUTE_MAX", "168h"); err != nil {
		return a, err
	}
	if a.SessionTTL > a.SessionAbsoluteMax {
		return a, fmt.Errorf("SESSION_TTL (%s) больше SESSION_ABSOLUTE_MAX (%s)", a.SessionTTL, a.SessionAbsoluteMax)
	}
	if a.CookieSecure, err = strconv.ParseBool(getenv("SESSION_COOKIE_SECURE", "false")); err != nil {
		return a, fmt.Errorf("SESSION_COOKIE_SECURE: %w", err)
	}
	if a.LoginMaxFailures, err = strconv.Atoi(getenv("LOGIN_MAX_FAILURES", "10")); err != nil || a.LoginMaxFailures < 1 {
		return a, fmt.Errorf("LOGIN_MAX_FAILURES: ожидается целое ≥ 1")
	}
	if a.LoginFailureWindow, err = duration("LOGIN_FAILURE_WINDOW", "15m"); err != nil {
		return a, err
	}
	return a, nil
}

func duration(key, def string) (time.Duration, error) {
	raw := getenv(key, def)
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s '%s': ожидается положительная длительность", key, raw)
	}
	return d, nil
}
