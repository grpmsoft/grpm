// Package cache provides persistent storage for ebuild metadata.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Common errors for cache operations.
var (
	ErrNotFound     = errors.New("cache entry not found")
	ErrInvalidEntry = errors.New("invalid cache entry")
	ErrCacheClosed  = errors.New("cache is closed")
)

// Cache provides persistent storage for ebuild metadata.
// Implementations must be thread-safe for concurrent access.
type Cache interface {
	// Get retrieves cached metadata for a package.
	// Returns ErrNotFound if entry does not exist.
	Get(ctx context.Context, category, name, version string) (*Entry, error)

	// Put stores metadata for a package.
	// Overwrites existing entry if present.
	Put(ctx context.Context, entry *Entry) error

	// PutBatch stores multiple entries efficiently.
	// Used for initial cache population.
	PutBatch(ctx context.Context, entries []*Entry) error

	// Delete removes cached metadata for a package.
	// Returns nil if entry does not exist.
	Delete(ctx context.Context, category, name, version string) error

	// Invalidate removes all entries older than given mtime for a package.
	// Used when ebuild directory is updated.
	Invalidate(ctx context.Context, category, name string, mtime time.Time) error

	// InvalidateAll removes all entries from the cache.
	InvalidateAll(ctx context.Context) error

	// Stats returns cache statistics.
	Stats() Stats

	// Close closes the cache and releases resources.
	Close() error
}

// Config holds cache configuration.
type Config struct {
	// Backend specifies the cache backend: "sqlite" or "memory"
	Backend string

	// Path is the database file path for SQLite backend.
	// Default: /var/cache/grpm/metadata/gentoo/cache.db
	Path string

	// MaxEntries is the maximum entries for memory backend (LRU eviction).
	// Default: 100000
	MaxEntries int

	// Version is the cache format version for migrations.
	Version int
}

// DefaultConfig returns the default cache configuration.
func DefaultConfig() *Config {
	return &Config{
		Backend:    "sqlite",
		Path:       "/var/cache/grpm/metadata/gentoo/cache.db",
		MaxEntries: 100000,
		Version:    1,
	}
}

// New creates a new cache instance based on configuration.
// Falls back to memory cache if SQLite is unavailable.
func New(cfg *Config) (Cache, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	switch cfg.Backend {
	case "sqlite":
		cache, err := NewSQLiteCache(cfg.Path)
		if err != nil {
			// Fallback to memory cache
			return NewMemoryCache(cfg.MaxEntries), nil
		}
		return cache, nil

	case "memory":
		return NewMemoryCache(cfg.MaxEntries), nil

	default:
		// Default to SQLite with fallback
		cache, err := NewSQLiteCache(cfg.Path)
		if err != nil {
			return NewMemoryCache(cfg.MaxEntries), nil
		}
		return cache, nil
	}
}

// NewWithFallback creates a cache with explicit fallback handling.
// Returns the primary cache and an error if fallback was used.
func NewWithFallback(cfg *Config) (Cache, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	cache, err := NewSQLiteCache(cfg.Path)
	if err != nil {
		return NewMemoryCache(cfg.MaxEntries), fmt.Errorf("SQLite unavailable, using memory cache: %w", err)
	}
	return cache, nil
}
