package library

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"

	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// ReportType defines what data to export.
type ReportType string

const (
	ReportMatched    ReportType = "matched"
	ReportMissing    ReportType = "missing"
	ReportPreferred  ReportType = "preferred"
	ReportUnmatched  ReportType = "unmatched"
	Report1G1R       ReportType = "1g1r"
	ReportStats      ReportType = "stats"
	ReportDuplicates ReportType = "duplicates"
	ReportMismatch   ReportType = "mismatch"
)

// ExportFormat defines output format.
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
	FormatTXT  ExportFormat = "txt"
)

// ExportRecord represents a single row in the export.
type ExportRecord struct {
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`
	Hash      string `json:"hash,omitempty"`
	MatchType string `json:"match_type,omitempty"`
	Flags     string `json:"flags,omitempty"`
	Status    string `json:"status,omitempty"`
}

// ExportResult contains the full export data.
type ExportResult struct {
	Library string         `json:"library"`
	System  string         `json:"system"`
	Report  string         `json:"report"`
	Count   int            `json:"count"`
	Records []ExportRecord `json:"records"`
}

// Exporter handles report generation.
type Exporter struct {
	db      *sql.DB
	manager *Manager
}

// NewExporter creates a new exporter.
func NewExporter(db *sql.DB, manager *Manager) *Exporter {
	return &Exporter{db: db, manager: manager}
}

// Export generates a report for the given library.
func (e *Exporter) Export(ctx context.Context, libraryName string, report ReportType, format ExportFormat) ([]byte, error) {
	ctx, span := tracing.StartSpan(ctx, "library.Export",
		tracing.WithAttributes(
			attribute.String("library.name", libraryName),
			attribute.String("report.type", string(report)),
			attribute.String("format", string(format)),
		),
	)
	defer span.End()

	lib, err := e.manager.Get(ctx, libraryName)
	if err != nil {
		return nil, err
	}

	result := ExportResult{
		Library: lib.Name,
		System:  lib.SystemName,
		Report:  string(report),
	}

	switch report {
	case ReportMatched:
		result.Records, err = e.getMatched(ctx, lib.ID)
	case ReportMissing:
		result.Records, err = e.getMissing(ctx, lib.ID, lib.SystemID)
	case ReportPreferred:
		result.Records, err = e.getPreferred(ctx, lib.SystemID)
	case ReportUnmatched:
		result.Records, err = e.getUnmatched(ctx, lib.ID)
	case Report1G1R:
		result.Records, err = e.get1G1R(ctx, lib.ID, lib.SystemID)
	case ReportStats:
		return e.exportStats(ctx, lib, format)
	case ReportDuplicates:
		result.Records, err = e.getDuplicates(ctx, lib.ID)
	case ReportMismatch:
		result.Records, err = e.getMismatch(ctx, lib.ID)
	default:
		return nil, fmt.Errorf("unknown report type: %s", report)
	}

	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}
	result.Count = len(result.Records)

	// Record success with result attributes
	tracing.AddSpanAttributes(span,
		attribute.Int("result.count", result.Count),
	)

	switch format {
	case FormatJSON:
		return json.MarshalIndent(result, "", "  ")
	case FormatCSV:
		return e.toCSV(result.Records, report)
	case FormatTXT:
		return e.toTXT(result.Records), nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

func (e *Exporter) getMatched(ctx context.Context, libraryID int64) ([]ExportRecord, error) {
	ctx, span := tracing.StartSpan(ctx, "export.getMatched")
	defer span.End()

	rows, err := e.db.QueryContext(ctx, `
		SELECT r.name, sf.path, sf.sha1, m.match_type, COALESCE(m.flags, '')
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		WHERE sf.library_id = ?
		ORDER BY r.name
	`, libraryID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		if err := rows.Scan(&rec.Name, &rec.Path, &rec.Hash, &rec.MatchType, &rec.Flags); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func (e *Exporter) getMissing(ctx context.Context, libraryID, systemID int64) ([]ExportRecord, error) {
	ctx, span := tracing.StartSpan(ctx, "export.getMissing")
	defer span.End()

	rows, err := e.db.QueryContext(ctx, `
		SELECT r.name
		FROM releases r
		WHERE r.system_id = ?
		AND r.id NOT IN (
			SELECT DISTINCT re.release_id
			FROM scanned_files sf
			JOIN matches m ON m.scanned_file_id = sf.id
			JOIN rom_entries re ON re.id = m.rom_entry_id
			WHERE sf.library_id = ?
		)
		ORDER BY r.name
	`, systemID, libraryID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		if err := rows.Scan(&rec.Name); err != nil {
			return nil, err
		}
		rec.Status = "missing"
		records = append(records, rec)
	}
	return records, nil
}

func (e *Exporter) getPreferred(ctx context.Context, systemID int64) ([]ExportRecord, error) {
	ctx, span := tracing.StartSpan(ctx, "export.getPreferred")
	defer span.End()

	rows, err := e.db.QueryContext(ctx, `
		SELECT name FROM releases
		WHERE system_id = ? AND is_preferred = 1
		ORDER BY name
	`, systemID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		if err := rows.Scan(&rec.Name); err != nil {
			return nil, err
		}
		rec.Status = "preferred"
		records = append(records, rec)
	}
	return records, nil
}

func (e *Exporter) getUnmatched(ctx context.Context, libraryID int64) ([]ExportRecord, error) {
	ctx, span := tracing.StartSpan(ctx, "export.getUnmatched")
	defer span.End()

	rows, err := e.db.QueryContext(ctx, `
		SELECT sf.path, sf.sha1
		FROM scanned_files sf
		LEFT JOIN matches m ON m.scanned_file_id = sf.id
		WHERE sf.library_id = ? AND m.id IS NULL
		ORDER BY sf.path
	`, libraryID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		if err := rows.Scan(&rec.Path, &rec.Hash); err != nil {
			return nil, err
		}
		rec.Name = rec.Path
		rec.Status = "unmatched"
		records = append(records, rec)
	}
	return records, nil
}

// get1G1R returns matched preferred releases - one per game (1 Game, 1 ROM).
func (e *Exporter) get1G1R(ctx context.Context, libraryID, systemID int64) ([]ExportRecord, error) {
	// Get preferred releases that are matched in this library
	// We include parent_id to group clones
	rows, err := e.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.parent_id, r.name, sf.path, sf.sha1, m.match_type
		FROM releases r
		JOIN rom_entries re ON re.release_id = r.id
		JOIN matches m ON m.rom_entry_id = re.id
		JOIN scanned_files sf ON sf.id = m.scanned_file_id
		WHERE r.system_id = ?
		  AND r.is_preferred = 1
		  AND sf.library_id = ?
		ORDER BY r.name
	`, systemID, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	type extendedRecord struct {
		ExportRecord
		id       int64
		parentID sql.NullInt64
	}

	// Group by ParentID (or ID if no parent)
	groups := make(map[int64][]extendedRecord)

	for rows.Next() {
		var rec extendedRecord
		if err := rows.Scan(&rec.id, &rec.parentID, &rec.Name, &rec.Path, &rec.Hash, &rec.MatchType); err != nil {
			return nil, err
		}
		rec.Status = "1g1r"

		groupID := rec.id
		if rec.parentID.Valid {
			groupID = rec.parentID.Int64
		}
		groups[groupID] = append(groups[groupID], rec)
	}

	var records []ExportRecord

	// Process groups to pick the best one
	for _, group := range groups {
		// Selection strategy:
		// 1. Prefer the parent release (has no parent_id set) if it's in our matched set
		// 2. Otherwise fall back to the first alphabetically (deterministic)
		//
		// The 'is_preferred=1' filter already selected our preferred regions.
		// This handles tie-breaking when multiple variants are both preferred and matched.

		best := group[0]
		for _, rec := range group {
			if !rec.parentID.Valid {
				// This is the parent release - prefer it
				best = rec
				break
			}
		}

		records = append(records, best.ExportRecord)
	}

	return records, nil
}

func (e *Exporter) toCSV(records []ExportRecord, report ReportType) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Header based on report type
	var header []string
	switch report {
	case ReportMatched, Report1G1R:
		header = []string{"name", "path", "hash", "match_type", "flags"}
	case ReportMissing, ReportPreferred:
		header = []string{"name", "status"}
	case ReportUnmatched:
		header = []string{"path", "hash", "status"}
	case ReportDuplicates:
		header = []string{"path", "hash", "matches"}
	case ReportMismatch:
		header = []string{"name", "path", "expected_hash", "actual_hash"}
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Data rows
	for _, rec := range records {
		var row []string
		switch report {
		case ReportMatched, Report1G1R:
			row = []string{rec.Name, rec.Path, rec.Hash, rec.MatchType, rec.Flags}
		case ReportMissing, ReportPreferred:
			row = []string{rec.Name, rec.Status}
		case ReportUnmatched:
			row = []string{rec.Path, rec.Hash, rec.Status}
		case ReportDuplicates:
			row = []string{rec.Path, rec.Hash, rec.Status}
		case ReportMismatch:
			row = []string{rec.Name, rec.Path, rec.Hash, rec.Status}
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// toTXT converts records to plain text format (one name per line).
// Useful for batch download lists or sharing wanted lists.
func (e *Exporter) toTXT(records []ExportRecord) []byte {
	var buf bytes.Buffer
	for _, rec := range records {
		buf.WriteString(rec.Name)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// StatsResult contains collection statistics.
type StatsResult struct {
	Library         string         `json:"library"`
	System          string         `json:"system"`
	TotalReleases   int            `json:"total_releases"`
	MatchedFiles    int            `json:"matched_files"`
	MissingReleases int            `json:"missing_releases"`
	PercentComplete float64        `json:"percent_complete"`
	RegionBreakdown map[string]int `json:"region_breakdown,omitempty"`
}

func (e *Exporter) exportStats(ctx context.Context, lib *Library, format ExportFormat) ([]byte, error) {
	stats := StatsResult{
		Library: lib.Name,
		System:  lib.SystemName,
	}

	// Total releases for system
	err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM releases WHERE system_id = ?", lib.SystemID).Scan(&stats.TotalReleases)
	if err != nil {
		return nil, err
	}

	// Matched releases (distinct)
	err = e.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT re.release_id)
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		WHERE sf.library_id = ?
	`, lib.ID).Scan(&stats.MatchedFiles)
	if err != nil {
		return nil, err
	}

	stats.MissingReleases = stats.TotalReleases - stats.MatchedFiles
	if stats.TotalReleases > 0 {
		stats.PercentComplete = float64(stats.MatchedFiles) / float64(stats.TotalReleases) * 100
	}

	// Region breakdown (parse from release names)
	stats.RegionBreakdown = make(map[string]int)
	rows, err := e.db.QueryContext(ctx, `
		SELECT r.name
		FROM releases r
		JOIN rom_entries re ON re.release_id = r.id
		JOIN matches m ON m.rom_entry_id = re.id
		JOIN scanned_files sf ON sf.id = m.scanned_file_id
		WHERE sf.library_id = ?
	`, lib.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		region := extractRegion(name)
		stats.RegionBreakdown[region]++
	}

	switch format {
	case FormatJSON:
		return json.MarshalIndent(stats, "", "  ")
	case FormatCSV:
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Library,%s\n", stats.Library)
		fmt.Fprintf(&buf, "System,%s\n", stats.System)
		fmt.Fprintf(&buf, "Total Releases,%d\n", stats.TotalReleases)
		fmt.Fprintf(&buf, "Matched,%d\n", stats.MatchedFiles)
		fmt.Fprintf(&buf, "Missing,%d\n", stats.MissingReleases)
		fmt.Fprintf(&buf, "Percent Complete,%.2f%%\n", stats.PercentComplete)
		return buf.Bytes(), nil
	case FormatTXT:
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "%s - %s\n", stats.Library, stats.System)
		fmt.Fprintf(&buf, "Complete: %.1f%% (%d/%d)\n", stats.PercentComplete, stats.MatchedFiles, stats.TotalReleases)
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

func extractRegion(name string) string {
	regions := []string{"USA", "Europe", "Japan", "World", "Germany", "France", "Spain", "Italy", "Korea", "China", "Australia"}
	for _, r := range regions {
		if containsRegion(name, r) {
			return r
		}
	}
	return "Other"
}

func containsRegion(name, region string) bool {
	// Check for (Region) pattern
	return len(name) > 0 && (hasSubstring(name, "("+region+")") || hasSubstring(name, "("+region+","))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (e *Exporter) getDuplicates(ctx context.Context, libraryID int64) ([]ExportRecord, error) {
	// Find files that match multiple releases
	rows, err := e.db.QueryContext(ctx, `
		SELECT sf.path, sf.sha1, COUNT(DISTINCT re.release_id) as match_count
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		WHERE sf.library_id = ?
		GROUP BY sf.id
		HAVING match_count > 1
		ORDER BY match_count DESC, sf.path
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		var count int
		if err := rows.Scan(&rec.Path, &rec.Hash, &count); err != nil {
			return nil, err
		}
		rec.Status = fmt.Sprintf("%d matches", count)
		records = append(records, rec)
	}
	return records, nil
}

// getMismatch finds files where the scanned hash doesn't match the expected DAT hash.
// This can indicate file corruption or incorrect file identification.
func (e *Exporter) getMismatch(ctx context.Context, libraryID int64) ([]ExportRecord, error) {
	// Find matched files where the file hash differs from expected ROM hash
	// This happens when we match by CRC32 but SHA1 differs, or vice versa
	rows, err := e.db.QueryContext(ctx, `
		SELECT r.name, sf.path, re.sha1 as expected, sf.sha1 as actual
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		WHERE sf.library_id = ?
		  AND sf.sha1 IS NOT NULL 
		  AND re.sha1 IS NOT NULL
		  AND sf.sha1 != re.sha1
		ORDER BY r.name
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []ExportRecord
	for rows.Next() {
		var rec ExportRecord
		var expected, actual string
		if err := rows.Scan(&rec.Name, &rec.Path, &expected, &actual); err != nil {
			return nil, err
		}
		rec.Hash = expected // expected hash
		rec.Status = actual // actual hash (mismatch)
		records = append(records, rec)
	}
	return records, nil
}
