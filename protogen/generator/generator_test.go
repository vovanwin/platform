package generator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// --- CamelToSnake ---

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"GetUser", "get_user"},
		{"RefreshToken", "refresh_token"},
		{"OAuthURL", "o_auth_u_r_l"},
		{"Get", "get"},
		{"getUser", "get_user"},
		{"A", "a"},
		{"", ""},
		{"HTMLParser", "h_t_m_l_parser"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CamelToSnake(tt.input)
			if got != tt.want {
				t.Errorf("CamelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- httpPathToFileName ---

func TestHttpPathToFileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/api/v1/auth/register", "api_v1_auth_register"},
		{"/api/v1/auth/refresh", "api_v1_auth_refresh"},
		{"/api/v1/oauth/{provider}/url", "api_v1_oauth_url"},
		{"/api/v1/account/link/{provider}", "api_v1_account_link"},
		{"/api/v1/sessions/{session_id}", "api_v1_sessions"},
		{"/api/v1/health", "api_v1_health"},
		{"/users", "users"},
		{"/{id}", ""},
		{"/a/{b}/{c}/d", "a_d"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := httpPathToFileName(tt.input)
			if got != tt.want {
				t.Errorf("httpPathToFileName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- formatType ---

func TestFormatType(t *testing.T) {
	tests := []struct {
		name       string
		pbAlias    string
		typeName   string
		wantType   string
		wantImport string
	}{
		{
			name:       "simple type",
			pbAlias:    "gate",
			typeName:   "LoginRequest",
			wantType:   "gatepb.LoginRequest",
			wantImport: "",
		},
		{
			name:       "google.protobuf.Empty",
			pbAlias:    "gate",
			typeName:   "google.protobuf.Empty",
			wantType:   "emptypb.Empty",
			wantImport: "google.golang.org/protobuf/types/known/emptypb",
		},
		{
			name:       "google.protobuf.Timestamp",
			pbAlias:    "gate",
			typeName:   "google.protobuf.Timestamp",
			wantType:   "timestamppb.Timestamp",
			wantImport: "google.golang.org/protobuf/types/known/timestamppb",
		},
		{
			name:       "different alias",
			pbAlias:    "users",
			typeName:   "GetUserRequest",
			wantType:   "userspb.GetUserRequest",
			wantImport: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, extraImport := formatType(tt.pbAlias, tt.typeName)
			if goType != tt.wantType {
				t.Errorf("formatType type = %q, want %q", goType, tt.wantType)
			}
			if extraImport != tt.wantImport {
				t.Errorf("formatType import = %q, want %q", extraImport, tt.wantImport)
			}
		})
	}
}

// --- parseGoPackage ---

func TestParseGoPackage(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantPkg   string
		wantAlias string
	}{
		{
			name:      "with semicolon alias",
			content:   `option go_package = "github.com/vovanwin/gate/pkg/gate;gate";`,
			wantPkg:   "github.com/vovanwin/gate/pkg/gate",
			wantAlias: "gate",
		},
		{
			name:      "without semicolon",
			content:   `option go_package = "github.com/vovanwin/gate/pkg/gate";`,
			wantPkg:   "github.com/vovanwin/gate/pkg/gate",
			wantAlias: "gate",
		},
		{
			name:      "no go_package",
			content:   `syntax = "proto3";`,
			wantPkg:   "",
			wantAlias: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, alias := parseGoPackage(tt.content)
			if pkg != tt.wantPkg {
				t.Errorf("package = %q, want %q", pkg, tt.wantPkg)
			}
			if alias != tt.wantAlias {
				t.Errorf("alias = %q, want %q", alias, tt.wantAlias)
			}
		})
	}
}

// --- parseProto ---

func TestParseProto_WithHTTPAnnotations(t *testing.T) {
	content := `syntax = "proto3";

option go_package = "github.com/example/pkg/auth;auth";

service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse) {
    option (google.api.http) = {
      post: "/api/v1/auth/login"
      body: "*"
    };
  }

  rpc GetProfile(GetProfileRequest) returns (ProfileResponse) {
    option (google.api.http) = {
      get: "/api/v1/profile"
    };
  }
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	services, err := parseProto(protoPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	svc := services[0]
	if svc.Name != "AuthService" {
		t.Errorf("service name = %q, want %q", svc.Name, "AuthService")
	}
	if svc.StructName != "AuthGRPCServer" {
		t.Errorf("struct name = %q, want %q", svc.StructName, "AuthGRPCServer")
	}
	if svc.DirName != "auth" {
		t.Errorf("dir name = %q, want %q", svc.DirName, "auth")
	}

	if len(svc.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(svc.Methods))
	}

	login := svc.Methods[0]
	if login.Name != "Login" || login.Request != "LoginRequest" || login.Response != "LoginResponse" {
		t.Errorf("Login method = %+v", login)
	}
	if login.HTTPPath != "/api/v1/auth/login" {
		t.Errorf("Login HTTPPath = %q, want %q", login.HTTPPath, "/api/v1/auth/login")
	}
	if login.HTTPMethod != "post" {
		t.Errorf("Login HTTPMethod = %q, want %q", login.HTTPMethod, "post")
	}

	profile := svc.Methods[1]
	if profile.Name != "GetProfile" || profile.HTTPPath != "/api/v1/profile" {
		t.Errorf("GetProfile method = %+v", profile)
	}
	if profile.HTTPMethod != "get" {
		t.Errorf("GetProfile HTTPMethod = %q, want %q", profile.HTTPMethod, "get")
	}
}

func TestParseProto_WithoutHTTPAnnotations(t *testing.T) {
	content := `syntax = "proto3";

option go_package = "github.com/example/pkg/users;users";

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	services, err := parseProto(protoPath)
	if err != nil {
		t.Fatal(err)
	}

	svc := services[0]
	if len(svc.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(svc.Methods))
	}

	for _, m := range svc.Methods {
		if m.HTTPPath != "" {
			t.Errorf("method %s should have empty HTTPPath, got %q", m.Name, m.HTTPPath)
		}
		if m.HTTPMethod != "" {
			t.Errorf("method %s should have empty HTTPMethod, got %q", m.Name, m.HTTPMethod)
		}
	}
}

func TestParseProto_MixedAnnotations(t *testing.T) {
	content := `syntax = "proto3";

option go_package = "github.com/example/pkg/mixed;mixed";

service MixedService {
  rpc WithHTTP(Req) returns (Resp) {
    option (google.api.http) = {
      get: "/api/v1/with-http"
    };
  }

  rpc WithoutHTTP(Req) returns (Resp);
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	services, err := parseProto(protoPath)
	if err != nil {
		t.Fatal(err)
	}

	svc := services[0]
	if len(svc.Methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(svc.Methods))
	}

	methodByName := map[string]rpcMethod{}
	for _, m := range svc.Methods {
		methodByName[m.Name] = m
	}

	if methodByName["WithHTTP"].HTTPPath != "/api/v1/with-http" {
		t.Errorf("WithHTTP.HTTPPath = %q, want %q", methodByName["WithHTTP"].HTTPPath, "/api/v1/with-http")
	}
	if methodByName["WithHTTP"].HTTPMethod != "get" {
		t.Errorf("WithHTTP.HTTPMethod = %q, want %q", methodByName["WithHTTP"].HTTPMethod, "get")
	}
	if methodByName["WithoutHTTP"].HTTPPath != "" {
		t.Errorf("WithoutHTTP.HTTPPath = %q, want empty", methodByName["WithoutHTTP"].HTTPPath)
	}
}

func TestParseProto_NoGoPackage(t *testing.T) {
	content := `syntax = "proto3";

service NoPackageService {
  rpc Ping(PingReq) returns (PingResp);
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := parseProto(protoPath)
	if err == nil {
		t.Fatal("expected error for missing go_package, got nil")
	}
}

func TestParseProto_AllHTTPMethods(t *testing.T) {
	content := `syntax = "proto3";

option go_package = "github.com/example/pkg/crud;crud";

service CrudService {
  rpc Create(Req) returns (Resp) {
    option (google.api.http) = {
      post: "/api/v1/items"
      body: "*"
    };
  }
  rpc Get(Req) returns (Resp) {
    option (google.api.http) = {
      get: "/api/v1/items/{id}"
    };
  }
  rpc Update(Req) returns (Resp) {
    option (google.api.http) = {
      put: "/api/v1/items/{id}"
      body: "*"
    };
  }
  rpc Patch(Req) returns (Resp) {
    option (google.api.http) = {
      patch: "/api/v1/items/{id}"
      body: "*"
    };
  }
  rpc Delete(Req) returns (Resp) {
    option (google.api.http) = {
      delete: "/api/v1/items/{id}"
    };
  }
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	services, err := parseProto(protoPath)
	if err != nil {
		t.Fatal(err)
	}

	svc := services[0]
	if len(svc.Methods) != 5 {
		t.Fatalf("expected 5 methods, got %d", len(svc.Methods))
	}

	want := map[string]struct{ path, method string }{
		"Create": {"/api/v1/items", "post"},
		"Get":    {"/api/v1/items/{id}", "get"},
		"Update": {"/api/v1/items/{id}", "put"},
		"Patch":  {"/api/v1/items/{id}", "patch"},
		"Delete": {"/api/v1/items/{id}", "delete"},
	}

	for _, m := range svc.Methods {
		w, ok := want[m.Name]
		if !ok {
			t.Errorf("unexpected method %q", m.Name)
			continue
		}
		if m.HTTPPath != w.path {
			t.Errorf("%s.HTTPPath = %q, want %q", m.Name, m.HTTPPath, w.path)
		}
		if m.HTTPMethod != w.method {
			t.Errorf("%s.HTTPMethod = %q, want %q", m.Name, m.HTTPMethod, w.method)
		}
	}
}

func TestParseProto_FullQualifiedTypes(t *testing.T) {
	content := `syntax = "proto3";

option go_package = "github.com/example/pkg/svc;svc";

service SvcService {
  rpc Ping(google.protobuf.Empty) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      get: "/api/v1/ping"
    };
  }
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	services, err := parseProto(protoPath)
	if err != nil {
		t.Fatal(err)
	}

	m := services[0].Methods[0]
	if m.Request != "google.protobuf.Empty" {
		t.Errorf("request = %q, want %q", m.Request, "google.protobuf.Empty")
	}
	if m.Response != "google.protobuf.Empty" {
		t.Errorf("response = %q, want %q", m.Response, "google.protobuf.Empty")
	}
}

func TestParseProto_MultipleServices(t *testing.T) {
	content := `syntax = "proto3";

option go_package = "github.com/example/pkg/multi;multi";

service AlphaService {
  rpc Ping(Req) returns (Resp) {
    option (google.api.http) = {
      get: "/api/v1/alpha/ping"
    };
  }
}

service BetaService {
  rpc Pong(Req) returns (Resp) {
    option (google.api.http) = {
      get: "/api/v1/beta/pong"
    };
  }
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	services, err := parseProto(protoPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	if services[0].Name != "AlphaService" {
		t.Errorf("first service = %q, want AlphaService", services[0].Name)
	}
	if services[0].DirName != "alpha" {
		t.Errorf("first dirName = %q, want alpha", services[0].DirName)
	}
	if services[1].Name != "BetaService" {
		t.Errorf("second service = %q, want BetaService", services[1].Name)
	}
}

// --- parseGoModule ---

func TestParseGoModule(t *testing.T) {
	content := `module github.com/vovanwin/myproject

go 1.23
`
	dir := t.TempDir()
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := parseGoModule(goModPath)
	want := "github.com/vovanwin/myproject"
	if got != want {
		t.Errorf("parseGoModule() = %q, want %q", got, want)
	}
}

func TestParseGoModule_NotFound(t *testing.T) {
	got := parseGoModule("/nonexistent/go.mod")
	if got != "" {
		t.Errorf("parseGoModule() = %q, want empty", got)
	}
}

// --- genController ---

func TestGenController(t *testing.T) {
	svc := service{
		Name:       "UserService",
		GoPackage:  "github.com/example/pkg/users",
		PbAlias:    "users",
		StructName: "UserGRPCServer",
		DirName:    "user",
	}

	got := genController(svc)

	mustContain := []string{
		"package user",
		`userspb "github.com/example/pkg/users"`,
		`"log/slog"`,
		`"go.uber.org/fx"`,
		"type Deps struct {",
		"fx.In",
		"Log *slog.Logger",
		"type UserGRPCServer struct {",
		"userspb.UnimplementedUserServiceServer",
		"log *slog.Logger",
		"func NewUserGRPCServer(deps Deps) *UserGRPCServer {",
		"return &UserGRPCServer{log: deps.Log}",
	}

	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("genController missing %q\n\ngot:\n%s", s, got)
		}
	}
}

func TestGenController_DifferentService(t *testing.T) {
	svc := service{
		Name:       "OrderService",
		GoPackage:  "github.com/shop/pkg/orders",
		PbAlias:    "orders",
		StructName: "OrderGRPCServer",
		DirName:    "order",
	}

	got := genController(svc)

	mustContain := []string{
		"package order",
		`orderspb "github.com/shop/pkg/orders"`,
		"type OrderGRPCServer struct {",
		"orderspb.UnimplementedOrderServiceServer",
		"func NewOrderGRPCServer(deps Deps) *OrderGRPCServer {",
	}

	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("genController missing %q\n\ngot:\n%s", s, got)
		}
	}
}

// --- genModule ---

func TestGenModule(t *testing.T) {
	svc := service{
		Name:       "UserService",
		GoPackage:  "github.com/example/pkg/users",
		PbAlias:    "users",
		StructName: "UserGRPCServer",
		DirName:    "user",
	}
	serverPkg := "github.com/vovanwin/platform/server"

	got := genModule(svc, "github.com/example", serverPkg)

	mustContain := []string{
		"package user",
		`"context"`,
		`"github.com/vovanwin/platform/server"`,
		`userspb "github.com/example/pkg/users"`,
		"func Module() fx.Option {",
		"fx.Provide(NewUserGRPCServer)",
		"userspb.RegisterUserServiceServer(s, srv)",
		"userspb.RegisterUserServiceHandlerServer(ctx, mux, srv)",
	}

	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("genModule missing %q\n\ngot:\n%s", s, got)
		}
	}
}

func TestGenModule_CustomServerPkg(t *testing.T) {
	svc := service{
		Name:       "GateService",
		GoPackage:  "github.com/example/pkg/gate",
		PbAlias:    "gate",
		StructName: "GateGRPCServer",
		DirName:    "gate",
	}

	got := genModule(svc, "github.com/example", "github.com/custom/server")

	if !strings.Contains(got, `"github.com/custom/server"`) {
		t.Errorf("genModule should use custom server pkg\n\ngot:\n%s", got)
	}
}

// --- genMethod ---

func TestGenMethod_SimpleTypes(t *testing.T) {
	svc := service{
		Name:       "UserService",
		GoPackage:  "github.com/example/pkg/users",
		PbAlias:    "users",
		StructName: "UserGRPCServer",
		DirName:    "user",
	}
	m := rpcMethod{
		Name:       "GetUser",
		Request:    "GetUserRequest",
		Response:   "GetUserResponse",
		HTTPMethod: "get",
		HTTPPath:   "/api/v1/users/{id}",
	}

	got := genMethod(svc, m)

	mustContain := []string{
		"package user",
		`"context"`,
		`userspb "github.com/example/pkg/users"`,
		"func (s *UserGRPCServer) GetUser(_ context.Context, req *userspb.GetUserRequest) (*userspb.GetUserResponse, error) {",
		"// TODO: implement",
		`panic("not implemented")`,
	}

	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("genMethod missing %q\n\ngot:\n%s", s, got)
		}
	}
}

func TestGenMethod_FullyQualifiedEmpty(t *testing.T) {
	svc := service{
		Name:       "GateService",
		GoPackage:  "github.com/vovanwin/gate/pkg/gate",
		PbAlias:    "gate",
		StructName: "GateGRPCServer",
		DirName:    "gate",
	}
	m := rpcMethod{
		Name:       "GetLinkedAccounts",
		Request:    "google.protobuf.Empty",
		Response:   "LinkedAccountsResponse",
		HTTPMethod: "get",
		HTTPPath:   "/api/v1/account/links",
	}

	got := genMethod(svc, m)

	// Must use emptypb.Empty, NOT gatepb.google.protobuf.Empty
	mustContain := []string{
		"emptypb.Empty",
		`"google.golang.org/protobuf/types/known/emptypb"`,
		"*gatepb.LinkedAccountsResponse",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("genMethod missing %q\n\ngot:\n%s", s, got)
		}
	}

	// Must NOT contain the broken form
	mustNotContain := []string{
		"gatepb.google.protobuf.Empty",
	}
	for _, s := range mustNotContain {
		if strings.Contains(got, s) {
			t.Errorf("genMethod should NOT contain %q\n\ngot:\n%s", s, got)
		}
	}
}

func TestGenMethod_BothFullyQualified(t *testing.T) {
	svc := service{
		Name:       "GateService",
		GoPackage:  "github.com/vovanwin/gate/pkg/gate",
		PbAlias:    "gate",
		StructName: "GateGRPCServer",
		DirName:    "gate",
	}
	m := rpcMethod{
		Name:       "Logout",
		Request:    "google.protobuf.Empty",
		Response:   "google.protobuf.Empty",
		HTTPMethod: "post",
		HTTPPath:   "/api/v1/auth/logout",
	}

	got := genMethod(svc, m)

	// Both req and resp are Empty — should import emptypb once, no gatepb
	if !strings.Contains(got, "emptypb.Empty") {
		t.Errorf("should use emptypb.Empty\n\ngot:\n%s", got)
	}
	if strings.Contains(got, "gatepb") {
		t.Errorf("should NOT import gatepb when both types are fully-qualified\n\ngot:\n%s", got)
	}
}

func TestGenMethod_ResponseFullyQualified(t *testing.T) {
	svc := service{
		Name:       "GateService",
		GoPackage:  "github.com/vovanwin/gate/pkg/gate",
		PbAlias:    "gate",
		StructName: "GateGRPCServer",
		DirName:    "gate",
	}
	m := rpcMethod{
		Name:       "RevokeSession",
		Request:    "RevokeSessionRequest",
		Response:   "google.protobuf.Empty",
		HTTPMethod: "delete",
		HTTPPath:   "/api/v1/sessions/{session_id}",
	}

	got := genMethod(svc, m)

	mustContain := []string{
		"*gatepb.RevokeSessionRequest",
		"*emptypb.Empty",
		`"google.golang.org/protobuf/types/known/emptypb"`,
		`gatepb "github.com/vovanwin/gate/pkg/gate"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("genMethod missing %q\n\ngot:\n%s", s, got)
		}
	}
}

// --- Run (integration) ---

func setupTestProject(t *testing.T, proto string) (root, outputDir string) {
	t.Helper()
	root = t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	apiDir := filepath.Join(root, "api", "svc")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(apiDir, "svc.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir = filepath.Join(root, "internal", "controller")
	return root, outputDir
}

func TestRun_Integration(t *testing.T) {
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/gate;gate";

service GateService {
  rpc Login(LoginRequest) returns (LoginResponse) {
    option (google.api.http) = {
      post: "/api/v1/auth/login"
      body: "*"
    };
  }

  rpc GetHealth(HealthRequest) returns (HealthResponse) {
    option (google.api.http) = {
      get: "/api/v1/health"
    };
  }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiDir := filepath.Join(root, "api", "gate")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "gate.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "internal", "controller")

	err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server")
	if err != nil {
		t.Fatal(err)
	}

	controllerDir := filepath.Join(outputDir, "gate")

	// file names now include HTTP method suffix
	expectedFiles := []string{
		"0_controller.go",
		"0_module.go",
		"api_v1_auth_login_post.go",
		"api_v1_health_get.go",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(controllerDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// old-style names must NOT exist
	oldNames := []string{"controller.go", "module.go", "login.go", "get_health.go", "api_v1_auth_login.go", "api_v1_health.go"}
	for _, f := range oldNames {
		path := filepath.Join(controllerDir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file %s should not exist (old naming)", f)
		}
	}
}

func TestRun_GeneratedMethodContent(t *testing.T) {
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/order;order";

service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse) {
    option (google.api.http) = {
      post: "/api/v1/orders"
      body: "*"
    };
  }
}
`
	root, outputDir := setupTestProject(t, proto)

	apiDir := filepath.Join(root, "api", "order")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "order.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	controllerDir := filepath.Join(outputDir, "order")

	// check method file with _post suffix
	data, err := os.ReadFile(filepath.Join(controllerDir, "api_v1_orders_post.go"))
	if err != nil {
		t.Fatal(err)
	}
	method := string(data)

	methodExpected := []string{
		"package order",
		`orderpb "github.com/example/test/pkg/order"`,
		"func (s *OrderGRPCServer) CreateOrder(_ context.Context, req *orderpb.CreateOrderRequest) (*orderpb.CreateOrderResponse, error) {",
		`panic("not implemented")`,
	}
	for _, s := range methodExpected {
		if !strings.Contains(method, s) {
			t.Errorf("api_v1_orders_post.go missing %q\n\ngot:\n%s", s, method)
		}
	}
}

func TestRun_FallbackToSnakeCase(t *testing.T) {
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/simple;simple";

service SimpleService {
  rpc DoSomething(Req) returns (Resp);
}
`
	root, outputDir := setupTestProject(t, proto)

	apiDir := filepath.Join(root, "api", "simple")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "simple.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	// no HTTP annotation -> snake_case without method suffix
	path := filepath.Join(outputDir, "simple", "do_something.go")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file do_something.go to exist (snake_case fallback)")
	}
}

func TestRun_DoesNotOverwrite(t *testing.T) {
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/svc;svc";

service SvcService {
  rpc Ping(Req) returns (Resp) {
    option (google.api.http) = {
      get: "/api/v1/ping"
    };
  }
}
`
	root, outputDir := setupTestProject(t, proto)

	controllerDir := filepath.Join(outputDir, "svc")
	if err := os.MkdirAll(controllerDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"0_controller.go":    "// custom controller\n",
		"0_module.go":        "// custom module\n",
		"api_v1_ping_get.go": "// custom method\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(controllerDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	for name, wantContent := range files {
		data, err := os.ReadFile(filepath.Join(controllerDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != wantContent {
			t.Errorf("%s was overwritten: got %q, want %q", name, string(data), wantContent)
		}
	}
}

func TestRun_NoProtoFiles(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "internal", "controller")

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}
}

func TestRun_NoGoMod(t *testing.T) {
	root := t.TempDir()

	err := Run(root, "api", "output", "")
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("error should mention go.mod, got: %v", err)
	}
}

func TestRun_FileSortOrder(t *testing.T) {
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/order;order";

service OrderService {
  rpc Create(Req) returns (Resp) {
    option (google.api.http) = {
      post: "/api/v1/orders"
      body: "*"
    };
  }
  rpc Delete(Req) returns (Resp) {
    option (google.api.http) = {
      delete: "/api/v1/orders/{id}"
    };
  }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiDir := filepath.Join(root, "api", "order")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "order.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "internal", "controller")

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(outputDir, "order"))
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	if len(names) < 2 {
		t.Fatalf("expected at least 2 files, got %d", len(names))
	}
	if names[0] != "0_controller.go" {
		t.Errorf("first file = %q, want %q", names[0], "0_controller.go")
	}
	if names[1] != "0_module.go" {
		t.Errorf("second file = %q, want %q", names[1], "0_module.go")
	}
}

func TestRun_DuplicatePathsDifferentMethods(t *testing.T) {
	// Same path, different HTTP methods -> different files
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/profile;profile";

service ProfileService {
  rpc GetProfile(Req) returns (Resp) {
    option (google.api.http) = {
      get: "/api/v1/profile"
    };
  }

  rpc UpdateProfile(Req) returns (Resp) {
    option (google.api.http) = {
      patch: "/api/v1/profile"
      body: "*"
    };
  }

  rpc DeleteProfile(Req) returns (Resp) {
    option (google.api.http) = {
      delete: "/api/v1/profile"
    };
  }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiDir := filepath.Join(root, "api", "profile")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "profile.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "internal", "controller")

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	profileDir := filepath.Join(outputDir, "profile")

	// Each method gets its own file thanks to HTTP method suffix
	wantFiles := map[string]string{
		"api_v1_profile_get.go":    "GetProfile",
		"api_v1_profile_patch.go":  "UpdateProfile",
		"api_v1_profile_delete.go": "DeleteProfile",
	}

	for fileName, methodName := range wantFiles {
		data, err := os.ReadFile(filepath.Join(profileDir, fileName))
		if err != nil {
			t.Errorf("expected file %s to exist", fileName)
			continue
		}
		if !strings.Contains(string(data), methodName) {
			t.Errorf("%s should contain method %s\n\ngot:\n%s", fileName, methodName, string(data))
		}
	}
}

func TestRun_PathParamsWithMethodSuffix(t *testing.T) {
	// Same path with param, different HTTP methods
	proto := `syntax = "proto3";

option go_package = "github.com/example/test/pkg/link;link";

service LinkService {
  rpc LinkOAuth(Req) returns (Resp) {
    option (google.api.http) = {
      post: "/api/v1/account/link/{provider}"
      body: "*"
    };
  }

  rpc UnlinkOAuth(Req) returns (Resp) {
    option (google.api.http) = {
      delete: "/api/v1/account/link/{provider}"
    };
  }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiDir := filepath.Join(root, "api", "link")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "link.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "internal", "controller")

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(outputDir, "link")

	// POST and DELETE to same path -> different files
	if _, err := os.Stat(filepath.Join(linkDir, "api_v1_account_link_post.go")); os.IsNotExist(err) {
		t.Error("expected api_v1_account_link_post.go")
	}
	if _, err := os.Stat(filepath.Join(linkDir, "api_v1_account_link_delete.go")); os.IsNotExist(err) {
		t.Error("expected api_v1_account_link_delete.go")
	}
}

func TestRun_GateProtoRealWorld(t *testing.T) {
	proto := `syntax = "proto3";

package gate.v1;

option go_package = "github.com/vovanwin/gate/pkg/gate;gate";

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

service GateService {
  rpc Register(RegisterRequest) returns (AuthResponse) {
    option (google.api.http) = {
      post: "/api/v1/auth/register"
      body: "*"
    };
  }

  rpc Login(LoginRequest) returns (AuthResponse) {
    option (google.api.http) = {
      post: "/api/v1/auth/login"
      body: "*"
    };
  }

  rpc RefreshToken(RefreshTokenRequest) returns (AuthResponse) {
    option (google.api.http) = {
      post: "/api/v1/auth/refresh"
      body: "*"
    };
  }

  rpc OAuthURL(OAuthURLRequest) returns (OAuthURLResponse) {
    option (google.api.http) = {
      get: "/api/v1/oauth/{provider}/url"
    };
  }

  rpc GetProfile(google.protobuf.Empty) returns (ProfileResponse) {
    option (google.api.http) = {
      get: "/api/v1/profile"
    };
  }

  rpc UpdateProfile(UpdateProfileRequest) returns (ProfileResponse) {
    option (google.api.http) = {
      patch: "/api/v1/profile"
      body: "*"
    };
  }

  rpc RevokeSession(RevokeSessionRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      delete: "/api/v1/sessions/{session_id}"
    };
  }
}

message RegisterRequest { string email = 1; }
message LoginRequest { string email = 1; }
message RefreshTokenRequest { string refresh_token = 1; }
message AuthResponse { string access_token = 1; }
message OAuthURLRequest { string provider = 1; }
message OAuthURLResponse { string url = 1; }
message ProfileResponse { string name = 1; }
message UpdateProfileRequest { string name = 1; }
message RevokeSessionRequest { string session_id = 1; }
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/vovanwin/gate\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiDir := filepath.Join(root, "api", "gate")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "gate.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(root, "internal", "controller")

	if err := Run(root, "api", outputDir, "github.com/vovanwin/platform/server"); err != nil {
		t.Fatal(err)
	}

	gateDir := filepath.Join(outputDir, "gate")

	wantFiles := map[string]string{
		"0_controller.go":              "GateGRPCServer",
		"0_module.go":                  "Module()",
		"api_v1_auth_register_post.go": "Register",
		"api_v1_auth_login_post.go":    "Login",
		"api_v1_auth_refresh_post.go":  "RefreshToken",
		"api_v1_oauth_url_get.go":      "OAuthURL",
		"api_v1_profile_get.go":        "GetProfile",
		"api_v1_profile_patch.go":      "UpdateProfile",
		"api_v1_sessions_delete.go":    "RevokeSession",
	}

	entries, err := os.ReadDir(gateDir)
	if err != nil {
		t.Fatal(err)
	}

	gotFiles := make(map[string]bool)
	for _, e := range entries {
		gotFiles[e.Name()] = true
	}

	for fileName, mustContain := range wantFiles {
		if !gotFiles[fileName] {
			t.Errorf("missing file: %s (got files: %v)", fileName, gotFiles)
			continue
		}

		data, err := os.ReadFile(filepath.Join(gateDir, fileName))
		if err != nil {
			t.Errorf("reading %s: %v", fileName, err)
			continue
		}

		if !strings.Contains(string(data), mustContain) {
			t.Errorf("%s should contain %q\n\ngot:\n%s", fileName, mustContain, string(data))
		}
	}

	// GetProfile and UpdateProfile now have separate files (_get vs _patch)
	// Verify GetProfile uses emptypb.Empty, not gatepb.google.protobuf.Empty
	profileData, err := os.ReadFile(filepath.Join(gateDir, "api_v1_profile_get.go"))
	if err != nil {
		t.Fatal(err)
	}
	profileStr := string(profileData)
	if strings.Contains(profileStr, "gatepb.google") {
		t.Errorf("api_v1_profile_get.go has broken type gatepb.google...\n\ngot:\n%s", profileStr)
	}
	if !strings.Contains(profileStr, "emptypb.Empty") {
		t.Errorf("api_v1_profile_get.go should use emptypb.Empty\n\ngot:\n%s", profileStr)
	}

	// Verify RevokeSession response uses emptypb.Empty
	revokeData, err := os.ReadFile(filepath.Join(gateDir, "api_v1_sessions_delete.go"))
	if err != nil {
		t.Fatal(err)
	}
	revokeStr := string(revokeData)
	if !strings.Contains(revokeStr, "*emptypb.Empty") {
		t.Errorf("api_v1_sessions_delete.go should use *emptypb.Empty\n\ngot:\n%s", revokeStr)
	}
	if !strings.Contains(revokeStr, "*gatepb.RevokeSessionRequest") {
		t.Errorf("api_v1_sessions_delete.go should use *gatepb.RevokeSessionRequest\n\ngot:\n%s", revokeStr)
	}
}
