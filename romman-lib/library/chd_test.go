package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCHDFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/game.chd", true},
		{"/path/to/game.CHD", true},
		{"/path/to/game.rom", false},
		{"/path/to/game.iso", false},
		{"/path/to/.chd", true},
		{"game.chd", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsCHDFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetExtLower(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"game.CHD", ".chd"},
		{"game.ROM", ".rom"},
		{"file.TXT", ".txt"},
		{"no_extension", ""},
		{"/path/to/file.ISO", ".iso"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getExtLower(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCHDInfo_Struct(t *testing.T) {
	info := &CHDInfo{
		Version:      5,
		TotalHunks:   1000,
		HunkBytes:    4096,
		LogicalBytes: 4096000,
		SHA1:         "abc123",
		DataSHA1:     "def456",
		ParentSHA1:   "ghi789",
	}

	assert.Equal(t, uint32(5), info.Version)
	assert.Equal(t, uint32(1000), info.TotalHunks)
	assert.Equal(t, uint32(4096), info.HunkBytes)
	assert.Equal(t, uint64(4096000), info.LogicalBytes)
}

func TestParseCHD_NotFound(t *testing.T) {
	_, err := ParseCHD("/nonexistent/file.chd")
	assert.Error(t, err)
}
