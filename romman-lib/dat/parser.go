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

// Game represents a game/machine entry in the DAT file.
// Supports both Logiqx DAT format and MAME XML format.
type Game struct {
	Name         string `xml:"name,attr"`
	ID           string `xml:"id,attr"`        // Some DATs use numeric IDs
	CloneOf      string `xml:"cloneof,attr"`   // Standard: references parent by name
	CloneOfID    string `xml:"cloneofid,attr"` // Variant: references parent by ID
	RomOf        string `xml:"romof,attr"`     // MAME: parent set for shared ROMs
	SampleOf     string `xml:"sampleof,attr"`  // MAME: parent set for shared samples
	Description  string `xml:"description"`
	Year         string `xml:"year"`         // MAME: release year
	Manufacturer string `xml:"manufacturer"` // MAME: manufacturer name
	Roms         []Rom  `xml:"rom"`

	// MAME-specific attributes
	IsBIOS     string `xml:"isbios,attr"`       // "yes" or "no"
	IsDevice   string `xml:"isdevice,attr"`     // "yes" or "no"
	IsMech     string `xml:"ismechanical,attr"` // "yes" or "no"
	Runnable   string `xml:"runnable,attr"`     // "yes" or "no"
	SourceFile string `xml:"sourcefile,attr"`   // Driver source file
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

	// Post-process: resolve CloneOfID to CloneOf (name) if needed
	resolveCloneOfIDs(dat)

	return dat, nil
}

// resolveCloneOfIDs converts cloneofid references to cloneof (name-based) references.
// Some DAT files use numeric IDs instead of game names for parent/clone relationships.
func resolveCloneOfIDs(dat *DATFile) {
	// Build ID -> Name mapping
	idToName := make(map[string]string)
	for _, game := range dat.Games {
		if game.ID != "" {
			idToName[game.ID] = game.Name
		}
	}

	// Resolve CloneOfID to CloneOf
	for i := range dat.Games {
		if dat.Games[i].CloneOfID != "" && dat.Games[i].CloneOf == "" {
			if parentName, ok := idToName[dat.Games[i].CloneOfID]; ok {
				dat.Games[i].CloneOf = parentName
			}
		}
	}
}
