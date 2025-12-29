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
