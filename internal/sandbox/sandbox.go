// Package sandbox provides build isolation for ebuild execution.
//
// The sandbox prevents ebuilds from modifying the live filesystem during
// compilation by using Linux namespaces for process and filesystem isolation.
// On unsupported platforms (Windows, macOS), a no-op implementation is used.
//
// Example usage:
//
//	cfg := &sandbox.Config{
//	    Enabled:      true,
//	    Backend:      "namespace",
//	    WritableDirs: []string{"/var/tmp/portage"},
//	    DenyNetwork:  true,
//	}
//	sb, err := sandbox.New(cfg)
//	if err != nil {
//	    return err
//	}
//
//	cmd := exec.Command("make", "install")
//	if err := sb.Run(context.Background(), cmd); err != nil {
//	    // Check for violations
//	    for _, v := range sb.Violations() {
//	        log.Printf("Sandbox violation: %s on %s", v.Operation, v.Path)
//	    }
//	    return err
//	}
package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// Sandbox provides build isolation for package compilation.
//
// It prevents unauthorized filesystem access during ebuild execution,
// ensuring that builds cannot modify the live system.
type Sandbox interface {
	// Run executes a command within the sandbox.
	// The command's environment and working directory should be set
	// before calling Run.
	Run(ctx context.Context, cmd *exec.Cmd) error

	// AddWritablePath adds a path that can be written to.
	// This should be called before Run to configure allowed write paths.
	AddWritablePath(path string)

	// AddReadOnlyPath adds a path that can only be read.
	// This should be called before Run to configure allowed read paths.
	AddReadOnlyPath(path string)

	// Violations returns any sandbox violations detected during execution.
	// This should be called after Run to check for policy violations.
	Violations() []Violation

	// Close cleans up sandbox resources.
	// Should be called when the sandbox is no longer needed.
	Close() error
}

// Violation represents a sandbox policy violation.
//
// Violations are recorded when a process attempts to perform
// an operation that is not permitted by the sandbox configuration.
type Violation struct {
	// Path is the filesystem path involved in the violation.
	Path string

	// Operation is the type of operation attempted.
	// Values: "write", "exec", "net", "read"
	Operation string

	// Timestamp is when the violation occurred.
	Timestamp time.Time

	// Denied indicates whether the operation was blocked.
	// If false, the violation was only logged but not prevented.
	Denied bool

	// Details provides additional context about the violation.
	Details string
}

// String returns a human-readable description of the violation.
func (v Violation) String() string {
	status := "denied"
	if !v.Denied {
		status = "allowed"
	}
	if v.Details != "" {
		return fmt.Sprintf("[%s] %s %s on %s: %s", status, v.Operation, v.Timestamp.Format(time.RFC3339), v.Path, v.Details)
	}
	return fmt.Sprintf("[%s] %s %s on %s", status, v.Operation, v.Timestamp.Format(time.RFC3339), v.Path)
}

// New creates a new Sandbox based on the provided configuration.
//
// On Linux, it uses namespace-based isolation (mount, PID, optionally network).
// On other platforms, it returns a no-op sandbox that logs a warning.
//
// If cfg is nil or cfg.Enabled is false, returns a no-op sandbox.
func New(cfg *Config) (Sandbox, error) {
	if cfg == nil {
		cfg = DefaultConfig()
		cfg.Enabled = false
	}

	if !cfg.Enabled {
		return NewNoopSandbox(false), nil
	}

	switch runtime.GOOS {
	case "linux":
		return NewNamespaceSandbox(cfg)
	default:
		// Unsupported platform - use no-op with warning
		return NewNoopSandbox(true), nil
	}
}

// MustNew creates a new Sandbox and panics on error.
// This should only be used in initialization code where errors are fatal.
func MustNew(cfg *Config) Sandbox {
	sb, err := New(cfg)
	if err != nil {
		panic(fmt.Sprintf("sandbox: failed to create sandbox: %v", err))
	}
	return sb
}
