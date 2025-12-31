package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ryanm101/romman-lib/dat"
)

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
	case "status":
		showSystemsStatus()
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

func showSystemsStatus() {
	database, err := openDB()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Conn().Query(`
		SELECT 
			s.name,
			COUNT(DISTINCT r.id) as releases,
			COUNT(DISTINCT CASE WHEN r.is_preferred = 1 THEN r.id END) as preferred,
			(SELECT COUNT(*) FROM libraries WHERE system_id = s.id) as libraries
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
	_, _ = fmt.Fprintln(w, "SYSTEM\tRELEASES\tPREFERRED\tLIBRARIES")
	_, _ = fmt.Fprintln(w, "------\t--------\t---------\t---------")

	for rows.Next() {
		var name string
		var releases, preferred, libraries int
		if err := rows.Scan(&name, &releases, &preferred, &libraries); err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", name, releases, preferred, libraries)
	}
	_ = w.Flush()
}
