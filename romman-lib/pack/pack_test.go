package pack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator()
	assert.NotNil(t, g)
	assert.NotNil(t, g.exporters)
	// Should have default exporters registered
	assert.Contains(t, g.exporters, FormatRetroArch)
	assert.Contains(t, g.exporters, FormatSimple)
}

func TestGenerator_EstimateSize(t *testing.T) {
	g := NewGenerator()
	games := []Game{
		{Size: 1000},
		{Size: 2000},
		{Size: 500},
	}
	estimate := g.EstimateSize(games)
	// Should be total + 10% overhead
	assert.Equal(t, int64(3850), estimate) // 3500 + 350
}

func TestSimpleExporter_Export(t *testing.T) {
	// Create temp ROM file
	tmpDir := t.TempDir()
	romPath := filepath.Join(tmpDir, "test.nes")
	romContent := []byte("fake rom content")
	// #nosec G306
	require.NoError(t, os.WriteFile(romPath, romContent, 0644))

	games := []Game{
		{
			Name:     "Test Game",
			System:   "nes",
			FilePath: romPath,
			FileName: "test.nes",
			Size:     int64(len(romContent)),
		},
	}

	// Generate zip
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	exporter := &SimpleExporter{}
	err := exporter.Export(games, zw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	// Verify zip contents
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
	assert.Equal(t, "nes/test.nes", zr.File[0].Name)

	// Verify content
	rc, err := zr.File[0].Open()
	require.NoError(t, err)
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, romContent, content)
}

func TestSimpleExporter_NoGames(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	exporter := &SimpleExporter{}
	err := exporter.Export([]Game{}, zw)
	assert.ErrorIs(t, err, ErrNoGames)
}

func TestRetroArchExporter_Export(t *testing.T) {
	// Create temp ROM file
	tmpDir := t.TempDir()
	romPath := filepath.Join(tmpDir, "mario.nes")
	romContent := []byte("fake nes rom")
	// #nosec G306
	require.NoError(t, os.WriteFile(romPath, romContent, 0644))

	games := []Game{
		{
			Name:       "Super Mario Bros",
			System:     "nes",
			SystemName: "Nintendo Entertainment System",
			FilePath:   romPath,
			FileName:   "mario.nes",
			Size:       int64(len(romContent)),
		},
	}

	// Generate zip
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	exporter := &RetroArchExporter{}
	err := exporter.Export(games, zw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	// Verify zip contents
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	// Should have ROM + playlist
	require.Len(t, zr.File, 2)

	// Find files by name
	var romFile, playlistFile *zip.File
	for _, f := range zr.File {
		switch f.Name {
		case "roms/nes/mario.nes":
			romFile = f
		case "playlists/Nintendo Entertainment System.lpl":
			playlistFile = f
		}
	}

	assert.NotNil(t, romFile, "ROM file not found in zip")
	assert.NotNil(t, playlistFile, "Playlist file not found in zip")

	// Verify playlist content
	if playlistFile != nil {
		rc, err := playlistFile.Open()
		require.NoError(t, err)
		defer func() { _ = rc.Close() }()

		var playlist RetroArchPlaylist
		require.NoError(t, json.NewDecoder(rc).Decode(&playlist))

		assert.Equal(t, "1.5", playlist.Version)
		require.Len(t, playlist.Items, 1)
		assert.Equal(t, "Super Mario Bros", playlist.Items[0].Label)
		assert.Contains(t, playlist.Items[0].Path, "mario.nes")
	}
}

func TestGenerator_Generate(t *testing.T) {
	// Create temp ROM file
	tmpDir := t.TempDir()
	romPath := filepath.Join(tmpDir, "game.sfc")
	romContent := []byte("snes rom data")
	// #nosec G306
	require.NoError(t, os.WriteFile(romPath, romContent, 0644))

	g := NewGenerator()
	req := Request{
		Name:   "My Pack",
		Format: FormatSimple,
		Games: []Game{
			{
				Name:     "Cool Game",
				System:   "snes",
				FilePath: romPath,
				FileName: "game.sfc",
				Size:     int64(len(romContent)),
			},
		},
	}

	var buf bytes.Buffer
	result, err := g.Generate(req, &buf)
	require.NoError(t, err)

	assert.Equal(t, "My Pack", result.Name)
	assert.Equal(t, 1, result.FileCount)
	assert.Equal(t, FormatSimple, result.Format)
}

func TestGenerator_UnsupportedFormat(t *testing.T) {
	g := NewGenerator()
	req := Request{
		Format: "unknown",
		Games:  []Game{},
	}

	var buf bytes.Buffer
	_, err := g.Generate(req, &buf)
	assert.ErrorIs(t, err, ErrUnsupportedFormat)
}
