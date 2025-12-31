# romman-web

A lightweight, high-performance web dashboard for ROM Manager, built with Go's standard library.

## Features

- **Summary Dashboard**: High-level statistics of imported systems, libraries, and total releases.
- **Systems List**: Overview of all imported systems and their preferred release counts.
- **Library Progress**: Visual progress bars showing the match percentage for each registered library.
- **JSON API**: RESTful endpoints for integration with other tools.
- **Single Binary**: The entire UI is embedded in the Go binary for zero-dependency deployment.

## API Endpoints

- `GET /api/stats`: Returns global counts.
- `GET /api/systems`: Returns list of all systems.
- `GET /api/libraries`: Returns list of libraries with match percentages.
- `GET /metrics`: Prometheus metrics endpoint.

## Build

```bash
make build
# or
go build -o romman-web .
```

## Running

By default, the server runs on port `8080`. You can override this with the `ROMMAN_PORT` environment variable.

```bash
export ROMMAN_PORT=9000
./romman-web
```

Access the dashboard at `http://localhost:9000`.
