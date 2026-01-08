//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// IsProcessRunning checks if process with given PID is running on Unix
func (r *DaemonRepository) IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, use signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
