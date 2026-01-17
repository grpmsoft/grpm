package pkg

import (
	"strings"
)

// Slot represents a package slot for parallel installation support
// It is a Value Object (immutable, compared by value)
type Slot struct {
	Name           string
	Subslot        string
	SubslotRebuild bool // true for := operator (rebuild on subslot change)
}

// NewSlot creates a new Slot Value Object
func NewSlot(name, subslot string) Slot {
	return Slot{
		Name:    name,
		Subslot: subslot,
	}
}

func (s Slot) String() string {
	if s.Subslot != "" {
		return s.Name + "/" + s.Subslot
	}
	return s.Name
}

// Equals checks if two slots are equal (Value Object equality)
func (s Slot) Equals(other Slot) bool {
	return s.Name == other.Name && s.Subslot == other.Subslot
}

// IsCompatibleWith checks if this slot is compatible with another
// Slots are compatible if they have different slot names or same slot/subslot
func (s Slot) IsCompatibleWith(other Slot) bool {
	// Different slot names - always compatible
	if s.Name != other.Name {
		return true
	}

	// Same slot name - check subslots
	// Compatible if subslots match or one is empty
	if s.Subslot == "" || other.Subslot == "" {
		return true
	}

	return s.Subslot == other.Subslot
}

// ParseSlot parses a string representation of a slot
// Supports: "0", "0/1", "=" (any slot with subslot rebuild)
func ParseSlot(slot string) Slot {
	// Check for subslot rebuild operator ":="
	if slot == "=" {
		return Slot{
			Name:           "",
			Subslot:        "",
			SubslotRebuild: true,
		}
	}

	parts := strings.Split(slot, "/")
	if len(parts) > 1 {
		return Slot{
			Name:           parts[0],
			Subslot:        parts[1],
			SubslotRebuild: false,
		}
	}
	return Slot{
		Name:           slot,
		Subslot:        "",
		SubslotRebuild: false,
	}
}

// Package represents a Gentoo package with its metadata and dependencies
// It is an Aggregate Root in DDD terms, controlling access to its dependencies
type Package struct {
	Name     string
	Version  string
	Slot     Slot
	UseFlags map[string]bool
	Keywords []string // KEYWORDS from ebuild (e.g., ["amd64", "~x86", "-arm"])
	Deps     []Constraint
	Provides []Constraint // Virtual packages provided by this package
}

// NewPackage creates a new package instance with validation
func NewPackage(name, version, slotStr string) *Package {
	// TODO: Add validation - name must match category/package format
	// TODO: Version should use Version Value Object
	return &Package{
		Name:     name,
		Version:  version,
		Slot:     ParseSlot(slotStr),
		UseFlags: make(map[string]bool),
		Keywords: make([]string, 0),
		Deps:     make([]Constraint, 0),
		Provides: make([]Constraint, 0),
	}
}

// HasKeyword checks if the package has a specific keyword (stable or testing).
// Supports: "amd64" (stable), "~amd64" (testing), "-amd64" (disabled).
func (p *Package) HasKeyword(keyword string) bool {
	for _, kw := range p.Keywords {
		if kw == keyword {
			return true
		}
	}
	return false
}

// IsKeyworded returns true if the package has any KEYWORDS defined.
// Packages without KEYWORDS are "unkeyworded" and typically masked.
func (p *Package) IsKeyworded() bool {
	return len(p.Keywords) > 0
}

// HasStableKeyword checks if the package has a stable keyword for the given arch.
// Example: HasStableKeyword("amd64") returns true if KEYWORDS contains "amd64".
func (p *Package) HasStableKeyword(arch string) bool {
	return p.HasKeyword(arch)
}

// HasTestingKeyword checks if the package has a testing keyword for the given arch.
// Example: HasTestingKeyword("amd64") returns true if KEYWORDS contains "~amd64".
func (p *Package) HasTestingKeyword(arch string) bool {
	return p.HasKeyword("~" + arch)
}

// IsKeywordAccepted checks if the package is accepted given ACCEPT_KEYWORDS.
// acceptKeywords: list like ["amd64", "~amd64"] from make.conf.
func (p *Package) IsKeywordAccepted(acceptKeywords []string) bool {
	// If package has no keywords, it's unkeyworded
	// Accept only if "**" is in acceptKeywords
	if !p.IsKeyworded() {
		for _, ak := range acceptKeywords {
			if ak == "**" {
				return true
			}
		}
		return false
	}

	// Check if any of the package's keywords match accepted keywords
	for _, kw := range p.Keywords {
		// Skip negative keywords
		if strings.HasPrefix(kw, "-") {
			continue
		}

		// Check for wildcard matches
		for _, ak := range acceptKeywords {
			if ak == "*" || ak == "**" {
				return true
			}
			if ak == "~*" && strings.HasPrefix(kw, "~") {
				return true
			}
			if kw == ak {
				return true
			}
			// Testing keyword matches stable acceptance
			// e.g., ~amd64 in KEYWORDS matches amd64 in ACCEPT_KEYWORDS
			if strings.HasPrefix(kw, "~") && kw[1:] == ak {
				// Need ~amd64 in ACCEPT_KEYWORDS to accept ~amd64 in KEYWORDS
				// But NOT amd64 - that only accepts stable
				continue
			}
		}
	}

	return false
}

// ID returns the unique identifier for this package (Aggregate Root identity)
func (p *Package) ID() string {
	return p.Name + "-" + p.Version
}

// FullName returns the canonical package name with version
func (p *Package) FullName() string {
	return p.Name + "-" + p.Version
}

// AddDependency adds a dependency constraint to this package
func (p *Package) AddDependency(constraint Constraint) {
	p.Deps = append(p.Deps, constraint)
}

// ConflictsWith checks if this package conflicts with another due to slot incompatibility
func (p *Package) ConflictsWith(other *Package) bool {
	// Packages with different names can conflict due to slots
	if p.Name == other.Name {
		return false // Different versions of the same package are handled separately
	}

	// Conflict occurs if slots match but subslots differ
	return p.Slot.Name == other.Slot.Name && p.Slot.Subslot != other.Slot.Subslot
}

// SatisfiesConstraint checks if this package satisfies a given constraint
func (p *Package) SatisfiesConstraint(c Constraint) bool {
	// Check package name matches
	if c.Name != p.Name {
		return false
	}

	// Check version constraint if present
	if c.Version != nil {
		return c.Version.Satisfies(p.Version)
	}

	// Check slot constraint if present
	if c.Type == ConstraintTypeSlot && c.Slot != "" {
		return p.Slot.Name == c.Slot
	}

	// No specific constraints - satisfied
	return true
}

// IsCompatibleWith checks if this package can be installed alongside another
func (p *Package) IsCompatibleWith(other *Package) bool {
	// Same package different version - check slots
	if p.Name == other.Name {
		return p.Slot.IsCompatibleWith(other.Slot)
	}

	// Different packages - check if they conflict
	return !p.ConflictsWith(other)
}

// HasDependency checks if this package depends on a specific package
func (p *Package) HasDependency(packageName string) bool {
	for _, dep := range p.Deps {
		if dep.Name == packageName {
			return true
		}
	}
	return false
}

// GetDependenciesByType returns dependencies filtered by type
func (p *Package) GetDependenciesByType(constraintType ConstraintType) []Constraint {
	var result []Constraint
	for _, dep := range p.Deps {
		if dep.Type == constraintType {
			result = append(result, dep)
		}
	}
	return result
}
