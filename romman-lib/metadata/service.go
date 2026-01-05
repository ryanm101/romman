package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ryanm101/romman-lib/db"
)

// Service manages metadata scraping and storage.
type Service struct {
	db        *db.DB
	provider  Provider
	mediaRoot string
}

// NewService creates a new metadata service.
func NewService(d *db.DB, p Provider, mediaRoot string) *Service {
	return &Service{db: d, provider: p, mediaRoot: mediaRoot}
}

// ScrapeGame searches for a game, fetches metadata, and downloads media.
func (s *Service) ScrapeGame(ctx context.Context, releaseID int64, gameName string) error {
	// 1. Search
	results, err := s.provider.Search(gameName)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no results found for %q", gameName)
	}

	// Pick first match for now
	// TODO: Add fuzzy matching or user selection logic
	best := results[0]

	// 2. Get Details
	details, err := s.provider.GetDetails(best.ID)
	if err != nil {
		return fmt.Errorf("failed to get details: %w", err)
	}

	// 3. Save Metadata
	err = s.db.SetGameMetadata(ctx, db.GameMetadata{
		ReleaseID:   releaseID,
		ProviderID:  details.ID,
		Description: details.Description,
		ReleaseDate: details.ReleaseDate,
		Developer:   details.Developer,
		Publisher:   details.Publisher,
		Rating:      details.Rating,
	})
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// 4. Download Boxart
	if details.BoxartURL != "" {
		sysName, err := s.db.GetSystemNameForRelease(ctx, releaseID)
		if err != nil {
			return fmt.Errorf("failed to get system name: %w", err)
		}

		// Clean system name for path
		sysName = filepath.Base(sysName) // Basic sanitization

		localPath := filepath.Join(s.mediaRoot, sysName, fmt.Sprintf("%d-boxart.jpg", releaseID))
		if err := s.downloadFile(details.BoxartURL, localPath); err != nil {
			return fmt.Errorf("failed to download boxart: %w", err)
		}

		err = s.db.AddGameMedia(ctx, releaseID, "boxart", details.BoxartURL, localPath)
		if err != nil {
			return fmt.Errorf("failed to save media record: %w", err)
		}
	}

	return nil
}

func (s *Service) downloadFile(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil { //nolint:gosec // Standard dir permissions
		return err
	}

	resp, err := http.Get(url) //nolint:gosec // URL from trusted IGDB API
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http error: %s", resp.Status)
	}

	f, err := os.Create(dest) //nolint:gosec // Path validated upstream
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)
	return err
}
