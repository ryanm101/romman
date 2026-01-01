# romman-lib

The core logic engine for the ROM Manager project. This library handles all database operations, file matching, and preference selection.

## Features

- **Database Management**: Schema and migrations for the SQLite-backed storage.
- **DAT Parsing**: Support for standard Logiqx XML DAT files.
- **Library Scanner**: Multi-threaded directory walker with zip archive support and hash caching.
- **Matching Engine**: Hash-first matching (SHA1/CRC32) with fallback to name-based heuristic matching.
- **Preference Rules**: Logic for selecting a single "preferred" release per game based on region, language, and stability.
- **Cleanup Planning**: Logic for detecting duplicates and generating move/delete plans.
- **System Detection**: Auto-detect systems from directory names with configurable YAML mappings.

## Key Packages

- `db`: SQLite connection management and query helpers.
- `dat`: DAT file parsing, system detection, and YAML-based mappings.
- `library`: Scanner, matching, and ROM status tracking.
- `config`: User configuration and region preferences.
- `logging`: Structured logging helpers using `slog`.
- `tracing`: OpenTelemetry instrumentation using OTLP.
- `metrics`: Prometheus metrics for monitoring.

## System Mappings

The `dat` package includes a flexible system mapping feature that:

1. **Embeds defaults**: 180+ built-in mappings for common directory names and DAT patterns
2. **Supports user overrides**: Custom mappings via `systems.yaml` take precedence
3. **Auto-detection**: `DetectSystemFromDirName()` identifies systems from folder names

### Custom Mappings

Create a `systems.yaml` file to add custom mappings:

```yaml
directory_mappings:
  mynes: nes
  my-custom-roms: snes

dat_mappings:
  "my custom dat name": nes

display_names:
  nes: "Nintendo Entertainment System (NES)"
```

See `examples/systems.yaml` for a complete template.

## Usage

This package is intended to be used by the various frontend interfaces (`romman-cli`, `romman-tui`, etc.).

```go
import "github.com/ryanm101/romman-lib/library"
import "github.com/ryanm101/romman-lib/dat"

// Detect system from directory name
system, found := dat.DetectSystemFromDirName("megadrive")
// system = "md", found = true

// Get display name
name := dat.GetSystemDisplayName("md")
// name = "Sega Genesis / Mega Drive"
```

## Testing

Run tests to ensure matching and logic correctness:

```bash
go test ./...
```
