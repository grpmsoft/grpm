// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 messaging functions (die, einfo, ewarn, etc.).
package ebuild

import (
	"strings"
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
func (h *Helpers) Die(args []string) error {
	msg := strings.Join(args, " ")
	h.writeStderr(colorRed + " * " + colorReset + "ERROR: " + msg + "\n")
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
