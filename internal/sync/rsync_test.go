package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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

// =============================================================================
// Smart Retry and Mirror Fallback Tests
// =============================================================================

func TestDefaultStrategy(t *testing.T) {
	s := DefaultStrategy()

	if s.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", s.MaxRetries)
	}
	if s.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay=2s, got %v", s.RetryDelay)
	}
	if s.MaxMirrors != 5 {
		t.Errorf("expected MaxMirrors=5, got %d", s.MaxMirrors)
	}
	if s.ConnectionTimeout != 30*time.Second {
		t.Errorf("expected ConnectionTimeout=30s, got %v", s.ConnectionTimeout)
	}
}

func TestWithStrategy(t *testing.T) {
	syncer := NewRsyncSyncer()
	customStrategy := SyncStrategy{
		MaxRetries:        5,
		RetryDelay:        1 * time.Second,
		MaxMirrors:        10,
		ConnectionTimeout: 60 * time.Second,
	}

	result := syncer.WithStrategy(customStrategy)

	if result != syncer {
		t.Error("expected WithStrategy to return same syncer")
	}
	if syncer.strategy.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", syncer.strategy.MaxRetries)
	}
}

func TestBuildMirrorList(t *testing.T) {
	tests := []struct {
		name       string
		customURL  string
		maxMirrors int
		wantFirst  string
		wantLen    int
	}{
		{
			name:       "default mirrors",
			customURL:  "",
			maxMirrors: 5,
			wantFirst:  "rsync://rsync.gentoo.org/gentoo-portage",
			wantLen:    5,
		},
		{
			name:       "custom URL first",
			customURL:  "rsync://custom.example.com/repo",
			maxMirrors: 3,
			wantFirst:  "rsync://custom.example.com/repo",
			wantLen:    3,
		},
		{
			name:       "default URL not duplicated",
			customURL:  "rsync://rsync.gentoo.org/gentoo-portage",
			maxMirrors: 5,
			wantFirst:  "rsync://rsync.gentoo.org/gentoo-portage",
			wantLen:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncer := NewRsyncSyncer()
			syncer.strategy.MaxMirrors = tt.maxMirrors

			mirrors := syncer.buildMirrorList(tt.customURL)

			if len(mirrors) != tt.wantLen {
				t.Errorf("expected %d mirrors, got %d", tt.wantLen, len(mirrors))
			}
			if len(mirrors) > 0 && mirrors[0] != tt.wantFirst {
				t.Errorf("expected first mirror '%s', got '%s'", tt.wantFirst, mirrors[0])
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	syncer := NewRsyncSyncer()

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"connection refused", errors.New("dial tcp: connection refused"), true},
		{"timeout", errors.New("i/o timeout"), true},
		{"EOF", errors.New("unexpected EOF"), true},
		{"broken pipe", errors.New("write: broken pipe"), true},
		{"forcibly closed", errors.New("forcibly closed by the remote host"), true},
		{"permission denied", errors.New("permission denied"), false},
		{"disk full", errors.New("no space left on device"), false},
		{"generic error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("expected retryable=%v, got %v for error: %v",
					tt.retryable, result, tt.err)
			}
		})
	}
}

func TestIsPermanentError(t *testing.T) {
	syncer := NewRsyncSyncer()

	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{"nil error", nil, false},
		{"permission denied", errors.New("permission denied"), true},
		{"disk full", errors.New("disk full"), true},
		{"no space left", errors.New("no space left on device"), true},
		{"read-only fs", errors.New("read-only file system"), true},
		{"connection reset", errors.New("connection reset"), false},
		{"timeout", errors.New("timeout"), false},
		{"generic error", errors.New("some error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.isPermanentError(tt.err)
			if result != tt.permanent {
				t.Errorf("expected permanent=%v, got %v for error: %v",
					tt.permanent, result, tt.err)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	syncer := NewRsyncSyncer()

	tests := []struct {
		url      string
		wantHost string
	}{
		{"rsync://rsync.gentoo.org/gentoo-portage", "rsync.gentoo.org"},
		{"rsync://mirror.example.com/repo", "mirror.example.com"},
		{"rsync://host:873/module", "host:873"},
		{"rsync://", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			host := syncer.extractHost(tt.url)
			if host != tt.wantHost {
				t.Errorf("expected host '%s', got '%s'", tt.wantHost, host)
			}
		})
	}
}

func TestGentooMirrorsNotEmpty(t *testing.T) {
	if len(GentooMirrors) == 0 {
		t.Error("GentooMirrors should not be empty")
	}

	// Verify all mirrors have correct format
	for i, mirror := range GentooMirrors {
		if mirror == "" {
			t.Errorf("mirror %d is empty", i)
		}
		if !contains(mirror, "rsync://") {
			t.Errorf("mirror %d should start with rsync://: %s", i, mirror)
		}
		if !contains(mirror, "gentoo-portage") {
			t.Errorf("mirror %d should contain gentoo-portage: %s", i, mirror)
		}
	}
}

func TestSyncStrategyValidation(t *testing.T) {
	s := DefaultStrategy()

	if s.MaxRetries < 1 {
		t.Error("MaxRetries should be at least 1")
	}
	if s.MaxRetries > 10 {
		t.Error("MaxRetries should not exceed 10")
	}
	if s.RetryDelay < time.Second {
		t.Error("RetryDelay should be at least 1 second")
	}
	if s.ConnectionTimeout < 10*time.Second {
		t.Error("ConnectionTimeout should be at least 10 seconds")
	}
}

func TestBuildMirrorListNoLimit(t *testing.T) {
	syncer := NewRsyncSyncer()
	syncer.strategy.MaxMirrors = 0 // No limit

	mirrors := syncer.buildMirrorList("")

	// Should include all Gentoo mirrors
	if len(mirrors) != len(GentooMirrors) {
		t.Errorf("expected %d mirrors, got %d", len(GentooMirrors), len(mirrors))
	}
}
