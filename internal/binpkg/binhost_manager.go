// Package binpkg implements local binhost management.
package binpkg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BinhostManager manages a local binary package repository (binhost).
//
// Directory structure:
//
//	/var/cache/binpkgs/
//	├── Packages         # Index file
//	├── category/
//	│   ├── package-1.0.0.gpkg.tar
//	│   └── package-1.0.0.gpkg.tar.sig
//	└── ...
//
// Example:
//
//	manager := binpkg.NewBinhostManager("/var/cache/binpkgs")
//	err := manager.Add(binaryPackage)
//	err = manager.GenerateIndex()
type BinhostManager struct {
	// Root is the binhost root directory
	Root string

	// Packages is the list of packages in binhost
	Packages []*BinaryPackage

	// Verbose enables detailed logging
	Verbose bool

	// IndexFormat is the Packages index format (portage or gentoo)
	IndexFormat string
}

// NewBinhostManager creates a new binhost manager.
func NewBinhostManager(root string) *BinhostManager {
	return &BinhostManager{
		Root:        root,
		Packages:    make([]*BinaryPackage, 0),
		Verbose:     false,
		IndexFormat: "portage", // Default to Portage format
	}
}

// Initialize initializes the binhost directory structure.
func (m *BinhostManager) Initialize() error {
	if m.Verbose {
		fmt.Printf("Initializing binhost at: %s\n", m.Root)
	}

	// Create root directory
	if err := os.MkdirAll(m.Root, 0755); err != nil {
		return fmt.Errorf("failed to create binhost root: %w", err)
	}

	return nil
}

// Add adds a binary package to the binhost.
//
// Steps:
//  1. Extract package metadata
//  2. Create category directory
//  3. Copy package file to binhost
//  4. Copy signature if present
//  5. Add to package list
func (m *BinhostManager) Add(pkg *BinaryPackage) error {
	if pkg == nil || pkg.Package == nil {
		return fmt.Errorf("package cannot be nil")
	}

	if m.Verbose {
		fmt.Printf("Adding package: %s-%s\n", pkg.Package.Name, pkg.Package.Version)
	}

	// Extract category from package name (e.g., "sys-libs/zlib" -> "sys-libs")
	parts := strings.SplitN(pkg.Package.Name, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid package name format: %s", pkg.Package.Name)
	}
	category := parts[0]
	packageName := parts[1]

	// Create category directory
	categoryDir := filepath.Join(m.Root, category)
	if err := os.MkdirAll(categoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create category directory: %w", err)
	}

	// Generate destination filename
	destFilename := fmt.Sprintf("%s-%s%s", packageName, pkg.Package.Version, pkg.Format.Extension())
	destPath := filepath.Join(categoryDir, destFilename)

	// Copy package file
	if err := copyFile(pkg.Path, destPath); err != nil {
		return fmt.Errorf("failed to copy package file: %w", err)
	}

	// Copy signature if present
	if pkg.Signature != nil {
		sigSrcPath := pkg.Path + ".sig"
		sigDestPath := destPath + ".sig"

		// Check if signature file exists
		if _, err := os.Stat(sigSrcPath); err == nil {
			if err := copyFile(sigSrcPath, sigDestPath); err != nil {
				// Non-critical error
				if m.Verbose {
					fmt.Printf("Warning: failed to copy signature: %v\n", err)
				}
			}
		}
	}

	// Update package path to binhost location
	pkg.Path = destPath

	// Add to package list
	m.Packages = append(m.Packages, pkg)

	if m.Verbose {
		fmt.Printf("Package added successfully: %s\n", destPath)
	}

	return nil
}

// Remove removes a package from the binhost.
func (m *BinhostManager) Remove(category, name, version string) error {
	packageAtom := fmt.Sprintf("%s/%s-%s", category, name, version)

	if m.Verbose {
		fmt.Printf("Removing package: %s\n", packageAtom)
	}

	// Find package in list
	index := -1
	var pkg *BinaryPackage
	for i, p := range m.Packages {
		if p.Package.Name == fmt.Sprintf("%s/%s", category, name) && p.Package.Version == version {
			index = i
			pkg = p
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("package not found: %s", packageAtom)
	}

	// Remove package file
	if err := os.Remove(pkg.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove package file: %w", err)
	}

	// Remove signature file if exists
	sigPath := pkg.Path + ".sig"
	if err := os.Remove(sigPath); err != nil && !os.IsNotExist(err) {
		if m.Verbose {
			fmt.Printf("Warning: failed to remove signature: %v\n", err)
		}
	}

	// Remove from package list
	m.Packages = append(m.Packages[:index], m.Packages[index+1:]...)

	if m.Verbose {
		fmt.Printf("Package removed successfully: %s\n", packageAtom)
	}

	return nil
}

// List lists all packages in the binhost.
func (m *BinhostManager) List() []*BinaryPackage {
	return m.Packages
}

// Scan scans the binhost directory and loads all packages.
func (m *BinhostManager) Scan() error {
	if m.Verbose {
		fmt.Printf("Scanning binhost: %s\n", m.Root)
	}

	m.Packages = make([]*BinaryPackage, 0)

	// Walk binhost directory
	err := filepath.Walk(m.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is a binary package
		format := DetectFormat(path)
		if format == FormatUnknown {
			return nil
		}

		// Load package
		pkg, err := LoadBinaryPackage(path)
		if err != nil {
			if m.Verbose {
				fmt.Printf("Warning: failed to load package %s: %v\n", path, err)
			}
			return nil // Continue scanning
		}

		m.Packages = append(m.Packages, pkg)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan binhost: %w", err)
	}

	if m.Verbose {
		fmt.Printf("Found %d packages\n", len(m.Packages))
	}

	return nil
}

// GenerateIndex generates the Packages index file.
//
// Format:
//
//	PACKAGES: {package-count}
//	TIMESTAMP: {unix-timestamp}
//
//	CPV: {category/package-version}
//	SIZE: {bytes}
//	SHA256: {checksum}
//	USE: {use-flags}
//	REPO: gentoo
//	BUILD_TIME: {unix-timestamp}
//	PATH: {category/package-version.gpkg.tar}
func (m *BinhostManager) GenerateIndex() error {
	if m.Verbose {
		fmt.Printf("Generating Packages index\n")
	}

	indexPath := filepath.Join(m.Root, "Packages")

	// Open index file
	file, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Write header
	_, _ = fmt.Fprintf(file, "PACKAGES: %d\n", len(m.Packages))
	_, _ = fmt.Fprintf(file, "TIMESTAMP: %d\n", time.Now().Unix())
	_, _ = fmt.Fprintln(file)

	// Sort packages by CPV
	sorted := make([]*BinaryPackage, len(m.Packages))
	copy(sorted, m.Packages)
	sort.Slice(sorted, func(i, j int) bool {
		cpvI := fmt.Sprintf("%s-%s", sorted[i].Package.Name, sorted[i].Package.Version)
		cpvJ := fmt.Sprintf("%s-%s", sorted[j].Package.Name, sorted[j].Package.Version)
		return cpvI < cpvJ
	})

	// Write package entries
	for _, pkg := range sorted {
		if err := m.writePackageEntry(file, pkg); err != nil {
			return fmt.Errorf("failed to write package entry: %w", err)
		}
	}

	if m.Verbose {
		fmt.Printf("Index generated: %s\n", indexPath)
	}

	return nil
}

// writePackageEntry writes a single package entry to the index.
func (m *BinhostManager) writePackageEntry(file *os.File, pkg *BinaryPackage) error {
	// CPV (Category/Package-Version)
	cpv := fmt.Sprintf("%s-%s", pkg.Package.Name, pkg.Package.Version)
	_, _ = fmt.Fprintf(file, "CPV: %s\n", cpv)

	// Size
	if pkg.Size > 0 {
		_, _ = fmt.Fprintf(file, "SIZE: %d\n", pkg.Size)
	}

	// Checksum
	if pkg.Checksum != "" {
		_, _ = fmt.Fprintf(file, "SHA256: %s\n", pkg.Checksum)
	}

	// USE flags
	if pkg.BuildInfo != nil && len(pkg.BuildInfo.USE) > 0 {
		_, _ = fmt.Fprintf(file, "USE: %s\n", strings.Join(pkg.BuildInfo.USE, " "))
	}

	// Repository
	if pkg.BuildInfo != nil && pkg.BuildInfo.Repository != "" {
		_, _ = fmt.Fprintf(file, "REPO: %s\n", pkg.BuildInfo.Repository)
	} else {
		_, _ = fmt.Fprintln(file, "REPO: gentoo")
	}

	// Build time
	if pkg.BuildInfo != nil && !pkg.BuildInfo.BuildDate.IsZero() {
		_, _ = fmt.Fprintf(file, "BUILD_TIME: %d\n", pkg.BuildInfo.BuildDate.Unix())
	}

	// Path (relative to binhost root)
	relPath, err := filepath.Rel(m.Root, pkg.Path)
	if err != nil {
		relPath = filepath.Base(pkg.Path)
	}
	_, _ = fmt.Fprintf(file, "PATH: %s\n", filepath.ToSlash(relPath))

	// Empty line separator
	_, _ = fmt.Fprintln(file)

	return nil
}

// Clean removes orphaned files and empty directories.
func (m *BinhostManager) Clean() error {
	if m.Verbose {
		fmt.Printf("Cleaning binhost\n")
	}

	// Build set of valid paths
	validPaths := make(map[string]bool)
	for _, pkg := range m.Packages {
		validPaths[pkg.Path] = true
		validPaths[pkg.Path+".sig"] = true
	}

	// Walk binhost and remove orphaned files
	err := filepath.Walk(m.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip root directory
		if path == m.Root {
			return nil
		}

		// Skip directories (will be cleaned up later)
		if info.IsDir() {
			return nil
		}

		// Skip Packages index
		if filepath.Base(path) == "Packages" {
			return nil
		}

		// Check if path is valid
		if !validPaths[path] {
			// Check if it's a package file
			format := DetectFormat(path)
			if format != FormatUnknown || strings.HasSuffix(path, ".sig") {
				if m.Verbose {
					fmt.Printf("Removing orphaned file: %s\n", path)
				}
				if err := os.Remove(path); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to clean binhost: %w", err)
	}

	// Remove empty directories
	_ = filepath.Walk(m.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == m.Root {
			return err
		}

		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				if m.Verbose {
					fmt.Printf("Removing empty directory: %s\n", path)
				}
				return os.Remove(path)
			}
		}

		return nil
	})

	return nil
}

// copyFile uses the existing copyFile function from binhost.go
