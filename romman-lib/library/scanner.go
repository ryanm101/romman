package library

import (
	"archive/zip"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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

// ScanConfig configures parallel scanning behavior.
type ScanConfig struct {
	Workers   int  // Number of parallel workers (default: NumCPU)
	BatchSize int  // Number of files per transaction batch (default: 100)
	Parallel  bool // Use parallel scanning (default: true)
}

// DefaultScanConfig returns sensible defaults for scanning.
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		Workers:   runtime.NumCPU(),
		BatchSize: 100,
		Parallel:  true,
	}
}

// ignoredExtensions contains file extensions that should be skipped during scanning.
// These are typically save files, state files, or other non-ROM data.
var ignoredExtensions = map[string]bool{
	// Save files
	".srm": true, // SRAM save
	".sav": true, // Generic save
	".eep": true, // EEPROM save
	".fla": true, // Flash save
	".rtc": true, // Real-time clock

	// State files
	".state": true, // Save state
	".st0":   true, // Save state slot 0
	".st1":   true, // Save state slot 1
	".st2":   true, // Save state slot 2
	".st3":   true, // Save state slot 3
	".st4":   true, // Save state slot 4
	".st5":   true, // Save state slot 5
	".st6":   true, // Save state slot 6
	".st7":   true, // Save state slot 7
	".st8":   true, // Save state slot 8
	".st9":   true, // Save state slot 9
	".oops":  true, // RetroArch undo state

	// Thumbnails and metadata
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".txt":  true,
	".nfo":  true,
	".xml":  true,
	".json": true,

	// Playlists and config
	".cfg": true,
	".lpl": true, // RetroArch playlist
	".opt": true, // Core options
}

// isIgnoredExtension returns true if the file extension should be skipped.
func isIgnoredExtension(ext string) bool {
	return ignoredExtensions[ext]
}

// Scanner handles library scanning operations.
type Scanner struct {
	db      *sql.DB
	manager *Manager
	config  ScanConfig
}

// NewScanner creates a new library scanner with default config.
func NewScanner(db *sql.DB) *Scanner {
	return &Scanner{
		db:      db,
		manager: NewManager(db),
		config:  DefaultScanConfig(),
	}
}

// NewScannerWithConfig creates a scanner with custom configuration.
func NewScannerWithConfig(db *sql.DB, config ScanConfig) *Scanner {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	return &Scanner{
		db:      db,
		manager: NewManager(db),
		config:  config,
	}
}

// Scan scans a library for ROM files and matches them against the database.
// If parallel scanning is enabled in config, uses a worker pool for hashing.
func (s *Scanner) Scan(libraryName string) (*ScanResult, error) {
	lib, err := s.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	if s.config.Parallel && s.config.Workers > 1 {
		return s.scanParallel(lib)
	}
	return s.scanSequential(lib)
}

// fileJob represents a file to be hashed.
type fileJob struct {
	path        string
	archivePath string
	size        int64
	mtime       int64
	isZipEntry  bool
	isCHD       bool
	zipPath     string
}

// hashResult contains the result of hashing a file.
type hashResult struct {
	job       fileJob
	sha1      string
	crc32     string
	wasHashed bool // true if newly hashed, false if cache hit
	err       error
}

// scanParallel performs parallel file discovery and hashing.
func (s *Scanner) scanParallel(lib *Library) (*ScanResult, error) {
	// Channels for worker pool
	jobs := make(chan fileJob, s.config.Workers*10)
	results := make(chan hashResult, s.config.Workers*10)

	// Atomic counters for results
	var filesScanned, filesHashed, filesSkipped int64

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < s.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.hashWorker(lib.ID, jobs, results)
		}()
	}

	// Collector goroutine - batches results for DB writes
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	var collectorErr error
	go func() {
		defer collectorWg.Done()
		batch := make([]hashResult, 0, s.config.BatchSize)

		for r := range results {
			if r.err != nil {
				slog.Warn("failed to hash file", "path", r.job.path, "error", r.err)
				continue
			}

			atomic.AddInt64(&filesScanned, 1)
			if r.wasHashed {
				atomic.AddInt64(&filesHashed, 1)
			} else {
				atomic.AddInt64(&filesSkipped, 1)
			}

			batch = append(batch, r)
			if len(batch) >= s.config.BatchSize {
				if err := s.storeBatch(lib.ID, batch); err != nil {
					collectorErr = err
					return
				}
				batch = batch[:0]
			}
		}

		// Flush remaining batch
		if len(batch) > 0 {
			if err := s.storeBatch(lib.ID, batch); err != nil {
				collectorErr = err
			}
		}
	}()

	// Walk and discover files (producer)
	err := filepath.Walk(lib.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if isIgnoredExtension(ext) {
			return nil
		}

		if ext == ".zip" {
			// Queue zip entries
			if err := s.queueZipEntries(path, info, jobs); err != nil {
				slog.Warn("failed to open zip", "path", path, "error", err)
			}
			return nil
		}

		// Check for CHD files
		isCHD := ext == ".chd"

		// Queue regular file
		jobs <- fileJob{
			path:  path,
			size:  info.Size(),
			mtime: info.ModTime().Unix(),
			isCHD: isCHD,
		}
		return nil
	})

	close(jobs)
	wg.Wait()
	close(results)
	collectorWg.Wait()

	if err != nil {
		return nil, fmt.Errorf("failed to walk library: %w", err)
	}
	if collectorErr != nil {
		return nil, fmt.Errorf("failed to store results: %w", collectorErr)
	}

	// Cleanup and matching
	if err := s.cleanupStaleFiles(lib); err != nil {
		return nil, fmt.Errorf("failed to cleanup stale files: %w", err)
	}

	matchResult, err := s.matchFiles(lib)
	if err != nil {
		return nil, fmt.Errorf("failed to match files: %w", err)
	}

	if err := s.manager.UpdateLastScan(lib.ID); err != nil {
		return nil, fmt.Errorf("failed to update scan time: %w", err)
	}

	return &ScanResult{
		FilesScanned:   int(filesScanned),
		FilesHashed:    int(filesHashed),
		FilesSkipped:   int(filesSkipped),
		MatchesFound:   matchResult.MatchesFound,
		UnmatchedFiles: matchResult.UnmatchedFiles,
	}, nil
}

// queueZipEntries reads a zip file and queues its entries for hashing.
func (s *Scanner) queueZipEntries(zipPath string, zipInfo os.FileInfo, jobs chan<- fileJob) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	mtime := zipInfo.ModTime().Unix()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		jobs <- fileJob{
			path:        zipPath,
			archivePath: f.Name,
			size:        int64(f.UncompressedSize64),
			mtime:       mtime,
			isZipEntry:  true,
			zipPath:     zipPath,
		}
	}
	return nil
}

// hashWorker is a worker that hashes files from the jobs channel.
func (s *Scanner) hashWorker(libraryID int64, jobs <-chan fileJob, results chan<- hashResult) {
	for job := range jobs {
		// Check cache first
		cached, err := s.getCachedFile(libraryID, job.path, job.archivePath, job.size, job.mtime)
		if err != nil {
			results <- hashResult{job: job, err: err}
			continue
		}
		if cached != nil {
			// Cache hit
			results <- hashResult{job: job, sha1: cached.SHA1, crc32: cached.CRC32, wasHashed: false}
			continue
		}

		// Hash the file
		var sha1Hash, crc32Hash string
		if job.isZipEntry {
			sha1Hash, crc32Hash, err = s.hashZipEntry(job.zipPath, job.archivePath)
		} else if job.isCHD {
			sha1Hash, crc32Hash, err = s.hashCHDFile(job.path)
		} else {
			sha1Hash, crc32Hash, err = s.hashFile(job.path)
		}

		if err != nil {
			results <- hashResult{job: job, err: err}
			continue
		}

		results <- hashResult{job: job, sha1: sha1Hash, crc32: crc32Hash, wasHashed: true}
	}
}

// hashFile computes hashes for a regular file.
func (s *Scanner) hashFile(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()
	return computeHashes(f)
}

// hashCHDFile extracts hashes from a CHD file header without decompression.
func (s *Scanner) hashCHDFile(path string) (string, string, error) {
	info, err := ParseCHD(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse CHD: %w", err)
	}

	// Use DataSHA1 (raw data hash) for matching, as this is what DATs use.
	// CHD files don't have a traditional CRC32; we leave it empty.
	return info.DataSHA1, "", nil
}

// hashZipEntry computes hashes for a file inside a zip archive.
func (s *Scanner) hashZipEntry(zipPath, entryName string) (string, string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name == entryName {
			rc, err := f.Open()
			if err != nil {
				return "", "", err
			}
			sha1, crc32, err := computeHashes(rc)
			_ = rc.Close()
			return sha1, crc32, err
		}
	}
	return "", "", fmt.Errorf("entry %s not found in %s", entryName, zipPath)
}

// storeBatch writes a batch of hash results to the database in a single transaction.
func (s *Scanner) storeBatch(libraryID int64, batch []hashResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1, crc32, archive_path)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(library_id, path, archive_path) DO UPDATE SET
			size = excluded.size,
			mtime = excluded.mtime,
			sha1 = excluded.sha1,
			crc32 = excluded.crc32,
			scanned_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, r := range batch {
		var archivePathVal interface{}
		if r.job.archivePath != "" {
			archivePathVal = r.job.archivePath
		}
		_, err := stmt.Exec(libraryID, r.job.path, r.job.size, r.job.mtime, r.sha1, r.crc32, archivePathVal)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// scanSequential is the original sequential scanning implementation.
func (s *Scanner) scanSequential(lib *Library) (*ScanResult, error) {
	result := &ScanResult{}

	// Walk the library directory
	err := filepath.Walk(lib.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Skip non-ROM files (saves, states, thumbnails, etc.)
		if isIgnoredExtension(ext) {
			return nil
		}

		// Handle zip files
		if ext == ".zip" {
			zipResult, err := s.scanZipFile(lib, path, info)
			if err != nil {
				// Log error but continue scanning
				slog.Warn("failed to scan zip", "path", path, "error", err)
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
			slog.Warn("failed to scan file", "path", path, "error", err)
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

	// Clean up stale scanned files (files that no longer exist or are now ignored)
	if err := s.cleanupStaleFiles(lib); err != nil {
		return nil, fmt.Errorf("failed to cleanup stale files: %w", err)
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
			slog.Warn("failed to scan zip entry", "entry", f.Name, "error", err)
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

// cleanupStaleFiles removes scanned file entries that no longer exist or should be ignored.
func (s *Scanner) cleanupStaleFiles(lib *Library) error {
	// Get all scanned files for this library
	rows, err := s.db.Query(`
		SELECT id, path, archive_path FROM scanned_files WHERE library_id = ?
	`, lib.ID)
	if err != nil {
		return err
	}

	var toDelete []int64
	for rows.Next() {
		var id int64
		var path string
		var archivePath sql.NullString
		if err := rows.Scan(&id, &path, &archivePath); err != nil {
			_ = rows.Close()
			return err
		}

		// Check if file should be deleted
		shouldDelete := false

		// Check if extension is now ignored (for non-archive files)
		if !archivePath.Valid || archivePath.String == "" {
			ext := strings.ToLower(filepath.Ext(path))
			if isIgnoredExtension(ext) {
				shouldDelete = true
			}
		}

		// Check if file still exists (only for non-archive files)
		if !shouldDelete && (!archivePath.Valid || archivePath.String == "") {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				shouldDelete = true
			}
		}

		if shouldDelete {
			toDelete = append(toDelete, id)
		}
	}
	_ = rows.Close()

	// Delete stale entries
	for _, id := range toDelete {
		_, err := s.db.Exec("DELETE FROM scanned_files WHERE id = ?", id)
		if err != nil {
			return err
		}
	}

	return nil
}

type matchResult struct {
	MatchesFound   int
	UnmatchedFiles int
}

type fileToMatch struct {
	id    int64
	sha1  string
	crc32 string
	path  string
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

type releaseNameEntry struct {
	releaseID  int64
	romEntryID int64
	romName    string
	normalized string
}

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
