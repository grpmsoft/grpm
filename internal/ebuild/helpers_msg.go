// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 messaging functions (die, einfo, ewarn, etc.).
package ebuild

import (
	"errors"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

// ============================================================================
// EAPI 8 Messaging Functions
// ============================================================================

// Die terminates ebuild execution with an error message.
//
// Usage: die "error message"
//
// In Portage, die() causes immediate termination. We return an error that
// should be propagated up to stop execution.
//
// Per PMS Section 12.3.1: If called under the nonfatal command (EAPI 4+),
// die returns a non-zero exit status instead of aborting the build.
func (h *Helpers) Die(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorRed + " * " + colorReset + "ERROR: " + msg + "\n")

	// In nonfatal mode, return exit status 1 instead of DieError
	// Per PMS Section 12.3.1: nonfatal causes die to return non-zero exit status
	if h.nonfatalMode {
		h.SetLastExitStatus(1)
		return interp.ExitStatus(1)
	}

	return &DieError{Message: msg}
}

// Einfo prints an informational message (green asterisk).
//
// Usage: einfo "message"
func (h *Helpers) Einfo(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout(colorGreen + " * " + colorReset + msg + "\n")
	return nil
}

// Ewarn prints a warning message (yellow asterisk).
//
// Usage: ewarn "message"
func (h *Helpers) Ewarn(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorYellow + " * " + colorReset + msg + "\n")
	return nil
}

// Eerror prints an error message (red asterisk).
//
// Usage: eerror "message"
func (h *Helpers) Eerror(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorRed + " * " + colorReset + msg + "\n")
	return nil
}

// Elog prints a message to the elog system.
//
// Usage: elog "message"
//
// In Portage, elog messages are saved for later review. Here we just output.
func (h *Helpers) Elog(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout("LOG: " + msg + "\n")
	return nil
}

// Ebegin prints a "begin" message for a status operation.
//
// Usage: ebegin "Starting something"
func (h *Helpers) Ebegin(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout(colorGreen + " * " + colorReset + msg + " ...")
	return nil
}

// Eend prints an "end" message with success/failure indicator.
//
// Usage: eend $? "failure message"
//
// Parameters:
//   - args[0]: exit code (0 = success, non-zero = failure)
//   - args[1:]: optional failure message
func (h *Helpers) Eend(args []string) error {
	exitCode := "0"
	failMsg := ""

	if len(args) >= 1 {
		exitCode = args[0]
	}
	if len(args) >= 2 {
		failMsg = strings.Join(args[1:], " ")
	}

	if exitCode == "0" {
		h.writeStdout(" " + colorGreen + "[ ok ]" + colorReset + "\n")
	} else {
		h.writeStdout(" " + colorRed + "[ !! ]" + colorReset + "\n")
		if failMsg != "" {
			h.writeStderr(colorRed + " * " + colorReset + failMsg + "\n")
		}
	}
	return nil
}

// Assert checks the shell's pipe status and dies if any element is non-zero.
//
// Usage: assert [message]
//
// Per PMS Section 12.3.6, assert checks the shell's pipe status array
// (PIPESTATUS), and if any element is non-zero (indicating failure),
// calls die with the optional message.
//
// Available in EAPIs 0-8. Banned in EAPI 9 per PMS Table 12.3.
//
// Example:
//
//	./configure && make || assert "Build failed"
//	some_command | other_command
//	assert "Pipeline failed"
func (h *Helpers) Assert(args []string) error {
	// Get pipe status - if not set, use last exit status
	pipeStatus := h.GetPipeStatus()

	// Check if any element in pipe status is non-zero
	for _, status := range pipeStatus {
		if status != 0 {
			// Build the error message
			msg := "assert: command failed"
			if len(args) > 0 {
				msg = strings.Join(args, " ")
			}
			// Call die with the message
			return h.Die([]string{msg})
		}
	}

	// All commands succeeded
	return nil
}

// Nonfatal runs a command with die suppressed.
//
// Usage: nonfatal <command> [args...]
//
// Per PMS Section 12.3.1: Takes one or more arguments and executes them as
// a command, preserving the exit status. If this results in a command being
// called that would normally abort the build process due to a failure, instead
// a non-zero exit status shall be returned.
//
// Available in EAPI 4+ only. In EAPI 7+ it must be both a shell function and
// an external command for xargs compatibility.
//
// Example:
//
//	if nonfatal emake check; then
//	    einfo "Tests passed"
//	else
//	    ewarn "Tests failed, continuing anyway"
//	fi
//
//	nonfatal dodoc README || ewarn "README not found"
func (h *Helpers) Nonfatal(args []string) error {
	// Check EAPI support
	if h.env != nil && !h.env.EAPIFeatures.SupportsNonfatal() {
		return fmt.Errorf("nonfatal: not available in EAPI %s (requires EAPI 4+)", h.env.EAPI)
	}

	// Require at least one argument (the command to run)
	if len(args) == 0 {
		h.SetLastExitStatus(1)
		return fmt.Errorf("nonfatal: requires a command")
	}

	// Check that command dispatcher is set
	if h.commandDispatcher == nil {
		h.SetLastExitStatus(1)
		return fmt.Errorf("nonfatal: command dispatcher not configured")
	}

	// Set nonfatal mode before executing command
	h.SetNonfatalMode(true)
	defer h.SetNonfatalMode(false)

	// Execute the command through the dispatcher
	cmd := args[0]
	cmdArgs := args[1:]

	err := h.commandDispatcher(cmd, cmdArgs)

	// Handle the result
	if err != nil {
		// Check if it's a DieError - convert to non-fatal exit status
		var dieErr *DieError
		if errors.As(err, &dieErr) {
			h.SetLastExitStatus(1)
			return interp.ExitStatus(1)
		}

		// Check if it's already an exit status
		var exitStatus interp.ExitStatus
		if errors.As(err, &exitStatus) {
			h.SetLastExitStatus(int(exitStatus))
			return err
		}

		// Other errors - set exit status 1
		h.SetLastExitStatus(1)
		return interp.ExitStatus(1)
	}

	// Success
	h.SetLastExitStatus(0)
	return nil
}

// Einfon prints an informational message without a trailing newline.
//
// Usage: einfon "message"
//
// Per PMS Section 12.3.5: Same as einfo but without trailing newline.
func (h *Helpers) Einfon(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStdout(colorGreen + " * " + colorReset + msg)
	return nil
}
