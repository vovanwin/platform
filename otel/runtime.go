package otel

import (
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

// StartRuntimeMetrics запускает сбор Go runtime метрик:
// goroutines, heap alloc, GC pauses, открытые файловые дескрипторы и др.
// Метрики автоматически экспортируются через глобальный MeterProvider.
func StartRuntimeMetrics() error {
	if err := runtime.Start(); err != nil {
		return fmt.Errorf("start runtime metrics: %w", err)
	}
	return nil
}
