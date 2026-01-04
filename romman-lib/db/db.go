package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database connection with ROM manager functionality.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens or creates a SQLite database at the given path.
// The connection is instrumented with OpenTelemetry for automatic query tracing.
func Open(ctx context.Context, path string) (*DB, error) {
	dbName := filepath.Base(path)

	// Use otelsql to wrap the database connection with tracing
	conn, err := otelsql.Open("sqlite", path,
		otelsql.WithAttributes(
			attribute.String("db.system", "sqlite"),
			attribute.String("db.name", dbName),
		),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Register metrics for connection pool health (ignore error, metrics are optional)
	_, _ = otelsql.RegisterDBStatsMetrics(conn, otelsql.WithAttributes(
		attribute.String("db.system", "sqlite"),
		attribute.String("db.name", dbName),
	))

	// Enable WAL mode for better concurrent access
	if _, err := conn.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout to 30 seconds to wait for locks instead of failing immediately
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout=30000"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{conn: conn, path: path}
	if err := db.migrate(ctx); err != nil {
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
func (db *DB) migrate(ctx context.Context) error {
	// Create schema version table if not exists
	if _, err := db.conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		)
	`); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current version
	var version int
	err := db.conn.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	// Run migrations
	if version < 1 {
		if err := db.migrateV1(ctx); err != nil {
			return err
		}
	}
	if version < 2 {
		if err := db.migrateV2(ctx); err != nil {
			return err
		}
	}
	if version < 3 {
		if err := db.migrateV3(ctx); err != nil {
			return err
		}
	}
	if version < 4 {
		if err := db.migrateV4(ctx); err != nil {
			return err
		}
	}
	if version < 5 {
		if err := db.migrateV5(ctx); err != nil {
			return err
		}
	}
	if version < 6 {
		if err := db.migrateV6(ctx); err != nil {
			return err
		}
	}
	if version < 7 {
		if err := db.migrateV7(ctx); err != nil {
			return err
		}
	}
	if version < 8 {
		if err := db.migrateV8(ctx); err != nil {
			return err
		}
	}
	if version < 9 {
		if err := db.migrateV9(ctx); err != nil {
			return err
		}
	}

	return nil
}

// ... existing migrations ...

// migrateV5 adds metadata and media tables.
func (db *DB) migrateV5(ctx context.Context) error {
	schema := `
		-- Game metadata (scraped)
		CREATE TABLE IF NOT EXISTS game_metadata (
			release_id INTEGER PRIMARY KEY,
			provider_id TEXT, -- e.g., 'igdb:12345'
			description TEXT,
			release_date TEXT,
			developer TEXT,
			publisher TEXT,
			rating REAL,
			scraped_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(release_id) REFERENCES releases(id) ON DELETE CASCADE
		);

		-- Game media (images)
		CREATE TABLE IF NOT EXISTS game_media (
			id INTEGER PRIMARY KEY,
			release_id INTEGER NOT NULL,
			type TEXT NOT NULL, -- 'boxart', 'screenshot', 'logo'
			url TEXT,           -- Remote URL
			local_path TEXT,    -- Local file path
			FOREIGN KEY(release_id) REFERENCES releases(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_game_media_release_id ON game_media(release_id);

		INSERT INTO schema_version (version) VALUES (5);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v5 migration: %w", err)
	}

	return nil
}

// migrateV1 creates the initial schema.
func (db *DB) migrateV1(ctx context.Context) error {
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

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v1 migration: %w", err)
	}

	return nil
}

// migrateV2 adds library scanning tables.
func (db *DB) migrateV2(ctx context.Context) error {
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

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v2 migration: %w", err)
	}

	return nil
}

// migrateV3 adds flags column to matches for storing ROM status.
func (db *DB) migrateV3(ctx context.Context) error {
	schema := `
		ALTER TABLE matches ADD COLUMN flags TEXT;

		INSERT INTO schema_version (version) VALUES (3);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v3 migration: %w", err)
	}

	return nil
}

// migrateV4 adds preferred release tracking.
func (db *DB) migrateV4(ctx context.Context) error {
	schema := `
		ALTER TABLE releases ADD COLUMN is_preferred INTEGER DEFAULT 0;
		ALTER TABLE releases ADD COLUMN ignore_reason TEXT;

		CREATE INDEX IF NOT EXISTS idx_releases_is_preferred ON releases(is_preferred);

		INSERT INTO schema_version (version) VALUES (4);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v4 migration: %w", err)
	}

	return nil
}

// migrateV6 adds parent/clone support.
func (db *DB) migrateV6(ctx context.Context) error {
	schema := `
		ALTER TABLE releases ADD COLUMN clone_of TEXT;
		ALTER TABLE releases ADD COLUMN parent_id INTEGER REFERENCES releases(id) ON DELETE SET NULL;

		CREATE INDEX IF NOT EXISTS idx_releases_parent_id ON releases(parent_id);

		INSERT INTO schema_version (version) VALUES (6);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v6 migration: %w", err)
	}

	return nil
}

// migrateV7 adds multi-DAT source support.
func (db *DB) migrateV7(ctx context.Context) error {
	schema := `
		-- Track multiple DAT sources per system
		CREATE TABLE IF NOT EXISTS dat_sources (
			id INTEGER PRIMARY KEY,
			system_id INTEGER NOT NULL REFERENCES systems(id) ON DELETE CASCADE,
			source_type TEXT NOT NULL,       -- 'no-intro', 'redump', 'tosec', 'mame', 'other'
			dat_name TEXT,                   -- Original DAT header name
			dat_version TEXT,
			dat_date TEXT,
			dat_file_path TEXT,              -- Original import path
			dat_file_hash TEXT,              -- SHA256 of DAT file for update detection
			priority INTEGER DEFAULT 0,      -- Lower = higher priority, first added = 0
			imported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(system_id, source_type)
		);

		CREATE INDEX IF NOT EXISTS idx_dat_sources_system_id ON dat_sources(system_id);

		-- Link releases to their source DAT (optional for backward compat)
		ALTER TABLE releases ADD COLUMN dat_source_id INTEGER REFERENCES dat_sources(id) ON DELETE SET NULL;

		INSERT INTO schema_version (version) VALUES (7);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v7 migration: %w", err)
	}

	return nil
}

// migrateV8 adds MAME-specific metadata to releases.
func (db *DB) migrateV8(ctx context.Context) error {
	schema := `
		-- Add MAME metadata columns to releases
		ALTER TABLE releases ADD COLUMN year TEXT;
		ALTER TABLE releases ADD COLUMN manufacturer TEXT;
		ALTER TABLE releases ADD COLUMN is_bios INTEGER DEFAULT 0;
		ALTER TABLE releases ADD COLUMN is_device INTEGER DEFAULT 0;
		ALTER TABLE releases ADD COLUMN is_mechanical INTEGER DEFAULT 0;

		INSERT INTO schema_version (version) VALUES (8);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v8 migration: %w", err)
	}

	return nil
}

// migrateV9 adds performance indexes.
func (db *DB) migrateV9(ctx context.Context) error {
	schema := `
		-- Performance indexes for common query patterns
		CREATE INDEX IF NOT EXISTS idx_releases_system_name ON releases(system_id, name);
		CREATE INDEX IF NOT EXISTS idx_scanned_files_path ON scanned_files(path);
		CREATE INDEX IF NOT EXISTS idx_dat_sources_hash ON dat_sources(dat_file_hash);
		CREATE INDEX IF NOT EXISTS idx_rom_entries_sha1_crc ON rom_entries(sha1, crc32);
		CREATE INDEX IF NOT EXISTS idx_matches_type ON matches(match_type);

		INSERT INTO schema_version (version) VALUES (9);
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to execute v9 migration: %w", err)
	}

	return nil
}
