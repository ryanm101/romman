# romman-cli

The authoritative command-line interface for ROM Manager. It provides granular control over the catalogue and libraries.

## Commands

- `dat import <file>`: Import a system DAT file into the catalogue.
- `systems`: List all imported systems and their game counts.
- `library add <name> <path> <system>`: Register a new ROM library.
- `library scan <name>`: Scan a library, compute hashes, and match games.
- `library status <name>`: Show completeness statistics and missing games.
- `prefer rebuild <system>`: Recompute preferred releases based on current rules.
- `cleanup generate <library>`: Create a sidecar JSON plan to remove/quarantine duplicates.
- `cleanup execute <plan.json>`: Apply a generated cleanup plan.
- `library scan-all`: Scan all registered libraries.

## Environment Variables

- `ROMMAN_DB`: Path to the SQLite database file.
- `ROMMAN_CONFIG`: Path to the configuration file (default: `.romman.yaml`).
- `OTEL_EXPORTER_OTLP_ENDPOINT`: If set, enables OpenTelemetry tracing and points to the OTLP collector (e.g., `localhost:4317`).

## Examples

Importing a DAT and scanning a library:
```bash
romman dat import nes.dat
romman library add "NES Library" /roms/nes nes
romman library scan "NES Library"
```

## Build

```bash
make build
# or
go build -o romman .
```
