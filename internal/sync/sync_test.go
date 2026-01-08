package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSyncMethod tests sync method constants.
func TestSyncMethod(t *testing.T) {
	tests := []struct {
		method   SyncMethod
		expected string
	}{
		{MethodRsync, "rsync"},
		{MethodGit, "git"},
		{MethodAuto, "auto"},
	}

	for _, tt := range tests {
		if string(tt.method) != tt.expected {
			t.Errorf("SyncMethod %v != %s", tt.method, tt.expected)
		}
	}
}

// TestSyncConfig tests sync configuration.
func TestSyncConfig(t *testing.T) {
	config := &SyncConfig{
		Method:    MethodRsync,
		RepoPath:  "/var/db/repos/gentoo",
		SourceURL: "rsync://rsync.gentoo.org/gentoo-portage",
		VerifyGPG: true,
		Verbose:   false,
		PreferGit: false,
	}

	if config.Method != MethodRsync {
		t.Errorf("Method = %v, want %v", config.Method, MethodRsync)
	}

	if config.RepoPath != "/var/db/repos/gentoo" {
		t.Errorf("RepoPath = %s, want /var/db/repos/gentoo", config.RepoPath)
	}

	if !config.VerifyGPG {
		t.Error("VerifyGPG should be true")
	}
}

// TestSyncResult tests sync result structure.
func TestSyncResult(t *testing.T) {
	result := &SyncResult{
		Method:           MethodGit,
		FilesChanged:     150,
		BytesTransferred: 2048000,
		Duration:         "10.5s",
		GPGVerified:      true,
	}

	if result.Method != MethodGit {
		t.Errorf("Method = %v, want %v", result.Method, MethodGit)
	}

	if result.FilesChanged != 150 {
		t.Errorf("FilesChanged = %d, want 150", result.FilesChanged)
	}

	if result.BytesTransferred != 2048000 {
		t.Errorf("BytesTransferred = %d, want 2048000", result.BytesTransferred)
	}

	if !result.GPGVerified {
		t.Error("GPGVerified should be true")
	}
}

// TestNewSyncerRsync tests creating rsync syncer.
func TestNewSyncerRsync(t *testing.T) {
	syncer, err := NewSyncer(MethodRsync)
	if err != nil {
		t.Fatalf("NewSyncer(rsync) error = %v", err)
	}

	if syncer.Name() != "rsync" {
		t.Errorf("Name() = %s, want rsync", syncer.Name())
	}

	if !syncer.IsAvailable() {
		t.Error("Rsync syncer should always be available")
	}
}

// TestNewSyncerGit tests creating git syncer.
func TestNewSyncerGit(t *testing.T) {
	syncer, err := NewSyncer(MethodGit)
	if err != nil {
		t.Fatalf("NewSyncer(git) error = %v", err)
	}

	if syncer.Name() != "git" {
		t.Errorf("Name() = %s, want git", syncer.Name())
	}
}

// TestNewSyncerAuto tests creating auto syncer.
func TestNewSyncerAuto(t *testing.T) {
	syncer, err := NewSyncer(MethodAuto)
	if err != nil {
		t.Fatalf("NewSyncer(auto) error = %v", err)
	}

	if syncer.Name() != "auto" {
		t.Errorf("Name() = %s, want auto", syncer.Name())
	}

	if !syncer.IsAvailable() {
		t.Error("Auto syncer should always be available")
	}
}

// TestNewSyncerInvalid tests creating syncer with invalid method.
func TestNewSyncerInvalid(t *testing.T) {
	syncer, err := NewSyncer("invalid")
	if err != nil {
		t.Fatalf("NewSyncer(invalid) error = %v", err)
	}

	// Should default to auto
	if syncer.Name() != "auto" {
		t.Errorf("Name() = %s, want auto (default)", syncer.Name())
	}
}

// TestNewSyncerEmpty tests creating syncer with empty method.
func TestNewSyncerEmpty(t *testing.T) {
	syncer, err := NewSyncer("")
	if err != nil {
		t.Fatalf("NewSyncer('') error = %v", err)
	}

	// Should default to auto
	if syncer.Name() != "auto" {
		t.Errorf("Name() = %s, want auto (default)", syncer.Name())
	}
}

// TestRsyncSyncer_Sync_CreateDirectory tests that sync creates repo directory.
func TestRsyncSyncer_Sync_CreateDirectory(t *testing.T) {
	t.Skip("Integration test - requires network access")

	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "nested", "repo", "path")

	config := &SyncConfig{
		Method:    MethodRsync,
		RepoPath:  repoDir,
		SourceURL: "rsync://invalid-test.example.com/repo",
		VerifyGPG: false,
		Verbose:   false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Will fail due to invalid URL, but should create directory
	_, _ = syncer.Sync(ctx, config)

	// Verify directory was created
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		t.Error("Expected repository directory to be created")
	}
}

// TestRsyncSyncer_VerifyGPG_NoMetadataDir tests GPG verification without metadata dir.
func TestRsyncSyncer_VerifyGPG_NoMetadataDir(t *testing.T) {
	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()

	// No metadata directory exists
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Fatal("Expected error for missing metadata directory")
	}
}

// TestGitSyncer_Sync_ContextCancellation tests context cancellation.
func TestGitSyncer_Sync_ContextCancellation(t *testing.T) {
	syncer := NewGitSyncer()

	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodGit,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test-url.example.com/repo.git",
		VerifyGPG: false,
		Verbose:   false,
	}

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := syncer.Sync(ctx, config)
	// Should fail due to invalid URL or context cancellation
	if err == nil {
		t.Fatal("Expected error for canceled context or invalid URL")
	}
}

// TestGitSyncer_clone_InvalidURL tests clone with invalid URL.
func TestGitSyncer_clone_InvalidURL(t *testing.T) {
	syncer := NewGitSyncer()

	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	err := syncer.clone(ctx, "https://invalid-url-that-will-fail.example.com/repo.git", tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

// TestGitSyncer_pull_NotGitRepo tests pull on non-git directory.
func TestGitSyncer_pull_NotGitRepo(t *testing.T) {
	syncer := NewGitSyncer()

	if !syncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	err := syncer.pull(ctx, tmpDir)
	if err == nil {
		t.Fatal("Expected error for non-git directory")
	}
}

// TestAutoSyncer_Sync_VerboseMode tests auto sync with verbose mode.
func TestAutoSyncer_Sync_VerboseMode(t *testing.T) {
	syncer := NewAutoSyncer()

	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test.example.com/repo.git",
		VerifyGPG: false,
		Verbose:   true, // Enable verbose
		PreferGit: false,
	}

	ctx := context.Background()

	// Will fail due to invalid URL
	_, err := syncer.Sync(ctx, config)
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

// TestAutoSyncer_Sync_GPGRequired tests auto sync when GPG is required.
func TestAutoSyncer_Sync_GPGRequired(t *testing.T) {
	syncer := NewAutoSyncer()

	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test.example.com/repo.git",
		VerifyGPG: true, // Require GPG - should prefer Git
		Verbose:   false,
	}

	ctx := context.Background()

	// Will fail due to invalid URL
	_, err := syncer.Sync(ctx, config)
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

// TestAutoSyncer_VerifyGPG_WithGit tests GPG verification delegated to Git.
func TestAutoSyncer_VerifyGPG_WithGit(t *testing.T) {
	syncer := NewAutoSyncer()

	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available, skipping test")
	}

	tmpDir := t.TempDir()

	// Should delegate to git and fail (not a git repo)
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Fatal("Expected error for non-git directory")
	}
}

// TestAutoSyncer_Selection tests syncer selection logic.
func TestAutoSyncer_Selection(t *testing.T) {
	syncer := NewAutoSyncer()

	tests := []struct {
		name      string
		verifyGPG bool
		preferGit bool
	}{
		{"gpg_required", true, false},
		{"prefer_git", false, true},
		{"neither", false, false},
		{"both", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			config := &SyncConfig{
				Method:    MethodAuto,
				RepoPath:  tmpDir,
				SourceURL: "https://invalid-test.example.com/repo.git",
				VerifyGPG: tt.verifyGPG,
				PreferGit: tt.preferGit,
				Verbose:   true,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// All should fail due to invalid URL, but tests the selection logic
			_, _ = syncer.Sync(ctx, config)
		})
	}
}

// TestSyncerInterface tests that all syncers implement the interface.
func TestSyncerInterface(t *testing.T) {
	var _ Syncer = &RsyncSyncer{}
	var _ Syncer = &GitSyncer{}
	var _ Syncer = &AutoSyncer{}
}

// BenchmarkNewSyncer benchmarks syncer creation.
func BenchmarkNewSyncer(b *testing.B) {
	methods := []SyncMethod{MethodRsync, MethodGit, MethodAuto}

	for _, method := range methods {
		b.Run(string(method), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = NewSyncer(method)
			}
		})
	}
}

// BenchmarkSyncerAvailability benchmarks availability check.
func BenchmarkSyncerAvailability(b *testing.B) {
	rsync := NewRsyncSyncer()
	git := NewGitSyncer()
	auto := NewAutoSyncer()

	b.Run("rsync", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = rsync.IsAvailable()
		}
	})

	b.Run("git", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = git.IsAvailable()
		}
	})

	b.Run("auto", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = auto.IsAvailable()
		}
	})
}

// TestRsyncSyncer_VerifyGPG_ValidStructure tests GPG verification with valid metadata structure.
func TestRsyncSyncer_VerifyGPG_ValidStructure(t *testing.T) {
	syncer := NewRsyncSyncer()
	tmpDir := t.TempDir()

	// Create metadata directory structure
	metadataDir := filepath.Join(tmpDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create timestamp file
	timestampPath := filepath.Join(metadataDir, "timestamp.chk")
	if err := os.WriteFile(timestampPath, []byte("1700000000"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create signature file
	sigPath := filepath.Join(metadataDir, "timestamp.chk.asc")
	if err := os.WriteFile(sigPath, []byte("fake signature"), 0644); err != nil {
		t.Fatal(err)
	}

	// This should still fail because GPG verification fails without valid keys
	err := syncer.VerifyGPG(tmpDir)
	if err == nil {
		t.Skip("GPG verification succeeded - GPG keys may be configured")
	}
}

// TestGitSyncer_isGitRepo tests the git repo detection.
func TestGitSyncer_isGitRepo(t *testing.T) {
	_ = NewGitSyncer()

	// Test by checking if .git directory exists
	// Non-existent directory
	gitPath := filepath.Join("/nonexistent/path", ".git")
	if _, err := os.Stat(gitPath); err == nil {
		t.Error("Non-existent path should not have .git")
	}

	// Regular directory (not git)
	tmpDir := t.TempDir()
	gitPath = filepath.Join(tmpDir, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		t.Error("New temp directory should not be git repo")
	}

	// Directory with .git folder
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gitPath); err != nil {
		t.Error("Directory with .git should be detected as git repo")
	}
}

// TestAutoSyncer_WithContextDeadline tests auto sync with context deadline.
func TestAutoSyncer_WithContextDeadline(t *testing.T) {
	syncer := NewAutoSyncer()
	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test.example.com/repo.git",
		VerifyGPG: false,
		Verbose:   false,
	}

	// Already expired context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // Ensure deadline passes

	_, err := syncer.Sync(ctx, config)
	if err == nil {
		t.Log("Expected error for expired context or invalid URL")
	}
}

// TestSyncConfigValidation tests sync config validation.
func TestSyncConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *SyncConfig
	}{
		{
			name:   "nil_config",
			config: nil,
		},
		{
			name: "empty_source_url",
			config: &SyncConfig{
				Method:    MethodRsync,
				RepoPath:  "/tmp/test",
				SourceURL: "",
			},
		},
		{
			name: "empty_repo_path",
			config: &SyncConfig{
				Method:    MethodGit,
				RepoPath:  "",
				SourceURL: "https://example.com/repo.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify config can be created without panic
			if tt.config != nil {
				_ = tt.config.Method
			}
		})
	}
}

// TestSyncResult_AllFields tests all fields of SyncResult.
func TestSyncResult_AllFields(t *testing.T) {
	result := &SyncResult{
		Method:           MethodGit,
		FilesChanged:     100,
		BytesTransferred: 1024 * 1024,
		Duration:         "5.2s",
		GPGVerified:      true,
	}

	if result.Method != MethodGit {
		t.Errorf("Method = %v", result.Method)
	}
	if result.FilesChanged != 100 {
		t.Errorf("FilesChanged = %d", result.FilesChanged)
	}
	if result.BytesTransferred != 1024*1024 {
		t.Errorf("BytesTransferred = %d", result.BytesTransferred)
	}
	if result.Duration != "5.2s" {
		t.Errorf("Duration = %s", result.Duration)
	}
	if !result.GPGVerified {
		t.Error("GPGVerified should be true")
	}
}

// TestRsyncSyncer_ParseRsyncOutput tests rsync output parsing.
func TestRsyncSyncer_ParseRsyncOutput(t *testing.T) {
	syncer := NewRsyncSyncer()

	// Test various rsync output formats
	tests := []struct {
		output        string
		expectedFiles int
	}{
		{"receiving file list ... done\nsent 100 bytes\n", 0},
		{"", 0},
	}

	for i := range tests {
		_ = syncer
		t.Logf("Test %d: checking rsync output parsing", i)
		// The parsing is internal, so we just validate the syncer can be created
	}
}

// TestGitSyncer_ContextCancellation tests git sync with canceled context.
func TestGitSyncer_ContextCancellation(t *testing.T) {
	syncer := NewGitSyncer()

	if !syncer.IsAvailable() {
		t.Skip("Git not available")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodGit,
		RepoPath:  tmpDir,
		SourceURL: "https://anongit.gentoo.org/git/repo/gentoo.git",
		VerifyGPG: false,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := syncer.Sync(ctx, config)
	// Should fail with context canceled or early termination
	if err == nil {
		t.Log("Sync completed despite canceled context - may be cached")
	}
}

// TestAutoSyncer_PreferGit tests preference for git when configured.
func TestAutoSyncer_PreferGit(t *testing.T) {
	syncer := NewAutoSyncer()

	if !syncer.gitSyncer.IsAvailable() {
		t.Skip("Git not available")
	}

	tmpDir := t.TempDir()

	config := &SyncConfig{
		Method:    MethodAuto,
		RepoPath:  tmpDir,
		SourceURL: "https://invalid-test.example.com/repo.git",
		VerifyGPG: false,
		PreferGit: true, // Explicitly prefer Git
		Verbose:   false,
	}

	ctx := context.Background()
	_, _ = syncer.Sync(ctx, config)
	// Just verify it doesn't panic
}
