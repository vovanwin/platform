# logger

Пакет для создания структурированного логгера на базе `slog` с опциональной отправкой логов в Grafana Loki.

## Установка

```go
import "github.com/vovanwin/platform/logger"
```

## Параметры Options

| Поле | Тип | Описание |
|------|-----|----------|
| `Level` | `string` | Уровень логирования: `DEBUG`, `INFO`, `WARN`, `ERROR`. По умолчанию `INFO` |
| `JSON` | `bool` | `true` — JSON вывод (для прода), `false` — текстовый (для локальной разработки) |
| `LokiEnabled` | `bool` | Включить отправку логов в Loki |
| `LokiURL` | `string` | URL Loki push API (напр. `http://localhost:3100/loki/api/v1/push`) |
| `ServiceName` | `string` | Имя сервиса для label `service_name` в Loki |

## Использование

### Базовый (без Loki)

```go
log, closer := logger.NewLogger(logger.Options{
    Level: "DEBUG",
    JSON:  false,
})
defer closer() // no-op когда Loki выключен

log.Info("сервер запущен", slog.String("addr", ":8080"))
```

### С отправкой в Loki

```go
log, closer := logger.NewLogger(logger.Options{
    Level:       "INFO",
    JSON:        true,
    LokiEnabled: true,
    LokiURL:     "http://localhost:3100/loki/api/v1/push",
    ServiceName: "my-service",
})
defer closer() // останавливает Loki client, flush буфера

log.Warn("high latency", slog.Duration("duration", elapsed))
```

При включённом Loki логи пишутся одновременно в stdout и в Loki через `multiHandler`.

### С trace_id в логах

Для связи логов с трейсами в Grafana оберните handler через `otel.NewTraceIDHandler`:

```go
import platformotel "github.com/vovanwin/platform/otel"

log, closer := logger.NewLogger(logger.Options{
    Level: "INFO",
    JSON:  true,
})
defer closer()

// Оборачиваем handler — теперь каждый лог содержит trace_id и span_id
tracedLog := slog.New(platformotel.NewTraceIDHandler(log.Handler()))
slog.SetDefault(tracedLog)

// В обработчиках используем slog.InfoContext(ctx, ...) чтобы trace_id попал в лог
slog.InfoContext(ctx, "order created", slog.Int("order_id", 42))
// Вывод: {"trace_id":"abc123...", "span_id":"def456...", "msg":"order created", ...}
```

## Возвращаемые значения

`NewLogger` возвращает `(*slog.Logger, func())`:
- Логгер автоматически устанавливается как `slog.Default()`
- Вторая функция — closer: при Loki = `client.Stop()`, без Loki = no-op
