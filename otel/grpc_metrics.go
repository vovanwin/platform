package otel

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// grpcMethodInstruments — набор метрик для одного gRPC метода.
type grpcMethodInstruments struct {
	requests otelmetric.Int64Counter
	errors   otelmetric.Int64Counter
	duration otelmetric.Float64Histogram
}

// GRPCMetrics создаёт отдельные OTEL-инструменты для каждого зарегистрированного gRPC метода.
//
// Пример метрик для метода "/users.UserService/GetUser":
//
//	myapp.grpc.users.userservice.getuser.requests  — счётчик вызовов
//	myapp.grpc.users.userservice.getuser.errors    — счётчик ошибок (non-OK status)
//	myapp.grpc.users.userservice.getuser.duration  — гистограмма длительности (секунды)
type GRPCMetrics struct {
	methods  map[string]*grpcMethodInstruments
	fallback *grpcMethodInstruments
}

// NewGRPCMetrics создаёт метрики для каждого из переданных gRPC методов.
// Формат метода: полный путь (напр. "/users.UserService/GetUser").
// Незарегистрированные методы попадают в fallback-метрику "{appName}.grpc.other".
func NewGRPCMetrics(appName string, methods []string) *GRPCMetrics {
	meter := otel.Meter(appName)
	gm := &GRPCMetrics{
		methods: make(map[string]*grpcMethodInstruments, len(methods)),
	}

	for _, method := range methods {
		name := sanitizeGRPCMethod(method)
		prefix := appName + ".grpc." + name
		gm.methods[method] = newGRPCMethodInstruments(meter, prefix)
	}

	gm.fallback = newGRPCMethodInstruments(meter, appName+".grpc.other")

	return gm
}

func newGRPCMethodInstruments(meter otelmetric.Meter, prefix string) *grpcMethodInstruments {
	requests, _ := meter.Int64Counter(
		prefix+".requests",
		otelmetric.WithDescription("Total gRPC calls for "+prefix),
	)
	errors, _ := meter.Int64Counter(
		prefix+".errors",
		otelmetric.WithDescription("Total gRPC errors for "+prefix),
	)
	duration, _ := meter.Float64Histogram(
		prefix+".duration",
		otelmetric.WithDescription("gRPC call duration for "+prefix),
		otelmetric.WithUnit("s"),
		otelmetric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	return &grpcMethodInstruments{requests: requests, errors: errors, duration: duration}
}

// UnaryInterceptor возвращает gRPC unary interceptor для записи per-method метрик.
func (gm *GRPCMetrics) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		inst := gm.methods[info.FullMethod]
		if inst == nil {
			inst = gm.fallback
		}

		inst.requests.Add(ctx, 1)

		if err != nil {
			st, _ := status.FromError(err)
			inst.errors.Add(ctx, 1, otelmetric.WithAttributes(
				attribute.String("grpc_code", st.Code().String()),
			))
		}

		inst.duration.Record(ctx, time.Since(start).Seconds())

		return resp, err
	}
}

// StreamInterceptor возвращает gRPC stream interceptor для записи per-method метрик.
func (gm *GRPCMetrics) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		err := handler(srv, ss)

		inst := gm.methods[info.FullMethod]
		if inst == nil {
			inst = gm.fallback
		}

		inst.requests.Add(ss.Context(), 1)

		if err != nil {
			st, _ := status.FromError(err)
			inst.errors.Add(ss.Context(), 1, otelmetric.WithAttributes(
				attribute.String("grpc_code", st.Code().String()),
			))
		}

		inst.duration.Record(ss.Context(), time.Since(start).Seconds())

		return err
	}
}

// sanitizeGRPCMethod превращает "/users.UserService/GetUser" в "users.userservice.getuser".
func sanitizeGRPCMethod(method string) string {
	s := strings.ToLower(method)
	s = strings.ReplaceAll(s, "/", ".")
	s = strings.Trim(s, ".")
	return s
}
