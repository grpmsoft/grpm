// Package state implements system state tracking for installed packages.
//
// This module provides Gentoo VarDB (/var/db/pkg) compatible package database
// for tracking installed packages, their files, and metadata.
//
// VarDB Structure:
//
//	/var/db/pkg/
//	├── category/
//	│   └── package-version/
//	│       ├── CONTENTS        # File list
//	│       ├── RDEPEND         # Runtime dependencies
//	│       ├── DEPEND          # Build dependencies
//	│       ├── USE             # USE flags used
//	│       ├── CFLAGS          # Compilation flags
//	│       ├── SIZE            # Package size
//	│       └── BUILD_TIME      # Installation timestamp
//
// Example:
//
//	db := state.NewPackageDatabase("/var/db/pkg")
//	err := db.Add(installedPkg)
//	pkg, err := db.Get("sys-libs/zlib-1.2.13")
//	owners := db.FindFileOwners("/usr/bin/gcc")
package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// FileType represents the type of an installed file.
type FileType int

const (
	// FileTypeRegular is a regular file.
	FileTypeRegular FileType = iota
	// FileTypeDirectory is a directory.
	FileTypeDirectory
	// FileTypeSymlink is a symbolic link.
	FileTypeSymlink
	// FileTypeHardlink is a hard link.
	FileTypeHardlink
)

// String returns the string representation of FileType.
func (ft FileType) String() string {
	switch ft {
	case FileTypeRegular:
		return "obj"
	case FileTypeDirectory:
		return "dir"
	case FileTypeSymlink:
		return "sym"
	case FileTypeHardlink:
		return "hardlink"
	default:
		return "unknown"
	}
}

// PackageDatabase tracks installed packages.
//
// This database is thread-safe and compatible with Gentoo's VarDB format.
// All operations are protected by a read-write mutex.
type PackageDatabase struct {
	// Root is the VarDB root directory (usually /var/db/pkg).
	Root string

	// packages maps package atoms to installed package metadata.
	// Key format: "category/package-version"
	packages map[string]*InstalledPackage

	// fileIndex maps file paths to package atoms for quick owner lookups.
	// Key: absolute file path, Value: package atom
	fileIndex map[string]string

	// mu protects concurrent access to the database.
	mu sync.RWMutex
}

// InstalledPackage represents a package installed on the system.
type InstalledPackage struct {
	// Package is the package metadata.
	Package *pkg.Package

	// InstallTime is when the package was installed.
	InstallTime time.Time

	// BuildInfo contains build-time information.
	BuildInfo BuildInfo

	// Files is the list of installed files.
	Files []InstalledFile

	// USE contains the USE flags used during installation.
	USE []string

	// CFLAGS contains compilation flags used.
	CFLAGS string

	// CXXFLAGS contains C++ compilation flags used.
	CXXFLAGS string

	// LDFLAGS contains linker flags used.
	LDFLAGS string

	// Size is the total installed size in bytes.
	Size int64
}

// BuildInfo contains build-time metadata.
type BuildInfo struct {
	// Host is the build host name.
	Host string

	// BuildDate is when the package was built.
	BuildDate time.Time

	// CFLAGS are the C compilation flags.
	CFLAGS string

	// CXXFLAGS are the C++ compilation flags.
	CXXFLAGS string

	// LDFLAGS are the linker flags.
	LDFLAGS string

	// Features are Portage features enabled during build.
	// Example: ["sandbox", "ccache", "parallel-fetch"]
	Features []string

	// EAPI is the EAPI version used.
	EAPI string
}

// InstalledFile represents a file installed by a package.
type InstalledFile struct {
	// Path is the absolute file path.
	Path string

	// Type is the file type (regular, directory, symlink, hardlink).
	Type FileType

	// Size is the file size in bytes (0 for directories).
	Size int64

	// Mode is the file permission mode.
	Mode uint32

	// Hash is the SHA256 hash of the file content (empty for directories).
	Hash string

	// Target is the symlink target (only for symlinks).
	Target string

	// MTime is the file modification time.
	MTime int64
}

// NewPackageDatabase creates a new package database.
//
// The root parameter should be the VarDB root directory (usually /var/db/pkg).
// The database is initially empty; use Load() to load existing packages.
//
// Example:
//
//	db := NewPackageDatabase("/var/db/pkg")
func NewPackageDatabase(root string) *PackageDatabase {
	return &PackageDatabase{
		Root:      root,
		packages:  make(map[string]*InstalledPackage),
		fileIndex: make(map[string]string),
	}
}

// Add adds a package to the database.
//
// If a package with the same atom already exists, it will be replaced.
// All files from the package are indexed for quick owner lookups.
//
// This operation is thread-safe.
func (db *PackageDatabase) Add(pkg *InstalledPackage) error {
	if pkg == nil {
		return fmt.Errorf("cannot add nil package")
	}

	if pkg.Package == nil {
		return fmt.Errorf("package metadata is nil")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Format atom as "category/package-version"
	atom := fmt.Sprintf("%s-%s", pkg.Package.Name, pkg.Package.Version)

	// Remove old package if exists (to clean up file index)
	if oldPkg, exists := db.packages[atom]; exists {
		for _, file := range oldPkg.Files {
			delete(db.fileIndex, file.Path)
		}
	}

	// Add new package
	db.packages[atom] = pkg

	// Index all files
	for _, file := range pkg.Files {
		db.fileIndex[file.Path] = atom
	}

	return nil
}

// Remove removes a package from the database.
//
// Returns an error if the package does not exist.
// This operation is thread-safe.
func (db *PackageDatabase) Remove(atom string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	pkg, exists := db.packages[atom]
	if !exists {
		return fmt.Errorf("package not found: %s", atom)
	}

	// Remove file index entries
	for _, file := range pkg.Files {
		delete(db.fileIndex, file.Path)
	}

	// Remove package
	delete(db.packages, atom)

	return nil
}

// Get retrieves a package from the database.
//
// Returns nil, ErrNotFound if the package does not exist.
// This operation is thread-safe.
func (db *PackageDatabase) Get(atom string) (*InstalledPackage, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	pkg, exists := db.packages[atom]
	if !exists {
		return nil, fmt.Errorf("package not found: %s", atom)
	}

	return pkg, nil
}

// List returns all installed packages.
//
// The returned slice is a snapshot of the current state.
// This operation is thread-safe.
func (db *PackageDatabase) List() []*InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	packages := make([]*InstalledPackage, 0, len(db.packages))
	for _, pkg := range db.packages {
		packages = append(packages, pkg)
	}

	return packages
}

// Count returns the number of installed packages.
//
// This operation is thread-safe.
func (db *PackageDatabase) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return len(db.packages)
}

// FindFileOwners finds all packages that own the specified file.
//
// Returns an empty slice if no package owns the file.
// This operation is thread-safe and uses an optimized file index.
//
// Example:
//
//	owners := db.FindFileOwners("/usr/bin/gcc")
//	for _, pkg := range owners {
//	    fmt.Println(pkg.Package.Name)
//	}
func (db *PackageDatabase) FindFileOwners(path string) []*InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var owners []*InstalledPackage

	// Quick lookup using file index
	if atom, exists := db.fileIndex[path]; exists {
		if pkg, ok := db.packages[atom]; ok {
			owners = append(owners, pkg)
		}
	}

	return owners
}

// WhoOwns returns the package atom that owns the specified file.
//
// Returns an error if no package owns the file.
// This operation is thread-safe and uses an optimized file index.
//
// Example:
//
//	owner, err := db.WhoOwns("/usr/bin/gcc")
//	if err != nil {
//	    fmt.Println("File not owned by any package")
//	}
func (db *PackageDatabase) WhoOwns(path string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Quick lookup using file index
	if atom, exists := db.fileIndex[path]; exists {
		return atom, nil
	}

	return "", fmt.Errorf("file not owned by any package: %s", path)
}

// Has checks if a package is installed.
//
// This operation is thread-safe.
func (db *PackageDatabase) Has(atom string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	_, exists := db.packages[atom]
	return exists
}

// IsInstalled checks if any version of a package is installed.
//
// The name parameter should be "category/package" format (e.g., "sys-libs/glib").
// This is different from Has() which requires the full atom with version.
//
// This operation is thread-safe.
//
// Example:
//
//	if db.IsInstalled("sys-libs/glib") {
//	    fmt.Println("glib is installed")
//	}
func (db *PackageDatabase) IsInstalled(name string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, pkg := range db.packages {
		if pkg.Package != nil && pkg.Package.Name == name {
			return true
		}
	}
	return false
}

// GetInstalledVersion returns the installed version of a package.
//
// The name parameter should be "category/package" format (e.g., "sys-libs/glib").
// Returns the InstalledPackage if found, nil otherwise.
// If multiple versions are installed (different slots), returns the first one found.
//
// This operation is thread-safe.
func (db *PackageDatabase) GetInstalledVersion(name string) *InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, pkg := range db.packages {
		if pkg.Package != nil && pkg.Package.Name == name {
			return pkg
		}
	}
	return nil
}

// Clear removes all packages from the database.
//
// This operation is thread-safe.
func (db *PackageDatabase) Clear() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.packages = make(map[string]*InstalledPackage)
	db.fileIndex = make(map[string]string)
}

// Stats returns database statistics.
func (db *PackageDatabase) Stats() DatabaseStats {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := DatabaseStats{
		PackageCount: len(db.packages),
		FileCount:    len(db.fileIndex),
	}

	var totalSize int64
	for _, pkg := range db.packages {
		totalSize += pkg.Size
	}
	stats.TotalSize = totalSize

	return stats
}

// DatabaseStats contains database statistics.
type DatabaseStats struct {
	// PackageCount is the number of installed packages.
	PackageCount int

	// FileCount is the total number of tracked files.
	FileCount int

	// TotalSize is the total installed size in bytes.
	TotalSize int64
}
