package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	platformotel "github.com/vovanwin/platform/otel"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// moduleParams — зависимости серверного модуля, включая группы регистраторов.
type moduleParams struct {
	fx.In

	LC                  fx.Lifecycle
	Cfg                 Config
	Log                 *slog.Logger
	GRPCRegistrators    []GRPCRegistrator    `group:"grpc_registrators"`
	GatewayRegistrators []GatewayRegistrator `group:"gateway_registrators"`
}

// NewModule создаёт fx.Module для серверного пакета.
// gRPC и gateway регистраторы собираются автоматически через fx groups.
// Потребитель должен предоставить server.Config и *slog.Logger через fx.Provide.
func NewModule(opts ...Option) fx.Option {
	return fx.Module("server",
		fx.Invoke(func(p moduleParams) {
			s := newServer(p.Cfg, opts...)

			s.grpcRegistrators = append(s.grpcRegistrators, p.GRPCRegistrators...)
			s.gatewayRegistrators = append(s.gatewayRegistrators, p.GatewayRegistrators...)

			p.LC.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					// Автоматически обнаруживаем gRPC методы до initOtel,
					// чтобы per-method interceptors попали в grpcOptions до создания сервера.
					s.discoverGRPCMethods(p.Log)

					if err := s.initOtel(ctx, p.Log); err != nil {
						return fmt.Errorf("init otel: %w", err)
					}
					if err := s.initGRPC(p.Log); err != nil {
						return err
					}
					if err := s.initHTTP(p.Log); err != nil {
						return err
					}
					s.initSwagger(p.Log)
					s.initDebug(p.Log)
					s.printBanner()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					_ = s.stopHTTP(ctx, p.Log)
					_ = s.stopSwagger(ctx, p.Log)
					_ = s.stopDebug(ctx, p.Log)
					s.stopGRPC(p.Log)
					s.stopOtel(ctx, p.Log)
					return nil
				},
			})
		}),
	)
}

// discoverGRPCMethods автоматически обнаруживает все gRPC методы из зарегистрированных сервисов.
// Создаёт временный gRPC сервер, регистрирует все сервисы, извлекает методы через GetServiceInfo(),
// и добавляет их в s.grpcMethods. Вызывается ДО initOtel, чтобы per-method interceptors
// были включены в grpcOptions до создания настоящего gRPC сервера.
func (s *Server) discoverGRPCMethods(log *slog.Logger) {
	if s.otelCfg == nil || len(s.grpcRegistrators) == 0 {
		return
	}

	// Создаём временный gRPC сервер только для обнаружения методов
	tmpServer := grpc.NewServer()
	for _, reg := range s.grpcRegistrators {
		reg(tmpServer)
	}

	seen := make(map[string]struct{})
	for _, m := range s.grpcMethods {
		seen[m] = struct{}{}
	}

	var discovered int
	for serviceName, info := range tmpServer.GetServiceInfo() {
		for _, method := range info.Methods {
			fullMethod := "/" + serviceName + "/" + method.Name
			if _, ok := seen[fullMethod]; !ok {
				s.grpcMethods = append(s.grpcMethods, fullMethod)
				seen[fullMethod] = struct{}{}
				discovered++
			}
		}
	}

	tmpServer.Stop()

	if discovered > 0 {
		log.Info("gRPC методы обнаружены автоматически",
			slog.Int("discovered", discovered),
			slog.Int("total", len(s.grpcMethods)),
		)
	}
}

// initOtel инициализирует OTEL провайдеры и добавляет middleware, если WithOtel был вызван.
func (s *Server) initOtel(ctx context.Context, log *slog.Logger) error {
	if s.otelCfg == nil {
		return nil
	}

	cfg := *s.otelCfg
	provider := &platformotel.Provider{}

	tp, err := platformotel.InitTracer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	provider.TracerProvider = tp

	mp, err := platformotel.InitMeter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init meter: %w", err)
	}
	provider.MeterProvider = mp

	s.otelProvider = provider

	// Запускаем сбор Go runtime метрик (goroutines, heap, GC)
	if err := platformotel.StartRuntimeMetrics(); err != nil {
		log.Warn("Не удалось запустить runtime метрики", slog.String("error", err.Error()))
	}

	// Добавляем HTTP middleware (в начало цепочки: recovery → metrics → tracing → trace_id header → пользовательские)
	otelMiddleware := []func(http.Handler) http.Handler{
		platformotel.RecoveryMiddleware(cfg.ServiceName),
		platformotel.MetricsMiddleware(cfg.ServiceName),
		platformotel.HTTPMiddleware(cfg.ServiceName),
		platformotel.TraceIDMiddleware(),
	}
	s.httpMiddleware = append(otelMiddleware, s.httpMiddleware...)

	// Per-route HTTP метрики (отдельные инструменты на каждый роут)
	if len(s.httpRoutes) > 0 {
		rm := platformotel.NewRouteMetrics(cfg.ServiceName, s.httpRoutes)
		chiRouteFunc := func(r *http.Request) string {
			rctx := chi.RouteContext(r.Context())
			if rctx != nil {
				return rctx.RoutePattern()
			}
			return r.URL.Path
		}
		s.httpMiddleware = append(s.httpMiddleware, rm.Middleware(chiRouteFunc))
	}

	// Добавляем gRPC stats handler для трейсинга + trace_id в response headers
	s.grpcOptions = append(s.grpcOptions,
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(platformotel.TraceIDUnaryInterceptor()),
	)

	// Per-method gRPC метрики (автоматически обнаруженные + ручные)
	if len(s.grpcMethods) > 0 {
		gm := platformotel.NewGRPCMetrics(cfg.ServiceName, s.grpcMethods)
		s.grpcOptions = append(s.grpcOptions,
			grpc.ChainUnaryInterceptor(gm.UnaryInterceptor()),
			grpc.ChainStreamInterceptor(gm.StreamInterceptor()),
		)
	}

	// Монтируем /metrics на debug-сервер
	s.debugHandlers = append(s.debugHandlers, DebugHandler{
		Pattern: "/metrics",
		Handler: platformotel.MetricsHandler(),
	})

	log.Info("OTEL инициализирован",
		slog.String("service", cfg.ServiceName),
		slog.String("endpoint", cfg.Endpoint),
	)

	return nil
}

func (s *Server) stopOtel(ctx context.Context, log *slog.Logger) {
	if s.otelProvider != nil {
		log.Info("OTEL провайдеры завершают работу...")
		if err := s.otelProvider.Shutdown(ctx); err != nil {
			log.Error("Ошибка при завершении OTEL", slog.String("error", err.Error()))
		}
	}
}

func (s *Server) printBanner() {
	host := s.cfg.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}

	httpAddr := net.JoinHostPort(host, s.cfg.HTTPPort)
	grpcAddr := net.JoinHostPort(host, s.cfg.GRPCPort)
	swaggerAddr := net.JoinHostPort(host, s.cfg.SwaggerPort)
	debugAddr := net.JoinHostPort(host, s.cfg.DebugPort)

	fmt.Println()
	fmt.Println("  ┌──────────────────────────────────────────────┐")
	fmt.Println("  │              Сервер запущен                   │")
	fmt.Println("  ├──────────────────────────────────────────────┤")
	fmt.Printf("  │  HTTP:     http://%s\n", httpAddr)
	fmt.Printf("  │  gRPC:     %s\n", grpcAddr)
	fmt.Printf("  │  Swagger:  http://%s\n", swaggerAddr)
	fmt.Printf("  │  Debug:    http://%s/debug/pprof/\n", debugAddr)
	fmt.Printf("  │  Health:   http://%s/healthz\n", debugAddr)
	if s.otelCfg != nil {
		fmt.Printf("  │  Metrics:  http://%s/metrics\n", debugAddr)
	}
	if len(s.debugHandlers) > 0 {
		for _, h := range s.debugHandlers {
			if h.Pattern == "/metrics" && s.otelCfg != nil {
				continue // уже вывели выше
			}
			fmt.Printf("  │  Custom:   http://%s%s\n", debugAddr, h.Pattern)
		}
	}
	fmt.Println("  └──────────────────────────────────────────────┘")
	fmt.Println()
}
