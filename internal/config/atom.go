// Package config - atom pattern matching for package.use files.
//
// This file implements Portage-compatible atom pattern matching for
// package.use, package.mask, package.accept_keywords, etc.
//
// Pattern types and their specificity (higher = more specific):
//
//	=cpv          → 6  (exact version match)
//	~cpv          → 5  (any revision)
//	=cpv*         → 4  (version prefix)
//	cp:slot       → 3  (slot-specific)
//	>=/<=/>/< cpv → 2  (version range)
//	cp            → 1  (package only, no version)
//	cp:slot with wildcard → 0
//	cp with wildcard      → -1
package config

import (
	"regexp"
	"strings"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// AtomSpecificity represents the priority of an atom pattern.
// Higher values indicate more specific patterns that should take precedence.
type AtomSpecificity int

const (
	SpecificityWildcard      AtomSpecificity = -1 // */* or category/*
	SpecificityWildcardSlot  AtomSpecificity = 0  // */*:slot
	SpecificityCP            AtomSpecificity = 1  // category/package
	SpecificityRange         AtomSpecificity = 2  // >=, <=, <, >
	SpecificitySlot          AtomSpecificity = 3  // cp:slot
	SpecificityVersionPrefix AtomSpecificity = 4  // =cpv*
	SpecificityRevision      AtomSpecificity = 5  // ~cpv
	SpecificityExact         AtomSpecificity = 6  // =cpv
)

// PackageAtom represents a parsed package atom with optional version constraints.
type PackageAtom struct {
	// Operator is the version operator: =, >=, <=, >, <, ~, =* (empty for no version)
	Operator string

	// Category is the package category (e.g., "app-misc", "dev-*", "*")
	Category string

	// Name is the package name (e.g., "hello", "*")
	Name string

	// Version is the version constraint (e.g., "2.10", "2.0")
	Version string

	// Slot is the slot constraint (e.g., "0", "0/1.22")
	Slot string

	// Repository is the repository constraint (e.g., "gentoo", "local")
	Repository string

	// Raw is the original atom string
	Raw string
}

// ParseAtom parses a package atom string into its components.
// Supported formats:
//   - category/package
//   - =category/package-version
//   - >=category/package-version
//   - category/package:slot
//   - category/package::repository
//   - */* (wildcard)
//   - category/* (category wildcard)
func ParseAtom(atom string) *PackageAtom {
	result := &PackageAtom{Raw: atom}

	// Handle repository suffix (::repo)
	if idx := strings.Index(atom, "::"); idx != -1 {
		result.Repository = atom[idx+2:]
		atom = atom[:idx]
	}

	// Handle slot suffix (:slot)
	if idx := strings.Index(atom, ":"); idx != -1 {
		result.Slot = atom[idx+1:]
		atom = atom[:idx]
	}

	// Handle version operators
	for _, op := range []string{">=", "<=", ">", "<", "~", "=*", "="} {
		if strings.HasPrefix(atom, op) {
			result.Operator = op
			atom = strings.TrimPrefix(atom, op)
			break
		}
	}

	// Special handling for =* (version prefix)
	if result.Operator == "=*" {
		result.Operator = "=*"
	} else if result.Operator == "=" && strings.HasSuffix(atom, "*") {
		// Handle =cpv* format (trailing asterisk)
		result.Operator = "=*"
		atom = strings.TrimSuffix(atom, "*")
	}

	// Split category/name-version
	parts := strings.SplitN(atom, "/", 2)
	if len(parts) == 2 {
		result.Category = parts[0]
		nameVersion := parts[1]

		// Extract version if operator is present
		if result.Operator != "" {
			// Find last hyphen followed by a digit (version separator)
			lastHyphen := -1
			for i := len(nameVersion) - 1; i >= 0; i-- {
				if nameVersion[i] == '-' && i+1 < len(nameVersion) {
					next := nameVersion[i+1]
					if next >= '0' && next <= '9' {
						lastHyphen = i
						break
					}
				}
			}
			if lastHyphen > 0 {
				result.Name = nameVersion[:lastHyphen]
				result.Version = nameVersion[lastHyphen+1:]
			} else {
				result.Name = nameVersion
			}
		} else {
			result.Name = nameVersion
		}
	}

	return result
}

// GetSpecificity returns the specificity of the atom pattern.
func (a *PackageAtom) GetSpecificity() AtomSpecificity {
	hasWildcard := strings.Contains(a.Category, "*") || strings.Contains(a.Name, "*")

	if hasWildcard {
		if a.Slot != "" {
			return SpecificityWildcardSlot
		}
		return SpecificityWildcard
	}

	switch a.Operator {
	case "=":
		return SpecificityExact
	case "~":
		return SpecificityRevision
	case "=*":
		return SpecificityVersionPrefix
	case ">=", "<=", ">", "<":
		return SpecificityRange
	default:
		if a.Slot != "" {
			return SpecificitySlot
		}
		return SpecificityCP
	}
}

// Matches checks if a package (category/name with version and slot) matches this atom.
func (a *PackageAtom) Matches(category, name, version, slot string) bool {
	// Match category
	if !matchPattern(a.Category, category) {
		return false
	}

	// Match name
	if !matchPattern(a.Name, name) {
		return false
	}

	// Match slot if specified
	if a.Slot != "" && slot != "" {
		if !matchSlot(a.Slot, slot) {
			return false
		}
	}

	// Match version if operator is specified
	if a.Operator != "" && a.Version != "" {
		if !a.matchVersion(version) {
			return false
		}
	}

	return true
}

// matchVersion checks if the given version satisfies the atom's version constraint.
func (a *PackageAtom) matchVersion(pkgVersion string) bool {
	if pkgVersion == "" {
		return a.Operator == ""
	}

	cmp := compareVersions(pkgVersion, a.Version)

	switch a.Operator {
	case "=":
		return cmp == 0
	case ">=":
		return cmp >= 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case "<":
		return cmp < 0
	case "~":
		// Match any revision - compare without revision suffix
		baseVersion := stripRevision(pkgVersion)
		atomBaseVersion := stripRevision(a.Version)
		return compareVersions(baseVersion, atomBaseVersion) == 0
	case "=*":
		// Version prefix match
		return strings.HasPrefix(pkgVersion, a.Version)
	default:
		return true
	}
}

// matchPattern matches a string against a pattern with wildcards.
// Patterns:
//   - "*" matches anything
//   - "foo*" matches strings starting with "foo"
//   - "*foo" matches strings ending with "foo"
//   - "foo" matches exactly "foo"
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == value {
		return true
	}

	// Handle prefix wildcard: dev-*
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	// Handle suffix wildcard: *-libs
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(value, suffix)
	}

	return false
}

// matchSlot checks if a package slot matches the atom's slot constraint.
// Supports basic slot matching and subslot matching.
func matchSlot(atomSlot, pkgSlot string) bool {
	if atomSlot == "*" {
		return true
	}

	// Handle subslot: "0/1" vs "0/1.22"
	atomParts := strings.SplitN(atomSlot, "/", 2)
	pkgParts := strings.SplitN(pkgSlot, "/", 2)

	// Main slot must match
	if atomParts[0] != pkgParts[0] && atomParts[0] != "*" {
		return false
	}

	// If atom specifies subslot, it must match
	if len(atomParts) > 1 && len(pkgParts) > 1 {
		return atomParts[1] == pkgParts[1] || atomParts[1] == "*"
	}

	return true
}

// stripRevision removes the revision suffix (-rN) from a version string.
func stripRevision(version string) string {
	// Find last -r followed by digits
	for i := len(version) - 1; i >= 2; i-- {
		if version[i-1] == '-' && version[i] == 'r' {
			// Check if rest is digits
			rest := version[i+1:]
			if isDigits(rest) {
				return version[:i-1]
			}
		}
	}
	return version
}

// isDigits returns true if the string contains only digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// compareVersions compares two Gentoo version strings using PMS-compliant algorithm.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Delegates to internal/pkg.CompareVersions for full PMS compliance.
func compareVersions(a, b string) int {
	return pkg.CompareVersions(a, b)
}

// expandUSEExpand expands USE_EXPAND syntax in flag list.
// "CPU_FLAGS_X86: avx2 sse4_2" -> ["cpu_flags_x86_avx2", "cpu_flags_x86_sse4_2"]
func expandUSEExpand(flags []string) []string {
	var result []string
	var prefix string

	for _, flag := range flags {
		// Check for USE_EXPAND prefix (ends with ":")
		if strings.HasSuffix(flag, ":") {
			// Set prefix for subsequent flags
			prefix = strings.ToLower(strings.TrimSuffix(flag, ":")) + "_"
			continue
		}

		if prefix != "" {
			// Apply prefix to this flag
			if strings.HasPrefix(flag, "-") {
				// Negative flag: -avx2 -> -cpu_flags_x86_avx2
				result = append(result, "-"+prefix+flag[1:])
			} else {
				result = append(result, prefix+flag)
			}
		} else {
			result = append(result, flag)
		}
	}

	return result
}

// USEExpandRegex matches USE_EXPAND group names (e.g., "CPU_FLAGS_X86:", "PYTHON_TARGETS:")
var USEExpandRegex = regexp.MustCompile(`^[A-Z][A-Z0-9_]*:$`)

// IsUSEExpandPrefix returns true if the flag looks like a USE_EXPAND prefix.
func IsUSEExpandPrefix(flag string) bool {
	return USEExpandRegex.MatchString(flag)
}
