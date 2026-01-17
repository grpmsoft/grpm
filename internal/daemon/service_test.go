package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/grpmsoft/grpm/api/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// dialDaemon creates a gRPC connection to daemon (lazy connection in gRPC v1.76+)
func dialDaemon(socketPath string) (*grpc.ClientConn, error) {
	return grpc.NewClient("unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// TestGRPCService_Ping tests gRPC Ping method
func TestGRPCService_Ping(t *testing.T) {
	// Start daemon
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	if err := d.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	}()

	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Create gRPC client
	conn, err := dialDaemon(config.SocketPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGRPMServiceClient(conn)

	// Test Ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.Message != "pong" {
		t.Errorf("Expected 'pong', got '%s'", resp.Message)
	}

	if resp.Timestamp == 0 {
		t.Error("Expected non-zero timestamp")
	}
}

// TestGRPCService_GetStatus tests gRPC GetStatus method
func TestGRPCService_GetStatus(t *testing.T) {
	// Start daemon
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	if err := d.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	}()

	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Create gRPC client
	conn, err := dialDaemon(config.SocketPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGRPMServiceClient(conn)

	// Test GetStatus
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	// Verify daemon status
	if resp.Daemon == nil {
		t.Fatal("Daemon status is nil")
	}

	if resp.Daemon.State != "running" {
		t.Errorf("Expected state 'running', got '%s'", resp.Daemon.State)
	}

	if resp.Daemon.Pid == 0 {
		t.Error("Expected non-zero PID")
	}

	if resp.Daemon.Version == "" {
		t.Error("Expected non-empty version")
	}

	// Verify system status
	if resp.System == nil {
		t.Fatal("System status is nil")
	}
}

// TestGRPCService_InstallPackage tests streaming InstallPackage method
func TestGRPCService_InstallPackage(t *testing.T) {
	// Start daemon
	config := DefaultConfig()
	config.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	config.RESTEnabled = false

	d := New(config)

	if err := d.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.Stop(stopCtx)
	}()

	if err := d.WaitReady(1 * time.Second); err != nil {
		t.Fatalf("Daemon not ready: %v", err)
	}

	// Create gRPC client
	conn, err := dialDaemon(config.SocketPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGRPMServiceClient(conn)

	// Test InstallPackage (should stream progress)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.InstallPackage(ctx, &pb.InstallPackageRequest{
		PackageName: "test-package",
	})
	if err != nil {
		t.Fatalf("InstallPackage failed: %v", err)
	}

	// Receive stream events
	eventCount := 0
	for {
		resp, err := stream.Recv()
		if err != nil {
			break // Stream ended
		}

		eventCount++

		// Verify event type
		switch event := resp.Event.(type) {
		case *pb.InstallPackageResponse_Progress:
			if event.Progress.Stage == "" {
				t.Error("Expected non-empty stage")
			}
		case *pb.InstallPackageResponse_Completion:
			// Installation completed (success or failure)
		case *pb.InstallPackageResponse_Error:
			// Error event is valid response
		default:
			t.Errorf("Unexpected event type: %T", event)
		}
	}

	if eventCount == 0 {
		t.Error("Expected at least one stream event")
	}
}
