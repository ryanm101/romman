package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ryanm101/romman-lib/library"
)

func handlePreferCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman prefer <command>")
		os.Exit(1)
	}

	switch args[0] {
	case "rebuild":
		if len(args) < 2 {
			fmt.Println("Usage: romman prefer rebuild <system>")
			os.Exit(1)
		}
		rebuildPreferred(args[1])
	case "list":
		if len(args) < 2 {
			fmt.Println("Usage: romman prefer list <system>")
			os.Exit(1)
		}
		listPreferred(args[1])
	default:
		fmt.Printf("Unknown prefer command: %s\n", args[0])
		os.Exit(1)
	}
}

func rebuildPreferred(systemName string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	var systemID int64
	err = database.Conn().QueryRow("SELECT id FROM systems WHERE name = ?", systemName).Scan(&systemID)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "System not found: %s\n", systemName)
		os.Exit(1)
	}

	fmt.Printf("Rebuilding preferred releases for: %s\n", systemName)

	config := library.DefaultPreferenceConfig()
	selector := library.NewPreferenceSelector(database.Conn(), config)

	if err := selector.SelectPreferred(context.Background(), systemID); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error rebuilding preferred: %v\n", err)
		os.Exit(1)
	}

	var preferredCount, ignoredCount int
	_ = database.Conn().QueryRow(`
		SELECT COUNT(*) FROM releases WHERE system_id = ? AND is_preferred = 1
	`, systemID).Scan(&preferredCount)
	_ = database.Conn().QueryRow(`
		SELECT COUNT(*) FROM releases WHERE system_id = ? AND is_preferred = 0
	`, systemID).Scan(&ignoredCount)

	res := map[string]interface{}{
		"system":    systemName,
		"preferred": preferredCount,
		"ignored":   ignoredCount,
	}

	if outputCfg.JSON {
		PrintResult(res)
		return
	}

	fmt.Printf("\nResults:\n")
	fmt.Printf("  Preferred releases: %d\n", preferredCount)
	fmt.Printf("  Ignored variants: %d\n", ignoredCount)
}

func listPreferred(systemName string) {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	var systemID int64
	err = database.Conn().QueryRow("SELECT id FROM systems WHERE name = ?", systemName).Scan(&systemID)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "System not found: %s\n", systemName)
		os.Exit(1)
	}

	config := library.DefaultPreferenceConfig()
	selector := library.NewPreferenceSelector(database.Conn(), config)

	preferred, err := selector.GetPreferredReleases(systemID)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting preferred releases: %v\n", err)
		os.Exit(1)
	}

	if outputCfg.JSON {
		PrintResult(preferred)
		return
	}

	if len(preferred) == 0 {
		fmt.Println("No preferred releases selected yet.")
		fmt.Printf("Run: romman prefer rebuild %s\n", systemName)
		return
	}

	fmt.Printf("Preferred releases for %s (%d):\n\n", systemName, len(preferred))
	for _, r := range preferred {
		fmt.Printf("  %s\n", r.Name)
	}
}
