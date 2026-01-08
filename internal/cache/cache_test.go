package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testEntry creates a test Entry with given package info.
func testEntry(category, name, version string) *Entry {
	return &Entry{
		Category:    category,
		Name:        name,
		Version:     version,
		EAPI:        "8",
		Slot:        "0",
		SubSlot:     version,
		Keywords:    []string{"amd64", "~arm64"},
		IUSE:        []string{"static-libs", "minizip"},
		Use:         []string{},
		License:     "ZLIB",
		Description: "Standard (de)compression library",
		Homepage:    "https://zlib.net/",
		Depend:      "",
		RDepend:     "",
		BDepend:     "",
		PDepend:     "",
		SrcURI:      []string{"https://zlib.net/zlib-" + version + ".tar.gz"},
		EbuildMtime: time.Now().Add(-24 * time.Hour), // 24 hours ago
		CachedAt:    time.Now(),
	}
}

// TestEntry tests Entry methods.
func TestEntry(t *testing.T) {
	e := testEntry("sys-libs", "zlib", "1.2.13")

	t.Run("Key", func(t *testing.T) {
		expected := "sys-libs/zlib-1.2.13"
		if got := e.Key(); got != expected {
			t.Errorf("Key() = %q, want %q", got, expected)
		}
	})

	t.Run("Atom", func(t *testing.T) {
		expected := "sys-libs/zlib"
		if got := e.Atom(); got != expected {
			t.Errorf("Atom() = %q, want %q", got, expected)
		}
	})

	t.Run("IsValid", func(t *testing.T) {
		// Should be valid when mtime matches
		if !e.IsValid(e.EbuildMtime) {
			t.Error("IsValid() = false for matching mtime")
		}

		// Should be invalid when mtime differs
		if e.IsValid(time.Now()) {
			t.Error("IsValid() = true for different mtime")
		}
	})

	t.Run("IsExpired", func(t *testing.T) {
		// Should not be expired with long duration
		if e.IsExpired(7 * 24 * time.Hour) {
			t.Error("IsExpired() = true for 7 day duration")
		}

		// Create old entry
		oldEntry := testEntry("sys-libs", "zlib", "1.2.12")
		oldEntry.CachedAt = time.Now().Add(-48 * time.Hour)

		// Should be expired with short duration
		if !oldEntry.IsExpired(24 * time.Hour) {
			t.Error("IsExpired() = false for 24 hour duration on 48 hour old entry")
		}
	})
}

// TestStats tests Stats methods.
func TestStats(t *testing.T) {
	t.Run("HitRate_Zero", func(t *testing.T) {
		s := Stats{Hits: 0, Misses: 0}
		if got := s.HitRate(); got != 0.0 {
			t.Errorf("HitRate() = %f, want 0.0", got)
		}
	})

	t.Run("HitRate_50Percent", func(t *testing.T) {
		s := Stats{Hits: 50, Misses: 50}
		if got := s.HitRate(); got != 50.0 {
			t.Errorf("HitRate() = %f, want 50.0", got)
		}
	})

	t.Run("HitRate_100Percent", func(t *testing.T) {
		s := Stats{Hits: 100, Misses: 0}
		if got := s.HitRate(); got != 100.0 {
			t.Errorf("HitRate() = %f, want 100.0", got)
		}
	})
}

// cacheTests defines common tests for all Cache implementations.
func cacheTests(t *testing.T, c Cache) {
	ctx := context.Background()

	t.Run("PutGet", func(t *testing.T) {
		entry := testEntry("sys-libs", "zlib", "1.2.13")
		if err := c.Put(ctx, entry); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		got, err := c.Get(ctx, "sys-libs", "zlib", "1.2.13")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if got.Category != entry.Category || got.Name != entry.Name || got.Version != entry.Version {
			t.Errorf("Get() returned wrong entry: %+v", got)
		}
		if got.EAPI != entry.EAPI {
			t.Errorf("EAPI = %q, want %q", got.EAPI, entry.EAPI)
		}
		if len(got.Keywords) != len(entry.Keywords) {
			t.Errorf("Keywords length = %d, want %d", len(got.Keywords), len(entry.Keywords))
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		_, err := c.Get(ctx, "app-misc", "nonexistent", "1.0")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("PutNil", func(t *testing.T) {
		err := c.Put(ctx, nil)
		if !errors.Is(err, ErrInvalidEntry) {
			t.Errorf("Put(nil) error = %v, want ErrInvalidEntry", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		entry := testEntry("app-misc", "hello", "2.10")
		if err := c.Put(ctx, entry); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		// Verify it exists
		if _, err := c.Get(ctx, "app-misc", "hello", "2.10"); err != nil {
			t.Fatalf("Get() before delete error = %v", err)
		}

		// Delete it
		if err := c.Delete(ctx, "app-misc", "hello", "2.10"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Verify it's gone
		_, err := c.Get(ctx, "app-misc", "hello", "2.10")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get() after delete error = %v, want ErrNotFound", err)
		}
	})

	t.Run("DeleteNonexistent", func(t *testing.T) {
		// Should not error when deleting nonexistent entry
		err := c.Delete(ctx, "app-misc", "nonexistent", "1.0")
		if err != nil {
			t.Errorf("Delete() error = %v, want nil", err)
		}
	})

	t.Run("PutBatch", func(t *testing.T) {
		entries := []*Entry{
			testEntry("dev-libs", "openssl", "3.0.0"),
			testEntry("dev-libs", "openssl", "3.1.0"),
			testEntry("dev-libs", "openssl", "3.2.0"),
		}

		if err := c.PutBatch(ctx, entries); err != nil {
			t.Fatalf("PutBatch() error = %v", err)
		}

		// Verify all entries exist
		for _, e := range entries {
			got, err := c.Get(ctx, e.Category, e.Name, e.Version)
			if err != nil {
				t.Errorf("Get(%s) error = %v", e.Key(), err)
				continue
			}
			if got.Version != e.Version {
				t.Errorf("Get(%s) version = %q, want %q", e.Key(), got.Version, e.Version)
			}
		}
	})

	t.Run("PutBatchEmpty", func(t *testing.T) {
		err := c.PutBatch(ctx, []*Entry{})
		if err != nil {
			t.Errorf("PutBatch([]) error = %v, want nil", err)
		}
	})

	t.Run("Invalidate", func(t *testing.T) {
		// Add entries with different mtimes
		oldEntry := testEntry("dev-libs", "gmp", "6.2.0")
		oldEntry.EbuildMtime = time.Now().Add(-72 * time.Hour)
		if err := c.Put(ctx, oldEntry); err != nil {
			t.Fatalf("Put(old) error = %v", err)
		}

		newEntry := testEntry("dev-libs", "gmp", "6.3.0")
		newEntry.EbuildMtime = time.Now()
		if err := c.Put(ctx, newEntry); err != nil {
			t.Fatalf("Put(new) error = %v", err)
		}

		// Invalidate entries older than 48 hours ago
		threshold := time.Now().Add(-48 * time.Hour)
		if err := c.Invalidate(ctx, "dev-libs", "gmp", threshold); err != nil {
			t.Fatalf("Invalidate() error = %v", err)
		}

		// Old entry should be gone
		_, err := c.Get(ctx, "dev-libs", "gmp", "6.2.0")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(old) error = %v, want ErrNotFound", err)
		}

		// New entry should still exist
		_, err = c.Get(ctx, "dev-libs", "gmp", "6.3.0")
		if err != nil {
			t.Errorf("Get(new) error = %v, want nil", err)
		}
	})

	t.Run("InvalidateAll", func(t *testing.T) {
		// Add some entries
		if err := c.Put(ctx, testEntry("sys-apps", "grep", "3.11")); err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		if err := c.Put(ctx, testEntry("sys-apps", "sed", "4.9")); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		// Clear all
		if err := c.InvalidateAll(ctx); err != nil {
			t.Fatalf("InvalidateAll() error = %v", err)
		}

		// Verify cache is empty
		stats := c.Stats()
		if stats.Entries != 0 {
			t.Errorf("Stats().Entries = %d, want 0", stats.Entries)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		// Reset cache
		_ = c.InvalidateAll(ctx)

		// Add entries
		if err := c.Put(ctx, testEntry("sys-libs", "ncurses", "6.4")); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		// Generate hits and misses
		_, _ = c.Get(ctx, "sys-libs", "ncurses", "6.4")  // hit
		_, _ = c.Get(ctx, "sys-libs", "ncurses", "6.4")  // hit
		_, _ = c.Get(ctx, "sys-libs", "nonexistent", "") // miss

		stats := c.Stats()
		if stats.Hits < 2 {
			t.Errorf("Stats().Hits = %d, want >= 2", stats.Hits)
		}
		if stats.Misses < 1 {
			t.Errorf("Stats().Misses = %d, want >= 1", stats.Misses)
		}
		if stats.Entries < 1 {
			t.Errorf("Stats().Entries = %d, want >= 1", stats.Entries)
		}
	})

	t.Run("ClosedCache", func(t *testing.T) {
		// Skip for the shared cache - test separately
		t.Skip("Tested in individual backend tests")
	})
}

// TestMemoryCache tests the in-memory cache implementation.
func TestMemoryCache(t *testing.T) {
	c := NewMemoryCache(1000)
	defer c.Close()

	cacheTests(t, c)

	t.Run("LRUEviction", func(t *testing.T) {
		small := NewMemoryCache(3) // Small cache for testing eviction
		ctx := context.Background()

		// Add 3 entries (fills cache)
		_ = small.Put(ctx, testEntry("cat", "pkg1", "1.0"))
		_ = small.Put(ctx, testEntry("cat", "pkg2", "1.0"))
		_ = small.Put(ctx, testEntry("cat", "pkg3", "1.0"))

		// Access pkg1 to make it most recently used
		_, _ = small.Get(ctx, "cat", "pkg1", "1.0")

		// Add new entry (should evict pkg2, the LRU)
		_ = small.Put(ctx, testEntry("cat", "pkg4", "1.0"))

		// pkg2 should be evicted
		_, err := small.Get(ctx, "cat", "pkg2", "1.0")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(pkg2) error = %v, want ErrNotFound (should be evicted)", err)
		}

		// pkg1, pkg3, pkg4 should still exist
		if _, err := small.Get(ctx, "cat", "pkg1", "1.0"); err != nil {
			t.Errorf("Get(pkg1) error = %v, want nil", err)
		}
		if _, err := small.Get(ctx, "cat", "pkg3", "1.0"); err != nil {
			t.Errorf("Get(pkg3) error = %v, want nil", err)
		}
		if _, err := small.Get(ctx, "cat", "pkg4", "1.0"); err != nil {
			t.Errorf("Get(pkg4) error = %v, want nil", err)
		}

		small.Close()
	})

	t.Run("ClosedCache", func(t *testing.T) {
		closed := NewMemoryCache(100)
		closed.Close()

		ctx := context.Background()

		if _, err := closed.Get(ctx, "cat", "pkg", "1.0"); !errors.Is(err, ErrCacheClosed) {
			t.Errorf("Get() on closed cache = %v, want ErrCacheClosed", err)
		}
		if err := closed.Put(ctx, testEntry("cat", "pkg", "1.0")); !errors.Is(err, ErrCacheClosed) {
			t.Errorf("Put() on closed cache = %v, want ErrCacheClosed", err)
		}
	})

	t.Run("DefaultMaxEntries", func(t *testing.T) {
		c := NewMemoryCache(0) // Should default to 100000
		defer c.Close()

		if c.maxEntries != 100000 {
			t.Errorf("maxEntries = %d, want 100000", c.maxEntries)
		}
	})
}

// TestSQLiteCache tests the SQLite cache implementation.
func TestSQLiteCache(t *testing.T) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "grpm-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test-cache.db")
	c, err := NewSQLiteCache(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteCache() error = %v", err)
	}
	defer c.Close()

	cacheTests(t, c)

	t.Run("ClosedCache", func(t *testing.T) {
		closedDBPath := filepath.Join(tmpDir, "closed-cache.db")
		closed, err := NewSQLiteCache(closedDBPath)
		if err != nil {
			t.Fatalf("NewSQLiteCache() error = %v", err)
		}
		closed.Close()

		ctx := context.Background()

		if _, err := closed.Get(ctx, "cat", "pkg", "1.0"); !errors.Is(err, ErrCacheClosed) {
			t.Errorf("Get() on closed cache = %v, want ErrCacheClosed", err)
		}
		if err := closed.Put(ctx, testEntry("cat", "pkg", "1.0")); !errors.Is(err, ErrCacheClosed) {
			t.Errorf("Put() on closed cache = %v, want ErrCacheClosed", err)
		}
	})

	t.Run("Persistence", func(t *testing.T) {
		persistDBPath := filepath.Join(tmpDir, "persist-cache.db")

		// Create cache and add entry
		c1, err := NewSQLiteCache(persistDBPath)
		if err != nil {
			t.Fatalf("NewSQLiteCache(1) error = %v", err)
		}

		ctx := context.Background()
		if err := c1.Put(ctx, testEntry("sys-libs", "persist", "1.0")); err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		c1.Close()

		// Reopen cache and verify entry exists
		c2, err := NewSQLiteCache(persistDBPath)
		if err != nil {
			t.Fatalf("NewSQLiteCache(2) error = %v", err)
		}
		defer c2.Close()

		got, err := c2.Get(ctx, "sys-libs", "persist", "1.0")
		if err != nil {
			t.Fatalf("Get() after reopen error = %v", err)
		}
		if got.Name != "persist" {
			t.Errorf("Get() name = %q, want %q", got.Name, "persist")
		}
	})
}

// TestNew tests the cache factory function.
func TestNew(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grpm-cache-factory")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("DefaultConfig", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.Backend != "sqlite" {
			t.Errorf("Backend = %q, want %q", cfg.Backend, "sqlite")
		}
		if cfg.MaxEntries != 100000 {
			t.Errorf("MaxEntries = %d, want %d", cfg.MaxEntries, 100000)
		}
	})

	t.Run("MemoryBackend", func(t *testing.T) {
		cfg := &Config{Backend: "memory", MaxEntries: 1000}
		c, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer c.Close()

		// Verify it's a memory cache by checking stats
		stats := c.Stats()
		if stats.Entries != 0 {
			t.Errorf("Initial entries = %d, want 0", stats.Entries)
		}
	})

	t.Run("SQLiteBackend", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "factory-sqlite.db")
		cfg := &Config{Backend: "sqlite", Path: dbPath}
		c, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer c.Close()

		// Verify database file was created
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("SQLite database file was not created")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		// Should use defaults - may fall back to memory if SQLite path fails
		c, err := New(nil)
		if err != nil {
			t.Fatalf("New(nil) error = %v", err)
		}
		defer c.Close()
	})
}

// TestNewWithFallback tests the factory with explicit fallback handling.
func TestNewWithFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grpm-cache-fallback")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("SQLiteSuccess", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "fallback-success.db")
		cfg := &Config{Backend: "sqlite", Path: dbPath, MaxEntries: 1000}
		c, err := NewWithFallback(cfg)
		if err != nil {
			t.Fatalf("NewWithFallback() error = %v", err)
		}
		defer c.Close()
	})
}

// BenchmarkMemoryCache benchmarks memory cache operations.
func BenchmarkMemoryCache(b *testing.B) {
	c := NewMemoryCache(100000)
	defer c.Close()
	ctx := context.Background()

	// Pre-populate with some entries
	for i := 0; i < 1000; i++ {
		e := testEntry("cat", "pkg", "1.0")
		e.Version = "1.0." + string(rune('0'+i%10))
		_ = c.Put(ctx, e)
	}

	b.Run("Put", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e := testEntry("bench", "pkg", "1.0")
			_ = c.Put(ctx, e)
		}
	})

	b.Run("Get", func(b *testing.B) {
		e := testEntry("bench-get", "pkg", "1.0")
		_ = c.Put(ctx, e)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = c.Get(ctx, "bench-get", "pkg", "1.0")
		}
	})
}

// BenchmarkSQLiteCache benchmarks SQLite cache operations.
func BenchmarkSQLiteCache(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "grpm-cache-bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "bench-cache.db")
	c, err := NewSQLiteCache(dbPath)
	if err != nil {
		b.Fatalf("NewSQLiteCache() error = %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	b.Run("Put", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e := testEntry("bench", "pkg", "1.0")
			e.Version = "1.0." + string(rune('0'+i%10))
			_ = c.Put(ctx, e)
		}
	})

	b.Run("Get", func(b *testing.B) {
		e := testEntry("bench-get", "pkg", "1.0")
		_ = c.Put(ctx, e)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = c.Get(ctx, "bench-get", "pkg", "1.0")
		}
	})

	b.Run("PutBatch100", func(b *testing.B) {
		entries := make([]*Entry, 100)
		for i := range entries {
			entries[i] = testEntry("batch", "pkg", "1.0")
			entries[i].Version = "1.0." + string(rune('0'+i%10))
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = c.PutBatch(ctx, entries)
		}
	})
}
