# romman-cli

The authoritative command-line interface for ROM Manager. It provides granular control over the catalogue and libraries.

## Commands

- `dat import <file>`: Import a system DAT file into the catalogue.
- `systems`: List all imported systems and their game counts.
- `library add <name> <path> <system>`: Register a new ROM library.
- `library scan <name>`: Scan a library, compute hashes, and match games.
- `library status <name>`: Show completeness statistics and missing games.
- `prefer rebuild <system>`: Recompute preferred releases based on current rules.
- `cleanup generate <library>`: Create a sidecar JSON plan to remove/quarantine duplicates.
- `cleanup execute <plan.json>`: Apply a generated cleanup plan.

## Examples

Importing a DAT and scanning a library:
```bash
romman dat import nes.dat
romman library add "NES Library" /roms/nes nes
romman library scan "NES Library"
```

## Build

```bash
make build
# or
go build -o romman .
```
