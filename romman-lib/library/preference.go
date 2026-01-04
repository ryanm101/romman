package library

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ryanm101/romman-lib/tracing"
	"go.opentelemetry.io/otel/attribute"
)

// PreferenceConfig holds user preferences for release selection.
type PreferenceConfig struct {
	RegionOrder []string // Region priority, e.g. ["Europe", "World", "USA"]
}

// DefaultPreferenceConfig returns the default preference configuration.
func DefaultPreferenceConfig() PreferenceConfig {
	return PreferenceConfig{
		RegionOrder: []string{"Europe", "World", "USA", "Japan"},
	}
}

// ReleaseCandidate represents a release being considered for selection.
type ReleaseCandidate struct {
	ReleaseID    int64
	Name         string
	BaseTitle    string
	Regions      []string
	Languages    []string
	Revision     int
	Stability    Stability
	Score        int
	IsPreferred  bool
	IgnoreReason string
}

// Stability represents the stability level of a release.
type Stability int

const (
	StabilityStable Stability = iota
	StabilityBeta
	StabilityProto
	StabilitySample
	StabilityDemo
)

var (
	// Regex patterns for parsing release names
	revisionPattern = regexp.MustCompile(`\(Rev\s*([A-Z0-9]+)\)|\(v([0-9.]+)\)`)
	betaPattern     = regexp.MustCompile(`\(Beta[^)]*\)`)
	protoPattern    = regexp.MustCompile(`\(Proto[^)]*\)`)
	samplePattern   = regexp.MustCompile(`\(Sample[^)]*\)`)
	demoPattern     = regexp.MustCompile(`\(Demo[^)]*\)`)
	regionPattern   = regexp.MustCompile(`\(((?:Europe|USA|Japan|World|Korea|France|Germany|Spain|Italy|Brazil|Australia|Asia|China|Taiwan)[^)]*)\)`)
	languagePattern = regexp.MustCompile(`\(([A-Z][a-z](?:,[A-Z][a-z])*)\)`)
)

// PreferenceSelector selects preferred releases using deterministic rules.
type PreferenceSelector struct {
	db     *sql.DB
	config PreferenceConfig
}

// NewPreferenceSelector creates a new preference selector.
func NewPreferenceSelector(db *sql.DB, config PreferenceConfig) *PreferenceSelector {
	return &PreferenceSelector{
		db:     db,
		config: config,
	}
}

// SelectPreferred runs the selection algorithm for a system.
func (p *PreferenceSelector) SelectPreferred(ctx context.Context, systemID int64) error {
	ctx, span := tracing.StartSpan(ctx, "library.SelectPreferred",
		tracing.WithAttributes(attribute.Int64("system.id", systemID)),
	)
	defer span.End()

	// Get all releases for the system
	releases, err := p.getReleases(ctx, systemID)
	if err != nil {
		tracing.RecordError(span, err)
		return fmt.Errorf("failed to get releases: %w", err)
	}

	// Group by base title
	groups := make(map[string][]*ReleaseCandidate)
	for i := range releases {
		groups[releases[i].BaseTitle] = append(groups[releases[i].BaseTitle], &releases[i])
	}

	// Select preferred for each group
	var preferredCount int
	for _, candidates := range groups {
		p.selectFromGroup(candidates)
		for _, c := range candidates {
			if c.IsPreferred {
				preferredCount++
			}
		}
	}

	// Update database
	for _, r := range releases {
		if err := p.updateRelease(r); err != nil {
			tracing.RecordError(span, err)
			return fmt.Errorf("failed to update release %d: %w", r.ReleaseID, err)
		}
	}

	tracing.AddSpanAttributes(span,
		attribute.Int("result.releases", len(releases)),
		attribute.Int("result.groups", len(groups)),
		attribute.Int("result.preferred", preferredCount),
	)

	return nil
}

func (p *PreferenceSelector) getReleases(ctx context.Context, systemID int64) ([]ReleaseCandidate, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, name FROM releases WHERE system_id = ?
	`, systemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var releases []ReleaseCandidate
	for rows.Next() {
		var r ReleaseCandidate
		if err := rows.Scan(&r.ReleaseID, &r.Name); err != nil {
			return nil, err
		}
		p.parseReleaseName(&r)
		releases = append(releases, r)
	}

	return releases, nil
}

func (p *PreferenceSelector) parseReleaseName(r *ReleaseCandidate) {
	name := r.Name

	// Extract base title (before first parenthesis)
	if idx := strings.Index(name, "("); idx > 0 {
		r.BaseTitle = strings.TrimSpace(name[:idx])
	} else {
		r.BaseTitle = name
	}

	// Parse stability
	r.Stability = StabilityStable
	if betaPattern.MatchString(name) {
		r.Stability = StabilityBeta
	} else if protoPattern.MatchString(name) {
		r.Stability = StabilityProto
	} else if samplePattern.MatchString(name) {
		r.Stability = StabilitySample
	} else if demoPattern.MatchString(name) {
		r.Stability = StabilityDemo
	}

	// Parse revision
	r.Revision = 0
	if matches := revisionPattern.FindStringSubmatch(name); matches != nil {
		if matches[1] != "" {
			// Letter revision (A=1, B=2, etc)
			r.Revision = int(matches[1][0] - 'A' + 1)
		} else if matches[2] != "" {
			// Numeric version
			r.Revision = parseVersion(matches[2])
		}
	}

	// Parse regions
	if matches := regionPattern.FindStringSubmatch(name); matches != nil {
		regions := strings.Split(matches[1], ", ")
		for _, region := range regions {
			r.Regions = append(r.Regions, strings.TrimSpace(region))
		}
	}

	// Parse languages
	if matches := languagePattern.FindStringSubmatch(name); matches != nil {
		langs := strings.Split(matches[1], ",")
		for _, lang := range langs {
			r.Languages = append(r.Languages, strings.TrimSpace(lang))
		}
	}

	// Default English for English-region releases
	if len(r.Languages) == 0 {
		for _, region := range r.Regions {
			switch region {
			case "USA", "Europe", "World", "Australia":
				r.Languages = []string{"En"}
			}
		}
	}
}

func parseVersion(v string) int {
	// Convert "1.2" to 120 for comparison
	parts := strings.Split(v, ".")
	result := 0
	multiplier := 100
	for _, part := range parts {
		var num int
		_, _ = fmt.Sscanf(part, "%d", &num)
		result += num * multiplier
		multiplier /= 10
	}
	return result
}

func (p *PreferenceSelector) selectFromGroup(candidates []*ReleaseCandidate) {
	if len(candidates) == 0 {
		return
	}

	// Score each candidate
	for _, c := range candidates {
		c.Score = p.scoreCandidate(c)
	}

	// Sort by score (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Mark preferred (first one)
	candidates[0].IsPreferred = true

	// Mark others as ignored
	for i := 1; i < len(candidates); i++ {
		candidates[i].IsPreferred = false
		candidates[i].IgnoreReason = determineIgnoreReason(candidates[0], candidates[i])
	}
}

func (p *PreferenceSelector) scoreCandidate(c *ReleaseCandidate) int {
	score := 0

	// Language: must include English (+1000)
	hasEnglish := false
	for _, lang := range c.Languages {
		if lang == "En" || lang == "English" {
			hasEnglish = true
			break
		}
	}
	if hasEnglish {
		score += 1000
	}

	// Stability: stable > beta > proto > sample > demo
	switch c.Stability {
	case StabilityStable:
		score += 500
	case StabilityBeta:
		score += 100
	case StabilityProto:
		score += 50
	case StabilitySample:
		score += 25
	case StabilityDemo:
		score += 10
	}

	// Revision: higher is better
	score += c.Revision * 10

	// Region: use config order
	for i, preferredRegion := range p.config.RegionOrder {
		for _, region := range c.Regions {
			if strings.Contains(region, preferredRegion) {
				score += (len(p.config.RegionOrder) - i) * 50
				goto regionDone
			}
		}
	}
regionDone:

	return score
}

func determineIgnoreReason(preferred, other *ReleaseCandidate) string {
	reasons := []string{}

	// Check language
	preferredHasEn := containsEnglish(preferred.Languages)
	otherHasEn := containsEnglish(other.Languages)
	if preferredHasEn && !otherHasEn {
		reasons = append(reasons, "no-english")
	}

	// Check stability
	if preferred.Stability < other.Stability {
		reasons = append(reasons, "less-stable")
	}

	// Check revision
	if preferred.Revision > other.Revision {
		reasons = append(reasons, "older-revision")
	}

	// Check region
	if len(reasons) == 0 && len(preferred.Regions) > 0 && len(other.Regions) > 0 {
		reasons = append(reasons, "lower-region-priority")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "duplicate")
	}

	return strings.Join(reasons, ",")
}

func containsEnglish(languages []string) bool {
	for _, lang := range languages {
		if lang == "En" || lang == "English" {
			return true
		}
	}
	return false
}

func (p *PreferenceSelector) updateRelease(r ReleaseCandidate) error {
	_, err := p.db.Exec(`
		UPDATE releases 
		SET is_preferred = ?, ignore_reason = ?
		WHERE id = ?
	`, r.IsPreferred, nullableString(r.IgnoreReason), r.ReleaseID)
	return err
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// GetPreferredReleases returns the preferred releases for a system.
func (p *PreferenceSelector) GetPreferredReleases(systemID int64) ([]ReleaseCandidate, error) {
	rows, err := p.db.Query(`
		SELECT id, name, COALESCE(is_preferred, 0)
		FROM releases 
		WHERE system_id = ? AND is_preferred = 1
		ORDER BY name
	`, systemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var releases []ReleaseCandidate
	for rows.Next() {
		var r ReleaseCandidate
		if err := rows.Scan(&r.ReleaseID, &r.Name, &r.IsPreferred); err != nil {
			return nil, err
		}
		p.parseReleaseName(&r)
		releases = append(releases, r)
	}

	return releases, nil
}
