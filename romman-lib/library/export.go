package library

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
)

// ReportType defines what data to export.
type ReportType string

const (
	ReportMatched   ReportType = "matched"
	ReportMissing   ReportType = "missing"
	ReportPreferred ReportType = "preferred"
	ReportUnmatched ReportType = "unmatched"
)

// ExportFormat defines output format.
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
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
func (e *Exporter) Export(libraryName string, report ReportType, format ExportFormat) ([]byte, error) {
	lib, err := e.manager.Get(libraryName)
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
		result.Records, err = e.getMatched(lib.ID)
	case ReportMissing:
		result.Records, err = e.getMissing(lib.ID, lib.SystemID)
	case ReportPreferred:
		result.Records, err = e.getPreferred(lib.SystemID)
	case ReportUnmatched:
		result.Records, err = e.getUnmatched(lib.ID)
	default:
		return nil, fmt.Errorf("unknown report type: %s", report)
	}

	if err != nil {
		return nil, err
	}
	result.Count = len(result.Records)

	switch format {
	case FormatJSON:
		return json.MarshalIndent(result, "", "  ")
	case FormatCSV:
		return e.toCSV(result.Records, report)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

func (e *Exporter) getMatched(libraryID int64) ([]ExportRecord, error) {
	rows, err := e.db.Query(`
		SELECT r.name, sf.path, sf.sha1, m.match_type, COALESCE(m.flags, '')
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

func (e *Exporter) getMissing(libraryID, systemID int64) ([]ExportRecord, error) {
	rows, err := e.db.Query(`
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

func (e *Exporter) getPreferred(systemID int64) ([]ExportRecord, error) {
	rows, err := e.db.Query(`
		SELECT name FROM releases
		WHERE system_id = ? AND is_preferred = 1
		ORDER BY name
	`, systemID)
	if err != nil {
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

func (e *Exporter) getUnmatched(libraryID int64) ([]ExportRecord, error) {
	rows, err := e.db.Query(`
		SELECT sf.path, sf.sha1
		FROM scanned_files sf
		LEFT JOIN matches m ON m.scanned_file_id = sf.id
		WHERE sf.library_id = ? AND m.id IS NULL
		ORDER BY sf.path
	`, libraryID)
	if err != nil {
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

func (e *Exporter) toCSV(records []ExportRecord, report ReportType) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Header based on report type
	var header []string
	switch report {
	case ReportMatched:
		header = []string{"name", "path", "hash", "match_type", "flags"}
	case ReportMissing, ReportPreferred:
		header = []string{"name", "status"}
	case ReportUnmatched:
		header = []string{"path", "hash", "status"}
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Data rows
	for _, rec := range records {
		var row []string
		switch report {
		case ReportMatched:
			row = []string{rec.Name, rec.Path, rec.Hash, rec.MatchType, rec.Flags}
		case ReportMissing, ReportPreferred:
			row = []string{rec.Name, rec.Status}
		case ReportUnmatched:
			row = []string{rec.Path, rec.Hash, rec.Status}
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}
