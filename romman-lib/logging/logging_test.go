package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "text", cfg.Format)
	assert.Equal(t, "info", cfg.Level)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{"debug", "debug", "DEBUG"},
		{"Debug uppercase", "DEBUG", "DEBUG"},
		{"info", "info", "INFO"},
		{"warn", "warn", "WARN"},
		{"warning alias", "warning", "WARN"},
		{"error", "error", "ERROR"},
		{"unknown defaults to info", "unknown", "INFO"},
		{"empty defaults to info", "", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := parseLevel(tt.level)
			assert.Equal(t, tt.expected, level.String())
		})
	}
}

func TestSetup_TextFormat(t *testing.T) {
	cfg := Config{Format: "text", Level: "info"}
	Setup(cfg)

	// Verify logger was created (Get() shouldn't return nil)
	assert.NotNil(t, Get())
}

func TestSetup_JSONFormat(t *testing.T) {
	cfg := Config{Format: "json", Level: "debug"}
	Setup(cfg)

	assert.NotNil(t, Get())
}

func TestGet_ReturnsDefaultBeforeSetup(t *testing.T) {
	// Reset global logger for this test
	oldLogger := logger
	logger = nil
	defer func() { logger = oldLogger }()

	got := Get()
	assert.NotNil(t, got)
}

func TestLogFunctions_DoNotPanic(t *testing.T) {
	Setup(DefaultConfig())

	// These should not panic
	assert.NotPanics(t, func() { Debug("test message") })
	assert.NotPanics(t, func() { Info("test message") })
	assert.NotPanics(t, func() { Warn("test message") })
	assert.NotPanics(t, func() { Error("test message") })
}
