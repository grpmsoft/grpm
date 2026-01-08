//go:build !linux

package privilege

import (
	"fmt"
	"os/exec"
)

// DropPrivileges is a no-op on non-Linux platforms.
//
// Privilege dropping requires Linux-specific syscalls and is not
// available on other operating systems. GRPM targets Gentoo Linux
// exclusively, so this is expected to be the fallback stub.
func (m *Manager) DropPrivileges(cmd *exec.Cmd) error {
	// No-op on non-Linux platforms
	return nil
}

// DropPrivilegesForPhase is a no-op on non-Linux platforms.
func (m *Manager) DropPrivilegesForPhase(cmd *exec.Cmd, phase string) error {
	// No-op on non-Linux platforms
	return nil
}

// DropPrivilegesForFetch is a no-op on non-Linux platforms.
func (m *Manager) DropPrivilegesForFetch(cmd *exec.Cmd) error {
	// No-op on non-Linux platforms
	return nil
}

// lookupPortageUser is not supported on non-Linux platforms.
func lookupPortageUser() (uid, gid uint32, err error) {
	return 0, 0, fmt.Errorf("%w: not supported on this platform", ErrUserNotFound)
}

// IsRoot returns false on non-Linux platforms.
//
// Privilege management is not supported on non-Linux systems.
func IsRoot() bool {
	return false
}

// CanDropPrivileges returns false on non-Linux platforms.
func CanDropPrivileges() bool {
	return false
}

// PortageUserExists returns false on non-Linux platforms.
func PortageUserExists() bool {
	return false
}

// GetCurrentUser returns 0,0 on non-Linux platforms.
func GetCurrentUser() (uid, gid uint32) {
	return 0, 0
}

// SetOwnership is a no-op on non-Linux platforms.
func (m *Manager) SetOwnership(path string) error {
	return nil
}

// SetOwnershipRecursive is a no-op on non-Linux platforms.
func (m *Manager) SetOwnershipRecursive(path string) error {
	return nil
}
