# Milestone 7 – Configuration & Batch Operations

## Goal
Add user configuration file and batch commands for multi-system operations.

## Features

### Configuration File (.romman.yaml)
- DB path override
- Default region order
- Ignored extensions list
- Quarantine path default

### Batch Commands
- `library scan-all` - Scan all libraries
- `prefer rebuild-all` - Rebuild all systems
- `systems status` - Summary of all systems

### 1G1R Mode
- Filter preferred releases to 1 per base title
- Consider user region preferences
- CLI flag: `--1g1r`

## Steps
1. Define config file schema
2. Implement config loading (YAML)
3. Add batch scan command
4. Add batch prefer command
5. Implement 1G1R filtering
6. Update TUI for 1G1R toggle

## Acceptance Criteria
- Config file respected by all commands
- Batch commands work across all systems
- 1G1R produces one release per game
