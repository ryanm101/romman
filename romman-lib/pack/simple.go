package pack

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

// SimpleExporter copies ROMs to a zip with basic folder structure.
type SimpleExporter struct{}

// Format returns the format this exporter produces.
func (e *SimpleExporter) Format() Format {
	return FormatSimple
}

// Export writes games to the zip with system-based folder structure.
func (e *SimpleExporter) Export(games []Game, zw *zip.Writer) error {
	if len(games) == 0 {
		return ErrNoGames
	}

	for _, game := range games {
		// Create path: system/filename.ext
		zipPath := filepath.Join(game.System, game.FileName)

		if err := addFileToZip(zw, game.FilePath, zipPath); err != nil {
			return err
		}
	}

	return nil
}

// addFileToZip adds a file from the filesystem to the zip archive.
func addFileToZip(zw *zip.Writer, srcPath, zipPath string) error {
	// #nosec G304
	file, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFound
		}
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}
