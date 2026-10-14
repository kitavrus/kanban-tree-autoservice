package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// RequestLogger — структурный access-лог (slog).
func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Info("http",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"dur_ms", time.Since(start).Milliseconds(),
			"request_id", requestid.FromContext(c),
		)
		return err
	}
}
