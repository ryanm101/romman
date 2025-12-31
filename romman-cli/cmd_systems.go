package main

import (
	"fmt"
	"os"

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

	var rowsData [][]string
	var jsonData []map[string]interface{}

	for rows.Next() {
		var name, datName string
		var releaseCount int
		if err := rows.Scan(&name, &datName, &releaseCount); err != nil {
			continue
		}
		rowsData = append(rowsData, []string{name, datName, fmt.Sprintf("%d", releaseCount)})
		jsonData = append(jsonData, map[string]interface{}{
			"name":     name,
			"datName":  datName,
			"releases": releaseCount,
		})
	}

	if outputCfg.JSON {
		PrintResult(jsonData)
	} else {
		PrintTable([]string{"SYSTEM", "DAT NAME", "RELEASES"}, rowsData)
	}
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

	res := map[string]interface{}{
		"name":        name,
		"displayName": dat.GetSystemDisplayName(name),
		"datName":     system.datName,
		"datDesc":     system.datDesc,
		"datVersion":  system.datVersion,
		"datDate":     system.datDate,
		"releases":    releaseCount,
		"roms":        romCount,
		"added":       system.createdAt,
	}

	if outputCfg.JSON {
		PrintResult(res)
	} else {
		fmt.Printf("System: %s\n", name)
		fmt.Printf("Display Name: %s\n", res["displayName"])
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

	var rowsData [][]string
	var jsonData []map[string]interface{}

	for rows.Next() {
		var name string
		var releases, preferred, libraries int
		if err := rows.Scan(&name, &releases, &preferred, &libraries); err != nil {
			continue
		}
		rowsData = append(rowsData, []string{name, fmt.Sprintf("%d", releases), fmt.Sprintf("%d", preferred), fmt.Sprintf("%d", libraries)})
		jsonData = append(jsonData, map[string]interface{}{
			"name":      name,
			"releases":  releases,
			"preferred": preferred,
			"libraries": libraries,
		})
	}

	if outputCfg.JSON {
		PrintResult(jsonData)
	} else {
		PrintTable([]string{"SYSTEM", "RELEASES", "PREFERRED", "LIBRARIES"}, rowsData)
	}
}
