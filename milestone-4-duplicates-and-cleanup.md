# Milestone 4 – Duplicates & Cleanup Plans

## Goal
Detect duplicates and allow safe cleanup via explicit plans.

## Duplicate Types
- Exact duplicates (same hash)
- Variant duplicates (same title, different release)
- Packaging duplicates (multiple files for same rom entry)

## Steps
1. Implement duplicate detectors.
2. Generate cleanup plan JSON:
   - Move / delete / ignore actions
   - Per-system quarantine paths
3. Implement dry-run execution.
4. Implement real execution.
5. Record plan execution results.

## Acceptance Criteria
- No file operations occur without a plan
- Dry-run and exec outputs differ only by side effects
- Quarantine is per-system and predictable

