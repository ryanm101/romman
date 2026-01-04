package library

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// RenameAction represents a single file rename operation.
type RenameAction struct {
	OldPath string
	NewPath string
	Status  string // "pending", "done", "skipped", "error"
	Error   string
}

// RenameResult contains the outcome of a rename operation.
type RenameResult struct {
	Actions []RenameAction
	Renamed int
	Skipped int
	Errors  int
	DryRun  bool
}

// Renamer handles file renaming to match DAT names.
type Renamer struct {
	db      *sql.DB
	manager *Manager
}

// NewRenamer creates a new renamer.
func NewRenamer(db *sql.DB, manager *Manager) *Renamer {
	return &Renamer{db: db, manager: manager}
}

// Rename renames files in a library to match their DAT entry names.
func (r *Renamer) Rename(ctx context.Context, libraryName string, dryRun bool) (*RenameResult, error) {
	ctx, span := tracing.StartSpan(ctx, "library.Rename",
		tracing.WithAttributes(
			attribute.String("library.name", libraryName),
			attribute.Bool("dry_run", dryRun),
		),
	)
	defer span.End()

	lib, err := r.manager.Get(ctx, libraryName)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	result := &RenameResult{DryRun: dryRun}

	// Get all matched files with their expected names
	rows, err := r.db.QueryContext(ctx, `
		SELECT sf.id, sf.path, re.name, r.name as release_name
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		WHERE sf.library_id = ? AND sf.archive_path IS NULL
		ORDER BY sf.path
	`, lib.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var fileID int64
		var currentPath, romName, releaseName string
		if err := rows.Scan(&fileID, &currentPath, &romName, &releaseName); err != nil {
			continue
		}

		// Determine new filename
		dir := filepath.Dir(currentPath)
		ext := filepath.Ext(currentPath)

		// Use ROM name if it includes extension, otherwise use release name
		var newName string
		if strings.Contains(romName, ".") {
			newName = romName
		} else {
			newName = releaseName + ext
		}

		// Sanitize filename
		newName = sanitizeFilename(newName)
		newPath := filepath.Join(dir, newName)

		action := RenameAction{
			OldPath: currentPath,
			NewPath: newPath,
		}

		// Skip if already correctly named
		if currentPath == newPath {
			action.Status = "skipped"
			action.Error = "already named correctly"
			result.Skipped++
			result.Actions = append(result.Actions, action)
			continue
		}

		// Check if target exists
		if _, err := os.Stat(newPath); err == nil {
			action.Status = "skipped"
			action.Error = "target file exists"
			result.Skipped++
			result.Actions = append(result.Actions, action)
			continue
		}

		if dryRun {
			action.Status = "pending"
			result.Actions = append(result.Actions, action)
			continue
		}

		// Perform rename
		if err := os.Rename(currentPath, newPath); err != nil {
			action.Status = "error"
			action.Error = err.Error()
			result.Errors++
			result.Actions = append(result.Actions, action)
			continue
		}

		// Update database
		_, err = r.db.ExecContext(ctx, `
			UPDATE scanned_files SET path = ? WHERE id = ?
		`, newPath, fileID)
		if err != nil {
			action.Status = "error"
			action.Error = fmt.Sprintf("renamed but db update failed: %v", err)
			result.Errors++
		} else {
			action.Status = "done"
			result.Renamed++
		}
		result.Actions = append(result.Actions, action)
	}

	// Record results
	tracing.AddSpanAttributes(span,
		attribute.Int("result.renamed", result.Renamed),
		attribute.Int("result.skipped", result.Skipped),
		attribute.Int("result.errors", result.Errors),
	)

	return result, nil
}

// sanitizeFilename removes or replaces invalid characters.
func sanitizeFilename(name string) string {
	// Replace invalid filesystem characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", " -",
		"*", "",
		"?", "",
		"\"", "'",
		"<", "",
		">", "",
		"|", "-",
	)
	return replacer.Replace(name)
}
