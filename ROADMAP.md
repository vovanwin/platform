# Roadmap платформы

## Observability

### Метрики
- **Runtime метрики Go** — goroutines, heap alloc, GC pauses, open file descriptors через `prometheus/collectors.NewGoCollector()` и `NewProcessCollector()`. Автоматическая регистрация при `WithOtel`.
- **HTTP response size** — `Float64Histogram` по байтам тела ответа, помогает ловить аномальные payload.
- **HTTP request size** — `Float64Histogram` по `Content-Length` входящего запроса.
- **gRPC message size** — размер входящих/исходящих protobuf сообщений.
- **Database метрики** — middleware/wrapper для SQL: query duration, error rate, connection pool stats (open/idle/waiting).
- **HTTP client метрики** — `http.RoundTripper` wrapper: duration, status, host для исходящих HTTP запросов.
- **Cache метрики** — hit/miss rate, latency для Redis/memcached.

### Трейсинг
- **Database трейсинг** — автоматические спаны для SQL запросов с текстом запроса в атрибутах (sanitized).
- **HTTP client трейсинг** — `otelhttp.Transport` wrapper для исходящих запросов с propagation.
- **Redis трейсинг** — спаны для команд Redis.
- **Кастомные спаны в middleware** — добавление бизнес-атрибутов (user_id, tenant_id) из контекста запроса.
- **Exemplars** — привязка trace_id к метрикам для перехода из графика в трейс одним кликом.

### Логирование
- **Автоматический TraceID в логах** — встроить `TraceIDHandler` в `NewLogger`, чтобы не оборачивать вручную. Опция `WithTraceID bool` в `logger.Options`.
- **Structured error logging** — автоматическое добавление stack trace при `slog.Error`.
- **Sampling логов** — при высокой нагрузке логировать только N% debug/info записей.
- **Sensitive data masking** — slog handler для маскировки PII (email, phone, tokens) в логах.

## Server

### Middleware
- **Request ID** — генерация/проброс `X-Request-ID`, добавление в context и логи.
- **Rate limiter** — встроенный `rate.Limiter` middleware с конфигурацией через опции.
- **Circuit breaker** — `sony/gobreaker` для защиты от каскадных отказов.
- **CORS** — настраиваемый CORS middleware из коробки.
- **Auth middleware** — JWT/OAuth2 валидация с извлечением claims в context.
- **Request body limit** — защита от oversized payload.

### Health checks
- **Readiness probe** — `/readyz` с проверкой зависимостей (DB, Redis, message broker).
- **Регистрация check-функций** — `WithReadinessCheck("postgres", func() error {...})`.
- **Startup probe** — `/startupz` для k8s startupProbe.

### Graceful shutdown
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

| Приоритет | Что | Почему |
|-----------|-----|--------|
| Высокий | TraceID в логах автоматически | Связь логов и трейсов — основа debug workflow |
| Высокий | Runtime Go метрики | Видно утечки горутин и memory pressure |
| Высокий | Readiness probes | Нужно для production k8s |
| Средний | HTTP/gRPC client wrappers | Сквозной трейсинг между сервисами |
| Средний | Database трейсинг/метрики | Основной источник латентности |
| Средний | Request ID | Удобство отладки без трейсинга |
| Средний | Единый конфиг пакет | Убирает бойлерплейт из каждого сервиса |
| Низкий | Rate limiter / circuit breaker | Зависит от архитектуры (может быть на ingress) |
| Низкий | Message broker абстракция | Большой scope, зависит от выбора брокера |
