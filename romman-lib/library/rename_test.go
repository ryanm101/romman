package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no change needed", "game.rom", "game.rom"},
		{"replaces slash", "game/version.rom", "game-version.rom"},
		{"replaces backslash", "game\\version.rom", "game-version.rom"},
		{"replaces colon", "game:version.rom", "game -version.rom"},
		{"removes asterisk", "game*version.rom", "gameversion.rom"},
		{"removes question mark", "game?version.rom", "gameversion.rom"},
		{"replaces quotes", "game\"version\".rom", "game'version'.rom"},
		{"removes angle brackets", "game<version>.rom", "gameversion.rom"},
		{"replaces pipe", "game|version.rom", "game-version.rom"},
		{"handles multiple invalid chars", "g/a\\m:e*.rom", "g-a-m -e.rom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenameActionStatus(t *testing.T) {
	action := RenameAction{
		OldPath: "/old/path.rom",
		NewPath: "/new/path.rom",
		Status:  "pending",
	}

	assert.Equal(t, "/old/path.rom", action.OldPath)
	assert.Equal(t, "/new/path.rom", action.NewPath)
	assert.Equal(t, "pending", action.Status)
}

func TestRenameResultDefaults(t *testing.T) {
	result := &RenameResult{DryRun: true}

	assert.True(t, result.DryRun)
	assert.Equal(t, 0, result.Renamed)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 0, result.Errors)
}
