package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func handleConfigCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman config <command>")
		fmt.Println("Commands: show, init")
		os.Exit(1)
	}

	switch args[0] {
	case "show":
		showConfig()
	case "init":
		initConfig()
	default:
		fmt.Printf("Unknown config command: %s\n", args[0])
		os.Exit(1)
	}
}

func showConfig() {
	if outputCfg.JSON {
		PrintResult(cfg)
		return
	}

	// Pretty print as YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		PrintError("Error: failed to marshal config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("# Active Configuration")
	fmt.Println(string(data))
	fmt.Println("# Config file:", cfg.GetDBPath())
}

func initConfig() {
	configPath := ".romman.yaml"

	// Check if file exists
	if _, err := os.Stat(configPath); err == nil {
		PrintError("Error: config file already exists at %s\n", configPath)
		os.Exit(1)
	}

	example := `# ROM Manager Configuration
db:
  path: romman.db

# Directory containing DAT files
dat_dir: dat

logging:
  level: info   # debug, info, warn, error
  format: text  # text or json

# Parallel scanning settings
scan:
  parallel: true
  workers: 4
  batch_size: 100
`

	if err := os.WriteFile(configPath, []byte(example), 0o644); err != nil {
		PrintError("Error: failed to write config: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(map[string]string{"path": configPath, "status": "created"})
	} else {
		PrintInfo("Created config file: %s\n", configPath)
	}
}
