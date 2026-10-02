// Package pkg provides the canonical USE flag resolution for GRPM.
//
// USEResolver is the single source of truth for computing effective USE flags.
// All layers (repository parsing, SAT solver, CLI display) must delegate to
// this service to ensure consistent USE state across the entire pipeline.
//
// Resolution priority follows Portage (lowest to highest):
//  1. IUSE defaults (+flag = enabled, -flag or bare = disabled)
//  2. Profile USE flags (make.defaults only, NOT force/mask)
//  3. make.conf global USE flags
//  4. USE_EXPAND variables (PYTHON_TARGETS, etc.)
//  5. Per-package USE from package.use
//  6. use.force / use.mask (PMS: cannot be overridden by user config)
package pkg

import (
	"strings"
)

// USEResolverConfig provides the configuration sources for USE resolution.
// This interface breaks the dependency from the domain layer to infrastructure,
// keeping the domain layer independent of config/profile packages.
type USEResolverConfig interface {
	// ProfileUSEFlags returns USE flags from the system profile make.defaults.
	// Does NOT include use.force or use.mask — those are applied separately as
	// the final layer per PMS. Returns parent profiles first, then current.
	// Negated flags are returned with "-" prefix (e.g., "-doc").
	ProfileUSEFlags() []string

	// GlobalUSEFlags returns USE flags from make.conf global USE variable.
	// Negated flags are returned with "-" prefix (e.g., "-test").
	GlobalUSEFlags() []string

	// PackageUSEFlags returns per-package USE flags from package.use.
	// Matches by category, name, version, and slot with pattern support.
	// Negated flags are returned with "-" prefix (e.g., "-debug").
	PackageUSEFlags(category, name, version, slot string) []string

	// USEExpandValue returns the value of a USE_EXPAND variable.
	// First checks make.conf, then falls back to profile make.defaults.
	// Example: USEExpandValue("PYTHON_TARGETS") -> "python3_12 python3_13"
	// Returns empty string if not set.
	USEExpandValue(varName string) string

	// ForcedUSE returns flags from use.force (profile hierarchy).
	// These are unconditionally enabled regardless of user configuration.
	// Returns nil if not available (backward compat — force not applied).
	ForcedUSE() []string

	// MaskedUSE returns flags from use.mask (profile hierarchy).
	// These are unconditionally disabled regardless of user configuration.
	// Returns nil if not available (backward compat — mask not applied).
	MaskedUSE() []string
}

// USEExpandVars lists the USE_EXPAND variables that are expanded into USE flags.
// In Portage, each variable FOO_BAR="val1 val2" expands to USE flags
// foo_bar_val1, foo_bar_val2. This list covers the most commonly used
// variables in dependency conditions. Full USE_EXPAND support (ELIBC, KERNEL,
// ARCH, ABI_X86, etc.) requires implicit variable handling and is deferred.
var USEExpandVars = []string{
	"PYTHON_SINGLE_TARGET",
	"PYTHON_TARGETS",
	"LUA_SINGLE_TARGET",
	"LUA_TARGETS",
	"RUBY_TARGETS",
}

// ResolveEffectiveUSE computes the effective USE flags for a package.
//
// This is the CANONICAL implementation — all other layers must call this
// function (or wrap it) to determine which USE flags are active. This
// prevents the inconsistencies that arise when each layer re-implements
// the resolution logic independently.
//
// Parameters:
//   - iuseDefaults: map of IUSE flags to their default state.
//     true = flag has "+" prefix in IUSE (enabled by default).
//     false = flag has no prefix or "-" prefix (disabled by default).
//   - category: package category (e.g., "sys-libs")
//   - name: package name without category (e.g., "zlib")
//   - version: package version (e.g., "1.2.13-r1")
//   - slot: package slot string (e.g., "0" or "0/1.2")
//   - cfg: configuration sources (nil = no config, only IUSE defaults applied)
//
// Returns the set of enabled USE flags. A flag absent from the map is disabled.
func ResolveEffectiveUSE(iuseDefaults map[string]bool, category, name, version, slot string, cfg USEResolverConfig) map[string]bool {
	effectiveUSE := make(map[string]bool, len(iuseDefaults))

	// 1. Apply IUSE defaults: +flag means enabled by default
	for flag, defaultEnabled := range iuseDefaults {
		if defaultEnabled {
			effectiveUSE[flag] = true
		}
	}

	if cfg == nil {
		return effectiveUSE
	}

	// 2. Apply profile USE flags (make.defaults only)
	applyUSEFlagList(effectiveUSE, cfg.ProfileUSEFlags())

	// 3. Apply make.conf global USE flags
	applyUSEFlagList(effectiveUSE, cfg.GlobalUSEFlags())

	// 4. Apply USE_EXPAND variables
	for _, varName := range USEExpandVars {
		value := cfg.USEExpandValue(varName)
		if value == "" {
			continue
		}
		prefix := strings.ToLower(varName) + "_"
		for _, val := range strings.Fields(value) {
			flag := prefix + strings.ToLower(val)
			effectiveUSE[flag] = true
		}
	}

	// 5. Apply per-package USE from package.use
	applyUSEFlagList(effectiveUSE, cfg.PackageUSEFlags(category, name, version, slot))

	// 6. Apply use.force and use.mask (PMS: overrides everything, user cannot change)
	for _, flag := range cfg.ForcedUSE() {
		effectiveUSE[flag] = true
	}
	for _, flag := range cfg.MaskedUSE() {
		delete(effectiveUSE, flag)
	}

	return effectiveUSE
}

// IsUSEConditionalActive checks if a USE conditional is active given effective USE flags.
//
// Follows PMS (Package Manager Specification) USE conditional semantics:
//   - "" = no conditional, always active
//   - "flag" = active if flag is enabled
//   - "!flag" = active if flag is disabled
//
// This is the canonical check used by dependency filtering across all layers.
func IsUSEConditionalActive(useConditional string, effectiveUSE map[string]bool) bool {
	if useConditional == "" {
		return true
	}

	if strings.HasPrefix(useConditional, "!") {
		flag := useConditional[1:]
		return !effectiveUSE[flag]
	}

	return effectiveUSE[useConditional]
}

// applyUSEFlagList applies a list of USE flags to the effective USE map.
// Flags with "-" prefix disable (delete) the flag; flags without prefix enable it.
func applyUSEFlagList(effectiveUSE map[string]bool, flags []string) {
	for _, flag := range flags {
		if strings.HasPrefix(flag, "-") {
			delete(effectiveUSE, flag[1:])
		} else {
			effectiveUSE[flag] = true
		}
	}
}

// ParseIUSEDefaults parses an IUSE string into a defaults map.
//
// IUSE format per PMS:
//   - "+flag" = enabled by default
//   - "-flag" = disabled by default (explicit)
//   - "flag"  = disabled by default
//
// Returns:
//   - iuseDefaults: flag name -> true if default enabled (+flag)
//   - iuseFlags: flag name -> initial enabled state for Package.UseFlags
func ParseIUSEDefaults(iuseStr string) (iuseDefaults map[string]bool, iuseFlags map[string]bool) {
	fields := strings.Fields(iuseStr)
	iuseDefaults = make(map[string]bool, len(fields))
	iuseFlags = make(map[string]bool, len(fields))

	for _, flag := range fields {
		switch {
		case strings.HasPrefix(flag, "+"):
			cleanFlag := flag[1:]
			iuseDefaults[cleanFlag] = true
			iuseFlags[cleanFlag] = true
		case strings.HasPrefix(flag, "-"):
			cleanFlag := flag[1:]
			iuseDefaults[cleanFlag] = false
			iuseFlags[cleanFlag] = false
		default:
			iuseDefaults[flag] = false
			iuseFlags[flag] = false
		}
	}

	return iuseDefaults, iuseFlags
}

// SplitEnabledDisabled splits a map of effective USE flags into two sorted slices:
// one for enabled flags and one for disabled flags. Only flags present in iuseFlags
// are included (flags not declared in IUSE are excluded per Portage behavior).
//
// This utility is used by CLI display code and the solver to partition the
// resolved USE state.
func SplitEnabledDisabled(effectiveUSE map[string]bool, iuseFlags map[string]bool) (enabled, disabled []string) {
	for flag := range iuseFlags {
		if effectiveUSE[flag] {
			enabled = append(enabled, flag)
		} else {
			disabled = append(disabled, flag)
		}
	}
	return enabled, disabled
}
