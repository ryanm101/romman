# romman-cli

The authoritative command-line interface for ROM Manager. It provides granular control over the catalogue and libraries.

## Commands

### DAT Management
- `dat import <file>`: Import a system DAT file into the catalogue.
- `dat scan <directory>`: Scan a directory for DAT files and import them.

### System Management
- `systems`: List all imported systems and their game counts.
- `systems info <system>`: Show detailed information about a system.
- `systems status`: Show completeness status across all systems.

### Library Management
- `library add <name> <path> <system>`: Register a new ROM library.
- `library list`: List all registered libraries.
- `library scan <name>`: Scan a library, compute hashes, and match games.
- `library scan-all`: Scan all registered libraries.
- `library status <name>`: Show completeness statistics and missing games.
- `library unmatched <name>`: List files that couldn't be matched.
- `library discover <parent-dir> [--add] [--force]`: Auto-discover libraries from directory structure.
- `library rename <name> [--dry-run]`: Rename files to match DAT names.
- `library verify <name>`: Verify file integrity against stored hashes.

### Preference & Cleanup
- `prefer rebuild <system>`: Recompute preferred releases based on current rules.
- `prefer list <system>`: List all preferred releases for a system.
- `duplicates <library>`: Show duplicate files in a library.
- `cleanup generate <library>`: Create a sidecar JSON plan to remove/quarantine duplicates.
- `cleanup execute <plan.json>`: Apply a generated cleanup plan.

### Utilities
- `doctor`: Run database health checks and integrity verification.
- `backup <destination>`: Create a timestamped backup of the database.
- `config show`: Display current configuration.
- `config init`: Generate an example configuration file.
- `export retroarch <library> <output-dir>`: Export playlists for RetroArch.

## Global Options

- `--json`: Output in JSON format (for scripting).
- `--quiet`: Suppress non-essential output.

## Environment Variables

- `ROMMAN_DB`: Path to the SQLite database file.
- `ROMMAN_CONFIG`: Path to the configuration file (default: `.romman.yaml`).
- `ROMMAN_SYSTEMS_FILE`: Path to custom system mappings YAML file.
- `OTEL_EXPORTER_OTLP_ENDPOINT`: If set, enables OpenTelemetry tracing.

## Configuration

### System Mappings

ROM Manager can auto-detect systems from directory names. Custom mappings can be added via `systems.yaml`:

```yaml
# ~/.config/romman/systems.yaml
directory_mappings:
  mynes: nes
  my-custom-roms: snes
  
display_names:
  nes: "Nintendo Entertainment System (NES)"
```

Search locations (in order):
1. `ROMMAN_SYSTEMS_FILE` environment variable
2. `./systems.yaml` (current directory)
3. `~/.config/romman/systems.yaml`
4. `/etc/romman/systems.yaml`

## Examples

### Basic Workflow
```bash
romman dat import nes.dat
romman library add "NES Library" /roms/nes nes
romman library scan "NES Library"
```

### Auto-Discover Libraries
```bash
# Preview what would be discovered
romman library discover ~/roms

# Add discovered libraries (requires matching DAT files)
romman library discover ~/roms --add

# Force add libraries even without DAT files (creates stub systems)
romman library discover ~/roms --add --force
```

## Build

```bash
make build
# or
go build -o romman .
```
