package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/ryanm/romman-lib/dat"
	"github.com/ryanm/romman-lib/db"
	"github.com/ryanm/romman-lib/library"
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
			fmt.Println("Commands: add, list, scan, status, unmatched, discover")
			os.Exit(1)
		}
		handleLibraryCommand(os.Args[2:])
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
	fmt.Println("  duplicates list <library>           List duplicate files")
	fmt.Println("  cleanup plan <lib> <quarantine>     Generate cleanup plan")
	fmt.Println("  cleanup exec <plan> [--dry-run]     Execute cleanup plan")
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

func handleDuplicatesCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman duplicates <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		if len(args) < 2 {
			fmt.Println("Usage: romman duplicates list <library>")
			os.Exit(1)
		}
		listDuplicates(args[1])
	default:
		fmt.Printf("Unknown duplicates command: %s\n", args[0])
		os.Exit(1)
	}
}

func listDuplicates(libraryName string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	lib, err := manager.Get(libraryName)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	finder := library.NewDuplicateFinder(database.Conn())
	duplicates, err := finder.FindAllDuplicates(lib.ID)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error finding duplicates: %v\n", err)
		os.Exit(1)
	}

	if len(duplicates) == 0 {
		fmt.Println("No duplicates found.")
		return
	}

	fmt.Printf("Found %d duplicate groups:\n\n", len(duplicates))

	for i, dup := range duplicates {
		fmt.Printf("[%d] %s duplicate", i+1, dup.Type)
		if dup.Title != "" {
			fmt.Printf(" - %s", dup.Title)
		}
		if dup.Hash != "" {
			fmt.Printf(" (SHA1: %s...)", dup.Hash[:8])
		}
		fmt.Println()

		for _, file := range dup.Files {
			prefix := "  "
			if file.IsPreferred {
				prefix = "* "
			}
			flags := ""
			if file.Flags != "" {
				flags = fmt.Sprintf(" [%s]", file.Flags)
			}
			fmt.Printf("%s%s (%s)%s\n", prefix, filepath.Base(file.Path), file.MatchType, flags)
		}
		fmt.Println()
	}
}

func handleCleanupCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman cleanup <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "plan":
		if len(args) < 3 {
			fmt.Println("Usage: romman cleanup plan <library> <quarantine-dir>")
			os.Exit(1)
		}
		generateCleanupPlan(args[1], args[2])
	case "exec":
		if len(args) < 2 {
			fmt.Println("Usage: romman cleanup exec <plan-file> [--dry-run]")
			os.Exit(1)
		}
		dryRun := len(args) >= 3 && args[2] == "--dry-run"
		executeCleanupPlan(args[1], dryRun)
	default:
		fmt.Printf("Unknown cleanup command: %s\n", args[0])
		os.Exit(1)
	}
}

func generateCleanupPlan(libraryName, quarantineDir string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	finder := library.NewDuplicateFinder(database.Conn())
	planner := library.NewCleanupPlanner(finder, manager)

	absQuarantine, err := filepath.Abs(quarantineDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	plan, err := planner.GeneratePlan(libraryName, absQuarantine)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	// Save plan to file
	planFile := fmt.Sprintf("cleanup-%s-%s.json", libraryName, plan.CreatedAt.Format("20060102-150405"))
	if err := library.SavePlan(plan, planFile); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error saving plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cleanup plan generated: %s\n\n", planFile)
	fmt.Printf("Library: %s\n", plan.LibraryName)
	fmt.Printf("Quarantine: %s\n\n", plan.QuarantineDir)
	fmt.Printf("Summary:\n")
	fmt.Printf("  Total actions: %d\n", plan.Summary.TotalActions)
	fmt.Printf("  Keep (ignore): %d\n", plan.Summary.IgnoreCount)
	fmt.Printf("  Move to quarantine: %d\n", plan.Summary.MoveCount)
	fmt.Printf("  Space to reclaim: %.2f MB\n", float64(plan.Summary.SpaceReclaimed)/1024/1024)
	fmt.Println()
	fmt.Printf("To execute: romman cleanup exec %s [--dry-run]\n", planFile)
}

func executeCleanupPlan(planFile string, dryRun bool) {
	plan, err := library.LoadPlan(planFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading plan: %v\n", err)
		os.Exit(1)
	}

	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("Executing cleanup plan (%s): %s\n\n", mode, planFile)
	fmt.Printf("Library: %s\n", plan.LibraryName)
	fmt.Printf("Actions: %d\n\n", plan.Summary.TotalActions)

	if !dryRun {
		fmt.Print("This will move files to quarantine. Continue? [y/N] ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	result, err := library.ExecutePlan(plan, dryRun)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error executing plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nResults:\n")
	fmt.Printf("  Succeeded: %d\n", result.Succeeded)
	fmt.Printf("  Failed: %d\n", result.Failed)

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  %s: %s\n", e.Action.SourcePath, e.Error)
		}
	}

	if dryRun {
		fmt.Println("\n(Dry run - no files were modified)")
	}
}
