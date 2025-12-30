package library

import (
	"archive/zip"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScanResult contains statistics from a library scan.
type ScanResult struct {
	FilesScanned   int
	FilesHashed    int
	FilesSkipped   int // Unchanged files (hash cached)
	MatchesFound   int
	UnmatchedFiles int
}

// ScannedFile represents a file found during scanning.
type ScannedFile struct {
	ID          int64
	LibraryID   int64
	Path        string
	Size        int64
	Mtime       int64
	SHA1        string
	CRC32       string
	ArchivePath string // Path within zip, empty for regular files
}

// Scanner handles library scanning operations.
type Scanner struct {
	db      *sql.DB
	manager *Manager
}

// NewScanner creates a new library scanner.
func NewScanner(db *sql.DB) *Scanner {
	return &Scanner{
		db:      db,
		manager: NewManager(db),
	}
}

// Scan scans a library for ROM files and matches them against the database.
func (s *Scanner) Scan(libraryName string) (*ScanResult, error) {
	lib, err := s.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{}

	// Walk the library directory
	err = filepath.Walk(lib.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Handle zip files
		if ext == ".zip" {
			zipResult, err := s.scanZipFile(lib, path, info)
			if err != nil {
				// Log error but continue scanning
				fmt.Printf("Warning: failed to scan zip %s: %v\n", path, err)
				return nil
			}
			result.FilesScanned += zipResult.FilesScanned
			result.FilesHashed += zipResult.FilesHashed
			result.FilesSkipped += zipResult.FilesSkipped
			return nil
		}

		// Handle regular ROM files
		scanned, hashed, err := s.scanFile(lib, path, info, "")
		if err != nil {
			fmt.Printf("Warning: failed to scan %s: %v\n", path, err)
			return nil
		}
		result.FilesScanned++
		if hashed {
			result.FilesHashed++
		} else if scanned {
			result.FilesSkipped++
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk library: %w", err)
	}

	// Perform matching
	matchResult, err := s.matchFiles(lib)
	if err != nil {
		return nil, fmt.Errorf("failed to match files: %w", err)
	}
	result.MatchesFound = matchResult.MatchesFound
	result.UnmatchedFiles = matchResult.UnmatchedFiles

	// Update last scan time
	if err := s.manager.UpdateLastScan(lib.ID); err != nil {
		return nil, fmt.Errorf("failed to update scan time: %w", err)
	}

	return result, nil
}

func (s *Scanner) scanZipFile(lib *Library, zipPath string, zipInfo os.FileInfo) (*ScanResult, error) {
	result := &ScanResult{}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Use zip file's mtime for cache key
		mtime := zipInfo.ModTime().Unix()
		size := int64(f.UncompressedSize64)

		scanned, hashed, err := s.scanZipEntry(lib, zipPath, f, mtime, size)
		if err != nil {
			fmt.Printf("Warning: failed to scan zip entry %s: %v\n", f.Name, err)
			continue
		}

		result.FilesScanned++
		if hashed {
			result.FilesHashed++
		} else if scanned {
			result.FilesSkipped++
		}
	}

	return result, nil
}

func (s *Scanner) scanFile(lib *Library, path string, info os.FileInfo, archivePath string) (scanned, hashed bool, err error) {
	mtime := info.ModTime().Unix()
	size := info.Size()

	// Check if we already have this file cached
	cached, err := s.getCachedFile(lib.ID, path, archivePath, size, mtime)
	if err != nil {
		return false, false, err
	}
	if cached != nil {
		// File unchanged, use cached hashes
		return true, false, nil
	}

	// Need to hash the file
	f, err := os.Open(path)
	if err != nil {
		return false, false, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	sha1Hash, crc32Hash, err := computeHashes(f)
	if err != nil {
		return false, false, fmt.Errorf("failed to hash file: %w", err)
	}

	// Store in database
	if err := s.storeScannedFile(lib.ID, path, archivePath, size, mtime, sha1Hash, crc32Hash); err != nil {
		return false, false, fmt.Errorf("failed to store scanned file: %w", err)
	}

	return true, true, nil
}

func (s *Scanner) scanZipEntry(lib *Library, zipPath string, f *zip.File, mtime, size int64) (scanned, hashed bool, err error) {
	archivePath := f.Name

	// Check cache
	cached, err := s.getCachedFile(lib.ID, zipPath, archivePath, size, mtime)
	if err != nil {
		return false, false, err
	}
	if cached != nil {
		return true, false, nil
	}

	// Hash the zip entry
	rc, err := f.Open()
	if err != nil {
		return false, false, fmt.Errorf("failed to open zip entry: %w", err)
	}
	defer func() { _ = rc.Close() }()

	sha1Hash, crc32Hash, err := computeHashes(rc)
	if err != nil {
		return false, false, fmt.Errorf("failed to hash zip entry: %w", err)
	}

	// Store in database
	if err := s.storeScannedFile(lib.ID, zipPath, archivePath, size, mtime, sha1Hash, crc32Hash); err != nil {
		return false, false, fmt.Errorf("failed to store scanned file: %w", err)
	}

	return true, true, nil
}

func computeHashes(r io.Reader) (sha1Hex, crc32Hex string, err error) {
	sha1Hasher := sha1.New()
	crc32Hasher := crc32.NewIEEE()
	multiWriter := io.MultiWriter(sha1Hasher, crc32Hasher)

	if _, err := io.Copy(multiWriter, r); err != nil {
		return "", "", err
	}

	sha1Hex = hex.EncodeToString(sha1Hasher.Sum(nil))
	crc32Hex = fmt.Sprintf("%08x", crc32Hasher.Sum32())

	return sha1Hex, crc32Hex, nil
}

func (s *Scanner) getCachedFile(libraryID int64, path, archivePath string, size, mtime int64) (*ScannedFile, error) {
	sf := &ScannedFile{}
	var archivePathNull sql.NullString

	query := `
		SELECT id, library_id, path, size, mtime, sha1, crc32, archive_path
		FROM scanned_files
		WHERE library_id = ? AND path = ? AND COALESCE(archive_path, '') = ? AND size = ? AND mtime = ?
	`
	err := s.db.QueryRow(query, libraryID, path, archivePath, size, mtime).Scan(
		&sf.ID, &sf.LibraryID, &sf.Path, &sf.Size, &sf.Mtime, &sf.SHA1, &sf.CRC32, &archivePathNull,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if archivePathNull.Valid {
		sf.ArchivePath = archivePathNull.String
	}

	return sf, nil
}

func (s *Scanner) storeScannedFile(libraryID int64, path, archivePath string, size, mtime int64, sha1Hash, crc32Hash string) error {
	var archivePathVal interface{}
	if archivePath != "" {
		archivePathVal = archivePath
	}

	_, err := s.db.Exec(`
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1, crc32, archive_path)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(library_id, path, archive_path) DO UPDATE SET
			size = excluded.size,
			mtime = excluded.mtime,
			sha1 = excluded.sha1,
			crc32 = excluded.crc32,
			scanned_at = CURRENT_TIMESTAMP
	`, libraryID, path, size, mtime, sha1Hash, crc32Hash, archivePathVal)

	return err
}

type matchResult struct {
	MatchesFound   int
	UnmatchedFiles int
}

type fileToMatch struct {
	id    int64
	sha1  string
	crc32 string
}

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
		SELECT id, sha1, crc32 FROM scanned_files WHERE library_id = ?
	`, lib.ID)
	if err != nil {
		return nil, err
	}

	var files []fileToMatch
	for rows.Next() {
		var f fileToMatch
		if err := rows.Scan(&f.id, &f.sha1, &f.crc32); err != nil {
			_ = rows.Close()
			return nil, err
		}
		files = append(files, f)
	}
	_ = rows.Close()

	// Now match each file
	for _, f := range files {
		matched, err := s.matchSingleFile(lib.SystemID, f.id, f.sha1, f.crc32)
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

func (s *Scanner) matchSingleFile(systemID, fileID int64, sha1Hash, crc32Hash string) (bool, error) {
	// Try SHA1 match first
	var romEntryID int64
	err := s.db.QueryRow(`
		SELECT re.id FROM rom_entries re
		JOIN releases r ON re.release_id = r.id
		WHERE r.system_id = ? AND LOWER(re.sha1) = LOWER(?)
	`, systemID, sha1Hash).Scan(&romEntryID)

	if err == nil {
		// SHA1 match found
		_, err = s.db.Exec(`
			INSERT INTO matches (scanned_file_id, rom_entry_id, match_type)
			VALUES (?, ?, 'sha1')
		`, fileID, romEntryID)
		return err == nil, err
	}

	if err != sql.ErrNoRows {
		return false, err
	}

	// Try CRC32 fallback
	err = s.db.QueryRow(`
		SELECT re.id FROM rom_entries re
		JOIN releases r ON re.release_id = r.id
		WHERE r.system_id = ? AND LOWER(re.crc32) = LOWER(?)
	`, systemID, crc32Hash).Scan(&romEntryID)

	if err == nil {
		// CRC32 match found
		_, err = s.db.Exec(`
			INSERT INTO matches (scanned_file_id, rom_entry_id, match_type)
			VALUES (?, ?, 'crc32')
		`, fileID, romEntryID)
		return err == nil, err
	}

	if err == sql.ErrNoRows {
		return false, nil
	}

	return false, err
}

// GetStatus returns the status of releases for a library.
type ReleaseStatus struct {
	ReleaseName string
	ReleaseID   int64
	Status      string // "present", "missing", "partial"
	TotalROMs   int
	MatchedROMs int
}

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

// Summary returns a summary of library stats.
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
