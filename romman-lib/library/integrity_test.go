package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create test file with known content
	content := []byte("hello world")
	err := os.WriteFile(testFile, content, 0644) // #nosec G306
	assert.NoError(t, err)

	hash, err := hashFile(testFile)
	assert.NoError(t, err)
	// SHA1 of "hello world" is "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed"
	assert.Equal(t, "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed", hash)
}

func TestHashFile_NotFound(t *testing.T) {
	_, err := hashFile("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestIntegrityIssueTypes(t *testing.T) {
	issues := []IntegrityIssue{
		{Path: "/a.rom", IssueType: "changed", Details: "size changed"},
		{Path: "/b.rom", IssueType: "missing", Details: "file not found"},
		{Path: "/c.rom", IssueType: "incomplete", Details: "has 2/5 files"},
	}

	assert.Equal(t, "changed", issues[0].IssueType)
	assert.Equal(t, "missing", issues[1].IssueType)
	assert.Equal(t, "incomplete", issues[2].IssueType)
}

func TestIntegrityResultDefaults(t *testing.T) {
	result := &IntegrityResult{}

	assert.Equal(t, 0, result.FilesChecked)
	assert.Equal(t, 0, result.OK)
	assert.Equal(t, 0, result.Changed)
	assert.Equal(t, 0, result.Missing)
	assert.Equal(t, 0, result.Incomplete)
	assert.Empty(t, result.Issues)
}
