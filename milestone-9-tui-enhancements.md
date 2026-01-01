# Milestone 9 – TUI Enhancements

## Goal
Improve the TUI with better visual feedback, statistics, and navigation.

## Features

### Visual Improvements
- Progress bars in library panel (visual completion)
- Color-coded list items (green=matched, yellow=flagged, red=missing)
- Counts in tab labels: `[1] Matched (36)`

### Information Display
- Quick stats header in detail view (system name + totals)
- Status bar showing last scan time and DB path
- Expand item details (Enter shows path, hash, flags)

### Navigation
- Search/filter with `/` key
- Page up/down for faster scrolling

## Steps
1. Add progress bar component
2. Color-code detail items by type
3. Add counts to tab labels
4. Add stats header to detail view
5. Implement search/filter
6. Add item expansion view

## Acceptance Criteria
- Progress bars render correctly
- Colors distinguish item types
- Tab counts update dynamically
- Search filters list in real-time
