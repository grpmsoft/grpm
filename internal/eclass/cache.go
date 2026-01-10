// Package eclass provides dynamic eclass loading and execution.
//
// This package implements the eclass cache and executor for GRPM,
// replacing hardcoded Go eclass implementations with dynamic loading
// from repository eclass/ directories. This approach ensures:
//   - Gentoo eclass updates propagate to GRPM automatically
//   - Custom overlay eclasses work correctly
//   - Different repositories can have different eclass versions
//
// Reference: Portage lib/portage/eclass_cache.py
package eclass

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Eclass represents a cached eclass file.
//
// This is a value object that is immutable after creation.
// It stores metadata about an eclass file for caching and validation.
type Eclass struct {
	// Name is the eclass name without .eclass extension.
	Name string

	// Path is the full filesystem path to the .eclass file.
	Path string

	// EclassDir is the directory containing this eclass (e.g., /var/db/repos/gentoo/eclass).
	EclassDir string

	// Mtime is the modification time for cache invalidation.
	Mtime time.Time

	// Checksum is the SHA256 checksum for content verification.
	Checksum string

	// RepoName is the repository name this eclass belongs to.
	RepoName string
}

// ID returns a unique identifier for this eclass.
func (e *Eclass) ID() string {
	return e.Name + "@" + e.RepoName
}

// Cache manages eclass discovery, caching, and retrieval.
//
// It scans eclass/ directories in repositories and maintains a cache
// with mtime-based invalidation. The cache follows Portage's priority
// ordering: masters -> repo -> overrides.
//
// Thread-safe: All methods can be called concurrently.
type Cache struct {
	mu sync.RWMutex

	// eclasses maps eclass name to the resolved eclass entry.
	// When multiple repos define the same eclass, the highest-priority one wins.
	eclasses map[string]*Eclass

	// locations stores eclass directories in priority order (lowest to highest).
	// Later entries override earlier ones.
	locations []string

	// repoNames maps eclass directory path to repository name.
	repoNames map[string]string

	// masterEclassDir is the master (gentoo) eclass directory for deduplication.
	masterEclassDir string

	// masterEclasses stores mtime of master eclasses for identical-eclass detection.
	masterEclasses map[string]time.Time
}

// NewCache creates a new empty eclass cache.
func NewCache() *Cache {
	return &Cache{
		eclasses:       make(map[string]*Eclass),
		locations:      make([]string, 0),
		repoNames:      make(map[string]string),
		masterEclasses: make(map[string]time.Time),
	}
}

// NewCacheWithLocations creates a cache and scans the specified eclass directories.
//
// Locations are processed in order; later locations override earlier ones.
// This matches Portage behavior where overlays override the main repo.
func NewCacheWithLocations(locations []string) (*Cache, error) {
	c := NewCache()
	if err := c.AddLocations(locations); err != nil {
		return nil, err
	}
	return c, nil
}

// AddLocations adds eclass directories to the cache.
//
// Directories are scanned immediately and eclasses are added to the cache.
// Later locations override earlier ones (overlay behavior).
func (c *Cache) AddLocations(locations []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, loc := range locations {
		// Normalize path
		loc = filepath.Clean(loc)

		// Skip if already added
		alreadyAdded := false
		for _, existing := range c.locations {
			if existing == loc {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}

		c.locations = append(c.locations, loc)

		// Scan the directory
		if err := c.scanDirectoryLocked(loc); err != nil {
			// Log warning but continue - directory may not exist yet
			continue
		}
	}

	return nil
}

// AddRepoWithMasters adds a repository with its master repositories.
//
// Masters are added first (lower priority), then the repo itself (higher priority).
// This matches Portage's masters inheritance model.
//
// Parameters:
//   - repoName: Name of the repository (e.g., "gentoo", "guru")
//   - repoPath: Path to the repository root (eclass/ will be appended)
//   - masters: Paths to master repository roots (in order)
func (c *Cache) AddRepoWithMasters(repoName string, repoPath string, masters []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add masters first (lower priority)
	for _, master := range masters {
		eclassDir := filepath.Join(master, "eclass")
		if _, err := os.Stat(eclassDir); os.IsNotExist(err) {
			continue
		}

		// Determine master repo name from path
		masterName := filepath.Base(master)
		c.repoNames[eclassDir] = masterName

		// First master becomes the master eclass dir for deduplication
		if c.masterEclassDir == "" {
			c.masterEclassDir = eclassDir
		}

		if !c.hasLocation(eclassDir) {
			c.locations = append(c.locations, eclassDir)
			if err := c.scanDirectoryLocked(eclassDir); err != nil {
				continue
			}
		}
	}

	// Add the repo itself (higher priority)
	eclassDir := filepath.Join(repoPath, "eclass")
	c.repoNames[eclassDir] = repoName

	if _, err := os.Stat(eclassDir); err == nil {
		if !c.hasLocation(eclassDir) {
			c.locations = append(c.locations, eclassDir)
			if err := c.scanDirectoryLocked(eclassDir); err != nil {
				return err
			}
		}
	}

	return nil
}

// hasLocation checks if a location is already in the list.
// Must be called with lock held.
func (c *Cache) hasLocation(loc string) bool {
	for _, existing := range c.locations {
		if existing == loc {
			return true
		}
	}
	return false
}

// scanDirectoryLocked scans an eclass directory and adds entries to the cache.
// Must be called with lock held.
func (c *Cache) scanDirectoryLocked(eclassDir string) error {
	entries, err := os.ReadDir(eclassDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error - directory may not exist
		}
		return fmt.Errorf("reading eclass directory %s: %w", eclassDir, err)
	}

	repoName := c.repoNames[eclassDir]
	isMaster := eclassDir == c.masterEclassDir

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".eclass") {
			continue
		}

		eclassName := strings.TrimSuffix(name, ".eclass")
		eclassPath := filepath.Join(eclassDir, name)

		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}
		mtime := info.ModTime()

		// If this is the master repo, record mtime for deduplication
		if isMaster {
			c.masterEclasses[eclassName] = mtime
		} else if c.masterEclassDir != "" {
			// Check if this eclass is identical to master (same mtime)
			// If so, prefer the master entry
			if masterMtime, ok := c.masterEclasses[eclassName]; ok {
				if masterMtime.Equal(mtime) {
					continue // Skip - identical to master
				}
			}
		}

		// Create eclass entry
		eclass := &Eclass{
			Name:      eclassName,
			Path:      eclassPath,
			EclassDir: eclassDir,
			Mtime:     mtime,
			RepoName:  repoName,
		}

		// Checksum is computed lazily on first access
		c.eclasses[eclassName] = eclass
	}

	return nil
}

// Get retrieves an eclass by name.
//
// Returns the highest-priority eclass with the given name, or an error
// if the eclass is not found.
func (c *Cache) Get(name string) (*Eclass, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	eclass, ok := c.eclasses[name]
	if !ok {
		return nil, &EclassNotFoundError{Name: name, Locations: c.locations}
	}

	return eclass, nil
}

// GetWithChecksum retrieves an eclass and computes its checksum if not cached.
//
// This is more expensive than Get() but ensures checksum is populated.
func (c *Cache) GetWithChecksum(name string) (*Eclass, error) {
	c.mu.RLock()
	eclass, ok := c.eclasses[name]
	c.mu.RUnlock()

	if !ok {
		return nil, &EclassNotFoundError{Name: name, Locations: c.locations}
	}

	// Compute checksum if not cached
	if eclass.Checksum == "" {
		checksum, err := computeChecksum(eclass.Path)
		if err != nil {
			return nil, fmt.Errorf("computing checksum for %s: %w", name, err)
		}

		c.mu.Lock()
		// Create new eclass with checksum (immutability)
		updated := &Eclass{
			Name:      eclass.Name,
			Path:      eclass.Path,
			EclassDir: eclass.EclassDir,
			Mtime:     eclass.Mtime,
			Checksum:  checksum,
			RepoName:  eclass.RepoName,
		}
		c.eclasses[name] = updated
		c.mu.Unlock()

		return updated, nil
	}

	return eclass, nil
}

// Has checks if an eclass exists in the cache.
func (c *Cache) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.eclasses[name]
	return ok
}

// List returns all eclass names in the cache.
func (c *Cache) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.eclasses))
	for name := range c.eclasses {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Invalidate removes an eclass from the cache, forcing reload on next access.
func (c *Cache) Invalidate(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.eclasses, name)
}

// InvalidateAll clears the entire cache.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eclasses = make(map[string]*Eclass)
	c.masterEclasses = make(map[string]time.Time)
}

// Refresh rescans all eclass directories and updates the cache.
//
// This detects new eclasses, removed eclasses, and modified eclasses
// based on mtime changes.
func (c *Cache) Refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear existing entries
	c.eclasses = make(map[string]*Eclass)
	c.masterEclasses = make(map[string]time.Time)

	// Rescan all locations
	for _, loc := range c.locations {
		if err := c.scanDirectoryLocked(loc); err != nil {
			return err
		}
	}

	return nil
}

// ValidateEclass checks if a cached eclass is still valid.
//
// Returns true if the eclass file has the same mtime as cached.
// This is used for incremental validation without re-scanning.
func (c *Cache) ValidateEclass(name string) (bool, error) {
	c.mu.RLock()
	eclass, ok := c.eclasses[name]
	c.mu.RUnlock()

	if !ok {
		return false, nil
	}

	info, err := os.Stat(eclass.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// File was removed - invalidate
			c.Invalidate(name)
			return false, nil
		}
		return false, err
	}

	return info.ModTime().Equal(eclass.Mtime), nil
}

// Locations returns the list of eclass directories in priority order.
func (c *Cache) Locations() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.locations))
	copy(result, c.locations)
	return result
}

// LocationsString returns locations as a space-separated string.
//
// This format is used by PORTAGE_ECLASS_LOCATIONS environment variable.
func (c *Cache) LocationsString() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Reverse order for bash consumption (highest priority first)
	reversed := make([]string, len(c.locations))
	for i, loc := range c.locations {
		reversed[len(c.locations)-1-i] = loc
	}
	return strings.Join(reversed, " ")
}

// GetEclassData returns eclass data for the specified inherited eclasses.
//
// This is used for cache validation - returns a map of eclass name to Eclass.
func (c *Cache) GetEclassData(inherits []string) (map[string]*Eclass, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*Eclass, len(inherits))
	for _, name := range inherits {
		eclass, ok := c.eclasses[name]
		if !ok {
			return nil, &EclassNotFoundError{Name: name, Locations: c.locations}
		}
		result[name] = eclass
	}
	return result, nil
}

// computeChecksum calculates SHA256 checksum of a file.
func computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// EclassNotFoundError is returned when an eclass cannot be found.
type EclassNotFoundError struct {
	Name      string
	Locations []string
}

func (e *EclassNotFoundError) Error() string {
	return fmt.Sprintf("eclass %s not found in: %v", e.Name, e.Locations)
}

// IsEclassNotFound checks if an error is an EclassNotFoundError.
func IsEclassNotFound(err error) bool {
	var eclassErr *EclassNotFoundError
	return errors.As(err, &eclassErr)
}
