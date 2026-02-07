package otel

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler возвращает HTTP handler для Prometheus метрик.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
