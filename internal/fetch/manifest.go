package fetch

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrInvalidManifest indicates the Manifest file format is invalid.
var ErrInvalidManifest = errors.New("invalid manifest format")

// ErrManifestNotFound indicates the Manifest file does not exist.
var ErrManifestNotFound = errors.New("manifest file not found")

// EntryType represents the type of entry in a Manifest file.
type EntryType string

const (
	// EntryTypeDist represents a DIST entry (distribution file/tarball)
	EntryTypeDist EntryType = "DIST"

	// EntryTypeEbuild represents an EBUILD entry
	EntryTypeEbuild EntryType = "EBUILD"

	// EntryTypeAux represents an AUX entry (auxiliary file)
	EntryTypeAux EntryType = "AUX"

	// EntryTypeMisc represents a MISC entry (miscellaneous file)
	EntryTypeMisc EntryType = "MISC"
)

// ManifestEntry represents a single entry in a Manifest file.
//
// ManifestEntry is a Value Object containing file metadata and checksums.
type ManifestEntry struct {
	// Type is the entry type (DIST, EBUILD, AUX, MISC)
	Type EntryType

	// Filename is the name of the file
	Filename string

	// Size is the file size in bytes
	Size int64

	// Checksums contains the hash values
	Checksums Checksums
}

// Manifest represents a parsed Gentoo Manifest file.
//
// The Manifest file contains checksums for all files in a package directory,
// including source tarballs (DIST), ebuild files (EBUILD), patches (AUX),
// and other files (MISC).
//
// Format (GLEP 74):
//
//	DIST filename size BLAKE2B hash SHA512 hash
//	EBUILD filename size BLAKE2B hash SHA512 hash
//	AUX filename size BLAKE2B hash SHA512 hash
//
// Example:
//
//	DIST hello-2.10.tar.gz 725946 BLAKE2B abc123... SHA512 def456...
type Manifest struct {
	// Path is the path to the Manifest file
	Path string

	// Entries contains all parsed entries indexed by filename
	Entries map[string]ManifestEntry

	// DistFiles contains only DIST entries for quick access
	DistFiles []ManifestEntry
}

// ParseManifest parses a Manifest file and returns the parsed content.
//
// The function reads the file at the given path and parses each line
// according to the Gentoo Manifest format (GLEP 74).
//
// Returns ErrManifestNotFound if the file does not exist.
// Returns ErrInvalidManifest if the format is invalid.
func ParseManifest(path string) (*Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrManifestNotFound, path)
		}
		return nil, fmt.Errorf("opening manifest %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	manifest := &Manifest{
		Path:      path,
		Entries:   make(map[string]ManifestEntry),
		DistFiles: make([]ManifestEntry, 0),
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseManifestLine(line)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrInvalidManifest, lineNum, err)
		}

		manifest.Entries[entry.Filename] = entry

		if entry.Type == EntryTypeDist {
			manifest.DistFiles = append(manifest.DistFiles, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	return manifest, nil
}

// parseManifestLine parses a single line from a Manifest file.
//
// Format: TYPE filename size [ALGO hash]...
// Example: DIST hello-2.10.tar.gz 725946 BLAKE2B abc... SHA512 def...
func parseManifestLine(line string) (ManifestEntry, error) {
	fields := strings.Fields(line)

	// Minimum fields: TYPE filename size ALGO hash
	if len(fields) < 5 {
		return ManifestEntry{}, fmt.Errorf("insufficient fields: got %d, need at least 5", len(fields))
	}

	entryType := EntryType(fields[0])

	// Validate entry type
	switch entryType {
	case EntryTypeDist, EntryTypeEbuild, EntryTypeAux, EntryTypeMisc:
		// Valid
	default:
		return ManifestEntry{}, fmt.Errorf("unknown entry type: %s", fields[0])
	}

	filename := fields[1]

	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return ManifestEntry{}, fmt.Errorf("invalid size %q: %w", fields[2], err)
	}

	// Parse checksum pairs: ALGO hash ALGO hash ...
	checksums := Checksums{}
	for i := 3; i < len(fields)-1; i += 2 {
		algo := fields[i]
		hash := fields[i+1]

		switch algo {
		case "SHA256":
			checksums.SHA256 = hash
		case "SHA512":
			checksums.SHA512 = hash
		case "BLAKE2B":
			checksums.BLAKE2B = hash
		default:
			// Ignore unknown algorithms for forward compatibility
		}
	}

	return ManifestEntry{
		Type:      entryType,
		Filename:  filename,
		Size:      size,
		Checksums: checksums,
	}, nil
}

// GetDistfiles returns all DIST entries as Distfile objects ready for fetching.
//
// This converts ManifestEntry to Distfile Value Objects used by the Fetcher.
func (m *Manifest) GetDistfiles() []Distfile {
	distfiles := make([]Distfile, 0, len(m.DistFiles))

	for _, entry := range m.DistFiles {
		distfile := NewDistfile(entry.Filename, entry.Size, entry.Checksums)
		distfiles = append(distfiles, distfile)
	}

	return distfiles
}

// GetEntry returns a specific entry by filename.
//
// Returns the entry and true if found, or zero value and false if not found.
func (m *Manifest) GetEntry(filename string) (ManifestEntry, bool) {
	entry, ok := m.Entries[filename]
	return entry, ok
}

// HasDistfile checks if the Manifest contains a specific distfile.
func (m *Manifest) HasDistfile(filename string) bool {
	entry, ok := m.Entries[filename]
	return ok && entry.Type == EntryTypeDist
}

// ManifestPath returns the expected Manifest path for a package.
//
// Given a repository path and category/package name, returns the full path
// to the Manifest file.
//
// Example:
//
//	ManifestPath("/var/db/repos/gentoo", "app-misc/hello")
//	// Returns: /var/db/repos/gentoo/app-misc/hello/Manifest
func ManifestPath(repoPath, categoryPackage string) string {
	return filepath.Join(repoPath, categoryPackage, "Manifest")
}

// ParseManifestForPackage is a convenience function to parse a package's Manifest.
//
// It constructs the Manifest path from the repository and package name,
// then parses the file.
func ParseManifestForPackage(repoPath, categoryPackage string) (*Manifest, error) {
	path := ManifestPath(repoPath, categoryPackage)
	return ParseManifest(path)
}
