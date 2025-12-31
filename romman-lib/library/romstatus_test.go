package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFilenameStatus_Verified(t *testing.T) {
	status := ParseFilenameStatus("Super Mario Bros [!].nes")
	assert.True(t, status.IsVerified)
	assert.Equal(t, "Super Mario Bros", status.BaseTitle)
}

func TestParseFilenameStatus_BadDump(t *testing.T) {
	tests := []string{
		"Game [b].nes",
		"Game [b1].nes",
		"Game [b2].nes",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			status := ParseFilenameStatus(tt)
			assert.True(t, status.IsBadDump)
		})
	}
}

func TestParseFilenameStatus_Regions(t *testing.T) {
	tests := []struct {
		filename string
		region   string
	}{
		{"Game (USA).nes", "USA"},
		{"Game (Europe).nes", "Europe"},
		{"Game (Japan).nes", "Japan"},
		{"Game (U).nes", "U"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			status := ParseFilenameStatus(tt.filename)
			assert.Equal(t, tt.region, status.Region)
		})
	}
}

func TestParseFilenameStatus_AllFlags(t *testing.T) {
	status := ParseFilenameStatus("Game [c].nes")
	assert.True(t, status.IsCracked)

	status = ParseFilenameStatus("Game [f].nes")
	assert.True(t, status.IsFixed)

	status = ParseFilenameStatus("Game [h].nes")
	assert.True(t, status.IsHack)

	status = ParseFilenameStatus("Game [o].nes")
	assert.True(t, status.IsOverdump)

	status = ParseFilenameStatus("Game [p].nes")
	assert.True(t, status.IsPirate)

	status = ParseFilenameStatus("Game [t].nes")
	assert.True(t, status.IsTrainer)

	status = ParseFilenameStatus("Game [T+Eng].nes")
	assert.True(t, status.IsTranslated)

	status = ParseFilenameStatus("Game [a].nes")
	assert.True(t, status.IsAlternate)
}

func TestROMStatus_GetStatusFlags(t *testing.T) {
	status := ROMStatus{IsVerified: true, IsBadDump: true}
	flags := status.GetStatusFlags()
	assert.Contains(t, flags, "verified")
	assert.Contains(t, flags, "bad-dump")
}

func TestROMStatus_GetStatusFlags_Empty(t *testing.T) {
	status := ROMStatus{}
	assert.Empty(t, status.GetStatusFlags())
}

func TestROMStatus_IsModified(t *testing.T) {
	modified := ROMStatus{IsCracked: true}
	assert.True(t, modified.IsModified())

	clean := ROMStatus{IsVerified: true}
	assert.False(t, clean.IsModified())
}

func TestROMStatus_IsProblematic(t *testing.T) {
	problematic := ROMStatus{IsBadDump: true}
	assert.True(t, problematic.IsProblematic())

	good := ROMStatus{IsVerified: true}
	assert.False(t, good.IsProblematic())
}

func TestNormalizeTitleForMatching(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Super Mario Bros (USA).nes", "supermariobros"},
		{"Game [!] (Europe).zip", "game"},
		{"File-With_Special.rom", "filewithspecial"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeTitleForMatching(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRegionCode(t *testing.T) {
	assert.True(t, isRegionCode("USA"))
	assert.True(t, isRegionCode("EUROPE"))
	assert.True(t, isRegionCode("U"))
	assert.True(t, isRegionCode("JUE"))
	assert.False(t, isRegionCode("UNKNOWN"))
	assert.False(t, isRegionCode("REV1"))
}

func TestIsDigit(t *testing.T) {
	assert.True(t, isDigit('0'))
	assert.True(t, isDigit('5'))
	assert.True(t, isDigit('9'))
	assert.False(t, isDigit('a'))
	assert.False(t, isDigit('!'))
}
