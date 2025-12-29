# ROM Manager – Test Plan

## Test Strategy
- Unit tests for parsing, hashing, and selection logic
- Integration tests using real DATs and sample ROM trees
- Destructive actions tested only via dry-run

## Test Categories

### DAT Import
- Valid Logiqx DAT imports
- Duplicate DAT re-imports
- DAT with missing hashes
- Unknown system detection

### Scanning
- Extracted ROMs
- Zip-contained ROMs
- Mixed valid/invalid files
- Large directory (10k+ files)

### Matching
- SHA1 exact matches
- CRC fallback matches
- False-positive prevention
- Multi-file releases

### Preferred Selection
- Multiple languages including English
- Stable vs beta precedence
- Revision ordering
- Region priority changes

### Duplicates
- Identical hashes in multiple locations
- Multiple variants of same title
- Duplicate matches to same rom_entry

### Cleanup Plans
- Plan generation correctness
- Dry-run safety
- Execution correctness
- Partial failures and rollback behaviour

## Non-Functional Tests
- Cross-platform (Linux, Windows, macOS)
- Performance on spinning disk vs SSD
- Database corruption resilience
- Interrupted scan recovery

## Acceptance Exit Criteria
- No data loss without explicit execution
- All core commands produce valid JSON
- Repeatable results across runs

