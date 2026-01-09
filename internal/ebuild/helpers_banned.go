// Package ebuild implements ebuild execution engine.
//
// This file provides banned command validation per PMS Section 12.3.2.
// Certain commands are banned in certain EAPIs to maintain compatibility
// and enforce best practices.
package ebuild

import (
	"fmt"
)

// bannedCommands maps EAPI versions to sets of banned commands.
// Per PMS Table 12.3, commands are progressively banned in newer EAPIs.
//
// EAPI 4: dohard, dosed banned
// EAPI 5: adds useq
// EAPI 6: adds einstall
// EAPI 7: adds dohtml, dolib, libopts, hasv, hasq
// EAPI 8: same as 7, plus useq, hasv, hasq explicitly banned
//
// Note: The PMS table shows when commands BECOME banned (Yes = banned).
var bannedCommands = map[string]map[string]bool{
	"0": {},
	"1": {},
	"2": {},
	"3": {},
	"4": {
		"dohard": true,
		"dosed":  true,
	},
	"5": {
		"dohard": true,
		"dosed":  true,
		"useq":   true,
	},
	"6": {
		"dohard":   true,
		"dosed":    true,
		"useq":     true,
		"einstall": true,
	},
	"7": {
		"dohard":   true,
		"dosed":    true,
		"useq":     true,
		"einstall": true,
		"dohtml":   true,
		"dolib":    true,
		"libopts":  true,
		// Note: hasv and hasq are NOT banned until EAPI 8
	},
	"8": {
		"dohard":   true,
		"dosed":    true,
		"useq":     true,
		"einstall": true,
		"dohtml":   true,
		"dolib":    true,
		"libopts":  true,
		"hasv":     true,
		"hasq":     true,
	},
}

// BannedCommandError is returned when a banned command is called.
// This error indicates that the ebuild used a command that is not
// available in its declared EAPI.
type BannedCommandError struct {
	Command string
	EAPI    string
}

func (e *BannedCommandError) Error() string {
	return fmt.Sprintf("%s: banned in EAPI %s", e.Command, e.EAPI)
}

// IsBannedCommand checks if a command is banned in the given EAPI.
//
// Per PMS Section 12.3.2: Some commands are banned in some EAPIs.
// If a banned command is called, the package manager must abort
// the build process indicating an error.
//
// Parameters:
//   - eapi: The EAPI version string (e.g., "7", "8")
//   - command: The command name to check (e.g., "dohtml", "dolib")
//
// Returns true if the command is banned in the given EAPI.
func IsBannedCommand(eapi, command string) bool {
	banned, ok := bannedCommands[eapi]
	if !ok {
		// Unknown EAPI - assume no commands are banned
		// (EAPI validation should catch invalid EAPIs elsewhere)
		return false
	}
	return banned[command]
}

// GetBannedCommands returns the set of banned commands for a given EAPI.
//
// Returns a map where keys are banned command names and values are true.
// Returns an empty map for unknown EAPIs.
func GetBannedCommands(eapi string) map[string]bool {
	banned, ok := bannedCommands[eapi]
	if !ok {
		return map[string]bool{}
	}
	// Return a copy to prevent modification
	result := make(map[string]bool, len(banned))
	for k, v := range banned {
		result[k] = v
	}
	return result
}

// CheckBannedCommand checks if a command is banned and returns an error if so.
//
// This method checks if the given command is banned in the current EAPI
// (from the Helpers environment) and returns a BannedCommandError if banned.
//
// Per PMS Section 12.3.2: If a banned command is called, the package manager
// must abort the build process indicating an error.
//
// Example:
//
//	if err := h.CheckBannedCommand("dohtml"); err != nil {
//	    return err // Command is banned
//	}
//	// Command is allowed, proceed with execution
func (h *Helpers) CheckBannedCommand(command string) error {
	if h.env == nil {
		return nil
	}
	if IsBannedCommand(h.env.EAPI, command) {
		return &BannedCommandError{
			Command: command,
			EAPI:    h.env.EAPI,
		}
	}
	return nil
}

// --- Banned Command Stub Implementations ---
//
// These stubs exist to provide clear error messages when ebuilds
// try to use banned commands. They check EAPI compliance and return
// appropriate errors.

// Dohard is banned in EAPI 4+.
//
// Per PMS Section 12.3.2 / Table 12.3: dohard is banned starting from EAPI 4.
// Use ln or dosym instead.
func (h *Helpers) Dohard(args []string) error {
	if err := h.CheckBannedCommand("dohard"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-3: Would implement actual functionality here
	// For now, return not implemented for old EAPIs
	return fmt.Errorf("dohard: not implemented for EAPI %s", h.env.EAPI)
}

// Dosed is banned in EAPI 4+.
//
// Per PMS Section 12.3.2 / Table 12.3: dosed is banned starting from EAPI 4.
// Use sed directly instead.
func (h *Helpers) Dosed(args []string) error {
	if err := h.CheckBannedCommand("dosed"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-3: Would implement actual functionality here
	return fmt.Errorf("dosed: not implemented for EAPI %s", h.env.EAPI)
}

// Useq is banned in EAPI 5+.
//
// Per PMS Section 12.3.2 / Table 12.3: useq is banned starting from EAPI 5.
// It was a deprecated synonym for use.
// Use the 'use' command instead.
func (h *Helpers) Useq(args []string) error {
	if err := h.CheckBannedCommand("useq"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-4: Deprecated synonym for use
	return h.Use(args)
}

// Einstall is banned in EAPI 6+.
//
// Per PMS Section 12.3.2 / Table 12.3: einstall is banned starting from EAPI 6.
// Use emake install or einstalldocs instead.
func (h *Helpers) Einstall(args []string) error {
	if err := h.CheckBannedCommand("einstall"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-5: Would implement actual functionality here
	// einstall was a shortcut for emake with predefined install paths
	return fmt.Errorf("einstall: not implemented for EAPI %s", h.env.EAPI)
}

// Dohtml is banned in EAPI 7+.
//
// Per PMS Section 12.3.2 / Table 12.3: dohtml is banned starting from EAPI 7.
// Use dodoc -r instead, or install HTML files manually with doins.
func (h *Helpers) Dohtml(args []string) error {
	if err := h.CheckBannedCommand("dohtml"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-6: Would implement actual functionality here
	return fmt.Errorf("dohtml: not implemented for EAPI %s", h.env.EAPI)
}

// DolibBanned is the banned version of dolib for EAPI 7+.
//
// Per PMS Section 12.3.2 / Table 12.3: dolib is banned starting from EAPI 7.
// Use dolib.a or dolib.so instead with explicit file type.
//
// Note: The actual Dolib helper in helpers_install.go handles EAPI 0-6.
// This function is registered in the banned command map for EAPI 7+.
func (h *Helpers) DolibBanned(args []string) error {
	if err := h.CheckBannedCommand("dolib"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// Should not reach here - dolib is banned
	return fmt.Errorf("dolib: not implemented for EAPI %s", h.env.EAPI)
}

// Libopts is banned in EAPI 7+.
//
// Per PMS Section 12.3.2 / Table 12.3: libopts is banned starting from EAPI 7.
// It was used to set options for dolib. With dolib banned, libopts is also banned.
func (h *Helpers) Libopts(args []string) error {
	if err := h.CheckBannedCommand("libopts"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-6: Would implement actual functionality here
	return fmt.Errorf("libopts: not implemented for EAPI %s", h.env.EAPI)
}

// Hasv is banned in EAPI 8+.
//
// Per PMS Section 12.3.2 / Table 12.3: hasv is banned starting from EAPI 8.
// It was the verbose version of has. Use has and handle output separately.
func (h *Helpers) Hasv(args []string) error {
	if err := h.CheckBannedCommand("hasv"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-7: hasv is same as has but prints the first argument if found
	if len(args) < 2 {
		return nil // false
	}
	needle := args[0]
	haystack := args[1:]
	for _, item := range haystack {
		if item == needle {
			h.writeStdout(needle)
			return nil
		}
	}
	return exitFalse()
}

// Hasq is banned in EAPI 8+.
//
// Per PMS Section 12.3.2 / Table 12.3: hasq is banned starting from EAPI 8.
// It was a deprecated synonym for has. Use has instead.
func (h *Helpers) Hasq(args []string) error {
	if err := h.CheckBannedCommand("hasq"); err != nil {
		return &DieError{Message: err.Error()}
	}
	// EAPI 0-7: Deprecated synonym for has
	return h.Has(args)
}
