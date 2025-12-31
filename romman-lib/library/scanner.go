package library

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ryanm101/romman-lib/metrics"
	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
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
var ignoredExtensions = map[string]bool{
	// Save files
	".srm": true, ".sav": true, ".eep": true, ".fla": true, ".rtc": true,
	// State files
	".state": true, ".st0": true, ".st1": true, ".st2": true, ".st3": true,
	".st4": true, ".st5": true, ".st6": true, ".st7": true, ".st8": true, ".st9": true, ".oops": true,
	// Thumbnails and metadata
	".png": true, ".jpg": true, ".jpeg": true, ".txt": true, ".nfo": true, ".xml": true, ".json": true,
	// Playlists and config
	".cfg": true, ".lpl": true, ".opt": true,
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
func (s *Scanner) Scan(ctx context.Context, libraryName string) (*ScanResult, error) {
	defer metrics.RecordScanDuration(libraryName, time.Now())

	ctx, span := tracing.StartSpan(ctx, "scan: "+libraryName,
		tracing.WithAttributes(attribute.String("library.name", libraryName)),
	)
	defer span.End()

	lib, err := s.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.String("system.name", lib.SystemName))

	if s.config.Parallel && s.config.Workers > 1 {
		return s.scanParallel(ctx, lib)
	}
	return s.scanSequential(ctx, lib)
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
func (s *Scanner) scanParallel(ctx context.Context, lib *Library) (*ScanResult, error) {
	ctx, span := tracing.StartSpan(ctx, "scanParallel: "+lib.Name)
	defer span.End()

	jobs := make(chan fileJob, s.config.Workers*10)
	results := make(chan hashResult, s.config.Workers*10)

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

	// Collector goroutine
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
			metrics.FilesProcessed.WithLabelValues(lib.Name, "scanned").Inc()
			if r.wasHashed {
				atomic.AddInt64(&filesHashed, 1)
				metrics.FilesProcessed.WithLabelValues(lib.Name, "hashed").Inc()
			} else {
				atomic.AddInt64(&filesSkipped, 1)
				metrics.FilesProcessed.WithLabelValues(lib.Name, "skipped").Inc()
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

		if len(batch) > 0 {
			if err := s.storeBatch(lib.ID, batch); err != nil {
				collectorErr = err
			}
		}
	}()

	// Walk and discover files
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
			if err := s.queueZipEntries(path, info, jobs); err != nil {
				slog.Warn("failed to open zip", "path", path, "error", err)
			}
			return nil
		}

		isCHD := ext == ".chd"
		jobs <- fileJob{path: path, size: info.Size(), mtime: info.ModTime().Unix(), isCHD: isCHD}
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
		cached, err := s.getCachedFile(libraryID, job.path, job.archivePath, job.size, job.mtime)
		if err != nil {
			results <- hashResult{job: job, err: err}
			continue
		}
		if cached != nil {
			results <- hashResult{job: job, sha1: cached.SHA1, crc32: cached.CRC32, wasHashed: false}
			continue
		}

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

// scanSequential is the original sequential scanning implementation.
func (s *Scanner) scanSequential(ctx context.Context, lib *Library) (*ScanResult, error) {
	ctx, span := tracing.StartSpan(ctx, "scanSequential: "+lib.Name)
	defer span.End()

	result := &ScanResult{}

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
			zipResult, err := s.scanZipFile(lib, path, info)
			if err != nil {
				slog.Warn("failed to scan zip", "path", path, "error", err)
				return nil
			}
			result.FilesScanned += zipResult.FilesScanned
			result.FilesHashed += zipResult.FilesHashed
			result.FilesSkipped += zipResult.FilesSkipped

			metrics.FilesProcessed.WithLabelValues(lib.Name, "scanned").Add(float64(zipResult.FilesScanned))
			metrics.FilesProcessed.WithLabelValues(lib.Name, "hashed").Add(float64(zipResult.FilesHashed))
			metrics.FilesProcessed.WithLabelValues(lib.Name, "skipped").Add(float64(zipResult.FilesSkipped))
			return nil
		}

		scanned, hashed, err := s.scanFile(lib, path, info, "")
		if err != nil {
			slog.Warn("failed to scan file", "path", path, "error", err)
			return nil
		}
		result.FilesScanned++
		metrics.FilesProcessed.WithLabelValues(lib.Name, "scanned").Inc()
		if hashed {
			result.FilesHashed++
			metrics.FilesProcessed.WithLabelValues(lib.Name, "hashed").Inc()
		} else if scanned {
			result.FilesSkipped++
			metrics.FilesProcessed.WithLabelValues(lib.Name, "skipped").Inc()
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk library: %w", err)
	}

	if err := s.cleanupStaleFiles(lib); err != nil {
		return nil, fmt.Errorf("failed to cleanup stale files: %w", err)
	}

	matchResult, err := s.matchFiles(lib)
	if err != nil {
		return nil, fmt.Errorf("failed to match files: %w", err)
	}
	result.MatchesFound = matchResult.MatchesFound
	result.UnmatchedFiles = matchResult.UnmatchedFiles

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

	cached, err := s.getCachedFile(lib.ID, path, archivePath, size, mtime)
	if err != nil {
		return false, false, err
	}
	if cached != nil {
		return true, false, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, false, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	sha1Hash, crc32Hash, err := computeHashes(f)
	if err != nil {
		return false, false, fmt.Errorf("failed to hash file: %w", err)
	}

	if err := s.storeScannedFile(lib.ID, path, archivePath, size, mtime, sha1Hash, crc32Hash); err != nil {
		return false, false, fmt.Errorf("failed to store scanned file: %w", err)
	}

	return true, true, nil
}

func (s *Scanner) scanZipEntry(lib *Library, zipPath string, f *zip.File, mtime, size int64) (scanned, hashed bool, err error) {
	archivePath := f.Name

	cached, err := s.getCachedFile(lib.ID, zipPath, archivePath, size, mtime)
	if err != nil {
		return false, false, err
	}
	if cached != nil {
		return true, false, nil
	}

	rc, err := f.Open()
	if err != nil {
		return false, false, fmt.Errorf("failed to open zip entry: %w", err)
	}
	defer func() { _ = rc.Close() }()

	sha1Hash, crc32Hash, err := computeHashes(rc)
	if err != nil {
		return false, false, fmt.Errorf("failed to hash zip entry: %w", err)
	}

	if err := s.storeScannedFile(lib.ID, zipPath, archivePath, size, mtime, sha1Hash, crc32Hash); err != nil {
		return false, false, fmt.Errorf("failed to store scanned file: %w", err)
	}

	return true, true, nil
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

		shouldDelete := false

		if !archivePath.Valid || archivePath.String == "" {
			ext := strings.ToLower(filepath.Ext(path))
			if isIgnoredExtension(ext) {
				shouldDelete = true
			}
		}

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

	for _, id := range toDelete {
		_, err := s.db.Exec("DELETE FROM scanned_files WHERE id = ?", id)
		if err != nil {
			return err
		}
	}

	return nil
}
