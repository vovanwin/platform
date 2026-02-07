package generator

import "fmt"

func genController(svc service) string {
	return fmt.Sprintf(`package %s

import (
	"log/slog"

	%spb "%s"
	"go.uber.org/fx"
)

// Deps содержит зависимости для %s.
type Deps struct {
	fx.In

	Log *slog.Logger
}

// %s реализует gRPC сервис %s.
type %s struct {
	%spb.Unimplemented%sServer
	log *slog.Logger
}

// New%s создаёт новый %s.
func New%s(deps Deps) *%s {
	return &%s{log: deps.Log}
}
`, svc.DirName,
		svc.PbAlias, svc.GoPackage,
		svc.StructName,
		svc.StructName, svc.Name,
		svc.StructName,
		svc.PbAlias, svc.Name,
		svc.StructName, svc.StructName,
		svc.StructName, svc.StructName,
		svc.StructName)
}

func genModule(svc service, goModule, serverPkg string) string {
	return fmt.Sprintf(`package %s

import (
	"context"

	"%s"
	%spb "%s"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// Module возвращает fx.Option для подключения %s.
func Module() fx.Option {
	return fx.Options(
		fx.Provide(New%s),
		fx.Provide(
			fx.Annotate(
				func(srv *%s) server.GRPCRegistrator {
					return func(s *grpc.Server) {
						%spb.Register%sServer(s, srv)
					}
				},
				fx.ResultTags(`+"`"+`group:"grpc_registrators"`+"`"+`),
			),
		),
		fx.Provide(
			fx.Annotate(
				func(srv *%s) server.GatewayRegistrator {
					return func(ctx context.Context, mux *runtime.ServeMux, _ *grpc.Server) error {
						return %spb.Register%sHandlerServer(ctx, mux, srv)
					}
				},
				fx.ResultTags(`+"`"+`group:"gateway_registrators"`+"`"+`),
			),
		),
	)
}
`, svc.DirName,
		serverPkg,
		svc.PbAlias, svc.GoPackage,
		svc.Name,
		svc.StructName,
		svc.StructName,
		svc.PbAlias, svc.Name,
		svc.StructName,
		svc.PbAlias, svc.Name)
}

func genMethod(svc service, m rpcMethod) string {
	return fmt.Sprintf(`package %s

import (
	"context"

	%spb "%s"
)

func (s *%s) %s(_ context.Context, req *%spb.%s) (*%spb.%s, error) {
	// TODO: implement
	panic("not implemented")
}
`, svc.DirName,
		svc.PbAlias, svc.GoPackage,
		svc.StructName, m.Name, svc.PbAlias, m.Request, svc.PbAlias, m.Response)
}
