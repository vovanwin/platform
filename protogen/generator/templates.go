package generator

import (
	"fmt"
	"strings"
)

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

// formatType formats a proto type for Go code generation.
// Simple types like "LoginRequest" become "gatepb.LoginRequest".
// Fully-qualified types like "google.protobuf.Empty" stay as-is
// but the last segment is used: "emptypb.Empty".
func formatType(pbAlias, typeName string) (goType string, extraImport string) {
	if !strings.Contains(typeName, ".") {
		return fmt.Sprintf("%spb.%s", pbAlias, typeName), ""
	}
	// google.protobuf.Empty -> emptypb.Empty
	// google.protobuf.Timestamp -> timestamppb.Timestamp
	parts := strings.Split(typeName, ".")
	name := parts[len(parts)-1]                                   // "Empty"
	pkg := strings.ToLower(name) + "pb"                           // "emptypb"
	importPath := "google.golang.org/protobuf/types/known/" + pkg // "google.golang.org/protobuf/types/known/emptypb"
	return fmt.Sprintf("%s.%s", pkg, name), importPath
}

func genMethod(svc service, m rpcMethod) string {
	reqType, reqImport := formatType(svc.PbAlias, m.Request)
	respType, respImport := formatType(svc.PbAlias, m.Response)

	// Collect unique imports
	imports := []string{`"context"`}
	pbImport := fmt.Sprintf("\t%spb \"%s\"", svc.PbAlias, svc.GoPackage)

	needPb := !strings.Contains(m.Request, ".") || !strings.Contains(m.Response, ".")
	if needPb {
		imports = append(imports, pbImport)
	}

	seen := make(map[string]bool)
	for _, imp := range []string{reqImport, respImport} {
		if imp != "" && !seen[imp] {
			seen[imp] = true
			imports = append(imports, fmt.Sprintf("\t\"%s\"", imp))
		}
	}

	importBlock := strings.Join(imports, "\n")

	return fmt.Sprintf(`package %s

import (
%s
)

func (s *%s) %s(_ context.Context, req *%s) (*%s, error) {
	// TODO: implement
	panic("not implemented")
}
`, svc.DirName,
		importBlock,
		svc.StructName, m.Name, reqType, respType)
}
