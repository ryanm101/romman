package library

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// OrganizeAction represents a file organization action.
type OrganizeAction struct {
	SourcePath  string
	DestPath    string
	Action      string // "move", "copy", "rename"
	ReleaseName string
	Reason      string
}

// OrganizeOptions configures the organization behavior.
type OrganizeOptions struct {
	OutputDir     string // Destination directory
	Structure     string // "flat", "system", "system-region"
	RenameToDAT   bool   // Rename files to match DAT names
	DryRun        bool   // Preview without making changes
	MatchedOnly   bool   // Only organize matched files
	PreferredOnly bool   // Only organize preferred releases
}

// OrganizeResult contains the result of an organization operation.
type OrganizeResult struct {
	Actions   []OrganizeAction
	Moved     int
	Skipped   int
	Errors    int
	ErrorMsgs []string
}

// Organizer handles ROM file organization.
type Organizer struct {
	db      *sql.DB
	manager *Manager
}

// NewOrganizer creates a new ROM organizer.
func NewOrganizer(db *sql.DB, manager *Manager) *Organizer {
	return &Organizer{db: db, manager: manager}
}

// Plan generates an organization plan without executing it.
func (o *Organizer) Plan(ctx context.Context, libraryName string, opts OrganizeOptions) (*OrganizeResult, error) {
	lib, err := o.manager.Get(ctx, libraryName)
	if err != nil {
		return nil, err
	}

	result := &OrganizeResult{}

	// Get matched files with their release info
	query := `
		SELECT sf.path, r.name, s.name as system_name
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		JOIN systems s ON s.id = r.system_id
		WHERE sf.library_id = ?
	`
	args := []interface{}{lib.ID}

	if opts.PreferredOnly {
		query += " AND r.is_preferred = 1"
	}

	query += " GROUP BY sf.id ORDER BY r.name"

	rows, err := o.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query matched files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seen := make(map[string]bool)

	for rows.Next() {
		var srcPath, releaseName, systemName string
		if err := rows.Scan(&srcPath, &releaseName, &systemName); err != nil {
			return nil, err
		}

		// Skip duplicates
		if seen[srcPath] {
			continue
		}
		seen[srcPath] = true

		// Determine destination path
		destPath := o.buildDestPath(srcPath, releaseName, systemName, opts)

		// Skip if source and dest are the same
		if srcPath == destPath {
			result.Skipped++
			continue
		}

		action := OrganizeAction{
			SourcePath:  srcPath,
			DestPath:    destPath,
			Action:      "move",
			ReleaseName: releaseName,
			Reason:      "matched",
		}

		result.Actions = append(result.Actions, action)
	}

	return result, nil
}

// Execute performs the organization based on a plan.
func (o *Organizer) Execute(result *OrganizeResult, dryRun bool) error {
	for i := range result.Actions {
		action := &result.Actions[i]

		if dryRun {
			result.Moved++
			continue
		}

		// Create destination directory
		destDir := filepath.Dir(action.DestPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			result.Errors++
			result.ErrorMsgs = append(result.ErrorMsgs, fmt.Sprintf("failed to create dir %s: %v", destDir, err))
			continue
		}

		// Move the file
		if err := os.Rename(action.SourcePath, action.DestPath); err != nil {
			result.Errors++
			result.ErrorMsgs = append(result.ErrorMsgs, fmt.Sprintf("failed to move %s: %v", action.SourcePath, err))
			continue
		}

		result.Moved++
	}

	return nil
}

// buildDestPath constructs the destination path based on options.
func (o *Organizer) buildDestPath(srcPath, releaseName, systemName string, opts OrganizeOptions) string {
	ext := filepath.Ext(srcPath) // Preserve original extension
	baseName := filepath.Base(srcPath)

	// Determine filename
	var fileName string
	if opts.RenameToDAT {
		// Use release name from DAT, clean for filesystem
		fileName = sanitizeFilename(releaseName) + ext
	} else {
		fileName = baseName
	}

	// Determine directory structure
	var destDir string
	switch opts.Structure {
	case "system":
		destDir = filepath.Join(opts.OutputDir, systemName)
	case "system-region":
		region := extractRegion(releaseName)
		destDir = filepath.Join(opts.OutputDir, systemName, region)
	default: // "flat"
		destDir = opts.OutputDir
	}

	return filepath.Join(destDir, fileName)
}
