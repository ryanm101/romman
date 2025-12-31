package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryanm101/romman-lib/library"
)

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
