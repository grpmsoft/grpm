// Package binpkg implements TBZ2 format writing.
package binpkg

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	cbzip2 "github.com/dsnet/compress/bzip2"
)

// TBZ2Writer writes TBZ2 format packages.
//
// TBZ2 format (legacy):
//   - Tar archive with package files
//   - Bzip2 compressed
//   - XPAK metadata appended at end
//
// Format: [tar.bz2][XPAK]
type TBZ2Writer struct {
	// OutputPath is the path for the output .tbz2 file
	OutputPath string

	// Verbose enables detailed logging
	Verbose bool
}

// NewTBZ2Writer creates a new TBZ2 writer.
func NewTBZ2Writer(outputPath string) *TBZ2Writer {
	return &TBZ2Writer{
		OutputPath: outputPath,
		Verbose:    false,
	}
}

// Write writes a TBZ2 package.
//
// Parameters:
//   - metadata: Package build metadata
//   - stagingDir: Directory containing files to pack
//
// Returns:
//   - error: If writing fails
func (w *TBZ2Writer) Write(metadata *BuildMetadata, stagingDir string) error {
	if w.Verbose {
		fmt.Printf("Creating TBZ2 package: %s\n", w.OutputPath)
	}

	// Create output directory if needed
	dir := filepath.Dir(w.OutputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 1: Create tar.bz2 archive
	tarBz2Path := w.OutputPath + ".tmp"
	if err := w.createTarBz2(stagingDir, tarBz2Path); err != nil {
		return fmt.Errorf("failed to create tar.bz2: %w", err)
	}
	defer func() { _ = os.Remove(tarBz2Path) }()

	// Step 2: Create XPAK metadata
	xpakData, err := w.createXPAK(metadata, stagingDir)
	if err != nil {
		return fmt.Errorf("failed to create XPAK: %w", err)
	}

	// Step 3: Combine tar.bz2 + XPAK
	if err := w.combineFiles(tarBz2Path, xpakData); err != nil {
		return fmt.Errorf("failed to combine files: %w", err)
	}

	if w.Verbose {
		fmt.Printf("TBZ2 package created successfully: %s\n", w.OutputPath)
	}

	return nil
}

// createTarBz2 creates a bzip2-compressed tar archive.
func (w *TBZ2Writer) createTarBz2(stagingDir, outputPath string) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// Create bzip2 writer
	bz2Writer, err := cbzip2.NewWriter(outFile, &cbzip2.WriterConfig{Level: 9})
	if err != nil {
		return fmt.Errorf("failed to create bzip2 writer: %w", err)
	}
	defer func() { _ = bz2Writer.Close() }()

	// Create tar writer
	tarWriter := tar.NewWriter(bz2Writer)
	defer func() { _ = tarWriter.Close() }()

	// Walk staging directory and add files to tar
	return filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Relative path
		relPath, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Convert to Unix path
		tarPath := filepath.ToSlash(relPath)

		if info.IsDir() {
			// Add directory
			return w.addDirectoryToTar(tarWriter, tarPath, info)
		} else if info.Mode()&os.ModeSymlink != 0 {
			// Add symlink
			return w.addSymlinkToTar(tarWriter, tarPath, path)
		} else {
			// Add regular file
			return w.addFileToTar(tarWriter, tarPath, path, info)
		}
	})
}

// addDirectoryToTar adds a directory entry to tar.
func (w *TBZ2Writer) addDirectoryToTar(tarWriter *tar.Writer, tarPath string, info os.FileInfo) error {
	header := &tar.Header{
		Name:     tarPath + "/",
		Mode:     int64(info.Mode().Perm()),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeDir,
	}
	return tarWriter.WriteHeader(header)
}

// addSymlinkToTar adds a symlink entry to tar.
func (w *TBZ2Writer) addSymlinkToTar(tarWriter *tar.Writer, tarPath, srcPath string) error {
	target, err := os.Readlink(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", srcPath, err)
	}

	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:     tarPath,
		Mode:     int64(info.Mode().Perm()),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeSymlink,
		Linkname: target,
	}
	return tarWriter.WriteHeader(header)
}

// addFileToTar adds a regular file to tar.
func (w *TBZ2Writer) addFileToTar(tarWriter *tar.Writer, tarPath, srcPath string, info os.FileInfo) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", srcPath, err)
	}
	defer func() { _ = file.Close() }()

	header := &tar.Header{
		Name:    tarPath,
		Size:    info.Size(),
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("failed to write file data: %w", err)
	}

	return nil
}

// createXPAK creates XPAK metadata.
//
// XPAK format:
//   - "XPAKPACK" (8 bytes)
//   - Index section (key offsets and lengths)
//   - Data section (key values)
//   - Footer: index_len (4) + data_len (4) + xpak_len (4) + "XPAKSTOP" (8)
func (w *TBZ2Writer) createXPAK(metadata *BuildMetadata, stagingDir string) ([]byte, error) {
	// Collect XPAK entries
	entries := make(map[string][]byte)

	// Add metadata entries
	if !metadata.BuildDate.IsZero() {
		entries["BUILD_TIME"] = []byte(strconv.FormatInt(metadata.BuildDate.Unix(), 10))
	}
	if metadata.BuildHost != "" {
		entries["BUILD_HOST"] = []byte(metadata.BuildHost)
	}
	if metadata.CFLAGS != "" {
		entries["CFLAGS"] = []byte(metadata.CFLAGS)
	}
	if metadata.CXXFLAGS != "" {
		entries["CXXFLAGS"] = []byte(metadata.CXXFLAGS)
	}
	if metadata.LDFLAGS != "" {
		entries["LDFLAGS"] = []byte(metadata.LDFLAGS)
	}
	if len(metadata.USE) > 0 {
		entries["USE"] = []byte(strings.Join(metadata.USE, " "))
	}
	if len(metadata.Features) > 0 {
		entries["FEATURES"] = []byte(strings.Join(metadata.Features, " "))
	}
	if metadata.EAPI != "" {
		entries["EAPI"] = []byte(metadata.EAPI)
	}
	if metadata.Size > 0 {
		entries["SIZE"] = []byte(strconv.FormatInt(metadata.Size, 10))
	}

	// Create CONTENTS file
	contents, err := w.createContents(stagingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create CONTENTS: %w", err)
	}
	entries["CONTENTS"] = []byte(contents)

	// Encode XPAK
	return encodeXPAK(entries)
}

// createContents creates CONTENTS file content.
func (w *TBZ2Writer) createContents(stagingDir string) (string, error) {
	contents := ""

	err := filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// Absolute path (as it would be installed)
		absPath := "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			contents += fmt.Sprintf("dir %s\n", absPath)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			contents += fmt.Sprintf("sym %s -> %s\n", absPath, target)
		} else {
			checksum, err := computeSHA256(path)
			if err != nil {
				return err
			}
			contents += fmt.Sprintf("obj %s %s %d %d\n", absPath, checksum, info.Mode().Perm(), info.ModTime().Unix())
		}

		return nil
	})

	return contents, err
}

// encodeXPAK encodes entries into XPAK format.
func encodeXPAK(entries map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer

	// Write magic "XPAKPACK"
	buf.WriteString("XPAKPACK")

	// Build index and data sections
	var indexBuf bytes.Buffer
	var dataBuf bytes.Buffer

	dataOffset := int32(0)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}

	for _, key := range keys {
		value := entries[key]

		// Write index entry: key_len (4) + key + data_offset (4) + data_len (4)
		keyBytes := []byte(key)
		if err := binary.Write(&indexBuf, binary.BigEndian, int32(len(keyBytes))); err != nil {
			return nil, err
		}
		indexBuf.Write(keyBytes)

		if err := binary.Write(&indexBuf, binary.BigEndian, dataOffset); err != nil {
			return nil, err
		}
		if err := binary.Write(&indexBuf, binary.BigEndian, int32(len(value))); err != nil {
			return nil, err
		}

		// Write data
		dataBuf.Write(value)
		dataOffset += int32(len(value))
	}

	// Write index section
	buf.Write(indexBuf.Bytes())

	// Write data section
	buf.Write(dataBuf.Bytes())

	// Write footer: index_len + data_len + xpak_len + "XPAKSTOP"
	indexLen := int32(indexBuf.Len())
	dataLen := int32(dataBuf.Len())
	xpakLen := 8 + indexLen + dataLen + 20 // "XPAKPACK" + index + data + footer

	if err := binary.Write(&buf, binary.BigEndian, indexLen); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, dataLen); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.BigEndian, xpakLen); err != nil {
		return nil, err
	}
	buf.WriteString("XPAKSTOP")

	return buf.Bytes(), nil
}

// combineFiles combines tar.bz2 and XPAK into final TBZ2 package.
func (w *TBZ2Writer) combineFiles(tarBz2Path string, xpakData []byte) error {
	// Open tar.bz2 file
	tarFile, err := os.Open(tarBz2Path)
	if err != nil {
		return err
	}
	defer func() { _ = tarFile.Close() }()

	// Create output file
	outFile, err := os.Create(w.OutputPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	// Copy tar.bz2
	if _, err := io.Copy(outFile, tarFile); err != nil {
		return fmt.Errorf("failed to copy tar.bz2: %w", err)
	}

	// Append XPAK
	if _, err := outFile.Write(xpakData); err != nil {
		return fmt.Errorf("failed to write XPAK: %w", err)
	}

	return nil
}
