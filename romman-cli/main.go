package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ryanm101/romman-lib/config"
	"github.com/ryanm101/romman-lib/db"
	"github.com/ryanm101/romman-lib/logging"
	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/baggage"
)

var cfg *config.Config

func main() {
	ctx := context.Background()

	// Set global baggage
	m, _ := baggage.NewMember("app.version", "2.0.0")
	b, _ := baggage.New(m)
	ctx = baggage.ContextWithBaggage(ctx, b)

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

	// Parse global flags (--json, --quiet)
	args := parseGlobalFlags(os.Args[1:])

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "dat":
		if len(args) < 2 {
			fmt.Println("Usage: romman dat <command>")
			fmt.Println("Commands: import, scan")
			os.Exit(1)
		}
		handleDatCommand(ctx, args[1:])
	case "systems":
		if len(args) < 2 {
			fmt.Println("Usage: romman systems <command>")
			fmt.Println("Commands: list, info, status")
			os.Exit(1)
		}
		handleSystemsCommand(ctx, args[1:])
	case "library":
		if len(args) < 2 {
			fmt.Println("Usage: romman library <command>")
			fmt.Println("Commands: add, list, scan, status, unmatched, discover")
			os.Exit(1)
		}
		handleLibraryCommand(ctx, args[1:])
	case "duplicates":
		if len(args) < 2 {
			fmt.Println("Usage: romman duplicates <command>")
			fmt.Println("Commands: list")
			os.Exit(1)
		}
		handleDuplicatesCommand(ctx, args[1:])
	case "cleanup":
		if len(args) < 2 {
			fmt.Println("Usage: romman cleanup <command>")
			fmt.Println("Commands: plan, exec")
			os.Exit(1)
		}
		handleCleanupCommand(ctx, args[1:])
	case "prefer":
		if len(args) < 2 {
			fmt.Println("Usage: romman prefer <command>")
			fmt.Println("Commands: rebuild, list")
			os.Exit(1)
		}
		handlePreferCommand(ctx, args[1:])
	case "export":
		if len(args) < 3 {
			fmt.Println("Usage: romman export <library> <report> <format> [file]")
			fmt.Println("       romman export <library> retroarch <output.lpl>")
			fmt.Println("Reports: matched, missing, preferred, unmatched, 1g1r")
			fmt.Println("Formats: csv, json, retroarch")
			os.Exit(1)
		}
		handleExportCommand(ctx, args[1:])

	case "help", "-h", "--help":
		printUsage()
	case "doctor":
		handleDoctorCommand(ctx, args[1:])
	case "backup":
		if len(args) < 2 {
			fmt.Println("Usage: romman backup <destination>")
			os.Exit(1)
		}
		handleBackupCommand(ctx, args[1:])
	case "config":
		handleConfigCommand(ctx, args[1:])
	case "scrape":
		handleScrapeCommand(ctx, args[1:])
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("romman - ROM Manager")
	fmt.Println()
	fmt.Println("Usage: romman [global options] <command> [options]")
	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  --json                              Output in JSON format")
	fmt.Println("  --quiet, -q                         Suppress non-error output")
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
	fmt.Println("  doctor                              Run database health checks")
	fmt.Println("  backup <dest>                       Backup database to destination")
	fmt.Println("  config show                         Show active configuration")
	fmt.Println("  config init                         Initialize example config")
	fmt.Println("  scrape <release_id>                 Scrape metadata from IGDB")
	fmt.Println("  help                                Show this help")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  ROMMAN_DB                           Database path (default: romman.db)")
}

func getDBPath() string {
	return cfg.GetDBPath()
}

func openDB(ctx context.Context) (*db.DB, error) {
	return db.Open(ctx, getDBPath())
}
