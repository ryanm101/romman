package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanm/romman/internal/db"
)

func TestLibraryManager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Create a system first
	_, err = database.Conn().Exec(`
		INSERT INTO systems (name, dat_name) VALUES ('nes', 'Nintendo - NES')
	`)
	require.NoError(t, err)

	// Create a library directory
	libPath := filepath.Join(tmpDir, "roms")
	require.NoError(t, os.MkdirAll(libPath, 0755))

	manager := NewManager(database.Conn())

	// Test Add
	lib, err := manager.Add("my-nes", libPath, "nes")
	require.NoError(t, err)
	assert.Equal(t, "my-nes", lib.Name)
	assert.Equal(t, libPath, lib.RootPath)
	assert.Equal(t, "nes", lib.SystemName)

	// Test Get
	lib2, err := manager.Get("my-nes")
	require.NoError(t, err)
	assert.Equal(t, lib.ID, lib2.ID)
	assert.Equal(t, lib.Name, lib2.Name)

	// Test List
	libs, err := manager.List()
	require.NoError(t, err)
	assert.Len(t, libs, 1)

	// Test Get non-existent
	_, err = manager.Get("nonexistent")
	assert.Error(t, err)

	// Test Add with non-existent system
	_, err = manager.Add("bad", libPath, "nonexistent")
	assert.Error(t, err)

	// Test Delete
	err = manager.Delete("my-nes")
	require.NoError(t, err)

	libs, err = manager.List()
	require.NoError(t, err)
	assert.Len(t, libs, 0)
}
