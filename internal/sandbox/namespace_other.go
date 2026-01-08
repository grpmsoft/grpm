//go:build !linux

package sandbox

import (
	"context"
	"errors"
	"os/exec"
)

// NamespaceSandbox is not available on non-Linux platforms.
// This stub exists to allow the package to compile on all platforms.
// On non-Linux systems, use NoopSandbox instead.
type NamespaceSandbox struct{}

// NewNamespaceSandbox returns an error on non-Linux platforms.
//
// Linux namespace isolation is only available on Linux.
// Use NoopSandbox on other platforms.
func NewNamespaceSandbox(cfg *Config) (*NamespaceSandbox, error) {
	return nil, ErrNamespaceNotSupported
}

// Run is not available on non-Linux platforms.
func (s *NamespaceSandbox) Run(ctx context.Context, cmd *exec.Cmd) error {
	return ErrNamespaceNotSupported
}

// AddWritablePath is not available on non-Linux platforms.
func (s *NamespaceSandbox) AddWritablePath(path string) {}

// AddReadOnlyPath is not available on non-Linux platforms.
func (s *NamespaceSandbox) AddReadOnlyPath(path string) {}

// Violations is not available on non-Linux platforms.
func (s *NamespaceSandbox) Violations() []Violation {
	return nil
}

// Close is not available on non-Linux platforms.
func (s *NamespaceSandbox) Close() error {
	return nil
}

// ErrNamespaceNotSupported is returned when namespace features are not available.
var ErrNamespaceNotSupported = errors.New("sandbox: Linux namespace isolation not supported on this platform")

// CheckKernelSupport always returns an error on non-Linux platforms.
func CheckKernelSupport(cfg *Config) error {
	return ErrNamespaceNotSupported
}
