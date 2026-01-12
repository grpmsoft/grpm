package ebuild

import (
	"errors"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/interp"
)

// ============================================================================
// Die Tests
// ============================================================================

func TestHelpers_Die(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Die([]string{"Something failed"})
	if err == nil {
		t.Fatal("expected error from Die")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}

	if dieErr.Message != "Something failed" {
		t.Errorf("expected message 'Something failed', got: %s", dieErr.Message)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Something failed") {
		t.Errorf("expected 'Something failed' in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Die_EmptyMessage(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	err := helpers.Die([]string{})
	if err == nil {
		t.Fatal("expected error from Die")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}

	if dieErr.Error() != "die called" {
		t.Errorf("expected 'die called', got: %s", dieErr.Error())
	}
}

// ============================================================================
// Einfo Tests
// ============================================================================

func TestHelpers_Einfo(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Einfo([]string{"Building", "package"})
	if err != nil {
		t.Fatalf("Einfo failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Building package") {
		t.Errorf("expected 'Building package', got: %s", output)
	}
	if !strings.Contains(output, "*") {
		t.Errorf("expected asterisk marker, got: %s", output)
	}
}

func TestHelpers_Einfo_WithColor(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Einfo([]string{"Test message"})
	if err != nil {
		t.Fatalf("Einfo failed: %v", err)
	}

	output := stdout.String()
	// Check for green color code
	if !strings.Contains(output, "\033[32m") {
		t.Errorf("expected green color code, got: %s", output)
	}
}

// ============================================================================
// Ewarn Tests
// ============================================================================

func TestHelpers_Ewarn(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Ewarn([]string{"Warning message"})
	if err != nil {
		t.Fatalf("Ewarn failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "Warning message") {
		t.Errorf("expected 'Warning message' in stderr, got: %s", output)
	}
}

func TestHelpers_Ewarn_WithColor(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Ewarn([]string{"Test"})
	if err != nil {
		t.Fatalf("Ewarn failed: %v", err)
	}

	output := stderr.String()
	// Check for yellow color code
	if !strings.Contains(output, "\033[33m") {
		t.Errorf("expected yellow color code, got: %s", output)
	}
}

// ============================================================================
// Eerror Tests
// ============================================================================

func TestHelpers_Eerror(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Eerror([]string{"Error message"})
	if err != nil {
		t.Fatalf("Eerror failed: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "Error message") {
		t.Errorf("expected 'Error message' in stderr, got: %s", output)
	}
}

func TestHelpers_Eerror_WithColor(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.Eerror([]string{"Test"})
	if err != nil {
		t.Fatalf("Eerror failed: %v", err)
	}

	output := stderr.String()
	// Check for red color code
	if !strings.Contains(output, "\033[31m") {
		t.Errorf("expected red color code, got: %s", output)
	}
}

// ============================================================================
// Elog Tests
// ============================================================================

func TestHelpers_Elog(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Elog([]string{"Log message"})
	if err != nil {
		t.Fatalf("Elog failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Log message") {
		t.Errorf("expected 'Log message', got: %s", output)
	}
	if !strings.Contains(output, "LOG:") {
		t.Errorf("expected 'LOG:' prefix, got: %s", output)
	}
}

// ============================================================================
// Ebegin/Eend Tests
// ============================================================================

func TestHelpers_Ebegin(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Ebegin([]string{"Running task"})
	if err != nil {
		t.Fatalf("Ebegin failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Running task") {
		t.Errorf("expected 'Running task', got: %s", output)
	}
	if !strings.Contains(output, "...") {
		t.Errorf("expected '...' suffix, got: %s", output)
	}
}

func TestHelpers_Eend_Success(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Eend([]string{"0"})
	if err != nil {
		t.Fatalf("Eend failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' for success, got: %s", output)
	}
}

func TestHelpers_Eend_Failure(t *testing.T) {
	helpers, stdout, stderr := createTestHelpers(t)

	err := helpers.Eend([]string{"1", "Task failed"})
	if err != nil {
		t.Fatalf("Eend failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "!!") {
		t.Errorf("expected '!!' for failure, got: %s", output)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Task failed") {
		t.Errorf("expected 'Task failed' in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Eend_NoArgs(t *testing.T) {
	helpers, stdout, _ := createTestHelpers(t)

	err := helpers.Eend([]string{})
	if err != nil {
		t.Fatalf("Eend failed: %v", err)
	}

	output := stdout.String()
	// Default is success (exit code 0)
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' for default success, got: %s", output)
	}
}

// ============================================================================
// Assert Tests (PMS Section 12.3.6)
// ============================================================================

func TestHelpers_Assert_Success_ExitStatusZero(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Set exit status to 0 (success)
	helpers.SetLastExitStatus(0)

	err := helpers.Assert([]string{})
	if err != nil {
		t.Errorf("expected no error when exit status is 0, got: %v", err)
	}
}

func TestHelpers_Assert_Failure_ExitStatusNonZero(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	// Set exit status to 1 (failure)
	helpers.SetLastExitStatus(1)

	err := helpers.Assert([]string{})
	if err == nil {
		t.Fatal("expected error when exit status is non-zero")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}

	// Default message should be "assert: command failed"
	if !strings.Contains(dieErr.Message, "assert: command failed") {
		t.Errorf("expected default assert message, got: %s", dieErr.Message)
	}

	// Check stderr output
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "assert: command failed") {
		t.Errorf("expected assert message in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Assert_Failure_WithCustomMessage(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	// Set exit status to 2 (failure)
	helpers.SetLastExitStatus(2)

	err := helpers.Assert([]string{"Build", "failed"})
	if err == nil {
		t.Fatal("expected error when exit status is non-zero")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}

	// Custom message should be used
	if dieErr.Message != "Build failed" {
		t.Errorf("expected 'Build failed', got: %s", dieErr.Message)
	}

	// Check stderr output
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Build failed") {
		t.Errorf("expected 'Build failed' in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Assert_PipeStatus_AllSuccess(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Set pipe status with all zeros (all commands succeeded)
	helpers.SetPipeStatus([]int{0, 0, 0})

	err := helpers.Assert([]string{})
	if err != nil {
		t.Errorf("expected no error when all pipe statuses are 0, got: %v", err)
	}
}

func TestHelpers_Assert_PipeStatus_FirstFailed(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Set pipe status with first command failed
	helpers.SetPipeStatus([]int{1, 0, 0})

	err := helpers.Assert([]string{"Pipeline failed"})
	if err == nil {
		t.Fatal("expected error when first pipe status is non-zero")
	}

	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}
}

func TestHelpers_Assert_PipeStatus_MiddleFailed(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Set pipe status with middle command failed
	helpers.SetPipeStatus([]int{0, 2, 0})

	err := helpers.Assert([]string{})
	if err == nil {
		t.Fatal("expected error when middle pipe status is non-zero")
	}
}

func TestHelpers_Assert_PipeStatus_LastFailed(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Set pipe status with last command failed
	helpers.SetPipeStatus([]int{0, 0, 3})

	err := helpers.Assert([]string{})
	if err == nil {
		t.Fatal("expected error when last pipe status is non-zero")
	}
}

func TestHelpers_Assert_GetPipeStatus_FallbackToExitStatus(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Don't set pipe status, only exit status
	helpers.SetLastExitStatus(5)

	// GetPipeStatus should return lastExitStatus as single element
	pipeStatus := helpers.GetPipeStatus()
	if len(pipeStatus) != 1 {
		t.Errorf("expected single element, got: %d", len(pipeStatus))
	}
	if pipeStatus[0] != 5 {
		t.Errorf("expected exit status 5, got: %d", pipeStatus[0])
	}
}

func TestHelpers_Assert_SetPipeStatus_CopiesSlice(t *testing.T) {
	helpers, _, _ := createTestHelpers(t)

	// Create original slice
	original := []int{1, 2, 3}
	helpers.SetPipeStatus(original)

	// Modify original
	original[0] = 99

	// GetPipeStatus should return a copy, not affected by modification
	pipeStatus := helpers.GetPipeStatus()
	if pipeStatus[0] == 99 {
		t.Error("SetPipeStatus should copy the slice, not reference it")
	}
	if pipeStatus[0] != 1 {
		t.Errorf("expected 1, got: %d", pipeStatus[0])
	}
}

// ============================================================================
// Nonfatal Tests
// ============================================================================

func TestHelpers_Nonfatal_EAPI_Check_UnsupportedEAPI(t *testing.T) {
	// Test that nonfatal is not available in EAPI 0-3
	for _, eapi := range []string{"0", "1", "2", "3"} {
		t.Run("EAPI_"+eapi, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, eapi)

			// Set a dummy command dispatcher
			helpers.SetCommandDispatcher(func(cmd string, args []string) error {
				return nil
			})

			err := helpers.Nonfatal([]string{"die", "test"})
			if err == nil {
				t.Errorf("EAPI %s: expected error for nonfatal in unsupported EAPI", eapi)
			}
			if !strings.Contains(err.Error(), "requires EAPI 4+") {
				t.Errorf("EAPI %s: expected EAPI error, got: %v", eapi, err)
			}
		})
	}
}

func TestHelpers_Nonfatal_EAPI_Check_SupportedEAPI(t *testing.T) {
	// Test that nonfatal works in EAPI 4+
	for _, eapi := range []string{"4", "5", "6", "7", "8"} {
		t.Run("EAPI_"+eapi, func(t *testing.T) {
			helpers, _, _ := createTestHelpersWithEAPI(t, eapi)

			// Set a command dispatcher that succeeds
			helpers.SetCommandDispatcher(func(cmd string, args []string) error {
				return nil
			})

			err := helpers.Nonfatal([]string{"einfo", "test"})
			if err != nil {
				t.Errorf("EAPI %s: unexpected error for nonfatal: %v", eapi, err)
			}
		})
	}
}

func TestHelpers_Nonfatal_RequiresCommand(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	helpers.SetCommandDispatcher(func(cmd string, args []string) error {
		return nil
	})

	err := helpers.Nonfatal([]string{})
	if err == nil {
		t.Error("expected error when no command provided")
	}
	if !strings.Contains(err.Error(), "requires a command") {
		t.Errorf("expected 'requires a command' error, got: %v", err)
	}
}

func TestHelpers_Nonfatal_RequiresDispatcher(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	// Don't set command dispatcher
	err := helpers.Nonfatal([]string{"die", "test"})
	if err == nil {
		t.Error("expected error when dispatcher not set")
	}
	if !strings.Contains(err.Error(), "command dispatcher not configured") {
		t.Errorf("expected dispatcher error, got: %v", err)
	}
}

func TestHelpers_Nonfatal_CatchesDie(t *testing.T) {
	helpers, _, stderr := createTestHelpersWithEAPI(t, "8")

	// Set dispatcher that calls die through helpers
	helpers.SetCommandDispatcher(func(cmd string, args []string) error {
		if cmd == "die" {
			return helpers.Die(args)
		}
		return nil
	})

	// Call nonfatal die - should catch the DieError and return exit status
	err := helpers.Nonfatal([]string{"die", "test error"})

	// Should return an exit status error, not a DieError
	var exitErr interp.ExitStatus
	if !errors.As(err, &exitErr) {
		t.Errorf("expected ExitStatus error, got: %T - %v", err, err)
	}

	// Exit status should be 1
	if int(exitErr) != 1 {
		t.Errorf("expected exit status 1, got: %d", int(exitErr))
	}

	// Last exit status should be set
	if helpers.GetLastExitStatus() != 1 {
		t.Errorf("expected lastExitStatus 1, got: %d", helpers.GetLastExitStatus())
	}

	// Error message should still be in stderr (die prints before returning)
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "test error") {
		t.Errorf("expected error message in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Nonfatal_SuccessfulCommand(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	commandCalled := false
	helpers.SetCommandDispatcher(func(cmd string, args []string) error {
		if cmd == "einfo" {
			commandCalled = true
		}
		return nil
	})

	err := helpers.Nonfatal([]string{"einfo", "Test message"})
	if err != nil {
		t.Errorf("expected no error for successful command, got: %v", err)
	}

	if !commandCalled {
		t.Error("expected command to be called")
	}

	if helpers.GetLastExitStatus() != 0 {
		t.Errorf("expected lastExitStatus 0, got: %d", helpers.GetLastExitStatus())
	}
}

func TestHelpers_Nonfatal_SetsNonfatalMode(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	// Track whether nonfatal mode was set during command execution
	modeWasSet := false

	helpers.SetCommandDispatcher(func(cmd string, args []string) error {
		modeWasSet = helpers.IsNonfatalMode()
		return nil
	})

	// Before call, nonfatal mode should be false
	if helpers.IsNonfatalMode() {
		t.Error("nonfatal mode should be false before call")
	}

	err := helpers.Nonfatal([]string{"test", "command"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// During execution, nonfatal mode should have been true
	if !modeWasSet {
		t.Error("nonfatal mode should have been true during command execution")
	}

	// After call, nonfatal mode should be false again
	if helpers.IsNonfatalMode() {
		t.Error("nonfatal mode should be false after call")
	}
}

func TestHelpers_Nonfatal_RestoresNonfatalModeOnError(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	helpers.SetCommandDispatcher(func(cmd string, args []string) error {
		return &DieError{Message: "test"}
	})

	// Before call
	if helpers.IsNonfatalMode() {
		t.Error("nonfatal mode should be false before call")
	}

	_ = helpers.Nonfatal([]string{"die", "test"})

	// After call (even with error), nonfatal mode should be restored
	if helpers.IsNonfatalMode() {
		t.Error("nonfatal mode should be false after call, even on error")
	}
}

func TestHelpers_Nonfatal_PassesExitStatus(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	helpers.SetCommandDispatcher(func(cmd string, args []string) error {
		return interp.ExitStatus(42)
	})

	err := helpers.Nonfatal([]string{"test"})

	var exitErr interp.ExitStatus
	if !errors.As(err, &exitErr) {
		t.Errorf("expected ExitStatus error, got: %T", err)
	}
	if int(exitErr) != 42 {
		t.Errorf("expected exit status 42, got: %d", int(exitErr))
	}
	if helpers.GetLastExitStatus() != 42 {
		t.Errorf("expected lastExitStatus 42, got: %d", helpers.GetLastExitStatus())
	}
}

func TestHelpers_Die_InNonfatalMode(t *testing.T) {
	helpers, _, stderr := createTestHelpersWithEAPI(t, "8")

	// Enable nonfatal mode
	helpers.SetNonfatalMode(true)

	err := helpers.Die([]string{"test error"})

	// Should return exit status, not DieError
	var exitErr interp.ExitStatus
	if !errors.As(err, &exitErr) {
		t.Errorf("expected ExitStatus in nonfatal mode, got: %T", err)
	}
	if int(exitErr) != 1 {
		t.Errorf("expected exit status 1, got: %d", int(exitErr))
	}

	// Error message should still be output
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "test error") {
		t.Errorf("expected error message in stderr, got: %s", errOutput)
	}
}

func TestHelpers_Die_NotInNonfatalMode(t *testing.T) {
	helpers, _, _ := createTestHelpersWithEAPI(t, "8")

	// Nonfatal mode is false by default
	err := helpers.Die([]string{"test error"})

	// Should return DieError
	var dieErr *DieError
	if !errors.As(err, &dieErr) {
		t.Errorf("expected DieError, got: %T", err)
	}
}

// ============================================================================
// Debug Print Tests (PMS Section 12.3.16)
// ============================================================================

func TestHelpers_DebugPrint_DisabledByDefault(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	// Debug mode should be disabled by default
	err := helpers.DebugPrint([]string{"test message"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Output should be empty (no-op)
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr when debug disabled, got: %s", stderr.String())
	}
}

func TestHelpers_DebugPrint_EnabledWithPortageDebug(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	// Set PORTAGE_DEBUG in environment via os.Setenv for this test
	t.Setenv("PORTAGE_DEBUG", "1")

	err := helpers.DebugPrint([]string{"test message"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Output should contain the message
	output := stderr.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message' in stderr, got: %s", output)
	}
	if !strings.Contains(output, "debug:") {
		t.Errorf("expected 'debug:' prefix in stderr, got: %s", output)
	}
}

func TestHelpers_DebugPrint_EnabledWithGrpmDebug(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	// Set GRPM_DEBUG in environment
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrint([]string{"grpm test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Output should contain the message
	output := stderr.String()
	if !strings.Contains(output, "grpm test") {
		t.Errorf("expected 'grpm test' in stderr, got: %s", output)
	}
}

func TestHelpers_DebugPrint_DisabledWithZero(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	// Set debug variables to "0" - should be disabled
	t.Setenv("PORTAGE_DEBUG", "0")
	t.Setenv("GRPM_DEBUG", "0")

	err := helpers.DebugPrint([]string{"test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Output should be empty
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr when debug=0, got: %s", stderr.String())
	}
}

func TestHelpers_DebugPrint_MultipleArguments(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrint([]string{"arg1", "arg2", "arg3"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "arg1 arg2 arg3") {
		t.Errorf("expected 'arg1 arg2 arg3' in stderr, got: %s", output)
	}
}

func TestHelpers_DebugPrint_EmptyArgs(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrint([]string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should output empty message with prefix
	output := stderr.String()
	if !strings.Contains(output, "debug:") {
		t.Errorf("expected 'debug:' prefix in stderr, got: %s", output)
	}
}

func TestHelpers_DebugPrintFunction_WithFunctionName(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrintFunction([]string{"src_configure"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "src_configure: entering function") {
		t.Errorf("expected 'src_configure: entering function', got: %s", output)
	}
}

func TestHelpers_DebugPrintFunction_WithExtraArgs(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrintFunction([]string{"my_function", "param1", "param2"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "my_function: entering function") {
		t.Errorf("expected 'my_function: entering function', got: %s", output)
	}
	if !strings.Contains(output, "param1") {
		t.Errorf("expected 'param1' in output, got: %s", output)
	}
	if !strings.Contains(output, "param2") {
		t.Errorf("expected 'param2' in output, got: %s", output)
	}
}

func TestHelpers_DebugPrintFunction_EmptyArgs(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrintFunction([]string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "(unknown): entering function") {
		t.Errorf("expected '(unknown): entering function', got: %s", output)
	}
}

func TestHelpers_DebugPrintFunction_DisabledByDefault(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.DebugPrintFunction([]string{"some_function"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should be silent when debug disabled
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr when debug disabled, got: %s", stderr.String())
	}
}

func TestHelpers_DebugPrintSection_WithSection(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrintSection([]string{"installation"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "now in section installation") {
		t.Errorf("expected 'now in section installation', got: %s", output)
	}
}

func TestHelpers_DebugPrintSection_MultipleWords(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrintSection([]string{"installing", "documentation", "files"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "now in section installing documentation files") {
		t.Errorf("expected 'now in section installing documentation files', got: %s", output)
	}
}

func TestHelpers_DebugPrintSection_EmptyArgs(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrintSection([]string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "now in section") {
		t.Errorf("expected 'now in section' in output, got: %s", output)
	}
}

func TestHelpers_DebugPrintSection_DisabledByDefault(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)

	err := helpers.DebugPrintSection([]string{"test section"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should be silent when debug disabled
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr when debug disabled, got: %s", stderr.String())
	}
}

func TestHelpers_DebugPrint_ColorOutput(t *testing.T) {
	helpers, _, stderr := createTestHelpers(t)
	t.Setenv("GRPM_DEBUG", "1")

	err := helpers.DebugPrint([]string{"color test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := stderr.String()
	// Check for cyan color code
	if !strings.Contains(output, "\033[36m") {
		t.Errorf("expected cyan color code in output, got: %s", output)
	}
}
