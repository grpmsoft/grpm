package cli

import (
	"context"
	"fmt"
	"net"
	"time"

	pb "github.com/grpmsoft/grpm/api/gen"
	"github.com/grpmsoft/grpm/internal/daemon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client represents the CLI client that can communicate with daemon
type Client struct {
	// Connection to daemon (if available)
	daemonConn   *grpc.ClientConn
	daemonClient pb.GRPMServiceClient

	// Configuration
	socketPath string
	timeout    time.Duration

	// State
	daemonAvailable bool
}

// ClientConfig holds client configuration
type ClientConfig struct {
	SocketPath string
	Timeout    time.Duration
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		SocketPath: "/var/run/grpm.sock",
		Timeout:    5 * time.Second,
	}
}

// NewClient creates a new CLI client
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultClientConfig()
	}

	client := &Client{
		socketPath: config.SocketPath,
		timeout:    config.Timeout,
	}

	// Try to detect daemon
	client.daemonAvailable = client.detectDaemon()

	return client
}

// detectDaemon checks if daemon is available
func (c *Client) detectDaemon() bool {
	// Use same repository pattern as daemon for PID detection
	repo := daemon.NewDaemonRepository()

	// Try PID-based detection first (more reliable)
	if info, err := repo.GetInfo(); err == nil {
		if info.Status == daemon.StateRunning && info.PID > 0 {
			// PID file exists and process is running
			// Now verify we can connect to the socket
			return c.trySocketConnection()
		}
	}

	// Fallback: Try socket connection directly
	// (in case PID file doesn't exist but daemon is running)
	return c.trySocketConnection()
}

// trySocketConnection attempts to connect to Unix socket
func (c *Client) trySocketConnection() bool {
	// Quick check: does socket exist?
	testConn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return false
	}
	_ = testConn.Close() // CRITICAL: Close test connection to avoid goroutine leak

	// Connect via gRPC (lazy connection in v1.76+)
	conn, err := grpc.NewClient("unix:"+c.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return false
	}

	c.daemonConn = conn
	c.daemonClient = pb.NewGRPMServiceClient(conn)

	return true
}

// IsDaemonAvailable returns true if daemon is running and accessible
func (c *Client) IsDaemonAvailable() bool {
	return c.daemonAvailable
}

// Close closes the connection to daemon
func (c *Client) Close() error {
	if c.daemonConn != nil {
		return c.daemonConn.Close()
	}
	return nil
}

// Ping checks if daemon is responsive
func (c *Client) Ping() error {
	if !c.daemonAvailable || c.daemonClient == nil {
		return fmt.Errorf("daemon not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use gRPC ping method
	resp, err := c.daemonClient.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	if resp.Message != "pong" {
		return fmt.Errorf("unexpected response: %s", resp.Message)
	}

	return nil
}

// ReconnectDaemon attempts to reconnect to daemon
func (c *Client) ReconnectDaemon() error {
	// Close existing connection
	if c.daemonConn != nil {
		_ = c.daemonConn.Close()
		c.daemonConn = nil
	}
	c.daemonClient = nil

	// Try to reconnect
	c.daemonAvailable = c.detectDaemon()
	if !c.daemonAvailable {
		return fmt.Errorf("failed to reconnect to daemon")
	}

	return nil
}

// GetSocketPath returns the socket path
func (c *Client) GetSocketPath() string {
	return c.socketPath
}

// GetGRPCClient returns the gRPC client for direct service calls
func (c *Client) GetGRPCClient() pb.GRPMServiceClient {
	return c.daemonClient
}
