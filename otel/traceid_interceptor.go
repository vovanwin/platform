package otel

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TraceIDUnaryInterceptor добавляет X-Trace-ID в gRPC response metadata (headers).
// Позволяет gRPC клиентам получить trace_id для отладки.
func TraceIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			header := metadata.Pairs("x-trace-id", span.SpanContext().TraceID().String())
			_ = grpc.SetHeader(ctx, header)
		}

		return handler(ctx, req)
	}
}
