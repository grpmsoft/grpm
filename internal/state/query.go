package state

import (
	"strings"
	"time"
)

// QuerySpec specifies criteria for querying installed packages.
type QuerySpec struct {
	// Category filters packages by category.
	// Example: "sys-libs", "app-editors"
	Category string

	// NamePattern filters packages by name pattern (supports wildcards).
	// Example: "zlib*", "*vim*"
	NamePattern string

	// InstalledAfter filters packages installed after this time.
	InstalledAfter time.Time

	// InstalledBefore filters packages installed before this time.
	InstalledBefore time.Time

	// HasUSEFlag filters packages that have this USE flag enabled.
	HasUSEFlag string

	// OwnsFile filters packages that own this file.
	OwnsFile string

	// MinSize filters packages larger than this size (bytes).
	MinSize int64

	// MaxSize filters packages smaller than this size (bytes).
	MaxSize int64

	// Limit limits the number of results (0 = no limit).
	Limit int
}

// Query executes a query against the package database.
//
// Returns packages matching ALL specified criteria.
// Empty QuerySpec returns all packages.
//
// Example:
//
//	// Find all sys-libs packages
//	results := db.Query(QuerySpec{Category: "sys-libs"})
//
//	// Find packages with 'ssl' USE flag
//	results := db.Query(QuerySpec{HasUSEFlag: "ssl"})
//
//	// Find large packages (>100MB)
//	results := db.Query(QuerySpec{MinSize: 100 * 1024 * 1024})
func (db *PackageDatabase) Query(spec QuerySpec) []*InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	results := make([]*InstalledPackage, 0)

	for _, pkg := range db.packages {
		if matchesQuery(pkg, spec) {
			results = append(results, pkg)

			// Check limit
			if spec.Limit > 0 && len(results) >= spec.Limit {
				break
			}
		}
	}

	return results
}

// matchesQuery checks if a package matches the query spec.
func matchesQuery(pkg *InstalledPackage, spec QuerySpec) bool {
	// Category filter
	if spec.Category != "" {
		// Extract category from package name (e.g., "sys-libs/zlib" -> "sys-libs")
		parts := strings.Split(pkg.Package.Name, "/")
		if len(parts) < 2 || parts[0] != spec.Category {
			return false
		}
	}

	// Name pattern filter
	if spec.NamePattern != "" {
		if !matchPattern(pkg.Package.Name, spec.NamePattern) {
			return false
		}
	}

	// Install time filters
	if !spec.InstalledAfter.IsZero() && pkg.InstallTime.Before(spec.InstalledAfter) {
		return false
	}

	if !spec.InstalledBefore.IsZero() && pkg.InstallTime.After(spec.InstalledBefore) {
		return false
	}

	// USE flag filter
	if spec.HasUSEFlag != "" {
		if !hasUSEFlag(pkg.USE, spec.HasUSEFlag) {
			return false
		}
	}

	// File ownership filter
	if spec.OwnsFile != "" {
		if !ownsFile(pkg, spec.OwnsFile) {
			return false
		}
	}

	// Size filters
	if spec.MinSize > 0 && pkg.Size < spec.MinSize {
		return false
	}

	if spec.MaxSize > 0 && pkg.Size > spec.MaxSize {
		return false
	}

	return true
}

// matchPattern matches a string against a pattern with wildcards.
//
// Supports:
//   - * (matches any sequence of characters)
//   - Exact match if no wildcard
func matchPattern(s, pattern string) bool {
	if pattern == "" {
		return true
	}

	// No wildcard - exact match
	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	// Convert pattern to simple regex
	// Example: "zlib*" -> prefix match, "*vim*" -> contains
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// *pattern* - contains
		middle := strings.Trim(pattern, "*")
		return strings.Contains(s, middle)
	}

	if strings.HasPrefix(pattern, "*") {
		// *pattern - suffix match
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(s, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		// pattern* - prefix match
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(s, prefix)
	}

	// Multiple wildcards - split and check
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}

	// Check first part (prefix)
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}

	// Check last part (suffix)
	if !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false
	}

	// Check middle parts (contains)
	currentPos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(s[currentPos:], parts[i])
		if idx == -1 {
			return false
		}
		currentPos += idx + len(parts[i])
	}

	return true
}

// hasUSEFlag checks if a USE flag list contains the specified flag.
func hasUSEFlag(useFlags []string, flag string) bool {
	for _, f := range useFlags {
		if f == flag {
			return true
		}
	}
	return false
}

// ownsFile checks if a package owns the specified file.
func ownsFile(pkg *InstalledPackage, path string) bool {
	for _, file := range pkg.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

// FindByCategory returns all packages in a category.
//
// This is a convenience method equivalent to Query(QuerySpec{Category: category}).
func (db *PackageDatabase) FindByCategory(category string) []*InstalledPackage {
	return db.Query(QuerySpec{Category: category})
}

// FindByPattern returns all packages matching a name pattern.
//
// Supports wildcards (* matches any sequence).
//
// Example:
//
//	// Find all zlib packages
//	pkgs := db.FindByPattern("*zlib*")
//
//	// Find packages starting with "vim"
//	pkgs := db.FindByPattern("vim*")
func (db *PackageDatabase) FindByPattern(pattern string) []*InstalledPackage {
	return db.Query(QuerySpec{NamePattern: pattern})
}

// FindInstalledAfter returns packages installed after the specified time.
func (db *PackageDatabase) FindInstalledAfter(t time.Time) []*InstalledPackage {
	return db.Query(QuerySpec{InstalledAfter: t})
}

// FindInstalledBefore returns packages installed before the specified time.
func (db *PackageDatabase) FindInstalledBefore(t time.Time) []*InstalledPackage {
	return db.Query(QuerySpec{InstalledBefore: t})
}

// FindWithUSEFlag returns packages with the specified USE flag enabled.
func (db *PackageDatabase) FindWithUSEFlag(flag string) []*InstalledPackage {
	return db.Query(QuerySpec{HasUSEFlag: flag})
}

// FindLargePackages returns packages larger than the specified size.
//
// Size is in bytes.
func (db *PackageDatabase) FindLargePackages(minSize int64) []*InstalledPackage {
	return db.Query(QuerySpec{MinSize: minSize})
}
