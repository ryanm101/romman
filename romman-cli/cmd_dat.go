package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanm101/romman-lib/dat"
)

func handleDatCommand(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman dat <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "import":
		if len(args) < 2 {
			fmt.Println("Usage: romman dat import <file> [file...]")
			os.Exit(1)
		}
		importDATs(ctx, args[1:])
	case "scan":
		scanDatDir(ctx)
	default:
		fmt.Printf("Unknown dat command: %s\n", args[0])
		os.Exit(1)
	}
}

func importDATs(ctx context.Context, paths []string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	importer := dat.NewImporter(database.Conn())

	var results []*dat.ImportResult
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error resolving path %s: %v\n", path, err)
			continue
		}

		if !outputCfg.Quiet && !outputCfg.JSON {
			fmt.Printf("Importing %s...\n", filepath.Base(path))
		}
		result, err := importer.Import(ctx, absPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			continue
		}
		results = append(results, result)

		if !outputCfg.Quiet && !outputCfg.JSON {
			status := "updated"
			if result.IsNewSystem {
				status = "created"
			}
			fmt.Printf("  System: %s (%s)\n", result.SystemName, status)
			fmt.Printf("  Games imported: %d, ROMs: %d, Skipped: %d\n",
				result.GamesImported, result.RomsImported, result.GamesSkipped)
		}
	}

	if outputCfg.JSON {
		PrintResult(results)
	}
}

func scanDatDir(ctx context.Context) {
	datDir := cfg.GetDatDir()
	if datDir == "" {
		fmt.Println("No dat_dir configured in .romman.yaml")
		fmt.Println("Set dat_dir or use: romman dat import <file>")
		os.Exit(1)
	}

	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	importer := dat.NewImporter(database.Conn())

	entries, err := os.ReadDir(datDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error reading dat_dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scanning DAT directory: %s\n\n", datDir)

	var results []*dat.ImportResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".dat" && ext != ".xml" {
			continue
		}

		path := filepath.Join(datDir, entry.Name())
		if !outputCfg.Quiet && !outputCfg.JSON {
			fmt.Printf("Importing %s...\n", entry.Name())
		}

		result, err := importer.Import(ctx, path)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		results = append(results, result)

		if !outputCfg.Quiet && !outputCfg.JSON {
			status := "updated"
			if result.IsNewSystem {
				status = "created"
			}
			fmt.Printf("  System: %s (%s) - %d games\n", result.SystemName, status, result.GamesImported)
		}
	}

	if outputCfg.JSON {
		PrintResult(results)
	} else if !outputCfg.Quiet {
		fmt.Printf("\nImported %d DAT files\n", len(results))
	}
}
