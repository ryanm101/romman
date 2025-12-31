package library

import (
	"database/sql"
	"fmt"
	"path/filepath"
)

// matchResult holds the result of matching files.
type matchResult struct {
	MatchesFound   int
	UnmatchedFiles int
}

// fileToMatch represents a file to be matched.
type fileToMatch struct {
	id    int64
	sha1  string
	crc32 string
	path  string
}

// releaseNameEntry represents a ROM name from the database.
type releaseNameEntry struct {
	releaseID  int64
	romEntryID int64
	romName    string
	normalized string
}

// matchFiles matches all scanned files against known ROM entries.
func (s *Scanner) matchFiles(lib *Library) (*matchResult, error) {
	result := &matchResult{}

	// Clear existing matches for this library
	_, err := s.db.Exec(`
		DELETE FROM matches
		WHERE scanned_file_id IN (
			SELECT id FROM scanned_files WHERE library_id = ?
		)
	`, lib.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to clear matches: %w", err)
	}

	// Get all scanned files - collect them first to avoid holding rows open during writes
	rows, err := s.db.Query(`
		SELECT id, sha1, crc32, path FROM scanned_files WHERE library_id = ?
	`, lib.ID)
	if err != nil {
		return nil, err
	}

	var files []fileToMatch
	for rows.Next() {
		var f fileToMatch
		if err := rows.Scan(&f.id, &f.sha1, &f.crc32, &f.path); err != nil {
			_ = rows.Close()
			return nil, err
		}
		files = append(files, f)
	}
	_ = rows.Close()

	// Build a map of normalized release names for fuzzy matching
	releaseNames, err := s.buildReleaseNameIndex(lib.SystemID)
	if err != nil {
		return nil, fmt.Errorf("failed to build release index: %w", err)
	}

	// Now match each file
	for _, f := range files {
		matched, err := s.matchSingleFile(lib.SystemID, f, releaseNames)
		if err != nil {
			return nil, err
		}

		if matched {
			result.MatchesFound++
		} else {
			result.UnmatchedFiles++
		}
	}

	return result, nil
}

// buildReleaseNameIndex builds an index of normalized ROM names for matching.
func (s *Scanner) buildReleaseNameIndex(systemID int64) (map[string][]releaseNameEntry, error) {
	rows, err := s.db.Query(`
		SELECT r.id, re.id, re.name
		FROM rom_entries re
		JOIN releases r ON re.release_id = r.id
		WHERE r.system_id = ?
	`, systemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	index := make(map[string][]releaseNameEntry)
	for rows.Next() {
		var entry releaseNameEntry
		if err := rows.Scan(&entry.releaseID, &entry.romEntryID, &entry.romName); err != nil {
			return nil, err
		}
		entry.normalized = NormalizeTitleForMatching(entry.romName)
		index[entry.normalized] = append(index[entry.normalized], entry)
	}

	return index, nil
}

// matchSingleFile attempts to match a single file against ROM entries.
func (s *Scanner) matchSingleFile(systemID int64, f fileToMatch, releaseNames map[string][]releaseNameEntry) (bool, error) {
	// Try SHA1 match first (exact match)
	var romEntryID int64
	err := s.db.QueryRow(`
		SELECT re.id FROM rom_entries re
		JOIN releases r ON re.release_id = r.id
		WHERE r.system_id = ? AND LOWER(re.sha1) = LOWER(?)
	`, systemID, f.sha1).Scan(&romEntryID)

	if err == nil {
		// SHA1 match found - verified good dump
		return s.insertMatch(f.id, romEntryID, "sha1", "")
	}

	if err != sql.ErrNoRows {
		return false, err
	}

	// Try CRC32 fallback
	err = s.db.QueryRow(`
		SELECT re.id FROM rom_entries re
		JOIN releases r ON re.release_id = r.id
		WHERE r.system_id = ? AND LOWER(re.crc32) = LOWER(?)
	`, systemID, f.crc32).Scan(&romEntryID)

	if err == nil {
		// CRC32 match found
		return s.insertMatch(f.id, romEntryID, "crc32", "")
	}

	if err != sql.ErrNoRows {
		return false, err
	}

	// Try name-based matching
	filename := filepath.Base(f.path)
	status := ParseFilenameStatus(filename)
	normalized := NormalizeTitleForMatching(filename)

	if entries, ok := releaseNames[normalized]; ok && len(entries) > 0 {
		// Name match found - use first match
		entry := entries[0]
		flags := status.GetStatusFlags()
		matchType := "name"
		if status.IsModified() || status.IsProblematic() {
			matchType = "name_modified"
		}
		return s.insertMatch(f.id, entry.romEntryID, matchType, flags)
	}

	return false, nil
}

// insertMatch inserts a match record into the database.
func (s *Scanner) insertMatch(scannedFileID, romEntryID int64, matchType, flags string) (bool, error) {
	var flagsVal interface{}
	if flags != "" {
		flagsVal = flags
	}

	_, err := s.db.Exec(`
		INSERT INTO matches (scanned_file_id, rom_entry_id, match_type, flags)
		VALUES (?, ?, ?, ?)
	`, scannedFileID, romEntryID, matchType, flagsVal)
	return err == nil, err
}
