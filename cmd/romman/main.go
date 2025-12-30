package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/ryanm/romman/internal/dat"
	"github.com/ryanm/romman/internal/db"
	"github.com/ryanm/romman/internal/library"
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
	case "library":
		if len(os.Args) < 3 {
			fmt.Println("Usage: romman library <command>")
			fmt.Println("Commands: add, list, scan, status, unmatched")
			os.Exit(1)
		}
		handleLibraryCommand(os.Args[2:])
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
	fmt.Println("  systems list                        List all systems")
	fmt.Println("  systems info <name>                 Show system details")
	fmt.Println("  library add <name> <path> <system>  Add a library")
	fmt.Println("  library list                        List all libraries")
	fmt.Println("  library discover <dir> [--add]      Auto-detect libraries from subdirs")
	fmt.Println("  library scan <name>                 Scan a library for ROMs")
	fmt.Println("  library status <name>               Show release status")
	fmt.Println("  library unmatched <name>            Show unmatched files")
	fmt.Println("  help                                Show this help")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  ROMMAN_DB                           Database path (default: romman.db)")
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

func handleLibraryCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman library <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		if len(args) < 4 {
			fmt.Println("Usage: romman library add <name> <path> <system>")
			os.Exit(1)
		}
		addLibrary(args[1], args[2], args[3])
	case "list":
		listLibraries()
	case "scan":
		if len(args) < 2 {
			fmt.Println("Usage: romman library scan <name>")
			os.Exit(1)
		}
		scanLibrary(args[1])
	case "status":
		if len(args) < 2 {
			fmt.Println("Usage: romman library status <name>")
			os.Exit(1)
		}
		showLibraryStatus(args[1])
	case "unmatched":
		if len(args) < 2 {
			fmt.Println("Usage: romman library unmatched <name>")
			os.Exit(1)
		}
		showUnmatchedFiles(args[1])
	case "discover":
		if len(args) < 2 {
			fmt.Println("Usage: romman library discover <parent-dir> [--add]")
			os.Exit(1)
		}
		autoAdd := len(args) >= 3 && args[2] == "--add"
		discoverLibraries(args[1], autoAdd)
	default:
		fmt.Printf("Unknown library command: %s\n", args[0])
		os.Exit(1)
	}
}

func addLibrary(name, path, system string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	absPath, err := filepath.Abs(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Verify path exists
	info, err := os.Stat(absPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: path does not exist: %s\n", absPath)
		os.Exit(1)
	}
	if !info.IsDir() {
		_, _ = fmt.Fprintf(os.Stderr, "Error: path is not a directory: %s\n", absPath)
		os.Exit(1)
	}

	manager := library.NewManager(database.Conn())
	lib, err := manager.Add(name, absPath, system)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error adding library: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Library added: %s\n", lib.Name)
	fmt.Printf("  Path: %s\n", lib.RootPath)
	fmt.Printf("  System: %s\n", lib.SystemName)
}

func listLibraries() {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	libs, err := manager.List()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error listing libraries: %v\n", err)
		os.Exit(1)
	}

	if len(libs) == 0 {
		fmt.Println("No libraries configured.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSYSTEM\tPATH\tLAST SCAN")
	_, _ = fmt.Fprintln(w, "----\t------\t----\t---------")

	for _, lib := range libs {
		lastScan := "never"
		if lib.LastScanAt != nil {
			lastScan = lib.LastScanAt.Format("2006-01-02 15:04")
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", lib.Name, lib.SystemName, lib.RootPath, lastScan)
	}
	_ = w.Flush()
}

func scanLibrary(name string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	scanner := library.NewScanner(database.Conn())

	fmt.Printf("Scanning library: %s\n", name)
	result, err := scanner.Scan(name)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error scanning library: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Files scanned: %d\n", result.FilesScanned)
	fmt.Printf("Files hashed: %d\n", result.FilesHashed)
	fmt.Printf("Files cached: %d\n", result.FilesSkipped)
	fmt.Println()
	fmt.Printf("Matches found: %d\n", result.MatchesFound)
	fmt.Printf("Unmatched files: %d\n", result.UnmatchedFiles)
}

func showLibraryStatus(name string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	scanner := library.NewScanner(database.Conn())

	// Get summary first
	summary, err := scanner.GetSummary(name)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting library summary: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Library: %s\n", summary.Library.Name)
	fmt.Printf("System: %s\n", summary.Library.SystemName)
	fmt.Printf("Path: %s\n", summary.Library.RootPath)
	if summary.LastScan != nil {
		fmt.Printf("Last Scan: %s\n", summary.LastScan.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Last Scan: never")
	}
	fmt.Println()
	fmt.Printf("Total Files: %d\n", summary.TotalFiles)
	fmt.Printf("Matched: %d\n", summary.MatchedFiles)
	fmt.Printf("Unmatched: %d\n", summary.UnmatchedFiles)

	// Get release status breakdown
	statuses, err := scanner.GetLibraryStatus(name)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting library status: %v\n", err)
		os.Exit(1)
	}

	var present, missing, partial int
	for _, s := range statuses {
		switch s.Status {
		case "present":
			present++
		case "missing":
			missing++
		case "partial":
			partial++
		}
	}

	fmt.Println()
	fmt.Printf("Releases: %d total\n", len(statuses))
	fmt.Printf("  Present: %d\n", present)
	fmt.Printf("  Partial: %d\n", partial)
	fmt.Printf("  Missing: %d\n", missing)
}

func showUnmatchedFiles(name string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	scanner := library.NewScanner(database.Conn())
	files, err := scanner.GetUnmatchedFiles(name)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting unmatched files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No unmatched files.")
		return
	}

	fmt.Printf("Unmatched files (%d):\n", len(files))
	for _, f := range files {
		fmt.Printf("  %s\n", f)
	}
}

func discoverLibraries(parentDir string, autoAdd bool) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	absPath, err := filepath.Abs(parentDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
		os.Exit(1)
	}

	manager := library.NewManager(database.Conn())

	var discovered []struct {
		name   string
		path   string
		system string
	}

	fmt.Printf("Discovering libraries in: %s\n\n", absPath)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		dirPath := filepath.Join(absPath, dirName)

		// Try to detect system from directory name
		system, found := dat.DetectSystemFromDirName(dirName)
		if !found {
			fmt.Printf("  %-20s -> (unknown system, skipped)\n", dirName)
			continue
		}

		// Check if system exists in database
		var systemExists bool
		err := database.Conn().QueryRow("SELECT 1 FROM systems WHERE name = ?", system).Scan(&systemExists)
		if err != nil {
			fmt.Printf("  %-20s -> %s (no DAT imported, skipped)\n", dirName, system)
			continue
		}

		fmt.Printf("  %-20s -> %s\n", dirName, system)
		discovered = append(discovered, struct {
			name   string
			path   string
			system string
		}{dirName, dirPath, system})
	}

	fmt.Printf("\nFound %d libraries\n", len(discovered))

	if !autoAdd {
		fmt.Println("\nTo add these libraries, run with --add flag:")
		fmt.Printf("  romman library discover %s --add\n", parentDir)
		return
	}

	fmt.Println("\nAdding libraries...")
	added := 0
	for _, lib := range discovered {
		_, err := manager.Add(lib.name, lib.path, lib.system)
		if err != nil {
			fmt.Printf("  %s: %v\n", lib.name, err)
			continue
		}
		fmt.Printf("  Added: %s\n", lib.name)
		added++
	}
	fmt.Printf("\nAdded %d libraries\n", added)
}
