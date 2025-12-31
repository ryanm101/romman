package library

import (
	"database/sql"
	"fmt"
	"time"
)

// ReleaseStatus represents the status of a release in a library.
type ReleaseStatus struct {
	ReleaseName string
	ReleaseID   int64
	Status      string // "present", "missing", "partial"
	TotalROMs   int
	MatchedROMs int
}

// determineReleaseStatus returns status based on matched vs total ROMs.
func determineReleaseStatus(matched, total int) string {
	if matched == 0 {
		return "missing"
	}
	if matched == total {
		return "present"
	}
	return "partial"
}

// GetLibraryStatus returns the status of all releases for a library's system.
func (s *Scanner) GetLibraryStatus(libraryName string) ([]*ReleaseStatus, error) {
	lib, err := s.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT 
			r.id,
			r.name,
			COUNT(re.id) as total_roms,
			COUNT(m.id) as matched_roms
		FROM releases r
		JOIN rom_entries re ON re.release_id = r.id
		LEFT JOIN matches m ON m.rom_entry_id = re.id 
			AND m.scanned_file_id IN (SELECT id FROM scanned_files WHERE library_id = ?)
		WHERE r.system_id = ?
		GROUP BY r.id
		ORDER BY r.name
	`, lib.ID, lib.SystemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var statuses []*ReleaseStatus
	for rows.Next() {
		status := &ReleaseStatus{}
		if err := rows.Scan(&status.ReleaseID, &status.ReleaseName, &status.TotalROMs, &status.MatchedROMs); err != nil {
			return nil, err
		}

		status.Status = determineReleaseStatus(status.MatchedROMs, status.TotalROMs)
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetUnmatchedFiles returns files that don't match any known ROM.
func (s *Scanner) GetUnmatchedFiles(libraryName string) ([]string, error) {
	lib, err := s.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT sf.path, sf.archive_path
		FROM scanned_files sf
		LEFT JOIN matches m ON m.scanned_file_id = sf.id
		WHERE sf.library_id = ? AND m.id IS NULL
		ORDER BY sf.path
	`, lib.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var files []string
	for rows.Next() {
		var path string
		var archivePath sql.NullString
		if err := rows.Scan(&path, &archivePath); err != nil {
			return nil, err
		}

		if archivePath.Valid && archivePath.String != "" {
			files = append(files, fmt.Sprintf("%s:%s", path, archivePath.String))
		} else {
			files = append(files, path)
		}
	}

	return files, nil
}

// LibrarySummary contains summary stats for a library.
type LibrarySummary struct {
	Library        *Library
	TotalFiles     int
	MatchedFiles   int
	UnmatchedFiles int
	LastScan       *time.Time
}

// GetSummary returns a summary for a library.
func (s *Scanner) GetSummary(libraryName string) (*LibrarySummary, error) {
	lib, err := s.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	summary := &LibrarySummary{
		Library:  lib,
		LastScan: lib.LastScanAt,
	}

	// Total files
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM scanned_files WHERE library_id = ?
	`, lib.ID).Scan(&summary.TotalFiles)
	if err != nil {
		return nil, err
	}

	// Matched files
	err = s.db.QueryRow(`
		SELECT COUNT(DISTINCT scanned_file_id) FROM matches
		WHERE scanned_file_id IN (SELECT id FROM scanned_files WHERE library_id = ?)
	`, lib.ID).Scan(&summary.MatchedFiles)
	if err != nil {
		return nil, err
	}

	summary.UnmatchedFiles = summary.TotalFiles - summary.MatchedFiles

	return summary, nil
}
