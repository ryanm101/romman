package metadata

// GameMetadata represents enriched information about a game.
type GameMetadata struct {
	ID          string  // Provider specific ID (e.g. "igdb:12345")
	Description string  // Game summary/description
	ReleaseDate string  // ISO 8601 date string (approximate)
	Developer   string  // Main developer
	Publisher   string  // Main publisher
	Rating      float64 // Rating out of 100
	BoxartURL   string  // URL to boxart image
}

// Provider defines the interface for fetching game metadata.
type Provider interface {
	// Name returns the provider name (e.g., "igdb").
	Name() string
	// Search finds games matching the query.
	Search(query string) ([]GameMetadata, error)
	// GetDetails fetches detailed metadata for a specific ID.
	GetDetails(id string) (*GameMetadata, error)
}
