package dat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanm101/romman-lib/db"
)

func TestImporter_Import(t *testing.T) {
	// Create a test DAT file
	datContent := `<?xml version="1.0"?>
<!DOCTYPE datafile SYSTEM "http://www.logiqx.com/Dats/datafile.dtd">
<datafile>
	<header>
		<name>Nintendo - Game Boy Advance</name>
		<description>Nintendo - Game Boy Advance (TEST)</description>
		<version>2024-01-01</version>
		<date>2024-01-01</date>
	</header>
	<game name="Test Game (USA)">
		<description>Test Game (USA)</description>
		<rom name="Test Game (USA).gba" size="4194304" crc="12345678" sha1="abcdef1234567890abcdef1234567890abcdef12"/>
	</game>
	<game name="Another Game (Europe)">
		<description>Another Game (Europe)</description>
		<rom name="Another Game (Europe).gba" size="8388608" crc="87654321" sha1="fedcba0987654321fedcba0987654321fedcba09"/>
	</game>
</datafile>`

	tmpDir := t.TempDir()
	datPath := filepath.Join(tmpDir, "gba.dat")
	err := os.WriteFile(datPath, []byte(datContent), 0644)
	require.NoError(t, err)

	// Create test database
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Import the DAT
	importer := NewImporter(database.Conn())
	result, err := importer.Import(context.Background(), datPath)
	require.NoError(t, err)

	assert.Equal(t, "gba", result.SystemName)
	assert.True(t, result.IsNewSystem)
	assert.Equal(t, 2, result.GamesImported)
	assert.Equal(t, 2, result.RomsImported)
	assert.Equal(t, 0, result.GamesSkipped)

	// Verify database contents
	var systemCount int
	err = database.Conn().QueryRow("SELECT COUNT(*) FROM systems").Scan(&systemCount)
	require.NoError(t, err)
	assert.Equal(t, 1, systemCount)

	var releaseCount int
	err = database.Conn().QueryRow("SELECT COUNT(*) FROM releases").Scan(&releaseCount)
	require.NoError(t, err)
	assert.Equal(t, 2, releaseCount)

	var romCount int
	err = database.Conn().QueryRow("SELECT COUNT(*) FROM rom_entries").Scan(&romCount)
	require.NoError(t, err)
	assert.Equal(t, 2, romCount)
}

func TestImporter_Idempotent(t *testing.T) {
	datContent := `<?xml version="1.0"?>
<datafile>
	<header>
		<name>Nintendo - NES</name>
		<version>1.0</version>
	</header>
	<game name="Test Game">
		<rom name="test.nes" size="1024" crc="AAAAAAAA"/>
	</game>
</datafile>`

	tmpDir := t.TempDir()
	datPath := filepath.Join(tmpDir, "nes.dat")
	err := os.WriteFile(datPath, []byte(datContent), 0644)
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	importer := NewImporter(database.Conn())

	// First import
	result1, err := importer.Import(context.Background(), datPath)
	require.NoError(t, err)
	assert.True(t, result1.IsNewSystem)
	assert.Equal(t, 1, result1.GamesImported)

	// Second import - should be idempotent
	result2, err := importer.Import(context.Background(), datPath)
	require.NoError(t, err)
	assert.False(t, result2.IsNewSystem)
	assert.Equal(t, 0, result2.GamesImported)
	assert.Equal(t, 1, result2.GamesSkipped)

	// Verify only one system and one release exist
	var systemCount int
	_ = database.Conn().QueryRow("SELECT COUNT(*) FROM systems").Scan(&systemCount)
	assert.Equal(t, 1, systemCount)

	var releaseCount int
	_ = database.Conn().QueryRow("SELECT COUNT(*) FROM releases").Scan(&releaseCount)
	assert.Equal(t, 1, releaseCount)
}
