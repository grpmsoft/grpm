package solver

import (
	"log"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// UseFlagSolver resolves USE flag dependencies
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

// IsUseFlagEnabled checks if a USE flag is enabled for a package
func (ufs *UseFlagSolver) IsUseFlagEnabled(packageName, flag string) bool {
	// Check package-specific USE flags first
	if pkgFlags, exists := ufs.packageUseFlags[packageName]; exists {
		for _, f := range pkgFlags {
			if strings.HasPrefix(f, "-") && strings.TrimPrefix(f, "-") == flag {
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

// EvaluateUseCondition evaluates a USE flag condition
// Examples: "ssl", "!ssl", "ssl,mysql", "ssl,-bindist"
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

// FilterDependenciesByUseFlags filters dependencies based on USE flag conditions
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

// ResolveUseFlagsForPackage resolves final USE flags for a package
// Combines: IUSE (available flags) + profile defaults + global USE + package.use
func (ufs *UseFlagSolver) ResolveUseFlagsForPackage(p *pkg.Package) (map[string]bool, error) {
	resolved := make(map[string]bool)

	// Start with package's available USE flags (from IUSE)
	for flag := range p.UseFlags {
		// Check if enabled globally
		resolved[flag] = ufs.IsUseFlagEnabled(p.Name, flag)
	}

	// Apply package-specific overrides
	if pkgFlags, exists := ufs.packageUseFlags[p.Name]; exists {
		for _, flag := range pkgFlags {
			if strings.HasPrefix(flag, "-") {
				flagName := strings.TrimPrefix(flag, "-")
				resolved[flagName] = false
			} else {
				resolved[flag] = true
			}
		}
	}

	return resolved, nil
}

// ValidateUseFlagCombination checks if USE flag combination is valid
// Checks for conflicts like "ssl -ssl" or required dependencies
func (ufs *UseFlagSolver) ValidateUseFlagCombination(p *pkg.Package, flags map[string]bool) error {
	// Check for contradictions
	for _, enabled := range flags {
		if !enabled {
			continue
		}

		// Check if this flag has required dependencies
		// TODO: Implement REQUIRED_USE parsing
	}

	return nil
}

// GetEnabledUseFlags returns list of enabled USE flags for display
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

// String returns human-readable representation of USE flag configuration
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
