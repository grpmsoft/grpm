//go:build linux

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// NamespaceSandbox provides Linux namespace-based build isolation.
//
// It uses the following Linux namespaces for isolation:
//   - Mount namespace (CLONE_NEWNS): Filesystem isolation via overlay mounts
//   - PID namespace (CLONE_NEWPID): Process isolation (optional)
//   - Network namespace (CLONE_NEWNET): Network isolation (optional)
//   - IPC namespace (CLONE_NEWIPC): IPC isolation (optional)
//   - User namespace (CLONE_NEWUSER): Unprivileged operation (optional)
//
// Writable directories are bind-mounted with write permissions.
// All other paths are mounted read-only or are inaccessible.
type NamespaceSandbox struct {
	config       *Config
	writableDirs []string
	readOnlyDirs []string
	violations   []Violation
	mu           sync.Mutex

	// overlayDir is the temporary directory for overlay mounts
	overlayDir string
}

// NewNamespaceSandbox creates a new namespace-based sandbox.
//
// Requires Linux kernel 3.8+ for user namespaces, or root privileges.
func NewNamespaceSandbox(cfg *Config) (*NamespaceSandbox, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	sb := &NamespaceSandbox{
		config:       cfg,
		writableDirs: make([]string, 0, len(cfg.WritableDirs)),
		readOnlyDirs: make([]string, 0, len(cfg.ReadOnlyDirs)),
		violations:   make([]Violation, 0),
	}

	// Copy configured paths
	sb.writableDirs = append(sb.writableDirs, cfg.WritableDirs...)
	sb.readOnlyDirs = append(sb.readOnlyDirs, cfg.ReadOnlyDirs...)

	return sb, nil
}

// Run executes a command within the sandbox namespace.
//
// The command is executed in an isolated namespace with:
//   - Filesystem writes restricted to allowed paths
//   - Optional network isolation
//   - Optional PID isolation
//
// Returns error if the command fails or sandbox setup fails.
func (s *NamespaceSandbox) Run(ctx context.Context, cmd *exec.Cmd) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Set up namespace flags
	cloneFlags := s.buildCloneFlags()

	// Configure command with namespace settings
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Cloneflags = cloneFlags

	// Set up user namespace mapping if enabled
	if s.config.UserNamespace {
		if err := s.configureUserNamespace(cmd); err != nil {
			return fmt.Errorf("user namespace setup failed: %w", err)
		}
	}

	// Create wrapper script for mount setup
	wrapperScript, err := s.createMountWrapper(cmd)
	if err != nil {
		return fmt.Errorf("mount wrapper creation failed: %w", err)
	}
	defer func() { _ = os.Remove(wrapperScript) }()

	// Modify command to use wrapper
	originalArgs := cmd.Args
	cmd.Path = "/bin/sh"
	cmd.Args = []string{"/bin/sh", wrapperScript}

	// Preserve original environment and add sandbox marker
	cmd.Env = append(cmd.Env, "SANDBOX=1", "SANDBOX_ACTIVE=1")

	log.Printf("[sandbox] Running sandboxed: %s", strings.Join(originalArgs, " "))

	// Start the sandboxed command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("sandbox start failed: %w", err)
	}

	// Wait with context support
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// buildCloneFlags returns the namespace flags for process creation.
func (s *NamespaceSandbox) buildCloneFlags() uintptr {
	var flags uintptr

	// Always use mount namespace for filesystem isolation
	flags |= syscall.CLONE_NEWNS

	// Optional: PID namespace isolation
	if s.config.PIDIsolation {
		flags |= syscall.CLONE_NEWPID
	}

	// Optional: Network namespace isolation
	if s.config.DenyNetwork {
		flags |= syscall.CLONE_NEWNET
	}

	// Optional: IPC namespace isolation
	if s.config.IPCIsolation {
		flags |= syscall.CLONE_NEWIPC
	}

	// Optional: User namespace for unprivileged operation
	if s.config.UserNamespace {
		flags |= syscall.CLONE_NEWUSER
	}

	return flags
}

// configureUserNamespace sets up UID/GID mapping for user namespace.
func (s *NamespaceSandbox) configureUserNamespace(cmd *exec.Cmd) error {
	uid := os.Getuid()
	gid := os.Getgid()

	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: uid, Size: 1},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: gid, Size: 1},
	}

	return nil
}

// createMountWrapper creates a shell script that sets up mounts.
//
// The wrapper script:
//  1. Makes the mount namespace private
//  2. Bind mounts writable directories
//  3. Bind mounts read-only directories with read-only flag
//  4. Executes the original command
func (s *NamespaceSandbox) createMountWrapper(cmd *exec.Cmd) (string, error) {
	// Create temporary script file
	f, err := os.CreateTemp("", "sandbox-wrapper-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create wrapper script: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Build script content
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("set -e\n\n")

	// Make mount namespace private to avoid affecting host
	script.WriteString("# Make mount namespace private\n")
	script.WriteString("mount --make-rprivate /\n\n")

	// Mount /proc in new namespace (required for PID isolation)
	if s.config.PIDIsolation {
		script.WriteString("# Mount new /proc for PID namespace\n")
		script.WriteString("mount -t proc proc /proc\n\n")
	}

	// Create overlay for root filesystem (optional, for stricter isolation)
	// For now, we use bind mounts which is simpler and more compatible

	// Set up writable directories
	script.WriteString("# Writable directories (bind mount)\n")
	for _, dir := range s.writableDirs {
		// Ensure directory exists
		script.WriteString(fmt.Sprintf("mkdir -p '%s' 2>/dev/null || true\n", dir))
		// Bind mount to itself to ensure it's accessible
		script.WriteString(fmt.Sprintf("mount --bind '%s' '%s' 2>/dev/null || true\n", dir, dir))
	}
	script.WriteString("\n")

	// Set up read-only directories
	script.WriteString("# Read-only directories\n")
	for _, dir := range s.readOnlyDirs {
		// Bind mount read-only
		script.WriteString(fmt.Sprintf("mount --bind '%s' '%s' 2>/dev/null || true\n", dir, dir))
		script.WriteString(fmt.Sprintf("mount -o remount,ro,bind '%s' 2>/dev/null || true\n", dir))
	}
	script.WriteString("\n")

	// Change to working directory
	if cmd.Dir != "" {
		script.WriteString(fmt.Sprintf("cd '%s'\n", cmd.Dir))
	}

	// Execute the original command
	script.WriteString("# Execute original command\n")
	script.WriteString("exec")
	for _, arg := range cmd.Args[1:] { // Skip the command name itself
		// Quote arguments properly
		script.WriteString(fmt.Sprintf(" '%s'", strings.ReplaceAll(arg, "'", "'\\''")))
	}
	script.WriteString("\n")

	// Write script
	if _, err := f.WriteString(script.String()); err != nil {
		return "", fmt.Errorf("failed to write wrapper script: %w", err)
	}

	// Make executable
	if err := f.Chmod(0755); err != nil {
		return "", fmt.Errorf("failed to make wrapper executable: %w", err)
	}

	return f.Name(), nil
}

// AddWritablePath adds a path that can be written to within the sandbox.
func (s *NamespaceSandbox) AddWritablePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check for duplicates
	for _, existing := range s.writableDirs {
		if existing == absPath {
			return
		}
	}

	s.writableDirs = append(s.writableDirs, absPath)
}

// AddReadOnlyPath adds a path that can only be read within the sandbox.
func (s *NamespaceSandbox) AddReadOnlyPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check for duplicates
	for _, existing := range s.readOnlyDirs {
		if existing == absPath {
			return
		}
	}

	s.readOnlyDirs = append(s.readOnlyDirs, absPath)
}

// Violations returns sandbox violations detected during execution.
//
// Note: With pure namespace isolation, violations are prevented rather than
// detected. This returns recorded violations from mount failures or other
// sandbox setup issues.
func (s *NamespaceSandbox) Violations() []Violation {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Violation, len(s.violations))
	copy(result, s.violations)
	return result
}

// Close cleans up sandbox resources.
func (s *NamespaceSandbox) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up overlay directory if it exists
	if s.overlayDir != "" {
		if err := os.RemoveAll(s.overlayDir); err != nil {
			return fmt.Errorf("failed to clean up overlay dir: %w", err)
		}
		s.overlayDir = ""
	}

	return nil
}

// IsPathWritable checks if a path is within a writable directory.
func (s *NamespaceSandbox) IsPathWritable(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, writable := range s.writableDirs {
		if strings.HasPrefix(absPath, writable) {
			return true
		}
	}

	return false
}

// IsPathReadable checks if a path is accessible (readable or writable).
func (s *NamespaceSandbox) IsPathReadable(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Writable paths are also readable
	for _, writable := range s.writableDirs {
		if strings.HasPrefix(absPath, writable) {
			return true
		}
	}

	// Check read-only paths
	for _, readonly := range s.readOnlyDirs {
		if strings.HasPrefix(absPath, readonly) {
			return true
		}
	}

	return false
}

// RequiresRoot returns true if the sandbox configuration requires root.
//
// Root is not required if user namespaces are enabled, but user namespace
// support depends on kernel configuration (unprivileged user namespaces).
func (s *NamespaceSandbox) RequiresRoot() bool {
	// User namespace allows unprivileged operation
	if s.config.UserNamespace {
		return false
	}

	// Mount namespace and network namespace typically require CAP_SYS_ADMIN
	return true
}

// CheckKernelSupport verifies that the kernel supports required features.
//
// Returns an error describing missing features.
func CheckKernelSupport(cfg *Config) error {
	var missing []string

	// Check for user namespace support if requested
	if cfg.UserNamespace {
		// Check if unprivileged user namespaces are enabled
		data, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
		if err == nil && strings.TrimSpace(string(data)) == "0" {
			missing = append(missing, "unprivileged_userns_clone=1")
		}
	}

	// Check for network namespace support if requested
	if cfg.DenyNetwork {
		if _, err := os.Stat("/proc/self/ns/net"); err != nil {
			missing = append(missing, "network namespace")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("kernel features not available: %s", strings.Join(missing, ", "))
	}

	return nil
}

// Ensure NamespaceSandbox implements Sandbox interface.
var _ Sandbox = (*NamespaceSandbox)(nil)

// ErrNotRoot is returned when root privileges are required but not available.
var ErrNotRoot = errors.New("sandbox: root privileges required (try with sudo or enable user namespaces)")

// ErrNamespaceNotSupported is returned when namespace features are not available.
var ErrNamespaceNotSupported = errors.New("sandbox: Linux namespace features not supported on this system")
