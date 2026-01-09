// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 8 USE flag functions (use, usev, usex, etc.).
package ebuild

import (
	"strings"
)

// ============================================================================
// EAPI 8 USE Flag Functions
// ============================================================================

// Has checks if a value exists in a list.
//
// Usage: has value item1 item2 item3 ...
//
// Returns exit code 0 if found, 1 if not found.
func (h *Helpers) Has(args []string) error {
	if len(args) < 2 {
		return exitFalse()
	}

	needle := args[0]
	haystack := args[1:]

	for _, item := range haystack {
		if item == needle {
			return nil // Found - exit code 0
		}
	}

	return exitFalse() // Not found - exit code 1
}

// Use checks if a USE flag is enabled.
//
// Usage: use flagname
//
// Returns exit code 0 if flag is enabled, 1 if disabled or not in IUSE.
func (h *Helpers) Use(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]

	// Handle negation (use !flag)
	negate := false
	if strings.HasPrefix(flag, "!") {
		negate = true
		flag = flag[1:]
	}

	enabled := h.isUseEnabled(flag)

	if negate {
		enabled = !enabled
	}

	if enabled {
		return nil // Exit code 0
	}
	return exitFalse() // Exit code 1
}

// Usev prints the flag name if it's enabled.
//
// Usage: usev flagname [value]
//
// If flag is enabled, prints value (default: flagname).
// Returns exit code 0 if flag is enabled, 1 otherwise.
func (h *Helpers) Usev(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]
	value := flag
	if len(args) >= 2 {
		value = args[1]
	}

	if h.isUseEnabled(flag) {
		h.writeStdout(value)
		return nil
	}

	return exitFalse()
}

// Usex outputs conditional values based on USE flag state.
//
// Usage: usex flag [true] [false] [trueSuffix] [falseSuffix]
//
// Outputs:
//   - If flag enabled: true + trueSuffix (default: "yes")
//   - If flag disabled: false + falseSuffix (default: "no")
func (h *Helpers) Usex(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]
	trueVal := "yes"
	falseVal := "no"
	trueSuffix := ""
	falseSuffix := ""

	if len(args) >= 2 {
		trueVal = args[1]
	}
	if len(args) >= 3 {
		falseVal = args[2]
	}
	if len(args) >= 4 {
		trueSuffix = args[3]
	}
	if len(args) >= 5 {
		falseSuffix = args[4]
	}

	if h.isUseEnabled(flag) {
		h.writeStdout(trueVal + trueSuffix)
	} else {
		h.writeStdout(falseVal + falseSuffix)
	}

	return nil
}

// InIuse checks if a flag is declared in IUSE.
//
// Usage: in_iuse flagname
//
// Returns exit code 0 if flag is in IUSE, 1 otherwise.
func (h *Helpers) InIuse(args []string) error {
	if len(args) < 1 {
		return exitFalse()
	}

	flag := args[0]

	if h.isInIuse(flag) {
		return nil
	}

	return exitFalse()
}

// isUseEnabled checks if a USE flag is enabled.
func (h *Helpers) isUseEnabled(flag string) bool {
	if h.env == nil {
		return false
	}

	// Check in Package.UseFlags first (preferred)
	if h.env.Package != nil {
		if enabled, exists := h.env.Package.UseFlags[flag]; exists {
			return enabled
		}
	}

	// Fall back to USE environment variable
	useFlags := strings.Fields(h.env.USE)
	for _, f := range useFlags {
		if f == flag {
			return true
		}
		// Handle -flag (disabled)
		if f == "-"+flag {
			return false
		}
	}

	return false
}

// isInIuse checks if a flag is declared in IUSE.
func (h *Helpers) isInIuse(flag string) bool {
	if h.env == nil {
		return false
	}

	// Check in Package.UseFlags (all keys are valid IUSE)
	if h.env.Package != nil {
		if _, exists := h.env.Package.UseFlags[flag]; exists {
			return true
		}
	}

	// Fall back to IUSE environment variable (not in Environment struct currently)
	// TODO: Add IUSE field to Environment struct
	return false
}
