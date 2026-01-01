# Milestone 12 – Observability & Instrumentation

## Overview
Implement structured logging and distributed tracing using Go's standard library and OpenTelemetry for production-grade observability.

## Goals
1. **Structured Logging**: Replace all `fmt.Printf` calls with `log/slog`.
2. **Tracing**: Integrate OpenTelemetry for spans across scan operations.
3. **Metrics**: Expose Prometheus-compatible metrics for monitoring.

## Scope

### 1. Structured Logging (romman-lib)
- Adopt Go 1.21+ `log/slog` as the standard logger.
- Create a configurable logger that supports JSON and text output formats.
- Replace all library `fmt.Printf` warning calls with `slog.Warn`.
- Add log levels: Debug (verbose hashing), Info (scan progress), Warn (recoverable errors).

### 2. OpenTelemetry Integration
- Add `go.opentelemetry.io/otel` dependency.
- Instrument the scanner with spans:
  - `scan.library` (root span)
  - `scan.file` (per-file span with hash duration)
  - `scan.match` (matching phase)
- Export traces to OTLP endpoint (configurable via `OTEL_EXPORTER_OTLP_ENDPOINT`).

### 3. Metrics Endpoint (romman-web)
- Add `/metrics` endpoint exposing Prometheus format.
- Track: `romman_scans_total`, `romman_files_hashed`, `romman_match_rate`.

## Acceptance Criteria
- [ ] All library logging uses `slog` instead of `fmt.Printf`.
- [ ] Logger output format is configurable (JSON for production, text for dev).
- [ ] Traces are visible in Jaeger/Tempo when OTLP endpoint is configured.
- [ ] `/metrics` endpoint returns valid Prometheus scrape data.
- [ ] No performance regression from instrumentation (< 5% overhead).
