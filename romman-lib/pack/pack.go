// Package pack provides game pack generation for exporting ROMs to various target formats.
package pack

import (
	"archive/zip"
	"io"
)

// Format represents the target export format for a game pack.
type Format string

const (
	// FormatRetroArch creates playlists (.lpl) compatible with RetroArch.
	FormatRetroArch Format = "retroarch"
	// FormatEmulationStation creates gamelist.xml files for EmulationStation.
	FormatEmulationStation Format = "emulationstation"
	// FormatSimple just copies ROMs with folder structure.
	// FormatSimple just copies ROMs with folder structure.
	FormatSimple Format = "simple"
	// FormatArkOS creates file structure compatible with ArkOS (R36S etc) - alias for EmulationStation.
	FormatArkOS Format = "arkos"
)

// Game represents a game to include in a pack.
type Game struct {
	ID          int64  // Database ID
	Name        string // Game name (e.g., "Super Mario Bros")
	System      string // System ID (e.g., "nes")
	SystemName  string // Display name (e.g., "Nintendo Entertainment System")
	FilePath    string // Absolute path to the ROM file
	FileName    string // Just the filename (e.g., "Super Mario Bros.nes")
	Size        int64  // File size in bytes
	ArchivePath string // Path within archive, if ROM is in a zip
}

// Request defines what games to pack and in what format.
type Request struct {
	Games  []Game // Games to include
	Format Format // Target format
	Name   string // Optional pack name (used for zip filename)
}

// Result contains metadata about a generated pack.
type Result struct {
	Name      string // Pack name
	FileCount int    // Number of files in the pack
	TotalSize int64  // Total size in bytes
	Format    Format // Format used
	ZipPath   string // Path to generated zip (if written to disk)
}

// Exporter generates game packs in a specific format.
type Exporter interface {
	// Export writes a game pack to the provided zip writer.
	Export(games []Game, zw *zip.Writer) error
	// Format returns the format this exporter produces.
	Format() Format
}

// Generator orchestrates game pack creation.
type Generator struct {
	exporters map[Format]Exporter
}

// NewGenerator creates a new pack generator with registered exporters.
func NewGenerator() *Generator {
	g := &Generator{
		exporters: make(map[Format]Exporter),
	}
	// Register default exporters
	g.RegisterExporter(&RetroArchExporter{})
	g.RegisterExporter(&EmulationStationExporter{})
	g.RegisterExporter(&SimpleExporter{})

	// Register ArkOS as an alias for EmulationStation
	esExporter := &EmulationStationExporter{}
	g.exporters[FormatArkOS] = esExporter

	return g
}

// RegisterExporter adds an exporter for a specific format.
func (g *Generator) RegisterExporter(e Exporter) {
	g.exporters[e.Format()] = e
}

// Generate creates a game pack and writes it to the provided writer.
func (g *Generator) Generate(req Request, w io.Writer) (*Result, error) {
	exporter, ok := g.exporters[req.Format]
	if !ok {
		return nil, ErrUnsupportedFormat
	}

	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()

	if err := exporter.Export(req.Games, zw); err != nil {
		return nil, err
	}

	// Calculate totals
	var totalSize int64
	for _, game := range req.Games {
		totalSize += game.Size
	}

	return &Result{
		Name:      req.Name,
		FileCount: len(req.Games),
		TotalSize: totalSize,
		Format:    req.Format,
	}, nil
}

// EstimateSize calculates the approximate size of a pack without generating it.
func (g *Generator) EstimateSize(games []Game) int64 {
	var total int64
	for _, game := range games {
		total += game.Size
	}
	// Add ~10% overhead for zip metadata and manifests
	return total + (total / 10)
}
