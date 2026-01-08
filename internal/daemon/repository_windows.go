//go:build windows

package daemon

import (
	"os"
	"syscall"
)

// IsProcessRunning checks if process with given PID is running on Windows
func (r *DaemonRepository) IsProcessRunning(pid int) bool {
	_, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Windows, FindProcess always succeeds even for non-existent processes
	const (
		processQueryInformation = 0x0400
		synchronize             = 0x00100000
		standardRightsRead      = 0x00020000
	)

	desiredAccess := standardRightsRead | processQueryInformation | synchronize

	handle, err := syscall.OpenProcess(uint32(desiredAccess), false, uint32(pid))
	if err != nil {
		// Process doesn't exist or no access
		return false
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	// Get exit code
	var exitCode uint32
	err = syscall.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}
	// STILL_ACTIVE = 259
	return exitCode == 259
}
