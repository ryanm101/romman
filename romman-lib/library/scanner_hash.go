package library

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// computeHashes computes SHA1 and CRC32 hashes from a reader.
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
