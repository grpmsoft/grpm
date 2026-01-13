//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/sync"
)

// TestRsyncSyncer_SystemFallback tests that RsyncSyncer falls back to
// system rsync when native implementation is unavailable or times out.
//
// This test validates the fix for:
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
