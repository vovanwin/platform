# server

Пакет объединяет 4 сервера в один fx-модуль: gRPC, HTTP gateway (grpc-gateway), Swagger UI, Debug (pprof + healthz + /metrics).

## Установка

```go
import "github.com/vovanwin/platform/server"
```

## Конфигурация

```go
type Config struct {
    Host        string  // хост для привязки (напр. "0.0.0.0")
    GRPCPort    string  // порт gRPC сервера (напр. "50051")
    HTTPPort    string  // порт HTTP gateway (напр. "8080")
    SwaggerPort string  // порт Swagger UI (напр. "8081")
    DebugPort   string  // порт debug сервера (напр. "6060")
    SwaggerFS   fs.FS   // встроенная FS с файлами *.swagger.json
    ProtoFS     fs.FS   // встроенная FS с файлами *.proto
}
```

## Использование

### Минимальный пример

```go
fx.New(
    fx.Provide(func() server.Config { return cfg }),
    fx.Provide(func() *slog.Logger { return log }),

    server.NewModule(
        server.WithGRPCRegistrator(func(s *grpc.Server) {
            pb.RegisterMyServiceServer(s, myServer)
        }),
        server.WithGatewayRegistrator(func(ctx context.Context, mux *runtime.ServeMux, srv *grpc.Server) error {
            return pb.RegisterMyServiceHandlerServer(ctx, mux, myServer)
        }),
    ),
)
```

### С полным observability (WithOtel)

Одна опция включает трейсы, метрики, recovery, /metrics endpoint:

```go
import platformotel "github.com/vovanwin/platform/otel"

server.NewModule(
    server.WithOtel(platformotel.Config{
        ServiceName: "my-service",
        Endpoint:    "localhost:4317",
        SampleRate:  0.1, // 10% трейсов в проде
    }),
)
```

`WithOtel` автоматически добавляет:

| Что | Где |
|-----|-----|
| `RecoveryMiddleware` | HTTP gateway — ловит паники, пишет в спан |
| `MetricsMiddleware` | HTTP gateway — общие метрики (requests, errors, duration, inflight) |
| `HTTPMiddleware` | HTTP gateway — OTEL трейсинг |
| `otelgrpc.StatsHandler` | gRPC сервер — OTEL трейсинг |
| `/metrics` | Debug сервер — Prometheus endpoint |
| `Provider.Shutdown` | При остановке — graceful shutdown провайдеров |

### Per-route метрики

Отдельные графики для каждой HTTP ручки (без кардинальности labels):

```go
server.NewModule(
    server.WithOtel(platformotel.Config{
        ServiceName: "my-service",
        Endpoint:    "localhost:4317",
    }),
    server.WithHTTPRouteMetrics(
        "GET /api/v1/users",
        "POST /api/v1/users",
        "GET /api/v1/orders/{id}",
    ),
)
```

Каждый роут получает собственные метрики:
- `my-service.route.get.api.v1.users.requests`
- `my-service.route.get.api.v1.users.errors`
- `my-service.route.get.api.v1.users.duration`

### Per-method gRPC метрики

```go
server.NewModule(
    server.WithOtel(platformotel.Config{...}),
    server.WithGRPCMethodMetrics(
        "/users.UserService/GetUser",
        "/users.UserService/CreateUser",
    ),
)
```

## Все опции

| Опция | Описание |
|-------|----------|
| `WithOtel(cfg)` | Включает трейсы, метрики, recovery, /metrics — всё автоматически |
| `WithHTTPRouteMetrics(routes...)` | Per-route HTTP метрики (требует WithOtel) |
| `WithGRPCMethodMetrics(methods...)` | Per-method gRPC метрики (требует WithOtel) |
| `WithGRPCRegistrator(fn)` | Регистрация gRPC сервисов |
| `WithGatewayRegistrator(fn)` | Регистрация grpc-gateway хендлеров |
| `WithHTTPMiddleware(mw...)` | Пользовательские middleware на HTTP gateway |
| `WithDebugMiddleware(mw...)` | Middleware на Debug сервер |
| `WithDebugHandler(pattern, handler)` | Кастомный handler на Debug сервер |
| `WithGRPCOptions(opts...)` | Дополнительные опции gRPC сервера |

## Debug сервер

Всегда доступны:
- `GET /debug/pprof/` — профилирование
- `GET /healthz` — liveness probe

При `WithOtel`:
- `GET /metrics` — Prometheus метрики

## Зависимости (fx.Provide)

Модуль ожидает в DI-контейнере:
- `server.Config` — конфигурация портов
- `*slog.Logger` — логгер

Регистраторы gRPC и gateway собираются через fx groups (`grpc_registrators`, `gateway_registrators`).
