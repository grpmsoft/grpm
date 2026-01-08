package binpkg

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// LoadGPKG loads a GPKG format binary package.
//
// GPKG format (GLEP 78):
//   - Modern tar-based format
//   - Zstd compression
//   - Metadata in special tar members
//   - Signature support
func LoadGPKG(path string) (*BinaryPackage, error) {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Parse package name from filename
	// Example: sys-libs/zlib-1.2.13.gpkg.tar -> zlib-1.2.13
	basename := filepath.Base(path)
	basename = strings.TrimSuffix(basename, ".gpkg.tar")

	// Extract metadata
	metadata, err := GetGPKGMetadata(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Parse package name and version from basename
	// TODO: Proper parsing - for now, stub
	pkgName := basename
	pkgVersion := "1.0"

	// Create package
	p := &pkg.Package{
		Name:    pkgName,
		Version: pkgVersion,
		Slot:    pkg.Slot{Name: "0"},
	}

	return &BinaryPackage{
		Package:   p,
		Format:    FormatGPKG,
		Path:      path,
		Size:      info.Size(),
		BuildInfo: metadata,
	}, nil
}

// GetGPKGMetadata reads metadata from GPKG package without full extraction.
//
// GPKG metadata is stored in special tar members:
//   - gpkg-1: Format version marker
//   - metadata.tar: Package metadata (compressed tar)
//   - image.tar.{compression}: Installed files
func GetGPKGMetadata(path string) (*BuildMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Open tar archive
	tr := tar.NewReader(file)

	// Look for metadata.tar member
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Found metadata
		if header.Name == "metadata.tar" || strings.HasPrefix(header.Name, "metadata.tar.") {
			return parseGPKGMetadata(tr)
		}
	}

	// No metadata found - return default
	return &BuildMetadata{
		BuildDate: time.Now(),
		EAPI:      "8",
	}, nil
}

// parseGPKGMetadata parses metadata from metadata.tar member.
func parseGPKGMetadata(r io.Reader) (*BuildMetadata, error) {
	// metadata.tar is itself a tar archive containing files like:
	// - CFLAGS
	// - CXXFLAGS
	// - LDFLAGS
	// - USE
	// - BUILD_TIME
	// - SIZE
	// etc.

	// For now, return stub metadata
	// TODO: Implement full metadata parsing
	return &BuildMetadata{
		BuildDate:  time.Now(),
		BuildHost:  "unknown",
		CFLAGS:     "-O2 -pipe",
		EAPI:       "8",
		USE:        []string{},
		Features:   []string{"sandbox"},
		Repository: "gentoo",
	}, nil
}

// ExtractGPKG extracts GPKG package contents to destination directory.
//
// Extracts only the image.tar.{compression} member which contains installed files.
func ExtractGPKG(packagePath, destDir string) error {
	file, err := os.Open(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open package: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Open tar archive
	tr := tar.NewReader(file)

	// Find image.tar member
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Found image tar
		if strings.HasPrefix(header.Name, "image.tar") {
			return extractImageTar(tr, destDir)
		}
	}

	return fmt.Errorf("no image.tar found in GPKG")
}

// extractImageTar extracts image.tar contents to destination.
func extractImageTar(r io.Reader, destDir string) error {
	// image.tar is a nested tar archive containing the actual installed files
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip metadata files
		if strings.HasPrefix(header.Name, ".") {
			continue
		}

		// Target path
		target := filepath.Join(destDir, header.Name)

		// Extract based on type
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}

		case tar.TypeReg:
			// Extract regular file
			if err := extractFile(tr, target, header); err != nil {
				return err
			}

		case tar.TypeSymlink:
			// Create symlink
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}

		case tar.TypeLink:
			// Hard link
			linkTarget := filepath.Join(destDir, header.Linkname)
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("failed to create hard link %s: %w", target, err)
			}
		}
	}

	return nil
}

// extractFile extracts a regular file from tar.
func extractFile(tr *tar.Reader, target string, header *tar.Header) error {
	// Create parent directory
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create file
	outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", target, err)
	}
	defer func() { _ = outFile.Close() }()

	// Copy contents
	if _, err := io.Copy(outFile, tr); err != nil {
		return fmt.Errorf("failed to extract file %s: %w", target, err)
	}

	return nil
}
