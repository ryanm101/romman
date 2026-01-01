package dat

import (
	"embed"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed system_defaults.yaml
var defaultsFS embed.FS

// SystemMappingsConfig holds system mappings from YAML.
type SystemMappingsConfig struct {
	// DirectoryMappings maps directory names to system IDs
	DirectoryMappings map[string]string `yaml:"directory_mappings"`
	// DATMappings maps DAT name patterns to system IDs
	DATMappings map[string]string `yaml:"dat_mappings"`
	// DisplayNames maps system IDs to human-readable names
	DisplayNames map[string]string `yaml:"display_names"`
}

var (
	cachedMappings     *SystemMappingsConfig
	cachedMappingsOnce sync.Once
)

// LoadSystemMappings loads system mappings, starting with embedded defaults
// then merging any user-defined overrides from systems.yaml.
// It searches for user config in: ROMMAN_SYSTEMS_FILE, current directory,
// ~/.config/romman/, /etc/romman/
func LoadSystemMappings() *SystemMappingsConfig {
	cachedMappingsOnce.Do(func() {
		// Start with embedded defaults
		cachedMappings = loadEmbeddedDefaults()

		// Merge user overrides on top
		paths := getSystemMappingPaths()
		for _, path := range paths {
			if cfg, err := loadMappingsFromFile(path); err == nil {
				mergeMappings(cachedMappings, cfg)
			}
		}
	})
	return cachedMappings
}

// loadEmbeddedDefaults loads the built-in defaults from the embedded YAML.
func loadEmbeddedDefaults() *SystemMappingsConfig {
	cfg := &SystemMappingsConfig{
		DirectoryMappings: make(map[string]string),
		DATMappings:       make(map[string]string),
		DisplayNames:      make(map[string]string),
	}

	data, err := defaultsFS.ReadFile("system_defaults.yaml")
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg
	}

	return cfg
}

// mergeMappings merges source mappings into dest (source takes precedence).
func mergeMappings(dest, source *SystemMappingsConfig) {
	if source == nil {
		return
	}
	for k, v := range source.DirectoryMappings {
		dest.DirectoryMappings[k] = v
	}
	for k, v := range source.DATMappings {
		dest.DATMappings[k] = v
	}
	for k, v := range source.DisplayNames {
		dest.DisplayNames[k] = v
	}
}

func getSystemMappingPaths() []string {
	var paths []string

	// Check ROMMAN_SYSTEMS_FILE env var first (highest priority)
	if envPath := os.Getenv("ROMMAN_SYSTEMS_FILE"); envPath != "" {
		paths = append(paths, envPath)
	}

	// Check current directory
	paths = append(paths, "systems.yaml")

	// Check ~/.config/romman/
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "romman", "systems.yaml"))
	}

	// Check /etc/romman/
	paths = append(paths, "/etc/romman/systems.yaml")

	return paths
}

func loadMappingsFromFile(path string) (*SystemMappingsConfig, error) {
	// #nosec G304
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg SystemMappingsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ResetSystemMappings clears cached mappings (for testing).
func ResetSystemMappings() {
	cachedMappingsOnce = sync.Once{}
	cachedMappings = nil
}

// GetDefaultMappings returns a fresh copy of the embedded default mappings.
// Useful for inspection or testing.
func GetDefaultMappings() *SystemMappingsConfig {
	return loadEmbeddedDefaults()
}
