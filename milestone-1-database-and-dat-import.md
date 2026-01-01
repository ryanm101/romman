# Milestone 1 – Database & DAT Import

## Goal
Create the foundational database and ingest DAT files as a canonical game catalogue.

## Scope
- SQLite schema (v1)
- Logiqx XML DAT importer
- RetroArch-compatible system detection
- CLI commands for DAT import and inspection

## Steps
1. Define SQLite schema:
   - systems
   - releases
   - rom_entries
   - migrations table
2. Implement database migrations (idempotent).
3. Implement Logiqx XML DAT parser:
   - Stream-based (no full DOM load)
   - Extract game and rom entries with hashes
4. Parse metadata tokens:
   - Region
   - Languages
   - Revision / stability hints
5. Auto-detect system name:
   - Map DAT header / filename to RetroArch system names
6. Store all releases, even non-preferred ones.
7. CLI commands:
   - `dat import`
   - `systems list`
   - `systems info`

## Acceptance Criteria
- DAT imports complete without errors
- Re-importing same DAT is idempotent
- Systems are named consistently with RetroArch
- Hash data (SHA1/CRC) stored correctly

