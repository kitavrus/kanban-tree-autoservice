// Package errorspkg — единый формат ошибок API (wire — CLAUDE.md §API-1/§FE-1.1):
// JSON snake_case, валидация 400 несёт details.fields[] — клиент раскладывает по полям.
package errorspkg

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// Field — ошибка конкретного поля (валидация server-only, §FE-1.1).
type Field struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

type AppError struct {
	Code    string         `json:"code"`
	HTTP    int            `json:"-"`
	Message string         `json:"message"`
	Fields  []Field        `json:"-"`
	Extra   map[string]any `json:"-"` // доп. ключи details (snake_case); sentinel не мутируется — см. WithDetail
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }

func New(code string, httpStatus int, message string) *AppError {
	return &AppError{Code: code, HTTP: httpStatus, Message: message}
}

// WithDetail — КОПИЯ ошибки с доп. ключом details (ADR-102): sentinel'ы — общие var в
// пакетах-фичах (var ErrXxx = errorspkg.New(...)), их нельзя мутировать на лету — иначе
// один запрос подмешал бы Extra в ошибку, которую увидит другой параллельный запрос.
func (e *AppError) WithDetail(key string, v any) *AppError {
	cp := *e
	cp.Extra = make(map[string]any, len(e.Extra)+1)
	for k, val := range e.Extra {
		cp.Extra[k] = val
	}
	cp.Extra[key] = v
	return &cp
}

// ErrRequestTooLarge — тело запроса больше лимита роута (413). Общий sentinel для
// middleware.BodyLimit и хендлеров, читающих поток сами (загрузка вложений) — фича не
// импортирует серверный слой ради ошибки.
var ErrRequestTooLarge = New("REQUEST_TOO_LARGE", http.StatusRequestEntityTooLarge, "Слишком большой запрос")

func BadRequest(msg string) *AppError   { return New("BAD_REQUEST", http.StatusBadRequest, msg) }
func Unauthorized(msg string) *AppError { return New("UNAUTHORIZED", http.StatusUnauthorized, msg) }
func Forbidden(msg string) *AppError    { return New("FORBIDDEN", http.StatusForbidden, msg) }
func NotFound(msg string) *AppError     { return New("NOT_FOUND", http.StatusNotFound, msg) }
func Conflict(msg string) *AppError     { return New("CONFLICT", http.StatusConflict, msg) }
func Internal(msg string) *AppError     { return New("INTERNAL", http.StatusInternalServerError, msg) }

// Validation — 400 с пополевыми ошибками.
func Validation(fields ...Field) *AppError {
	e := New("VALIDATION", http.StatusBadRequest, "validation failed")
	e.Fields = fields
	return e
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// FiberErrorHandler — маппинг ошибок в wire-формат. Мусор/паника наружу не течёт:
// неизвестная ошибка = 500 INTERNAL без текста первопричины.
func FiberErrorHandler(c fiber.Ctx, err error) error {
	var app *AppError
	if errors.As(err, &app) {
		body := errorBody{Code: app.Code, Message: app.Message}
		// details = {"fields": Fields (если есть)} ∪ Extra; details опускается, если оба
		// пусты (ADR-102) — существующий wire VALIDATION (details.fields[]) не меняется.
		if len(app.Fields) > 0 || len(app.Extra) > 0 {
			details := make(map[string]any, len(app.Extra)+1)
			for k, v := range app.Extra {
				details[k] = v
			}
			if len(app.Fields) > 0 {
				details["fields"] = app.Fields
			}
			body.Details = details
		}
		return c.Status(app.HTTP).JSON(body)
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return c.Status(fe.Code).JSON(errorBody{Code: "HTTP_ERROR", Message: fe.Message})
	}
	// Неизвестная ошибка (SQL/паника/т.п.) — клиенту наружу текст НЕ течёт, но в
	// логи пишем полностью: иначе 500 немой и первопричину не найти без дебаггера.
	slog.Error("unhandled error", "err", err, "path", c.Path(), "method", c.Method())
	return c.Status(http.StatusInternalServerError).
		JSON(errorBody{Code: "INTERNAL", Message: "internal error"})
}
