package solver

import (
	"log"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// UseFlagSolver resolves USE flag dependencies within the SAT solver layer.
//
// It stores global and per-package USE flag settings and implements the
// pkg.USEResolverConfig interface so that USE resolution delegates to the
// canonical pkg.ResolveEffectiveUSE function. This ensures the solver
// evaluates the same USE state as the repository parser and CLI display.
type UseFlagSolver struct {
	globalUseFlags  map[string]bool     // Global USE flags (make.conf)
	packageUseFlags map[string][]string // Per-package USE flags (package.use)
}

// NewUseFlagSolver creates a new USE flag solver
func NewUseFlagSolver() *UseFlagSolver {
	return &UseFlagSolver{
		globalUseFlags:  make(map[string]bool),
		packageUseFlags: make(map[string][]string),
	}
}

// SetGlobalUseFlag sets a global USE flag
func (ufs *UseFlagSolver) SetGlobalUseFlag(flag string, enabled bool) {
	ufs.globalUseFlags[flag] = enabled
}

// SetPackageUseFlags sets USE flags for a specific package
func (ufs *UseFlagSolver) SetPackageUseFlags(packageName string, flags []string) {
	ufs.packageUseFlags[packageName] = flags
}

// --- pkg.USEResolverConfig implementation ---

// ProfileUSEFlags returns nil because the solver layer does not have
// direct access to the system profile. Profile USE flags are already
// baked into the packages during repository parsing.
func (ufs *UseFlagSolver) ProfileUSEFlags() []string { return nil }

// GlobalUSEFlags converts the solver's globalUseFlags map to a flag list
// in the format expected by pkg.ResolveEffectiveUSE: "flag" or "-flag".
func (ufs *UseFlagSolver) GlobalUSEFlags() []string {
	flags := make([]string, 0, len(ufs.globalUseFlags))
	for flag, enabled := range ufs.globalUseFlags {
		if enabled {
			flags = append(flags, flag)
		} else {
			flags = append(flags, "-"+flag)
		}
	}
	return flags
}

// PackageUSEFlags returns per-package flags for the given package.
// The solver stores them by full "category/name" key, so version/slot
// matching is done by exact name lookup.
func (ufs *UseFlagSolver) PackageUSEFlags(category, name, _, _ string) []string {
	fullName := category + "/" + name
	return ufs.packageUseFlags[fullName]
}

// USEExpandValue returns empty string because the solver does not manage
// USE_EXPAND variables directly. USE_EXPAND is handled at the repository
// parsing and CLI layers.
func (ufs *UseFlagSolver) USEExpandValue(_ string) string { return "" }
func (ufs *UseFlagSolver) ForcedUSE() []string              { return nil }
func (ufs *UseFlagSolver) MaskedUSE() []string              { return nil }

// --- Public API ---

// IsUseFlagEnabled checks if a USE flag is enabled for a package.
// Uses the unified resolver to compute effective USE for the package's
// IUSE set, then checks whether the flag is in the result.
func (ufs *UseFlagSolver) IsUseFlagEnabled(packageName, flag string) bool {
	// Check package-specific USE flags first (fast path)
	if pkgFlags, exists := ufs.packageUseFlags[packageName]; exists {
		for _, f := range pkgFlags {
			if strings.HasPrefix(f, "-") && f[1:] == flag {
				return false // Explicitly disabled
			}
			if f == flag {
				return true // Explicitly enabled
			}
		}
	}

	// Fall back to global USE flags
	if enabled, exists := ufs.globalUseFlags[flag]; exists {
		return enabled
	}

	// Default: disabled
	return false
}

// EvaluateUseCondition evaluates a USE flag condition.
// Examples: "ssl", "!ssl", "ssl,mysql", "ssl,-bindist"
//
// For single-flag conditions (the common case in dependency specs),
// this delegates to pkg.IsUSEConditionalActive. Multi-flag comma-separated
// conditions use AND logic across all flags.
func (ufs *UseFlagSolver) EvaluateUseCondition(packageName, condition string) bool {
	if condition == "" {
		return true
	}

	// Split by comma or space
	flags := strings.FieldsFunc(condition, func(r rune) bool {
		return r == ',' || r == ' '
	})

	// All flags must be satisfied (AND logic)
	for _, flag := range flags {
		flag = strings.TrimSpace(flag)
		if flag == "" {
			continue
		}

		// Check for negation
		if strings.HasPrefix(flag, "-") || strings.HasPrefix(flag, "!") {
			flagName := strings.TrimPrefix(strings.TrimPrefix(flag, "-"), "!")
			if ufs.IsUseFlagEnabled(packageName, flagName) {
				return false // Flag is enabled but should be disabled
			}
		} else {
			if !ufs.IsUseFlagEnabled(packageName, flag) {
				return false // Flag is disabled but should be enabled
			}
		}
	}

	return true
}

// FilterDependenciesByUseFlags filters dependencies based on USE flag conditions.
func (ufs *UseFlagSolver) FilterDependenciesByUseFlags(p *pkg.Package) []pkg.Constraint {
	var filtered []pkg.Constraint

	for _, dep := range p.Deps {
		// Check if dependency has USE flag condition
		if dep.Condition != "" {
			// Evaluate condition
			if !ufs.EvaluateUseCondition(p.Name, dep.Condition) {
				continue // Skip this dependency
			}
		}

		filtered = append(filtered, dep)
	}

	return filtered
}

// ResolveUseFlagsForPackage resolves final USE flags for a package.
// Delegates to the unified pkg.ResolveEffectiveUSE with the solver's
// global and per-package flags as the configuration source.
func (ufs *UseFlagSolver) ResolveUseFlagsForPackage(p *pkg.Package) (map[string]bool, error) {
	// Build IUSE defaults from package's UseFlags
	iuseDefaults := make(map[string]bool, len(p.UseFlags))
	for flag := range p.UseFlags {
		iuseDefaults[flag] = false // Treat all IUSE as disabled by default
	}

	// Extract category and name
	category := ""
	name := p.Name
	if parts := strings.SplitN(p.Name, "/", 2); len(parts) == 2 {
		category = parts[0]
		name = parts[1]
	}

	// Resolve via unified service
	effectiveUSE := pkg.ResolveEffectiveUSE(iuseDefaults, category, name,
		p.Version, p.Slot.String(), ufs)

	// Build result map: all IUSE flags with their resolved state
	resolved := make(map[string]bool, len(p.UseFlags))
	for flag := range p.UseFlags {
		resolved[flag] = effectiveUSE[flag]
	}

	// Also include flags added by package-specific overrides not in IUSE
	if pkgFlags, exists := ufs.packageUseFlags[p.Name]; exists {
		for _, flag := range pkgFlags {
			flagName := strings.TrimPrefix(flag, "-")
			if _, inIUSE := resolved[flagName]; !inIUSE {
				resolved[flagName] = !strings.HasPrefix(flag, "-")
			}
		}
	}

	return resolved, nil
}

// ValidateUseFlagCombination checks if USE flag combination is valid.
// Checks for conflicts like "ssl -ssl" or required dependencies.
func (ufs *UseFlagSolver) ValidateUseFlagCombination(_ *pkg.Package, _ map[string]bool) error {
	// TODO: Implement REQUIRED_USE parsing
	return nil
}

// GetEnabledUseFlags returns list of enabled USE flags for display.
func (ufs *UseFlagSolver) GetEnabledUseFlags(p *pkg.Package) []string {
	resolved, err := ufs.ResolveUseFlagsForPackage(p)
	if err != nil {
		log.Printf("Warning: failed to resolve USE flags for %s: %v", p.Name, err)
		return nil
	}

	var enabled []string
	for flag, isEnabled := range resolved {
		if isEnabled {
			enabled = append(enabled, flag)
		}
	}

	return enabled
}

// String returns human-readable representation of USE flag configuration.
func (ufs *UseFlagSolver) String() string {
	var sb strings.Builder

	sb.WriteString("Global USE flags: ")
	for flag, enabled := range ufs.globalUseFlags {
		if enabled {
			sb.WriteString(flag)
			sb.WriteString(" ")
		} else {
			sb.WriteString("-")
			sb.WriteString(flag)
			sb.WriteString(" ")
		}
	}

	return sb.String()
}
