package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database connection with ROM manager functionality.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens or creates a SQLite database at the given path.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying database connection.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// migrate runs database migrations up to the current schema version.
func (db *DB) migrate() error {
	// Create schema version table if not exists
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		)
	`); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current version
	var version int
	err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	// Run migrations
	if version < 1 {
		if err := db.migrateV1(); err != nil {
			return err
		}
	}
	if version < 2 {
		if err := db.migrateV2(); err != nil {
			return err
		}
	}
	if version < 3 {
		if err := db.migrateV3(); err != nil {
			return err
		}
	}
	if version < 4 {
		if err := db.migrateV4(); err != nil {
			return err
		}
	}

	return nil
}

// migrateV1 creates the initial schema.
func (db *DB) migrateV1() error {
	schema := `
		CREATE TABLE IF NOT EXISTS systems (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			dat_name TEXT,
			dat_description TEXT,
			dat_version TEXT,
			dat_date TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS releases (
			id INTEGER PRIMARY KEY,
			system_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			FOREIGN KEY(system_id) REFERENCES systems(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_releases_system_id ON releases(system_id);
		CREATE INDEX IF NOT EXISTS idx_releases_name ON releases(name);

		CREATE TABLE IF NOT EXISTS rom_entries (
			id INTEGER PRIMARY KEY,
			release_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			sha1 TEXT,
			crc32 TEXT,
			md5 TEXT,
			size INTEGER,
			FOREIGN KEY(release_id) REFERENCES releases(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_rom_entries_release_id ON rom_entries(release_id);
		CREATE INDEX IF NOT EXISTS idx_rom_entries_sha1 ON rom_entries(sha1);
		CREATE INDEX IF NOT EXISTS idx_rom_entries_crc32 ON rom_entries(crc32);

		INSERT INTO schema_version (version) VALUES (1);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute v1 migration: %w", err)
	}

	return nil
}

// migrateV2 adds library scanning tables.
func (db *DB) migrateV2() error {
	schema := `
		-- Libraries represent ROM collection directories
		CREATE TABLE IF NOT EXISTS libraries (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			root_path TEXT NOT NULL,
			system_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME,
			FOREIGN KEY(system_id) REFERENCES systems(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_libraries_system_id ON libraries(system_id);

		-- Scanned files with hash cache
		CREATE TABLE IF NOT EXISTS scanned_files (
			id INTEGER PRIMARY KEY,
			library_id INTEGER NOT NULL,
			path TEXT NOT NULL,
			size INTEGER NOT NULL,
			mtime INTEGER NOT NULL,
			sha1 TEXT,
			crc32 TEXT,
			archive_path TEXT,  -- path within zip if applicable
			scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(library_id) REFERENCES libraries(id) ON DELETE CASCADE,
			UNIQUE(library_id, path, archive_path)
		);

		CREATE INDEX IF NOT EXISTS idx_scanned_files_library_id ON scanned_files(library_id);
		CREATE INDEX IF NOT EXISTS idx_scanned_files_sha1 ON scanned_files(sha1);
		CREATE INDEX IF NOT EXISTS idx_scanned_files_crc32 ON scanned_files(crc32);

		-- Matches between scanned files and rom_entries
		CREATE TABLE IF NOT EXISTS matches (
			id INTEGER PRIMARY KEY,
			scanned_file_id INTEGER NOT NULL,
			rom_entry_id INTEGER NOT NULL,
			match_type TEXT NOT NULL,  -- 'sha1' or 'crc32'
			FOREIGN KEY(scanned_file_id) REFERENCES scanned_files(id) ON DELETE CASCADE,
			FOREIGN KEY(rom_entry_id) REFERENCES rom_entries(id) ON DELETE CASCADE,
			UNIQUE(scanned_file_id, rom_entry_id)
		);

		CREATE INDEX IF NOT EXISTS idx_matches_scanned_file_id ON matches(scanned_file_id);
		CREATE INDEX IF NOT EXISTS idx_matches_rom_entry_id ON matches(rom_entry_id);

		INSERT INTO schema_version (version) VALUES (2);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute v2 migration: %w", err)
	}

	return nil
}

// migrateV3 adds flags column to matches for storing ROM status.
func (db *DB) migrateV3() error {
	schema := `
		ALTER TABLE matches ADD COLUMN flags TEXT;

		INSERT INTO schema_version (version) VALUES (3);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute v3 migration: %w", err)
	}

	return nil
}

// migrateV4 adds preferred release tracking.
func (db *DB) migrateV4() error {
	schema := `
		ALTER TABLE releases ADD COLUMN is_preferred INTEGER DEFAULT 0;
		ALTER TABLE releases ADD COLUMN ignore_reason TEXT;

		CREATE INDEX IF NOT EXISTS idx_releases_is_preferred ON releases(is_preferred);

		INSERT INTO schema_version (version) VALUES (4);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute v4 migration: %w", err)
	}

	return nil
}
