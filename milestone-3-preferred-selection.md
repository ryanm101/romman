# Milestone 3 – Preferred Release Selection

## Goal
Select exactly one preferred release per game using deterministic rules.

## Selection Rules
1. Language:
   - Must include English (alone or among others)
2. Stability:
   - Prefer stable over beta/proto/sample
3. Revision:
   - Highest stable revision wins
4. Region:
   - Default order: Europe > World > USA
   - User-configurable

## Steps
1. Normalise base titles for grouping.
2. Parse revision and stability tokens.
3. Apply selection algorithm per base title.
4. Mark preferred release.
5. Mark others as ignored with reason.
6. CLI commands:
   - `prefer set`
   - `prefer rebuild`

## Acceptance Criteria
- Exactly one preferred release per base title
- Rules produce repeatable results
- Changing region order recomputes correctly

