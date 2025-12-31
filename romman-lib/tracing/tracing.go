// Package tracing provides OpenTelemetry instrumentation for ROM Manager.
package tracing

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "romman"
	serviceVersion = "1.0.0"
)

// Config holds tracing configuration.
type Config struct {
	Enabled  bool   // Enable tracing
	Endpoint string // OTLP endpoint (e.g., "localhost:4317")
}

// DefaultConfig returns sensible tracing defaults.
func DefaultConfig() Config {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	return Config{
		Enabled:  endpoint != "",
		Endpoint: endpoint,
	}
}

var tracer trace.Tracer

// Setup initializes the OpenTelemetry tracer provider.
// Returns a shutdown function that should be called on application exit.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		// No-op tracer when disabled
		tracer = otel.Tracer(serviceName)
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(serviceName)

	return tp.Shutdown, nil
}

// Tracer returns the configured tracer.
func Tracer() trace.Tracer {
	if tracer == nil {
		return otel.Tracer(serviceName)
	}
	return tracer
}

// StartSpan starts a new span with the given name.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}
