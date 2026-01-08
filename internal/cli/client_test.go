package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grpmsoft/grpm/internal/daemon"
)

// TestClient_DaemonDetection tests daemon auto-detection
func TestClient_DaemonDetection(t *testing.T) {
	// Test without daemon (should be unavailable)
	config := &ClientConfig{
		SocketPath: "/tmp/nonexistent.sock",
		Timeout:    100 * time.Millisecond,
	}

	client := NewClient(config)
	defer client.Close()

	if client.IsDaemonAvailable() {
		t.Error("Expected daemon to be unavailable")
	}
}

// TestClient_DaemonDetectionWithRunningDaemon tests detection with actual daemon
func TestClient_DaemonDetectionWithRunningDaemon(t *testing.T) {
	// Create temp socket path
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start daemon
	daemonConfig := daemon.DefaultConfig()
	daemonConfig.SocketPath = socketPath
	daemonConfig.RESTEnabled = false

	d := daemon.New(daemonConfig)

	// Start daemon
	if err := d.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	// Wait for daemon to be ready
	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Create client
	clientConfig := &ClientConfig{
		SocketPath: socketPath,
		Timeout:    1 * time.Second,
	}

	client := NewClient(clientConfig)

	// Cleanup: close client FIRST, then stop daemon
	defer func() {
		client.Close()                     // Close client connection first
		time.Sleep(100 * time.Millisecond) // Give connection time to close
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	}()

	// Should detect daemon
	if !client.IsDaemonAvailable() {
		t.Error("Expected daemon to be available")
	}

	// Should be able to ping
	if err := client.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

// TestClient_ReconnectDaemon tests reconnection logic
func TestClient_ReconnectDaemon(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Initially no daemon
	clientConfig := &ClientConfig{
		SocketPath: socketPath,
		Timeout:    100 * time.Millisecond,
	}

	client := NewClient(clientConfig)
	defer client.Close()

	if client.IsDaemonAvailable() {
		t.Error("Expected daemon to be initially unavailable")
	}

	// Start daemon
	daemonConfig := daemon.DefaultConfig()
	daemonConfig.SocketPath = socketPath
	daemonConfig.RESTEnabled = false

	d := daemon.New(daemonConfig)

	if err := d.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Try to reconnect
	if err := client.ReconnectDaemon(); err != nil {
		t.Errorf("Reconnect failed: %v", err)
	}

	if !client.IsDaemonAvailable() {
		t.Error("Expected daemon to be available after reconnect")
	}

	// Cleanup: close client FIRST, then stop daemon
	client.Close()
	time.Sleep(100 * time.Millisecond) // Give connection time to close
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Stop(stopCtx); err != nil {
		t.Logf("Stop error: %v", err)
	}
}

// TestClient_Ping tests ping functionality
func TestClient_Ping(t *testing.T) {
	// Without daemon
	client := NewClient(&ClientConfig{
		SocketPath: "/tmp/nonexistent.sock",
		Timeout:    100 * time.Millisecond,
	})
	defer client.Close()

	err := client.Ping()
	if err == nil {
		t.Error("Expected ping to fail without daemon")
	}
}

// TestDefaultClientConfig tests default configuration
func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	if config.SocketPath != "/var/run/grpm.sock" {
		t.Errorf("SocketPath = %s, want /var/run/grpm.sock", config.SocketPath)
	}

	if config.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", config.Timeout)
	}
}

// TestClient_GetSocketPath tests socket path getter
func TestClient_GetSocketPath(t *testing.T) {
	expectedPath := "/custom/path/test.sock"
	config := &ClientConfig{
		SocketPath: expectedPath,
		Timeout:    1 * time.Second,
	}

	client := NewClient(config)
	defer client.Close()

	if client.GetSocketPath() != expectedPath {
		t.Errorf("GetSocketPath() = %s, want %s", client.GetSocketPath(), expectedPath)
	}
}

// TestClient_CloseWithoutConnection tests closing without connection
func TestClient_CloseWithoutConnection(t *testing.T) {
	client := NewClient(&ClientConfig{
		SocketPath: "/tmp/nonexistent.sock",
		Timeout:    100 * time.Millisecond,
	})

	// Should not panic or error
	if err := client.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

// BenchmarkClient_Detection benchmarks daemon detection
func BenchmarkClient_Detection(b *testing.B) {
	config := &ClientConfig{
		SocketPath: "/tmp/nonexistent.sock",
		Timeout:    100 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := NewClient(config)
		_ = client.IsDaemonAvailable()
		_ = client.Close()
	}
}

// TestApp_Creation tests app creation
func TestApp_Creation(t *testing.T) {
	app := NewApp(&AppConfig{
		Version: "test-version",
		Verbose: false,
	})
	defer app.Close()

	if app == nil {
		t.Fatal("NewApp returned nil")
	}

	if app.version != "test-version" {
		t.Errorf("Version = %s, want test-version", app.version)
	}

	if app.GetClient() == nil {
		t.Error("Client is nil")
	}
}

// TestApp_IsDaemonMode tests daemon mode detection
func TestApp_IsDaemonMode(t *testing.T) {
	app := NewApp(&AppConfig{
		Version:    "test",
		SocketPath: "/tmp/nonexistent.sock",
	})
	defer app.Close()

	if app.IsDaemonMode() {
		t.Error("Expected standalone mode (no daemon)")
	}
}

// TestApp_SetVerbose tests verbose flag
func TestApp_SetVerbose(t *testing.T) {
	app := NewApp(&AppConfig{
		Version: "test",
		Verbose: false,
	})
	defer app.Close()

	// Should not panic
	app.SetVerbose(true)
	app.SetVerbose(false)
}

// TestApp_ExecuteErrors tests error handling
func TestApp_ExecuteErrors(t *testing.T) {
	app := NewApp(&AppConfig{
		Version:    "test",
		SocketPath: "/tmp/nonexistent.sock",
	})
	defer app.Close()

	// ExecuteViaDaemon should fail (no daemon)
	err := app.ExecuteViaDaemon("test", []string{})
	if err == nil {
		t.Error("Expected error when executing via daemon without daemon")
	}

	// ExecuteStandalone not implemented yet
	err = app.ExecuteStandalone("test", []string{})
	if err == nil {
		t.Error("Expected not implemented error")
	}
}

// TestApp_PrintVersion tests version printing
func TestApp_PrintVersion(t *testing.T) {
	// Redirect stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	app := NewApp(&AppConfig{
		Version: "1.2.3",
	})
	defer app.Close()

	app.PrintVersion()

	w.Close()
	os.Stdout = oldStdout

	// Just verify it doesn't panic
	// In real test we'd read from pipe
	_ = r
}
