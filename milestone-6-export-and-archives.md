# Milestone 6 – Export & Archive Support

## Goal
Add report export capabilities and support for scanning inside ZIP archives.

## Features

### Export Reports
- CLI command: `romman export <library> <format> [output]`
- Formats: CSV, JSON
- Reports: matched, missing, unmatched, preferred
- Include metadata: filename, hash, match type, flags

### ZIP Archive Scanning
- Detect ZIP files during scan
- Extract and hash ROM files inside
- Store archive path in database
- Support nested directories in ZIPs

## Steps
1. Add export command to CLI
2. Implement CSV/JSON formatters
3. Add ZIP detection in scanner
4. Implement in-memory ZIP extraction
5. Store archive_path for matched files
6. Update TUI to show archive paths

## Acceptance Criteria
- Export generates valid CSV/JSON
- ZIP contents matched against DAT
- Archive path shown in results
