package dat

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// SourceType represents the type of DAT source.
type SourceType string

const (
	SourceNoIntro SourceType = "no-intro"
	SourceRedump  SourceType = "redump"
	SourceTOSEC   SourceType = "tosec"
	SourceMAME    SourceType = "mame"
	SourceOther   SourceType = "other"
)

// DATSource represents a DAT file source in the database.
type DATSource struct {
	ID          int64
	SystemID    int64
	SourceType  SourceType
	DATName     string
	DATVersion  string
	DATDate     string
	DATFilePath string
	DATFileHash string
	Priority    int
}

// DetectSourceType determines the DAT source type from the DAT header name.
func DetectSourceType(datName string) SourceType {
	lower := strings.ToLower(datName)

	if strings.Contains(lower, "no-intro") || strings.Contains(lower, "nointro") {
		return SourceNoIntro
	}
	if strings.Contains(lower, "redump") {
		return SourceRedump
	}
	if strings.Contains(lower, "tosec") {
		return SourceTOSEC
	}
	if strings.Contains(lower, "mame") || strings.Contains(lower, "software list") {
		return SourceMAME
	}

	return SourceOther
}

// HashFile computes SHA256 hash of a file for update detection.
func HashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // Path from config, checked upstream
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetOrCreateDATSource retrieves or creates a DAT source entry.
func GetOrCreateDATSource(tx *sql.Tx, systemID int64, sourceType SourceType, dat *DATFile, datPath, datHash string) (*DATSource, bool, error) {
	var ds DATSource
	var isNew bool

	// Check if source already exists
	err := tx.QueryRow(`
		SELECT id, system_id, source_type, dat_name, dat_version, dat_date, dat_file_path, dat_file_hash, priority
		FROM dat_sources
		WHERE system_id = ? AND source_type = ?
	`, systemID, string(sourceType)).Scan(
		&ds.ID, &ds.SystemID, &ds.SourceType, &ds.DATName, &ds.DATVersion,
		&ds.DATDate, &ds.DATFilePath, &ds.DATFileHash, &ds.Priority,
	)

	if err == nil {
		// Source exists, update metadata
		_, err = tx.Exec(`
			UPDATE dat_sources
			SET dat_name = ?, dat_version = ?, dat_date = ?, dat_file_path = ?, dat_file_hash = ?, imported_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, dat.Header.Name, dat.Header.Version, dat.Header.Date, datPath, datHash, ds.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to update dat_source: %w", err)
		}

		ds.DATName = dat.Header.Name
		ds.DATVersion = dat.Header.Version
		ds.DATDate = dat.Header.Date
		ds.DATFilePath = datPath
		ds.DATFileHash = datHash
		return &ds, false, nil
	}

	if err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("failed to query dat_source: %w", err)
	}

	// Determine priority (next available)
	var maxPriority int
	_ = tx.QueryRow("SELECT COALESCE(MAX(priority), -1) FROM dat_sources WHERE system_id = ?", systemID).Scan(&maxPriority)
	priority := maxPriority + 1

	// Create new source
	result, err := tx.Exec(`
		INSERT INTO dat_sources (system_id, source_type, dat_name, dat_version, dat_date, dat_file_path, dat_file_hash, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, systemID, string(sourceType), dat.Header.Name, dat.Header.Version, dat.Header.Date, datPath, datHash, priority)
	if err != nil {
		return nil, false, fmt.Errorf("failed to insert dat_source: %w", err)
	}

	ds.ID, _ = result.LastInsertId()
	ds.SystemID = systemID
	ds.SourceType = sourceType
	ds.DATName = dat.Header.Name
	ds.DATVersion = dat.Header.Version
	ds.DATDate = dat.Header.Date
	ds.DATFilePath = datPath
	ds.DATFileHash = datHash
	ds.Priority = priority
	isNew = true

	return &ds, isNew, nil
}

// IsDATUnchanged checks if a DAT file has changed since last import.
func IsDATUnchanged(db *sql.DB, systemID int64, sourceType SourceType, currentHash string) (bool, error) {
	var storedHash string
	err := db.QueryRow(`
		SELECT dat_file_hash FROM dat_sources
		WHERE system_id = ? AND source_type = ?
	`, systemID, string(sourceType)).Scan(&storedHash)

	if err == sql.ErrNoRows {
		return false, nil // New DAT source
	}
	if err != nil {
		return false, err
	}

	return storedHash == currentHash, nil
}
