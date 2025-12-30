package library

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// IntegrityIssue represents a detected integrity problem.
type IntegrityIssue struct {
	Path      string
	IssueType string // "changed", "missing", "incomplete"
	Details   string
}

// IntegrityResult contains the outcome of an integrity check.
type IntegrityResult struct {
	FilesChecked int
	Issues       []IntegrityIssue
	OK           int
	Changed      int
	Missing      int
	Incomplete   int
}

// IntegrityChecker verifies library file integrity.
type IntegrityChecker struct {
	db      *sql.DB
	manager *Manager
}

// NewIntegrityChecker creates a new integrity checker.
func NewIntegrityChecker(db *sql.DB, manager *Manager) *IntegrityChecker {
	return &IntegrityChecker{db: db, manager: manager}
}

// Check verifies all files in a library.
func (c *IntegrityChecker) Check(libraryName string) (*IntegrityResult, error) {
	lib, err := c.manager.Get(libraryName)
	if err != nil {
		return nil, err
	}

	result := &IntegrityResult{}

	// Get all scanned files (non-archive only for now)
	rows, err := c.db.Query(`
		SELECT id, path, sha1, size FROM scanned_files
		WHERE library_id = ? AND archive_path IS NULL
	`, lib.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var fileID int64
		var path, storedHash string
		var storedSize int64
		if err := rows.Scan(&fileID, &path, &storedHash, &storedSize); err != nil {
			continue
		}

		result.FilesChecked++

		// Check if file exists
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			result.Issues = append(result.Issues, IntegrityIssue{
				Path:      path,
				IssueType: "missing",
				Details:   "file no longer exists",
			})
			result.Missing++
			continue
		}
		if err != nil {
			result.Issues = append(result.Issues, IntegrityIssue{
				Path:      path,
				IssueType: "error",
				Details:   err.Error(),
			})
			continue
		}

		// Check size first (fast check)
		if info.Size() != storedSize {
			result.Issues = append(result.Issues, IntegrityIssue{
				Path:      path,
				IssueType: "changed",
				Details:   fmt.Sprintf("size changed: %d -> %d", storedSize, info.Size()),
			})
			result.Changed++
			continue
		}

		// Verify hash
		currentHash, err := hashFile(path)
		if err != nil {
			result.Issues = append(result.Issues, IntegrityIssue{
				Path:      path,
				IssueType: "error",
				Details:   fmt.Sprintf("hash error: %v", err),
			})
			continue
		}

		if currentHash != storedHash {
			result.Issues = append(result.Issues, IntegrityIssue{
				Path:      path,
				IssueType: "changed",
				Details:   "hash mismatch",
			})
			result.Changed++
		} else {
			result.OK++
		}
	}

	// Check for incomplete multi-file games
	incompleteReleases, err := c.checkIncomplete(lib.ID)
	if err == nil {
		for _, rel := range incompleteReleases {
			result.Issues = append(result.Issues, IntegrityIssue{
				Path:      rel.Name,
				IssueType: "incomplete",
				Details:   fmt.Sprintf("has %d/%d files", rel.Matched, rel.Total),
			})
			result.Incomplete++
		}
	}

	return result, nil
}

type incompleteRelease struct {
	Name    string
	Total   int
	Matched int
}

func (c *IntegrityChecker) checkIncomplete(libraryID int64) ([]incompleteRelease, error) {
	rows, err := c.db.Query(`
		SELECT r.name, COUNT(re.id) as total,
			COUNT(DISTINCT CASE WHEN m.id IS NOT NULL THEN re.id END) as matched
		FROM releases r
		JOIN rom_entries re ON re.release_id = r.id
		LEFT JOIN matches m ON m.rom_entry_id = re.id
			AND m.scanned_file_id IN (SELECT id FROM scanned_files WHERE library_id = ?)
		JOIN libraries l ON l.system_id = r.system_id AND l.id = ?
		GROUP BY r.id
		HAVING total > 1 AND matched > 0 AND matched < total
		ORDER BY r.name
	`, libraryID, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []incompleteRelease
	for rows.Next() {
		var r incompleteRelease
		if err := rows.Scan(&r.Name, &r.Total, &r.Matched); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
