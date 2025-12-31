# romman-lib

The core logic engine for the ROM Manager project. This library handles all database operations, file matching, and preference selection.

## Features

- **Database Management**: Schema and migrations for the SQLite-backed storage.
- **DAT Parsing**: Support for standard Logiqx XML DAT files.
- **Library Scanner**: Multi-threaded directory walker with zip archive support and hash caching.
- **Matching Engine**: Hash-first matching (SHA1/CRC32) with fallback to name-based heuristic matching.
- **Preference Rules**: Logic for selecting a single "preferred" release per game based on region, language, and stability.
- **Cleanup Planning**: Logic for detecting duplicates and generating move/delete plans.

## Key Packages

- `db`: SQLite connection management and query helpers.
- `dat`: DAT file loading and release/rom mapping.
- `library`: Scanner, matching, and rom status tracking.
- `config`: User configuration and region preferences.
- `logging`: Structured logging helpers using `slog`.
- `tracing`: OpenTelemetry instrumentation using OTLP.

## Usage

This package is intended to be used by the various frontend interfaces (`romman-cli`, `romman-tui`, etc.).

```go
import "github.com/ryanm/romman-lib/library"
// ... logic here
```

## Testing

Run tests to ensure matching and logic correctness:

```bash
go test ./...
```
