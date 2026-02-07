# otel

Пакет для инициализации OpenTelemetry (трейсы, метрики) и набора HTTP/gRPC middleware.

## Установка

```go
import platformotel "github.com/vovanwin/platform/otel"
```

## Конфигурация

```go
type Config struct {
    ServiceName string  // имя сервиса для атрибутов ресурса
    Endpoint    string  // адрес OTLP коллектора (напр. "localhost:4317")
    SampleRate  float64 // доля трейсов 0.0–1.0, 0 = 100%
}
```

## Компоненты

### Провайдеры (otel.go)

```go
// Инициализация трейсов
tp, err := platformotel.InitTracer(ctx, platformotel.Config{
    ServiceName: "my-service",
    Endpoint:    "localhost:4317",
    SampleRate:  0.1, // 10% трейсов
})

// Инициализация метрик (OTLP + Prometheus)
mp, err := platformotel.InitMeter(ctx, platformotel.Config{
    ServiceName: "my-service",
    Endpoint:    "localhost:4317",
})

// Graceful shutdown
provider := &platformotel.Provider{TracerProvider: tp, MeterProvider: mp}
defer provider.Shutdown(ctx)
```

### HTTP middleware (middleware.go)

```go
// OTEL трейсинг для HTTP — оборачивает каждый запрос в спан
mux.Use(platformotel.HTTPMiddleware("my-service"))
```

### HTTP метрики (metrics_middleware.go)

Общие метрики с labels `method`, `route`, `status_code`:

```go
mux.Use(platformotel.MetricsMiddleware("my-service"))
```

Создаваемые метрики:

| Метрика | Тип | Labels |
|---------|-----|--------|
| `{app}.http.requests.total` | Counter | method, route, status_code |
| `{app}.http.errors.total` | Counter | method, route, status_code |
| `{app}.http.request.duration` | Histogram (s) | method, route |
| `{app}.http.requests.inflight` | UpDownCounter | method, route |

Бакеты гистограммы: `5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s`

### Per-route HTTP метрики (route_metrics.go)

Отдельные инструменты на каждый роут — **без проблем кардинальности**:

```go
rm := platformotel.NewRouteMetrics("my-service", []string{
    "GET /api/v1/users",
    "POST /api/v1/users",
    "GET /api/v1/orders/{id}",
})

// routeFunc извлекает паттерн роута после обработки запроса
mux.Use(rm.Middleware(func(r *http.Request) string {
    return chi.RouteContext(r.Context()).RoutePattern()
}))
```

Для роута `GET /api/v1/users` создаются метрики:
- `my-service.route.get.api.v1.users.requests`
- `my-service.route.get.api.v1.users.errors`
- `my-service.route.get.api.v1.users.duration`

Незарегистрированные роуты попадают в `my-service.route.other.*`.

### Per-method gRPC метрики (grpc_metrics.go)

Отдельные инструменты на каждый gRPC метод:

```go
gm := platformotel.NewGRPCMetrics("my-service", []string{
    "/users.UserService/GetUser",
    "/users.UserService/CreateUser",
})

grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(gm.UnaryInterceptor()),
    grpc.ChainStreamInterceptor(gm.StreamInterceptor()),
)
```

Для метода `/users.UserService/GetUser` создаются:
- `my-service.grpc.users.userservice.getuser.requests`
- `my-service.grpc.users.userservice.getuser.errors` (с label `grpc_code`)
- `my-service.grpc.users.userservice.getuser.duration`

### Panic recovery (recovery_middleware.go)

```go
mux.Use(platformotel.RecoveryMiddleware("my-service"))
```

При панике:
- Записывает stack trace в текущий спан
- Инкрементит `{app}.http.panics.total` (labels: method, route)
- Логирует через `slog.ErrorContext`
- Возвращает HTTP 500

### Trace ID в логах (traceid_handler.go)

```go
log := slog.New(platformotel.NewTraceIDHandler(slog.Default().Handler()))
slog.SetDefault(log)

// Теперь каждый лог с контекстом содержит trace_id и span_id
slog.InfoContext(ctx, "request processed")
```

### Prometheus handler (handler.go)

```go
debugMux.Handle("/metrics", platformotel.MetricsHandler())
```

## Интеграция с server

При использовании через `server.WithOtel()` всё вышеперечисленное настраивается автоматически — см. README пакета `server`.
