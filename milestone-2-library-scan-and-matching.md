# Milestone 2 – Library Scan & Hash Matching

## Goal
Detect which catalogue entries exist in local ROM libraries using hash-first matching.

## Scope
- Directory scanning (one dir per system)
- Zip-only archive support
- SHA1 + CRC32 hashing
- Match tracking and file inventory

## Steps
1. Implement library registration:
   - Named library
   - Root path
2. Implement directory walker:
   - System-scoped
   - Zip-aware
3. For each file:
   - Compute SHA1 and CRC32
   - Cache hash using size + mtime
4. Match files to rom_entries:
   - SHA1 first
   - CRC32 fallback
5. Record matches and unmatched files.
6. Compute release status:
   - Present (all rom_entries matched)
   - Missing (one or more missing)

## Acceptance Criteria
- Re-scans do not re-hash unchanged files
- Zip-contained ROMs match correctly
- Unmatched files are reported explicitly
- Status output matches expectation for known sets

