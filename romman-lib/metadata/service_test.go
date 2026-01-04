package metadata

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ryanm101/romman-lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Search(name string) ([]GameMetadata, error) {
	args := m.Called(name)
	return args.Get(0).([]GameMetadata), args.Error(1)
}

func (m *MockProvider) GetDetails(id string) (*GameMetadata, error) {
	args := m.Called(id)
	return args.Get(0).(*GameMetadata), args.Error(1)
}

func TestScrapeGame(t *testing.T) {
	// Setup DB
	tmpDB := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(context.Background(), tmpDB)
	assert.NoError(t, err)
	defer database.Close()

	// Setup release
	ctx := context.Background()
	_, err = database.Conn().Exec("INSERT INTO systems (name) VALUES ('nes')")
	assert.NoError(t, err)
	_, err = database.Conn().Exec("INSERT INTO releases (id, system_id, name) VALUES (100, 1, 'Super Mario Bros')")
	assert.NoError(t, err)

	// Setup Mock Provider
	mockProvider := new(MockProvider)
	mockProvider.On("Search", "Super Mario Bros").Return([]GameMetadata{
		{ID: "igdb:123", Description: "A plumber's journey"},
	}, nil)
	mockProvider.On("GetDetails", "igdb:123").Return(&GameMetadata{
		ID:          "igdb:123",
		Description: "Detailed description",
		ReleaseDate: "1985-09-13",
		Developer:   "Nintendo",
		Publisher:   "Nintendo",
		Rating:      95.5,
		BoxartURL:   "", // Skip download logic for now to avoid net access
	}, nil)

	// Setup Service
	mediaRoot := t.TempDir()
	service := NewService(database, mockProvider, mediaRoot)

	// Execute
	err = service.ScrapeGame(ctx, 100, "Super Mario Bros")
	assert.NoError(t, err)

	// Verify Metadata in DB
	md, err := database.GetGameMetadata(ctx, 100)
	assert.NoError(t, err)
	assert.NotNil(t, md)
	assert.Equal(t, "igdb:123", md.ProviderID)
	assert.Equal(t, "Detailed description", md.Description)
	assert.Equal(t, "Nintendo", md.Developer)
	assert.Equal(t, 95.5, md.Rating)
}
