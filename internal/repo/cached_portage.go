package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	basecache "github.com/grpmsoft/grpm/internal/cache"
	repocache "github.com/grpmsoft/grpm/internal/repo/cache"

	"github.com/grpmsoft/grpm/internal/logging"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// CachedPortageRepository extends PortageRepository with persistent caching.
// Provides SQLite-backed metadata cache and directory index for fast lookups.
type CachedPortageRepository struct {
	*PortageRepository
	repoCache     *repocache.CachedRepository
	cacheEnabled  bool
	parsedPackage sync.Map // path -> *pkg.Package (in-memory parsed cache)
}

// CachedPortageConfig holds configuration for cached repository.
type CachedPortageConfig struct {
	// RepoPath is the path to the Portage repository.
	RepoPath string

	// CachePath is the base directory for cache files.
	// Default: /var/cache/grpm/metadata
	CachePath string

	// EnableCache enables the SQLite metadata cache.
	// Default: true
	EnableCache bool

	// EnableIndex enables the repository directory index.
	// Default: true
	EnableIndex bool
}

// DefaultCachedPortageConfig returns default configuration.
func DefaultCachedPortageConfig(repoPath string) *CachedPortageConfig {
	return &CachedPortageConfig{
		RepoPath:    repoPath,
		CachePath:   "/var/cache/grpm/metadata",
		EnableCache: true,
		EnableIndex: true,
	}
}

// NewCachedPortageRepository creates a new cached Portage repository.
// Falls back to uncached operation if caching initialization fails.
func NewCachedPortageRepository(cfg *CachedPortageConfig) (*CachedPortageRepository, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Create base repository
	base, err := NewPortageRepository(cfg.RepoPath)
	if err != nil {
		return nil, err
	}

	cpr := &CachedPortageRepository{
		PortageRepository: base,
		cacheEnabled:      false,
	}

	// Initialize cache if enabled
	if cfg.EnableCache {
		repoCache, err := repocache.NewCachedRepository(cfg.RepoPath, cfg.CachePath)
		if err != nil {
			logging.Debug("Warning: cache initialization failed, running without cache: %v", err)
		} else {
			cpr.repoCache = repoCache
			cpr.cacheEnabled = repoCache.IsEnabled()

			if cpr.cacheEnabled {
				logging.Debug("Repository cache enabled: %s", cfg.CachePath)
			}
		}
	}

	return cpr, nil
}

// LoadPackage loads a package, checking cache first.
func (cpr *CachedPortageRepository) LoadPackage(name string) (*pkg.Package, error) {
	category, pkgName, found := strings.Cut(name, "/")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", name)
	}

	// Try cache first if enabled
	if cpr.cacheEnabled && cpr.repoCache != nil {
		// Get all versions from index
		versions, err := cpr.getVersionsFromIndex(category, pkgName)
		if err == nil && len(versions) > 0 {
			// Sort versions using Portage version comparison (highest first)
			sort.Slice(versions, func(i, j int) bool {
				return pkg.CompareVersions(versions[i], versions[j]) > 0
			})
			// Use highest version
			latestVersion := versions[0]
			p, err := cpr.loadPackageWithCache(category, pkgName, latestVersion)
			if err == nil {
				return p, nil
			}
		}
	}

	// Fall back to base implementation
	return cpr.PortageRepository.LoadPackage(name)
}

// GetAllVersions returns all versions of a package.
// Uses index if available, falls back to filesystem scan.
func (cpr *CachedPortageRepository) GetAllVersions(packageName string) ([]*pkg.Package, error) {
	category, pkgName, found := strings.Cut(packageName, "/")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", packageName)
	}

	// Try index first
	if cpr.cacheEnabled && cpr.repoCache != nil {
		entries, err := cpr.repoCache.LookupPackageVersions(context.Background(), category, pkgName)
		if err == nil && len(entries) > 0 {
			var packages []*pkg.Package
			for _, entry := range entries {
				p, err := cpr.loadPackageWithCache(category, pkgName, entry.Version)
				if err != nil {
					// Fall back to parsing
					p, err = cpr.parseEbuildCached(packageName, entry.Version)
					if err != nil {
						continue
					}
				}
				packages = append(packages, p)
			}
			if len(packages) > 0 {
				return packages, nil
			}
		}
	}

	// Fall back to base implementation
	return cpr.PortageRepository.GetAllVersions(packageName)
}

// FindByAtom finds all packages matching a PMS-compliant atom.
// Uses cache and index for faster lookups.
func (cpr *CachedPortageRepository) FindByAtom(atom *pkg.Atom) ([]*pkg.Package, error) {
	if atom == nil {
		return nil, fmt.Errorf("atom is nil")
	}

	// Get all versions (will use cache if available)
	versions, err := cpr.GetAllVersions(atom.CP())
	if err != nil {
		return []*pkg.Package{}, nil
	}

	// Filter by atom matching
	var result []*pkg.Package
	for _, p := range versions {
		if atom.Matches(p) {
			result = append(result, p)
		}
	}

	return result, nil
}

// loadPackageWithCache loads a specific package version, using cache if available.
func (cpr *CachedPortageRepository) loadPackageWithCache(category, name, version string) (*pkg.Package, error) {
	ctx := context.Background()

	// Check cache
	entry, err := cpr.repoCache.GetCachedEntry(ctx, category, name, version)
	if err == nil {
		// Convert cache entry to package
		return cpr.entryToPackage(entry, category, name, version)
	}

	// Cache miss - parse ebuild
	packageName := category + "/" + name
	p, err := cpr.parseEbuildCached(packageName, version)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := cpr.repoCache.CachePackage(ctx, p); err != nil {
		logging.Debug("Warning: failed to cache package %s: %v", packageName, err)
	}

	return p, nil
}

// parseEbuildCached parses an ebuild with in-memory caching.
func (cpr *CachedPortageRepository) parseEbuildCached(packageName, version string) (*pkg.Package, error) {
	category, pkgName, _ := strings.Cut(packageName, "/")
	ebuildPath := cpr.getEbuildPath(category, pkgName, version)

	// Check in-memory cache first
	if cached, ok := cpr.parsedPackage.Load(ebuildPath); ok {
		return cached.(*pkg.Package), nil
	}

	// Parse ebuild
	p, err := cpr.parseEbuild(packageName, ebuildPath)
	if err != nil {
		return nil, err
	}

	// Store in in-memory cache
	cpr.parsedPackage.Store(ebuildPath, p)

	return p, nil
}

// getVersionsFromIndex gets version list from index.
func (cpr *CachedPortageRepository) getVersionsFromIndex(category, name string) ([]string, error) {
	if cpr.repoCache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	entries, err := cpr.repoCache.LookupPackageVersions(context.Background(), category, name)
	if err != nil {
		return nil, err
	}

	versions := make([]string, len(entries))
	for i, e := range entries {
		versions[i] = e.Version
	}

	return versions, nil
}

// getEbuildPath constructs path to ebuild file.
func (cpr *CachedPortageRepository) getEbuildPath(category, name, version string) string {
	filename := fmt.Sprintf("%s-%s.ebuild", name, version)
	return filepath.Join(cpr.Path, category, name, filename)
}

// entryToPackage converts a cache entry to a package.
// May need to parse dependencies from the actual ebuild.
func (cpr *CachedPortageRepository) entryToPackage(entry *basecache.Entry, category, name, version string) (*pkg.Package, error) {
	p := &pkg.Package{
		Name:     category + "/" + name,
		Version:  version,
		Slot:     pkg.Slot{Name: entry.Slot, Subslot: entry.SubSlot},
		UseFlags: make(map[string]bool),
		Deps:     make([]pkg.Constraint, 0),
		Provides: make([]pkg.Constraint, 0),
	}

	// Convert IUSE
	for _, flag := range entry.IUSE {
		p.UseFlags[flag] = true
	}

	// Parse dependencies if present in cache
	if entry.Depend != "" || entry.RDepend != "" || entry.BDepend != "" {
		// Use cached dependency strings - parse on demand
		// For now, parse the ebuild for full dependency info
		ebuildPath := cpr.getEbuildPath(category, name, version)
		content, err := os.ReadFile(ebuildPath)
		if err == nil {
			meta := NewPackageMetadata(category, name, version)
			parser := NewEbuildParserWithMetadata(string(content), meta)
			parsedDeps, err := parser.ParseDependencies()
			if err == nil {
				for _, pd := range parsedDeps {
					if !pd.IsBlocker {
						constraint := pd.Constraint
						constraint.OrGroupID = pd.OrGroupID
						p.Deps = append(p.Deps, constraint)
					}
				}
			}
		}
	}

	return p, nil
}

// RebuildCache rebuilds the cache and index.
// Should be called after repository sync.
func (cpr *CachedPortageRepository) RebuildCache(ctx context.Context) error {
	if !cpr.cacheEnabled || cpr.repoCache == nil {
		return nil
	}

	logging.Debug("Rebuilding repository cache...")
	start := time.Now()

	if err := cpr.repoCache.RebuildIndex(ctx); err != nil {
		return fmt.Errorf("rebuilding index: %w", err)
	}

	// Clear in-memory cache
	cpr.parsedPackage = sync.Map{}

	logging.Debug("Cache rebuilt in %v", time.Since(start))
	return nil
}

// InvalidateCache invalidates all cached data.
func (cpr *CachedPortageRepository) InvalidateCache(ctx context.Context) error {
	if !cpr.cacheEnabled || cpr.repoCache == nil {
		return nil
	}

	// Clear in-memory cache
	cpr.parsedPackage = sync.Map{}

	return cpr.repoCache.InvalidateAll(ctx)
}

// EnsureCache ensures the cache is populated.
func (cpr *CachedPortageRepository) EnsureCache(ctx context.Context) error {
	if !cpr.cacheEnabled || cpr.repoCache == nil {
		return nil
	}

	return cpr.repoCache.EnsureIndex(ctx)
}

// CacheStats returns cache statistics.
func (cpr *CachedPortageRepository) CacheStats() repocache.CachedRepoStats {
	if !cpr.cacheEnabled || cpr.repoCache == nil {
		return repocache.CachedRepoStats{}
	}

	return cpr.repoCache.Stats()
}

// IsCacheEnabled returns true if caching is enabled.
func (cpr *CachedPortageRepository) IsCacheEnabled() bool {
	return cpr.cacheEnabled
}

// Close releases resources.
func (cpr *CachedPortageRepository) Close() error {
	if cpr.repoCache != nil {
		return cpr.repoCache.Close()
	}
	return nil
}
