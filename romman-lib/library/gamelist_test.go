package library

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanm101/romman-lib/db"
)

func TestExportGamelist(t *testing.T) {
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
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1)
		VALUES (1, '/roms/nes/smb.nes', 1024, 1234567890, 'abc123')
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

	manager := NewManager(database.Conn())
	exporter := NewExporter(database.Conn(), manager)

	opts := GamelistOptions{
		PathPrefix: "./",
	}

	result, err := exporter.ExportGamelist(context.Background(), "nes", opts)
	require.NoError(t, err)

	// Verify it contains valid XML
	assert.Contains(t, string(result), "<?xml")
	assert.Contains(t, string(result), "<gameList>")
	assert.Contains(t, string(result), "</gameList>")
}

func TestExportLaunchBox(t *testing.T) {
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
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1)
		VALUES (1, '/roms/nes/smb.nes', 1024, 1234567890, 'abc123')
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

	manager := NewManager(database.Conn())
	exporter := NewExporter(database.Conn(), manager)

	opts := LaunchBoxOptions{}

	result, err := exporter.ExportLaunchBox(context.Background(), "nes", opts)
	require.NoError(t, err)

	// Verify it contains valid LaunchBox XML
	assert.Contains(t, string(result), "<?xml")
	assert.Contains(t, string(result), "<LaunchBox>")
	assert.Contains(t, string(result), "</LaunchBox>")
	assert.Contains(t, string(result), "<Game>")
}

func TestGamelistOptions(t *testing.T) {
	opts := GamelistOptions{
		PathPrefix:  "./",
		MatchedOnly: true,
	}

	assert.Equal(t, "./", opts.PathPrefix)
	assert.True(t, opts.MatchedOnly)
}

func TestLaunchBoxOptions(t *testing.T) {
	opts := LaunchBoxOptions{
		MatchedOnly: true,
	}

	assert.True(t, opts.MatchedOnly)
}

func TestFormatPlatformName(t *testing.T) {
	tests := []struct {
		system   string
		expected string
	}{
		{"nes", "Nintendo Entertainment System"},
		{"snes", "Super Nintendo Entertainment System"},
		{"gb", "Nintendo Game Boy"},
		{"gba", "Nintendo Game Boy Advance"},
		{"n64", "Nintendo 64"},
		{"psx", "Sony Playstation"},
		{"genesis", "Sega Genesis"},
		{"unknown_system", "unknown_system"}, // unmapped returns original
	}

	for _, tt := range tests {
		t.Run(tt.system, func(t *testing.T) {
			result := formatPlatformName(tt.system)
			assert.Equal(t, tt.expected, result)
		})
	}
}
