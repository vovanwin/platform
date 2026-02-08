# Roadmap платформы

## Observability

### Метрики
- [x] **Runtime метрики Go** — goroutines, heap alloc, GC pauses через `go.opentelemetry.io/contrib/instrumentation/runtime`. Автоматический запуск при `WithOtel`.
- [x] **Per-route HTTP метрики** — `WithHTTPRouteMetrics` для отдельных счётчиков/гистограмм на каждый роут.
- [x] **Per-method gRPC метрики** — `WithGRPCMethodMetrics` + автоматическое обнаружение через `GetServiceInfo()`.
- [x] **Глобальные HTTP метрики** — requests.total, errors.total, request.duration, requests.inflight через `MetricsMiddleware`.
- [x] **Prometheus /metrics endpoint** — автоматически при `WithOtel` на debug-сервере.
- **HTTP response size** — `Float64Histogram` по байтам тела ответа, помогает ловить аномальные payload.
- **HTTP request size** — `Float64Histogram` по `Content-Length` входящего запроса.
- **gRPC message size** — размер входящих/исходящих protobuf сообщений.
- **Database метрики** — middleware/wrapper для SQL: query duration, error rate, connection pool stats (open/idle/waiting).
- **HTTP client метрики** — `http.RoundTripper` wrapper: duration, status, host для исходящих HTTP запросов.
- **Cache метрики** — hit/miss rate, latency для Redis/memcached.

### Трейсинг
- [x] **HTTP трейсинг** — автоматический через `HTTPMiddleware` (otelhttp) при `WithOtel`.
- [x] **gRPC трейсинг** — автоматический через `otelgrpc.NewServerHandler()` при `WithOtel`.
- **Database трейсинг** — автоматические спаны для SQL запросов с текстом запроса в атрибутах (sanitized).
- **HTTP client трейсинг** — `otelhttp.Transport` wrapper для исходящих запросов с propagation.
- **Redis трейсинг** — спаны для команд Redis.
- **Кастомные спаны в middleware** — добавление бизнес-атрибутов (user_id, tenant_id) из контекста запроса.
- **Exemplars** — привязка trace_id к метрикам для перехода из графика в трейс одним кликом.

### Логирование
- [x] **Автоматический TraceID в логах** — опция `TraceID bool` в `logger.Options`, оборачивает handler в `TraceIDHandler`.
- [x] **Loki интеграция** — отправка логов в Loki через `LokiEnabled`/`LokiURL` в `logger.Options`.
- **Structured error logging** — автоматическое добавление stack trace при `slog.Error`.
- **Sampling логов** — при высокой нагрузке логировать только N% debug/info записей.
- **Sensitive data masking** — slog handler для маскировки PII (email, phone, tokens) в логах.

## Server

### Middleware
- [x] **Recovery middleware** — panic recovery с записью в спан и счётчиком паник, автоматически при `WithOtel`.
- **Request ID** — генерация/проброс `X-Request-ID`, добавление в context и логи.
- **Rate limiter** — встроенный `rate.Limiter` middleware с конфигурацией через опции.
- **Circuit breaker** — `sony/gobreaker` для защиты от каскадных отказов.
- **CORS** — настраиваемый CORS middleware из коробки.
- **Auth middleware** — JWT/OAuth2 валидация с извлечением claims в context.
- **Request body limit** — защита от oversized payload.

### Health checks
- [x] **Liveness probe** — `/healthz` на debug-сервере.
- **Readiness probe** — `/readyz` с проверкой зависимостей (DB, Redis, message broker).
- **Регистрация check-функций** — `WithReadinessCheck("postgres", func() error {...})`.
- **Startup probe** — `/startupz` для k8s startupProbe.

### Graceful shutdown
- [x] **OTEL graceful shutdown** — автоматический при `WithOtel` (flush traces + metrics).
- **Orchestrated shutdown** — единый порядок остановки: drain HTTP → drain gRPC → flush OTEL → stop Loki → close DB. Сейчас пользователь управляет порядком вручную.
- **Shutdown timeout** — конфигурируемый таймаут на graceful shutdown.

## Инфраструктура

### Config
- **Единый конфиг** — пакет `config/` для чтения конфигурации из env/yaml/toml с валидацией и дефолтами.
- **Feature flags** — простой механизм для включения/выключения фич без редеплоя.

### Testing
- **Test helpers** — `platform/testing` с утилитами: test server, mock DB, fake OTEL collector.
- **Integration test infra** — testcontainers для Postgres, Redis, Kafka в тестах.

### Clients
- **gRPC client** — обёртка с retry, circuit breaker, трейсингом, метриками.
- **HTTP client** — обёртка с retry, трейсингом, метриками, propagation.
- **Message broker** — абстракция для Kafka/NATS/RabbitMQ с трейсингом.

## Приоритеты

| Приоритет | Что | Статус |
|-----------|-----|--------|
| Высокий | TraceID в логах автоматически | Готово |
| Высокий | Runtime Go метрики | Готово |
| Высокий | Readiness probes | В планах |
| Средний | HTTP/gRPC client wrappers | В планах |
| Средний | Database трейсинг/метрики | В планах |
| Средний | Request ID | В планах |
| Средний | Единый конфиг пакет | В планах |
| Низкий | Rate limiter / circuit breaker | В планах |
| Низкий | Message broker абстракция | В планах |
