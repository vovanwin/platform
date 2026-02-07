package otel

import (
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
)

// routeInstruments — набор метрик для одного роута.
type routeInstruments struct {
	requests otelmetric.Int64Counter
	errors   otelmetric.Int64Counter
	duration otelmetric.Float64Histogram
}

// RouteMetrics создаёт отдельные OTEL-инструменты для каждого зарегистрированного HTTP роута.
// Это избавляет от проблемы кардинальности: каждый роут = свои метрики = свой график в Grafana.
//
// Пример метрик для роута "GET /api/v1/users":
//
//	myapp.route.get.api.v1.users.requests  — счётчик запросов
//	myapp.route.get.api.v1.users.errors    — счётчик ошибок (status >= 400)
//	myapp.route.get.api.v1.users.duration  — гистограмма длительности (секунды)
type RouteMetrics struct {
	routes   map[string]*routeInstruments
	fallback *routeInstruments
}

// NewRouteMetrics создаёт метрики для каждого из переданных роутов.
// Формат роута: "METHOD /path" (напр. "GET /api/v1/users", "POST /api/v1/orders/{id}").
// Незарегистрированные роуты попадают в fallback-метрику "{appName}.route.other".
func NewRouteMetrics(appName string, routes []string) *RouteMetrics {
	meter := otel.Meter(appName)
	rm := &RouteMetrics{
		routes: make(map[string]*routeInstruments, len(routes)),
	}

	for _, route := range routes {
		name := sanitizeRouteName(route)
		prefix := appName + ".route." + name
		rm.routes[route] = newRouteInstruments(meter, prefix)
	}

	rm.fallback = newRouteInstruments(meter, appName+".route.other")

	return rm
}

func newRouteInstruments(meter otelmetric.Meter, prefix string) *routeInstruments {
	requests, _ := meter.Int64Counter(
		prefix+".requests",
		otelmetric.WithDescription("Total requests for "+prefix),
	)
	errors, _ := meter.Int64Counter(
		prefix+".errors",
		otelmetric.WithDescription("Total errors (status >= 400) for "+prefix),
	)
	duration, _ := meter.Float64Histogram(
		prefix+".duration",
		otelmetric.WithDescription("Request duration for "+prefix),
		otelmetric.WithUnit("s"),
		otelmetric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	return &routeInstruments{requests: requests, errors: errors, duration: duration}
}

// Middleware возвращает HTTP middleware для записи per-route метрик.
// routeFunc вызывается после обработки запроса для получения паттерна роута.
// Для chi: func(r *http.Request) string { return chi.RouteContext(r.Context()).RoutePattern() }
func (rm *RouteMetrics) Middleware(routeFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			// После обработки запроса chi уже установил RouteContext
			pattern := routeFunc(r)
			key := r.Method + " " + pattern

			inst := rm.routes[key]
			if inst == nil {
				inst = rm.fallback
			}

			ctx := r.Context()
			inst.requests.Add(ctx, 1)
			if sw.status >= 400 {
				inst.errors.Add(ctx, 1)
			}
			inst.duration.Record(ctx, time.Since(start).Seconds())
		})
	}
}

// sanitizeRouteName превращает "GET /api/v1/users/{id}" в "get.api.v1.users.id".
func sanitizeRouteName(route string) string {
	s := strings.ToLower(route)
	s = strings.ReplaceAll(s, " ", ".")
	s = strings.ReplaceAll(s, "/", ".")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, "..", ".")
	s = strings.Trim(s, ".")
	return s
}
