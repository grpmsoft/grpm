package binpkg

import (
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"archive/tar"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// LoadTBZ2 loads a TBZ2 format binary package.
//
// TBZ2 format (legacy):
//   - Bzip2-compressed tar archive
//   - XPAK metadata appended to end
//   - Format: [tar.bz2 data][XPAK][XPAK footer]
func LoadTBZ2(path string) (*BinaryPackage, error) {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Parse package name from filename
	// Example: sys-libs/zlib-1.2.13.tbz2 -> zlib-1.2.13
	basename := filepath.Base(path)
	basename = strings.TrimSuffix(basename, ".tbz2")
	basename = strings.TrimSuffix(basename, ".tar.bz2")

	// Extract metadata
	metadata, err := GetTBZ2Metadata(path)
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
		Format:    FormatTBZ2,
		Path:      path,
		Size:      info.Size(),
		BuildInfo: metadata,
	}, nil
}

// GetTBZ2Metadata reads metadata from TBZ2 package without full extraction.
//
// TBZ2 metadata is stored in XPAK format appended to the tar.bz2 archive.
func GetTBZ2Metadata(path string) (*BuildMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Parse XPAK metadata
	xpak, err := ParseXPAK(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XPAK: %w", err)
	}

	// Extract metadata from XPAK entries
	return xpakToMetadata(xpak)
}

// xpakToMetadata converts XPAK entries to BuildMetadata.
func xpakToMetadata(xpak *XPAK) (*BuildMetadata, error) {
	metadata := &BuildMetadata{}

	// Extract common fields
	if buildTime, exists := xpak.GetString("BUILD_TIME"); exists {
		if ts, err := strconv.ParseInt(buildTime, 10, 64); err == nil {
			metadata.BuildDate = time.Unix(ts, 0)
		}
	}

	if buildHost, exists := xpak.GetString("CBUILD"); exists {
		metadata.BuildHost = buildHost
	}

	if cflags, exists := xpak.GetString("CFLAGS"); exists {
		metadata.CFLAGS = cflags
	}

	if cxxflags, exists := xpak.GetString("CXXFLAGS"); exists {
		metadata.CXXFLAGS = cxxflags
	}

	if ldflags, exists := xpak.GetString("LDFLAGS"); exists {
		metadata.LDFLAGS = ldflags
	}

	if useFlags, exists := xpak.GetString("USE"); exists {
		metadata.USE = strings.Fields(useFlags)
	}

	if features, exists := xpak.GetString("FEATURES"); exists {
		metadata.Features = strings.Fields(features)
	}

	if eapi, exists := xpak.GetString("EAPI"); exists {
		metadata.EAPI = eapi
	} else {
		metadata.EAPI = "0" // EAPI 0 for old packages
	}

	if repo, exists := xpak.GetString("repository"); exists {
		metadata.Repository = repo
	}

	if sizeStr, exists := xpak.GetString("SIZE"); exists {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			metadata.Size = size
		}
	}

	// If no BUILD_TIME, use current time
	if metadata.BuildDate.IsZero() {
		metadata.BuildDate = time.Now()
	}

	return metadata, nil
}

// ExtractTBZ2 extracts TBZ2 package contents to destination directory.
//
// Extracts the tar.bz2 archive, stopping before XPAK metadata.
func ExtractTBZ2(packagePath, destDir string) error {
	file, err := os.Open(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open package: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Find XPAK offset to avoid extracting metadata
	xpakOffset, err := findXPAKOffset(file)
	if err != nil {
		return fmt.Errorf("failed to find XPAK offset: %w", err)
	}

	// Seek back to start
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Create limited reader to stop at XPAK
	limitedReader := io.LimitReader(file, xpakOffset)

	// Decompress bzip2
	bzReader := bzip2.NewReader(limitedReader)

	// Extract tar archive
	tr := tar.NewReader(bzReader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
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

// findXPAKOffset finds the byte offset where XPAK metadata starts.
func findXPAKOffset(r io.ReadSeeker) (int64, error) {
	// XPAK footer is 20 bytes from end: index_len (4) + data_len (4) + xpak_len (4) + XPAKSTOP (8)
	const footerSize = 20

	// Seek to footer
	if _, err := r.Seek(-footerSize, io.SeekEnd); err != nil {
		return 0, fmt.Errorf("failed to seek to XPAK footer: %w", err)
	}

	// Read footer
	footer := make([]byte, footerSize)
	if _, err := io.ReadFull(r, footer); err != nil {
		return 0, fmt.Errorf("failed to read XPAK footer: %w", err)
	}

	// Parse xpak_len (bytes 8-12)
	xpakLen := int64(footer[8]) << 24
	xpakLen |= int64(footer[9]) << 16
	xpakLen |= int64(footer[10]) << 8
	xpakLen |= int64(footer[11])

	// Get file size
	fileSize, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	// XPAK offset = fileSize - xpakLen - footerSize
	xpakOffset := fileSize - xpakLen - footerSize

	return xpakOffset, nil
}
