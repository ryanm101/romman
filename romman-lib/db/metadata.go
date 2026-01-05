package db

import (
	"context"
	"database/sql"
	"fmt"
)

// GameMetadata represents scraped metadata for a release.
type GameMetadata struct {
	ReleaseID   int64
	ProviderID  string
	Description string
	ReleaseDate string
	Developer   string
	Publisher   string
	Rating      float64
}

// SetGameMetadata saves metadata for a release.
func (db *DB) SetGameMetadata(ctx context.Context, md GameMetadata) error {
	query := `
		INSERT INTO game_metadata (release_id, provider_id, description, release_date, developer, publisher, rating, scraped_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(release_id) DO UPDATE SET
			provider_id = excluded.provider_id,
			description = excluded.description,
			release_date = excluded.release_date,
			developer = excluded.developer,
			publisher = excluded.publisher,
			rating = excluded.rating,
			scraped_at = CURRENT_TIMESTAMP
	`
	_, err := db.conn.ExecContext(ctx, query, md.ReleaseID, md.ProviderID, md.Description, md.ReleaseDate, md.Developer, md.Publisher, md.Rating)
	if err != nil {
		return fmt.Errorf("failed to save game metadata: %w", err)
	}
	return nil
}

// GetGameMetadata retrieves metadata for a release.
func (db *DB) GetGameMetadata(ctx context.Context, releaseID int64) (*GameMetadata, error) {
	query := `
		SELECT release_id, provider_id, description, release_date, developer, publisher, rating
		FROM game_metadata WHERE release_id = ?
	`
	row := db.conn.QueryRowContext(ctx, query, releaseID)

	var md GameMetadata
	if err := row.Scan(&md.ReleaseID, &md.ProviderID, &md.Description, &md.ReleaseDate, &md.Developer, &md.Publisher, &md.Rating); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get game metadata: %w", err)
	}
	return &md, nil
}

// AddGameMedia adds a media entry for a release.
func (db *DB) AddGameMedia(ctx context.Context, releaseID int64, mediaType, url, localPath string) error {
	// Simple append, or should we replace if same type exists?
	// For Boxart usually only 1 is needed.
	// Let's delete existing of same type to keep it simple (1 boxart per game).
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM game_media WHERE release_id = ? AND type = ?", releaseID, mediaType); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO game_media (release_id, type, url, local_path) VALUES (?, ?, ?, ?)", releaseID, mediaType, url, localPath); err != nil {
		return err
	}

	return tx.Commit()
}

// GetGameMedia returns all media for a release.
func (db *DB) GetGameMedia(ctx context.Context, releaseID int64) (map[string]string, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT type, local_path FROM game_media WHERE release_id = ?", releaseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	media := make(map[string]string)
	for rows.Next() {
		var t, p string
		if err := rows.Scan(&t, &p); err != nil {
			return nil, err
		}
		media[t] = p
	}
	return media, nil
}

// GetSystemNameForRelease returns the system name for a given release ID.
func (db *DB) GetSystemNameForRelease(ctx context.Context, releaseID int64) (string, error) {
	var sysName string
	query := `
		SELECT s.name 
		FROM systems s
		JOIN releases r ON r.system_id = s.id
		WHERE r.id = ?
	`
	err := db.conn.QueryRowContext(ctx, query, releaseID).Scan(&sysName)
	if err != nil {
		return "", err
	}
	return sysName, nil
}

// GetReleaseName returns name for a release.
func (db *DB) GetReleaseName(ctx context.Context, id int64) (string, error) {
	var name string
	err := db.conn.QueryRowContext(ctx, "SELECT name FROM releases WHERE id = ?", id).Scan(&name)
	return name, err
}
