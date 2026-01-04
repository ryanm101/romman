// Package tracing provides OpenTelemetry instrumentation for ROM Manager.
package tracing

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// Setup initializes the OpenTelemetry tracer provider.
// Returns a shutdown function that should be called on application exit.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	noopShutdown := func(context.Context) error { return nil }

	if !cfg.Enabled || cfg.Endpoint == "" {
		// When disabled, use global noop tracer (no Setup needed)
		return noopShutdown, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return noopShutdown, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"", // Omit schema URL to avoid conflicts with Default resource
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return noopShutdown, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Tracer returns the configured tracer from the global provider.
// This ensures all spans use the same tracer set up by Setup().
func Tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer(serviceName)
}

// StartSpan starts a new span with the given name.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// WithAttributes returns a SpanStartOption that adds the given attributes to the span.
func WithAttributes(attrs ...attribute.KeyValue) trace.SpanStartOption {
	return trace.WithAttributes(attrs...)
}

// RecordError records an error on the span and sets the span status to Error.
// This should be called when an operation fails.
func RecordError(span trace.Span, err error) {
	if err != nil && span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetSpanOK marks the span as successful.
// This should be called when an operation completes successfully.
func SetSpanOK(span trace.Span) {
	if span != nil {
		span.SetStatus(codes.Ok, "")
	}
}

// AddSpanAttributes adds attributes to an existing span.
func AddSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	if span != nil {
		span.SetAttributes(attrs...)
	}
}
