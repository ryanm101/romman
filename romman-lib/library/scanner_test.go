package library

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanm/romman-lib/db"
)

func TestScanner_BasicScan(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Setup: create system, release, and rom_entry
	_, err = database.Conn().Exec(`
		INSERT INTO systems (id, name, dat_name) VALUES (1, 'nes', 'Nintendo - NES')
	`)
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO releases (id, system_id, name) VALUES (1, 1, 'Test Game (USA)')
	`)
	require.NoError(t, err)

	// The content "test rom content" has these hashes:
	// SHA1: 331407b2bd72286d458f26c426d78f459d7116d3
	// CRC32: d3764b6a
	_, err = database.Conn().Exec(`
		INSERT INTO rom_entries (id, release_id, name, sha1, crc32, size)
		VALUES (1, 1, 'test.nes', '331407b2bd72286d458f26c426d78f459d7116d3', 'd3764b6a', 16)
	`)
	require.NoError(t, err)

	// Create library directory with a ROM file
	libPath := filepath.Join(tmpDir, "roms")
	require.NoError(t, os.MkdirAll(libPath, 0755))

	romPath := filepath.Join(libPath, "test.nes")
	require.NoError(t, os.WriteFile(romPath, []byte("test rom content"), 0644))

	// Add library
	manager := NewManager(database.Conn())
	_, err = manager.Add("test-lib", libPath, "nes")
	require.NoError(t, err)

	// Scan
	scanner := NewScanner(database.Conn())
	result, err := scanner.Scan("test-lib")
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesScanned)
	assert.Equal(t, 1, result.FilesHashed)
	assert.Equal(t, 1, result.MatchesFound)
	assert.Equal(t, 0, result.UnmatchedFiles)
}

func TestScanner_HashCaching(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Setup
	_, err = database.Conn().Exec(`INSERT INTO systems (id, name) VALUES (1, 'nes')`)
	require.NoError(t, err)

	libPath := filepath.Join(tmpDir, "roms")
	require.NoError(t, os.MkdirAll(libPath, 0755))

	romPath := filepath.Join(libPath, "test.nes")
	require.NoError(t, os.WriteFile(romPath, []byte("test content"), 0644))

	manager := NewManager(database.Conn())
	_, err = manager.Add("test-lib", libPath, "nes")
	require.NoError(t, err)

	scanner := NewScanner(database.Conn())

	// First scan - should hash
	result1, err := scanner.Scan("test-lib")
	require.NoError(t, err)
	assert.Equal(t, 1, result1.FilesHashed)
	assert.Equal(t, 0, result1.FilesSkipped)

	// Second scan - should use cache
	result2, err := scanner.Scan("test-lib")
	require.NoError(t, err)
	assert.Equal(t, 0, result2.FilesHashed)
	assert.Equal(t, 1, result2.FilesSkipped)
}

func TestScanner_ZipSupport(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Setup
	_, err = database.Conn().Exec(`INSERT INTO systems (id, name) VALUES (1, 'nes')`)
	require.NoError(t, err)

	_, err = database.Conn().Exec(`INSERT INTO releases (id, system_id, name) VALUES (1, 1, 'Test Game')`)
	require.NoError(t, err)

	// SHA1 of "zip rom content"
	_, err = database.Conn().Exec(`
		INSERT INTO rom_entries (id, release_id, name, sha1, size)
		VALUES (1, 1, 'game.nes', 'da39a3ee5e6b4b0d3255bfef95601890afd80709', 15)
	`)
	require.NoError(t, err)

	// Create library with zip file
	libPath := filepath.Join(tmpDir, "roms")
	require.NoError(t, os.MkdirAll(libPath, 0755))

	zipPath := filepath.Join(libPath, "game.zip")
	createTestZip(t, zipPath, "game.nes", []byte("zip rom content"))

	manager := NewManager(database.Conn())
	_, err = manager.Add("test-lib", libPath, "nes")
	require.NoError(t, err)

	// Scan
	scanner := NewScanner(database.Conn())
	result, err := scanner.Scan("test-lib")
	require.NoError(t, err)

	assert.Equal(t, 1, result.FilesScanned)
}

func createTestZip(t *testing.T, zipPath, filename string, content []byte) {
	t.Helper()

	f, err := os.Create(zipPath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	defer func() { _ = w.Close() }()

	fw, err := w.Create(filename)
	require.NoError(t, err)

	_, err = fw.Write(content)
	require.NoError(t, err)
}
