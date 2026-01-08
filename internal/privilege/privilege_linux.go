//go:build linux

package privilege

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// DropPrivileges configures a command to run as the portage user.
//
// This sets the Credential field of the command's SysProcAttr to
// run the command with the portage user's UID and GID.
//
// The command must not have been started yet.
//
// Returns nil if privilege dropping is not enabled or if the phase
// requires root privileges.
func (m *Manager) DropPrivileges(cmd *exec.Cmd) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return nil
	}

	if cmd == nil {
		return fmt.Errorf("%w: command is nil", ErrPrivilegeDropFailed)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: m.portageUID,
		Gid: m.portageGID,
	}

	return nil
}

// DropPrivilegesForPhase configures a command to run with appropriate privileges.
//
// If the phase requires root, the command runs with current privileges.
// Otherwise, and if userpriv is enabled, the command runs as portage user.
func (m *Manager) DropPrivilegesForPhase(cmd *exec.Cmd, phase string) error {
	if RequiresRoot(phase) {
		return nil
	}

	if !m.ShouldDropForPhase(phase) {
		return nil
	}

	return m.DropPrivileges(cmd)
}

// DropPrivilegesForFetch configures a command for fetch operations.
//
// If userfetch is enabled, the command runs as portage user.
func (m *Manager) DropPrivilegesForFetch(cmd *exec.Cmd) error {
	if !m.ShouldDropForFetch() {
		return nil
	}

	return m.DropPrivileges(cmd)
}

// lookupPortageUser finds the portage user's UID and GID.
//
// Returns ErrUserNotFound if the portage user doesn't exist.
func lookupPortageUser() (uid, gid uint32, err error) {
	u, err := lookupUser(DefaultPortageUser)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrUserNotFound, err)
	}

	uid, err = parseUID(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing portage UID: %w", err)
	}

	gid, err = parseGID(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing portage GID: %w", err)
	}

	return uid, gid, nil
}

// IsRoot returns true if the current process has root privileges.
func IsRoot() bool {
	return os.Geteuid() == 0
}

// CanDropPrivileges returns true if the current process can drop privileges.
//
// This requires either running as root, or having CAP_SETUID/CAP_SETGID.
func CanDropPrivileges() bool {
	// Running as root can drop to any user
	if IsRoot() {
		return true
	}

	// Non-root cannot drop privileges without capabilities
	// TODO: Check for CAP_SETUID/CAP_SETGID capabilities
	return false
}

// PortageUserExists returns true if the portage user exists on the system.
func PortageUserExists() bool {
	_, _, err := lookupPortageUser()
	return err == nil
}

// GetCurrentUser returns the current effective UID and GID.
func GetCurrentUser() (uid, gid uint32) {
	return uint32(os.Geteuid()), uint32(os.Getegid())
}

// SetOwnership changes ownership of a file or directory to the portage user.
//
// This is useful for setting up build directories that the portage user
// needs to write to.
func (m *Manager) SetOwnership(path string) error {
	m.mu.RLock()
	uid := m.portageUID
	gid := m.portageGID
	m.mu.RUnlock()

	if err := os.Chown(path, int(uid), int(gid)); err != nil {
		return fmt.Errorf("chown %s to %d:%d: %w", path, uid, gid, err)
	}

	return nil
}

// SetOwnershipRecursive changes ownership of a directory tree to the portage user.
func (m *Manager) SetOwnershipRecursive(path string) error {
	m.mu.RLock()
	uid := int(m.portageUID)
	gid := int(m.portageGID)
	m.mu.RUnlock()

	return walkDir(path, func(p string) error {
		return os.Chown(p, uid, gid)
	})
}

// walkDir walks a directory tree and calls fn for each path.
func walkDir(root string, fn func(path string) error) error {
	// First, process the root
	if err := fn(root); err != nil {
		return err
	}

	// Read directory entries
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", root, err)
	}

	for _, entry := range entries {
		path := root + "/" + entry.Name()
		if entry.IsDir() {
			if err := walkDir(path, fn); err != nil {
				return err
			}
		} else {
			if err := fn(path); err != nil {
				return err
			}
		}
	}

	return nil
}
