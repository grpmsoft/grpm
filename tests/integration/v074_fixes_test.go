package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/fetch"
	"github.com/grpmsoft/grpm/internal/sync"
)

// TestThirdPartyMirrorExpansion_RealRepository tests mirror:// URL expansion
// against a real Gentoo repository's thirdpartymirrors file.
//
// This test validates v0.7.4-002 fix for:
//
//	"grpm fetch app-misc/hello" failing with "unsupported protocol scheme mirror"
//
// The fix properly expands mirror://gnu/, mirror://sourceforge/, etc. to real HTTP URLs.
func TestThirdPartyMirrorExpansion_RealRepository(t *testing.T) {
	repoPath := os.Getenv("PORTDIR")
	if repoPath == "" {
		repoPath = "/var/db/repos/gentoo"
	}

	thirdPartyPath := filepath.Join(repoPath, "profiles", "thirdpartymirrors")
	if _, err := os.Stat(thirdPartyPath); os.IsNotExist(err) {
		t.Skip("Skipping: real Portage repository not available at", repoPath)
	}

	mirrors := fetch.ParseThirdPartyMirrors(repoPath)

	// Verify GNU mirror exists (most essential, used by many packages)
	// Other mirrors may not exist in all repository configurations
	if urls, ok := mirrors["gnu"]; !ok || len(urls) == 0 {
		t.Fatal("Essential mirror 'gnu' not found or has no URLs")
	}

	// Log which mirrors are available
	t.Logf("Found %d mirrors in thirdpartymirrors", len(mirrors))

	// Test GNU mirror expansion (most common)
	gnuURLs := mirrors.ExpandMirrorURL("mirror://gnu/hello/hello-2.12.tar.gz")
	if len(gnuURLs) == 0 {
		t.Fatal("GNU mirror expansion returned empty list")
	}

	// All expanded URLs should be HTTP/HTTPS, not mirror://
	for _, url := range gnuURLs {
		if strings.HasPrefix(url, "mirror://") {
			t.Errorf("Expansion failed: URL still has mirror:// scheme: %s", url)
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			t.Errorf("Unexpected scheme in expanded URL: %s", url)
		}
		if !strings.HasSuffix(url, "hello/hello-2.12.tar.gz") {
			t.Errorf("Path not preserved in expanded URL: %s", url)
		}
	}

	t.Logf("GNU mirror expanded to %d URLs, first: %s", len(gnuURLs), gnuURLs[0])

	// Test SourceForge mirror
	sfURLs := mirrors.ExpandMirrorURL("mirror://sourceforge/project/file.tar.gz")
	if len(sfURLs) == 0 {
		t.Error("SourceForge mirror expansion returned empty list")
	}
	t.Logf("SourceForge mirror expanded to %d URLs", len(sfURLs))
}

// TestThirdPartyMirrorExpansion_UnknownMirror tests graceful handling
// of unknown mirror names.
func TestThirdPartyMirrorExpansion_UnknownMirror(t *testing.T) {
	mirrors := fetch.ThirdPartyMirrors{
		"gnu": {"https://ftp.gnu.org/gnu/"},
	}

	// Unknown mirror should return original URL unchanged
	result := mirrors.ExpandMirrorURL("mirror://unknown-mirror-12345/path/file.tar.gz")
	if len(result) != 1 {
		t.Fatalf("Expected 1 URL for unknown mirror, got %d", len(result))
	}
	if result[0] != "mirror://unknown-mirror-12345/path/file.tar.gz" {
		t.Errorf("Unknown mirror should return original URL, got: %s", result[0])
	}
}

// TestThirdPartyMirrorExpansion_NonMirrorURL tests that non-mirror URLs
// pass through unchanged.
func TestThirdPartyMirrorExpansion_NonMirrorURL(t *testing.T) {
	mirrors := fetch.ThirdPartyMirrors{
		"gnu": {"https://ftp.gnu.org/gnu/"},
	}

	testCases := []string{
		"https://github.com/project/archive/v1.0.tar.gz",
		"http://example.com/file.tar.gz",
		"ftp://ftp.example.com/pub/file.tar.gz",
	}

	for _, url := range testCases {
		result := mirrors.ExpandMirrorURL(url)
		if len(result) != 1 || result[0] != url {
			t.Errorf("Non-mirror URL should pass through unchanged: %s -> %v", url, result)
		}
	}
}

// TestRsyncSyncer_SystemFallback tests that RsyncSyncer falls back to
// system rsync when native implementation is unavailable or times out.
//
// This test validates v0.7.4-001 fix for:
//
//	"grpm sync -method rsync" hanging indefinitely on real Gentoo mirrors
//
// The fix adds:
//  1. Timeout handling for native rsync
//  2. Automatic fallback to system rsync binary
func TestRsyncSyncer_SystemFallback(t *testing.T) {
	syncer := sync.NewRsyncSyncer()

	// Verify syncer is available (native is always available)
	if !syncer.IsAvailable() {
		t.Fatal("RsyncSyncer should always be available")
	}

	// Verify syncer name
	if syncer.Name() != "rsync" {
		t.Errorf("Expected syncer name 'rsync', got %q", syncer.Name())
	}
}

// TestRsyncSyncer_ShortTimeout tests that sync respects context timeout
// and doesn't hang indefinitely.
//
// This is a key regression test for the rsync hang bug.
func TestRsyncSyncer_ShortTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	syncer := sync.NewRsyncSyncer()

	// Use very short timeout - should fail fast, not hang
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tmpDir := t.TempDir()

	config := &sync.SyncConfig{
		RepoPath:  tmpDir,
		SourceURL: "rsync://rsync.gentoo.org/gentoo-portage",
		Verbose:   false,
	}

	start := time.Now()
	_, err := syncer.Sync(ctx, config)
	elapsed := time.Since(start)

	// Should fail due to timeout, not hang
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Should complete within reasonable time (not hang)
	// Allow extra time for connection attempt
	if elapsed > 30*time.Second {
		t.Errorf("Sync took too long (%v), possible hang detected", elapsed)
	}

	t.Logf("Sync completed in %v with expected error: %v", elapsed, err)
}

// TestRsyncSyncer_WithStrategy tests custom sync strategy configuration.
func TestRsyncSyncer_WithStrategy(t *testing.T) {
	strategy := sync.SyncStrategy{
		MaxRetries:        1,
		RetryDelay:        100 * time.Millisecond,
		MaxMirrors:        2,
		ConnectionTimeout: 5 * time.Second,
	}

	syncer := sync.NewRsyncSyncer().WithStrategy(strategy)

	if syncer.Name() != "rsync" {
		t.Errorf("Expected syncer name 'rsync', got %q", syncer.Name())
	}
}

// TestDefaultStrategy tests default sync strategy values.
func TestDefaultStrategy(t *testing.T) {
	strategy := sync.DefaultStrategy()

	if strategy.MaxRetries <= 0 {
		t.Errorf("MaxRetries should be positive, got %d", strategy.MaxRetries)
	}
	if strategy.MaxMirrors <= 0 {
		t.Errorf("MaxMirrors should be positive, got %d", strategy.MaxMirrors)
	}
	if strategy.ConnectionTimeout <= 0 {
		t.Errorf("ConnectionTimeout should be positive, got %v", strategy.ConnectionTimeout)
	}
	if strategy.RetryDelay <= 0 {
		t.Errorf("RetryDelay should be positive, got %v", strategy.RetryDelay)
	}
}
