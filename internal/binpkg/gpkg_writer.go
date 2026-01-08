// Package binpkg implements GPKG format writing.
package binpkg

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GPKGWriter writes GPKG format packages.
//
// GPKG format (GLEP 78):
//   - Tar-based archive structure
//   - metadata.xml - package metadata
//   - Manifest - file checksums
//   - CONTENTS - file list
//   - image/ - package files
//
// Example:
//
//	writer := binpkg.NewGPKGWriter("/tmp/output.gpkg.tar")
//	writer.SetCompression(binpkg.CompressionZstd)
//	err := writer.Write(metadata, files)
type GPKGWriter struct {
	// OutputPath is the path for the output .gpkg.tar file
	OutputPath string

	// Compression is the compression type
	Compression CompressionType

	// Verbose enables detailed logging
	Verbose bool

	// tarWriter is the underlying tar writer
	tarWriter *tar.Writer

	// file is the output file
	file *os.File

	// compressor is the compression writer
	compressor io.WriteCloser
}

// NewGPKGWriter creates a new GPKG writer.
func NewGPKGWriter(outputPath string) *GPKGWriter {
	return &GPKGWriter{
		OutputPath:  outputPath,
		Compression: CompressionZstd,
		Verbose:     false,
	}
}

// SetCompression sets the compression type.
func (w *GPKGWriter) SetCompression(compression CompressionType) {
	w.Compression = compression
}

// Write writes a GPKG package.
//
// Parameters:
//   - metadata: Package build metadata
//   - stagingDir: Directory containing files to pack (image/)
//
// Returns:
//   - error: If writing fails
func (w *GPKGWriter) Write(metadata *BuildMetadata, stagingDir string) error {
	if w.Verbose {
		fmt.Printf("Creating GPKG package: %s\n", w.OutputPath)
	}

	// Create output file
	if err := w.create(); err != nil {
		return err
	}
	defer func() { _ = w.Close() }()

	// Write metadata files
	if err := w.writeMetadataXML(metadata); err != nil {
		return fmt.Errorf("failed to write metadata.xml: %w", err)
	}

	if err := w.writeManifest(stagingDir); err != nil {
		return fmt.Errorf("failed to write Manifest: %w", err)
	}

	if err := w.writeContents(stagingDir); err != nil {
		return fmt.Errorf("failed to write CONTENTS: %w", err)
	}

	// Write package files
	if err := w.writeImageDirectory(stagingDir); err != nil {
		return fmt.Errorf("failed to write image directory: %w", err)
	}

	if w.Verbose {
		fmt.Printf("GPKG package created successfully: %s\n", w.OutputPath)
	}

	return nil
}

// create creates the output file and initializes writers.
func (w *GPKGWriter) create() error {
	// Create output directory if needed
	dir := filepath.Dir(w.OutputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(w.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	w.file = file

	// TODO: Add compression layer based on w.Compression
	// For now, write uncompressed tar
	w.tarWriter = tar.NewWriter(file)

	return nil
}

// Close closes the writer.
func (w *GPKGWriter) Close() error {
	var firstErr error

	// Close tar writer
	if w.tarWriter != nil {
		if err := w.tarWriter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Close compressor if present
	if w.compressor != nil {
		if err := w.compressor.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Close file
	if w.file != nil {
		if err := w.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// writeMetadataXML writes metadata.xml file.
func (w *GPKGWriter) writeMetadataXML(metadata *BuildMetadata) error {
	// Generate metadata XML
	xml := w.generateMetadataXML(metadata)

	// Write to tar
	return w.writeStringToTar("metadata.xml", xml)
}

// generateMetadataXML generates metadata.xml content.
func (w *GPKGWriter) generateMetadataXML(metadata *BuildMetadata) string {
	// Simplified metadata XML (full implementation would use encoding/xml)
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<pkgmetadata>
`
	if metadata.BuildHost != "" {
		xml += fmt.Sprintf("  <buildhost>%s</buildhost>\n", metadata.BuildHost)
	}
	if !metadata.BuildDate.IsZero() {
		xml += fmt.Sprintf("  <builddate>%s</builddate>\n", metadata.BuildDate.Format(time.RFC3339))
	}
	if metadata.CFLAGS != "" {
		xml += fmt.Sprintf("  <cflags>%s</cflags>\n", metadata.CFLAGS)
	}
	if len(metadata.USE) > 0 {
		xml += fmt.Sprintf("  <use>%s</use>\n", strings.Join(metadata.USE, " "))
	}
	xml += `</pkgmetadata>
`
	return xml
}

// writeManifest writes Manifest file with checksums.
func (w *GPKGWriter) writeManifest(stagingDir string) error {
	manifest := "MANIFEST 1.0\n"

	// Walk staging directory and compute checksums
	err := filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Compute SHA256
		checksum, err := computeSHA256(path)
		if err != nil {
			return fmt.Errorf("failed to compute checksum for %s: %w", path, err)
		}

		// Relative path
		relPath, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}

		// Add to manifest
		manifest += fmt.Sprintf("DATA %s %d SHA256 %s\n", relPath, info.Size(), checksum)

		return nil
	})

	if err != nil {
		return err
	}

	return w.writeStringToTar("Manifest", manifest)
}

// writeContents writes CONTENTS file.
func (w *GPKGWriter) writeContents(stagingDir string) error {
	contents := ""

	// Walk staging directory
	err := filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
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

		// Absolute path (as it would be installed)
		absPath := "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			contents += fmt.Sprintf("dir %s\n", absPath)
		} else if info.Mode()&os.ModeSymlink != 0 {
			// Symlink
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			contents += fmt.Sprintf("sym %s -> %s\n", absPath, target)
		} else {
			// Regular file
			checksum, err := computeSHA256(path)
			if err != nil {
				return err
			}
			contents += fmt.Sprintf("obj %s %s %d %d\n", absPath, checksum, info.Mode().Perm(), info.ModTime().Unix())
		}

		return nil
	})

	if err != nil {
		return err
	}

	return w.writeStringToTar("CONTENTS", contents)
}

// writeImageDirectory writes the image/ directory to tar.
func (w *GPKGWriter) writeImageDirectory(stagingDir string) error {
	// Walk staging directory and add all files to tar
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

		// Tar path (under image/)
		tarPath := filepath.Join("image", relPath)

		if info.IsDir() {
			// Add directory
			return w.addDirectoryToTar(tarPath, info)
		} else if info.Mode()&os.ModeSymlink != 0 {
			// Add symlink
			return w.addSymlinkToTar(tarPath, path)
		} else {
			// Add regular file
			return w.addFileToTar(tarPath, path, info)
		}
	})
}

// addDirectoryToTar adds a directory entry to tar.
func (w *GPKGWriter) addDirectoryToTar(tarPath string, info os.FileInfo) error {
	header := &tar.Header{
		Name:     tarPath + "/",
		Mode:     int64(info.Mode().Perm()),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeDir,
	}

	return w.tarWriter.WriteHeader(header)
}

// addSymlinkToTar adds a symlink entry to tar.
func (w *GPKGWriter) addSymlinkToTar(tarPath, srcPath string) error {
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

	return w.tarWriter.WriteHeader(header)
}

// addFileToTar adds a regular file to tar.
func (w *GPKGWriter) addFileToTar(tarPath, srcPath string, info os.FileInfo) error {
	// Open source file
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", srcPath, err)
	}
	defer func() { _ = file.Close() }()

	// Create tar header
	header := &tar.Header{
		Name:    tarPath,
		Size:    info.Size(),
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),
	}

	// Write header
	if err := w.tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file data
	if _, err := io.Copy(w.tarWriter, file); err != nil {
		return fmt.Errorf("failed to write file data: %w", err)
	}

	return nil
}

// writeStringToTar writes a string as a file in tar.
func (w *GPKGWriter) writeStringToTar(name, content string) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(content)),
		Mode:    0644,
		ModTime: time.Now(),
	}

	if err := w.tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := w.tarWriter.Write([]byte(content)); err != nil {
		return err
	}

	return nil
}

// computeSHA256 computes SHA256 checksum of a file.
func computeSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
