package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func handleBackupCommand(ctx context.Context, args []string) {
	_ = ctx // May be used for cancellation in future
	if len(args) < 1 {
		fmt.Println("Usage: romman backup <destination>")
		fmt.Println("  Creates a timestamped backup of the database")
		os.Exit(1)
	}

	destDir := args[0]

	// Ensure destination directory exists
	// #nosec G301
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		PrintError("Error: failed to create destination directory: %v\n", err)
		os.Exit(1)
	}

	// Open source database
	srcPath := getDBPath()
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		PrintError("Error: database not found at %s\n", srcPath)
		os.Exit(1)
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("romman-%s.db", timestamp)
	destPath := filepath.Join(destDir, backupName)

	// Copy the file
	// #nosec G304
	src, err := os.ReadFile(srcPath)
	if err != nil {
		PrintError("Error: failed to read database: %v\n", err)
		os.Exit(1)
	}

	// #nosec G306
	if err := os.WriteFile(destPath, src, 0o644); err != nil {
		PrintError("Error: failed to write backup: %v\n", err)
		os.Exit(1)
	}

	result := map[string]interface{}{
		"source":      srcPath,
		"destination": destPath,
		"size":        len(src),
		"timestamp":   timestamp,
	}

	if outputCfg.JSON {
		PrintResult(result)
	} else {
		PrintInfo("Backup created: %s\n", destPath)
		PrintInfo("  Size: %d bytes\n", len(src))
	}
}
