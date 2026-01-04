package tracing

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestDefaultConfig(t *testing.T) {
	// Save and restore env
	orig := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	defer func() { _ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", orig) }()

	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	cfg := DefaultConfig()
	assert.False(t, cfg.Enabled)
	assert.Empty(t, cfg.Endpoint)
}

func TestDefaultConfig_WithEnv(t *testing.T) {
	// Save and restore env
	orig := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	defer func() { _ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", orig) }()

	_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "localhost:4317", cfg.Endpoint)
}

func TestSetup_Disabled(t *testing.T) {
	cfg := Config{Enabled: false}
	shutdown, err := Setup(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, shutdown)

	// Shutdown should not error
	err = shutdown(context.Background())
	assert.NoError(t, err)
}

func TestSetup_EmptyEndpoint(t *testing.T) {
	cfg := Config{Enabled: true, Endpoint: ""}
	shutdown, err := Setup(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, shutdown)
}

func TestTracer_ReturnsNonNil(t *testing.T) {
	// Reset tracer for this test
	oldTracer := tracer
	tracer = nil
	defer func() { tracer = oldTracer }()

	tr := Tracer()
	assert.NotNil(t, tr)
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-span")

	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, newCtx)

	span.End()
}

func TestWithAttributes(t *testing.T) {
	// Just verify it doesn't panic
	assert.NotPanics(t, func() {
		_ = WithAttributes()
	})
}

func TestRecordError(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test-error")

	// Should not panic with nil error
	assert.NotPanics(t, func() {
		RecordError(span, nil)
	})

	// Should not panic with actual error
	assert.NotPanics(t, func() {
		RecordError(span, assert.AnError)
	})

	span.End()
}

func TestRecordError_NilSpan(t *testing.T) {
	// Should not panic with nil span
	assert.NotPanics(t, func() {
		RecordError(nil, assert.AnError)
	})
}

func TestSetSpanOK(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test-ok")

	// Should not panic
	assert.NotPanics(t, func() {
		SetSpanOK(span)
	})

	span.End()
}

func TestSetSpanOK_NilSpan(t *testing.T) {
	// Should not panic with nil span
	assert.NotPanics(t, func() {
		SetSpanOK(nil)
	})
}

func TestAddSpanAttributes(t *testing.T) {
	ctx := context.Background()
	_, span := StartSpan(ctx, "test-attrs")

	// Should not panic
	assert.NotPanics(t, func() {
		AddSpanAttributes(span)
	})

	span.End()
}

func TestAddSpanAttributes_NilSpan(t *testing.T) {
	// Should not panic with nil span
	assert.NotPanics(t, func() {
		AddSpanAttributes(nil)
	})
}

func TestStartSpan_WithOptions(t *testing.T) {
	ctx := context.Background()

	// Start with attributes
	newCtx, span := StartSpan(ctx, "test-with-attrs",
		WithAttributes(
			attribute.String("key", "value"),
			attribute.Int("count", 42),
		),
	)

	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, newCtx)

	span.End()
}

func TestTracerAfterSetup(t *testing.T) {
	// Setup with disabled config
	cfg := Config{Enabled: false}
	shutdown, err := Setup(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = shutdown(context.Background()) }()

	// Tracer should be available
	tr := Tracer()
	assert.NotNil(t, tr)

	// Should be able to use it
	_, span := tr.Start(context.Background(), "test")
	assert.NotNil(t, span)
	span.End()
}
