package observability

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer — OTLP/HTTP экспорт (Jaeger из monitoring-стека /infra-bootstrap).
// Пустой endpoint → трейсинг no-op (dev без Jaeger), shutdown безопасен.
func InitTracer(ctx context.Context, service, endpoint string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	res, err := sdkresource.Merge(sdkresource.Default(),
		sdkresource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(service)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// TraceMiddleware — span на каждый HTTP-запрос.
func TraceMiddleware(service string) fiber.Handler {
	tracer := otel.Tracer(service)
	return func(c fiber.Ctx) error {
		ctx, span := tracer.Start(c.Context(), c.Method()+" "+c.Path(),
			trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		c.SetContext(ctx)
		err := c.Next()
		span.SetAttributes(
			attribute.Int("http.status_code", c.Response().StatusCode()),
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", c.Path()),
		)
		return err
	}
}
