package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration.
type Config struct {
	DBPath        string   `yaml:"db_path"`
	DatDir        string   `yaml:"dat_dir"`
	RegionOrder   []string `yaml:"region_order"`
	QuarantineDir string   `yaml:"quarantine_dir"`
}

// DefaultConfig returns configuration with default values.
func DefaultConfig() *Config {
	return &Config{
		DBPath:      "romman.db",
		RegionOrder: []string{"Europe", "World", "USA", "Japan"},
	}
}

// configPaths returns the list of paths to search for config file.
func configPaths() []string {
	paths := []string{
		".romman.yaml",
		".romman.yml",
	}

	// Check home config dir
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "romman", "config.yaml"),
			filepath.Join(home, ".config", "romman", "config.yml"),
			filepath.Join(home, ".romman.yaml"),
		)
	}

	return paths
}

// Load loads configuration from file or returns defaults.
// Priority: env ROMMAN_CONFIG > search paths > defaults
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Check env for explicit config path
	if envPath := os.Getenv("ROMMAN_CONFIG"); envPath != "" {
		if err := cfg.loadFromFile(envPath); err != nil {
			return nil, err
		}
		cfg.applyEnvOverrides()
		return cfg, nil
	}

	// Search for config file
	for _, path := range configPaths() {
		if _, err := os.Stat(path); err == nil {
			if err := cfg.loadFromFile(path); err != nil {
				return nil, err
			}
			break
		}
	}

	cfg.applyEnvOverrides()
	return cfg, nil
}

func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

func (c *Config) applyEnvOverrides() {
	if dbPath := os.Getenv("ROMMAN_DB"); dbPath != "" {
		c.DBPath = dbPath
	}
	if datDir := os.Getenv("ROMMAN_DAT_DIR"); datDir != "" {
		c.DatDir = datDir
	}
}

// GetDBPath returns the database path, applying defaults.
func (c *Config) GetDBPath() string {
	if c.DBPath != "" {
		return c.DBPath
	}
	return "romman.db"
}

// GetDatDir returns the DAT files directory.
func (c *Config) GetDatDir() string {
	return c.DatDir
}

// GetRegionOrder returns region priority order.
func (c *Config) GetRegionOrder() []string {
	if len(c.RegionOrder) > 0 {
		return c.RegionOrder
	}
	return []string{"Europe", "World", "USA", "Japan"}
}
