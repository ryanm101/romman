package library

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLibraryError_Error(t *testing.T) {
	err := &LibraryError{
		Op:      "get library",
		Library: "nes",
		Err:     ErrNotFound,
	}

	assert.Contains(t, err.Error(), "get library")
	assert.Contains(t, err.Error(), "nes")
	assert.Contains(t, err.Error(), "not found")
}

func TestLibraryError_ErrorNoLibrary(t *testing.T) {
	err := &LibraryError{
		Op:  "list libraries",
		Err: ErrDatabase,
	}

	result := err.Error()
	assert.Contains(t, result, "list libraries")
	assert.Contains(t, result, "database error")
}

func TestLibraryError_Unwrap(t *testing.T) {
	err := &LibraryError{
		Op:  "test",
		Err: ErrNotFound,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, ErrNotFound, unwrapped)
}

func TestSystemError_Error(t *testing.T) {
	err := &SystemError{
		Op:     "import system",
		System: "gba",
		Err:    ErrDuplicate,
	}

	assert.Contains(t, err.Error(), "import system")
	assert.Contains(t, err.Error(), "gba")
}

func TestSystemError_ErrorNoSystem(t *testing.T) {
	err := &SystemError{
		Op:  "list systems",
		Err: ErrDatabase,
	}

	result := err.Error()
	assert.Contains(t, result, "list systems")
}

func TestSystemError_Unwrap(t *testing.T) {
	err := &SystemError{
		Op:  "test",
		Err: ErrInvalidArg,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, ErrInvalidArg, unwrapped)
}

func TestWrapDBError_Nil(t *testing.T) {
	err := WrapDBError(nil, "test")
	assert.Nil(t, err)
}

func TestWrapDBError_NoRows(t *testing.T) {
	err := WrapDBError(sql.ErrNoRows, "find item")

	var libErr *LibraryError
	assert.True(t, errors.As(err, &libErr))
	assert.True(t, errors.Is(libErr.Err, ErrNotFound))
}

func TestWrapDBError_UniqueConstraint(t *testing.T) {
	sqlErr := errors.New("UNIQUE constraint failed: libraries.name")
	err := WrapDBError(sqlErr, "add library")

	var libErr *LibraryError
	assert.True(t, errors.As(err, &libErr))
	assert.True(t, errors.Is(libErr.Err, ErrDuplicate))
}

func TestWrapDBError_ForeignKeyConstraint(t *testing.T) {
	sqlErr := errors.New("FOREIGN KEY constraint failed")
	err := WrapDBError(sqlErr, "add release")

	var libErr *LibraryError
	assert.True(t, errors.As(err, &libErr))
	assert.True(t, errors.Is(libErr.Err, ErrDatabase))
}

func TestWrapDBError_NoSuchTable(t *testing.T) {
	sqlErr := errors.New("no such table: systems")
	err := WrapDBError(sqlErr, "query")

	var libErr *LibraryError
	assert.True(t, errors.As(err, &libErr))
	assert.True(t, errors.Is(libErr.Err, ErrDatabase))
}

func TestWrapDBError_GenericError(t *testing.T) {
	sqlErr := errors.New("some other database error")
	err := WrapDBError(sqlErr, "operation")

	var libErr *LibraryError
	assert.True(t, errors.As(err, &libErr))
	assert.True(t, errors.Is(libErr.Err, ErrDatabase))
}

func TestNotFoundError(t *testing.T) {
	err := NotFoundError("library", "nes")

	var libErr *LibraryError
	assert.True(t, errors.As(err, &libErr))
	assert.Equal(t, "find library", libErr.Op)
	assert.Equal(t, "nes", libErr.Library)
	assert.True(t, errors.Is(libErr.Err, ErrNotFound))
}

func TestContains(t *testing.T) {
	assert.True(t, contains("hello world", "world"))
	assert.True(t, contains("hello world", "hello"))
	assert.True(t, contains("hello", "hello"))
	assert.False(t, contains("hello", "world"))
	assert.False(t, contains("he", "hello"))
	assert.True(t, contains("", ""))
}

func TestSentinelErrors(t *testing.T) {
	assert.Equal(t, "not found", ErrNotFound.Error())
	assert.Equal(t, "duplicate entry", ErrDuplicate.Error())
	assert.Equal(t, "database error", ErrDatabase.Error())
	assert.Equal(t, "invalid argument", ErrInvalidArg.Error())
}
