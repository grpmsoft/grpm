package repo

import "github.com/grpmsoft/grpm/internal/pkg"

// Specification represents a query specification for filtering packages
// This implements the Specification pattern from DDD
type Specification interface {
	IsSatisfiedBy(p *pkg.Package) bool
}

// AndSpecification combines multiple specifications with AND logic
type AndSpecification struct {
	specs []Specification
}

// NewAndSpecification creates an AND specification
func NewAndSpecification(specs ...Specification) Specification {
	return &AndSpecification{specs: specs}
}

func (s *AndSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	for _, spec := range s.specs {
		if !spec.IsSatisfiedBy(p) {
			return false
		}
	}
	return true
}

// OrSpecification combines multiple specifications with OR logic
type OrSpecification struct {
	specs []Specification
}

// NewOrSpecification creates an OR specification
func NewOrSpecification(specs ...Specification) Specification {
	return &OrSpecification{specs: specs}
}

func (s *OrSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	for _, spec := range s.specs {
		if spec.IsSatisfiedBy(p) {
			return true
		}
	}
	return false
}

// NotSpecification negates a specification
type NotSpecification struct {
	spec Specification
}

// NewNotSpecification creates a NOT specification
func NewNotSpecification(spec Specification) Specification {
	return &NotSpecification{spec: spec}
}

func (s *NotSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	return !s.spec.IsSatisfiedBy(p)
}

// NameSpecification filters packages by name
type NameSpecification struct {
	name string
}

// NewNameSpecification creates a specification for exact package name match
func NewNameSpecification(name string) Specification {
	return &NameSpecification{name: name}
}

func (s *NameSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	return p.Name == s.name
}

// VersionSpecification filters packages by version constraint
type VersionSpecification struct {
	constraint *pkg.VersionConstraint
}

// NewVersionSpecification creates a specification for version constraint
func NewVersionSpecification(constraint *pkg.VersionConstraint) Specification {
	return &VersionSpecification{constraint: constraint}
}

func (s *VersionSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	if s.constraint == nil {
		return true
	}
	return s.constraint.Satisfies(p.Version)
}

// SlotSpecification filters packages by slot
type SlotSpecification struct {
	slotName string
}

// NewSlotSpecification creates a specification for slot filtering
func NewSlotSpecification(slotName string) Specification {
	return &SlotSpecification{slotName: slotName}
}

func (s *SlotSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	return p.Slot.Name == s.slotName
}

// CategorySpecification filters packages by category (e.g., "sys-libs")
type CategorySpecification struct {
	category string
}

// NewCategorySpecification creates a specification for category filtering
func NewCategorySpecification(category string) Specification {
	return &CategorySpecification{category: category}
}

func (s *CategorySpecification) IsSatisfiedBy(p *pkg.Package) bool {
	// Extract category from package name (category/package format)
	for i, char := range p.Name {
		if char == '/' {
			return p.Name[:i] == s.category
		}
	}
	return false
}

// NamePatternSpecification filters packages by name pattern (substring match)
type NamePatternSpecification struct {
	pattern string
}

// NewNamePatternSpec creates a specification for pattern-based name matching
// Supports substring matching for search functionality
func NewNamePatternSpec(pattern string) Specification {
	return &NamePatternSpecification{pattern: pattern}
}

func (s *NamePatternSpecification) IsSatisfiedBy(p *pkg.Package) bool {
	// Simple substring match for now
	// TODO: Support glob patterns or regex
	return contains(p.Name, s.pattern)
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive substring match
	s = toLower(s)
	substr = toLower(substr)

	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
