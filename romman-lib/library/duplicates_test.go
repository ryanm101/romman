package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDuplicateTypeConstants(t *testing.T) {
	assert.Equal(t, DuplicateType("exact"), DuplicateExact)
	assert.Equal(t, DuplicateType("variant"), DuplicateVariant)
	assert.Equal(t, DuplicateType("package"), DuplicatePackage)
}

func TestMarkPreferred_Empty(t *testing.T) {
	var files []DuplicateFile
	markPreferred(files) // Should not panic
	assert.Empty(t, files)
}

func TestMarkPreferred_SingleFile(t *testing.T) {
	files := []DuplicateFile{
		{ScannedFileID: 1, Path: "/a.rom"},
	}
	markPreferred(files)
	assert.True(t, files[0].IsPreferred)
}

func TestMarkPreferred_PrefersSHA1Match(t *testing.T) {
	files := []DuplicateFile{
		{ScannedFileID: 1, Path: "/a.rom", MatchType: "crc32"},
		{ScannedFileID: 2, Path: "/b.rom", MatchType: "sha1"},
		{ScannedFileID: 3, Path: "/c.rom", MatchType: "name"},
	}
	markPreferred(files)

	assert.False(t, files[0].IsPreferred)
	assert.True(t, files[1].IsPreferred) // sha1 should be preferred
	assert.False(t, files[2].IsPreferred)
}

func TestMarkPreferred_PenalizesFlags(t *testing.T) {
	files := []DuplicateFile{
		{ScannedFileID: 1, Path: "/a.rom", MatchType: "sha1", Flags: "bad-dump"},
		{ScannedFileID: 2, Path: "/b.rom", MatchType: "sha1", Flags: ""},
	}
	markPreferred(files)

	assert.False(t, files[0].IsPreferred) // has flags
	assert.True(t, files[1].IsPreferred)  // no flags
}

func TestScoreFile(t *testing.T) {
	tests := []struct {
		name     string
		file     DuplicateFile
		minScore int
		maxScore int
	}{
		{"sha1 match", DuplicateFile{MatchType: "sha1", Path: "/a.rom"}, 90, 110},
		{"crc32 match", DuplicateFile{MatchType: "crc32", Path: "/a.rom"}, 70, 90},
		{"name match", DuplicateFile{MatchType: "name", Path: "/a.rom"}, 40, 60},
		{"name_modified", DuplicateFile{MatchType: "name_modified", Path: "/a.rom"}, 10, 30},
		{"with flags penalty", DuplicateFile{MatchType: "sha1", Flags: "bad", Path: "/a.rom"}, 80, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreFile(tt.file)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
		})
	}
}
