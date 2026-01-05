package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Henry-Sarabia/igdb/v2"
)

// IGDBProvider implements the Provider interface for IGDB.
type IGDBProvider struct {
	client *igdb.Client
}

// NewIGDBProvider creates a new IGDB provider.
// It automatically fetches an access token using the provided Client ID and Secret.
func NewIGDBProvider(clientID, clientSecret string) (*IGDBProvider, error) {
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("IGDB Client ID and Secret are required")
	}

	token, err := getTwitchToken(clientID, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Twitch: %w", err)
	}

	client := igdb.NewClient(clientID, token, nil)
	return &IGDBProvider{client: client}, nil
}

func (p *IGDBProvider) Name() string {
	return "igdb"
}

func (p *IGDBProvider) Search(query string) ([]GameMetadata, error) {
	games, err := p.client.Games.Search(
		query,
		igdb.SetFields("id", "name", "summary", "first_release_date", "total_rating", "cover.url", "involved_companies.company.name", "involved_companies.publisher"),
		igdb.SetLimit(10),
	)
	if err != nil {
		return nil, err
	}

	results := make([]GameMetadata, 0, 10) // Pre-allocate for typical result count
	for _, g := range games {
		results = append(results, p.convertGame(g))
	}
	return results, nil
}

func (p *IGDBProvider) GetDetails(id string) (*GameMetadata, error) {
	// Parse ID (format: "igdb:12345")
	parts := strings.Split(id, ":")
	if len(parts) != 2 || parts[0] != "igdb" {
		return nil, fmt.Errorf("invalid IGDB ID: %s", id)
	}

	var numericID int
	if _, err := fmt.Sscanf(parts[1], "%d", &numericID); err != nil {
		return nil, fmt.Errorf("invalid numeric ID: %s", parts[1])
	}

	// Get returns a single *Game and error
	game, err := p.client.Games.Get(
		numericID,
		igdb.SetFields("id", "name", "summary", "first_release_date", "total_rating", "cover", "involved_companies"),
	)
	if err != nil {
		return nil, err
	}

	md := p.convertGame(game)
	return &md, nil
}

func (p *IGDBProvider) convertGame(g *igdb.Game) GameMetadata {
	md := GameMetadata{
		ID:          fmt.Sprintf("igdb:%d", g.ID),
		Description: g.Summary,
		Rating:      g.TotalRating,
	}

	if g.FirstReleaseDate != 0 {
		md.ReleaseDate = time.Unix(int64(g.FirstReleaseDate), 0).Format("2006-01-02")
	}

	// TODO: Fetch Cover and Companies separately since v2 struct uses IDs (int)
	// We need to call p.client.Covers.Get(g.Cover) and p.client.Companies.Get(...)
	// For now leaving these empty to ensure compilation.

	return md
}

// getTwitchToken fetches an App Access Token from Twitch.
func getTwitchToken(clientID, clientSecret string) (string, error) {
	u := "https://id.twitch.tv/oauth2/token"
	vals := url.Values{}
	vals.Set("client_id", clientID)
	vals.Set("client_secret", clientSecret)
	vals.Set("grant_type", "client_credentials")

	resp, err := http.PostForm(u, vals)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.AccessToken, nil
}
