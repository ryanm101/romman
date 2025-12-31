package library

import "testing"

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "abcd", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"", "hello", 5},
		{"hello", "", 5},
	}

	for _, tc := range tests {
		t.Run(tc.a+"_"+tc.b, func(t *testing.T) {
			result := LevenshteinDistance(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestNormalizeForFuzzy(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Super Mario Bros (Europe).zip", "supermariobros"},
		{"SONIC THE HEDGEHOG (USA).bin", "sonicthehedgehog"},
		{"Game Title (Rev 1) [!].rom", "gametitle1"},
		{"Test-Game_v1.2.nes", "testgamev12"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeForFuzzy(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeForFuzzy(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFuzzyMatcher_FindBestMatch(t *testing.T) {
	fm := NewFuzzyMatcher()

	candidates := []FuzzyMatch{
		{ReleaseName: "Super Mario Bros (Europe)", ReleaseID: 1, RomEntryID: 1},
		{ReleaseName: "Super Mario Bros 2 (USA)", ReleaseID: 2, RomEntryID: 2},
		{ReleaseName: "Sonic The Hedgehog (Europe)", ReleaseID: 3, RomEntryID: 3},
	}

	// Should match Super Mario Bros
	match := fm.FindBestMatch("Super Mario Bros.zip", candidates)
	if match == nil {
		t.Fatal("Expected a match, got nil")
	}
	if match.ReleaseID != 1 {
		t.Errorf("Expected ReleaseID 1, got %d", match.ReleaseID)
	}
	if match.Confidence < 0.8 {
		t.Errorf("Expected high confidence, got %f", match.Confidence)
	}
}

func TestFuzzyMatcher_NoMatch(t *testing.T) {
	fm := NewFuzzyMatcher()
	fm.Threshold = 3 // Low threshold

	candidates := []FuzzyMatch{
		{ReleaseName: "Completely Different Game", ReleaseID: 1, RomEntryID: 1},
	}

	// Should not match due to high distance
	match := fm.FindBestMatch("Super Mario Bros.zip", candidates)
	if match != nil {
		t.Errorf("Expected no match, got %+v", match)
	}
}
