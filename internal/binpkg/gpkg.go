package binpkg

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// gpkgPackageInfo contains extended package information from GPKG metadata.
//
// This is an internal structure used during GPKG loading to hold both
// the standard BuildMetadata and Gentoo-specific package identifiers
// (CATEGORY, PF, SLOT) that don't belong in the BuildMetadata struct.
type gpkgPackageInfo struct {
	metadata *BuildMetadata
	category string // CATEGORY: e.g., "sys-libs"
	pf       string // PF: package-version[-revision], e.g., "zlib-1.2.13-r1"
	slot     string // SLOT: e.g., "0/1"
}

// getGPKGPackageInfo extracts extended package info from a GPKG file.
//
// This function reads the metadata.tar and extracts:
//   - BuildMetadata (standard fields)
//   - CATEGORY, PF, SLOT (Gentoo package identifiers)
func getGPKGPackageInfo(path string) (*gpkgPackageInfo, error) {
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
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		// Found metadata
		if header.Name == "metadata.tar" || strings.HasPrefix(header.Name, "metadata.tar.") {
			return parseGPKGExtendedMetadata(tr)
		}
	}

	// No metadata found - return default
	return &gpkgPackageInfo{
		metadata: &BuildMetadata{
			BuildDate: time.Now(),
			EAPI:      "8",
		},
	}, nil
}

// parseGPKGExtendedMetadata parses metadata.tar and extracts both BuildMetadata
// and Gentoo-specific package identifiers.
func parseGPKGExtendedMetadata(r io.Reader) (*gpkgPackageInfo, error) {
	tr := tar.NewReader(r)

	// Metadata fields collected from tar entries
	metaFiles := make(map[string]string)

	// Read all metadata files from the nested tar
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading metadata.tar: %w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Read file content
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("reading metadata file %s: %w", header.Name, err)
		}

		// Store with base name (strip any path prefix)
		fileName := filepath.Base(header.Name)
		metaFiles[fileName] = strings.TrimSpace(string(content))
	}

	// Build metadata from collected files
	metadata := &BuildMetadata{
		BuildDate:  time.Now(),
		EAPI:       "8",
		USE:        []string{},
		Features:   []string{},
		Repository: "gentoo",
	}

	// Parse BUILD_TIME (unix timestamp)
	if buildTime, ok := metaFiles["BUILD_TIME"]; ok {
		if timestamp, err := strconv.ParseInt(buildTime, 10, 64); err == nil {
			metadata.BuildDate = time.Unix(timestamp, 0)
		}
	}

	// Parse SIZE (installed size in bytes)
	if sizeStr, ok := metaFiles["SIZE"]; ok {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			metadata.Size = size
		}
	}

	// Parse simple string fields
	if cflags, ok := metaFiles["CFLAGS"]; ok {
		metadata.CFLAGS = cflags
	}
	if cxxflags, ok := metaFiles["CXXFLAGS"]; ok {
		metadata.CXXFLAGS = cxxflags
	}
	if ldflags, ok := metaFiles["LDFLAGS"]; ok {
		metadata.LDFLAGS = ldflags
	}
	if eapi, ok := metaFiles["EAPI"]; ok {
		metadata.EAPI = eapi
	}
	if repo, ok := metaFiles["REPOSITORY"]; ok {
		metadata.Repository = repo
	}
	if buildHost, ok := metaFiles["BUILD_HOST"]; ok {
		metadata.BuildHost = buildHost
	}

	// Parse USE flags (space-separated)
	if useFlags, ok := metaFiles["USE"]; ok && useFlags != "" {
		metadata.USE = strings.Fields(useFlags)
	}

	// Parse FEATURES (space-separated)
	if features, ok := metaFiles["FEATURES"]; ok && features != "" {
		metadata.Features = strings.Fields(features)
	}

	// Build extended package info
	pkgInfo := &gpkgPackageInfo{
		metadata: metadata,
		category: metaFiles["CATEGORY"],
		pf:       metaFiles["PF"],
		slot:     metaFiles["SLOT"],
	}

	return pkgInfo, nil
}

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

	// Parse package name from filename as fallback
	// Example: sys-libs/zlib-1.2.13.gpkg.tar -> zlib-1.2.13
	basename := filepath.Base(path)
	basename = strings.TrimSuffix(basename, ".gpkg.tar")

	// Extract extended package info (metadata + CATEGORY/PF/SLOT)
	pkgInfo, err := getGPKGPackageInfo(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read package info: %w", err)
	}

	// Determine package name and version from metadata or filename
	pkgName, pkgVersion := parseGPKGPackageInfo(pkgInfo, basename)

	// Parse slot from metadata
	slot := pkg.Slot{Name: "0"}
	if pkgInfo.slot != "" {
		slot = pkg.ParseSlot(pkgInfo.slot)
	}

	// Create package
	p := &pkg.Package{
		Name:    pkgName,
		Version: pkgVersion,
		Slot:    slot,
	}

	return &BinaryPackage{
		Package:   p,
		Format:    FormatGPKG,
		Path:      path,
		Size:      info.Size(),
		BuildInfo: pkgInfo.metadata,
	}, nil
}

// GetGPKGMetadata reads metadata from GPKG package without full extraction.
//
// GPKG metadata is stored in special tar members:
//   - gpkg-1: Format version marker
//   - metadata.tar: Package metadata (compressed tar)
//   - image.tar.{compression}: Installed files
func GetGPKGMetadata(path string) (*BuildMetadata, error) {
	pkgInfo, err := getGPKGPackageInfo(path)
	if err != nil {
		return nil, err
	}
	return pkgInfo.metadata, nil
}

// parseGPKGPackageInfo extracts package name and version from metadata or basename.
//
// Metadata parsing order (preferred):
//  1. CATEGORY + PF from metadata files
//  2. Fallback: parse from filename basename
//
// PF format: name-version[-revision], e.g., "zlib-1.2.13-r1" or "hello-2.10"
func parseGPKGPackageInfo(pkgInfo *gpkgPackageInfo, basename string) (name, version string) {
	// Prefer CATEGORY + PF from metadata if available
	if pkgInfo != nil && pkgInfo.pf != "" {
		pkgName, pkgVersion := splitPkgNameVersion(pkgInfo.pf)
		// If CATEGORY is available, prepend it to the package name
		if pkgInfo.category != "" {
			pkgName = pkgInfo.category + "/" + pkgName
		}
		return pkgName, pkgVersion
	}

	// Fallback: parse from basename
	// Basename format: "category--name-version" or "name-version"
	// Examples:
	//   - "sys-libs--zlib-1.2.13" (GPKG format with category)
	//   - "zlib-1.2.13" (simple format)

	// Try to extract from GPKG naming convention (category--name-version)
	category := ""
	if idx := strings.Index(basename, "--"); idx != -1 {
		// Has category prefix: "sys-libs--zlib-1.2.13"
		category = basename[:idx]
		basename = basename[idx+2:]
	}

	// Parse name-version using Gentoo convention
	pkgName, pkgVersion := splitPkgNameVersion(basename)

	// Prepend category if available
	if category != "" {
		pkgName = category + "/" + pkgName
	}

	return pkgName, pkgVersion
}

// splitPkgNameVersion splits "name-version" into name and version.
//
// Uses Gentoo convention: version starts at last hyphen followed by digit.
// Examples:
//   - "zlib-1.2.13" -> ("zlib", "1.2.13")
//   - "hello-2.10-r1" -> ("hello", "2.10-r1")
//   - "gtk+-3.24.38" -> ("gtk+", "3.24.38")
func splitPkgNameVersion(s string) (name, version string) {
	lastHyphen := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			// Check if next char is a digit (start of version)
			if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
				lastHyphen = i
				break
			}
		}
	}

	if lastHyphen == -1 {
		// No version found, return whole string as name
		return s, ""
	}

	return s[:lastHyphen], s[lastHyphen+1:]
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
		if errors.Is(err, io.EOF) {
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
		if errors.Is(err, io.EOF) {
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
