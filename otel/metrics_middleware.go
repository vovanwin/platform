package otel

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// durationBuckets — явные границы бакетов гистограммы для HTTP latency (секунды).
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// MetricsMiddleware возвращает HTTP middleware, который собирает per-route метрики:
//   - {appName}.http.requests.total — счётчик запросов (method, route, status_code)
//   - {appName}.http.errors.total — счётчик ошибок status >= 400 (method, route, status_code)
//   - {appName}.http.request.duration — гистограмма длительности в секундах (method, route)
//   - {appName}.http.requests.inflight — текущие in-flight запросы (method, route)
func MetricsMiddleware(appName string) func(http.Handler) http.Handler {
	meter := otel.Meter(appName)

	requestsTotal, _ := meter.Int64Counter(
		appName+".http.requests.total",
		otelmetric.WithDescription("Total number of HTTP requests"),
	)

	errorsTotal, _ := meter.Int64Counter(
		appName+".http.errors.total",
		otelmetric.WithDescription("Total number of HTTP errors (status >= 400)"),
	)

	duration, _ := meter.Float64Histogram(
		appName+".http.request.duration",
		otelmetric.WithDescription("HTTP request duration in seconds"),
		otelmetric.WithUnit("s"),
		otelmetric.WithExplicitBucketBoundaries(durationBuckets...),
	)

	inflight, _ := meter.Int64UpDownCounter(
		appName+".http.requests.inflight",
		otelmetric.WithDescription("Number of in-flight HTTP requests"),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method := r.Method
			route := r.URL.Path

			inflightAttrs := otelmetric.WithAttributes(
				attribute.String("method", method),
				attribute.String("route", route),
			)

			inflight.Add(r.Context(), 1, inflightAttrs)
			start := time.Now()

			sw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			inflight.Add(r.Context(), -1, inflightAttrs)

			elapsed := time.Since(start).Seconds()
			statusCode := strconv.Itoa(sw.status)

			attrs := otelmetric.WithAttributes(
				attribute.String("method", method),
				attribute.String("route", route),
				attribute.String("status_code", statusCode),
			)

			requestsTotal.Add(r.Context(), 1, attrs)

			if sw.status >= 400 {
				errorsTotal.Add(r.Context(), 1, attrs)
			}

			duration.Record(r.Context(), elapsed, inflightAttrs)
		})
	}
}
