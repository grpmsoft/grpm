// Package privilege provides privilege dropping for ebuild execution.
//
// This package implements Portage's userpriv, userfetch, and usersandbox
// features, allowing build phases to run as an unprivileged user (portage:portage)
// rather than root for improved security.
//
// Example usage:
//
//	features := privilege.Features{
//	    UserPriv:    true,
//	    UserFetch:   true,
//	    UserSandbox: false,
//	}
//	mgr, err := privilege.NewManager(features)
//	if err != nil {
//	    return err
//	}
//
//	// Check if phase should drop privileges
//	if !privilege.RequiresRoot(phase) {
//	    if err := mgr.DropPrivileges(cmd); err != nil {
//	        return err
//	    }
//	}
//
//	return cmd.Run()
package privilege

import (
	"errors"
	"fmt"
	"os/user"
	"strconv"
	"sync"
)

// DefaultPortageUID is the default UID for the portage user.
const DefaultPortageUID uint32 = 250

// DefaultPortageGID is the default GID for the portage group.
const DefaultPortageGID uint32 = 250

// DefaultPortageUser is the name of the unprivileged build user.
const DefaultPortageUser = "portage"

// DefaultPortageGroup is the name of the unprivileged build group.
const DefaultPortageGroup = "portage"

// DefaultPortageHome is the home directory for the portage user.
const DefaultPortageHome = "/var/tmp/portage"

// Features configures privilege dropping behavior.
//
// These correspond to Portage's FEATURES settings:
//   - userpriv: Drop privileges for build phases (src_unpack through src_install)
//   - userfetch: Drop privileges for distfile fetching
//   - usersandbox: Enable user namespace sandbox for unprivileged operation
type Features struct {
	// UserPriv enables privilege dropping for build phases.
	// When true, build phases run as the portage user instead of root.
	// Merge phases (preinst, postinst, qmerge) still run as root.
	UserPriv bool

	// UserFetch enables privilege dropping for fetch operations.
	// When true, distfile downloads run as the portage user.
	UserFetch bool

	// UserSandbox enables user namespace sandbox.
	// When true, the build runs in an unprivileged user namespace.
	// This allows running the sandbox without root privileges.
	UserSandbox bool
}

// Manager handles privilege dropping and restoration.
//
// The Manager looks up the portage user at creation time and
// provides methods to apply privilege dropping to commands.
type Manager struct {
	portageUID uint32
	portageGID uint32
	enabled    bool
	features   Features

	// mu protects concurrent access to manager state
	mu sync.RWMutex
}

// ManagerOption is a functional option for configuring the Manager.
type ManagerOption func(*Manager) error

// WithUID sets a custom UID for the unprivileged user.
func WithUID(uid uint32) ManagerOption {
	return func(m *Manager) error {
		m.portageUID = uid
		return nil
	}
}

// WithGID sets a custom GID for the unprivileged group.
func WithGID(gid uint32) ManagerOption {
	return func(m *Manager) error {
		m.portageGID = gid
		return nil
	}
}

// NewManager creates a privilege manager with the specified features.
//
// If the portage user exists on the system, its UID/GID are used.
// Otherwise, default values (250:250) are used.
//
// Returns an error if privilege dropping is requested but cannot be configured.
func NewManager(features Features, opts ...ManagerOption) (*Manager, error) {
	m := &Manager{
		portageUID: DefaultPortageUID,
		portageGID: DefaultPortageGID,
		enabled:    features.UserPriv || features.UserFetch,
		features:   features,
	}

	// Try to look up the portage user
	if m.enabled {
		uid, gid, err := lookupPortageUser()
		if err == nil {
			m.portageUID = uid
			m.portageGID = gid
		}
		// If lookup fails, continue with defaults - the user may be created later
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, fmt.Errorf("applying option: %w", err)
		}
	}

	return m, nil
}

// Enabled returns true if privilege dropping is enabled.
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Features returns the current feature configuration.
func (m *Manager) Features() Features {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.features
}

// PortageUID returns the UID of the portage user.
func (m *Manager) PortageUID() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.portageUID
}

// PortageGID returns the GID of the portage group.
func (m *Manager) PortageGID() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.portageGID
}

// ShouldDropForPhase returns true if privileges should be dropped for the phase.
//
// Build phases (unpack, prepare, configure, compile, test, install) run
// as the portage user when userpriv is enabled.
//
// Merge phases (preinst, postinst, prerm, postrm, qmerge) always run as root.
func (m *Manager) ShouldDropForPhase(phase string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.features.UserPriv {
		return false
	}

	return !RequiresRoot(phase)
}

// ShouldDropForFetch returns true if privileges should be dropped for fetching.
func (m *Manager) ShouldDropForFetch() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.features.UserFetch
}

// RequiresRoot returns true if the specified phase requires root privileges.
//
// Phases that require root:
//   - preinst: Pre-installation script (modifies live filesystem)
//   - postinst: Post-installation script (modifies live filesystem)
//   - prerm: Pre-removal script (modifies live filesystem)
//   - postrm: Post-removal script (modifies live filesystem)
//   - qmerge: Merge files to live filesystem
//
// Phases that do NOT require root:
//   - fetch: Download distfiles
//   - unpack: Extract source archives
//   - prepare: Apply patches
//   - configure: Run configure scripts
//   - compile: Build the package
//   - test: Run package tests
//   - install: Install to staging directory (${D})
func RequiresRoot(phase string) bool {
	switch phase {
	case "preinst", "postinst", "prerm", "postrm", "qmerge":
		return true
	default:
		return false
	}
}

// PhasePrivilegeInfo describes the privilege requirements for a phase.
type PhasePrivilegeInfo struct {
	// Phase is the phase name.
	Phase string

	// RequiresRoot is true if the phase must run as root.
	RequiresRoot bool

	// Reason explains why the phase does or doesn't require root.
	Reason string
}

// GetPhaseInfo returns privilege information for a phase.
func GetPhaseInfo(phase string) PhasePrivilegeInfo {
	switch phase {
	case "fetch":
		return PhasePrivilegeInfo{phase, false, "network access only, no system changes"}
	case "unpack":
		return PhasePrivilegeInfo{phase, false, "extracts to build directory only"}
	case "prepare":
		return PhasePrivilegeInfo{phase, false, "modifies source in build directory only"}
	case "configure":
		return PhasePrivilegeInfo{phase, false, "runs configure scripts in build directory"}
	case "compile":
		return PhasePrivilegeInfo{phase, false, "compiles in build directory only"}
	case "test":
		return PhasePrivilegeInfo{phase, false, "runs tests in build directory only"}
	case "install":
		return PhasePrivilegeInfo{phase, false, "installs to staging directory ${D} only"}
	case "preinst":
		return PhasePrivilegeInfo{phase, true, "runs scripts that may modify live filesystem"}
	case "postinst":
		return PhasePrivilegeInfo{phase, true, "runs scripts that modify live filesystem"}
	case "prerm":
		return PhasePrivilegeInfo{phase, true, "runs scripts before removing from live filesystem"}
	case "postrm":
		return PhasePrivilegeInfo{phase, true, "runs scripts after removing from live filesystem"}
	case "qmerge":
		return PhasePrivilegeInfo{phase, true, "merges files to live filesystem"}
	default:
		return PhasePrivilegeInfo{phase, false, "unknown phase, assuming no root required"}
	}
}

// parseUID parses a string UID to uint32.
func parseUID(s string) (uint32, error) {
	uid, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid UID %q: %w", s, err)
	}
	return uint32(uid), nil
}

// parseGID parses a string GID to uint32.
func parseGID(s string) (uint32, error) {
	gid, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid GID %q: %w", s, err)
	}
	return uint32(gid), nil
}

// Error types for privilege operations.
var (
	// ErrUserNotFound is returned when the portage user doesn't exist.
	ErrUserNotFound = errors.New("privilege: portage user not found")

	// ErrGroupNotFound is returned when the portage group doesn't exist.
	ErrGroupNotFound = errors.New("privilege: portage group not found")

	// ErrNotRoot is returned when root privileges are required but not available.
	ErrNotRoot = errors.New("privilege: root privileges required")

	// ErrPrivilegeDropFailed is returned when privilege dropping fails.
	ErrPrivilegeDropFailed = errors.New("privilege: failed to drop privileges")
)

// lookupUser wraps user.Lookup for testing.
var lookupUser = user.Lookup
