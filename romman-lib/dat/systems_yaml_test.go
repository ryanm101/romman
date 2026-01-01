package dat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSystemMappings_NoFile(t *testing.T) {
	// Reset cached mappings
	ResetSystemMappings()

	// Load should succeed with empty user mappings
	cfg := LoadSystemMappings()
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.DirectoryMappings)
	assert.NotNil(t, cfg.DATMappings)
	assert.NotNil(t, cfg.DisplayNames)
}

func TestLoadSystemMappings_FromFile(t *testing.T) {
	// Reset cached mappings
	ResetSystemMappings()

	// Create temp YAML file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "systems.yaml")
	yamlContent := `
directory_mappings:
  customdir: customsystem
  mynes: nes
dat_mappings:
  "custom dat name": customsystem
display_names:
  customsystem: "My Custom System"
`
	// #nosec G306
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set env var to point to our test file
	origEnv := os.Getenv("ROMMAN_SYSTEMS_FILE")
	defer func() { _ = os.Setenv("ROMMAN_SYSTEMS_FILE", origEnv) }()
	_ = os.Setenv("ROMMAN_SYSTEMS_FILE", yamlPath)

	cfg := LoadSystemMappings()
	assert.Equal(t, "customsystem", cfg.DirectoryMappings["customdir"])
	assert.Equal(t, "nes", cfg.DirectoryMappings["mynes"])
	assert.Equal(t, "customsystem", cfg.DATMappings["custom dat name"])
	assert.Equal(t, "My Custom System", cfg.DisplayNames["customsystem"])
}

func TestDetectSystemFromDirName_UserMappingOverride(t *testing.T) {
	// Reset cached mappings
	ResetSystemMappings()

	// Create temp YAML file with override
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "systems.yaml")
	yamlContent := `
directory_mappings:
  nes: customnes
  mynewsystem: newsys
`
	// #nosec G306
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set env var to point to our test file
	origEnv := os.Getenv("ROMMAN_SYSTEMS_FILE")
	defer func() {
		_ = os.Setenv("ROMMAN_SYSTEMS_FILE", origEnv)
		ResetSystemMappings()
	}()
	_ = os.Setenv("ROMMAN_SYSTEMS_FILE", yamlPath)

	// User override should take precedence
	sys, found := DetectSystemFromDirName("nes")
	assert.True(t, found)
	assert.Equal(t, "customnes", sys)

	// New user mapping should work
	sys, found = DetectSystemFromDirName("mynewsystem")
	assert.True(t, found)
	assert.Equal(t, "newsys", sys)
}

func TestDetectSystemFromDirName_BuiltInMappings(t *testing.T) {
	// Reset and don't load any user file
	ResetSystemMappings()
	origEnv := os.Getenv("ROMMAN_SYSTEMS_FILE")
	defer func() { _ = os.Setenv("ROMMAN_SYSTEMS_FILE", origEnv) }()
	_ = os.Unsetenv("ROMMAN_SYSTEMS_FILE")

	tests := []struct {
		dirName  string
		expected string
		found    bool
	}{
		{"nes", "nes", true},
		{"NES", "nes", true},
		{"SNES", "snes", true},
		{"gameboy", "gb", true},
		{"Game-Boy", "gb", true},
		{"game_boy", "gb", true},
		{"megadrive", "md", true},
		{"mega-drive", "md", true},
		{"psx", "psx", true},
		{"ps1", "psx", true},
		{"unknown_system", "", false},
		{"fds", "fds", true},
		{"virtualboy", "vb", true},
		{"sg1000", "sg1000", true},
		{"amstradcpc", "cpc", true},
		{"zxspectrum", "zxspectrum", true},
		{"colecovision", "coleco", true},
		{"pcenginecd", "pcecd", true},
		{"supergrafx", "sgx", true},
	}

	for _, tt := range tests {
		t.Run(tt.dirName, func(t *testing.T) {
			sys, found := DetectSystemFromDirName(tt.dirName)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected, sys)
			}
		})
	}
}

func TestResetSystemMappings(t *testing.T) {
	// Load once
	cfg1 := LoadSystemMappings()
	assert.NotNil(t, cfg1)

	// Reset
	ResetSystemMappings()

	// Load again - should get fresh instance
	cfg2 := LoadSystemMappings()
	assert.NotNil(t, cfg2)
}

func TestGetSystemMappingPaths(t *testing.T) {
	// Test that env var path is included first
	origEnv := os.Getenv("ROMMAN_SYSTEMS_FILE")
	defer func() { _ = os.Setenv("ROMMAN_SYSTEMS_FILE", origEnv) }()

	_ = os.Setenv("ROMMAN_SYSTEMS_FILE", "/custom/path/systems.yaml")
	paths := getSystemMappingPaths()

	assert.Contains(t, paths, "/custom/path/systems.yaml")
	assert.Equal(t, "/custom/path/systems.yaml", paths[0]) // Should be first
}

func TestGetDefaultMappings_HasBuiltInDefaults(t *testing.T) {
	// Get fresh embedded defaults (not affected by user config)
	defaults := GetDefaultMappings()

	// Verify key directory mappings are present
	assert.Equal(t, "nes", defaults.DirectoryMappings["nes"])
	assert.Equal(t, "snes", defaults.DirectoryMappings["snes"])
	assert.Equal(t, "md", defaults.DirectoryMappings["megadrive"])
	assert.Equal(t, "psx", defaults.DirectoryMappings["psx"])
	assert.Equal(t, "fds", defaults.DirectoryMappings["fds"])
	assert.Equal(t, "sgx", defaults.DirectoryMappings["supergrafx"])

	// Verify DAT mappings are present
	assert.Equal(t, "nes", defaults.DATMappings["nintendo - nintendo entertainment system"])
	assert.Equal(t, "psx", defaults.DATMappings["sony - playstation"])

	// Verify display names are present
	assert.Equal(t, "Nintendo Entertainment System", defaults.DisplayNames["nes"])
	assert.Equal(t, "Sony PlayStation", defaults.DisplayNames["psx"])
}
