package library

import (
	"path/filepath"
	"strings"
	"unicode"
)

// FuzzyMatcher provides fuzzy string matching for ROM filenames.
type FuzzyMatcher struct {
	Threshold int // Maximum Levenshtein distance to consider a match (default: 5)
}

// NewFuzzyMatcher creates a new fuzzy matcher with default threshold.
func NewFuzzyMatcher() *FuzzyMatcher {
	return &FuzzyMatcher{Threshold: 5}
}

// FuzzyMatch represents a potential fuzzy match result.
type FuzzyMatch struct {
	ReleaseName string
	ReleaseID   int64
	RomEntryID  int64
	Distance    int
	Confidence  float64 // 0.0 to 1.0, higher is better
}

// LevenshteinDistance computes the edit distance between two strings.
func LevenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Normalize for comparison
	a = normalizeForFuzzy(a)
	b = normalizeForFuzzy(b)

	if a == b {
		return 0
	}

	// Create matrix
	lenA, lenB := len(a), len(b)
	prev := make([]int, lenB+1)
	curr := make([]int, lenB+1)

	for j := 0; j <= lenB; j++ {
		prev[j] = j
	}

	for i := 1; i <= lenA; i++ {
		curr[0] = i
		for j := 1; j <= lenB; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[lenB]
}

// normalizeForFuzzy prepares a string for fuzzy comparison.
func normalizeForFuzzy(s string) string {
	// Remove file extension
	s = strings.TrimSuffix(s, filepath.Ext(s))

	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove common ROM naming tokens
	tokens := []string{
		"(europe)", "(usa)", "(japan)", "(world)",
		"(en)", "(ja)", "(fr)", "(de)", "(es)", "(it)",
		"(rev ", "(v1.", "(v2.", "(proto)", "(beta)", "(demo)",
		"[!]", "[b]", "[a]", "[h]", "[o]", "[t]",
	}
	for _, t := range tokens {
		s = strings.ReplaceAll(s, t, "")
	}

	// Remove non-alphanumeric characters
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// FindBestMatch finds the closest matching release for a filename.
func (fm *FuzzyMatcher) FindBestMatch(filename string, candidates []FuzzyMatch) *FuzzyMatch {
	normalizedFilename := normalizeForFuzzy(filename)

	var bestMatch *FuzzyMatch
	bestDistance := fm.Threshold + 1

	for i := range candidates {
		candidate := &candidates[i]
		normalizedRelease := normalizeForFuzzy(candidate.ReleaseName)

		distance := LevenshteinDistance(normalizedFilename, normalizedRelease)
		candidate.Distance = distance

		// Calculate confidence (higher is better)
		maxLen := max(len(normalizedFilename), len(normalizedRelease))
		if maxLen > 0 {
			candidate.Confidence = 1.0 - float64(distance)/float64(maxLen)
		}

		if distance < bestDistance && distance <= fm.Threshold {
			bestDistance = distance
			bestMatch = candidate
		}
	}

	return bestMatch
}

// min returns the minimum of three integers.
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
