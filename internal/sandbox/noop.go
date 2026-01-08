package sandbox

import (
	"context"
	"log"
	"os/exec"
	"sync"
)

// NoopSandbox is a no-operation sandbox implementation.
//
// It runs commands without any isolation, useful for:
//   - Unsupported platforms (Windows, macOS)
//   - When sandbox is disabled in configuration
//   - Testing and development environments
//
// If warnOnUse is true, it logs a warning when Run is called.
type NoopSandbox struct {
	warnOnUse    bool
	warnedOnce   sync.Once
	writableDirs []string
	readOnlyDirs []string
	violations   []Violation
	mu           sync.Mutex
}

// NewNoopSandbox creates a new no-op sandbox.
//
// If warnOnUse is true, a warning is logged on first command execution,
// indicating that the sandbox is not providing isolation.
func NewNoopSandbox(warnOnUse bool) *NoopSandbox {
	return &NoopSandbox{
		warnOnUse:    warnOnUse,
		writableDirs: make([]string, 0),
		readOnlyDirs: make([]string, 0),
		violations:   make([]Violation, 0),
	}
}

// Run executes the command without sandboxing.
//
// The command runs with full filesystem access. If warnOnUse was set,
// a warning is logged on the first invocation.
func (s *NoopSandbox) Run(ctx context.Context, cmd *exec.Cmd) error {
	if s.warnOnUse {
		s.warnedOnce.Do(func() {
			log.Printf("[sandbox] WARNING: Sandbox disabled on this platform. " +
				"Build isolation is NOT active - files can be modified outside allowed paths.")
		})
	}

	// Handle context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait with context
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context canceled - kill process
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done // Wait for process to exit
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// AddWritablePath records a writable path (no-op for isolation).
//
// Paths are stored but not enforced since this is a no-op sandbox.
func (s *NoopSandbox) AddWritablePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writableDirs = append(s.writableDirs, path)
}

// AddReadOnlyPath records a read-only path (no-op for isolation).
//
// Paths are stored but not enforced since this is a no-op sandbox.
func (s *NoopSandbox) AddReadOnlyPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readOnlyDirs = append(s.readOnlyDirs, path)
}

// Violations returns an empty slice since no-op sandbox cannot detect violations.
func (s *NoopSandbox) Violations() []Violation {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Return a copy to prevent external modification
	result := make([]Violation, len(s.violations))
	copy(result, s.violations)
	return result
}

// Close is a no-op as there are no resources to clean up.
func (s *NoopSandbox) Close() error {
	return nil
}

// WritableDirs returns the list of recorded writable directories.
// Used for testing and debugging.
func (s *NoopSandbox) WritableDirs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.writableDirs))
	copy(result, s.writableDirs)
	return result
}

// ReadOnlyDirs returns the list of recorded read-only directories.
// Used for testing and debugging.
func (s *NoopSandbox) ReadOnlyDirs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.readOnlyDirs))
	copy(result, s.readOnlyDirs)
	return result
}

// Ensure NoopSandbox implements Sandbox interface.
var _ Sandbox = (*NoopSandbox)(nil)
