package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryCache implements Cache using an in-memory map with LRU eviction.
// Thread-safe for concurrent access.
type MemoryCache struct {
	maxEntries int
	closed     atomic.Bool

	mu      sync.RWMutex
	entries map[string]*memoryEntry // key = "category/name-version"
	order   []string                // LRU order: oldest first

	// Statistics
	hits   int64
	misses int64
}

// memoryEntry wraps an Entry with access tracking.
type memoryEntry struct {
	entry      *Entry
	lastAccess time.Time
}

// NewMemoryCache creates a new in-memory cache.
//
// maxEntries limits the number of cached entries. When exceeded,
// least recently used entries are evicted.
// If maxEntries <= 0, defaults to 100000.
func NewMemoryCache(maxEntries int) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = 100000
	}

	return &MemoryCache{
		maxEntries: maxEntries,
		entries:    make(map[string]*memoryEntry),
		order:      make([]string, 0),
	}
}

// makeKey creates a cache key from category, name, and version.
func makeKey(category, name, version string) string {
	return category + "/" + name + "-" + version
}

// Get retrieves cached metadata for a package.
func (c *MemoryCache) Get(ctx context.Context, category, name, version string) (*Entry, error) {
	if c.closed.Load() {
		return nil, ErrCacheClosed
	}

	key := makeKey(category, name, version)

	c.mu.RLock()
	me, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&c.misses, 1)
		return nil, ErrNotFound
	}

	// Update access time (requires write lock)
	c.mu.Lock()
	me.lastAccess = time.Now()
	c.moveToEnd(key)
	c.mu.Unlock()

	atomic.AddInt64(&c.hits, 1)

	// Return a copy to prevent external mutation
	entryCopy := *me.entry
	return &entryCopy, nil
}

// Put stores metadata for a package.
func (c *MemoryCache) Put(ctx context.Context, entry *Entry) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	if entry == nil {
		return ErrInvalidEntry
	}

	// Make a copy to prevent external mutation
	entryCopy := *entry
	if entryCopy.CachedAt.IsZero() {
		entryCopy.CachedAt = time.Now()
	}

	key := entryCopy.Key()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists
	if _, exists := c.entries[key]; exists {
		c.entries[key] = &memoryEntry{
			entry:      &entryCopy,
			lastAccess: time.Now(),
		}
		c.moveToEnd(key)
		return nil
	}

	// Check if we need to evict
	for len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	// Add new entry
	c.entries[key] = &memoryEntry{
		entry:      &entryCopy,
		lastAccess: time.Now(),
	}
	c.order = append(c.order, key)

	return nil
}

// PutBatch stores multiple entries efficiently.
func (c *MemoryCache) PutBatch(ctx context.Context, entries []*Entry) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	for _, entry := range entries {
		if err := c.Put(ctx, entry); err != nil {
			return err
		}
	}

	return nil
}

// Delete removes cached metadata for a package.
func (c *MemoryCache) Delete(ctx context.Context, category, name, version string) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	key := makeKey(category, name, version)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		return nil // Not an error if entry doesn't exist
	}

	delete(c.entries, key)
	c.removeFromOrder(key)

	return nil
}

// Invalidate removes all entries older than given mtime for a package.
func (c *MemoryCache) Invalidate(ctx context.Context, category, name string, mtime time.Time) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	prefix := category + "/" + name + "-"

	c.mu.Lock()
	defer c.mu.Unlock()

	var keysToDelete []string
	for key, me := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			if me.entry.EbuildMtime.Before(mtime) {
				keysToDelete = append(keysToDelete, key)
			}
		}
	}

	for _, key := range keysToDelete {
		delete(c.entries, key)
		c.removeFromOrder(key)
	}

	return nil
}

// InvalidateAll removes all entries from the cache.
func (c *MemoryCache) InvalidateAll(ctx context.Context) error {
	if c.closed.Load() {
		return ErrCacheClosed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*memoryEntry)
	c.order = make([]string, 0)
	c.hits = 0
	c.misses = 0

	return nil
}

// Stats returns cache statistics.
func (c *MemoryCache) Stats() Stats {
	c.mu.RLock()
	entryCount := int64(len(c.entries))
	c.mu.RUnlock()

	// Estimate size: rough approximation
	// Each entry ~500 bytes average
	estimatedSize := entryCount * 500

	return Stats{
		Hits:       atomic.LoadInt64(&c.hits),
		Misses:     atomic.LoadInt64(&c.misses),
		Entries:    entryCount,
		Size:       estimatedSize,
		LastUpdate: time.Now(),
	}
}

// Close closes the cache and releases resources.
func (c *MemoryCache) Close() error {
	if c.closed.Swap(true) {
		return nil // Already closed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = nil
	c.order = nil

	return nil
}

// moveToEnd moves a key to the end of the LRU order (most recently used).
// Must be called with write lock held.
func (c *MemoryCache) moveToEnd(key string) {
	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

// removeFromOrder removes a key from the LRU order.
// Must be called with write lock held.
func (c *MemoryCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// evictOldest removes the oldest entry from the cache.
// Must be called with write lock held.
func (c *MemoryCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}

	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldest)
}
