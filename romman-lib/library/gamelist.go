package library

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"time"
)

// GamelistGame represents a single game entry in EmulationStation's gamelist.xml.
type GamelistGame struct {
	XMLName     xml.Name `xml:"game"`
	Path        string   `xml:"path"`
	Name        string   `xml:"name"`
	Desc        string   `xml:"desc,omitempty"`
	Image       string   `xml:"image,omitempty"`
	Rating      string   `xml:"rating,omitempty"`
	ReleaseDate string   `xml:"releasedate,omitempty"`
	Developer   string   `xml:"developer,omitempty"`
	Publisher   string   `xml:"publisher,omitempty"`
	Genre       string   `xml:"genre,omitempty"`
	Players     string   `xml:"players,omitempty"`
}

// GamelistXML represents the root gamelist.xml structure.
type GamelistXML struct {
	XMLName xml.Name       `xml:"gameList"`
	Games   []GamelistGame `xml:"game"`
}

// GamelistOptions configures the gamelist export.
type GamelistOptions struct {
	MatchedOnly bool   // Only include matched games
	PathPrefix  string // Prefix to prepend to paths (e.g., "./roms/")
	ImageDir    string // Directory for images (e.g., "./images/")
}

// ExportGamelist generates an EmulationStation gamelist.xml for a library.
func (e *Exporter) ExportGamelist(ctx context.Context, libraryName string, opts GamelistOptions) ([]byte, error) {
	lib, err := e.manager.Get(ctx, libraryName)
	if err != nil {
		return nil, err
	}

	var games []GamelistGame

	if opts.MatchedOnly {
		// Export only matched games with their file paths
		games, err = e.getMatchedGamelist(ctx, lib.ID, opts)
	} else {
		// Export all releases for the system (including missing)
		games, err = e.getAllReleasesGamelist(ctx, lib.SystemID, lib.ID, opts)
	}

	if err != nil {
		return nil, err
	}

	gamelist := GamelistXML{Games: games}

	output, err := xml.MarshalIndent(gamelist, "", "  ")
	if err != nil {
		return nil, err
	}

	// Add XML header
	return append([]byte(xml.Header), output...), nil
}

func (e *Exporter) getMatchedGamelist(ctx context.Context, libraryID int64, opts GamelistOptions) ([]GamelistGame, error) {
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

	var games []GamelistGame
	for rows.Next() {
		var name, path string
		if err := rows.Scan(&name, &path); err != nil {
			return nil, err
		}

		game := GamelistGame{
			Name: name,
			Path: formatGamelistPath(path, opts.PathPrefix),
		}

		// Add image path if directory specified
		if opts.ImageDir != "" {
			baseName := filepath.Base(path)
			ext := filepath.Ext(baseName)
			imageName := baseName[:len(baseName)-len(ext)] + "-image.png"
			game.Image = filepath.Join(opts.ImageDir, imageName)
		}

		games = append(games, game)
	}

	return games, nil
}

func (e *Exporter) getAllReleasesGamelist(ctx context.Context, systemID, libraryID int64, opts GamelistOptions) ([]GamelistGame, error) {
	// Get all releases, left join to matches to include status
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

	var games []GamelistGame
	for rows.Next() {
		var name, path string
		if err := rows.Scan(&name, &path); err != nil {
			return nil, err
		}

		game := GamelistGame{
			Name: name,
		}

		if path != "" {
			game.Path = formatGamelistPath(path, opts.PathPrefix)
			if opts.ImageDir != "" {
				baseName := filepath.Base(path)
				ext := filepath.Ext(baseName)
				imageName := baseName[:len(baseName)-len(ext)] + "-image.png"
				game.Image = filepath.Join(opts.ImageDir, imageName)
			}
		}

		games = append(games, game)
	}

	return games, nil
}

func formatGamelistPath(path, prefix string) string {
	if prefix != "" {
		return prefix + filepath.Base(path)
	}
	return path
}

// FormatESDate formats a time.Time to EmulationStation's date format.
func FormatESDate(t time.Time) string {
	return t.Format("20060102T150405")
}
