// Package profile implements Gentoo profile system support.
//
// Gentoo profiles define system-wide defaults for USE flags, package masks,
// and system packages. Profiles support inheritance through parent files.
//
// Profile Structure:
//
//	/etc/portage/make.profile/
//	├── make.defaults        # Default variables (USE, CFLAGS, etc.)
//	├── parent               # Profile inheritance
//	├── use.mask             # Masked USE flags
//	├── use.force            # Forced USE flags
//	├── packages             # System packages
//	├── package.mask         # Masked packages
//	└── package.use          # Per-package USE flags
//
// Example:
//
//	profile, err := profile.LoadProfile("/etc/portage/make.profile")
//	if err != nil {
//	    return err
//	}
//	useFlags := profile.GetUSEFlags()
//	systemPkgs := profile.GetSystemPackages()
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profile represents a Gentoo profile with its configuration.
type Profile struct {
	// Path is the filesystem path to the profile directory.
	// Example: /etc/portage/make.profile or /var/db/repos/gentoo/profiles/default/linux/amd64/23.0
	Path string

	// Name is the profile identifier.
	// Example: gentoo:default/linux/amd64/23.0
	Name string

	// Parents contains inherited profiles.
	// Profiles can inherit from multiple parents.
	Parents []*Profile

	// MakeDefaults contains variables from make.defaults file.
	// Keys: USE, CFLAGS, CXXFLAGS, ARCH, etc.
	MakeDefaults map[string]string

	// USEMask contains masked USE flags from use.mask.
	// These USE flags cannot be enabled.
	USEMask []string

	// USEForce contains forced USE flags from use.force.
	// These USE flags are always enabled.
	USEForce []string

	// Packages contains system packages from packages file.
	// Format: *category/package (lines starting with *)
	Packages []string

	// PackageMask contains masked packages from package.mask.
	PackageMask []string

	// PackageUnmask contains unmasked packages from package.unmask.
	PackageUnmask []string

	// PackageUse contains per-package USE flags from package.use.
	// Key: package atom, Value: USE flags
	PackageUse map[string][]string

	// Keywords contains package keywords from package.keywords.
	// Key: package atom, Value: keyword (e.g., "~amd64")
	Keywords map[string]string

	// EAPI is the profile's EAPI version.
	EAPI string

	// resolved indicates whether parent profiles have been resolved.
	resolved bool
}

// LoadProfile loads a Gentoo profile from the specified path.
//
// The path should point to a profile directory (e.g., /etc/portage/make.profile).
// This function does NOT automatically resolve parent profiles. Call Resolve()
// after loading to resolve the full profile inheritance chain.
//
// Example:
//
//	profile, err := LoadProfile("/etc/portage/make.profile")
//	if err != nil {
//	    return err
//	}
//	if err := profile.Resolve(); err != nil {
//	    return err
//	}
func LoadProfile(path string) (*Profile, error) {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("profile path does not exist: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("profile path is not a directory: %s", path)
	}

	// Create profile
	profile := &Profile{
		Path:          path,
		Name:          extractProfileName(path),
		MakeDefaults:  make(map[string]string),
		PackageUse:    make(map[string][]string),
		Keywords:      make(map[string]string),
		USEMask:       make([]string, 0),
		USEForce:      make([]string, 0),
		Packages:      make([]string, 0),
		PackageMask:   make([]string, 0),
		PackageUnmask: make([]string, 0),
	}

	// Load EAPI
	if err := profile.loadEAPI(); err != nil {
		// EAPI file is optional, default to "0"
		profile.EAPI = "0"
	}

	// Load profile files (all optional)
	_ = profile.loadMakeDefaults()  // Optional: make.defaults
	_ = profile.loadUSEMask()       // Optional: use.mask
	_ = profile.loadUSEForce()      // Optional: use.force
	_ = profile.loadPackages()      // Optional: packages
	_ = profile.loadPackageMask()   // Optional: package.mask
	_ = profile.loadPackageUnmask() // Optional: package.unmask
	_ = profile.loadPackageUse()    // Optional: package.use
	_ = profile.loadKeywords()      // Optional: package.keywords

	return profile, nil
}

// Resolve resolves the profile's parent inheritance chain.
//
// This function:
// 1. Loads all parent profiles
// 2. Recursively resolves parent profiles
// 3. Merges configuration from parents (parents are applied first)
//
// After calling Resolve(), GetUSEFlags() and GetSystemPackages() will
// return the fully merged configuration.
func (p *Profile) Resolve() error {
	if p.resolved {
		return nil
	}

	// Load parent file
	parents, err := p.loadParents()
	if err != nil {
		// No parents is OK
		p.resolved = true
		return nil
	}

	// Resolve each parent
	for _, parent := range parents {
		if err := parent.Resolve(); err != nil {
			return fmt.Errorf("failed to resolve parent profile %s: %w", parent.Path, err)
		}
	}

	p.Parents = parents
	p.resolved = true
	return nil
}

// GetUSEFlags returns all USE flags from this profile and its parents.
//
// The returned flags include:
// - USE flags from make.defaults (this profile and parents)
// - Forced USE flags (use.force)
// - Negated masked USE flags (-flag from use.mask)
//
// Parent USE flags are applied first, then this profile's flags.
func (p *Profile) GetUSEFlags() []string {
	flags := make([]string, 0)

	// Collect from parents first (bottom-up)
	for _, parent := range p.Parents {
		flags = append(flags, parent.GetUSEFlags()...)
	}

	// Add USE flags from make.defaults
	if use, exists := p.MakeDefaults["USE"]; exists {
		flags = append(flags, parseUSEFlags(use)...)
	}

	// Add forced USE flags
	flags = append(flags, p.USEForce...)

	// Add negated masked USE flags
	for _, masked := range p.USEMask {
		flags = append(flags, "-"+masked)
	}

	return deduplicateUSEFlags(flags)
}

// GetSystemPackages returns all system packages from this profile and its parents.
//
// System packages are defined in the "packages" file with lines starting with "*".
// Example:
//
//	*sys-apps/baselayout
//	*virtual/libc
//
// Parent packages are included first.
func (p *Profile) GetSystemPackages() []string {
	packages := make([]string, 0)

	// Collect from parents first
	for _, parent := range p.Parents {
		packages = append(packages, parent.GetSystemPackages()...)
	}

	// Add this profile's packages
	packages = append(packages, p.Packages...)

	return deduplicateStrings(packages)
}

// GetMakeDefault returns a make.defaults variable value.
//
// If the variable is not defined in this profile, parent profiles are checked.
// Returns empty string if not found.
func (p *Profile) GetMakeDefault(key string) string {
	// Check this profile first
	if value, exists := p.MakeDefaults[key]; exists {
		return value
	}

	// Check parents (last parent has priority)
	for i := len(p.Parents) - 1; i >= 0; i-- {
		if value := p.Parents[i].GetMakeDefault(key); value != "" {
			return value
		}
	}

	return ""
}

// GetPackageUSE returns USE flags for a specific package.
//
// Returns USE flags from package.use for the given package atom.
// Parent package.use entries are included.
func (p *Profile) GetPackageUSE(atom string) []string {
	flags := make([]string, 0)

	// Collect from parents first
	for _, parent := range p.Parents {
		flags = append(flags, parent.GetPackageUSE(atom)...)
	}

	// Add this profile's package USE flags
	if useFlags, exists := p.PackageUse[atom]; exists {
		flags = append(flags, useFlags...)
	}

	return flags
}

// IsPackageMasked checks if a package is masked in this profile.
//
// Checks both package.mask and package.unmask files.
// Parent profiles are checked as well.
func (p *Profile) IsPackageMasked(atom string) bool {
	// Check unmask first (higher priority)
	for _, unmask := range p.PackageUnmask {
		if matchesAtom(atom, unmask) {
			return false
		}
	}

	// Check mask
	for _, mask := range p.PackageMask {
		if matchesAtom(atom, mask) {
			return true
		}
	}

	// Check parents
	for _, parent := range p.Parents {
		if parent.IsPackageMasked(atom) {
			return true
		}
	}

	return false
}

// extractProfileName extracts a profile name from its path.
//
// Example:
//
//	/var/db/repos/gentoo/profiles/default/linux/amd64/23.0
//	-> default/linux/amd64/23.0
func extractProfileName(path string) string {
	// Try to extract relative to "profiles" directory
	if idx := strings.Index(path, "profiles/"); idx >= 0 {
		return path[idx+len("profiles/"):]
	}

	// If not found, use basename
	return filepath.Base(path)
}

// parseUSEFlags splits a USE flag string into individual flags.
//
// Example: "ssl unicode -gtk" -> ["ssl", "unicode", "-gtk"]
func parseUSEFlags(use string) []string {
	fields := strings.Fields(use)
	result := make([]string, 0, len(fields))

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}

	return result
}

// deduplicateUSEFlags removes duplicate USE flags, keeping the last occurrence.
//
// Later flags override earlier ones (Portage behavior).
func deduplicateUSEFlags(flags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(flags))

	// Process in reverse to keep last occurrence
	for i := len(flags) - 1; i >= 0; i-- {
		flag := flags[i]
		baseName := strings.TrimPrefix(flag, "-")

		if !seen[baseName] {
			seen[baseName] = true
			result = append([]string{flag}, result...) // Prepend
		}
	}

	return result
}

// deduplicateStrings removes duplicate strings while preserving order.
func deduplicateStrings(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// matchesAtom checks if a package atom matches a pattern.
//
// This is a simplified matcher. Full implementation would use pkg.Constraint.
// For now, it does simple string matching.
func matchesAtom(atom, pattern string) bool {
	// Simple exact match for now
	// TODO: Implement proper atom matching with version constraints
	return atom == pattern || strings.HasPrefix(atom, pattern)
}
