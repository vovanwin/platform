package otel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceIDHandler оборачивает slog.Handler, добавляя trace_id и span_id из контекста в каждую запись.
// Это связывает логи с трейсами в Grafana/Tempo.
type TraceIDHandler struct {
	inner slog.Handler
}

// NewTraceIDHandler создаёт handler, добавляющий trace_id/span_id к логам.
func NewTraceIDHandler(inner slog.Handler) *TraceIDHandler {
	return &TraceIDHandler{inner: inner}
}

func (h *TraceIDHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *TraceIDHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, record)
}

func (h *TraceIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceIDHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *TraceIDHandler) WithGroup(name string) slog.Handler {
	return &TraceIDHandler{inner: h.inner.WithGroup(name)}
}
