package library

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanm101/romman-lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupExportTestDB(t *testing.T) *sql.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)

	// Caller is responsible for closing via t.Cleanup
	t.Cleanup(func() { _ = database.Close() })

	return database.Conn()
}

func setupExportTestData(t *testing.T, conn *sql.DB) {
	// Create system
	_, err := conn.Exec(`
		INSERT INTO systems (name, dat_name, dat_description, dat_version, dat_date)
		VALUES ('testsystem', 'Test System', 'Test Description', '1.0', '2024-01-01')
	`)
	require.NoError(t, err)

	// Create release
	_, err = conn.Exec(`
		INSERT INTO releases (system_id, name)
		VALUES (1, 'Test Game (USA)')
	`)
	require.NoError(t, err)

	// Create rom entry
	_, err = conn.Exec(`
		INSERT INTO rom_entries (release_id, name, size, sha1, crc32, md5)
		VALUES (1, 'test.bin', 1024, 'abc123', 'def456', 'ghi789')
	`)
	require.NoError(t, err)

	// Create library
	_, err = conn.Exec(`
		INSERT INTO libraries (name, root_path, system_id)
		VALUES ('testlib', '/tmp/testlib', 1)
	`)
	require.NoError(t, err)
}

func TestExporter_ExportMatched_CSV(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	// Add scanned file with match
	_, err := conn.Exec(`
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1, crc32)
		VALUES (1, '/tmp/testlib/test.bin', 1024, 1234567890, 'abc123', 'def456')
	`)
	require.NoError(t, err)

	_, err = conn.Exec(`
		INSERT INTO matches (scanned_file_id, rom_entry_id, match_type)
		VALUES (1, 1, 'sha1')
	`)
	require.NoError(t, err)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	data, err := exporter.Export("testlib", ReportMatched, FormatCSV)
	require.NoError(t, err)

	csv := string(data)
	assert.Contains(t, csv, "name,path,hash,match_type,flags")
	assert.Contains(t, csv, "Test Game")
}

func TestExporter_ExportMatched_JSON(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	// Add scanned file with match
	_, err := conn.Exec(`
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1, crc32)
		VALUES (1, '/tmp/testlib/test.bin', 1024, 1234567890, 'abc123', 'def456')
	`)
	require.NoError(t, err)

	_, err = conn.Exec(`
		INSERT INTO matches (scanned_file_id, rom_entry_id, match_type)
		VALUES (1, 1, 'sha1')
	`)
	require.NoError(t, err)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	data, err := exporter.Export("testlib", ReportMatched, FormatJSON)
	require.NoError(t, err)

	var result ExportResult
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "testlib", result.Library)
	assert.Equal(t, "testsystem", result.System)
	assert.Equal(t, "matched", result.Report)
	assert.Equal(t, 1, result.Count)
}

func TestExporter_ExportMissing(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	data, err := exporter.Export("testlib", ReportMissing, FormatJSON)
	require.NoError(t, err)

	var result ExportResult
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "missing", result.Report)
	assert.Equal(t, 1, result.Count) // The test game should be missing
}

func TestExporter_ExportUnmatched(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	// Add unmatched file (no match record)
	_, err := conn.Exec(`
		INSERT INTO scanned_files (library_id, path, size, mtime, sha1, crc32)
		VALUES (1, '/tmp/testlib/unknown.bin', 2048, 1234567890, 'xyz999', 'aaa111')
	`)
	require.NoError(t, err)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	data, err := exporter.Export("testlib", ReportUnmatched, FormatCSV)
	require.NoError(t, err)

	csv := string(data)
	assert.Contains(t, csv, "path,hash,status")
	assert.Contains(t, csv, "unknown.bin")
}

func TestExporter_InvalidLibrary(t *testing.T) {
	conn := setupExportTestDB(t)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	_, err := exporter.Export("nonexistent", ReportMatched, FormatCSV)
	assert.Error(t, err)
}

func TestExporter_InvalidReportType(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	_, err := exporter.Export("testlib", ReportType("invalid"), FormatCSV)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown report type")
}

func TestExporter_InvalidFormat(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	_, err := exporter.Export("testlib", ReportMatched, ExportFormat("xml"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

func TestExporter_CSVEscapesCommas(t *testing.T) {
	conn := setupExportTestDB(t)
	setupExportTestData(t, conn)

	manager := NewManager(conn)
	exporter := NewExporter(conn, manager)

	records := []ExportRecord{
		{Name: "Game, The (USA)", Path: "/path/to/file.bin", Hash: "abc123", MatchType: "sha1"},
	}

	data, err := exporter.toCSV(records, ReportMatched)
	require.NoError(t, err)

	csv := string(data)
	// CSV should properly quote the comma in the name
	assert.True(t, strings.Contains(csv, `"Game, The (USA)"`))
}
