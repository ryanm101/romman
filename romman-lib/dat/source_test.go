package dat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		name     string
		datName  string
		expected SourceType
	}{
		{"No-Intro explicit", "Nintendo - Game Boy (No-Intro)", SourceNoIntro},
		{"No-Intro lowercase", "no-intro - nes", SourceNoIntro},
		{"No-Intro merged", "Nintendo - NES (NoIntro) v2024", SourceNoIntro},
		{"Redump explicit", "Redump - Sony PlayStation", SourceRedump},
		{"Redump lowercase", "redump psx", SourceRedump},
		{"TOSEC explicit", "TOSEC - Commodore 64", SourceTOSEC},
		{"TOSEC lowercase", "tosec amiga", SourceTOSEC},
		{"MAME explicit", "MAME 0.250", SourceMAME},
		{"MAME software list", "Software List - NES", SourceMAME},
		{"Unknown source", "Nintendo Game Boy", SourceOther},
		{"Empty string", "", SourceOther},
		{"Random name", "My Custom DAT", SourceOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectSourceType(tt.datName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file with known content
	testFile := filepath.Join(tmpDir, "test.dat")
	content := []byte("test content for hashing")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	hash, err := HashFile(testFile)
	require.NoError(t, err)

	// SHA256 of "test content for hashing"
	assert.Len(t, hash, 64, "SHA256 hash should be 64 hex characters")
	assert.NotEmpty(t, hash)

	// Same file should produce same hash
	hash2, err := HashFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2, "same file should produce same hash")
}

func TestHashFile_NotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file.dat")
	assert.Error(t, err)
}

func TestHashFile_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two files with different content
	file1 := filepath.Join(tmpDir, "file1.dat")
	file2 := filepath.Join(tmpDir, "file2.dat")

	err := os.WriteFile(file1, []byte("content 1"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(file2, []byte("content 2"), 0644)
	require.NoError(t, err)

	hash1, err := HashFile(file1)
	require.NoError(t, err)

	hash2, err := HashFile(file2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "different files should have different hashes")
}

func TestSourceType_String(t *testing.T) {
	assert.Equal(t, "no-intro", string(SourceNoIntro))
	assert.Equal(t, "redump", string(SourceRedump))
	assert.Equal(t, "tosec", string(SourceTOSEC))
	assert.Equal(t, "mame", string(SourceMAME))
	assert.Equal(t, "other", string(SourceOther))
}

func TestDATSource_Struct(t *testing.T) {
	ds := DATSource{
		ID:          1,
		SystemID:    2,
		SourceType:  SourceNoIntro,
		DATName:     "Test DAT",
		DATVersion:  "1.0",
		DATDate:     "2024-01-01",
		DATFilePath: "/path/to/dat.xml",
		DATFileHash: "abc123",
		Priority:    0,
	}

	assert.Equal(t, int64(1), ds.ID)
	assert.Equal(t, int64(2), ds.SystemID)
	assert.Equal(t, SourceNoIntro, ds.SourceType)
	assert.Equal(t, "Test DAT", ds.DATName)
	assert.Equal(t, 0, ds.Priority)
}
