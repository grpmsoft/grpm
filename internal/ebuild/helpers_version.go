// Package ebuild implements ebuild execution engine.
//
// This file provides EAPI 7+ version manipulation functions (ver_cut, ver_rs, ver_test).
package ebuild

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// ============================================================================
// EAPI 8 Version Functions
// ============================================================================

// VerCut extracts version components.
//
// Usage: ver_cut 1 1.2.3     -> 1
// Usage: ver_cut 1-2 1.2.3   -> 1.2
// Usage: ver_cut 2- 1.2.3    -> 2.3
//
// Gentoo version cutting utility.
func (h *Helpers) VerCut(args []string) error {
	if len(args) < 2 {
		return &DieError{Message: "ver_cut: requires range and version arguments"}
	}

	rangeSpec := args[0]
	version := args[1]

	result, err := h.verCutImpl(rangeSpec, version)
	if err != nil {
		return &DieError{Message: fmt.Sprintf("ver_cut: %v", err)}
	}

	h.writeStdout(result)
	return nil
}

// verCutImpl implements version cutting logic.
func (h *Helpers) verCutImpl(rangeSpec, version string) (string, error) {
	// Split version into components
	parts := h.splitVersion(version)
	if len(parts) == 0 {
		return "", nil
	}

	// Parse range
	start, end, err := h.parseVerRange(rangeSpec, len(parts))
	if err != nil {
		return "", err
	}

	// Extract requested parts
	if start > len(parts) {
		return "", nil
	}
	if end > len(parts) {
		end = len(parts)
	}

	return strings.Join(parts[start-1:end], "."), nil
}

// splitVersion splits a version string into components.
func (h *Helpers) splitVersion(version string) []string {
	// Split on . - _ characters
	var parts []string
	var current strings.Builder

	for _, r := range version {
		if r == '.' || r == '-' || r == '_' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseVerRange parses a range specification like "1", "1-2", "2-".
func (h *Helpers) parseVerRange(rangeSpec string, maxParts int) (int, int, error) {
	if strings.Contains(rangeSpec, "-") {
		parts := strings.SplitN(rangeSpec, "-", 2)
		start := 1
		end := maxParts

		if parts[0] != "" {
			var err error
			start, err = strconv.Atoi(parts[0])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid start: %s", parts[0])
			}
		}

		if parts[1] != "" {
			var err error
			end, err = strconv.Atoi(parts[1])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid end: %s", parts[1])
			}
		}

		return start, end, nil
	}

	// Single number
	n, err := strconv.Atoi(rangeSpec)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range: %s", rangeSpec)
	}

	return n, n, nil
}

// VerRs replaces version separators.
//
// Usage: ver_rs 1-2 . 1_2_3  -> 1.2.3
//
// Replaces separators at specified positions.
func (h *Helpers) VerRs(args []string) error {
	if len(args) < 3 {
		return &DieError{Message: "ver_rs: requires range, separator, and version arguments"}
	}

	rangeSpec := args[0]
	newSep := args[1]
	version := args[2]

	result := h.verRsImpl(rangeSpec, newSep, version)
	h.writeStdout(result)
	return nil
}

// verRsImpl implements separator replacement logic.
func (h *Helpers) verRsImpl(rangeSpec, newSep, version string) string {
	// Find separator positions
	var sepPositions []int
	for i, r := range version {
		if r == '.' || r == '-' || r == '_' {
			sepPositions = append(sepPositions, i)
		}
	}

	if len(sepPositions) == 0 {
		return version
	}

	// Parse range
	start, end, err := h.parseVerRange(rangeSpec, len(sepPositions))
	if err != nil {
		return version
	}

	// Replace separators at specified positions
	result := []byte(version)
	for i := start - 1; i < end && i < len(sepPositions); i++ {
		pos := sepPositions[i]
		result[pos] = []byte(newSep)[0]
	}

	return string(result)
}

// VerTest compares two version strings per PMS Section 12.3.14.
//
// Usage: ver_test <v1> <op> <v2>
//
// Operators:
//   - -eq: v1 equals v2
//   - -ne: v1 not equals v2
//   - -lt: v1 less than v2
//   - -le: v1 less than or equal to v2
//   - -gt: v1 greater than v2
//   - -ge: v1 greater than or equal to v2
//
// Returns: exit code 0 (true) or 1 (false) via exitFalse().
// Available in EAPI 7+.
func (h *Helpers) VerTest(args []string) error {
	if len(args) != 3 {
		return &DieError{Message: "ver_test: requires exactly 3 arguments: <v1> <op> <v2>"}
	}

	v1, op, v2 := args[0], args[1], args[2]

	// Use PMS-compliant version comparison from pkg package
	cmp := pkg.CompareVersions(v1, v2)

	var result bool
	switch op {
	case "-eq":
		result = cmp == 0
	case "-ne":
		result = cmp != 0
	case "-lt":
		result = cmp < 0
	case "-le":
		result = cmp <= 0
	case "-gt":
		result = cmp > 0
	case "-ge":
		result = cmp >= 0
	default:
		return &DieError{Message: fmt.Sprintf("ver_test: unknown operator: %s", op)}
	}

	if !result {
		return exitFalse()
	}
	return nil
}

// ============================================================================
// EAPI 8 Utility Functions
// ============================================================================

// GetFilesDir returns the ebuild FILESDIR path.
//
// Usage: get_filesdir
//
// Returns path to files/ directory in ebuild directory.
func (h *Helpers) GetFilesDir(args []string) error {
	if h.env == nil {
		return &DieError{Message: "get_filesdir: environment not set"}
	}

	filesDir := filepath.Join(h.env.PORTDIR, h.env.CATEGORY, h.env.PN, "files")
	h.writeStdout(filesDir)
	return nil
}

// Inherit handles eclass inheritance.
//
// Usage: inherit eclass1 eclass2
//
// Loads one or more eclasses by sourcing their bash files through the interpreter.
// Eclass functions and variables become available in the current execution context.
// The INHERITED variable is updated to track all inherited eclasses.
//
// Eclass files are loaded from:
//   - ${PORTDIR}/eclass/${name}.eclass (primary)
//   - /var/db/repos/gentoo/eclass/${name}.eclass (fallback)
//
// Some eclasses have built-in Go implementations for common functions:
//   - toolchain-funcs (tc-getCC, tc-getCXX, etc.)
//   - eutils (epatch delegates to eapply)
//   - multilib (get_libdir, etc.)
//   - flag-o-matic (append-flags, filter-flags, etc.)
//   - linux-info (get_version, linux_config_exists, etc.)
func (h *Helpers) Inherit(args []string) error {
	if len(args) == 0 {
		return nil
	}

	// Check if eclass loader is available
	if h.eclassLoader == nil {
		// Fallback to stub behavior if no loader is wired up
		for _, eclass := range args {
			h.writeStdout(fmt.Sprintf(">>> Inheriting eclass: %s (no loader)\n", eclass))
		}
		return nil
	}

	// Use background context since inherit is called from bash scripts
	// which don't have context propagation
	ctx := context.Background()

	// Load all requested eclasses
	if err := h.eclassLoader.Inherit(ctx, args); err != nil {
		return &DieError{Message: fmt.Sprintf("inherit failed: %v", err)}
	}

	return nil
}
