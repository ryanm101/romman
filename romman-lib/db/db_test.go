package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err, "should open database without error")
	defer func() { _ = db.Close() }()

	// Check that the file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should exist")
}

func TestSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var version int
	err = db.Conn().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 9, version, "schema version should be 9")
}

func TestTablesExist(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	tables := []string{
		"systems", "releases", "rom_entries", "schema_version",
		"libraries", "scanned_files", "matches",
		"game_metadata", "game_media",
	}
	for _, table := range tables {
		var name string
		err := db.Conn().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		assert.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open and close multiple times
	for i := 0; i < 3; i++ {
		db, err := Open(context.Background(), dbPath)
		require.NoError(t, err, "should open database on attempt %d", i+1)
		_ = db.Close()
	}

	// Verify schema version is still 6
	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var version int
	err = db.Conn().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 9, version, "schema version should still be 9 after multiple opens")
}

func TestV6Columns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// V6 adds clone_of and parent_id to releases
	_, err = db.Conn().Exec(`
		INSERT INTO systems (name) VALUES ('test')
	`)
	require.NoError(t, err)

	_, err = db.Conn().Exec(`
		INSERT INTO releases (system_id, name, clone_of, parent_id)
		VALUES (1, 'Test Game', 'Parent Game', NULL)
	`)
	require.NoError(t, err)

	var cloneOf string
	err = db.Conn().QueryRow("SELECT clone_of FROM releases WHERE name = 'Test Game'").Scan(&cloneOf)
	require.NoError(t, err)
	assert.Equal(t, "Parent Game", cloneOf)
}

func TestV7DATSourcesTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// V7 adds dat_sources table
	_, err = db.Conn().Exec(`
		INSERT INTO systems (name) VALUES ('test')
	`)
	require.NoError(t, err)

	_, err = db.Conn().Exec(`
		INSERT INTO dat_sources (system_id, source_type, dat_name, dat_version, dat_file_path, dat_file_hash, priority)
		VALUES (1, 'no-intro', 'Test DAT', '1.0', '/path/to/dat.xml', 'abc123', 0)
	`)
	require.NoError(t, err)

	var count int
	err = db.Conn().QueryRow("SELECT COUNT(*) FROM dat_sources").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestV8MAMEColumns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// V8 adds year, manufacturer, is_bios, is_device, is_mechanical
	_, err = db.Conn().Exec(`INSERT INTO systems (name) VALUES ('mame')`)
	require.NoError(t, err)

	_, err = db.Conn().Exec(`
		INSERT INTO releases (system_id, name, year, manufacturer, is_bios, is_device, is_mechanical)
		VALUES (1, 'Pac-Man', '1980', 'Namco', 0, 0, 0)
	`)
	require.NoError(t, err)

	var year, manufacturer string
	var isBios int
	err = db.Conn().QueryRow(`
		SELECT year, manufacturer, is_bios FROM releases WHERE name = 'Pac-Man'
	`).Scan(&year, &manufacturer, &isBios)
	require.NoError(t, err)

	assert.Equal(t, "1980", year)
	assert.Equal(t, "Namco", manufacturer)
	assert.Equal(t, 0, isBios)
}

func TestV9Indexes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// V9 adds performance indexes
	indexes := []string{
		"idx_releases_system_name",
		"idx_scanned_files_path",
		"idx_dat_sources_hash",
		"idx_rom_entries_sha1_crc",
		"idx_matches_type",
	}

	for _, idx := range indexes {
		var name string
		err := db.Conn().QueryRow(`
			SELECT name FROM sqlite_master WHERE type='index' AND name=?
		`, idx).Scan(&name)
		assert.NoError(t, err, "index %s should exist", idx)
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	// Attempting to query after close should fail
	_, err = db.Conn().Query("SELECT 1")
	assert.Error(t, err)
}

func TestDBConn(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	conn := db.Conn()
	assert.NotNil(t, conn)

	// Verify it's usable
	err = conn.Ping()
	assert.NoError(t, err)
}
