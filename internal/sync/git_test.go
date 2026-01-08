package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitSyncer_Name tests syncer name
func TestGitSyncer_Name(t *testing.T) {
	syncer := NewGitSyncer()
	if syncer.Name() != "git" {
		t.Errorf("Expected name 'git', got '%s'", syncer.Name())
	}
}

// TestGitSyncer_IsAvailable tests git availability check
func TestGitSyncer_IsAvailable(t *testing.T) {
	syncer := NewGitSyncer()

	// Check if git is actually installed
	_, err := exec.LookPath("git")
	expectedAvailable := (err == nil)

	if syncer.IsAvailable() != expectedAvailable {
		t.Errorf("Expected IsAvailable()=%v, got %v", expectedAvailable, syncer.IsAvailable())
	}
}

// TestGitSyncer_Clone tests git clone operation
func TestGitSyncer_Clone(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	t.Skip("Integration test - requires network access and takes time")

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Use a small test repository
	url := "https://github.com/golang/example.git"

	err := syncer.clone(ctx, url, tmpDir)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Check that .git directory exists
	gitDir := filepath.Join(tmpDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Expected .git directory to exist after clone")
	}
}

// TestGitSyncer_Pull tests git pull operation
func TestGitSyncer_Pull(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	t.Skip("Integration test - requires existing git repository")

	tmpDir := t.TempDir()
	ctx := context.Background()

	// First clone
	url := "https://github.com/golang/example.git"
	if err := syncer.clone(ctx, url, tmpDir); err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Then pull
	err := syncer.pull(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
}

// TestGitSyncer_VerifyGPG_NoGitRepo tests GPG verification on non-git directory
func TestGitSyncer_VerifyGPG_NoGitRepo(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	// Try to verify GPG on non-git directory
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Fatal("Expected error for non-git directory, got nil")
	}
}

// TestGitSyncer_Sync_EmptyURL tests sync with default URL
func TestGitSyncer_Sync_EmptyURL(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	t.Skip("Integration test - requires network access to anongit.gentoo.org")

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodGit,
		RepoPath:  tmpDir,
		SourceURL: "", // Should use default gentoo.git
		VerifyGPG: false,
		Verbose:   true,
	}

	ctx := context.Background()
	result, err := syncer.Sync(ctx, config)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Method != MethodGit {
		t.Errorf("Expected method 'git', got '%s'", result.Method)
	}

	// Check that .git directory exists
	gitDir := filepath.Join(tmpDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Expected .git directory to exist after sync")
	}
}

// TestGitSyncer_Sync_InvalidURL tests sync with invalid URL
func TestGitSyncer_Sync_InvalidURL(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodGit,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-host-that-does-not-exist.example.com/repo.git",
		VerifyGPG: false,
		Verbose:   false,
	}

	ctx := context.Background()
	_, err := syncer.Sync(ctx, config)
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}
}

// TestGitSyncer_Sync_ExistingRepo tests incremental pull
func TestGitSyncer_Sync_ExistingRepo(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	t.Skip("Integration test - requires network access")

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Use a small test repository
	url := "https://github.com/golang/example.git"

	config := &SyncConfig{
		Method:    MethodGit,
		RepoPath:  tmpDir,
		SourceURL: url,
		VerifyGPG: false,
		Verbose:   true,
	}

	// First sync (clone)
	result1, err := syncer.Sync(ctx, config)
	if err != nil {
		t.Fatalf("First sync failed: %v", err)
	}

	// Second sync (pull)
	result2, err := syncer.Sync(ctx, config)
	if err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}

	if result1.Method != MethodGit || result2.Method != MethodGit {
		t.Error("Expected method 'git' for both syncs")
	}
}

// TestGitSyncer_Sync_WithGPGVerification tests sync with GPG verification
func TestGitSyncer_Sync_WithGPGVerification(t *testing.T) {
	syncer := NewGitSyncer()

	// Skip if git not available
	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	t.Skip("Integration test - requires GPG keys imported and network access")

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodGit,
		RepoPath:  tmpDir,
		SourceURL: "https://anongit.gentoo.org/git/repo/gentoo.git",
		VerifyGPG: true,
		Verbose:   true,
	}

	ctx := context.Background()
	result, err := syncer.Sync(ctx, config)
	if err != nil {
		t.Fatalf("Sync with GPG verification failed: %v", err)
	}

	if !result.GPGVerified {
		t.Error("Expected GPGVerified=true, got false")
	}
}
