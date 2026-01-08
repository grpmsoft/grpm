package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/grpmsoft/grpm/internal/application"
	"github.com/grpmsoft/grpm/internal/repo"
	"google.golang.org/grpc"
)

// Daemon represents the GRPM daemon service
type Daemon struct {
	config     *Config
	repository *DaemonRepository

	// Servers
	grpcServer *grpc.Server
	grpcLis    net.Listener
	restServer *http.Server

	// State
	state   DaemonState
	stateMu sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Ready channel - signals when daemon is ready to serve
	ready chan struct{}

	// Application Services (Phase 3+)
	portageRepo    repo.Repository             // Infrastructure: Portage repository
	packageService *application.PackageService // Application: Package operations

	// Background Services (Phase 4+)
	jobQueue *JobQueue // Job queue for concurrent operations
}

// DaemonState represents daemon operational state
type DaemonState int

const (
	StateStarting DaemonState = iota
	StateRunning
	StateStopping
	StateStopped
)

func (s DaemonState) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// New creates a new daemon instance
func New(config *Config) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize infrastructure: Portage repository
	var portageRepo repo.Repository
	portageRepoImpl, err := repo.NewPortageRepository(config.PortageRepoPath)
	if err != nil {
		// Repository not available - use mock for testing/development
		// This is normal in development environments or when Portage is not installed
		portageRepo = repo.NewMockRepository()
	} else {
		portageRepo = portageRepoImpl
	}

	// Initialize application services
	packageService := application.NewPackageService(portageRepo)

	// Initialize conflict detector for safe concurrent package operations
	conflictDetector := NewConflictDetector(packageService)

	// Initialize job queue with conflict detection
	jobQueue := NewJobQueue(config.QueueMaxWorkers, config.QueueMaxSize, conflictDetector)

	d := &Daemon{
		config:         config,
		repository:     NewDaemonRepository(),
		grpcServer:     grpc.NewServer(),
		state:          StateStarting,
		ctx:            ctx,
		cancel:         cancel,
		ready:          make(chan struct{}),
		portageRepo:    portageRepo,
		packageService: packageService,
		jobQueue:       jobQueue,
	}

	// Register gRPC service with application service
	RegisterGRPMService(d, packageService)

	return d
}

// Start starts the daemon servers (non-blocking)
func (d *Daemon) Start() error {
	log.Printf("Starting GRPM daemon (socket: %s)", d.config.SocketPath)
	d.setState(StateStarting)

	// Start job queue
	d.jobQueue.Start()
	log.Printf("Job queue started (workers: %d, queue size: %d)", d.config.QueueMaxWorkers, d.config.QueueMaxSize)

	// Start gRPC server
	if err := d.startGRPCServer(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Start REST API if enabled
	if d.config.RESTEnabled {
		if err := d.startRESTServer(); err != nil {
			return fmt.Errorf("failed to start REST server: %w", err)
		}
	}

	// Save PID to file
	if err := d.repository.SavePID(os.Getpid()); err != nil {
		log.Printf("Warning: failed to save PID file: %v", err)
		// Don't fail daemon start if PID file write fails
	} else {
		log.Printf("PID file saved: %s (PID: %d)", d.repository.GetPIDFile(), os.Getpid())
	}

	// Mark as ready
	d.setState(StateRunning)
	close(d.ready)
	log.Printf("GRPM daemon started successfully (state: %s)", d.state)

	return nil
}

// Run starts the daemon and blocks until shutdown signal
func (d *Daemon) Run() error {
	// Start servers
	if err := d.Start(); err != nil {
		return err
	}

	// Wait for shutdown signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Printf("Received shutdown signal")

	// Stop with timeout
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return d.Stop(stopCtx)
}

// Stop gracefully stops the daemon with timeout
func (d *Daemon) Stop(ctx context.Context) error {
	log.Printf("Stopping GRPM daemon...")
	d.setState(StateStopping)

	// Close listener first to stop accepting new connections
	if d.grpcLis != nil {
		_ = d.grpcLis.Close()
	}

	// Stop gRPC server (with timeout)
	if d.grpcServer != nil {
		// GracefulStop blocks, so run with timeout
		stopped := make(chan struct{})
		go func() {
			d.grpcServer.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
			log.Printf("gRPC server stopped gracefully")
		case <-ctx.Done():
			log.Printf("gRPC graceful stop timed out, forcing stop")
			d.grpcServer.Stop()
		}
	}

	// Stop REST server (respects context timeout)
	var restErr error
	if d.restServer != nil {
		if err := d.restServer.Shutdown(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Printf("REST server graceful shutdown timed out, forcing close")
				restErr = d.restServer.Close()
			} else {
				restErr = err
			}
		} else {
			log.Printf("REST server stopped gracefully")
		}
	}

	// Stop job queue
	log.Printf("Stopping job queue...")
	if err := d.jobQueue.Stop(ctx); err != nil {
		log.Printf("Warning: job queue shutdown error: %v", err)
	} else {
		log.Printf("Job queue stopped successfully")
	}

	// Cancel daemon context
	d.cancel()

	// Wait for all server goroutines to finish (from Start())
	// This waits for the goroutines started in startGRPCServer() and startRESTServer()
	waitDone := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(waitDone)
	}()

	// Wait for goroutines or timeout
	select {
	case <-waitDone:
		log.Printf("All server goroutines finished")
	case <-time.After(5 * time.Second):
		log.Printf("Warning: some goroutines did not finish in time")
	}

	// Clean up socket (listener already closed at start of Stop())
	if err := os.RemoveAll(d.config.SocketPath); err != nil {
		log.Printf("Failed to remove socket: %v", err)
	}

	// Remove PID file
	if err := d.repository.ClearPID(); err != nil {
		log.Printf("Warning: failed to clear PID file: %v", err)
	} else {
		log.Printf("PID file removed")
	}

	d.setState(StateStopped)
	log.Printf("GRPM daemon stopped")

	return restErr
}

// startGRPCServer starts the gRPC server on Unix socket
func (d *Daemon) startGRPCServer() error {
	// Remove old socket if exists
	if err := os.RemoveAll(d.config.SocketPath); err != nil {
		return fmt.Errorf("failed to remove old socket: %w", err)
	}

	// Create Unix listener
	listener, err := net.Listen("unix", d.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on Unix socket: %w", err)
	}
	d.grpcLis = listener

	// Set socket permissions (root only)
	if err := os.Chmod(d.config.SocketPath, 0600); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Start gRPC server in background
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		log.Printf("gRPC server listening on %s", d.config.SocketPath)
		if err := d.grpcServer.Serve(listener); err != nil {
			// Serve returns error on Stop(), which is expected
			if !errors.Is(err, grpc.ErrServerStopped) {
				log.Printf("gRPC server error: %v", err)
			}
		}
	}()

	return nil
}

// startRESTServer starts the REST API server
func (d *Daemon) startRESTServer() error {
	// Create HTTP server
	d.restServer = &http.Server{
		Addr:         d.config.RESTBind,
		Handler:      d.createRESTHandler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start REST server in background
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		log.Printf("REST API listening on http://%s", d.config.RESTBind)
		if err := d.restServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("REST server error: %v", err)
		}
	}()

	return nil
}

// createRESTHandler creates REST API handler (basic for Phase 1)
func (d *Daemon) createRESTHandler() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ok","state":"%s"}`, d.GetState())
	})

	// Status endpoint
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"daemon":{"state":"%s","uptime":"TODO"}}`, d.GetState())
	})

	return mux
}

// GetState returns current daemon state (thread-safe)
func (d *Daemon) GetState() DaemonState {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	return d.state
}

// setState updates daemon state (thread-safe)
func (d *Daemon) setState(state DaemonState) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.state = state
}

// IsRunning returns true if daemon is in running state
func (d *Daemon) IsRunning() bool {
	return d.GetState() == StateRunning
}

// Ready returns a channel that closes when daemon is ready
func (d *Daemon) Ready() <-chan struct{} {
	return d.ready
}

// WaitReady waits for daemon to be ready with timeout
func (d *Daemon) WaitReady(timeout time.Duration) error {
	select {
	case <-d.ready:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("daemon not ready after %v", timeout)
	}
}

// GetRepository returns daemon repository (for status checks)
func (d *Daemon) GetRepository() *DaemonRepository {
	return d.repository
}

// GetJobQueue returns the job queue
func (d *Daemon) GetJobQueue() *JobQueue {
	return d.jobQueue
}
