# romman-tui

An interactive terminal user interface for ROM Manager, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Dashboard**: Overview of all imported systems and registered libraries.
- **Progress Bars**: Visual feedback on library completion status.
- **Detail View**: Filterable list of games (Matched, Missing, Flagged, Unmatched, Preferred).
- **Expansion**: Item selection shows path, match type, and flags.
- **Search**: Fast real-time filtering using the `/` key.
- **Keyboard Friendly**: Navigable with `j/k`, `Tab`, `Page Up/Down`, and `Esc`.

## Controls

| Key | Action |
| --- | --- |
| `Tab` | Switch between Systems and Libraries panels |
| `j`/`k` | Navigate lists |
| `Enter` | View library details or expand item |
| `s` | Trigger library scan |
| `/` | Enter search mode |
| `1`-`5` | Switch Detail View filters |
| `r` | Refresh data |
| `q` / `Esc`| Quit or go back |

## Build

```bash
make build
# or
go build -o romman-tui .
```
