package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ryanm101/romman-lib/metadata"
)

func handleScrapeCommand(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: romman scrape <release_id>")
		os.Exit(1)
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		PrintError("Error: invalid release id: %v\n", err)
		os.Exit(1)
	}

	db, err := openDB()
	if err != nil {
		PrintError("Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	name, err := db.GetReleaseName(ctx, id)
	if err != nil {
		PrintError("Error: failed to get release name: %v\n", err)
		os.Exit(1)
	}

	clientID := os.Getenv("IGDB_CLIENT_ID")
	clientSecret := os.Getenv("IGDB_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		PrintError("Error: IGDB_CLIENT_ID and IGDB_CLIENT_SECRET environment variables required\n")
		os.Exit(1)
	}

	provider, err := metadata.NewIGDBProvider(clientID, clientSecret)
	if err != nil {
		PrintError("Error: failed to init IGDB provider: %v\n", err)
		os.Exit(1)
	}

	homeDir, _ := os.UserHomeDir()
	mediaRoot := filepath.Join(homeDir, ".romman", "media")
	service := metadata.NewService(db, provider, mediaRoot)

	fmt.Printf("Scraping metadata for '%s' (ID: %d)...\n", name, id)
	start := time.Now()
	if err := service.ScrapeGame(ctx, id, name); err != nil {
		PrintError("Error: scraping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Scraped successfully in %s\n", time.Since(start))
}
