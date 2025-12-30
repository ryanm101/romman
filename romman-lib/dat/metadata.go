package dat

import (
	"regexp"
	"strings"
)

// Common regions in order of appearance frequency
var regionPatterns = []string{
	"USA", "Europe", "Japan", "World", "Asia", "Australia",
	"Brazil", "Canada", "China", "France", "Germany", "Hong Kong",
	"Italy", "Korea", "Netherlands", "Russia", "Spain", "Sweden", "Taiwan", "UK",
}

// Language codes commonly found in DAT files
var languageCodes = map[string]string{
	"En": "English", "Ja": "Japanese", "Fr": "French", "De": "German",
	"Es": "Spanish", "It": "Italian", "Nl": "Dutch", "Pt": "Portuguese",
	"Sv": "Swedish", "No": "Norwegian", "Da": "Danish", "Fi": "Finnish",
	"Pl": "Polish", "Ru": "Russian", "Zh": "Chinese", "Ko": "Korean",
}

// Stability levels
const (
	StabilityStable = "stable"
	StabilityBeta   = "beta"
	StabilityProto  = "proto"
	StabilitySample = "sample"
	StabilityDemo   = "demo"
)

// Metadata contains parsed metadata from a game title.
type Metadata struct {
	BaseTitle  string
	Regions    []string
	Languages  []string
	Revision   int
	Stability  string
	IsVerified bool // [!] marker
}

var (
	// Matches content in parentheses: (USA), (En,Fr), (Rev 1)
	parenRegex = regexp.MustCompile(`\(([^)]+)\)`)
	// Matches content in brackets: [!], [b], [h1]
	bracketRegex = regexp.MustCompile(`\[([^\]]+)\]`)
	// Matches revision number: Rev 1, Rev A, v1.1
	revisionRegex = regexp.MustCompile(`(?i)(?:Rev\s*|v)(\d+(?:\.\d+)?|[A-Z])`)
)

// ParseTitle extracts metadata from a game title string.
func ParseTitle(title string) Metadata {
	meta := Metadata{
		Stability: StabilityStable,
		Revision:  0,
	}

	// Extract base title (everything before first parenthesis or bracket)
	baseEnd := len(title)
	if idx := strings.IndexAny(title, "(["); idx != -1 {
		baseEnd = idx
	}
	meta.BaseTitle = strings.TrimSpace(title[:baseEnd])

	// Parse parenthesized content
	parenMatches := parenRegex.FindAllStringSubmatch(title, -1)
	for _, match := range parenMatches {
		content := match[1]
		parseParenContent(content, &meta)
	}

	// Parse bracketed content
	bracketMatches := bracketRegex.FindAllStringSubmatch(title, -1)
	for _, match := range bracketMatches {
		content := match[1]
		parseBracketContent(content, &meta)
	}

	return meta
}

func parseParenContent(content string, meta *Metadata) {
	// Check for regions
	for _, region := range regionPatterns {
		if strings.EqualFold(content, region) {
			meta.Regions = append(meta.Regions, region)
			return
		}
	}

	// Check for multi-region (e.g., "USA, Europe")
	if strings.Contains(content, ",") {
		parts := strings.Split(content, ",")
		allRegions := true
		var foundRegions []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			isRegion := false
			for _, region := range regionPatterns {
				if strings.EqualFold(part, region) {
					foundRegions = append(foundRegions, region)
					isRegion = true
					break
				}
			}
			// Check if it's a language code
			if !isRegion {
				if _, ok := languageCodes[part]; ok {
					meta.Languages = append(meta.Languages, part)
				} else {
					allRegions = false
				}
			}
		}
		if allRegions && len(foundRegions) > 0 {
			meta.Regions = append(meta.Regions, foundRegions...)
			return
		}
	}

	// Check for language codes (e.g., "En,Fr,De")
	if isLanguageList(content) {
		parts := strings.Split(content, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if _, ok := languageCodes[part]; ok {
				meta.Languages = append(meta.Languages, part)
			}
		}
		return
	}

	// Check for revision
	if revMatch := revisionRegex.FindStringSubmatch(content); revMatch != nil {
		meta.Revision = parseRevisionNumber(revMatch[1])
		return
	}

	// Check for stability markers
	lowerContent := strings.ToLower(content)
	switch {
	case strings.Contains(lowerContent, "beta"):
		meta.Stability = StabilityBeta
	case strings.Contains(lowerContent, "proto"):
		meta.Stability = StabilityProto
	case strings.Contains(lowerContent, "sample"):
		meta.Stability = StabilitySample
	case strings.Contains(lowerContent, "demo"):
		meta.Stability = StabilityDemo
	}
}

func parseBracketContent(content string, meta *Metadata) {
	switch content {
	case "!":
		meta.IsVerified = true
	case "b", "b1", "b2":
		meta.Stability = StabilityBeta
	case "p", "p1":
		meta.Stability = StabilityProto
	}
}

func isLanguageList(content string) bool {
	parts := strings.Split(content, ",")
	if len(parts) < 1 {
		return false
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if _, ok := languageCodes[part]; !ok {
			return false
		}
	}
	return true
}

func parseRevisionNumber(rev string) int {
	// Handle letter revisions (A=1, B=2, etc.)
	if len(rev) == 1 && rev[0] >= 'A' && rev[0] <= 'Z' {
		return int(rev[0] - 'A' + 1)
	}
	// Handle numeric revisions
	var num int
	for _, c := range rev {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else if c == '.' {
			break // Stop at decimal point for v1.1 style versions
		}
	}
	return num
}
