package observability

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests by method/path/status.",
	}, []string{"method", "path", "status"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// MetricsMiddleware — счётчик и гистограмма запросов (/metrics — Prometheus).
func MetricsMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		path := c.Route().Path // шаблон роута, не сырой URL — без кардинального взрыва
		httpRequests.WithLabelValues(c.Method(), path,
			strconv.Itoa(c.Response().StatusCode())).Inc()
		httpDuration.WithLabelValues(c.Method(), path).Observe(time.Since(start).Seconds())
		return err
	}
}
