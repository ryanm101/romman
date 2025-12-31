package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "romman.db", cfg.DBPath)
	assert.Equal(t, []string{"Europe", "World", "USA", "Japan"}, cfg.RegionOrder)
	assert.Equal(t, 0, cfg.Scan.Workers)
	assert.Equal(t, 100, cfg.Scan.BatchSize)
	assert.True(t, cfg.Scan.Parallel)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestConfig_GetDBPath(t *testing.T) {
	tests := []struct {
		name     string
		dbPath   string
		expected string
	}{
		{"returns configured path", "custom.db", "custom.db"},
		{"returns default when empty", "", "romman.db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{DBPath: tt.dbPath}
			assert.Equal(t, tt.expected, cfg.GetDBPath())
		})
	}
}

func TestConfig_GetRegionOrder(t *testing.T) {
	tests := []struct {
		name        string
		regionOrder []string
		expected    []string
	}{
		{"returns configured order", []string{"USA", "Europe"}, []string{"USA", "Europe"}},
		{"returns default when empty", nil, []string{"Europe", "World", "USA", "Japan"}},
		{"returns default when zero length", []string{}, []string{"Europe", "World", "USA", "Japan"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{RegionOrder: tt.regionOrder}
			assert.Equal(t, tt.expected, cfg.GetRegionOrder())
		})
	}
}

func TestConfig_LoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
db_path: /custom/path.db
dat_dir: /dat/files
region_order:
  - USA
  - Japan
quarantine_dir: /quarantine
scan:
  workers: 4
  batch_size: 50
  parallel: false
logging:
  format: json
  level: debug
`
	err := os.WriteFile(configPath, []byte(configContent), 0644) // #nosec G306
	require.NoError(t, err)

	cfg := DefaultConfig()
	err = cfg.loadFromFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, "/custom/path.db", cfg.DBPath)
	assert.Equal(t, "/dat/files", cfg.DatDir)
	assert.Equal(t, []string{"USA", "Japan"}, cfg.RegionOrder)
	assert.Equal(t, "/quarantine", cfg.QuarantineDir)
	assert.Equal(t, 4, cfg.Scan.Workers)
	assert.Equal(t, 50, cfg.Scan.BatchSize)
	assert.False(t, cfg.Scan.Parallel)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestConfig_LoadFromFile_NotFound(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.loadFromFile("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestConfig_LoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644) // #nosec G306
	require.NoError(t, err)

	cfg := DefaultConfig()
	err = cfg.loadFromFile(configPath)
	assert.Error(t, err)
}

func TestConfig_ApplyEnvOverrides(t *testing.T) {
	// Save and restore env
	origDB := os.Getenv("ROMMAN_DB")
	origDat := os.Getenv("ROMMAN_DAT_DIR")
	defer func() {
		_ = os.Setenv("ROMMAN_DB", origDB)
		_ = os.Setenv("ROMMAN_DAT_DIR", origDat)
	}()

	_ = os.Setenv("ROMMAN_DB", "/env/db.db")
	_ = os.Setenv("ROMMAN_DAT_DIR", "/env/dat")

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()

	assert.Equal(t, "/env/db.db", cfg.DBPath)
	assert.Equal(t, "/env/dat", cfg.DatDir)
}

func TestLoad_WithEnvConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte("db_path: from_file.db"), 0644) // #nosec G306
	require.NoError(t, err)

	// Save and restore env
	origConfig := os.Getenv("ROMMAN_CONFIG")
	defer func() { _ = os.Setenv("ROMMAN_CONFIG", origConfig) }()

	_ = os.Setenv("ROMMAN_CONFIG", configPath)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "from_file.db", cfg.DBPath)
}

func TestLoad_DefaultsWhenNoFile(t *testing.T) {
	// Save and restore env
	origConfig := os.Getenv("ROMMAN_CONFIG")
	origDB := os.Getenv("ROMMAN_DB")
	defer func() {
		_ = os.Setenv("ROMMAN_CONFIG", origConfig)
		_ = os.Setenv("ROMMAN_DB", origDB)
	}()

	_ = os.Unsetenv("ROMMAN_CONFIG")
	_ = os.Unsetenv("ROMMAN_DB")

	// Change to temp dir where no config exists
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "romman.db", cfg.DBPath)
}
