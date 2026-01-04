package library

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// DuplicateType represents the kind of duplicate found.
type DuplicateType string

const (
	DuplicateExact   DuplicateType = "exact"   // Same hash (SHA1)
	DuplicateVariant DuplicateType = "variant" // Same title, different release
	DuplicatePackage DuplicateType = "package" // Multiple files for same ROM entry
)

// Duplicate represents a group of duplicate files.
type Duplicate struct {
	Type      DuplicateType
	Hash      string // For exact duplicates
	Title     string // For variant duplicates
	ReleaseID int64  // For packaging duplicates
	Files     []DuplicateFile
}

// DuplicateFile is one file in a duplicate group.
type DuplicateFile struct {
	ScannedFileID int64
	Path          string
	Size          int64
	SHA1          string
	CRC32         string
	MatchType     string // sha1, crc32, name, name_modified
	Flags         string // bad-dump, cracked, etc.
	IsPreferred   bool   // Based on match quality
}

// DuplicateFinder finds duplicates in a library.
type DuplicateFinder struct {
	db *sql.DB
}

// NewDuplicateFinder creates a new duplicate finder.
func NewDuplicateFinder(db *sql.DB) *DuplicateFinder {
	return &DuplicateFinder{db: db}
}

// FindExactDuplicates finds files with identical SHA1 hashes.
func (d *DuplicateFinder) FindExactDuplicates(ctx context.Context, libraryID int64) ([]Duplicate, error) {
	// Find SHA1 hashes that appear more than once
	rows, err := d.db.QueryContext(ctx, `
		SELECT sha1, COUNT(*) as cnt
		FROM scanned_files
		WHERE library_id = ? AND sha1 IS NOT NULL AND sha1 != ''
		GROUP BY sha1
		HAVING cnt > 1
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var hashes []string
	for rows.Next() {
		var hash string
		var cnt int
		if err := rows.Scan(&hash, &cnt); err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}

	// Get file details for each duplicate hash
	var duplicates []Duplicate
	for _, hash := range hashes {
		files, err := d.getFilesForHash(ctx, libraryID, hash)
		if err != nil {
			return nil, err
		}
		if len(files) > 1 {
			duplicates = append(duplicates, Duplicate{
				Type:  DuplicateExact,
				Hash:  hash,
				Files: files,
			})
		}
	}

	return duplicates, nil
}

// FindVariantDuplicates finds files matched to different releases of the same game.
func (d *DuplicateFinder) FindVariantDuplicates(ctx context.Context, libraryID int64) ([]Duplicate, error) {
	// Get all matched files with their release names (base title only)
	rows, err := d.db.QueryContext(ctx, `
		SELECT sf.id, sf.path, sf.size, sf.sha1, sf.crc32, 
		       m.match_type, COALESCE(m.flags, ''),
		       r.name
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		WHERE sf.library_id = ?
		ORDER BY r.name
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Group by normalized base title
	titleGroups := make(map[string][]DuplicateFile)
	titleToRelease := make(map[string]string) // Store one release name per group

	for rows.Next() {
		var file DuplicateFile
		var releaseName string
		if err := rows.Scan(&file.ScannedFileID, &file.Path, &file.Size,
			&file.SHA1, &file.CRC32, &file.MatchType, &file.Flags, &releaseName); err != nil {
			return nil, err
		}

		// Normalize the release name for grouping
		normalized := NormalizeTitleForMatching(releaseName)
		titleGroups[normalized] = append(titleGroups[normalized], file)
		if _, ok := titleToRelease[normalized]; !ok {
			titleToRelease[normalized] = releaseName
		}
	}

	// Find groups with multiple files
	var duplicates []Duplicate
	for normalized, files := range titleGroups {
		if len(files) > 1 {
			// Mark preferred file (sha1 match with no problem flags is best)
			markPreferred(files)
			duplicates = append(duplicates, Duplicate{
				Type:  DuplicateVariant,
				Title: titleToRelease[normalized],
				Files: files,
			})
		}
	}

	return duplicates, nil
}

// FindPackagingDuplicates finds multiple files matched to the same ROM entry.
func (d *DuplicateFinder) FindPackagingDuplicates(ctx context.Context, libraryID int64) ([]Duplicate, error) {
	// Find ROM entries that have multiple matched files
	rows, err := d.db.QueryContext(ctx, `
		SELECT m.rom_entry_id, COUNT(*) as cnt
		FROM matches m
		JOIN scanned_files sf ON sf.id = m.scanned_file_id
		WHERE sf.library_id = ?
		GROUP BY m.rom_entry_id
		HAVING cnt > 1
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var romEntryIDs []int64
	for rows.Next() {
		var id int64
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, err
		}
		romEntryIDs = append(romEntryIDs, id)
	}

	// Get file details for each entry
	var duplicates []Duplicate
	for _, romEntryID := range romEntryIDs {
		files, err := d.getFilesForROMEntry(ctx, libraryID, romEntryID)
		if err != nil {
			return nil, err
		}
		if len(files) > 1 {
			markPreferred(files)
			duplicates = append(duplicates, Duplicate{
				Type:      DuplicatePackage,
				ReleaseID: romEntryID,
				Files:     files,
			})
		}
	}

	return duplicates, nil
}

// FindAllDuplicates finds all types of duplicates in a library.
func (d *DuplicateFinder) FindAllDuplicates(ctx context.Context, libraryID int64) ([]Duplicate, error) {
	ctx, span := tracing.StartSpan(ctx, "library.FindDuplicates",
		tracing.WithAttributes(attribute.Int64("library.id", libraryID)),
	)
	defer span.End()

	var all []Duplicate

	exact, err := d.FindExactDuplicates(ctx, libraryID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("exact duplicates: %w", err)
	}
	all = append(all, exact...)

	variants, err := d.FindVariantDuplicates(ctx, libraryID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("variant duplicates: %w", err)
	}
	all = append(all, variants...)

	packaging, err := d.FindPackagingDuplicates(ctx, libraryID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("packaging duplicates: %w", err)
	}
	all = append(all, packaging...)

	tracing.AddSpanAttributes(span,
		attribute.Int("result.exact_count", len(exact)),
		attribute.Int("result.variant_count", len(variants)),
		attribute.Int("result.packaging_count", len(packaging)),
		attribute.Int("result.total_count", len(all)),
	)

	return all, nil
}

func (d *DuplicateFinder) getFilesForHash(ctx context.Context, libraryID int64, sha1 string) ([]DuplicateFile, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT sf.id, sf.path, sf.size, sf.sha1, sf.crc32,
		       COALESCE(m.match_type, ''), COALESCE(m.flags, '')
		FROM scanned_files sf
		LEFT JOIN matches m ON m.scanned_file_id = sf.id
		WHERE sf.library_id = ? AND sf.sha1 = ?
	`, libraryID, sha1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var files []DuplicateFile
	for rows.Next() {
		var f DuplicateFile
		if err := rows.Scan(&f.ScannedFileID, &f.Path, &f.Size, &f.SHA1, &f.CRC32,
			&f.MatchType, &f.Flags); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	markPreferred(files)
	return files, nil
}

func (d *DuplicateFinder) getFilesForROMEntry(ctx context.Context, libraryID, romEntryID int64) ([]DuplicateFile, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT sf.id, sf.path, sf.size, sf.sha1, sf.crc32,
		       m.match_type, COALESCE(m.flags, '')
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		WHERE sf.library_id = ? AND m.rom_entry_id = ?
	`, libraryID, romEntryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var files []DuplicateFile
	for rows.Next() {
		var f DuplicateFile
		if err := rows.Scan(&f.ScannedFileID, &f.Path, &f.Size, &f.SHA1, &f.CRC32,
			&f.MatchType, &f.Flags); err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, nil
}

// markPreferred marks the best file in a duplicate group as preferred.
// Priority: sha1 > crc32 > name > name_modified, then no flags > has flags
func markPreferred(files []DuplicateFile) {
	if len(files) == 0 {
		return
	}

	bestIdx := 0
	bestScore := scoreFile(files[0])

	for i := 1; i < len(files); i++ {
		score := scoreFile(files[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	files[bestIdx].IsPreferred = true
}

func scoreFile(f DuplicateFile) int {
	score := 0

	// Match type scoring
	switch f.MatchType {
	case "sha1":
		score += 100
	case "crc32":
		score += 80
	case "name":
		score += 50
	case "name_modified":
		score += 20
	}

	// Penalty for problematic flags
	if f.Flags != "" {
		score -= 10
	}

	// Prefer shorter paths (likely better organized)
	score -= len(filepath.Dir(f.Path)) / 10

	return score
}
