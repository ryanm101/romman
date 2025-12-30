package library

import (
	"database/sql"
	"fmt"
	"time"
)

// Library represents a ROM collection directory.
type Library struct {
	ID         int64
	Name       string
	RootPath   string
	SystemID   int64
	SystemName string
	CreatedAt  time.Time
	LastScanAt *time.Time
}

// Manager handles library operations.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new library manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Add creates a new library for the given system.
func (m *Manager) Add(name, rootPath, systemName string) (*Library, error) {
	// Look up system ID
	var systemID int64
	err := m.db.QueryRow("SELECT id FROM systems WHERE name = ?", systemName).Scan(&systemID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("system not found: %s", systemName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up system: %w", err)
	}

	// Insert library
	result, err := m.db.Exec(`
		INSERT INTO libraries (name, root_path, system_id)
		VALUES (?, ?, ?)
	`, name, rootPath, systemID)
	if err != nil {
		return nil, fmt.Errorf("failed to create library: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get library ID: %w", err)
	}

	return &Library{
		ID:         id,
		Name:       name,
		RootPath:   rootPath,
		SystemID:   systemID,
		SystemName: systemName,
		CreatedAt:  time.Now(),
	}, nil
}

// Get retrieves a library by name.
func (m *Manager) Get(name string) (*Library, error) {
	lib := &Library{}
	var lastScanAt sql.NullTime

	err := m.db.QueryRow(`
		SELECT l.id, l.name, l.root_path, l.system_id, s.name, l.created_at, l.last_scan_at
		FROM libraries l
		JOIN systems s ON l.system_id = s.id
		WHERE l.name = ?
	`, name).Scan(&lib.ID, &lib.Name, &lib.RootPath, &lib.SystemID, &lib.SystemName, &lib.CreatedAt, &lastScanAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("library not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get library: %w", err)
	}

	if lastScanAt.Valid {
		lib.LastScanAt = &lastScanAt.Time
	}

	return lib, nil
}

// List returns all libraries.
func (m *Manager) List() ([]*Library, error) {
	rows, err := m.db.Query(`
		SELECT l.id, l.name, l.root_path, l.system_id, s.name, l.created_at, l.last_scan_at
		FROM libraries l
		JOIN systems s ON l.system_id = s.id
		ORDER BY l.name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list libraries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var libraries []*Library
	for rows.Next() {
		lib := &Library{}
		var lastScanAt sql.NullTime
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.RootPath, &lib.SystemID, &lib.SystemName, &lib.CreatedAt, &lastScanAt); err != nil {
			return nil, fmt.Errorf("failed to scan library: %w", err)
		}
		if lastScanAt.Valid {
			lib.LastScanAt = &lastScanAt.Time
		}
		libraries = append(libraries, lib)
	}

	return libraries, nil
}

// Delete removes a library and all its scanned files.
func (m *Manager) Delete(name string) error {
	result, err := m.db.Exec("DELETE FROM libraries WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("library not found: %s", name)
	}

	return nil
}

// UpdateLastScan updates the last scan timestamp for a library.
func (m *Manager) UpdateLastScan(libraryID int64) error {
	_, err := m.db.Exec("UPDATE libraries SET last_scan_at = CURRENT_TIMESTAMP WHERE id = ?", libraryID)
	return err
}
