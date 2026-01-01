package pack

import "errors"

// Errors for the pack package.
var (
	// ErrUnsupportedFormat is returned when requesting an unknown export format.
	ErrUnsupportedFormat = errors.New("unsupported export format")
	// ErrNoGames is returned when trying to generate a pack with no games.
	ErrNoGames = errors.New("no games specified for pack")
	// ErrFileNotFound is returned when a ROM file cannot be found.
	ErrFileNotFound = errors.New("ROM file not found")
)
