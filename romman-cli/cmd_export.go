package main

import (
	"fmt"
	"os"

	"github.com/ryanm101/romman-lib/library"
)

func handleExportCommand(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: romman export <library> <report> <format> [file]")
		fmt.Println("       romman export <library> retroarch <output.lpl>")
		os.Exit(1)
	}

	libraryName := args[0]
	reportOrFormat := args[1]

	if reportOrFormat == "retroarch" {
		if len(args) < 3 {
			fmt.Println("Usage: romman export <library> retroarch <output.lpl>")
			os.Exit(1)
		}
		outputPath := args[2]
		exportRetroArch(libraryName, outputPath)
		return
	}

	if len(args) < 3 {
		fmt.Println("Usage: romman export <library> <report> <format> [file]")
		os.Exit(1)
	}

	reportType := library.ReportType(args[1])
	format := library.ExportFormat(args[2])

	switch reportType {
	case library.ReportMatched, library.ReportMissing, library.ReportPreferred, library.ReportUnmatched, library.Report1G1R:
		// valid
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown report type: %s\n", args[1])
		fmt.Println("Valid reports: matched, missing, preferred, unmatched, 1g1r")
		os.Exit(1)
	}

	switch format {
	case library.FormatCSV, library.FormatJSON:
		// valid
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown format: %s\n", args[2])
		fmt.Println("Valid formats: csv, json")
		os.Exit(1)
	}

	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	exporter := library.NewExporter(database.Conn(), manager)

	data, err := exporter.Export(libraryName, reportType, format)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
		os.Exit(1)
	}

	if len(args) >= 4 {
		outputFile := args[3]
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %s %s to %s\n", reportType, format, outputFile)
	} else {
		fmt.Print(string(data))
	}
}

func exportRetroArch(libraryName, outputPath string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	exporter := library.NewRetroArchExporter(database.Conn())
	if err := exporter.ExportPlaylist(libraryName, outputPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error exporting RetroArch playlist: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Exported RetroArch playlist to %s\n", outputPath)
}
