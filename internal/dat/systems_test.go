package dat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectSystem_HeaderName(t *testing.T) {
	tests := []struct {
		headerName string
		expected   string
	}{
		{"Nintendo - Nintendo Entertainment System", "nes"},
		{"Nintendo - Super Nintendo Entertainment System", "snes"},
		{"Nintendo - Game Boy Advance", "gba"},
		{"Sega - Mega Drive - Genesis", "md"},
		{"Sony - PlayStation", "psx"},
		{"MAME", "mame"},
		{"Unknown System", ""}, // Should return empty for unknown
	}

	for _, tt := range tests {
		t.Run(tt.headerName, func(t *testing.T) {
			result := DetectSystem(tt.headerName, "")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectSystem_Filename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"Nintendo - NES.dat", "nes"},
		{"snes_roms.dat", "snes"},
		{"Genesis_Collection.dat", "md"},
		{"GBA Games.dat", "gba"},
		{"random_file.dat", ""}, // Unknown
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectSystem("", tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSystemDisplayName(t *testing.T) {
	tests := []struct {
		systemID string
		expected string
	}{
		{"nes", "Nintendo Entertainment System"},
		{"snes", "Super Nintendo Entertainment System"},
		{"md", "Sega Genesis / Mega Drive"},
		{"psx", "Sony PlayStation"},
		{"unknown", "unknown"}, // Falls back to ID
	}

	for _, tt := range tests {
		t.Run(tt.systemID, func(t *testing.T) {
			result := GetSystemDisplayName(tt.systemID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
