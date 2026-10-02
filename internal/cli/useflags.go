// Package cli implements CLI commands for GRPM.
//
// This file contains USE flag display formatting for emerge --pretend output.
// It implements Portage-compatible USE flag display showing enabled flags
// without prefix and disabled flags with "-" prefix.
package cli

import (
	"sort"
	"strings"

	"github.com/grpmsoft/grpm/internal/config"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// Common USE_EXPAND prefixes that should be displayed separately.
// These are standard Portage USE_EXPAND variables.
var defaultUSEExpandPrefixes = []string{
	"abi_mips_",
	"abi_s390_",
	"abi_x86_",
	"cpu_flags_x86_",
	"cpu_flags_arm_",
	"input_devices_",
	"l10n_",
	"llvm_targets_",
	"lua_single_target_",
	"lua_targets_",
	"python_single_target_",
	"python_targets_",
	"ruby_targets_",
	"video_cards_",
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

	// Remember which USE_EXPAND prefixes the package originally has in IUSE.
	// This prevents showing PYTHON_TARGETS on non-Python packages.
	originalExpandPrefixes := make(map[string]bool)
	for flag := range p.UseFlags {
		if prefix := getUSEExpandPrefix(flag); prefix != "" {
			originalExpandPrefixes[prefix] = true
		}
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
			// Only show USE_EXPAND flags for prefixes in the original IUSE
			if !originalExpandPrefixes[prefix] {
				continue
			}
			shortFlag := strings.TrimPrefix(flag, prefix)
			expandEnabled[prefix] = append(expandEnabled[prefix], shortFlag)
		} else {
			regularEnabled = append(regularEnabled, flag)
		}
	}

	for _, flag := range disabled {
		if prefix := getUSEExpandPrefix(flag); prefix != "" {
			// Only show USE_EXPAND flags for prefixes in the original IUSE
			if !originalExpandPrefixes[prefix] {
				continue
			}
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

// cliUSEConfig adapts config.Config to the pkg.USEResolverConfig interface
// for CLI-layer USE resolution. The CLI layer typically does not have access
// to the profile (it was not wired through before this unification), so
// ProfileUSEFlags returns nil. When profile support is added to the CLI,
// it can be passed here.
type cliUSEConfig struct {
	cfg *config.Config
}

func (c *cliUSEConfig) ProfileUSEFlags() []string { return nil }
func (c *cliUSEConfig) GlobalUSEFlags() []string {
	if c.cfg == nil || c.cfg.MakeConf == nil {
		return nil
	}
	return c.cfg.MakeConf.USE
}
func (c *cliUSEConfig) PackageUSEFlags(category, name, version, slot string) []string {
	if c.cfg == nil {
		return nil
	}
	return c.cfg.GetPackageUSEForPackage(category, name, version, slot)
}
func (c *cliUSEConfig) USEExpandValue(varName string) string {
	if c.cfg == nil {
		return ""
	}
	return c.cfg.GetVariable(varName)
}

// resolvePackageUSE determines the effective USE flags for a package.
// Delegates to the unified pkg.ResolveEffectiveUSE for the canonical
// Portage priority chain, then filters to only include flags in IUSE.
//
// Returns two slices: enabled flags and disabled flags.
// Only flags present in IUSE (p.UseFlags keys) are returned.
func resolvePackageUSE(p *pkg.Package, cfg *config.Config) (enabled, disabled []string) {
	if p == nil || len(p.UseFlags) == 0 {
		return nil, nil
	}

	// Extract category and package name
	category := ""
	pkgName := p.Name
	if parts := strings.SplitN(p.Name, "/", 2); len(parts) == 2 {
		category = parts[0]
		pkgName = parts[1]
	}

	// Build IUSE defaults map from p.UseFlags (the keys are the IUSE flags,
	// the values represent whether they were declared with + prefix).
	// Note: by the time we reach CLI, portage.go has already resolved
	// effective USE into p.UseFlags values. For display purposes we treat
	// the UseFlags map as the IUSE set and re-resolve from config.
	iuseDefaults := make(map[string]bool, len(p.UseFlags))
	for flag := range p.UseFlags {
		// UseFlags values at this point reflect parsed IUSE defaults
		// (true = +flag in IUSE, false = bare or -flag)
		iuseDefaults[flag] = false
	}

	// Resolve via unified service
	var resolverCfg pkg.USEResolverConfig
	if cfg != nil {
		resolverCfg = &cliUSEConfig{cfg: cfg}
	}

	effectiveUSE := pkg.ResolveEffectiveUSE(iuseDefaults, category, pkgName,
		p.Version, p.Slot.String(), resolverCfg)

	// Register USE_EXPAND flags into IUSE if not present
	// (eclasses add python_targets_* dynamically, our IUSE may be incomplete)
	for flag := range effectiveUSE {
		if _, inIUSE := p.UseFlags[flag]; !inIUSE {
			if prefix := getUSEExpandPrefix(flag); prefix != "" {
				// This is a USE_EXPAND flag added by the resolver; include it
				p.UseFlags[flag] = true
			}
		}
	}

	// Split into enabled/disabled, filtered to IUSE flags only
	return pkg.SplitEnabledDisabled(effectiveUSE, p.UseFlags)
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

// ApplyEffectiveUSE resolves the effective USE flags for a package and updates
// p.UseFlags in-place. This must be called before building a package so that
// NewEnvironment() picks up the correct USE flags including USE_EXPAND conversions.
//
// Delegates to the unified pkg.ResolveEffectiveUSE for consistent resolution.
//
// Per Portage behavior: USE_EXPAND variables from make.conf (e.g.,
// PYTHON_TARGETS="python3_12") are converted to USE flags
// (python_targets_python3_12) and also set as separate env variables.
func ApplyEffectiveUSE(p *pkg.Package, cfg *config.Config) {
	if p == nil {
		return
	}

	// Resolve effective USE flags using the unified resolver
	enabled, _ := resolvePackageUSE(p, cfg)

	// Update p.UseFlags: set all to false first, then enable resolved ones
	for flag := range p.UseFlags {
		p.UseFlags[flag] = false
	}
	for _, flag := range enabled {
		p.UseFlags[flag] = true
	}
}

// GetUSEExpandVars returns USE_EXPAND variables and their values from config.
// These should be set as environment variables alongside USE flags.
// For example: PYTHON_TARGETS="python3_11 python3_12"
func GetUSEExpandVars(cfg *config.Config) map[string]string {
	if cfg == nil {
		return nil
	}

	result := make(map[string]string)
	for _, varName := range pkg.USEExpandVars {
		value := cfg.GetVariable(varName)
		if value != "" {
			result[varName] = value
		}
	}
	return result
}

// FormatUSEFlagsFromEbuild formats USE flags by also reading IUSE defaults from ebuild.
// This provides more accurate formatting by respecting IUSE +flag defaults.
// Currently delegates to FormatUSEFlags since the unified resolver handles
// IUSE defaults at parse time.
func FormatUSEFlagsFromEbuild(p *pkg.Package, cfg *config.Config, _ string) string {
	return FormatUSEFlags(p, cfg)
}
