package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanm101/romman-lib/db"
)

func TestNewRetroArchExporter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	exporter := NewRetroArchExporter(database.Conn())
	assert.NotNil(t, exporter)
}

func TestRetroArchExporter_LibraryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	exporter := NewRetroArchExporter(database.Conn())

	err = exporter.ExportPlaylist("nonexistent", "/tmp/test.lpl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "library not found")
}

func TestRetroArchExporter_ExportPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Setup test data
	_, err = database.Conn().Exec("INSERT INTO systems (name) VALUES ('nes')")
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO releases (system_id, name, description)
		VALUES (1, 'Super Mario Bros (USA)', 'Super Mario Bros')
	`)
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO libraries (name, root_path, system_id)
		VALUES ('nes', '/roms/nes', 1)
	`)
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1, crc32)
		VALUES (1, '/roms/nes/smb.nes', 1024, 1234567890, 'abc123', 'AABBCCDD')
	`)
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO rom_entries (release_id, name, sha1, size)
		VALUES (1, 'Super Mario Bros (USA).nes', 'abc123', 1024)
	`)
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO matches (scanned_file_id, rom_entry_id, match_type)
		VALUES (1, 1, 'sha1')
	`)
	require.NoError(t, err)

	exporter := NewRetroArchExporter(database.Conn())

	outputPath := filepath.Join(tmpDir, "nes.lpl")
	err = exporter.ExportPlaylist("nes", outputPath)
	require.NoError(t, err)

	// Verify the file was created
	_, err = os.Stat(outputPath)
	assert.NoError(t, err)

	// Verify the content
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var playlist RetroArchPlaylist
	err = json.Unmarshal(data, &playlist)
	require.NoError(t, err)

	assert.Equal(t, "1.5", playlist.Version)
	assert.Len(t, playlist.Items, 1)
	assert.Equal(t, "Super Mario Bros (USA)", playlist.Items[0].Label)
}

func TestGetRetroArchDBName(t *testing.T) {
	tests := []struct {
		system   string
		expected string
	}{
		{"Nintendo - Game Boy", "Nintendo - Game Boy.lpl"},
		{"Nintendo - Game Boy Advance", "Nintendo - Game Boy Advance.lpl"},
		{"Sony - PlayStation", "Sony - PlayStation.lpl"},
		{"unknown_system", "unknown_system.lpl"},
	}

	for _, tt := range tests {
		t.Run(tt.system, func(t *testing.T) {
			result := getRetroArchDBName(tt.system)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRegion(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Super Mario Bros (USA)", "USA"},
		{"Sonic the Hedgehog (Europe)", "Europe"},
		{"Final Fantasy (Japan)", "Japan"},
		{"Pokemon Blue (USA, Europe)", "USA"},
		{"Tetris (World)", "World"},
		{"Unknown Game", "Other"},
		{"Game (Australia)", "Australia"},
		{"Game (France)", "France"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRegion(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}
