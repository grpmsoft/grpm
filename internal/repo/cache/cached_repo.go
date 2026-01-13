package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	basecache "github.com/grpmsoft/grpm/internal/cache"
	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// CachedRepository wraps a repository with caching capabilities.
// It intercepts package loading to check cache first, falling back
// to the underlying repository and populating the cache on miss.
type CachedRepository struct {
	cache    *RepoCache
	repoPath string

	// Statistics
	hits       int64
	misses     int64
	indexHits  int64
	indexLoads int64
}

// NewCachedRepository creates a repository wrapper with caching.
// repoPath is the path to the Portage repository.
// cachePath is the base directory for cache files.
// Returns a wrapper that can be used to speed up package lookups.
func NewCachedRepository(repoPath, cachePath string) (*CachedRepository, error) {
	// Derive repo name from path
	repoName := filepath.Base(repoPath)
	if repoName == "" || repoName == "." {
		repoName = "gentoo"
	}

	cfg := &Config{
		CachePath:   cachePath,
		RepoPath:    repoPath,
		RepoName:    repoName,
		Backend:     "sqlite",
		MaxEntries:  100000,
		EnableIndex: true,
	}

	cache, err := New(cfg)
	if err != nil {
		return nil, err
	}

	return &CachedRepository{
		cache:    cache,
		repoPath: repoPath,
	}, nil
}

// GetCachedEntry retrieves a cached entry for a package.
// Returns nil, basecache.ErrNotFound if not in cache.
func (cr *CachedRepository) GetCachedEntry(ctx context.Context, category, name, version string) (*basecache.Entry, error) {
	entry, err := cr.cache.Get(ctx, category, name, version)
	if err != nil {
		atomic.AddInt64(&cr.misses, 1)
		return nil, err
	}

	atomic.AddInt64(&cr.hits, 1)
	return entry, nil
}

// CachePackage stores a package in the cache.
func (cr *CachedRepository) CachePackage(ctx context.Context, p *pkg.Package) error {
	return cr.cache.Put(ctx, p)
}

// LookupPackageVersions returns all versions of a package from the index.
// This is much faster than scanning the filesystem.
// Returns nil if index is not available.
func (cr *CachedRepository) LookupPackageVersions(ctx context.Context, category, name string) ([]PackageEntry, error) {
	if cr.cache.Index() == nil {
		return nil, nil
	}

	entries, err := cr.cache.Index().LookupPackage(ctx, category, name)
	if err != nil {
		return nil, err
	}

	if len(entries) > 0 {
		atomic.AddInt64(&cr.indexHits, 1)
	}

	return entries, nil
}

// LookupCategoryPackages returns all package names in a category.
func (cr *CachedRepository) LookupCategoryPackages(ctx context.Context, category string) ([]string, error) {
	if cr.cache.Index() == nil {
		return nil, nil
	}

	return cr.cache.Index().LookupCategory(ctx, category)
}

// RebuildIndex rebuilds the repository index.
// Should be called after repository sync.
func (cr *CachedRepository) RebuildIndex(ctx context.Context) error {
	if cr.cache.Index() == nil {
		return nil
	}

	atomic.AddInt64(&cr.indexLoads, 1)
	return cr.cache.Index().Rebuild(ctx)
}

// EnsureIndex ensures the index is populated.
// If the index is empty or stale, it will be rebuilt.
func (cr *CachedRepository) EnsureIndex(ctx context.Context) error {
	if cr.cache.Index() == nil {
		return nil
	}

	// Check if index is valid
	if cr.cache.Index().IsValid(ctx) {
		return nil
	}

	// Rebuild stale index
	logging.Debug("Repository index is stale, rebuilding...")
	return cr.RebuildIndex(ctx)
}

// InvalidateAll clears both cache and index.
func (cr *CachedRepository) InvalidateAll(ctx context.Context) error {
	return cr.cache.InvalidateAll(ctx)
}

// Close releases resources.
func (cr *CachedRepository) Close() error {
	return cr.cache.Close()
}

// Stats returns cache statistics.
func (cr *CachedRepository) Stats() CachedRepoStats {
	cacheStats := cr.cache.Stats()

	return CachedRepoStats{
		CacheHits:   atomic.LoadInt64(&cr.hits),
		CacheMisses: atomic.LoadInt64(&cr.misses),
		IndexHits:   atomic.LoadInt64(&cr.indexHits),
		IndexLoads:  atomic.LoadInt64(&cr.indexLoads),
		CacheStats:  cacheStats,
	}
}

// CachedRepoStats holds statistics for the cached repository.
type CachedRepoStats struct {
	CacheHits   int64
	CacheMisses int64
	IndexHits   int64
	IndexLoads  int64
	CacheStats  basecache.Stats
}

// HitRate returns the cache hit rate as a percentage.
func (s *CachedRepoStats) HitRate() float64 {
	total := s.CacheHits + s.CacheMisses
	if total == 0 {
		return 0.0
	}
	return float64(s.CacheHits) / float64(total) * 100.0
}

// GetEbuildVersions returns all version strings for a package by scanning the filesystem.
// This is used as a fallback when the index is not available.
func (cr *CachedRepository) GetEbuildVersions(category, name string) ([]string, error) {
	pkgDir := filepath.Join(cr.repoPath, category, name)

	files, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("reading package directory: %w", err)
	}

	var versions []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".ebuild") {
			continue
		}

		version := strings.TrimSuffix(file.Name(), ".ebuild")
		version = strings.TrimPrefix(version, name+"-")
		versions = append(versions, version)
	}

	return versions, nil
}

// GetEbuildPath returns the full path to an ebuild file.
func (cr *CachedRepository) GetEbuildPath(category, name, version string) string {
	filename := fmt.Sprintf("%s-%s.ebuild", name, version)
	return filepath.Join(cr.repoPath, category, name, filename)
}

// IsEnabled returns true if caching is enabled.
func (cr *CachedRepository) IsEnabled() bool {
	return cr.cache.IsEnabled()
}

// IndexCount returns the number of package versions in the index.
func (cr *CachedRepository) IndexCount(ctx context.Context) (int64, error) {
	if cr.cache.Index() == nil {
		return 0, nil
	}

	return cr.cache.Index().Count(ctx)
}
