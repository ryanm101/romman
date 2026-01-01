package pack

import (
	"archive/zip"
	"encoding/xml"
	"path/filepath"
)

// EmulationStationExporter creates EmulationStation-compatible packs with gamelist.xml.
type EmulationStationExporter struct{}

// Format returns the format this exporter produces.
func (e *EmulationStationExporter) Format() Format {
	return FormatEmulationStation
}

// GameList represents the XML structure of a gamelist.xml file.
type GameList struct {
	XMLName xml.Name       `xml:"gameList"`
	Games   []GameListGame `xml:"game"`
}

// GameListGame represents a single game entry in gamelist.xml.
type GameListGame struct {
	Path        string `xml:"path"`
	Name        string `xml:"name"`
	Description string `xml:"desc,omitempty"`
	Image       string `xml:"image,omitempty"`
	Rating      string `xml:"rating,omitempty"`
	ReleaseDate string `xml:"releasedate,omitempty"`
	Developer   string `xml:"developer,omitempty"`
	Publisher   string `xml:"publisher,omitempty"`
	Genre       string `xml:"genre,omitempty"`
	Players     string `xml:"players,omitempty"`
}

// Export writes games to the zip with EmulationStation-compatible structure.
// Structure: roms/<system>/<game>.ext + roms/<system>/gamelist.xml
func (e *EmulationStationExporter) Export(games []Game, zw *zip.Writer) error {
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
		gamelist := GameList{
			Games: make([]GameListGame, 0, len(sysGames)),
		}

		// Add each game
		for _, game := range sysGames {
			// ROM path in zip: roms/<system>/<filename>
			romZipPath := filepath.Join("roms", system, game.FileName)

			// Add ROM file to zip
			if err := addFileToZip(zw, game.FilePath, romZipPath); err != nil {
				return err
			}

			// Gamelist path is relative (just the filename)
			gamelist.Games = append(gamelist.Games, GameListGame{
				Path: "./" + game.FileName,
				Name: game.Name,
			})
		}

		// Write gamelist.xml to zip
		gamelistPath := filepath.Join("roms", system, "gamelist.xml")
		gamelistData, err := xml.MarshalIndent(gamelist, "", "  ")
		if err != nil {
			return err
		}

		// Add XML header
		fullXML := append([]byte(xml.Header), gamelistData...)

		gw, err := zw.Create(gamelistPath)
		if err != nil {
			return err
		}
		if _, err := gw.Write(fullXML); err != nil {
			return err
		}
	}

	return nil
}
