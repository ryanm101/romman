package dat

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// Header represents the DAT file header element.
type Header struct {
	Name        string `xml:"name"`
	Description string `xml:"description"`
	Version     string `xml:"version"`
	Date        string `xml:"date"`
	Author      string `xml:"author"`
	Homepage    string `xml:"homepage"`
	URL         string `xml:"url"`
}

// Rom represents a single ROM file within a game.
type Rom struct {
	Name  string `xml:"name,attr"`
	Size  int64  `xml:"size,attr"`
	CRC32 string `xml:"crc,attr"`
	MD5   string `xml:"md5,attr"`
	SHA1  string `xml:"sha1,attr"`
}

// Game represents a game entry in the DAT file.
type Game struct {
	Name        string `xml:"name,attr"`
	CloneOf     string `xml:"cloneof,attr"`
	RomOf       string `xml:"romof,attr"`
	Description string `xml:"description"`
	Roms        []Rom  `xml:"rom"`
}

// DATFile represents a parsed Logiqx DAT file.
type DATFile struct {
	Header Header
	Games  []Game
}

// ParseFile parses a Logiqx XML DAT file from the given path.
func ParseFile(path string) (*DATFile, error) {
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to open DAT file: %w", err)
	}
	defer func() { _ = f.Close() }()

	return Parse(f)
}

// Parse parses a Logiqx XML DAT file from the given reader.
// Uses streaming XML parsing for memory efficiency.
func Parse(r io.Reader) (*DATFile, error) {
	decoder := xml.NewDecoder(r)

	dat := &DATFile{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read XML token: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "header":
				var header Header
				if err := decoder.DecodeElement(&header, &elem); err != nil {
					return nil, fmt.Errorf("failed to decode header: %w", err)
				}
				dat.Header = header

			case "game", "machine":
				var game Game
				if err := decoder.DecodeElement(&game, &elem); err != nil {
					return nil, fmt.Errorf("failed to decode game: %w", err)
				}
				dat.Games = append(dat.Games, game)
			}
		}
	}

	return dat, nil
}
