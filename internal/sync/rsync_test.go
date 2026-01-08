package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRsyncSyncer_Name tests syncer name
func TestRsyncSyncer_Name(t *testing.T) {
	syncer := NewRsyncSyncer()
	if syncer.Name() != "rsync" {
		t.Errorf("Expected name 'rsync', got '%s'", syncer.Name())
	}
}

// TestRsyncSyncer_IsAvailable tests availability check
func TestRsyncSyncer_IsAvailable(t *testing.T) {
	syncer := NewRsyncSyncer()
	// Native Go rsync is always available
	if !syncer.IsAvailable() {
		t.Error("Expected rsync to be available (native Go implementation)")
	}
}

// TestRsyncSyncer_VerifyGPG_MissingSignature tests GPG verification with missing signature
func TestRsyncSyncer_VerifyGPG_MissingSignature(t *testing.T) {
	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()

	// Create metadata directory but no signature file
	metadataDir := filepath.Join(tmpDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("Failed to create metadata dir: %v", err)
	}

	// Create timestamp file without signature
	timestampFile := filepath.Join(metadataDir, "timestamp.chk")
	if err := os.WriteFile(timestampFile, []byte("1234567890\n"), 0644); err != nil {
		t.Fatalf("Failed to create timestamp file: %v", err)
	}

	// Verify should fail - signature file not found
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Fatal("Expected error for missing signature file, got nil")
	}

	// Error should mention "signature file not found"
	if !contains(err.Error(), "signature file not found") {
		t.Errorf("Expected 'signature file not found' in error, got: %v", err)
	}
}

// TestRsyncSyncer_VerifyGPG_MissingTimestamp tests GPG verification with missing timestamp
func TestRsyncSyncer_VerifyGPG_MissingTimestamp(t *testing.T) {
	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()

	// Create metadata directory
	metadataDir := filepath.Join(tmpDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("Failed to create metadata dir: %v", err)
	}

	// Create signature file but no timestamp
	signatureFile := filepath.Join(metadataDir, "timestamp.chk.asc")
	if err := os.WriteFile(signatureFile, []byte("fake signature\n"), 0644); err != nil {
		t.Fatalf("Failed to create signature file: %v", err)
	}

	// Verify should fail - timestamp file not found
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Fatal("Expected error for missing timestamp file, got nil")
	}

	// Error should mention "timestamp file not found"
	if !contains(err.Error(), "timestamp file not found") {
		t.Errorf("Expected 'timestamp file not found' in error, got: %v", err)
	}
}

// TestRsyncSyncer_Sync_InvalidURL tests sync with invalid URL
func TestRsyncSyncer_Sync_InvalidURL(t *testing.T) {
	t.Skip("Integration test - requires network access")

	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodRsync,
		RepoPath:  tmpDir,
		SourceURL: "rsync://invalid-host-that-does-not-exist.example.com/repo",
		VerifyGPG: false,
		Verbose:   false,
	}

	ctx := context.Background()
	_, err := syncer.Sync(ctx, config)
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}
}

// TestRsyncSyncer_Sync_EmptyURL tests sync with default URL
func TestRsyncSyncer_Sync_EmptyURL(t *testing.T) {
	t.Skip("Integration test - requires network access to rsync.gentoo.org")

	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodRsync,
		RepoPath:  tmpDir,
		SourceURL: "", // Should use default rsync.gentoo.org
		VerifyGPG: false,
		Verbose:   true,
	}

	ctx := context.Background()
	result, err := syncer.Sync(ctx, config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Method != MethodRsync {
		t.Errorf("Expected method 'rsync', got '%s'", result.Method)
	}

	// Check that metadata was synced
	metadataDir := filepath.Join(tmpDir, "metadata")
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		t.Error("Expected metadata directory to exist after sync")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
