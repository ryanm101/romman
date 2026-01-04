package main

import (
	"context"
	"fmt"
	"os"
)

func handleDoctorCommand(ctx context.Context, args []string) {
	fmt.Println("Running database health checks...")
	database, err := openDB(ctx)
	if err != nil {
		PrintError("Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	issues := []string{}
	checks := []map[string]interface{}{}

	// Check 1: Database integrity
	var integrity string
	err = database.Conn().QueryRow("PRAGMA integrity_check").Scan(&integrity)
	dbCheck := map[string]interface{}{
		"name":   "database_integrity",
		"status": "pass",
	}
	if err != nil || integrity != "ok" {
		dbCheck["status"] = "fail"
		dbCheck["error"] = integrity
		issues = append(issues, fmt.Sprintf("Database integrity check failed: %s", integrity))
	}
	checks = append(checks, dbCheck)

	// Check 2: Orphaned matches (matches without scanned files)
	var orphanedMatches int
	err = database.Conn().QueryRow(`
		SELECT COUNT(*) FROM matches m
		LEFT JOIN scanned_files sf ON m.scanned_file_id = sf.id
		WHERE sf.id IS NULL
	`).Scan(&orphanedMatches)
	matchCheck := map[string]interface{}{
		"name":   "orphaned_matches",
		"status": "pass",
		"count":  orphanedMatches,
	}
	if err != nil {
		matchCheck["status"] = "error"
		matchCheck["error"] = err.Error()
	} else if orphanedMatches > 0 {
		matchCheck["status"] = "warn"
		issues = append(issues, fmt.Sprintf("Found %d orphaned matches", orphanedMatches))
	}
	checks = append(checks, matchCheck)

	// Check 3: Libraries with missing paths
	rows, err := database.Conn().Query(`SELECT name, root_path FROM libraries`)
	pathCheck := map[string]interface{}{
		"name":   "library_paths",
		"status": "pass",
	}
	var missingPaths []string
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var name, path string
			if err := rows.Scan(&name, &path); err == nil {
				if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
					missingPaths = append(missingPaths, name)
				}
			}
		}
	}
	if len(missingPaths) > 0 {
		pathCheck["status"] = "warn"
		pathCheck["missing"] = missingPaths
		issues = append(issues, fmt.Sprintf("Libraries with missing paths: %v", missingPaths))
	}
	checks = append(checks, pathCheck)

	// Check 4: Systems without releases
	var emptySystems int
	_ = database.Conn().QueryRow(`
		SELECT COUNT(*) FROM systems s
		LEFT JOIN releases r ON r.system_id = s.id
		GROUP BY s.id
		HAVING COUNT(r.id) = 0
	`).Scan(&emptySystems)
	systemCheck := map[string]interface{}{
		"name":   "empty_systems",
		"status": "pass",
		"count":  emptySystems,
	}
	// Ignore error for this check (might return nothing)
	checks = append(checks, systemCheck)

	result := map[string]interface{}{
		"checks": checks,
		"issues": len(issues),
		"status": "healthy",
	}

	if len(issues) > 0 {
		result["status"] = "issues_found"
	}

	if outputCfg.JSON {
		PrintResult(result)
	} else {
		fmt.Println("Database Health Check")
		fmt.Println("=====================")
		fmt.Println()

		for _, check := range checks {
			status := check["status"].(string)
			icon := "✓"
			switch status {
			case "fail":
				icon = "✗"
			case "warn":
				icon = "⚠"
			}
			fmt.Printf("%s %s: %s\n", icon, check["name"], status)
		}

		fmt.Println()
		if len(issues) == 0 {
			fmt.Println("All checks passed!")
		} else {
			fmt.Printf("Found %d issue(s):\n", len(issues))
			for _, issue := range issues {
				fmt.Printf("  - %s\n", issue)
			}
		}
	}
}
