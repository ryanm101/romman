package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPreferenceConfig(t *testing.T) {
	cfg := DefaultPreferenceConfig()

	assert.Contains(t, cfg.RegionOrder, "Europe")
	assert.Contains(t, cfg.RegionOrder, "USA")
	assert.Contains(t, cfg.RegionOrder, "Japan")
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"simple integer", "1", 100},
		{"decimal version", "1.2", 120},
		{"invalid returns 0", "abc", 0},
		{"empty returns 0", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsEnglish(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		expected  bool
	}{
		{"has English", []string{"En", "Fr"}, true},
		{"has English word", []string{"English"}, true},
		{"no English", []string{"Ja", "Fr"}, false},
		{"lowercase not matched", []string{"en"}, false},
		{"empty list", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsEnglish(tt.languages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNullableString(t *testing.T) {
	assert.Nil(t, nullableString(""))
	assert.Equal(t, "test", nullableString("test"))
}

func TestStabilityConstants(t *testing.T) {
	// Verify stability ordering (stable should be best)
	assert.Less(t, int(StabilityStable), int(StabilityBeta))
	assert.Less(t, int(StabilityBeta), int(StabilityProto))
	assert.Less(t, int(StabilityProto), int(StabilitySample))
	assert.Less(t, int(StabilitySample), int(StabilityDemo))
}
