# Milestone 14: User Experience Improvements

Focus on CLI usability, progress feedback, and operational tooling.

## Goals
- Better feedback during long operations
- Machine-readable output options
- Improved operational tooling

## Tasks

### CLI Enhancements
- [ ] Add progress bars for scan operations (files processed, time remaining)
- [ ] Add `--json` flag for machine-readable output on all commands
- [ ] Add `--quiet` flag to suppress progress output
- [ ] Add `romman doctor` command for database integrity checks
- [ ] Add `romman backup` command for database backup

### Web Enhancements
- [ ] Add `/health` endpoint for container health checks
- [ ] Add real-time scan progress via WebSocket/SSE
- [ ] Add dark mode toggle
- [ ] Add system icons/logos

### Configuration
- [ ] Add config validation on load (workers > 0, valid paths)
- [ ] Add `romman config show` command to display active config
- [ ] Add `romman config init` to generate example config

### Operational
- [ ] Auto-backup database before destructive operations
- [ ] Add scan resumption (remember progress on failure)
- [ ] Add `--dry-run` to more commands

## Success Criteria
- Long operations show progress
- All commands support `--json` output
- `/health` endpoint returns proper status codes
