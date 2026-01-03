package library

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanm101/romman-lib/db"
)

func TestNewOrganizer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	manager := NewManager(database.Conn())
	organizer := NewOrganizer(database.Conn(), manager)

	assert.NotNil(t, organizer)
}

func TestOrganizer_PlanEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	// Create a system and library
	_, err = database.Conn().Exec("INSERT INTO systems (name) VALUES ('nes')")
	require.NoError(t, err)

	_, err = database.Conn().Exec(`
		INSERT INTO libraries (name, root_path, system_id)
		VALUES ('test', '/tmp/roms', 1)
	`)
	require.NoError(t, err)

	manager := NewManager(database.Conn())
	organizer := NewOrganizer(database.Conn(), manager)

	opts := OrganizeOptions{
		OutputDir: "/tmp/organized",
		Structure: "system",
	}

	result, err := organizer.Plan("test", opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Actions, "should have no actions for empty library")
}

func TestOrganizer_BuildDestPath(t *testing.T) {
	organizer := &Organizer{}

	tests := []struct {
		name        string
		srcPath     string
		releaseName string
		systemName  string
		opts        OrganizeOptions
		expected    string
	}{
		{
			name:        "flat structure, no rename",
			srcPath:     "/roms/game.nes",
			releaseName: "Super Mario Bros (USA)",
			systemName:  "nes",
			opts: OrganizeOptions{
				OutputDir:   "/output",
				Structure:   "flat",
				RenameToDAT: false,
			},
			expected: "/output/game.nes",
		},
		{
			name:        "system structure, no rename",
			srcPath:     "/roms/game.nes",
			releaseName: "Super Mario Bros (USA)",
			systemName:  "nes",
			opts: OrganizeOptions{
				OutputDir:   "/output",
				Structure:   "system",
				RenameToDAT: false,
			},
			expected: "/output/nes/game.nes",
		},
		{
			name:        "flat structure, with rename",
			srcPath:     "/roms/game.nes",
			releaseName: "Super Mario Bros (USA)",
			systemName:  "nes",
			opts: OrganizeOptions{
				OutputDir:   "/output",
				Structure:   "flat",
				RenameToDAT: true,
			},
			expected: "/output/Super Mario Bros (USA).nes",
		},
		{
			name:        "system structure, with rename",
			srcPath:     "/roms/game.nes",
			releaseName: "Super Mario Bros (USA)",
			systemName:  "nes",
			opts: OrganizeOptions{
				OutputDir:   "/output",
				Structure:   "system",
				RenameToDAT: true,
			},
			expected: "/output/nes/Super Mario Bros (USA).nes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := organizer.buildDestPath(tt.srcPath, tt.releaseName, tt.systemName, tt.opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrganizeOptions_Defaults(t *testing.T) {
	opts := OrganizeOptions{
		OutputDir: "/output",
	}

	// Store defaults
	assert.Equal(t, "", opts.Structure, "default structure should be empty (caller sets)")
	assert.False(t, opts.RenameToDAT)
	assert.False(t, opts.DryRun)
	assert.False(t, opts.MatchedOnly)
	assert.False(t, opts.PreferredOnly)
}

func TestOrganizeResult(t *testing.T) {
	result := &OrganizeResult{}

	result.Actions = append(result.Actions, OrganizeAction{
		SourcePath:  "/src/game.nes",
		DestPath:    "/dst/game.nes",
		Action:      "move",
		ReleaseName: "Test Game",
	})
	result.Moved = 1

	assert.Len(t, result.Actions, 1)
	assert.Equal(t, 1, result.Moved)
	assert.Equal(t, 0, result.Errors)
}
