package otel

import (
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// TraceIDMiddleware добавляет заголовок X-Trace-ID в HTTP ответ.
// Должен стоять ПОСЛЕ HTTPMiddleware (otelhttp), который создаёт span в контексте.
// Позволяет клиентам получить trace_id для отладки и поиска трейса в Tempo/Grafana.
func TraceIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := trace.SpanFromContext(r.Context())
			if span.SpanContext().IsValid() {
				w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
			}

			next.ServeHTTP(w, r)
		})
	}
}
