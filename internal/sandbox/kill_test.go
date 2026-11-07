package sandbox

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestKillProcess_NotStarted(t *testing.T) {
	if err := killProcess(nil); err != nil {
		t.Fatalf("nil cmd: %v", err)
	}
	if err := killProcess(exec.Command("unused")); err != nil {
		t.Fatalf("cmd without process: %v", err)
	}
}

// A process that already exited must not turn a cancellation into a kill error.
func TestKillProcess_AlreadyExited(t *testing.T) {
	if runtime.GOOS == "windows" {
		// After Wait, os.Process.Kill returns EINVAL on Windows instead of
		// os.ErrProcessDone. The sandbox targets Linux, where this test runs in CI.
		t.Skip("Kill after Wait returns EINVAL on Windows, not os.ErrProcessDone")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run helper process: %v", err)
	}
	if err := killProcess(cmd); err != nil {
		t.Fatalf("exited process: %v", err)
	}
}
