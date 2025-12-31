package library

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RetroArchPlaylist represents the JSON structure of a .lpl file.
type RetroArchPlaylist struct {
	Version            string                  `json:"version"`
	DefaultCorePath    string                  `json:"default_core_path"`
	DefaultCoreName    string                  `json:"default_core_name"`
	LabelDisplayMode   int                     `json:"label_display_mode"`
	RightThumbnailMode int                     `json:"right_thumbnail_mode"`
	LeftThumbnailMode  int                     `json:"left_thumbnail_mode"`
	SortMode           int                     `json:"sort_mode"`
	Items              []RetroArchPlaylistItem `json:"items"`
}

// RetroArchPlaylistItem represents a single entry in the playlist.
type RetroArchPlaylistItem struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	CorePath string `json:"core_path"`
	CoreName string `json:"core_name"`
	CRC32    string `json:"crc32"`
	DBName   string `json:"db_name"`
}

// RetroArchExporter generates RetroArch-compatible playlists.
type RetroArchExporter struct {
	db      *sql.DB
	manager *Manager
}

// NewRetroArchExporter creates a new exporter.
func NewRetroArchExporter(db *sql.DB) *RetroArchExporter {
	return &RetroArchExporter{
		db:      db,
		manager: NewManager(db),
	}
}

// ExportPlaylist generates a .lpl playlist for a library.
func (e *RetroArchExporter) ExportPlaylist(libraryName, outputPath string) error {
	lib, err := e.manager.Get(libraryName)
	if err != nil {
		return fmt.Errorf("library not found: %w", err)
	}

	// Query matched files with release info
	rows, err := e.db.Query(`
		SELECT 
			sf.path,
			sf.archive_path,
			sf.crc32,
			COALESCE(r.name, sf.path) as label
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON m.rom_entry_id = re.id
		JOIN releases r ON re.release_id = r.id
		WHERE sf.library_id = ?
		ORDER BY r.name
	`, lib.ID)
	if err != nil {
		return fmt.Errorf("failed to query matched files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Get db_name from system
	dbName := getRetroArchDBName(lib.SystemName)

	playlist := RetroArchPlaylist{
		Version:            "1.5",
		DefaultCorePath:    "DETECT",
		DefaultCoreName:    "DETECT",
		LabelDisplayMode:   0,
		RightThumbnailMode: 0,
		LeftThumbnailMode:  0,
		SortMode:           0,
		Items:              []RetroArchPlaylistItem{},
	}

	for rows.Next() {
		var filePath, archivePath, crc32, label string
		var archivePathNull sql.NullString
		if err := rows.Scan(&filePath, &archivePathNull, &crc32, &label); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if archivePathNull.Valid {
			archivePath = archivePathNull.String
		}

		// Format path for RetroArch (zip#entry format for archives)
		romPath := filePath
		if archivePath != "" {
			romPath = filePath + "#" + archivePath
		}

		item := RetroArchPlaylistItem{
			Path:     romPath,
			Label:    label,
			CorePath: "DETECT",
			CoreName: "DETECT",
			CRC32:    crc32 + "|crc",
			DBName:   dbName,
		}
		playlist.Items = append(playlist.Items, item)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write playlist file
	data, err := json.MarshalIndent(playlist, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal playlist: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write playlist: %w", err)
	}

	return nil
}

// getRetroArchDBName maps system names to RetroArch database names.
func getRetroArchDBName(systemName string) string {
	// Common mappings from No-Intro names to RetroArch DB names
	dbNames := map[string]string{
		"Nintendo - Game Boy":                            "Nintendo - Game Boy.lpl",
		"Nintendo - Game Boy Color":                      "Nintendo - Game Boy Color.lpl",
		"Nintendo - Game Boy Advance":                    "Nintendo - Game Boy Advance.lpl",
		"Nintendo - Nintendo Entertainment System":       "Nintendo - Nintendo Entertainment System.lpl",
		"Nintendo - Super Nintendo Entertainment System": "Nintendo - Super Nintendo Entertainment System.lpl",
		"Nintendo - Nintendo 64":                         "Nintendo - Nintendo 64.lpl",
		"Nintendo - Nintendo DS":                         "Nintendo - Nintendo DS.lpl",
		"Sega - Master System - Mark III":                "Sega - Master System - Mark III.lpl",
		"Sega - Mega Drive - Genesis":                    "Sega - Mega Drive - Genesis.lpl",
		"Sega - Game Gear":                               "Sega - Game Gear.lpl",
		"Sony - PlayStation":                             "Sony - PlayStation.lpl",
		"Sony - PlayStation Portable":                    "Sony - PlayStation Portable.lpl",
		"Atari - 2600":                                   "Atari - 2600.lpl",
	}

	if dbName, ok := dbNames[systemName]; ok {
		return dbName
	}
	// Default to system name + .lpl
	return systemName + ".lpl"
}
