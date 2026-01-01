# Milestone 8 – Advanced Features

## Goal
Add ROM renaming, integrity checking, and optional web interface.

## Features

### ROM Renaming
- Rename files to match DAT entry names
- Dry-run mode with preview
- Backup original names
- Handle duplicates safely

### Integrity Checking
- Detect split/merged sets
- Identify incomplete multi-file games
- Verify existing hashes still match

### Fuzzy Matching (Optional)
- Levenshtein distance for title matching
- Configurable threshold
- Manual confirmation for uncertain matches

### Web UI (Optional)
- Simple HTTP server
- Dashboard with stats
- Read-only API
- Trigger scans remotely

## Steps
1. Implement rename command
2. Add integrity check command
3. Add fuzzy matching option
4. Design web API endpoints
5. Build minimal web dashboard

## Acceptance Criteria
- Renaming preserves data safety
- Integrity issues clearly reported
- Web UI functional if enabled
