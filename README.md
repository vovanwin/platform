# Platform

Go-пакет для быстрого запуска gRPC/HTTP сервера с поддержкой Swagger UI, debug-эндпоинтов и DI через `uber/fx`.

## Установка

```bash
go get github.com/vovanwin/platform
```

## Что внутри

| Пакет | Назначение |
|-------|-----------|
| `server` | 4 сервера в одном: gRPC, HTTP gateway (chi + grpc-gateway), Swagger UI, Debug (pprof + healthz) |
| `logger` | Обёртка над `slog` с настройкой уровня и формата (text/json) |

## Использование

```go
import (
    "github.com/vovanwin/platform/server"
    "go.uber.org/fx"
)

// 1. Предоставить конфиг и логгер через fx.Provide
fx.Provide(
    func() server.Config {
        return server.Config{
            Host: "localhost",
            GRPCPort: "7000", HTTPPort: "7001",
            SwaggerPort: "7002", DebugPort: "7003",
            SwaggerFS: embedSwagger, ProtoFS: embedProto,
        }
    },
)

// 2. Подключить серверный модуль
server.NewModule(
    server.WithHTTPMiddleware(middleware.Recoverer),
    server.WithGRPCOptions(grpc.StatsHandler(otelgrpc.NewServerHandler())),
    server.WithDebugHandler("/metrics", promhttp.Handler()),
)
```

## Опции сервера

| Опция | Описание |
|-------|----------|
| `WithHTTPMiddleware(mw...)` | Middleware для HTTP gateway |
| `WithDebugMiddleware(mw...)` | Middleware для debug-сервера |
| `WithDebugHandler(pattern, handler)` | Кастомный endpoint на debug-сервере (например `/metrics`) |
| `WithGRPCOptions(opts...)` | Опции gRPC сервера (interceptors, stats handlers) |
| `WithGRPCRegistrator(fn)` | Регистрация gRPC сервиса (обычно через fx groups) |
| `WithGatewayRegistrator(fn)` | Регистрация grpc-gateway handler (обычно через fx groups) |

## gRPC/Gateway регистрация через fx groups

Сервисы регистрируются автоматически через fx groups `grpc_registrators` и `gateway_registrators`:

```go
fx.Provide(
    fx.Annotate(
        func(srv *MyGRPCServer) server.GRPCRegistrator {
            return func(s *grpc.Server) { pb.RegisterMyServiceServer(s, srv) }
        },
        fx.ResultTags(`group:"grpc_registrators"`),
    ),
)
```

## Debug-сервер

Встроенные эндпоинты:
- `/debug/pprof/` — профилирование
- `/healthz` — liveness/readiness probe
- Кастомные через `WithDebugHandler`

## Логгер

```go
import "github.com/vovanwin/platform/logger"

log := logger.NewLogger(logger.Options{
    Level: "DEBUG", // DEBUG, INFO, WARN, ERROR
    JSON:  false,   // text или json формат
})
```
