// Package cli implements CLI commands for GRPM.
//
// This file contains USE flag display formatting for emerge --pretend output.
// It implements Portage-compatible USE flag display showing enabled flags
// without prefix and disabled flags with "-" prefix.
package cli

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// Common USE_EXPAND prefixes that should be displayed separately.
// These are standard Portage USE_EXPAND variables.
var defaultUSEExpandPrefixes = []string{
	"cpu_flags_x86_",
	"cpu_flags_arm_",
	"python_targets_",
	"python_single_target_",
	"ruby_targets_",
	"lua_targets_",
	"l10n_",
	"video_cards_",
	"input_devices_",
}

// FormatUSEFlags formats USE flags for display in emerge --pretend output.
//
// The format matches Portage's output:
//   - Enabled flags are shown without prefix
//   - Disabled flags are shown with "-" prefix
//   - USE_EXPAND flags (like PYTHON_TARGETS) are separated into their own variables
//   - Flags are sorted: enabled first, then disabled, alphabetically within each group
//
// Example output: USE="nls -doc -test" PYTHON_TARGETS="python3_11 python3_12"
func FormatUSEFlags(p *pkg.Package, cfg *config.Config) string {
	if p == nil {
		return `USE=""`
	}

	// Get effective USE flags for this package
	enabled, disabled := resolvePackageUSE(p, cfg)

	// Separate USE_EXPAND flags from regular USE flags
	regularEnabled := make([]string, 0)
	regularDisabled := make([]string, 0)
	expandEnabled := make(map[string][]string)  // prefix -> flags
	expandDisabled := make(map[string][]string) // prefix -> flags

	for _, flag := range enabled {
		if prefix := getUSEExpandPrefix(flag); prefix != "" {
			shortFlag := strings.TrimPrefix(flag, prefix)
			expandEnabled[prefix] = append(expandEnabled[prefix], shortFlag)
		} else {
			regularEnabled = append(regularEnabled, flag)
		}
	}

	for _, flag := range disabled {
		if prefix := getUSEExpandPrefix(flag); prefix != "" {
			shortFlag := strings.TrimPrefix(flag, prefix)
			expandDisabled[prefix] = append(expandDisabled[prefix], shortFlag)
		} else {
			regularDisabled = append(regularDisabled, flag)
		}
	}

	// Build output parts
	var parts []string

	// Regular USE flags (sorted: enabled first, then disabled)
	sort.Strings(regularEnabled)
	sort.Strings(regularDisabled)

	var useFlags []string
	useFlags = append(useFlags, regularEnabled...)
	for _, f := range regularDisabled {
		useFlags = append(useFlags, "-"+f)
	}

	parts = append(parts, `USE="`+strings.Join(useFlags, " ")+`"`)

	// USE_EXPAND variables (sorted by variable name)
	expandVars := make([]string, 0, len(expandEnabled)+len(expandDisabled))
	for prefix := range expandEnabled {
		expandVars = append(expandVars, prefix)
	}
	for prefix := range expandDisabled {
		if _, ok := expandEnabled[prefix]; !ok {
			expandVars = append(expandVars, prefix)
		}
	}
	sort.Strings(expandVars)

	for _, prefix := range expandVars {
		varName := prefixToVarName(prefix)

		// Sort flags within each USE_EXPAND variable
		enabled := expandEnabled[prefix]
		disabled := expandDisabled[prefix]
		sort.Strings(enabled)
		sort.Strings(disabled)

		var flags []string
		flags = append(flags, enabled...)
		for _, f := range disabled {
			flags = append(flags, "-"+f)
		}

		if len(flags) > 0 {
			parts = append(parts, varName+`="`+strings.Join(flags, " ")+`"`)
		}
	}

	return strings.Join(parts, " ")
}

// resolvePackageUSE determines the effective USE flags for a package.
//
// Resolution order (later overrides earlier):
//  1. IUSE defaults from ebuild (+flag enables, -flag disables)
//  2. Global USE from make.conf
//  3. Profile USE defaults (not implemented yet)
//  4. Per-package USE from package.use
//
// Returns two slices: enabled flags and disabled flags.
// Only flags present in IUSE are returned.
func resolvePackageUSE(p *pkg.Package, cfg *config.Config) (enabled, disabled []string) {
	if p == nil || len(p.UseFlags) == 0 {
		return nil, nil
	}

	// Start with IUSE defaults
	// The UseFlags map contains flags from IUSE with + and - prefixes stripped.
	// Currently, the parser sets all flags to true (enabled by default in IUSE).
	// We need to re-parse IUSE to get the real defaults.
	flagState := make(map[string]bool) // true = enabled, false = disabled

	// Initialize from package's UseFlags (IUSE)
	// By default, all IUSE flags are disabled unless they have + prefix
	for flag := range p.UseFlags {
		flagState[flag] = false
	}

	// Apply global USE from make.conf
	if cfg != nil && cfg.MakeConf != nil {
		for _, flag := range cfg.MakeConf.USE {
			if strings.HasPrefix(flag, "-") {
				baseFlag := strings.TrimPrefix(flag, "-")
				// Only apply if flag is in IUSE
				if _, exists := p.UseFlags[baseFlag]; exists {
					flagState[baseFlag] = false
				}
			} else {
				// Only apply if flag is in IUSE
				if _, exists := p.UseFlags[flag]; exists {
					flagState[flag] = true
				}
			}
		}
	}

	// Apply per-package USE from package.use
	if cfg != nil {
		// Extract category and package name for pattern matching
		category := ""
		pkgName := p.Name
		if parts := strings.SplitN(p.Name, "/", 2); len(parts) == 2 {
			category = parts[0]
			pkgName = parts[1]
		}

		// Get package-specific USE flags with pattern matching
		pkgUSE := cfg.GetPackageUSEForPackage(category, pkgName, p.Version, p.Slot.String())
		for _, flag := range pkgUSE {
			if strings.HasPrefix(flag, "-") {
				baseFlag := strings.TrimPrefix(flag, "-")
				// Only apply if flag is in IUSE
				if _, exists := p.UseFlags[baseFlag]; exists {
					flagState[baseFlag] = false
				}
			} else {
				// Only apply if flag is in IUSE
				if _, exists := p.UseFlags[flag]; exists {
					flagState[flag] = true
				}
			}
		}
	}

	// Build result slices
	for flag, isEnabled := range flagState {
		if isEnabled {
			enabled = append(enabled, flag)
		} else {
			disabled = append(disabled, flag)
		}
	}

	return enabled, disabled
}

// getUSEExpandPrefix returns the USE_EXPAND prefix if the flag matches one,
// or empty string otherwise.
//
// Example: "python_targets_python3_11" returns "python_targets_"
func getUSEExpandPrefix(flag string) string {
	for _, prefix := range defaultUSEExpandPrefixes {
		if strings.HasPrefix(flag, prefix) {
			return prefix
		}
	}
	return ""
}

// prefixToVarName converts a USE_EXPAND prefix to the variable name.
//
// Example: "python_targets_" -> "PYTHON_TARGETS"
func prefixToVarName(prefix string) string {
	// Remove trailing underscore and convert to uppercase
	name := strings.TrimSuffix(prefix, "_")
	return strings.ToUpper(name)
}

// getEbuildIUSEDefaults reads the ebuild file and extracts IUSE defaults.
// Returns a map of flag name -> default enabled state.
//
// IUSE format:
//   - "flag" -> disabled by default
//   - "+flag" -> enabled by default
//   - "-flag" -> disabled by default (explicit)
func getEbuildIUSEDefaults(ebuildPath string) map[string]bool {
	// This is a placeholder for future enhancement.
	// Currently, the portage.go parser strips +/- prefixes from IUSE,
	// so we don't have access to the defaults.
	// For now, we assume all flags are disabled by default (Portage behavior).
	return nil
}

// FormatUSEFlagsFromEbuild formats USE flags by also reading IUSE defaults from ebuild.
// This provides more accurate formatting by respecting IUSE +flag defaults.
func FormatUSEFlagsFromEbuild(p *pkg.Package, cfg *config.Config, repoPath string) string {
	if p == nil {
		return `USE=""`
	}

	// Try to read ebuild for IUSE defaults
	// If we can't read it, fall back to FormatUSEFlags
	if repoPath == "" {
		return FormatUSEFlags(p, cfg)
	}

	// Find ebuild path
	category := ""
	pkgName := p.Name
	if parts := strings.SplitN(p.Name, "/", 2); len(parts) == 2 {
		category = parts[0]
		pkgName = parts[1]
	}

	ebuildPath := filepath.Join(repoPath, category, pkgName,
		pkgName+"-"+p.Version+".ebuild")

	// Get IUSE defaults (for future enhancement)
	_ = getEbuildIUSEDefaults(ebuildPath)

	// For now, use the standard formatter
	return FormatUSEFlags(p, cfg)
}
