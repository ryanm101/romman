package dat

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SystemMapping maps DAT file naming patterns to RetroArch system names.
// Keys are lowercase patterns found in DAT names or filenames.
var SystemMapping = map[string]string{
	// Nintendo
	"nintendo - nintendo entertainment system":       "nes",
	"nintendo - family computer":                     "nes",
	"nintendo - super nintendo entertainment system": "snes",
	"nintendo - super famicom":                       "snes",
	"nintendo - game boy":                            "gb",
	"nintendo - game boy color":                      "gbc",
	"nintendo - game boy advance":                    "gba",
	"nintendo - nintendo 64":                         "n64",
	"nintendo - nintendo ds":                         "nds",
	"nintendo - nintendo dsi":                        "nds",
	"nintendo - nintendo 3ds":                        "3ds",
	"nintendo - nintendo gamecube":                   "gc",
	"nintendo - wii":                                 "wii",
	"nintendo - virtual boy":                         "vb",
	"nintendo - pokemon mini":                        "pokemini",

	// Sega
	"sega - master system - mark iii": "sms",
	"sega - mega drive - genesis":     "md",
	"sega - genesis":                  "md",
	"sega - mega drive":               "md",
	"sega - game gear":                "gg",
	"sega - 32x":                      "32x",
	"sega - mega-cd - sega cd":        "segacd",
	"sega - sega cd":                  "segacd",
	"sega - saturn":                   "saturn",
	"sega - dreamcast":                "dc",
	"sega - sg-1000":                  "sg1000",

	// Sony
	"sony - playstation":          "psx",
	"sony - playstation 2":        "ps2",
	"sony - playstation portable": "psp",
	"sony - playstation vita":     "vita",

	// Atari
	"atari - 2600":   "atari2600",
	"atari - 5200":   "atari5200",
	"atari - 7800":   "atari7800",
	"atari - jaguar": "atarijaguar",
	"atari - lynx":   "atarilynx",
	"atari - st":     "atarist",

	// NEC
	"nec - pc engine - turbografx-16":    "pce",
	"nec - pc engine supergrafx":         "pce",
	"nec - pc engine cd - turbografx-cd": "pcecd",
	"nec - pc-fx":                        "pcfx",

	// SNK
	"snk - neo geo pocket":       "ngp",
	"snk - neo geo pocket color": "ngpc",
	"snk - neo geo cd":           "neogeocd",

	// Other
	"bandai - wonderswan":       "wswan",
	"bandai - wonderswan color": "wswanc",
	"coleco - colecovision":     "coleco",
	"gce - vectrex":             "vectrex",
	"mattel - intellivision":    "intv",
	"magnavox - odyssey2":       "odyssey2",
	"panasonic - 3do":           "3do",
	"philips - videopac":        "odyssey2",
	"sinclair - zx spectrum":    "zxspectrum",
	"commodore - 64":            "c64",
	"commodore - amiga":         "amiga",
	"microsoft - msx":           "msx",
	"microsoft - msx2":          "msx2",

	// Arcade
	"mame":    "mame",
	"fbneo":   "fbneo",
	"fbalpha": "fba",
}

// nonAlphaNum strips non-alphanumeric chars for fuzzy matching
var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// DetectSystem attempts to detect the RetroArch system name from DAT metadata.
// It tries the header name first, then falls back to filename-based detection.
func DetectSystem(headerName, datFilename string) string {
	// Try header name first (most reliable)
	if sys := detectFromName(headerName); sys != "" {
		return sys
	}

	// Try filename-based detection
	if sys := detectFromFilename(datFilename); sys != "" {
		return sys
	}

	// Return empty if unknown - caller can use header name as fallback
	return ""
}

func detectFromName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))

	// Exact match first
	if sys, ok := SystemMapping[lower]; ok {
		return sys
	}

	// Prefix match for variations
	for pattern, sys := range SystemMapping {
		if strings.HasPrefix(lower, pattern) {
			return sys
		}
	}

	return ""
}

func detectFromFilename(filename string) string {
	// Get base name without extension
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Normalize: lowercase and strip special chars
	normalized := nonAlphaNum.ReplaceAllString(strings.ToLower(name), "")

	// Check patterns in order of priority (longer/more specific first)
	// This prevents "gb" from matching before "gba" etc.
	orderedPatterns := []struct {
		pattern string
		sys     string
	}{
		// Multi-word / longer patterns first
		{"mastersystem", "sms"},
		{"megadrive", "md"},
		{"mega-drive", "md"},
		{"gamecube", "gc"},
		{"dreamcast", "dc"},
		{"playstation", "psx"},
		{"turbografx", "pce"},
		{"wonderswan", "wswan"},
		{"atarilynx", "atarilynx"},
		{"atarijaguar", "atarijaguar"},
		{"atari2600", "atari2600"},
		{"atari7800", "atari7800"},
		{"coleco", "coleco"},
		{"vectrex", "vectrex"},
		{"genesis", "md"},
		{"gamegear", "gg"},
		{"segacd", "segacd"},
		{"saturn", "saturn"},
		{"neogeo", "neogeo"},
		{"fbneo", "fbneo"},
		// Shorter 3-letter codes (order matters: gba before gb)
		{"snes", "snes"},
		{"gba", "gba"},
		{"gbc", "gbc"},
		{"nes", "nes"},
		{"nds", "nds"},
		{"3ds", "3ds"},
		{"n64", "n64"},
		{"wii", "wii"},
		{"sms", "sms"},
		{"32x", "32x"},
		{"psx", "psx"},
		{"ps2", "ps2"},
		{"psp", "psp"},
		{"pce", "pce"},
		{"ngp", "ngp"},
		{"ngpc", "ngpc"},
		{"mame", "mame"},
		{"lynx", "atarilynx"},
		{"2600", "atari2600"},
		{"7800", "atari7800"},
		{"jaguar", "atarijaguar"},
		{"intv", "intv"},
		{"c64", "c64"},
		{"amiga", "amiga"},
		{"gb", "gb"}, // Must come after gba, gbc
		{"gc", "gc"},
		{"dc", "dc"},
		{"gg", "gg"},
	}

	for _, p := range orderedPatterns {
		if strings.Contains(normalized, p.pattern) {
			return p.sys
		}
	}

	return ""
}

// GetSystemDisplayName returns a human-readable name for a RetroArch system ID.
func GetSystemDisplayName(systemID string) string {
	displayNames := map[string]string{
		"nes":         "Nintendo Entertainment System",
		"snes":        "Super Nintendo Entertainment System",
		"gb":          "Game Boy",
		"gbc":         "Game Boy Color",
		"gba":         "Game Boy Advance",
		"n64":         "Nintendo 64",
		"nds":         "Nintendo DS",
		"3ds":         "Nintendo 3DS",
		"gc":          "Nintendo GameCube",
		"wii":         "Nintendo Wii",
		"vb":          "Virtual Boy",
		"pokemini":    "Pokemon Mini",
		"sms":         "Sega Master System",
		"md":          "Sega Genesis / Mega Drive",
		"gg":          "Sega Game Gear",
		"32x":         "Sega 32X",
		"segacd":      "Sega CD",
		"saturn":      "Sega Saturn",
		"dc":          "Sega Dreamcast",
		"sg1000":      "Sega SG-1000",
		"psx":         "Sony PlayStation",
		"ps2":         "Sony PlayStation 2",
		"psp":         "Sony PlayStation Portable",
		"vita":        "Sony PlayStation Vita",
		"atari2600":   "Atari 2600",
		"atari5200":   "Atari 5200",
		"atari7800":   "Atari 7800",
		"atarijaguar": "Atari Jaguar",
		"atarilynx":   "Atari Lynx",
		"atarist":     "Atari ST",
		"pce":         "PC Engine / TurboGrafx-16",
		"pcecd":       "PC Engine CD",
		"pcfx":        "PC-FX",
		"ngp":         "Neo Geo Pocket",
		"ngpc":        "Neo Geo Pocket Color",
		"neogeocd":    "Neo Geo CD",
		"wswan":       "WonderSwan",
		"wswanc":      "WonderSwan Color",
		"coleco":      "ColecoVision",
		"vectrex":     "Vectrex",
		"intv":        "Intellivision",
		"odyssey2":    "Odyssey 2",
		"3do":         "3DO",
		"zxspectrum":  "ZX Spectrum",
		"c64":         "Commodore 64",
		"amiga":       "Amiga",
		"msx":         "MSX",
		"msx2":        "MSX2",
		"mame":        "MAME",
		"fbneo":       "FinalBurn Neo",
		"fba":         "FinalBurn Alpha",
	}

	if name, ok := displayNames[systemID]; ok {
		return name
	}
	return systemID
}
