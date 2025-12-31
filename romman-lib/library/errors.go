package library

import (
	"database/sql"
	"errors"
	"fmt"
)

// Sentinel errors for common conditions.
var (
	ErrNotFound   = errors.New("not found")
	ErrDuplicate  = errors.New("duplicate entry")
	ErrDatabase   = errors.New("database error")
	ErrInvalidArg = errors.New("invalid argument")
)

// LibraryError provides context for library-related errors.
type LibraryError struct {
	Op      string // Operation that failed (e.g., "get library")
	Library string // Library name if applicable
	Err     error  // Underlying error
}

func (e *LibraryError) Error() string {
	if e.Library != "" {
		return fmt.Sprintf("%s '%s': %v", e.Op, e.Library, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *LibraryError) Unwrap() error {
	return e.Err
}

// SystemError provides context for system-related errors.
type SystemError struct {
	Op     string // Operation that failed
	System string // System name if applicable
	Err    error  // Underlying error
}

func (e *SystemError) Error() string {
	if e.System != "" {
		return fmt.Sprintf("%s '%s': %v", e.Op, e.System, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *SystemError) Unwrap() error {
	return e.Err
}

// WrapDBError converts a database error to a user-friendly error.
func WrapDBError(err error, op string) error {
	if err == nil {
		return nil
	}

	// Handle common SQL errors
	if errors.Is(err, sql.ErrNoRows) {
		return &LibraryError{Op: op, Err: ErrNotFound}
	}

	// Check for constraint violations (SQLite specific patterns)
	errStr := err.Error()
	if contains(errStr, "UNIQUE constraint failed") {
		return &LibraryError{Op: op, Err: fmt.Errorf("%w: entry already exists", ErrDuplicate)}
	}
	if contains(errStr, "FOREIGN KEY constraint failed") {
		return &LibraryError{Op: op, Err: fmt.Errorf("%w: referenced item does not exist", ErrDatabase)}
	}
	if contains(errStr, "no such table") {
		return &LibraryError{Op: op, Err: fmt.Errorf("%w: database not initialized", ErrDatabase)}
	}

	// Generic database error
	return &LibraryError{Op: op, Err: fmt.Errorf("%w: %v", ErrDatabase, err)}
}

// NotFoundError returns a user-friendly "not found" error.
func NotFoundError(itemType, name string) error {
	return &LibraryError{
		Op:      fmt.Sprintf("find %s", itemType),
		Library: name,
		Err:     ErrNotFound,
	}
}

// contains checks if s contains substr (simple helper to avoid strings import).
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
