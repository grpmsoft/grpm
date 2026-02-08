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
	"mvdan.cc/sh/v3/expand"
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

// verCutImpl implements version cutting logic matching Portage's ver_cut.
//
// Portage's __ver_split stores interleaved [separator, component] pairs:
//
//	"1.2.3" → ["", "1", ".", "2", ".", "3"]
//
// ver_cut extracts the slice and preserves original separators.
func (h *Helpers) verCutImpl(rangeSpec, version string) (string, error) {
	comp := verSplit(version)
	if len(comp) == 0 {
		return "", nil
	}

	max := len(comp) / 2

	start, end, err := h.parseVerRange(rangeSpec, max)
	if err != nil {
		return "", err
	}
	if start > max {
		return "", nil
	}
	if end > max {
		end = max
	}

	// Portage: echo "${comp[*]:start:end*2-start}"
	// start is 1-based component index → array index = start*2-1
	arrStart := start*2 - 1
	if start == 0 {
		arrStart = 0
	}
	arrLen := end*2 - arrStart
	if arrStart+arrLen > len(comp) {
		arrLen = len(comp) - arrStart
	}

	var sb strings.Builder
	for i := arrStart; i < arrStart+arrLen; i++ {
		sb.WriteString(comp[i])
	}
	return sb.String(), nil
}

// verSplit splits a version into interleaved [separator, component] pairs,
// matching Portage's __ver_split exactly.
//
// Example: "1.2_alpha3-r1" → ["", "1", ".", "2", "_", "alpha", "", "3", "-", "r", "", "1"]
func verSplit(version string) []string {
	var comp []string
	v := version

	for len(v) > 0 {
		// Cut the separator (non-alphanumeric prefix)
		sepEnd := 0
		for sepEnd < len(v) && !isAlphanumeric(v[sepEnd]) {
			sepEnd++
		}
		sep := v[:sepEnd]
		v = v[sepEnd:]

		// Cut the next component: either all digits or all letters
		compEnd := 0
		if len(v) > 0 && v[0] >= '0' && v[0] <= '9' {
			for compEnd < len(v) && v[compEnd] >= '0' && v[compEnd] <= '9' {
				compEnd++
			}
		} else {
			for compEnd < len(v) && isAlpha(v[compEnd]) {
				compEnd++
			}
		}
		c := v[:compEnd]
		v = v[compEnd:]

		comp = append(comp, sep, c)
	}
	return comp
}

func isAlphanumeric(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// splitVersion splits a version string into components (legacy, used by parseVerRange).
func (h *Helpers) splitVersion(version string) []string {
	comp := verSplit(version)
	var parts []string
	for i := 1; i < len(comp); i += 2 {
		if comp[i] != "" {
			parts = append(parts, comp[i])
		}
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

// verRsImpl implements separator replacement logic matching Portage's ver_rs.
//
// Portage ver_rs replaces separators at specified positions (1-indexed).
// Separator positions correspond to even indices in the verSplit array.
func (h *Helpers) verRsImpl(rangeSpec, newSep, version string) string {
	comp := verSplit(version)
	if len(comp) == 0 {
		return version
	}

	// max separator index = number of components - 1
	max := len(comp)/2 - 1
	if max < 1 {
		return version
	}

	start, end, err := h.parseVerRange(rangeSpec, max)
	if err != nil {
		return version
	}

	// Replace separators at specified positions
	// Separator at position N is at comp[N*2]
	for i := start; i <= end && i <= max; i++ {
		idx := i * 2
		if idx < len(comp) {
			// Skip position 0 with empty separator
			if idx == 0 && comp[idx] == "" {
				continue
			}
			comp[idx] = newSep
		}
	}

	var sb strings.Builder
	for _, s := range comp {
		sb.WriteString(s)
	}
	return sb.String()
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
	var v1, op, v2 string

	switch len(args) {
	case 3:
		v1, op, v2 = args[0], args[1], args[2]
	case 2:
		// 2-arg form: ver_test <op> <v2> — uses $PVR as implicit v1
		v1 = h.getEnvVar("PVR")
		if v1 == "" {
			v1 = h.getEnvVar("PV")
		}
		op, v2 = args[0], args[1]
	default:
		return &DieError{Message: "ver_test: requires 2 or 3 arguments"}
	}

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

// InheritWithEnv loads eclasses with access to the interpreter's current environment.
//
// This variant receives the environment from the interpreter context, allowing
// access to variables set during script execution (like EAPI).
func (h *Helpers) InheritWithEnv(args []string, env expand.Environ) error {
	if len(args) == 0 {
		return nil
	}

	// Check if eclass loader is available
	if h.eclassLoader == nil {
		for _, eclass := range args {
			h.writeStdout(fmt.Sprintf(">>> Inheriting eclass: %s (no loader)\n", eclass))
		}
		return nil
	}

	// Sync ALL runtime variables from the outer bash script to Environment.
	// Eclasses run in a nested interpreter that reads from buildEnvPairs(),
	// which uses Environment.ExtraVars. Variables set in the ebuild script
	// (e.g., DISTUTILS_USE_PEP517, PYTHON_COMPAT) must be propagated so
	// eclasses can see them at source time.
	if env != nil && h.env != nil {
		env.Each(func(name string, vr expand.Variable) bool {
			val := vr.String()
			if val != "" {
				h.env.SetVar(name, val)
			}
			return true
		})
	}

	ctx := context.Background()

	// For DynamicEclassLoader, also pass variables through its SetEnv mechanism
	if loader, ok := h.eclassLoader.(*DynamicEclassLoader); ok {
		envVars := map[string]string{}

		// Get variables from the interpreter's environment
		for _, varName := range []string{
			"EAPI", "P", "PN", "PV", "PR", "PVR", "PF",
			"CATEGORY", "SLOT", "USE", "PORTDIR", "DISTDIR",
			"WORKDIR", "S", "T", "D", "ROOT", "EROOT", "EPREFIX",
		} {
			if val := env.Get(varName).String(); val != "" {
				envVars[varName] = val
			}
		}

		loader.SetEnv(envVars)
	}

	// Load all requested eclasses
	if err := h.eclassLoader.Inherit(ctx, args); err != nil {
		return &DieError{Message: fmt.Sprintf("inherit failed: %v", err)}
	}

	return nil
}
