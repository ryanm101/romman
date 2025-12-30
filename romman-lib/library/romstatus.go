package library

import (
	"regexp"
	"strings"
)

// ROMStatus represents the quality/status of a ROM file based on filename tags.
type ROMStatus struct {
	IsVerified   bool   // [!] - verified good dump
	IsBadDump    bool   // [b] or [b1], [b2], etc. - bad dump
	IsCracked    bool   // [c] - cracked (copy protection removed)
	IsFixed      bool   // [f] - fixed (bugs patched)
	IsHack       bool   // [h] - hack
	IsOverdump   bool   // [o] - overdump
	IsPirate     bool   // [p] - pirate
	IsTrainer    bool   // [t] - trainer
	IsTranslated bool   // [T] - translated
	IsAlternate  bool   // [a] or [a1], [a2], etc. - alternate version
	BaseTitle    string // Title without region/tags
	Region       string // Region code (USA, Europe, Japan, etc.)
}

var (
	// Matches GoodTools-style tags in brackets
	bracketTagRegex = regexp.MustCompile(`\[([^\]]+)\]`)
	// Matches region codes in parentheses
	regionRegex = regexp.MustCompile(`\(([^)]+)\)`)
)

// ParseFilenameStatus extracts ROM status from a filename.
func ParseFilenameStatus(filename string) ROMStatus {
	status := ROMStatus{}

	// Extract base title (everything before first parenthesis or bracket)
	baseEnd := len(filename)
	if idx := strings.IndexAny(filename, "(["); idx != -1 {
		baseEnd = idx
	}
	status.BaseTitle = strings.TrimSpace(filename[:baseEnd])

	// Extract region from parentheses
	regionMatches := regionRegex.FindAllStringSubmatch(filename, -1)
	for _, match := range regionMatches {
		content := match[1]
		// Skip revision markers
		if strings.HasPrefix(strings.ToUpper(content), "REV") {
			continue
		}
		// Check for common region codes
		upper := strings.ToUpper(content)
		if isRegionCode(upper) {
			status.Region = content
			break
		}
	}

	// Extract bracket tags
	tagMatches := bracketTagRegex.FindAllStringSubmatch(filename, -1)
	for _, match := range tagMatches {
		tag := strings.ToLower(match[1])
		switch {
		case tag == "!":
			status.IsVerified = true
		case tag == "b" || strings.HasPrefix(tag, "b") && len(tag) <= 2 && isDigit(tag[len(tag)-1]):
			status.IsBadDump = true
		case tag == "c":
			status.IsCracked = true
		case tag == "f" || strings.HasPrefix(tag, "f") && len(tag) <= 2:
			status.IsFixed = true
		case tag == "h" || strings.HasPrefix(tag, "h") && len(tag) <= 2:
			status.IsHack = true
		case tag == "o" || strings.HasPrefix(tag, "o") && len(tag) <= 2:
			status.IsOverdump = true
		case tag == "p":
			status.IsPirate = true
		case tag == "t" || strings.HasPrefix(tag, "t") && len(tag) <= 2 && tag != "t+":
			status.IsTrainer = true
		case strings.HasPrefix(tag, "t+") || strings.HasPrefix(tag, "t-"):
			status.IsTranslated = true
		case tag == "a" || strings.HasPrefix(tag, "a") && len(tag) <= 2 && isDigit(tag[len(tag)-1]):
			status.IsAlternate = true
		}
	}

	return status
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isRegionCode(s string) bool {
	// Common single-letter region codes
	regionCodes := map[string]bool{
		"U": true, "E": true, "J": true, "W": true, "F": true, "G": true,
		"I": true, "S": true, "A": true, "K": true, "C": true, "B": true,
		// Multi-letter codes
		"USA": true, "EUR": true, "JPN": true, "WORLD": true,
		"EUROPE": true, "JAPAN": true, "ASIA": true,
		// Combined codes
		"UE": true, "JU": true, "JUE": true, "UJE": true,
	}
	return regionCodes[s]
}

// MatchType represents how a file was matched.
type MatchType string

const (
	MatchTypeSHA1      MatchType = "sha1"
	MatchTypeCRC32     MatchType = "crc32"
	MatchTypeName      MatchType = "name"       // Exact name match, but hash differs
	MatchTypeFuzzyName MatchType = "name_fuzzy" // Fuzzy name match
)

// NormalizeTitleForMatching normalizes a title for fuzzy matching.
func NormalizeTitleForMatching(title string) string {
	// Remove extension
	if idx := strings.LastIndex(title, "."); idx != -1 {
		title = title[:idx]
	}

	// Remove all content in brackets and parentheses
	title = bracketTagRegex.ReplaceAllString(title, "")
	title = regionRegex.ReplaceAllString(title, "")

	// Lowercase
	title = strings.ToLower(title)

	// Remove special characters, keep only alphanumeric
	var normalized strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			normalized.WriteRune(r)
		}
	}

	return normalized.String()
}

// GetStatusFlags returns a comma-separated string of status flags.
func (s ROMStatus) GetStatusFlags() string {
	var flags []string
	if s.IsVerified {
		flags = append(flags, "verified")
	}
	if s.IsBadDump {
		flags = append(flags, "bad-dump")
	}
	if s.IsCracked {
		flags = append(flags, "cracked")
	}
	if s.IsFixed {
		flags = append(flags, "fixed")
	}
	if s.IsHack {
		flags = append(flags, "hack")
	}
	if s.IsOverdump {
		flags = append(flags, "overdump")
	}
	if s.IsPirate {
		flags = append(flags, "pirate")
	}
	if s.IsTrainer {
		flags = append(flags, "trainer")
	}
	if s.IsTranslated {
		flags = append(flags, "translated")
	}
	if s.IsAlternate {
		flags = append(flags, "alternate")
	}
	if len(flags) == 0 {
		return ""
	}
	return strings.Join(flags, ",")
}

// IsModified returns true if the ROM is known to be modified from original.
func (s ROMStatus) IsModified() bool {
	return s.IsCracked || s.IsFixed || s.IsHack || s.IsTrainer || s.IsTranslated || s.IsPirate
}

// IsProblematic returns true if the ROM might have issues.
func (s ROMStatus) IsProblematic() bool {
	return s.IsBadDump || s.IsOverdump
}
