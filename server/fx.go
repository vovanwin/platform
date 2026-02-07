package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"go.uber.org/fx"
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
				OnStart: func(_ context.Context) error {
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
					return nil
				},
			})
		}),
	)
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
	if len(s.debugHandlers) > 0 {
		for _, h := range s.debugHandlers {
			fmt.Printf("  │  Custom:   http://%s%s\n", debugAddr, h.Pattern)
		}
	}
	fmt.Println("  └──────────────────────────────────────────────┘")
	fmt.Println()
}
