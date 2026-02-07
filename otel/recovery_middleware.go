package otel

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// RecoveryMiddleware ловит панику в HTTP handler, записывает stack trace в спан,
// инкрементит счётчик паник и возвращает 500.
func RecoveryMiddleware(appName string) func(http.Handler) http.Handler {
	meter := otel.Meter(appName)
	panicsTotal, _ := meter.Int64Counter(
		appName+".http.panics.total",
		otelmetric.WithDescription("Total number of recovered panics"),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()

					span := trace.SpanFromContext(r.Context())
					span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", rec))
					span.SetAttributes(attribute.String("panic.stack", string(stack)))

					panicsTotal.Add(r.Context(), 1, otelmetric.WithAttributes(
						attribute.String("method", r.Method),
						attribute.String("route", r.URL.Path),
					))

					slog.ErrorContext(r.Context(), "panic recovered",
						slog.Any("panic", rec),
						slog.String("stack", string(stack)),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
					)

					w.WriteHeader(http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
