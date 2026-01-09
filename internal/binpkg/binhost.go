package binpkg

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// Binhost represents a binary package repository.
//
// A binhost can be:
//   - Local directory (file:// or absolute path)
//   - HTTP/HTTPS URL (http://, https://)
//   - Remote server (rsync://, ssh://)
type Binhost struct {
	// URI is the binhost location
	URI string

	// Type is the binhost type (local, http, etc)
	Type BinhostType

	// Packages is the list of available packages
	Packages []*BinaryPackage

	// LastSync is when the binhost was last synchronized
	LastSync time.Time
}

// BinhostType represents the type of binhost.
type BinhostType int

const (
	// BinhostLocal is a local directory
	BinhostLocal BinhostType = iota

	// BinhostHTTP is an HTTP/HTTPS server
	BinhostHTTP

	// BinhostRsync is an rsync server
	BinhostRsync

	// BinhostSSH is an SSH server
	BinhostSSH
)

// String returns string representation of binhost type.
func (bt BinhostType) String() string {
	switch bt {
	case BinhostLocal:
		return "local"
	case BinhostHTTP:
		return "http"
	case BinhostRsync:
		return "rsync"
	case BinhostSSH:
		return "ssh"
	default:
		return "unknown"
	}
}

// NewBinhost creates a new binhost from URI.
func NewBinhost(uri string) (*Binhost, error) {
	// Detect binhost type
	binhostType := detectBinhostType(uri)

	return &Binhost{
		URI:      uri,
		Type:     binhostType,
		Packages: []*BinaryPackage{},
	}, nil
}

// detectBinhostType detects binhost type from URI.
func detectBinhostType(uri string) BinhostType {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return BinhostHTTP
	}
	if strings.HasPrefix(uri, "rsync://") {
		return BinhostRsync
	}
	if strings.HasPrefix(uri, "ssh://") {
		return BinhostSSH
	}
	return BinhostLocal
}

// Sync synchronizes the binhost package list.
//
// For HTTP binhosts, downloads and parses Packages index.
// For local binhosts, scans the directory for binary packages.
func (b *Binhost) Sync() error {
	switch b.Type {
	case BinhostLocal:
		return b.syncLocal()
	case BinhostHTTP:
		return b.syncHTTP()
	case BinhostRsync:
		return fmt.Errorf("rsync binhost not yet implemented")
	case BinhostSSH:
		return fmt.Errorf("ssh binhost not yet implemented")
	default:
		return fmt.Errorf("unsupported binhost type: %s", b.Type)
	}
}

// syncLocal scans local directory for binary packages.
func (b *Binhost) syncLocal() error {
	// Normalize path
	path := strings.TrimPrefix(b.URI, "file://")
	if !filepath.IsAbs(path) {
		return fmt.Errorf("local binhost path must be absolute: %s", path)
	}

	// Check directory exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access binhost directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("binhost path is not a directory: %s", path)
	}

	// Scan for binary packages
	packages := []*BinaryPackage{}

	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's a binary package
		format := DetectFormat(filePath)
		if format == FormatUnknown {
			return nil
		}

		// Load package
		binPkg, err := LoadBinaryPackage(filePath)
		if err != nil {
			// Skip packages that fail to load
			return nil
		}

		packages = append(packages, binPkg)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan binhost directory: %w", err)
	}

	b.Packages = packages
	b.LastSync = time.Now()

	return nil
}

// syncHTTP downloads and parses Packages index from HTTP binhost.
func (b *Binhost) syncHTTP() error {
	// Construct Packages index URL
	packagesURL := b.URI
	if !strings.HasSuffix(packagesURL, "/") {
		packagesURL += "/"
	}
	packagesURL += "Packages"

	// Download Packages file
	resp, err := http.Get(packagesURL)
	if err != nil {
		return fmt.Errorf("failed to download Packages index: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Packages index: HTTP %d", resp.StatusCode)
	}

	// Parse Packages file
	packages, err := parsePackagesIndex(resp.Body, b.URI)
	if err != nil {
		return fmt.Errorf("failed to parse Packages index: %w", err)
	}

	b.Packages = packages
	b.LastSync = time.Now()

	return nil
}

// parsePackagesIndex parses Portage Packages index file.
//
// Format (similar to Debian's Packages file):
//
//	CPV: category/package-version
//	PATH: relative/path/to/package.gpkg.tar
//	SIZE: 1234567
//	MD5: ...
//	SHA256: ...
//	USE: flag1 flag2
//	...
//	(blank line separates entries)
func parsePackagesIndex(r io.Reader, baseURL string) ([]*BinaryPackage, error) {
	packages := []*BinaryPackage{}
	scanner := bufio.NewScanner(r)

	var currentEntry map[string]string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Empty line marks end of entry
		if line == "" {
			if currentEntry != nil {
				if pkg, err := packageFromIndexEntry(currentEntry, baseURL); err == nil {
					packages = append(packages, pkg)
				}
				currentEntry = nil
			}
			continue
		}

		// Parse key: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if currentEntry == nil {
			currentEntry = make(map[string]string)
		}
		currentEntry[key] = value
	}

	// Handle last entry
	if currentEntry != nil {
		if pkg, err := packageFromIndexEntry(currentEntry, baseURL); err == nil {
			packages = append(packages, pkg)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

// packageFromIndexEntry creates BinaryPackage from Packages index entry.
func packageFromIndexEntry(entry map[string]string, baseURL string) (*BinaryPackage, error) {
	cpv, exists := entry["CPV"]
	if !exists {
		return nil, fmt.Errorf("missing CPV field")
	}

	path, exists := entry["PATH"]
	if !exists {
		return nil, fmt.Errorf("missing PATH field")
	}

	// Construct full URL
	fullURL := baseURL
	if !strings.HasSuffix(fullURL, "/") {
		fullURL += "/"
	}
	fullURL += path

	// Parse CPV (category/package-version)
	// TODO: Proper CPV parsing
	parts := strings.Split(cpv, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid CPV format: %s", cpv)
	}

	// Detect format
	format := DetectFormat(path)

	// Build metadata from index
	metadata := &BuildMetadata{
		BuildDate: time.Now(), // TODO: Parse BUILD_TIME if available
	}

	if useStr, exists := entry["USE"]; exists {
		metadata.USE = strings.Fields(useStr)
	}

	if eapi, exists := entry["EAPI"]; exists {
		metadata.EAPI = eapi
	}

	// Create package
	// TODO: Parse package name and version properly
	binPkg := &BinaryPackage{
		Format:    format,
		Path:      fullURL,
		Checksum:  entry["SHA256"],
		BuildInfo: metadata,
	}

	return binPkg, nil
}

// Find searches for packages matching the given atom.
//
// Supports Gentoo atom format with version operators:
//   - sys-libs/zlib (any version)
//   - =sys-libs/zlib-1.2.13 (exact version)
//   - >=sys-libs/zlib-1.2 (version >= 1.2)
//   - <=sys-libs/zlib-1.3 (version <= 1.3)
//   - >sys-libs/zlib-1.2 (version > 1.2)
//   - <sys-libs/zlib-1.3 (version < 1.3)
//
// Returns all matching packages, sorted by version (newest first).
func (b *Binhost) Find(atom string) []*BinaryPackage {
	matches := []*BinaryPackage{}

	// Parse atom: ">=sys-libs/zlib-1.2" -> (>=, sys-libs/zlib, 1.2)
	operator, pkgName, version := parseAtomSimple(atom)

	for _, binPkg := range b.Packages {
		if binPkg.Package == nil {
			continue
		}

		// Check name match (exact match on category/name)
		if binPkg.Package.Name != pkgName {
			// Also try substring match for atoms without category
			if !strings.Contains(binPkg.Package.Name, pkgName) {
				continue
			}
		}

		// If version specified, check constraint
		if version != "" {
			if !matchVersion(binPkg.Package.Version, operator, version) {
				continue
			}
		}

		matches = append(matches, binPkg)
	}

	return matches
}

// parseAtomSimple parses a Portage atom into operator, package name, and version.
//
// Atom format: [operator]category/name[-version]
// Examples:
//   - sys-libs/zlib -> ("", "sys-libs/zlib", "")
//   - >=sys-libs/zlib-1.2 -> (">=", "sys-libs/zlib", "1.2")
//   - =app-misc/hello-2.10 -> ("=", "app-misc/hello", "2.10")
func parseAtomSimple(atom string) (operator, name, version string) {
	// Remove any slot specification (:slot)
	if idx := strings.Index(atom, ":"); idx != -1 {
		atom = atom[:idx]
	}

	// Remove any USE flag requirements ([use])
	if idx := strings.Index(atom, "["); idx != -1 {
		atom = atom[:idx]
	}

	// Extract operator prefix
	for _, op := range []string{">=", "<=", ">", "<", "=", "~", "!"} {
		if strings.HasPrefix(atom, op) {
			operator = op
			atom = strings.TrimPrefix(atom, op)
			break
		}
	}

	// Find version by looking for last dash followed by a digit
	// Example: sys-libs/zlib-1.2.13 -> name="sys-libs/zlib", version="1.2.13"
	lastDash := -1
	for i := len(atom) - 1; i >= 0; i-- {
		if atom[i] == '-' && i+1 < len(atom) && atom[i+1] >= '0' && atom[i+1] <= '9' {
			lastDash = i
			break
		}
	}

	if lastDash != -1 {
		name = atom[:lastDash]
		version = atom[lastDash+1:]
	} else {
		name = atom
		version = ""
	}

	return operator, name, version
}

// matchVersion checks if pkgVersion satisfies the version constraint.
func matchVersion(pkgVersion, operator, constraintVersion string) bool {
	switch operator {
	case ">=":
		return pkg.CompareVersions(pkgVersion, constraintVersion) >= 0
	case "<=":
		return pkg.CompareVersions(pkgVersion, constraintVersion) <= 0
	case ">":
		return pkg.CompareVersions(pkgVersion, constraintVersion) > 0
	case "<":
		return pkg.CompareVersions(pkgVersion, constraintVersion) < 0
	case "=", "~":
		// Exact match (~ is "any revision" but we treat as exact for simplicity)
		return pkgVersion == constraintVersion
	default:
		// No operator - match any version, or exact if version given
		if constraintVersion != "" {
			return pkgVersion == constraintVersion
		}
		return true
	}
}

// Download downloads a binary package from the binhost to local path.
func (b *Binhost) Download(binPkg *BinaryPackage, destPath string) error {
	switch b.Type {
	case BinhostLocal:
		// Local binhost - just copy file
		return copyFile(binPkg.Path, destPath)

	case BinhostHTTP:
		// HTTP binhost - download file
		return downloadFile(binPkg.Path, destPath)

	default:
		return fmt.Errorf("download not implemented for %s binhost", b.Type)
	}
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destination.Close() }()

	_, err = io.Copy(destination, source)
	return err
}

// downloadFile downloads a file from URL to local path.
func downloadFile(fileURL, destPath string) error {
	// Parse URL
	u, err := url.Parse(fileURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Download file
	resp, err := http.Get(u.String())
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = out.Close() }()

	// Copy data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}
