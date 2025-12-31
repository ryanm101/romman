package library

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// CHD header constants
const (
	chdMagic    = "MComprHD"
	chdV4Header = 108
	chdV5Header = 124
	sha1Size    = 20
)

// CHDInfo contains metadata extracted from a CHD file header.
type CHDInfo struct {
	Version      uint32
	TotalHunks   uint32
	HunkBytes    uint32
	LogicalBytes uint64
	SHA1         string // SHA1 of compressed data
	DataSHA1     string // SHA1 of decompressed data (most important for matching)
	ParentSHA1   string // SHA1 of parent CHD (if delta file)
}

// ParseCHD reads a CHD file header and extracts hash information.
func ParseCHD(path string) (*CHDInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CHD: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Read magic and version
	header := make([]byte, 16)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	magic := string(header[:8])
	if magic != chdMagic {
		return nil, fmt.Errorf("not a valid CHD file (bad magic: %s)", magic)
	}

	headerLen := binary.BigEndian.Uint32(header[8:12])
	version := binary.BigEndian.Uint32(header[12:16])

	info := &CHDInfo{Version: version}

	// Seek back to start and read full header
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch version {
	case 4:
		return parseCHDv4(f, info, headerLen)
	case 5:
		return parseCHDv5(f, info, headerLen)
	default:
		return nil, fmt.Errorf("unsupported CHD version: %d", version)
	}
}

// parseCHDv4 parses CHD version 4 header.
// v4 header layout (108 bytes):
//
//	0-7:   magic
//	8-11:  header length
//	12-15: version
//	16-19: flags
//	20-23: compression type
//	24-27: total hunks
//	28-35: logical bytes
//	36-43: meta offset
//	44-47: hunk bytes
//	48-67: SHA1 (compressed)
//	68-87: Parent SHA1
//	88-107: Data SHA1 (raw)
func parseCHDv4(f *os.File, info *CHDInfo, headerLen uint32) (*CHDInfo, error) {
	header := make([]byte, headerLen)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("failed to read v4 header: %w", err)
	}

	info.TotalHunks = binary.BigEndian.Uint32(header[24:28])
	info.LogicalBytes = binary.BigEndian.Uint64(header[28:36])
	info.HunkBytes = binary.BigEndian.Uint32(header[44:48])

	// Extract SHA1 hashes
	info.SHA1 = hex.EncodeToString(header[48:68])
	info.ParentSHA1 = hex.EncodeToString(header[68:88])
	info.DataSHA1 = hex.EncodeToString(header[88:108])

	return info, nil
}

// parseCHDv5 parses CHD version 5 header.
// v5 header layout (124 bytes):
//
//	0-7:   magic
//	8-11:  header length
//	12-15: version
//	16-19: compressor 0
//	20-23: compressor 1
//	24-27: compressor 2
//	28-31: compressor 3
//	32-39: logical bytes
//	40-47: map offset
//	48-55: meta offset
//	56-59: hunk bytes
//	60-63: unit bytes
//	64-83: SHA1 (compressed)
//	84-103: Data SHA1 (raw)
//	104-123: Parent SHA1
func parseCHDv5(f *os.File, info *CHDInfo, headerLen uint32) (*CHDInfo, error) {
	header := make([]byte, headerLen)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("failed to read v5 header: %w", err)
	}

	info.LogicalBytes = binary.BigEndian.Uint64(header[32:40])
	info.HunkBytes = binary.BigEndian.Uint32(header[56:60])

	// Extract SHA1 hashes
	info.SHA1 = hex.EncodeToString(header[64:84])
	info.DataSHA1 = hex.EncodeToString(header[84:104])
	info.ParentSHA1 = hex.EncodeToString(header[104:124])

	return info, nil
}

// IsCHDFile checks if a file has a .chd extension.
func IsCHDFile(path string) bool {
	ext := getExtLower(path)
	return ext == ".chd"
}

func getExtLower(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext := path[i:]
			// Convert to lowercase
			result := make([]byte, len(ext))
			for j, c := range ext {
				if c >= 'A' && c <= 'Z' {
					result[j] = byte(c) + 32
				} else {
					result[j] = byte(c)
				}
			}
			return string(result)
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}
