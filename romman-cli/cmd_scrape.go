package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ryanm101/romman-lib/db"
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

	db, err := openDB(ctx)
	if err != nil {
		PrintError("Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	name, err := db.GetReleaseName(ctx, id)
	if err != nil {
		PrintError("Error: failed to get release name: %v\n", err)
		os.Exit(1)
	}

	service, err := setupMetadataService(db)
	if err != nil {
		PrintError("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scraping metadata for '%s' (ID: %d)...\n", name, id)
	start := time.Now()
	if err := service.ScrapeGame(ctx, id, name); err != nil {
		PrintError("Error: scraping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Scraped successfully in %s\n", time.Since(start))
}

func setupMetadataService(db *db.DB) (*metadata.Service, error) {
	clientID := os.Getenv("IGDB_CLIENT_ID")
	clientSecret := os.Getenv("IGDB_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("IGDB_CLIENT_ID and IGDB_CLIENT_SECRET environment variables required")
	}

	provider, err := metadata.NewIGDBProvider(clientID, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to init IGDB provider: %w", err)
	}

	homeDir, _ := os.UserHomeDir()
	mediaRoot := filepath.Join(homeDir, ".romman", "media")
	return metadata.NewService(db, provider, mediaRoot), nil
}
