package server

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	platformotel "github.com/vovanwin/platform/otel"
	"google.golang.org/grpc"
)

// Config — конфигурация серверов, передаётся потребителем.
type Config struct {
	Host        string
	GRPCPort    string
	HTTPPort    string
	SwaggerPort string
	DebugPort   string
	SwaggerFS   fs.FS // встроенная FS с файлами *.swagger.json
	ProtoFS     fs.FS // встроенная FS с файлами *.proto
}

// GRPCRegistrator — колбэк для регистрации gRPC сервисов.
type GRPCRegistrator func(s *grpc.Server)

// GatewayRegistrator — колбэк для регистрации grpc-gateway in-process.
type GatewayRegistrator func(ctx context.Context, mux *runtime.ServeMux, server *grpc.Server) error

// Option — функциональные опции для Server.
type Option func(*Server)

// DebugHandler — пользовательский обработчик для debug-сервера.
type DebugHandler struct {
	Pattern string
	Handler http.Handler
}

// Server объединяет все 4 сервера: gRPC, HTTP gateway, Swagger, Debug.
type Server struct {
	cfg Config

	grpcRegistrators    []GRPCRegistrator
	gatewayRegistrators []GatewayRegistrator
	httpMiddleware      []func(http.Handler) http.Handler
	debugMiddleware     []func(http.Handler) http.Handler
	debugHandlers       []DebugHandler
	grpcOptions         []grpc.ServerOption

	otelCfg      *platformotel.Config
	otelProvider *platformotel.Provider
	httpRoutes   []string // роуты для per-route HTTP метрик
	grpcMethods  []string // методы для per-method gRPC метрик

	grpcServer *grpc.Server
	httpServer *http.Server
	swaggerSrv *http.Server
	debugSrv   *http.Server
}

// WithGRPCRegistrator добавляет колбэк для регистрации gRPC сервисов.
func WithGRPCRegistrator(r GRPCRegistrator) Option {
	return func(s *Server) {
		s.grpcRegistrators = append(s.grpcRegistrators, r)
	}
}

// WithGatewayRegistrator добавляет колбэк для регистрации grpc-gateway хендлеров.
func WithGatewayRegistrator(r GatewayRegistrator) Option {
	return func(s *Server) {
		s.gatewayRegistrators = append(s.gatewayRegistrators, r)
	}
}

// WithHTTPMiddleware добавляет middleware на HTTP gateway.
func WithHTTPMiddleware(mw ...func(http.Handler) http.Handler) Option {
	return func(s *Server) {
		s.httpMiddleware = append(s.httpMiddleware, mw...)
	}
}

// WithDebugMiddleware добавляет middleware на Debug сервер.
func WithDebugMiddleware(mw ...func(http.Handler) http.Handler) Option {
	return func(s *Server) {
		s.debugMiddleware = append(s.debugMiddleware, mw...)
	}
}

// WithDebugHandler монтирует пользовательский HTTP handler на debug-сервер.
func WithDebugHandler(pattern string, handler http.Handler) Option {
	return func(s *Server) {
		s.debugHandlers = append(s.debugHandlers, DebugHandler{Pattern: pattern, Handler: handler})
	}
}

// WithGRPCOptions добавляет опции для gRPC сервера.
func WithGRPCOptions(opts ...grpc.ServerOption) Option {
	return func(s *Server) {
		s.grpcOptions = append(s.grpcOptions, opts...)
	}
}

// WithOtel включает автоматическую инициализацию OTEL при старте сервера.
// При включении автоматически добавляются:
//   - трейсы и метрики (TracerProvider + MeterProvider)
//   - RecoveryMiddleware (panic recovery с записью в спан)
//   - MetricsMiddleware (per-route HTTP метрики)
//   - HTTPMiddleware (OTEL трейсинг HTTP)
//   - otelgrpc StatsHandler (OTEL трейсинг gRPC)
//   - /metrics endpoint на debug-сервере
//   - graceful shutdown провайдеров при остановке
func WithOtel(cfg platformotel.Config) Option {
	return func(s *Server) {
		s.otelCfg = &cfg
	}
}

// WithHTTPRouteMetrics регистрирует HTTP роуты для отдельных per-route метрик.
// Каждый роут получает собственные счётчики и гистограмму (без проблем с кардинальностью).
// Формат: "METHOD /path" (напр. "GET /api/v1/users", "POST /api/v1/orders/{id}").
// Требует WithOtel — без него опция игнорируется.
func WithHTTPRouteMetrics(routes ...string) Option {
	return func(s *Server) {
		s.httpRoutes = append(s.httpRoutes, routes...)
	}
}

// WithGRPCMethodMetrics регистрирует gRPC методы для отдельных per-method метрик.
// Каждый метод получает собственные счётчики и гистограмму.
// Формат: полный путь (напр. "/users.UserService/GetUser").
// Требует WithOtel — без него опция игнорируется.
func WithGRPCMethodMetrics(methods ...string) Option {
	return func(s *Server) {
		s.grpcMethods = append(s.grpcMethods, methods...)
	}
}

func newServer(cfg Config, opts ...Option) *Server {
	s := &Server{cfg: cfg}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
