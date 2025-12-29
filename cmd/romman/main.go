package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/ryanm/romman/internal/dat"
	"github.com/ryanm/romman/internal/db"
)

const defaultDBPath = "romman.db"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "dat":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman dat <command>")
			fmt.Println("Commands: import")
			os.Exit(1)
		}
		handleDatCommand(os.Args[2:])
	case "systems":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman systems <command>")
			fmt.Println("Commands: list, info")
			os.Exit(1)
		}
		handleSystemsCommand(os.Args[2:])
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
	fmt.Println("  dat import <file>     Import a DAT file")
	fmt.Println("  systems list          List all systems")
	fmt.Println("  systems info <name>   Show system details")
	fmt.Println("  help                  Show this help")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  ROMMAN_DB             Database path (default: romman.db)")
}

func getDBPath() string {
	if path := os.Getenv("ROMMAN_DB"); path != "" {
		return path
	}
	return defaultDBPath
}

func openDB() (*db.DB, error) {
	return db.Open(getDBPath())
}

func handleDatCommand(args []string) {
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
		importDATs(args[1:])
	default:
		fmt.Printf("Unknown dat command: %s\n", args[0])
		os.Exit(1)
	}
}

func importDATs(paths []string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	importer := dat.NewImporter(database.Conn())

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error resolving path %s: %v\n", path, err)
			continue
		}

		fmt.Printf("Importing %s...\n", filepath.Base(path))
		result, err := importer.Import(absPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			continue
		}

		status := "updated"
		if result.IsNewSystem {
			status = "created"
		}
		fmt.Printf("  System: %s (%s)\n", result.SystemName, status)
		fmt.Printf("  Games imported: %d, ROMs: %d, Skipped: %d\n",
			result.GamesImported, result.RomsImported, result.GamesSkipped)
	}
}

func handleSystemsCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman systems <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		listSystems()
	case "info":
		if len(args) < 2 {
			fmt.Println("Usage: romman systems info <name>")
			os.Exit(1)
		}
		showSystemInfo(args[1])
	default:
		fmt.Printf("Unknown systems command: %s\n", args[0])
		os.Exit(1)
	}
}

func listSystems() {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Conn().Query(`
		SELECT s.name, s.dat_name, COUNT(r.id) as release_count
		FROM systems s
		LEFT JOIN releases r ON r.system_id = s.id
		GROUP BY s.id
		ORDER BY s.name
	`)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error querying systems: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = rows.Close() }()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SYSTEM\tDAT NAME\tRELEASES")
	_, _ = fmt.Fprintln(w, "------\t--------\t--------")

	for rows.Next() {
		var name, datName string
		var releaseCount int
		if err := rows.Scan(&name, &datName, &releaseCount); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error reading row: %v\n", err)
			continue
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", name, datName, releaseCount)
	}
	_ = w.Flush()
}

func showSystemInfo(name string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	var system struct {
		id         int64
		datName    string
		datDesc    string
		datVersion string
		datDate    string
		createdAt  string
	}

	err = database.Conn().QueryRow(`
		SELECT id, dat_name, dat_description, dat_version, dat_date, created_at
		FROM systems WHERE name = ?
	`, name).Scan(&system.id, &system.datName, &system.datDesc, &system.datVersion, &system.datDate, &system.createdAt)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "System not found: %s\n", name)
		os.Exit(1)
	}

	var releaseCount, romCount int
	_ = database.Conn().QueryRow(`
		SELECT COUNT(*) FROM releases WHERE system_id = ?
	`, system.id).Scan(&releaseCount)
	_ = database.Conn().QueryRow(`
		SELECT COUNT(*) FROM rom_entries re
		JOIN releases r ON re.release_id = r.id
		WHERE r.system_id = ?
	`, system.id).Scan(&romCount)

	fmt.Printf("System: %s\n", name)
	fmt.Printf("Display Name: %s\n", dat.GetSystemDisplayName(name))
	fmt.Println()
	fmt.Printf("DAT Name: %s\n", system.datName)
	fmt.Printf("DAT Description: %s\n", system.datDesc)
	fmt.Printf("DAT Version: %s\n", system.datVersion)
	fmt.Printf("DAT Date: %s\n", system.datDate)
	fmt.Println()
	fmt.Printf("Releases: %d\n", releaseCount)
	fmt.Printf("ROM Entries: %d\n", romCount)
	fmt.Printf("Added: %s\n", system.createdAt)
}
