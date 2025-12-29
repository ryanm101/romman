# ROM Manager – Project Objective

## Objective
Build a cross-platform ROM management tool that allows users to:
- Maintain a canonical catalogue of retro games per system using DAT files
- Detect which games are present or missing in their local ROM libraries
- Eliminate duplicates and non-preferred variants safely
- Prefer a single “best” version of each game based on language, stability, and region
- Operate via a deterministic CLI, with a UI layer that constructs and executes commands

The tool prioritises correctness, auditability, and user control over automation magic.

## Non-Goals
- Full MAME set correctness or parent/clone/device ROM resolution
- Automatic metadata scraping (covers, descriptions, ratings)
- Emulation, launching, or frontend replacement
- Online account sync or cloud storage

## Key Principles
- **Hash-first matching** (SHA1 preferred, CRC32 fallback)
- **Source-of-truth = DAT files**
- **No destructive actions without an explicit plan**
- **UI never owns logic** – CLI is authoritative
- **Cross-platform by default**

## Target Users
- Retro gaming enthusiasts with medium-to-large collections (10k+ ROMs)
- Users who want clean, minimal libraries (one preferred version per game)
- Users who already understand ROM concepts and value transparency

## Success Criteria
- User can import DATs and see a correct per-system catalogue
- User can scan libraries and get accurate present/missing status
- User can generate, review, and execute duplicate cleanup plans safely
- The same workflow works on Linux, Windows, macOS, and Steam Deck

