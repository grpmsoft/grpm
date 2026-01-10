// Package cache provides repository-level caching for package metadata.
// It wraps the core cache.Cache interface with repository-specific logic
// including directory indexing and invalidation strategies.
package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/grpmsoft/grpm/internal/cache"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// Common errors for repository cache operations.
var (
	ErrCacheDisabled = errors.New("cache is disabled")
	ErrRepoNotFound  = errors.New("repository not found")
)

// RepoCache provides package metadata caching with repository awareness.
// Thread-safe for concurrent access.
type RepoCache struct {
	inner    cache.Cache
	index    *RepoIndex
	repoPath string
	disabled bool

	mu sync.RWMutex

	// Statistics
	converts int64 // Package to Entry conversions
}

// Config holds repository cache configuration.
type Config struct {
	// CachePath is the base directory for cache files.
	// Default: /var/cache/grpm/metadata
	CachePath string

	// RepoPath is the path to the repository to cache.
	RepoPath string

	// RepoName is the repository name (e.g., "gentoo").
	// Used for cache file naming.
	RepoName string

	// Backend specifies the cache backend: "sqlite" or "memory".
	// Default: "sqlite"
	Backend string

	// MaxEntries is the maximum entries for memory backend.
	// Default: 100000
	MaxEntries int

	// EnableIndex enables the repository directory index.
	// Provides fast package lookups without filesystem scans.
	// Default: true
	EnableIndex bool

	// IndexPath is the path for the index database.
	// Default: CachePath/RepoName/index.db
	IndexPath string
}

// DefaultConfig returns the default repository cache configuration.
func DefaultConfig(repoPath, repoName string) *Config {
	basePath := "/var/cache/grpm/metadata"
	return &Config{
		CachePath:   basePath,
		RepoPath:    repoPath,
		RepoName:    repoName,
		Backend:     "sqlite",
		MaxEntries:  100000,
		EnableIndex: true,
		IndexPath:   filepath.Join(basePath, repoName, "index.db"),
	}
}

// New creates a new repository cache.
// Falls back gracefully if cache creation fails.
func New(cfg *Config) (*RepoCache, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	if cfg.RepoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	// Verify repository exists
	if _, err := os.Stat(cfg.RepoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrRepoNotFound, cfg.RepoPath)
	}

	rc := &RepoCache{
		repoPath: cfg.RepoPath,
	}

	// Create inner cache
	innerCfg := &cache.Config{
		Backend:    cfg.Backend,
		Path:       filepath.Join(cfg.CachePath, cfg.RepoName, "cache.db"),
		MaxEntries: cfg.MaxEntries,
	}

	inner, err := cache.New(innerCfg)
	if err != nil {
		// Cache creation failed - disable caching
		rc.disabled = true
		return rc, nil
	}
	rc.inner = inner

	// Create index if enabled
	if cfg.EnableIndex {
		indexPath := cfg.IndexPath
		if indexPath == "" {
			indexPath = filepath.Join(cfg.CachePath, cfg.RepoName, "index.db")
		}

		index, err := NewRepoIndex(indexPath, cfg.RepoPath)
		if err != nil {
			// Index creation failed - continue without it
			// The cache will still work, just slower
		} else {
			rc.index = index
		}
	}

	return rc, nil
}

// Get retrieves cached metadata for a package.
// Returns nil, cache.ErrNotFound if entry does not exist or is stale.
func (rc *RepoCache) Get(ctx context.Context, category, name, version string) (*cache.Entry, error) {
	if rc.disabled || rc.inner == nil {
		return nil, ErrCacheDisabled
	}

	// Get from cache
	entry, err := rc.inner.Get(ctx, category, name, version)
	if err != nil {
		return nil, err
	}

	// Validate entry against current ebuild
	ebuildPath := rc.ebuildPath(category, name, version)
	info, err := os.Stat(ebuildPath)
	if err != nil {
		// Ebuild no longer exists - invalidate cache entry
		_ = rc.inner.Delete(ctx, category, name, version)
		return nil, cache.ErrNotFound
	}

	// Check if ebuild was modified since caching
	if !entry.IsValid(info.ModTime()) {
		// Entry is stale - remove it
		_ = rc.inner.Delete(ctx, category, name, version)
		return nil, cache.ErrNotFound
	}

	return entry, nil
}

// Put stores package metadata in cache.
// Automatically captures ebuild mtime for invalidation.
func (rc *RepoCache) Put(ctx context.Context, p *pkg.Package) error {
	if rc.disabled || rc.inner == nil {
		return ErrCacheDisabled
	}

	if p == nil {
		return cache.ErrInvalidEntry
	}

	// Get ebuild mtime
	category, name := splitPackageName(p.Name)
	ebuildPath := rc.ebuildPath(category, name, p.Version)

	info, err := os.Stat(ebuildPath)
	if err != nil {
		// Can't stat ebuild - skip caching
		return nil
	}

	// Convert package to cache entry
	entry := rc.packageToEntry(p, info.ModTime())

	rc.mu.Lock()
	rc.converts++
	rc.mu.Unlock()

	return rc.inner.Put(ctx, entry)
}

// PutEntry stores a cache entry directly.
func (rc *RepoCache) PutEntry(ctx context.Context, entry *cache.Entry) error {
	if rc.disabled || rc.inner == nil {
		return ErrCacheDisabled
	}

	return rc.inner.Put(ctx, entry)
}

// Delete removes a cached entry.
func (rc *RepoCache) Delete(ctx context.Context, category, name, version string) error {
	if rc.disabled || rc.inner == nil {
		return nil
	}

	return rc.inner.Delete(ctx, category, name, version)
}

// InvalidatePackage removes all cached entries for a package.
// Called when the package directory is modified.
func (rc *RepoCache) InvalidatePackage(ctx context.Context, category, name string) error {
	if rc.disabled || rc.inner == nil {
		return nil
	}

	// Use mtime-based invalidation with far-future time to remove all entries
	// The Invalidate method removes entries with ebuild_mtime < threshold
	farFuture := time.Now().Add(100 * 365 * 24 * time.Hour) // 100 years in future
	return rc.inner.Invalidate(ctx, category, name, farFuture)
}

// InvalidateAll removes all cached entries.
// Called after repository sync.
func (rc *RepoCache) InvalidateAll(ctx context.Context) error {
	if rc.disabled || rc.inner == nil {
		return nil
	}

	// Clear cache
	if err := rc.inner.InvalidateAll(ctx); err != nil {
		return err
	}

	// Clear index if present
	if rc.index != nil {
		return rc.index.Clear(ctx)
	}

	return nil
}

// Stats returns cache statistics.
func (rc *RepoCache) Stats() cache.Stats {
	if rc.disabled || rc.inner == nil {
		return cache.Stats{}
	}

	return rc.inner.Stats()
}

// Index returns the repository index, or nil if disabled.
func (rc *RepoCache) Index() *RepoIndex {
	return rc.index
}

// Close closes the cache and releases resources.
func (rc *RepoCache) Close() error {
	var errs []error

	if rc.inner != nil {
		if err := rc.inner.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if rc.index != nil {
		if err := rc.index.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("closing cache: %v", errs)
	}

	return nil
}

// IsEnabled returns true if caching is enabled.
func (rc *RepoCache) IsEnabled() bool {
	return !rc.disabled && rc.inner != nil
}

// ebuildPath constructs the path to an ebuild file.
func (rc *RepoCache) ebuildPath(category, name, version string) string {
	filename := fmt.Sprintf("%s-%s.ebuild", name, version)
	return filepath.Join(rc.repoPath, category, name, filename)
}

// packageToEntry converts a pkg.Package to a cache.Entry.
func (rc *RepoCache) packageToEntry(p *pkg.Package, mtime time.Time) *cache.Entry {
	category, name := splitPackageName(p.Name)

	entry := &cache.Entry{
		Category:    category,
		Name:        name,
		Version:     p.Version,
		Slot:        p.Slot.Name,
		SubSlot:     p.Slot.Subslot,
		EbuildMtime: mtime,
		CachedAt:    time.Now(),
	}

	// Extract USE flags
	if len(p.UseFlags) > 0 {
		flags := make([]string, 0, len(p.UseFlags))
		for flag := range p.UseFlags {
			flags = append(flags, flag)
		}
		entry.IUSE = flags
	}

	return entry
}

// splitPackageName splits "category/name" into category and name.
func splitPackageName(name string) (category, pkgName string) {
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}
