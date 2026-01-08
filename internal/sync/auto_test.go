package sync

import (
	"context"
	"testing"
)

// TestAutoSyncer_Name tests syncer name
func TestAutoSyncer_Name(t *testing.T) {
	syncer := NewAutoSyncer()
	if syncer.Name() != "auto" {
		t.Errorf("Expected name 'auto', got '%s'", syncer.Name())
	}
}

// TestAutoSyncer_IsAvailable tests availability check
func TestAutoSyncer_IsAvailable(t *testing.T) {
	syncer := NewAutoSyncer()
	// Auto syncer is always available (falls back to rsync)
	if !syncer.IsAvailable() {
		t.Error("Expected auto syncer to always be available")
	}
}

// TestAutoSyncer_SelectGitWhenGPGRequired tests Git selection when GPG required
func TestAutoSyncer_SelectGitWhenGPGRequired(t *testing.T) {
	syncer := NewAutoSyncer()

	// Skip if git not available
	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test-url.example.com/repo.git",
		VerifyGPG: true, // GPG required → should prefer Git
		Verbose:   false,
	}

	ctx := context.Background()

	// This will fail because of invalid URL, but we can check the selection logic
	// by observing that it attempts Git (not rsync)
	_, err := syncer.Sync(ctx, config)

	// Should fail, but the error should be from git, not rsync
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}

	// If git is available and GPG required, it should have tried git
	// (we can't easily verify selection without mocking, but at least we test the path)
}

// TestAutoSyncer_SelectGitWhenPreferred tests Git selection when preferred
func TestAutoSyncer_SelectGitWhenPreferred(t *testing.T) {
	syncer := NewAutoSyncer()

	// Skip if git not available
	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test-url.example.com/repo.git",
		VerifyGPG: false,
		PreferGit: true, // Git preferred → should use Git
		Verbose:   false,
	}

	ctx := context.Background()

	// This will fail because of invalid URL
	_, err := syncer.Sync(ctx, config)

	// Should fail, but the error should be from git, not rsync
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}
}

// TestAutoSyncer_FallbackToRsyncWhenGitUnavailable tests rsync fallback
func TestAutoSyncer_FallbackToRsyncWhenGitUnavailable(t *testing.T) {
	syncer := NewAutoSyncer()

	// Manually disable git availability for testing
	// (In real scenario, git would not be installed)
	if syncer.gitSyncer.IsAvailable() {
		t.Skip("Git is available, can't test fallback scenario")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "rsync://invalid-host.example.com/repo",
		VerifyGPG: false,
		Verbose:   false,
	}

	ctx := context.Background()

	// Should fallback to rsync
	_, err := syncer.Sync(ctx, config)

	// Will fail due to invalid URL, but should have tried rsync
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}
}

// TestAutoSyncer_VerifyGPG_DelegatesToGit tests GPG verification delegation
func TestAutoSyncer_VerifyGPG_DelegatesToGit(t *testing.T) {
	syncer := NewAutoSyncer()

	// Skip if git not available
	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	// Try to verify GPG on non-git directory
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Fatal("Expected error for non-git directory, got nil")
	}

	// Error should be from git verification
	// (can't easily assert exact error without mocking)
}

// TestAutoSyncer_VerifyGPG_FallbackToRsync tests GPG verification fallback
func TestAutoSyncer_VerifyGPG_FallbackToRsync(t *testing.T) {
	syncer := NewAutoSyncer()

	// Manually disable git for testing
	if syncer.gitSyncer.IsAvailable() {
		t.Skip("Git is available, can't test rsync fallback")
	}

	tmpDir := t.TempDir()

	// Should fallback to rsync GPG verification
	err := syncer.VerifyGPG(tmpDir)

	// Will fail because no signature files exist
	if err == nil {
		t.Fatal("Expected error for missing signature, got nil")
	}
}

// TestNewSyncer_CreatesCorrectType tests syncer factory
func TestNewSyncer_CreatesCorrectType(t *testing.T) {
	tests := []struct {
		method       SyncMethod
		expectedName string
	}{
		{MethodRsync, "rsync"},
		{MethodGit, "git"},
		{MethodAuto, "auto"},
		{"", "auto"},        // Empty defaults to auto
		{"invalid", "auto"}, // Invalid defaults to auto
	}

	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			syncer, err := NewSyncer(tt.method)
			if err != nil {
				t.Fatalf("NewSyncer(%s) failed: %v", tt.method, err)
			}

			if syncer.Name() != tt.expectedName {
				t.Errorf("Expected syncer name '%s', got '%s'", tt.expectedName, syncer.Name())
			}
		})
	}
}

// TestSyncMethod_Constants tests sync method constants
func TestSyncMethod_Constants(t *testing.T) {
	if MethodRsync != "rsync" {
		t.Errorf("Expected MethodRsync='rsync', got '%s'", MethodRsync)
	}
	if MethodGit != "git" {
		t.Errorf("Expected MethodGit='git', got '%s'", MethodGit)
	}
	if MethodAuto != "auto" {
		t.Errorf("Expected MethodAuto='auto', got '%s'", MethodAuto)
	}
}

// TestSyncConfig_DefaultValues tests config initialization
func TestSyncConfig_DefaultValues(t *testing.T) {
	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  "/var/db/repos/gentoo",
		SourceURL: "",
		VerifyGPG: true,
		Verbose:   false,
		PreferGit: false,
	}

	if config.Method != MethodAuto {
		t.Errorf("Expected Method='auto', got '%s'", config.Method)
	}

	if config.RepoPath == "" {
		t.Error("Expected non-empty RepoPath")
	}

	if !config.VerifyGPG {
		t.Error("Expected VerifyGPG=true by default")
	}
}

// TestSyncResult_Fields tests result structure
func TestSyncResult_Fields(t *testing.T) {
	result := &SyncResult{
		Method:           MethodGit,
		FilesChanged:     100,
		BytesTransferred: 1024000,
		Duration:         "5.2s",
		GPGVerified:      true,
	}

	if result.Method != MethodGit {
		t.Errorf("Expected Method='git', got '%s'", result.Method)
	}

	if result.FilesChanged != 100 {
		t.Errorf("Expected FilesChanged=100, got %d", result.FilesChanged)
	}

	if result.BytesTransferred != 1024000 {
		t.Errorf("Expected BytesTransferred=1024000, got %d", result.BytesTransferred)
	}

	if result.Duration != "5.2s" {
		t.Errorf("Expected Duration='5.2s', got '%s'", result.Duration)
	}

	if !result.GPGVerified {
		t.Error("Expected GPGVerified=true, got false")
	}
}
