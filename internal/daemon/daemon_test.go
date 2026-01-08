package daemon

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDaemon_StartStop tests daemon start and stop lifecycle
func TestDaemon_StartStop(t *testing.T) {
	// Create temp directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create config
	config := DefaultConfig()
	config.SocketPath = socketPath
	config.RESTEnabled = false // Disable REST for simpler test

	// Create daemon
	d := New(config)

	// Start daemon
	if err := d.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for daemon to be ready
	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Check state
	if d.GetState() != StateRunning {
		t.Errorf("Expected state Running, got %s", d.GetState())
	}

	if !d.IsRunning() {
		t.Error("IsRunning() should return true")
	}

	// Stop daemon
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop daemon: %v", err)
	}

	// Check final state
	if d.GetState() != StateStopped {
		t.Errorf("Expected state Stopped, got %s", d.GetState())
	}

	// Verify socket is cleaned up
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("Socket was not cleaned up")
	}
}

// TestDaemon_RESTHealthCheck tests REST API health endpoint
func TestDaemon_RESTHealthCheck(t *testing.T) {
	// Create temp directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create config with REST enabled on specific port
	config := DefaultConfig()
	config.SocketPath = socketPath
	config.RESTEnabled = true
	config.RESTBind = "127.0.0.1:18081" // Use specific port for testing

	// Create daemon
	d := New(config)

	// Start daemon
	if err := d.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for ready
	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	}()

	// Give REST server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get("http://127.0.0.1:18081/health")
	if err != nil {
		t.Skipf("Skipping REST test: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned %d, want 200", resp.StatusCode)
	}

	// Test status endpoint
	resp, err = http.Get("http://127.0.0.1:18081/api/v1/status")
	if err != nil {
		t.Errorf("Status endpoint error: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status endpoint returned %d, want 200", resp.StatusCode)
		}
	}
}

// TestDaemon_StateTransitions tests state machine transitions
func TestDaemon_StateTransitions(t *testing.T) {
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	// Initial state should be Starting
	if d.GetState() != StateStarting {
		t.Errorf("Initial state should be Starting, got %s", d.GetState())
	}

	// Start daemon
	if err := d.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for ready
	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Should be Running
	if d.GetState() != StateRunning {
		t.Errorf("Expected Running state, got %s", d.GetState())
	}

	if !d.IsRunning() {
		t.Error("IsRunning() should return true")
	}

	// Stop daemon
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	// Verify Stopped state
	if d.GetState() != StateStopped {
		t.Errorf("Final state should be Stopped, got %s", d.GetState())
	}
}

// TestDaemon_StopTimeout tests timeout during shutdown
func TestDaemon_StopTimeout(t *testing.T) {
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	// Start daemon
	if err := d.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for ready
	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Stop with very short timeout
	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Should complete (possibly with timeout warnings in logs)
	if err := d.Stop(stopCtx); err != nil {
		// Timeout is acceptable, forced stop should succeed
		t.Logf("Stop with short timeout: %v", err)
	}

	// Should be stopped
	if d.GetState() != StateStopped {
		t.Errorf("Expected Stopped state, got %s", d.GetState())
	}
}

// TestDaemon_ReadyChannel tests ready channel behavior
func TestDaemon_ReadyChannel(t *testing.T) {
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	// Ready channel should not be closed initially
	select {
	case <-d.Ready():
		t.Error("Ready channel should not be closed before Start()")
	default:
		// Expected
	}

	// Start daemon
	if err := d.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	}()

	// Ready channel should be closed now
	select {
	case <-d.Ready():
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("Ready channel was not closed after Start()")
	}

	// WaitReady should return immediately
	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Errorf("WaitReady() failed: %v", err)
	}
}

// TestDaemon_WaitReadyTimeout tests WaitReady timeout
func TestDaemon_WaitReadyTimeout(t *testing.T) {
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	// Don't start daemon, WaitReady should timeout
	err := d.WaitReady(100 * time.Millisecond)
	if err == nil {
		t.Error("Expected WaitReady to timeout")
	}
}

// TestDefaultConfig tests default configuration values
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"SocketPath", config.SocketPath, "/var/run/grpm.sock"},
		{"LogLevel", config.LogLevel, "info"},
		{"CacheEnabled", config.CacheEnabled, true},
		{"CacheMaxSize", config.CacheMaxSize, "1GB"},
		{"MonitoringEnabled", config.MonitoringEnabled, true},
		{"QueueMaxWorkers", config.QueueMaxWorkers, 4},
		{"QueueMaxSize", config.QueueMaxSize, 100},
		{"RESTEnabled", config.RESTEnabled, true},
		{"RESTBind", config.RESTBind, "127.0.0.1:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestConfig_ParseDurations tests duration parsing
func TestConfig_ParseDurations(t *testing.T) {
	config := DefaultConfig()

	// Test CacheTTL parsing
	ttl, err := config.ParseCacheTTL()
	if err != nil {
		t.Errorf("ParseCacheTTL() error: %v", err)
	}
	if ttl != 24*time.Hour {
		t.Errorf("ParseCacheTTL() = %v, want 24h", ttl)
	}

	// Test MonitoringInterval parsing
	interval, err := config.ParseMonitoringInterval()
	if err != nil {
		t.Errorf("ParseMonitoringInterval() error: %v", err)
	}
	if interval != 1*time.Hour {
		t.Errorf("ParseMonitoringInterval() = %v, want 1h", interval)
	}
}

// BenchmarkDaemonStartStop benchmarks daemon lifecycle
func BenchmarkDaemonStartStop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config := DefaultConfig()
		config.SocketPath = filepath.Join(b.TempDir(), "bench.sock")
		config.RESTEnabled = false

		d := New(config)

		if err := d.Start(); err != nil {
			b.Fatalf("Start error: %v", err)
		}

		if err := d.WaitReady(1 * time.Second); err != nil {
			b.Fatalf("WaitReady error: %v", err)
		}

		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := d.Stop(stopCtx); err != nil {
			b.Fatalf("Stop error: %v", err)
		}
		cancel()
	}
}
