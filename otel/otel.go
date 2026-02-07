package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

const (
	defaultTimeout              = 5 * time.Second
	defaultCompressor           = "gzip"
	defaultRetryInitialInterval = 500 * time.Millisecond
	defaultRetryMaxInterval     = 5 * time.Second
	defaultRetryMaxElapsedTime  = 30 * time.Second
)

// Config — конфигурация OTEL провайдеров.
type Config struct {
	// ServiceName имя сервиса для атрибутов ресурса.
	ServiceName string
	// Endpoint адрес OTLP коллектора (напр. "localhost:4317").
	Endpoint string
	// SampleRate процент трейсов для сохранения (0.0–1.0). 0 означает 1.0 (100%).
	SampleRate float64
}

func (c Config) sampleRate() float64 {
	if c.SampleRate <= 0 || c.SampleRate > 1 {
		return 1.0
	}
	return c.SampleRate
}

// Provider хранит OTEL провайдеры для graceful shutdown.
type Provider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
}

// Shutdown корректно завершает работу всех провайдеров.
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("otel shutdown errors: %v", errs)
	}

	return nil
}

// InitTracer инициализирует глобальный TracerProvider с OTLP gRPC экспортером.
func InitTracer(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(defaultTimeout),
		otlptracegrpc.WithCompressor(defaultCompressor),
		otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: defaultRetryInitialInterval,
			MaxInterval:     defaultRetryMaxInterval,
			MaxElapsedTime:  defaultRetryMaxElapsedTime,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.sampleRate()))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// InitMeter инициализирует глобальный MeterProvider с OTLP gRPC экспортером и Prometheus reader.
func InitMeter(ctx context.Context, cfg Config) (*metric.MeterProvider, error) {
	otlpExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithTimeout(defaultTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(otlpExporter)),
		metric.WithReader(promExporter),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp, nil
}
