# Milestone 11 – Performance & Ecosystem Integration

## Overview
This milestone focuses on transforming ROM Manager into a production-grade tool by optimizing the core scanning engine and expanding compatibility with the emulation ecosystem.

## Goals
1. **Extreme Performance**: Move from sequential to parallel scanning.
2. **Disc Support**: Implement CHD file header parsing for CD-based systems.
3. **Frontend Polish**: Transition Web UI to a professional `go:embed` asset structure.
4. **Integration**: Enable direct library export for RetroArch.

## Scope

### 1. Parallel Scanner (romman-lib/library)
- Implement a worker pool (goroutines) for file hashing.
- Switch to batched SQLite transactions for scan persistence.
- Add thread-safe progress reporting to `ScanResult`.

### 2. CHD Support (romman-lib/library)
- Add a CHD header parser to extract SHA1/CRC32 from compressed disc images.
- Ensure the scanner treats `.chd` as a first-class citizen alongside `.zip`.

### 3. Web Asset Embedding (romman-web)
- Extract CSS, JS, and HTML into an `assets/` directory.
- Use `go:embed` to bundle assets into the binary.
- Clean up `main.go` by removing thousands of lines of raw strings.

### 4. RetroArch Integration (romman-cli/romman-lib)
- Implement `romman export <lib> retroarch` command.
- Generate `.lpl` playlist files matching the RetroArch format.
- Support standard RetroArch system name mapping.

## Acceptance Criteria
- [ ] Scanning a 1000-file library is at least 3x faster on multi-core systems.
- [ ] CHD files are correctly matched against Redump-style DATs.
- [ ] The `romman-web` binary remains a single file but assets are developed in standalone files.
- [ ] RetroArch successfully loads a generated `.lpl` playlist.
- [ ] All tests pass including new parallel-specific edge cases.
