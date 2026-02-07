package logger

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/grafana/loki-client-go/loki"
	slogloki "github.com/samber/slog-loki/v3"
)

// Options параметры для создания логгера.
type Options struct {
	// Level уровень логирования: DEBUG, INFO, WARN, ERROR
	Level string
	// JSON если true — вывод в JSON (для прода), иначе цветной текст (для локальной разработки)
	JSON bool
	// LokiEnabled включить отправку логов в Loki
	LokiEnabled bool
	// LokiURL URL Loki push API (напр. http://localhost:3100/loki/api/v1/push)
	LokiURL string
	// ServiceName имя сервиса для label service_name в Loki
	ServiceName string
}

// NewLogger создаёт slog.Logger и устанавливает его как глобальный (slog.Default).
// Второй возврат — функция-closer для Loki client. Если Loki выключен — no-op.
//
// Локально (JSON=false): цветной вывод через tint, время в читаемом формате, source для быстрого перехода в IDE.
// Прод (JSON=true): структурированный JSON в stdout, без цветов, с source для трейсинга ошибок.
func NewLogger(opts Options) (*slog.Logger, func()) {
	level := parseLevel(opts.Level)

	var consoleHandler slog.Handler
	if opts.JSON {
		consoleHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	} else {
		consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	}

	closer := func() {}

	var handler slog.Handler = consoleHandler

	if opts.LokiEnabled && opts.LokiURL != "" {
		lokiCfg, _ := loki.NewDefaultConfig(opts.LokiURL)
		lokiCfg.TenantID = ""
		lokiCfg.BatchWait = time.Second

		lokiClient, err := loki.New(lokiCfg)
		if err == nil {
			lokiHandler := slogloki.Option{
				Level:  level,
				Client: lokiClient,
			}.NewLokiHandler()

			lokiHandler = lokiHandler.WithAttrs([]slog.Attr{
				slog.String("service_name", opts.ServiceName),
			})

			handler = newMultiHandler(consoleHandler, lokiHandler)
			closer = func() { lokiClient.Stop() }
		}
	}

	l := slog.New(handler)
	slog.SetDefault(l)

	return l, closer
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
