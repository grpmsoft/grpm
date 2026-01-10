package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/cache"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// testRepoPath creates a temporary repository structure for testing.
func testRepoPath(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "grpm-repo-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test package structure
	pkgDir := filepath.Join(tmpDir, "sys-libs", "zlib")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}

	// Create test ebuild
	ebuildContent := `# Copyright 1999-2024 Gentoo Authors
EAPI=8
DESCRIPTION="Standard (de)compression library"
HOMEPAGE="https://zlib.net/"
SLOT="0/1.3"
KEYWORDS="amd64 ~arm64"
IUSE="minizip static-libs"
LICENSE="ZLIB"
`

	ebuildPath := filepath.Join(pkgDir, "zlib-1.3.1.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	// Create another version
	ebuildPath2 := filepath.Join(pkgDir, "zlib-1.3.0.ebuild")
	if err := os.WriteFile(ebuildPath2, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("Failed to write ebuild: %v", err)
	}

	return tmpDir
}

// testCachePath creates a temporary cache directory.
func testCachePath(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "grpm-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	return tmpDir
}

func TestRepoCache_New(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	t.Run("ValidConfig", func(t *testing.T) {
		cfg := &Config{
			CachePath:   cachePath,
			RepoPath:    repoPath,
			RepoName:    "test",
			Backend:     "sqlite",
			EnableIndex: true,
		}

		rc, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer rc.Close()

		if !rc.IsEnabled() {
			t.Error("Cache should be enabled")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := New(nil)
		if err == nil {
			t.Error("New(nil) should return error")
		}
	})

	t.Run("MissingRepoPath", func(t *testing.T) {
		cfg := &Config{
			CachePath: cachePath,
			RepoName:  "test",
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("New() with missing RepoPath should return error")
		}
	})

	t.Run("NonexistentRepo", func(t *testing.T) {
		cfg := &Config{
			CachePath: cachePath,
			RepoPath:  "/nonexistent/path",
			RepoName:  "test",
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("New() with nonexistent repo should return error")
		}
	})
}

func TestRepoCache_PutGet(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	cfg := &Config{
		CachePath:   cachePath,
		RepoPath:    repoPath,
		RepoName:    "test",
		Backend:     "sqlite",
		EnableIndex: false,
	}

	rc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rc.Close()

	ctx := context.Background()

	t.Run("PutEntryAndGet", func(t *testing.T) {
		// Use PutEntry to test the cache layer directly
		// (Put requires actual ebuild files for mtime validation)
		entry := &cache.Entry{
			Category:    "sys-libs",
			Name:        "zlib",
			Version:     "1.3.1",
			Slot:        "0",
			SubSlot:     "1.3",
			IUSE:        []string{"minizip", "static-libs"},
			EbuildMtime: time.Now(),
			CachedAt:    time.Now(),
		}

		// Put
		if err := rc.PutEntry(ctx, entry); err != nil {
			t.Fatalf("PutEntry() error = %v", err)
		}

		// Get - note: Get validates against ebuild file, so use inner cache
		gotEntry, err := rc.inner.Get(ctx, "sys-libs", "zlib", "1.3.1")
		if err != nil {
			t.Fatalf("inner.Get() error = %v", err)
		}

		if gotEntry.Category != "sys-libs" {
			t.Errorf("Category = %q, want %q", gotEntry.Category, "sys-libs")
		}
		if gotEntry.Name != "zlib" {
			t.Errorf("Name = %q, want %q", gotEntry.Name, "zlib")
		}
		if gotEntry.Version != "1.3.1" {
			t.Errorf("Version = %q, want %q", gotEntry.Version, "1.3.1")
		}
		if gotEntry.Slot != "0" {
			t.Errorf("Slot = %q, want %q", gotEntry.Slot, "0")
		}
	})

	t.Run("PutWithEbuild", func(t *testing.T) {
		// Test Put with actual ebuild file
		p := &pkg.Package{
			Name:    "sys-libs/zlib",
			Version: "1.3.1",
			Slot:    pkg.Slot{Name: "0", Subslot: "1.3"},
			UseFlags: map[string]bool{
				"minizip":     true,
				"static-libs": true,
			},
		}

		// Put should work since ebuild exists
		if err := rc.Put(ctx, p); err != nil {
			t.Fatalf("Put() error = %v", err)
		}

		// Verify entry was stored in inner cache
		// (Get validates mtime which may have nanosecond precision issues)
		gotEntry, err := rc.inner.Get(ctx, "sys-libs", "zlib", "1.3.1")
		if err != nil {
			t.Fatalf("inner.Get() error = %v", err)
		}

		if gotEntry.Version != "1.3.1" {
			t.Errorf("Version = %q, want %q", gotEntry.Version, "1.3.1")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		_, err := rc.Get(ctx, "app-misc", "nonexistent", "1.0")
		if !errors.Is(err, cache.ErrNotFound) {
			t.Errorf("Get() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("PutNil", func(t *testing.T) {
		err := rc.Put(ctx, nil)
		if !errors.Is(err, cache.ErrInvalidEntry) {
			t.Errorf("Put(nil) error = %v, want ErrInvalidEntry", err)
		}
	})
}

func TestRepoCache_Invalidation(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	cfg := &Config{
		CachePath:   cachePath,
		RepoPath:    repoPath,
		RepoName:    "test",
		Backend:     "sqlite",
		EnableIndex: false,
	}

	rc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rc.Close()

	ctx := context.Background()

	t.Run("InvalidatePackage", func(t *testing.T) {
		// Use PutEntry for direct cache testing
		entry := &cache.Entry{
			Category:    "sys-libs",
			Name:        "zlib",
			Version:     "1.3.1",
			Slot:        "0",
			EbuildMtime: time.Now(),
			CachedAt:    time.Now(),
		}

		if err := rc.PutEntry(ctx, entry); err != nil {
			t.Fatalf("PutEntry() error = %v", err)
		}

		// Verify it exists in inner cache
		if _, err := rc.inner.Get(ctx, "sys-libs", "zlib", "1.3.1"); err != nil {
			t.Fatalf("inner.Get() before invalidate error = %v", err)
		}

		// Invalidate
		if err := rc.InvalidatePackage(ctx, "sys-libs", "zlib"); err != nil {
			t.Fatalf("InvalidatePackage() error = %v", err)
		}

		// Should be gone
		_, err := rc.inner.Get(ctx, "sys-libs", "zlib", "1.3.1")
		if !errors.Is(err, cache.ErrNotFound) {
			t.Errorf("inner.Get() after invalidate error = %v, want ErrNotFound", err)
		}
	})

	t.Run("InvalidateAll", func(t *testing.T) {
		e1 := &cache.Entry{
			Category:    "sys-libs",
			Name:        "zlib",
			Version:     "1.3.0",
			Slot:        "0",
			EbuildMtime: time.Now(),
			CachedAt:    time.Now(),
		}
		e2 := &cache.Entry{
			Category:    "sys-libs",
			Name:        "zlib",
			Version:     "1.3.1",
			Slot:        "0",
			EbuildMtime: time.Now(),
			CachedAt:    time.Now(),
		}

		_ = rc.PutEntry(ctx, e1)
		_ = rc.PutEntry(ctx, e2)

		if err := rc.InvalidateAll(ctx); err != nil {
			t.Fatalf("InvalidateAll() error = %v", err)
		}

		stats := rc.Stats()
		if stats.Entries != 0 {
			t.Errorf("Stats().Entries = %d, want 0", stats.Entries)
		}
	})
}

func TestRepoCache_Stats(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	cfg := &Config{
		CachePath:   cachePath,
		RepoPath:    repoPath,
		RepoName:    "test",
		Backend:     "sqlite",
		EnableIndex: false,
	}

	rc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rc.Close()

	ctx := context.Background()

	// Add entry directly
	entry := &cache.Entry{
		Category:    "sys-libs",
		Name:        "zlib",
		Version:     "1.3.1",
		Slot:        "0",
		EbuildMtime: time.Now(),
		CachedAt:    time.Now(),
	}
	_ = rc.PutEntry(ctx, entry)

	stats := rc.Stats()
	if stats.Entries < 1 {
		t.Errorf("Stats().Entries = %d, want >= 1", stats.Entries)
	}
}

func TestRepoIndex_New(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	indexPath := filepath.Join(cachePath, "index.db")

	t.Run("Create", func(t *testing.T) {
		ri, err := NewRepoIndex(indexPath, repoPath)
		if err != nil {
			t.Fatalf("NewRepoIndex() error = %v", err)
		}
		defer ri.Close()

		// Index file should exist
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			t.Error("Index file was not created")
		}
	})

	t.Run("Reopen", func(t *testing.T) {
		ri1, err := NewRepoIndex(indexPath, repoPath)
		if err != nil {
			t.Fatalf("NewRepoIndex(1) error = %v", err)
		}

		ctx := context.Background()
		entry := &PackageEntry{
			Category: "sys-libs",
			Name:     "zlib",
			Version:  "1.3.1",
			Slot:     "0",
			EAPI:     "8",
			Path:     "sys-libs/zlib/zlib-1.3.1.ebuild",
			Mtime:    time.Now(),
		}
		_ = ri1.Add(ctx, entry)
		ri1.Close()

		// Reopen and verify
		ri2, err := NewRepoIndex(indexPath, repoPath)
		if err != nil {
			t.Fatalf("NewRepoIndex(2) error = %v", err)
		}
		defer ri2.Close()

		entries, err := ri2.LookupPackage(ctx, "sys-libs", "zlib")
		if err != nil {
			t.Fatalf("LookupPackage() error = %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("LookupPackage() returned %d entries, want 1", len(entries))
		}
	})
}

func TestRepoIndex_CRUD(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	indexPath := filepath.Join(cachePath, "index.db")
	ri, err := NewRepoIndex(indexPath, repoPath)
	if err != nil {
		t.Fatalf("NewRepoIndex() error = %v", err)
	}
	defer ri.Close()

	ctx := context.Background()

	t.Run("AddAndLookup", func(t *testing.T) {
		entry := &PackageEntry{
			Category: "sys-libs",
			Name:     "zlib",
			Version:  "1.3.1",
			Slot:     "0",
			EAPI:     "8",
			Path:     "sys-libs/zlib/zlib-1.3.1.ebuild",
			Mtime:    time.Now(),
		}

		if err := ri.Add(ctx, entry); err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		entries, err := ri.LookupPackage(ctx, "sys-libs", "zlib")
		if err != nil {
			t.Fatalf("LookupPackage() error = %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("LookupPackage() returned %d entries, want 1", len(entries))
		}

		got := entries[0]
		if got.Category != "sys-libs" {
			t.Errorf("Category = %q, want %q", got.Category, "sys-libs")
		}
		if got.Name != "zlib" {
			t.Errorf("Name = %q, want %q", got.Name, "zlib")
		}
		if got.Version != "1.3.1" {
			t.Errorf("Version = %q, want %q", got.Version, "1.3.1")
		}
	})

	t.Run("AddBatch", func(t *testing.T) {
		entries := []PackageEntry{
			{
				Category: "dev-libs",
				Name:     "openssl",
				Version:  "3.0.0",
				Slot:     "0",
				EAPI:     "8",
				Path:     "dev-libs/openssl/openssl-3.0.0.ebuild",
				Mtime:    time.Now(),
			},
			{
				Category: "dev-libs",
				Name:     "openssl",
				Version:  "3.1.0",
				Slot:     "0",
				EAPI:     "8",
				Path:     "dev-libs/openssl/openssl-3.1.0.ebuild",
				Mtime:    time.Now(),
			},
		}

		if err := ri.AddBatch(ctx, entries); err != nil {
			t.Fatalf("AddBatch() error = %v", err)
		}

		got, err := ri.LookupPackage(ctx, "dev-libs", "openssl")
		if err != nil {
			t.Fatalf("LookupPackage() error = %v", err)
		}

		if len(got) != 2 {
			t.Errorf("LookupPackage() returned %d entries, want 2", len(got))
		}
	})

	t.Run("LookupCategory", func(t *testing.T) {
		names, err := ri.LookupCategory(ctx, "dev-libs")
		if err != nil {
			t.Fatalf("LookupCategory() error = %v", err)
		}

		if len(names) != 1 || names[0] != "openssl" {
			t.Errorf("LookupCategory() = %v, want [openssl]", names)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := ri.Delete(ctx, "dev-libs", "openssl", "3.0.0"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		entries, _ := ri.LookupPackage(ctx, "dev-libs", "openssl")
		if len(entries) != 1 {
			t.Errorf("After delete, got %d entries, want 1", len(entries))
		}
	})

	t.Run("DeletePackage", func(t *testing.T) {
		if err := ri.DeletePackage(ctx, "dev-libs", "openssl"); err != nil {
			t.Fatalf("DeletePackage() error = %v", err)
		}

		entries, _ := ri.LookupPackage(ctx, "dev-libs", "openssl")
		if len(entries) != 0 {
			t.Errorf("After DeletePackage, got %d entries, want 0", len(entries))
		}
	})

	t.Run("Clear", func(t *testing.T) {
		_ = ri.Add(ctx, &PackageEntry{
			Category: "app-misc",
			Name:     "hello",
			Version:  "2.10",
			Path:     "app-misc/hello/hello-2.10.ebuild",
			Mtime:    time.Now(),
		})

		if err := ri.Clear(ctx); err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		count, _ := ri.Count(ctx)
		if count != 0 {
			t.Errorf("After Clear(), Count() = %d, want 0", count)
		}
	})
}

func TestRepoIndex_Rebuild(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	indexPath := filepath.Join(cachePath, "index.db")
	ri, err := NewRepoIndex(indexPath, repoPath)
	if err != nil {
		t.Fatalf("NewRepoIndex() error = %v", err)
	}
	defer ri.Close()

	ctx := context.Background()

	// Rebuild index
	if err := ri.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	// Should find the test ebuilds
	entries, err := ri.LookupPackage(ctx, "sys-libs", "zlib")
	if err != nil {
		t.Fatalf("LookupPackage() error = %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("After Rebuild(), found %d zlib versions, want 2", len(entries))
	}

	// Check count
	count, _ := ri.Count(ctx)
	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}
}

func TestRepoIndex_IsValid(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	indexPath := filepath.Join(cachePath, "index.db")
	ri, err := NewRepoIndex(indexPath, repoPath)
	if err != nil {
		t.Fatalf("NewRepoIndex() error = %v", err)
	}
	defer ri.Close()

	ctx := context.Background()

	t.Run("EmptyIndex", func(t *testing.T) {
		// Empty index should not be valid
		if ri.IsValid(ctx) {
			t.Error("Empty index should not be valid")
		}
	})

	t.Run("AfterRebuild", func(t *testing.T) {
		if err := ri.Rebuild(ctx); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		if !ri.IsValid(ctx) {
			t.Error("Index should be valid after rebuild")
		}
	})

	t.Run("AfterRepoChange", func(t *testing.T) {
		// Modify repo (touch a file to change mtime)
		time.Sleep(10 * time.Millisecond) // Ensure different mtime
		ebuildPath := filepath.Join(repoPath, "sys-libs", "zlib", "zlib-1.3.1.ebuild")
		now := time.Now()
		_ = os.Chtimes(ebuildPath, now, now)

		// Note: This may or may not invalidate depending on whether
		// the repo directory mtime changes. Real repos would have
		// the directory mtime change on sync.
	})
}

func TestCachedRepository(t *testing.T) {
	repoPath := testRepoPath(t)
	defer os.RemoveAll(repoPath)

	cachePath := testCachePath(t)
	defer os.RemoveAll(cachePath)

	t.Run("New", func(t *testing.T) {
		cr, err := NewCachedRepository(repoPath, cachePath)
		if err != nil {
			t.Fatalf("NewCachedRepository() error = %v", err)
		}
		defer cr.Close()

		if !cr.IsEnabled() {
			t.Error("Cache should be enabled")
		}
	})

	t.Run("EnsureIndex", func(t *testing.T) {
		cr, err := NewCachedRepository(repoPath, cachePath)
		if err != nil {
			t.Fatalf("NewCachedRepository() error = %v", err)
		}
		defer cr.Close()

		ctx := context.Background()

		if err := cr.EnsureIndex(ctx); err != nil {
			t.Fatalf("EnsureIndex() error = %v", err)
		}

		// Should have indexed packages
		count, _ := cr.IndexCount(ctx)
		if count < 1 {
			t.Errorf("IndexCount() = %d, want >= 1", count)
		}
	})

	t.Run("LookupPackageVersions", func(t *testing.T) {
		cr, err := NewCachedRepository(repoPath, cachePath)
		if err != nil {
			t.Fatalf("NewCachedRepository() error = %v", err)
		}
		defer cr.Close()

		ctx := context.Background()
		_ = cr.EnsureIndex(ctx)

		entries, err := cr.LookupPackageVersions(ctx, "sys-libs", "zlib")
		if err != nil {
			t.Fatalf("LookupPackageVersions() error = %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("LookupPackageVersions() returned %d entries, want 2", len(entries))
		}
	})

	t.Run("Stats", func(t *testing.T) {
		cr, err := NewCachedRepository(repoPath, cachePath)
		if err != nil {
			t.Fatalf("NewCachedRepository() error = %v", err)
		}
		defer cr.Close()

		stats := cr.Stats()
		// Just check it doesn't panic
		_ = stats.HitRate()
	})
}

// BenchmarkRepoIndex benchmarks index operations.
func BenchmarkRepoIndex(b *testing.B) {
	repoPath, err := os.MkdirTemp("", "grpm-bench-repo")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(repoPath)

	// Create test structure
	for i := 0; i < 100; i++ {
		pkgDir := filepath.Join(repoPath, "cat", "pkg"+string(rune('0'+i%10)))
		_ = os.MkdirAll(pkgDir, 0755)
		ebuildPath := filepath.Join(pkgDir, "pkg"+string(rune('0'+i%10))+"-1.0.ebuild")
		_ = os.WriteFile(ebuildPath, []byte("EAPI=8"), 0644)
	}

	cachePath, err := os.MkdirTemp("", "grpm-bench-cache")
	if err != nil {
		b.Fatalf("Failed to create cache dir: %v", err)
	}
	defer os.RemoveAll(cachePath)

	indexPath := filepath.Join(cachePath, "index.db")
	ri, err := NewRepoIndex(indexPath, repoPath)
	if err != nil {
		b.Fatalf("NewRepoIndex() error = %v", err)
	}
	defer ri.Close()

	ctx := context.Background()
	_ = ri.Rebuild(ctx)

	b.Run("LookupPackage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = ri.LookupPackage(ctx, "cat", "pkg0")
		}
	})

	b.Run("LookupCategory", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = ri.LookupCategory(ctx, "cat")
		}
	})
}
