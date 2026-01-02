# ROM Manager (romman)

A powerful, cross-platform ROM management tool designed for retro gaming enthusiasts who value clean, canonical libraries.

## Overview

`romman` helps you manage your retro game collection by comparing your local files against official catalogues (DAT files). It prioritises correctness and user control, allowing you to identify missing games, eliminate duplicates, and keep only the "best" version of every title.

## Key Features

- **Hash-First Matching**: Uses SHA1 (preferred) and CRC32 (fallback) to identify ROMs regardless of filename.
- **Catalogue Focused**: Uses standard DAT files as the source of truth for your library.
- **Preferred Selection**: Deterministic rules select the best release (e.g., Europe > World > USA) and highest stable revision.
- **Audit & Cleanup**: Generates explicit cleanup plans for duplicates; no files are moved or deleted without your approval.
- **Multiple Interfaces**:
  - **CLI**: Feature-complete command-line interface for batch operations.
  - **TUI**: Interactive terminal UI for browsing your collection and library status.
  - **Web**: (In Development) Modern web interface for management.

## Project Structure

- `romman-lib/`: Core logic, database schema, and scanner.
- `romman-cli/`: The authoritative command-line tool.
- `romman-tui/`: Interactive Terminal UI built with Bubble Tea.
- `romman-web/`: Frontend management interface.

## Getting Started

### Prerequisites

- Go 1.21+
- SQLite3
- Make (optional, for convenience)

### Build

Build all interfaces using the top-level Makefile:

```bash
make build-all
```

Binaries will be available in the `bin/` directory.

### Quick Workflow

1.  **Import a System Catalogue**:
    ```bash
    ./bin/romman dat import path/to/megadrive.dat
    ```

2.  **Add a Local Library**:
    ```bash
    ./bin/romman library add "Sega Genesis" /path/to/roms genesis
    ```

3.  **Scan and Match**:
    ```bash
    ./bin/romman library scan "Sega Genesis"
    ```

4.  **Browse in TUI**:
    ```bash
    ./bin/romman-tui
    ```



## Metadata & Media

`romman` integrates with IGDB to fetch game metadata and download boxart.

### Setup

Obtain your Client ID and Secret from the [Twitch Developer Console](https://dev.twitch.tv/console).

```bash
export IGDB_CLIENT_ID="your_client_id"
export IGDB_CLIENT_SECRET="your_client_secret"
```

### Usage

Scrape metadata for a specific release ID (requires a valid database ID):

```bash
romman scrape <release_id>
```

Or bulk scrape an entire library:

```bash
romman library scrape "My Library"
```

The metadata and boxart will be available in the Web UI.

## Observability

ROM Manager includes built-in support for structured logging and OpenTelemetry tracing.

### Logging

Structured logs are enabled by default using Go's `slog`. You can configure the log format (text/json) and level in `.romman.yaml`.

### Tracing

OpenTelemetry tracing is supported for long-running operations like DAT imports and library scans.

1.  **Start Jaeger** (for local visualization):
    ```bash
    docker compose -f docker-compose.jaeger.yml up -d
    ```
2.  **Run with Tracing**:
    Set the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable to point to your Jaeger/OTLP collector:
    ```bash
    export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
    ./bin/romman library scan "My Library"
    ```
3.  **View Traces**:
    Open [http://localhost:16686](http://localhost:16686) to browse spans and performance data.

## Development

Run tests across the workspace:

```bash
make test
```

## License

[License Type, e.g., MIT]
