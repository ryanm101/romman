package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanm101/romman-lib/dat"
	"github.com/ryanm101/romman-lib/library"
	"github.com/schollz/progressbar/v3"
)

func handleLibraryCommand(ctx context.Context, args []string) {
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
		scanLibrary(ctx, args[1])
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
			fmt.Println("Usage: romman library discover <parent-dir> [--add] [--force]")
			os.Exit(1)
		}
		autoAdd := false
		force := false
		for _, arg := range args[2:] {
			switch arg {
			case "--add":
				autoAdd = true
			case "--force":
				force = true
			}
		}
		discoverLibraries(args[1], autoAdd, force)
	case "scan-all":
		scanAllLibraries(ctx)
	case "rename":
		if len(args) < 2 {
			fmt.Println("Usage: romman library rename <name> [--dry-run]")
			os.Exit(1)
		}
		dryRun := len(args) >= 3 && args[2] == "--dry-run"
		renameLibraryFiles(args[1], dryRun)
	case "verify":
		if len(args) < 2 {
			fmt.Println("Usage: romman library verify <name>")
			os.Exit(1)
		}
		verifyLibrary(args[1])
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

	rowsData := make([][]string, 0, 20)
	jsonData := make([]map[string]interface{}, 0, 20)

	for _, lib := range libs {
		lastScan := "never"
		if lib.LastScanAt != nil {
			lastScan = lib.LastScanAt.Format("2006-01-02 15:04")
		}
		rowsData = append(rowsData, []string{lib.Name, lib.SystemName, lib.RootPath, lastScan})
		jsonData = append(jsonData, map[string]interface{}{
			"name":       lib.Name,
			"system":     lib.SystemName,
			"path":       lib.RootPath,
			"lastScanAt": lastScan,
		})
	}

	if outputCfg.JSON {
		PrintResult(jsonData)
	} else {
		PrintTable([]string{"NAME", "SYSTEM", "PATH", "LAST SCAN"}, rowsData)
	}
}

func scanLibrary(ctx context.Context, name string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	scanCfg := library.ScanConfig{
		Workers:   cfg.Scan.Workers,
		BatchSize: cfg.Scan.BatchSize,
		Parallel:  cfg.Scan.Parallel,
	}
	fmt.Printf("Scanning library: %s\n", name)

	var bar *progressbar.ProgressBar
	if !outputCfg.Quiet && !outputCfg.JSON {
		bar = progressbar.Default(-1, "Scanning")
	}

	scanCfg.OnProgress = func(p library.ScanProgress) {
		if bar != nil {
			if p.TotalFiles > 0 && bar.GetMax() == -1 {
				bar.ChangeMax64(p.TotalFiles)
			}
			_ = bar.Set64(p.FilesScanned)
		}
	}

	scanner := library.NewScannerWithConfig(database.Conn(), scanCfg)
	result, err := scanner.Scan(ctx, name)
	if bar != nil {
		_ = bar.Finish()
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error scanning library: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(result)
		return
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

	summary, err := scanner.GetSummary(name)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting library summary: %v\n", err)
		os.Exit(1)
	}

	res := map[string]interface{}{
		"library":   summary.Library.Name,
		"system":    summary.Library.SystemName,
		"path":      summary.Library.RootPath,
		"total":     summary.TotalFiles,
		"matched":   summary.MatchedFiles,
		"unmatched": summary.UnmatchedFiles,
	}
	if summary.LastScan != nil {
		res["lastScan"] = summary.LastScan.Format("2006-01-02 15:04:05")
	}

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

	res["releases"] = map[string]int{
		"total":   len(statuses),
		"present": present,
		"partial": partial,
		"missing": missing,
	}

	if outputCfg.JSON {
		PrintResult(res)
		return
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

	if outputCfg.JSON {
		PrintResult(files)
	} else {
		fmt.Printf("Unmatched files (%d):\n", len(files))
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
	}
}

func discoverLibraries(parentDir string, autoAdd bool, force bool) {
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

	type discoveredLib struct {
		name        string
		path        string
		system      string
		stubCreated bool
	}
	discovered := make([]discoveredLib, 0, 10)

	fmt.Printf("Discovering libraries in: %s\n\n", absPath)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		dirPath := filepath.Join(absPath, dirName)

		system, found := dat.DetectSystemFromDirName(dirName)
		if !found {
			fmt.Printf("  %-20s -> (unknown system, skipped)\n", dirName)
			continue
		}

		var systemExists bool
		err := database.Conn().QueryRow("SELECT 1 FROM systems WHERE name = ?", system).Scan(&systemExists)
		if err != nil {
			// System doesn't exist in DB
			if force {
				fmt.Printf("  %-20s -> %s (stub system will be created)\n", dirName, system)
				discovered = append(discovered, discoveredLib{dirName, dirPath, system, true})
			} else {
				fmt.Printf("  %-20s -> %s (no DAT imported, skipped)\n", dirName, system)
			}
			continue
		}

		fmt.Printf("  %-20s -> %s\n", dirName, system)
		discovered = append(discovered, discoveredLib{dirName, dirPath, system, false})
	}

	fmt.Printf("\nFound %d libraries\n", len(discovered))

	if !autoAdd {
		if force {
			fmt.Println("\nTo add these libraries with stub systems, run with --add --force flags:")
			fmt.Printf("  romman library discover %s --add --force\n", parentDir)
		} else {
			fmt.Println("\nTo add these libraries, run with --add flag:")
			fmt.Printf("  romman library discover %s --add\n", parentDir)
		}
		return
	}

	fmt.Println("\nAdding libraries...")
	added := 0
	stubsCreated := 0
	for _, lib := range discovered {
		// Create stub system if needed
		if lib.stubCreated {
			_, err := database.Conn().Exec(`
				INSERT OR IGNORE INTO systems (name, dat_name, dat_description)
				VALUES (?, ?, ?)
			`, lib.system, lib.system, fmt.Sprintf("Stub system for %s (no DAT imported)", lib.system))
			if err != nil {
				fmt.Printf("  %s: failed to create stub system: %v\n", lib.name, err)
				continue
			}
			stubsCreated++
		}

		_, err := manager.Add(lib.name, lib.path, lib.system)
		if err != nil {
			fmt.Printf("  %s: %v\n", lib.name, err)
			continue
		}
		if lib.stubCreated {
			fmt.Printf("  Added: %s (stub system created)\n", lib.name)
		} else {
			fmt.Printf("  Added: %s\n", lib.name)
		}
		added++
	}
	fmt.Printf("\nAdded %d libraries", added)
	if stubsCreated > 0 {
		fmt.Printf(" (%d stub systems created)", stubsCreated)
	}
	fmt.Println()
}

func scanAllLibraries(ctx context.Context) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	// scanner removed from here to be created per-library with progress bar support

	libs, err := manager.List()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error listing libraries: %v\n", err)
		os.Exit(1)
	}

	if len(libs) == 0 {
		fmt.Println("No libraries configured.")
		return
	}

	fmt.Printf("Scanning %d libraries...\n\n", len(libs))

	for _, lib := range libs {
		fmt.Printf("Scanning: %s\n", lib.Name)

		var bar *progressbar.ProgressBar
		if !outputCfg.Quiet && !outputCfg.JSON {
			bar = progressbar.Default(-1, "Scanning")
		}

		scanCfg := library.ScanConfig{
			Workers:   cfg.Scan.Workers,
			BatchSize: cfg.Scan.BatchSize,
			Parallel:  cfg.Scan.Parallel,
			OnProgress: func(p library.ScanProgress) {
				if bar != nil {
					if p.TotalFiles > 0 && bar.GetMax() == -1 {
						bar.ChangeMax64(p.TotalFiles)
					}
					_ = bar.Set64(p.FilesScanned)
				}
			},
		}

		scanner := library.NewScannerWithConfig(database.Conn(), scanCfg)
		result, err := scanner.Scan(ctx, lib.Name)
		if bar != nil {
			_ = bar.Finish()
		}

		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		fmt.Printf("  Files: %d, Matches: %d, Unmatched: %d\n",
			result.FilesScanned, result.MatchesFound, result.UnmatchedFiles)
	}

	fmt.Println("\nDone.")
}

func renameLibraryFiles(libraryName string, dryRun bool) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	renamer := library.NewRenamer(database.Conn(), manager)

	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}
	fmt.Printf("Renaming files in %s [%s]...\n\n", libraryName, mode)

	result, err := renamer.Rename(libraryName, dryRun)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, action := range result.Actions {
		switch action.Status {
		case "pending":
			fmt.Printf("  RENAME: %s\n      -> %s\n", action.OldPath, action.NewPath)
		case "done":
			fmt.Printf("  RENAMED: %s\n       -> %s\n", action.OldPath, action.NewPath)
		case "skipped":
			// Only show skipped if verbose needed
		case "error":
			fmt.Printf("  ERROR: %s: %s\n", action.OldPath, action.Error)
		}
	}

	if dryRun {
		pending := len(result.Actions) - result.Skipped
		fmt.Printf("\nWould rename: %d files\n", pending)
		fmt.Printf("Skipped: %d (already correct or target exists)\n", result.Skipped)
	} else {
		fmt.Printf("\nRenamed: %d files\n", result.Renamed)
		fmt.Printf("Skipped: %d, Errors: %d\n", result.Skipped, result.Errors)
	}
}

func verifyLibrary(libraryName string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	manager := library.NewManager(database.Conn())
	checker := library.NewIntegrityChecker(database.Conn(), manager)

	fmt.Printf("Verifying library: %s\n\n", libraryName)

	result, err := checker.Check(libraryName)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, issue := range result.Issues {
		fmt.Printf("  [%s] %s: %s\n", issue.IssueType, issue.Path, issue.Details)
	}

	fmt.Println()
	fmt.Printf("Files checked: %d\n", result.FilesChecked)
	fmt.Printf("OK: %d, Changed: %d, Missing: %d, Incomplete: %d\n",
		result.OK, result.Changed, result.Missing, result.Incomplete)

	if len(result.Issues) == 0 {
		fmt.Println("\n✓ All files verified OK")
	}
}
