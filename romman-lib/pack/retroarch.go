package pack

import (
	"archive/zip"
	"encoding/json"
	"path/filepath"
)

// RetroArchExporter creates RetroArch-compatible packs with playlists.
type RetroArchExporter struct{}

// Format returns the format this exporter produces.
func (e *RetroArchExporter) Format() Format {
	return FormatRetroArch
}

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

// Export writes games to the zip with RetroArch-compatible structure.
// Structure: roms/<system>/<game>.ext + playlists/<system>.lpl
func (e *RetroArchExporter) Export(games []Game, zw *zip.Writer) error {
	if len(games) == 0 {
		return ErrNoGames
	}

	// Group games by system
	systemGames := make(map[string][]Game)
	for _, game := range games {
		systemGames[game.System] = append(systemGames[game.System], game)
	}

	// Process each system
	for system, sysGames := range systemGames {
		// Create playlist for this system
		playlist := RetroArchPlaylist{
			Version:            "1.5",
			DefaultCorePath:    "DETECT",
			DefaultCoreName:    "DETECT",
			LabelDisplayMode:   0,
			RightThumbnailMode: 0,
			LeftThumbnailMode:  0,
			SortMode:           0,
			Items:              make([]RetroArchPlaylistItem, 0, len(sysGames)),
		}

		// Determine db_name (use system display name if available)
		dbName := system + ".lpl"
		if len(sysGames) > 0 && sysGames[0].SystemName != "" {
			dbName = sysGames[0].SystemName + ".lpl"
		}

		// Add each game
		for _, game := range sysGames {
			// ROM path in zip: roms/<system>/<filename>
			romZipPath := filepath.Join("roms", system, game.FileName)

			// Add ROM file to zip
			if err := addFileToZip(zw, game.FilePath, romZipPath); err != nil {
				return err
			}

			// Playlist paths are relative from RetroArch's perspective
			// Using portable path format: /roms/<system>/<filename>
			playlistPath := "/" + filepath.ToSlash(romZipPath)

			playlist.Items = append(playlist.Items, RetroArchPlaylistItem{
				Path:     playlistPath,
				Label:    game.Name,
				CorePath: "DETECT",
				CoreName: "DETECT",
				CRC32:    "|crc",
				DBName:   dbName,
			})
		}

		// Write playlist to zip
		playlistPath := filepath.Join("playlists", dbName)
		playlistData, err := json.MarshalIndent(playlist, "", "  ")
		if err != nil {
			return err
		}

		pw, err := zw.Create(playlistPath)
		if err != nil {
			return err
		}
		if _, err := pw.Write(playlistData); err != nil {
			return err
		}
	}

	return nil
}
