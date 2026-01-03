package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	require.NoError(t, err, "should open database without error")
	defer func() { _ = db.Close() }()

	// Check that the file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should exist")
}

func TestSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
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

	db, err := Open(dbPath)
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
		db, err := Open(dbPath)
		require.NoError(t, err, "should open database on attempt %d", i+1)
		_ = db.Close()
	}

	// Verify schema version is still 6
	db, err := Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var version int
	err = db.Conn().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 9, version, "schema version should still be 9 after multiple opens")
}
