package dat

import (
	"context"
	"database/sql"
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
	err := os.WriteFile(datPath, []byte(datContent), 0644) // #nosec G306
	require.NoError(t, err)

	// Create test database
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(context.Background(), dbPath)
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
	err := os.WriteFile(datPath, []byte(datContent), 0644) // #nosec G306
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	importer := NewImporter(database.Conn())

	// First import
	result1, err := importer.Import(context.Background(), datPath)
	require.NoError(t, err)
	assert.True(t, result1.IsNewSystem)
	assert.True(t, result1.IsNewSource)
	assert.Equal(t, 1, result1.GamesImported)
	assert.False(t, result1.Skipped)

	// Second import - DAT unchanged, should be skipped
	result2, err := importer.Import(context.Background(), datPath)
	require.NoError(t, err)
	assert.True(t, result2.Skipped, "second import should skip (DAT unchanged)")

	// Verify only one system and one release exist
	var systemCount int
	_ = database.Conn().QueryRow("SELECT COUNT(*) FROM systems").Scan(&systemCount)
	assert.Equal(t, 1, systemCount)

	var releaseCount int
	_ = database.Conn().QueryRow("SELECT COUNT(*) FROM releases").Scan(&releaseCount)
	assert.Equal(t, 1, releaseCount)
}

func TestImporter_Clones(t *testing.T) {
	datContent := `<?xml version="1.0"?>
<datafile>
	<header><name>Clones</name></header>
	<game name="Parent Game">
		<description>Parent Game</description>
	</game>
	<game name="Clone Game" cloneof="Parent Game" romof="Parent Game">
		<description>Clone Game</description>
	</game>
</datafile>`

	tmpDir := t.TempDir()
	datPath := filepath.Join(tmpDir, "clones.dat")
	err := os.WriteFile(datPath, []byte(datContent), 0644) // #nosec G306
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	importer := NewImporter(database.Conn())
	_, err = importer.Import(context.Background(), datPath)
	require.NoError(t, err)

	// Verify clone_of column
	var parentCloneOf, cloneCloneOf sql.NullString
	err = database.Conn().QueryRow("SELECT clone_of FROM releases WHERE name = 'Parent Game'").Scan(&parentCloneOf)
	require.NoError(t, err)
	assert.True(t, !parentCloneOf.Valid || parentCloneOf.String == "", "Parent should have empty/null clone_of")

	err = database.Conn().QueryRow("SELECT clone_of FROM releases WHERE name = 'Clone Game'").Scan(&cloneCloneOf)
	require.NoError(t, err)
	assert.Equal(t, "Parent Game", cloneCloneOf.String)

	// Verify parent_id is resolved correctly
	var parentParentID, cloneParentID sql.NullInt64
	err = database.Conn().QueryRow("SELECT parent_id FROM releases WHERE name = 'Parent Game'").Scan(&parentParentID)
	require.NoError(t, err)
	assert.False(t, parentParentID.Valid, "Parent should have NULL parent_id")

	err = database.Conn().QueryRow("SELECT parent_id FROM releases WHERE name = 'Clone Game'").Scan(&cloneParentID)
	require.NoError(t, err)
	assert.True(t, cloneParentID.Valid, "Clone should have parent_id set")

	// Verify parent_id points to the correct parent
	var parentName string
	err = database.Conn().QueryRow("SELECT name FROM releases WHERE id = ?", cloneParentID.Int64).Scan(&parentName)
	require.NoError(t, err)
	assert.Equal(t, "Parent Game", parentName, "Clone's parent_id should point to Parent Game")
}
