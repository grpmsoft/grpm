package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// killProcess kills a started command on context cancellation.
//
// A process that has already exited is not an error: the race between the
// context deadline and the process finishing on its own is expected.
func killProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill sandboxed process %d: %w", cmd.Process.Pid, err)
	}
	return nil
}
