package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ryanm101/romman-lib/config"
	"github.com/ryanm101/romman-lib/db"
	"github.com/ryanm101/romman-lib/logging"
	"github.com/ryanm101/romman-lib/tracing"
)

var cfg *config.Config

func main() {
	ctx := context.Background()

	// Load config
	var err error
	cfg, err = config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Setup Logging
	logging.Setup(logging.Config{
		Format: cfg.Logging.Format,
		Level:  cfg.Logging.Level,
	})

	// Setup Tracing
	shutdown, err := tracing.Setup(ctx, tracing.Config{
		Enabled:  os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "",
		Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	})
	if err != nil {
		logging.Error("failed to setup tracing", "error", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			logging.Error("failed to shutdown tracing", "error", err)
		}
	}()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "dat":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman dat <command>")
			fmt.Println("Commands: import, scan")
			os.Exit(1)
		}
		handleDatCommand(ctx, os.Args[2:])
	case "systems":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman systems <command>")
			fmt.Println("Commands: list, info, status")
			os.Exit(1)
		}
		handleSystemsCommand(os.Args[2:])
	case "library":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman library <command>")
			fmt.Println("Commands: add, list, scan, status, unmatched, discover")
			os.Exit(1)
		}
		handleLibraryCommand(ctx, os.Args[2:])
	case "duplicates":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman duplicates <command>")
			fmt.Println("Commands: list")
			os.Exit(1)
		}
		handleDuplicatesCommand(os.Args[2:])
	case "cleanup":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman cleanup <command>")
			fmt.Println("Commands: plan, exec")
			os.Exit(1)
		}
		handleCleanupCommand(os.Args[2:])
	case "prefer":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman prefer <command>")
			fmt.Println("Commands: rebuild, list")
			os.Exit(1)
		}
		handlePreferCommand(os.Args[2:])
	case "export":
		if len(os.Args) < 4 {
			fmt.Println("Usage: romman export <library> <report> <format> [file]")
			fmt.Println("       romman export <library> retroarch <output.lpl>")
			fmt.Println("Reports: matched, missing, preferred, unmatched, 1g1r")
			fmt.Println("Formats: csv, json, retroarch")
			os.Exit(1)
		}
		handleExportCommand(os.Args[2:])

	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("romman - ROM Manager")
	fmt.Println()
	fmt.Println("Usage: romman <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  dat import <file>                   Import a DAT file")
	fmt.Println("  dat scan                            Auto-import DATs from dat_dir")
	fmt.Println("  systems list                        List all systems")
	fmt.Println("  systems info <name>                 Show system details")
	fmt.Println("  systems status                      Show all systems summary")
	fmt.Println("  library add <name> <path> <system>  Add a library")
	fmt.Println("  library list                        List all libraries")
	fmt.Println("  library discover <dir> [--add]      Auto-detect libraries from subdirs")
	fmt.Println("  library scan <name>                 Scan a library for ROMs")
	fmt.Println("  library scan-all                    Scan all libraries")
	fmt.Println("  library status <name>               Show release status")
	fmt.Println("  library unmatched <name>            Show unmatched files")
	fmt.Println("  library rename <name> [--dry-run]   Rename files to DAT names")
	fmt.Println("  library verify <name>               Check file integrity")
	fmt.Println("  duplicates list <library>           List duplicate files")
	fmt.Println("  cleanup plan <lib> <quarantine>     Generate cleanup plan")
	fmt.Println("  cleanup exec <plan> [--dry-run]     Execute cleanup plan")
	fmt.Println("  prefer rebuild <system>             Rebuild preferred releases")
	fmt.Println("  prefer list <system>                List preferred releases")
	fmt.Println("  export <lib> <report> <fmt> [file]  Export report (csv/json)")
	fmt.Println("  help                                Show this help")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  ROMMAN_DB                           Database path (default: romman.db)")
}

func getDBPath() string {
	return cfg.GetDBPath()
}

func openDB() (*db.DB, error) {
	return db.Open(getDBPath())
}
