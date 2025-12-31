package tracing

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
