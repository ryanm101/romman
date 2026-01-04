package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ryanm101/romman-lib/library"
)

func handleExportCommand(ctx context.Context, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: romman export <library> <report> <format> [file]")
		fmt.Println("       romman export <library> retroarch <output.lpl>")
		fmt.Println("       romman export <library> gamelist <output.xml> [--matched-only]")
		os.Exit(1)
	}

	libName := args[0]
	reportOrFormat := args[1]

	if reportOrFormat == "retroarch" {
		if len(args) < 3 {
			fmt.Println("Usage: romman export <library> retroarch <output.lpl>")
			os.Exit(1)
		}
		outputPath := args[2]
		exportRetroArch(ctx, libName, outputPath)
		return
	}

	if reportOrFormat == "gamelist" {
		if len(args) < 3 {
			fmt.Println("Usage: romman export <library> gamelist <output.xml> [--matched-only]")
			os.Exit(1)
		}
		outputPath := args[2]
		matchedOnly := len(args) > 3 && args[3] == "--matched-only"
		exportGamelist(ctx, libName, outputPath, matchedOnly)
		return
	}

	if reportOrFormat == "launchbox" {
		if len(args) < 3 {
			fmt.Println("Usage: romman export <library> launchbox <output.xml> [--matched-only]")
			os.Exit(1)
		}
		outputPath := args[2]
		matchedOnly := len(args) > 3 && args[3] == "--matched-only"
		exportLaunchBox(ctx, libName, outputPath, matchedOnly)
		return
	}

	// Generic report export
	if len(args) < 3 {
		fmt.Println("Usage: romman export <library> <report> <format> [file]")
		os.Exit(1)
	}

	report := args[1]
	format := args[2]
	output := ""
	if len(args) >= 4 {
		output = args[3]
	}
	exportReport(ctx, libName, report, format, output)
}

func exportReport(ctx context.Context, libName, report, format, output string) {
	reportType := library.ReportType(report)
	exportFormat := library.ExportFormat(format)

	switch reportType {
	case library.ReportMatched, library.ReportMissing, library.ReportPreferred, library.ReportUnmatched, library.Report1G1R, library.ReportStats, library.ReportDuplicates, library.ReportMismatch:
		// valid
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown report type: %s\n", report)
		fmt.Println("Valid reports: matched, missing, preferred, unmatched, 1g1r, stats, duplicates, mismatch")
		os.Exit(1)
	}

	switch exportFormat {
	case library.FormatCSV, library.FormatJSON, library.FormatTXT:
		// valid
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown format: %s\n", format)
		fmt.Println("Valid formats: csv, json, txt")
		os.Exit(1)
	}

	database, err := openDB(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	exporter := library.NewExporter(database.Conn(), manager)

	data, err := exporter.Export(context.Background(), libName, reportType, exportFormat)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
		os.Exit(1)
	}

	if output != "" {
		// #nosec G306
		if err := os.WriteFile(output, data, 0644); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}

		if outputCfg.JSON {
			PrintResult(map[string]interface{}{
				"library": libName,
				"report":  reportType,
				"format":  exportFormat,
				"output":  output,
				"status":  "success",
			})
		} else {
			fmt.Printf("Exported %s %s to %s\n", reportType, exportFormat, output)
		}
	} else {
		if exportFormat == library.FormatJSON && outputCfg.JSON {
			// Already JSON, just print it
			fmt.Print(string(data))
		} else if outputCfg.JSON {
			PrintResult(map[string]interface{}{
				"library": libName,
				"report":  reportType,
				"format":  exportFormat,
				"data":    string(data),
			})
		} else {
			fmt.Print(string(data))
		}
	}
}

func exportRetroArch(ctx context.Context, libraryName, outputPath string) {
	database, err := openDB(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	exporter := library.NewRetroArchExporter(database.Conn())
	if err := exporter.ExportPlaylist(context.Background(), libraryName, outputPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error exporting RetroArch playlist: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(map[string]interface{}{
			"library": libraryName,
			"format":  "retroarch",
			"output":  outputPath,
			"status":  "success",
		})
	} else {
		fmt.Printf("Exported RetroArch playlist to %s\n", outputPath)
	}
}

func exportGamelist(ctx context.Context, libraryName, outputPath string, matchedOnly bool) {
	database, err := openDB(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	exporter := library.NewExporter(database.Conn(), manager)

	opts := library.GamelistOptions{
		MatchedOnly: matchedOnly,
		PathPrefix:  "./",
	}

	data, err := exporter.ExportGamelist(context.Background(), libraryName, opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error exporting gamelist: %v\n", err)
		os.Exit(1)
	}

	// #nosec G306
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(map[string]interface{}{
			"library":     libraryName,
			"format":      "gamelist",
			"output":      outputPath,
			"matchedOnly": matchedOnly,
			"status":      "success",
		})
	} else {
		fmt.Printf("Exported EmulationStation gamelist.xml to %s\n", outputPath)
	}
}

func exportLaunchBox(ctx context.Context, libraryName, outputPath string, matchedOnly bool) {
	database, err := openDB(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	exporter := library.NewExporter(database.Conn(), manager)

	opts := library.LaunchBoxOptions{
		MatchedOnly: matchedOnly,
		PathPrefix:  ".\\",
	}

	data, err := exporter.ExportLaunchBox(context.Background(), libraryName, opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error exporting LaunchBox: %v\n", err)
		os.Exit(1)
	}

	// #nosec G306
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(map[string]interface{}{
			"library":     libraryName,
			"format":      "launchbox",
			"output":      outputPath,
			"matchedOnly": matchedOnly,
			"status":      "success",
		})
	} else {
		fmt.Printf("Exported LaunchBox platform XML to %s\n", outputPath)
	}
}
