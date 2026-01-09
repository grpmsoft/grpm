package ebuild

import (
	"bytes"
	"errors"
	"testing"

	"mvdan.cc/sh/v3/interp"
)

// ============================================================================
// Exit Status Tests
// ============================================================================

func TestExitFalse(t *testing.T) {
	err := exitFalse()
	if err == nil {
		t.Fatal("expected error from exitFalse")
	}

	var exitErr interp.ExitStatus
	if !errors.As(err, &exitErr) {
		t.Errorf("expected exit status error, got: %T", err)
	}
	if uint8(exitErr) != 1 {
		t.Errorf("expected exit code 1, got: %d", uint8(exitErr))
	}
}

// ============================================================================
// Nil Environment Tests
// ============================================================================

func TestHelpers_NilEnvironment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	helpers := NewHelpers(nil, &stdout, &stderr)

	// USE check should fail with nil env
	err := helpers.Use([]string{"ssl"})
	if err == nil {
		t.Error("expected error with nil environment")
	}

	// Messaging should still work
	err = helpers.Einfo([]string{"test"})
	if err != nil {
		t.Errorf("Einfo should work with nil env: %v", err)
	}

	// Toolchain should return defaults
	stdout.Reset()
	err = helpers.TcGetCC([]string{})
	if err != nil {
		t.Errorf("TcGetCC should work with nil env: %v", err)
	}
	if stdout.String() != "gcc" {
		t.Errorf("expected default 'gcc', got: %s", stdout.String())
	}
}

// ============================================================================
// DieError Tests
// ============================================================================

func TestDieError_Error(t *testing.T) {
	err := &DieError{Message: "test error"}
	if err.Error() != "die: test error" {
		t.Errorf("expected 'die: test error', got: %s", err.Error())
	}

	emptyErr := &DieError{}
	if emptyErr.Error() != "die called" {
		t.Errorf("expected 'die called', got: %s", emptyErr.Error())
	}
}

// ============================================================================
// NewHelpers Tests
// ============================================================================

func TestNewHelpers_DefaultValues(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	if helpers.insDestTree != "/usr" {
		t.Errorf("expected insDestTree '/usr', got: %s", helpers.insDestTree)
	}
	if helpers.exeDestTree != "" {
		t.Errorf("expected exeDestTree '', got: %s", helpers.exeDestTree)
	}
	if helpers.docDestTree != "" {
		t.Errorf("expected docDestTree '', got: %s", helpers.docDestTree)
	}
	if helpers.destTree != "/usr" {
		t.Errorf("expected destTree '/usr', got: %s", helpers.destTree)
	}
	if helpers.libDir == "" {
		t.Error("expected libDir to be set")
	}
}
