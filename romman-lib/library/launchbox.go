package library

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"time"
)

// LBGame represents a game entry in LaunchBox's platform XML.
type LBGame struct {
	XMLName         xml.Name `xml:"Game"`
	Title           string   `xml:"Title"`
	Platform        string   `xml:"Platform"`
	ApplicationPath string   `xml:"ApplicationPath"`
	Developer       string   `xml:"Developer,omitempty"`
	Publisher       string   `xml:"Publisher,omitempty"`
	ReleaseDate     string   `xml:"ReleaseDate,omitempty"`
	Genre           string   `xml:"Genre,omitempty"`
	MaxPlayers      string   `xml:"MaxPlayers,omitempty"`
	Region          string   `xml:"Region,omitempty"`
	Notes           string   `xml:"Notes,omitempty"`
	Source          string   `xml:"Source,omitempty"`
}

// LBPlatformXML represents the root LaunchBox platform XML structure.
type LBPlatformXML struct {
	XMLName xml.Name `xml:"LaunchBox"`
	Games   []LBGame `xml:"Game"`
}

// LaunchBoxOptions configures the LaunchBox export.
type LaunchBoxOptions struct {
	MatchedOnly bool   // Only include matched games
	PathPrefix  string // Prefix for ApplicationPath (e.g., ".\\ROMs\\nes\\")
}

// ExportLaunchBox generates a LaunchBox platform XML for a library.
func (e *Exporter) ExportLaunchBox(ctx context.Context, libraryName string, opts LaunchBoxOptions) ([]byte, error) {
	lib, err := e.manager.Get(ctx, libraryName)
	if err != nil {
		return nil, err
	}

	var games []LBGame

	if opts.MatchedOnly {
		games, err = e.getMatchedLaunchBox(ctx, lib.ID, lib.SystemName, opts)
	} else {
		games, err = e.getAllReleasesLaunchBox(ctx, lib.SystemID, lib.ID, lib.SystemName, opts)
	}

	if err != nil {
		return nil, err
	}

	lbXML := LBPlatformXML{Games: games}

	output, err := xml.MarshalIndent(lbXML, "", "  ")
	if err != nil {
		return nil, err
	}

	// Add XML header
	return append([]byte(xml.Header), output...), nil
}

func (e *Exporter) getMatchedLaunchBox(ctx context.Context, libraryID int64, systemName string, opts LaunchBoxOptions) ([]LBGame, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT DISTINCT r.name, sf.path
		FROM scanned_files sf
		JOIN matches m ON m.scanned_file_id = sf.id
		JOIN rom_entries re ON re.id = m.rom_entry_id
		JOIN releases r ON r.id = re.release_id
		WHERE sf.library_id = ?
		ORDER BY r.name
	`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var games []LBGame
	for rows.Next() {
		var name, path string
		if err := rows.Scan(&name, &path); err != nil {
			return nil, err
		}

		game := LBGame{
			Title:           name,
			Platform:        formatPlatformName(systemName),
			ApplicationPath: formatLBPath(path, opts.PathPrefix),
			Source:          "romman",
			Region:          extractRegion(name),
		}

		games = append(games, game)
	}

	return games, nil
}

func (e *Exporter) getAllReleasesLaunchBox(ctx context.Context, systemID, libraryID int64, systemName string, opts LaunchBoxOptions) ([]LBGame, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT r.name, COALESCE(sf.path, '') as path
		FROM releases r
		LEFT JOIN rom_entries re ON re.release_id = r.id
		LEFT JOIN matches m ON m.rom_entry_id = re.id
		LEFT JOIN scanned_files sf ON sf.id = m.scanned_file_id AND sf.library_id = ?
		WHERE r.system_id = ?
		GROUP BY r.id
		ORDER BY r.name
	`, libraryID, systemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var games []LBGame
	for rows.Next() {
		var name, path string
		if err := rows.Scan(&name, &path); err != nil {
			return nil, err
		}

		game := LBGame{
			Title:    name,
			Platform: formatPlatformName(systemName),
			Source:   "romman",
			Region:   extractRegion(name),
		}

		if path != "" {
			game.ApplicationPath = formatLBPath(path, opts.PathPrefix)
		}

		games = append(games, game)
	}

	return games, nil
}

func formatLBPath(path, prefix string) string {
	if prefix != "" {
		return prefix + filepath.Base(path)
	}
	// LaunchBox uses Windows-style paths
	return ".\\" + filepath.Base(path)
}

// formatPlatformName converts system names to LaunchBox platform names.
func formatPlatformName(systemName string) string {
	// Common mappings from RomMan system names to LaunchBox platform names
	platformMap := map[string]string{
		"gb":           "Nintendo Game Boy",
		"gba":          "Nintendo Game Boy Advance",
		"gbc":          "Nintendo Game Boy Color",
		"nes":          "Nintendo Entertainment System",
		"snes":         "Super Nintendo Entertainment System",
		"n64":          "Nintendo 64",
		"nds":          "Nintendo DS",
		"3ds":          "Nintendo 3DS",
		"gamecube":     "Nintendo GameCube",
		"wii":          "Nintendo Wii",
		"wiiu":         "Nintendo Wii U",
		"switch":       "Nintendo Switch",
		"genesis":      "Sega Genesis",
		"megadrive":    "Sega Genesis",
		"mastersystem": "Sega Master System",
		"gamegear":     "Sega Game Gear",
		"saturn":       "Sega Saturn",
		"dreamcast":    "Sega Dreamcast",
		"psx":          "Sony Playstation",
		"ps2":          "Sony Playstation 2",
		"psp":          "Sony PSP",
		"ps3":          "Sony Playstation 3",
		"neogeo":       "SNK Neo Geo AES",
		"neogeocd":     "SNK Neo Geo CD",
		"arcade":       "Arcade",
		"mame":         "Arcade",
	}

	if mapped, ok := platformMap[systemName]; ok {
		return mapped
	}
	return systemName
}

// FormatLBDate formats a time.Time to LaunchBox's date format.
func FormatLBDate(t time.Time) string {
	return t.Format("2006-01-02")
}
