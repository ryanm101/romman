package dat

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// Importer handles importing DAT files into the database.
type Importer struct {
	db *sql.DB
}

// NewImporter creates a new DAT importer with the given database connection.
func NewImporter(db *sql.DB) *Importer {
	return &Importer{db: db}
}

// ImportResult contains statistics from a DAT import operation.
type ImportResult struct {
	SystemID      int64
	SystemName    string
	GamesImported int
	RomsImported  int
	GamesSkipped  int // Already existed
	IsNewSystem   bool
}

// Import imports a DAT file into the database.
// The import is idempotent - re-importing the same DAT will update existing entries.
func (imp *Importer) Import(ctx context.Context, datPath string) (*ImportResult, error) {
	ctx, span := tracing.StartSpan(ctx, "dat.Import",
		tracing.WithAttributes(attribute.String("dat.path", datPath)),
	)
	defer span.End()

	// Parse the DAT file
	dat, err := ParseFile(datPath)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("failed to parse DAT file: %w", err)
	}

	// Detect or determine system name
	systemName := DetectSystem(dat.Header.Name, datPath)
	if systemName == "" {
		// Fall back to using the DAT header name, normalized
		systemName = normalizeSystemName(dat.Header.Name)
	}

	span.SetAttributes(
		attribute.String("system.name", systemName),
		attribute.String("dat.name", dat.Header.Name),
		attribute.String("dat.version", dat.Header.Version),
		attribute.String("dat.date", dat.Header.Date),
	)

	// Start transaction
	tx, err := imp.db.Begin()
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get or create system
	systemID, isNew, err := imp.getOrCreateSystem(ctx, tx, systemName, dat)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("failed to get/create system: %w", err)
	}

	result := &ImportResult{
		SystemID:    systemID,
		SystemName:  systemName,
		IsNewSystem: isNew,
	}

	// Import each game
	for _, game := range dat.Games {
		imported, err := imp.importGame(ctx, tx, systemID, game)
		if err != nil {
			tracing.RecordError(span, err)
			return nil, fmt.Errorf("failed to import game %q: %w", game.Name, err)
		}

		if imported {
			result.GamesImported++
			result.RomsImported += len(game.Roms)
		} else {
			result.GamesSkipped++
		}
	}

	if err := tx.Commit(); err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Record success with result attributes
	tracing.AddSpanAttributes(span,
		attribute.Int("result.games_imported", result.GamesImported),
		attribute.Int("result.games_skipped", result.GamesSkipped),
		attribute.Int("result.roms_imported", result.RomsImported),
	)
	tracing.SetSpanOK(span)

	return result, nil
}

func (imp *Importer) getOrCreateSystem(ctx context.Context, tx *sql.Tx, name string, dat *DATFile) (int64, bool, error) {
	_, span := tracing.StartSpan(ctx, "dat.getOrCreateSystem",
		tracing.WithAttributes(attribute.String("system.name", name)),
	)
	defer span.End()

	// Try to find existing system
	var id int64
	err := tx.QueryRow("SELECT id FROM systems WHERE name = ?", name).Scan(&id)
	if err == nil {
		// System exists, update DAT metadata
		_, err = tx.Exec(`
			UPDATE systems 
			SET dat_name = ?, dat_description = ?, dat_version = ?, dat_date = ?
			WHERE id = ?`,
			dat.Header.Name, dat.Header.Description, dat.Header.Version, dat.Header.Date, id)
		if err != nil {
			return 0, false, fmt.Errorf("failed to update system: %w", err)
		}
		return id, false, nil
	}

	if err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("failed to query system: %w", err)
	}

	// Create new system
	result, err := tx.Exec(`
		INSERT INTO systems (name, dat_name, dat_description, dat_version, dat_date)
		VALUES (?, ?, ?, ?, ?)`,
		name, dat.Header.Name, dat.Header.Description, dat.Header.Version, dat.Header.Date)
	if err != nil {
		return 0, false, fmt.Errorf("failed to insert system: %w", err)
	}

	id, err = result.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get system ID: %w", err)
	}

	return id, true, nil
}

func (imp *Importer) importGame(ctx context.Context, tx *sql.Tx, systemID int64, game Game) (bool, error) {
	_, span := tracing.StartSpan(ctx, "game: "+game.Name,
		tracing.WithAttributes(
			attribute.String("game.name", game.Name),
			attribute.Int64("system.id", systemID),
		),
	)
	defer span.End()

	// Check if release already exists (by system + name)
	var existingID int64
	err := tx.QueryRow(
		"SELECT id FROM releases WHERE system_id = ? AND name = ?",
		systemID, game.Name,
	).Scan(&existingID)

	if err == nil {
		// Release exists, skip (idempotent)
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to check existing release: %w", err)
	}

	// Insert the release
	result, err := tx.Exec(
		"INSERT INTO releases (system_id, name, description) VALUES (?, ?, ?)",
		systemID, game.Name, game.Description,
	)
	if err != nil {
		return false, fmt.Errorf("failed to insert release: %w", err)
	}

	releaseID, err := result.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("failed to get release ID: %w", err)
	}

	// Insert ROM entries
	for _, rom := range game.Roms {
		_, err := tx.Exec(`
			INSERT INTO rom_entries (release_id, name, sha1, crc32, md5, size)
			VALUES (?, ?, ?, ?, ?, ?)`,
			releaseID, rom.Name, rom.SHA1, rom.CRC32, rom.MD5, rom.Size,
		)
		if err != nil {
			return false, fmt.Errorf("failed to insert ROM %q: %w", rom.Name, err)
		}
	}

	return true, nil
}

// normalizeSystemName creates a simple identifier from a DAT header name
func normalizeSystemName(name string) string {
	// Use the base of the path if it looks like a path
	if name == "" {
		return "unknown"
	}
	return filepath.Base(name)
}
