package library

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestExportMatchedIntegration tests matched export with real data.
func TestExportMatchedIntegration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := initTestSchema(db); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	// Setup test data
	setup1G1RTestData(t, db)

	manager := NewManager(db)
	exporter := NewExporter(db, manager)

	// Export matched as JSON
	data, err := exporter.Export("testlib", ReportMatched, FormatJSON)
	if err != nil {
		t.Fatalf("Failed to export: %v", err)
	}

	var result ExportResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result.Library != "testlib" {
		t.Errorf("Expected library 'testlib', got '%s'", result.Library)
	}
	if result.Count != len(result.Records) {
		t.Errorf("Count mismatch: %d vs %d", result.Count, len(result.Records))
	}
}

// TestExport1G1RIntegration tests the 1G1R export mode.
func TestExport1G1RIntegration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := initTestSchema(db); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	// Setup test data with preferred releases
	setup1G1RTestData(t, db)

	manager := NewManager(db)
	exporter := NewExporter(db, manager)

	// Export 1G1R as JSON
	data, err := exporter.Export("testlib", Report1G1R, FormatJSON)
	if err != nil {
		t.Fatalf("Failed to export 1G1R: %v", err)
	}

	var result ExportResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result.Report != "1g1r" {
		t.Errorf("Expected report '1g1r', got '%s'", result.Report)
	}

	// Verify only preferred AND matched releases are returned
	for _, rec := range result.Records {
		if rec.Status != "1g1r" {
			t.Errorf("Expected status '1g1r', got '%s'", rec.Status)
		}
	}
}

// TestExportCSVFormat tests CSV output formatting.
func TestExportCSVFormat(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := initTestSchema(db); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	setup1G1RTestData(t, db)

	manager := NewManager(db)
	exporter := NewExporter(db, manager)

	// Export as CSV
	data, err := exporter.Export("testlib", ReportMatched, FormatCSV)
	if err != nil {
		t.Fatalf("Failed to export CSV: %v", err)
	}

	csv := string(data)

	// Verify header is present
	if !strings.Contains(csv, "name,path,hash,match_type,flags") {
		t.Error("CSV missing expected header")
	}
}

// setup1G1RTestData creates test data for export integration tests.
func setup1G1RTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	// Create system
	_, err := db.Exec(`INSERT INTO systems (id, name, dat_name) VALUES (1, 'TestSystem', 'test.dat')`)
	if err != nil {
		t.Fatalf("Failed to create system: %v", err)
	}

	// Create library
	_, err = db.Exec(`INSERT INTO libraries (id, name, root_path, system_id) VALUES (1, 'testlib', '/test/path', 1)`)
	if err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Create releases
	_, err = db.Exec(`
		INSERT INTO releases (id, system_id, name, is_preferred) VALUES 
		(1, 1, 'Game A (Europe)', 1),
		(2, 1, 'Game A (USA)', 0),
		(3, 1, 'Game B (Europe)', 1)
	`)
	if err != nil {
		t.Fatalf("Failed to create releases: %v", err)
	}

	// Create ROM entries
	_, err = db.Exec(`
		INSERT INTO rom_entries (id, release_id, name, sha1, crc32) VALUES 
		(1, 1, 'gamea.bin', 'abc123', 'DEF456'),
		(2, 2, 'gamea.bin', 'abc123', 'DEF456'),
		(3, 3, 'gameb.bin', 'xyz789', 'GHI012')
	`)
	if err != nil {
		t.Fatalf("Failed to create ROM entries: %v", err)
	}

	// Create scanned files
	_, err = db.Exec(`
		INSERT INTO scanned_files (id, library_id, path, sha1, crc32) VALUES 
		(1, 1, '/test/path/gamea.bin', 'abc123', 'DEF456'),
		(2, 1, '/test/path/gameb.bin', 'xyz789', 'GHI012')
	`)
	if err != nil {
		t.Fatalf("Failed to create scanned files: %v", err)
	}

	// Create matches
	_, err = db.Exec(`
		INSERT INTO matches (id, scanned_file_id, rom_entry_id, match_type) VALUES 
		(1, 1, 1, 'sha1'),
		(2, 2, 3, 'sha1')
	`)
	if err != nil {
		t.Fatalf("Failed to create matches: %v", err)
	}
}
