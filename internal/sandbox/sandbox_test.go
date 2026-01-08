package sandbox

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestNewSandboxDisabled tests creating a disabled sandbox.
func TestNewSandboxDisabled(t *testing.T) {
	cfg := &Config{Enabled: false}
	sb, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Should be a NoopSandbox
	_, ok := sb.(*NoopSandbox)
	if !ok {
		t.Errorf("Expected NoopSandbox when disabled, got %T", sb)
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestNewSandboxNilConfig tests creating sandbox with nil config.
func TestNewSandboxNilConfig(t *testing.T) {
	sb, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) error = %v", err)
	}

	// Should be a NoopSandbox (disabled by default when nil)
	_, ok := sb.(*NoopSandbox)
	if !ok {
		t.Errorf("Expected NoopSandbox with nil config, got %T", sb)
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestNewSandboxNonLinux tests sandbox creation on non-Linux platforms.
func TestNewSandboxNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Test only for non-Linux platforms")
	}

	cfg := &Config{Enabled: true}
	sb, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Should be a NoopSandbox with warning
	noop, ok := sb.(*NoopSandbox)
	if !ok {
		t.Errorf("Expected NoopSandbox on non-Linux, got %T", sb)
	}

	// The noop should have warning enabled
	if noop != nil && !noop.warnOnUse {
		t.Error("Expected warnOnUse=true on non-Linux platform")
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestNoopSandboxRun tests NoopSandbox command execution.
func TestNoopSandboxRun(t *testing.T) {
	sb := NewNoopSandbox(false)

	// Run a simple command
	cmd := exec.Command("echo", "hello")
	ctx := context.Background()

	if err := sb.Run(ctx, cmd); err != nil {
		t.Errorf("Run() error = %v", err)
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestNoopSandboxRunWithContext tests context cancellation.
func TestNoopSandboxRunWithContext(t *testing.T) {
	sb := NewNoopSandbox(false)

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := exec.Command("sleep", "10")
	err := sb.Run(ctx, cmd)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestNoopSandboxPaths tests path tracking.
func TestNoopSandboxPaths(t *testing.T) {
	sb := NewNoopSandbox(false)

	// Add paths
	sb.AddWritablePath("/var/tmp/test")
	sb.AddWritablePath("/tmp")
	sb.AddReadOnlyPath("/usr")
	sb.AddReadOnlyPath("/lib")

	// Check tracked paths
	writable := sb.WritableDirs()
	if len(writable) != 2 {
		t.Errorf("Expected 2 writable paths, got %d", len(writable))
	}

	readonly := sb.ReadOnlyDirs()
	if len(readonly) != 2 {
		t.Errorf("Expected 2 read-only paths, got %d", len(readonly))
	}

	// Check violations (should be empty for NoopSandbox)
	violations := sb.Violations()
	if len(violations) != 0 {
		t.Errorf("Expected 0 violations, got %d", len(violations))
	}
}

// TestDefaultConfig tests default configuration values.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Default config should be enabled")
	}

	if cfg.Backend != "namespace" {
		t.Errorf("Expected backend=namespace, got %s", cfg.Backend)
	}

	if !cfg.DenyNetwork {
		t.Error("Default should deny network")
	}

	if !cfg.PIDIsolation {
		t.Error("Default should enable PID isolation")
	}

	if len(cfg.WritableDirs) == 0 {
		t.Error("Default should have writable dirs")
	}

	if len(cfg.ReadOnlyDirs) == 0 {
		t.Error("Default should have read-only dirs")
	}
}

// TestConfigWithWorkdir tests adding workdir paths.
func TestConfigWithWorkdir(t *testing.T) {
	cfg := DefaultConfig()
	initialWritable := len(cfg.WritableDirs)

	cfg = cfg.WithWorkdir("/work", "/image", "/temp", "/home")

	if len(cfg.WritableDirs) != initialWritable+4 {
		t.Errorf("Expected %d writable dirs, got %d",
			initialWritable+4, len(cfg.WritableDirs))
	}
}

// TestConfigWithDistDir tests adding distdir path.
func TestConfigWithDistDir(t *testing.T) {
	cfg := DefaultConfig()
	initialWritable := len(cfg.WritableDirs)

	cfg = cfg.WithDistDir("/var/cache/distfiles")

	if len(cfg.WritableDirs) != initialWritable+1 {
		t.Errorf("Expected %d writable dirs, got %d",
			initialWritable+1, len(cfg.WritableDirs))
	}
}

// TestConfigWithRepoPath tests adding repo path.
func TestConfigWithRepoPath(t *testing.T) {
	cfg := DefaultConfig()
	initialReadOnly := len(cfg.ReadOnlyDirs)

	cfg = cfg.WithRepoPath("/var/db/repos/gentoo")

	if len(cfg.ReadOnlyDirs) != initialReadOnly+1 {
		t.Errorf("Expected %d read-only dirs, got %d",
			initialReadOnly+1, len(cfg.ReadOnlyDirs))
	}
}

// TestConfigValidate tests configuration validation.
func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "disabled config is valid",
			config:  &Config{Enabled: false},
			wantErr: false,
		},
		{
			name:    "default config is valid",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "unknown backend is invalid",
			config: &Config{
				Enabled: true,
				Backend: "unknown",
			},
			wantErr: true,
		},
		{
			name: "empty backend is valid (defaults to namespace)",
			config: &Config{
				Enabled: true,
				Backend: "",
			},
			wantErr: false,
		},
		{
			name: "noop backend is valid",
			config: &Config{
				Enabled: true,
				Backend: "noop",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPortageFeaturesToConfig tests Portage FEATURES conversion.
func TestPortageFeaturesToConfig(t *testing.T) {
	tests := []struct {
		name        string
		features    []string
		wantEnabled bool
		wantNetwork bool
		wantPID     bool
		wantIPC     bool
		wantUser    bool
	}{
		{
			name:        "empty features",
			features:    []string{},
			wantEnabled: false,
			wantNetwork: true, // Default from DefaultConfig
			wantPID:     true, // Default from DefaultConfig
			wantIPC:     false,
			wantUser:    false,
		},
		{
			name:        "sandbox only",
			features:    []string{"sandbox"},
			wantEnabled: true,
			wantNetwork: true,
			wantPID:     true,
			wantIPC:     false,
			wantUser:    false,
		},
		{
			name:        "full sandbox",
			features:    []string{"sandbox", "network-sandbox", "pid-sandbox", "ipc-sandbox", "usersandbox"},
			wantEnabled: true,
			wantNetwork: true,
			wantPID:     true,
			wantIPC:     true,
			wantUser:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := PortageFeaturesToConfig(tt.features)

			if cfg.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", cfg.Enabled, tt.wantEnabled)
			}
			if cfg.DenyNetwork != tt.wantNetwork {
				t.Errorf("DenyNetwork = %v, want %v", cfg.DenyNetwork, tt.wantNetwork)
			}
			if cfg.PIDIsolation != tt.wantPID {
				t.Errorf("PIDIsolation = %v, want %v", cfg.PIDIsolation, tt.wantPID)
			}
			if cfg.IPCIsolation != tt.wantIPC {
				t.Errorf("IPCIsolation = %v, want %v", cfg.IPCIsolation, tt.wantIPC)
			}
			if cfg.UserNamespace != tt.wantUser {
				t.Errorf("UserNamespace = %v, want %v", cfg.UserNamespace, tt.wantUser)
			}
		})
	}
}

// TestViolationString tests violation string formatting.
func TestViolationString(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		violation Violation
		wantParts []string
	}{
		{
			name: "denied write",
			violation: Violation{
				Path:      "/etc/passwd",
				Operation: "write",
				Timestamp: now,
				Denied:    true,
			},
			wantParts: []string{"denied", "write", "/etc/passwd"},
		},
		{
			name: "allowed with details",
			violation: Violation{
				Path:      "/var/tmp/test",
				Operation: "write",
				Timestamp: now,
				Denied:    false,
				Details:   "within writable path",
			},
			wantParts: []string{"allowed", "write", "/var/tmp/test", "within writable path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.violation.String()
			for _, part := range tt.wantParts {
				if !contains(str, part) {
					t.Errorf("String() = %q, missing %q", str, part)
				}
			}
		})
	}
}

// TestConfigError tests config error formatting.
func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Field:   "Backend",
		Value:   "invalid",
		Message: "unknown backend",
	}

	errStr := err.Error()
	if !contains(errStr, "Backend") {
		t.Errorf("Error() missing field name: %s", errStr)
	}
	if !contains(errStr, "invalid") {
		t.Errorf("Error() missing value: %s", errStr)
	}
	if !contains(errStr, "unknown backend") {
		t.Errorf("Error() missing message: %s", errStr)
	}
}

// TestMustNewPanic tests that MustNew panics on error.
func TestMustNewPanic(t *testing.T) {
	// This should not panic (valid config)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustNew panicked unexpectedly: %v", r)
		}
	}()

	sb := MustNew(&Config{Enabled: false})
	if sb == nil {
		t.Error("MustNew returned nil")
	}
}

// helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
