package pkg

import (
	"strconv"
	"strings"
)

// ConstraintType defines the type of package constraint
type ConstraintType int

const (
	ConstraintTypeVersion ConstraintType = iota
	ConstraintTypeSlot
	ConstraintTypeUseFlag
)

// Status represents the status of dependency resolution
type Status int

const (
	StatusIndet Status = iota // Indeterminate
	StatusSat                 // Satisfiable - solution found
	StatusUnsat               // Unsatisfiable - dependency conflict
)

// VersionOperator defines version comparison operators
type VersionOperator int

const (
	OpEqual VersionOperator = iota
	OpGreater
	OpGreaterEqual
	OpLess
	OpLessEqual
)

// VersionConstraint represents a version constraint (Value Object - immutable)
// Once created, its state cannot be modified
type VersionConstraint struct {
	operator VersionOperator // lowercase = unexported = immutable from outside
	version  string
}

// Operator returns the version operator (read-only access)
func (vc *VersionConstraint) Operator() VersionOperator {
	return vc.operator
}

// Version returns the version string (read-only access)
func (vc *VersionConstraint) Version() string {
	return vc.version
}

// Constraint represents a general package constraint
type Constraint struct {
	Type      ConstraintType
	Name      string
	Version   *VersionConstraint // For version constraints
	Slot      string             // For slot constraints
	Flag      string             // For USE flags
	Required  bool               // Mandatory requirement
	Condition string             // USE flag condition
	OrGroupID int                // OR-group ID (0 = required, >0 = alternative)
}

func (c Constraint) String() string {
	if c.Version == nil {
		return c.Name
	}
	return c.Name + " " + c.Version.String()
}

// NewVersionConstraint creates a new immutable version constraint
func NewVersionConstraint(operator VersionOperator, version string) *VersionConstraint {
	return &VersionConstraint{
		operator: operator,
		version:  version,
	}
}

// NewExactVersionConstraint creates an exact version constraint
func NewExactVersionConstraint(version string) *VersionConstraint {
	return NewVersionConstraint(OpEqual, version)
}

// NewMinVersionConstraint creates a minimum version constraint
func NewMinVersionConstraint(version string) *VersionConstraint {
	return NewVersionConstraint(OpGreaterEqual, version)
}

// NewMaxVersionConstraint creates a maximum version constraint
func NewMaxVersionConstraint(version string) *VersionConstraint {
	return NewVersionConstraint(OpLessEqual, version)
}

// String returns the string representation of the version constraint
func (vc *VersionConstraint) String() string {
	if vc == nil {
		return "any"
	}

	switch vc.operator {
	case OpEqual:
		return vc.version
	case OpGreater:
		return ">" + vc.version
	case OpGreaterEqual:
		return ">=" + vc.version
	case OpLess:
		return "<" + vc.version
	case OpLessEqual:
		return "<=" + vc.version
	default:
		return "unknown"
	}
}

// Satisfies checks if the given version satisfies this constraint
func (vc *VersionConstraint) Satisfies(version string) bool {
	if vc == nil {
		return true
	}

	switch vc.operator {
	case OpEqual:
		return version == vc.version
	case OpGreater:
		return CompareVersions(version, vc.version) > 0
	case OpGreaterEqual:
		return CompareVersions(version, vc.version) >= 0
	case OpLess:
		return CompareVersions(version, vc.version) < 0
	case OpLessEqual:
		return CompareVersions(version, vc.version) <= 0
	default:
		return true
	}
}

// CompareVersions compares two Gentoo-format version strings
// Returns: <0 if v1<v2, 0 if v1==v2, >0 if v1>v2
func CompareVersions(v1, v2 string) int {
	// Split versions into components: 1.2.3_alpha4-r5 -> [1, 2, 3, "alpha", 4, 5]
	// Uses precompiled regex from version.go for performance
	splitVersion := func(v string) []interface{} {
		parts := versionComponentsRe.FindAllString(v, -1)

		var result []interface{}
		for _, part := range parts {
			if num, err := strconv.Atoi(part); err == nil {
				result = append(result, num)
			} else {
				result = append(result, part)
			}
		}
		return result
	}

	parts1 := splitVersion(v1)
	parts2 := splitVersion(v2)

	// Compare components
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		switch a := parts1[i].(type) {
		case int:
			if b, ok := parts2[i].(int); ok {
				if a != b {
					return a - b
				}
			} else {
				// Numbers are always greater than strings
				return 1
			}
		case string:
			if b, ok := parts2[i].(string); ok {
				if cmp := strings.Compare(a, b); cmp != 0 {
					return cmp
				}
			} else {
				// Strings are always less than numbers
				return -1
			}
		}
	}

	// If all components are equal, the longer version is considered greater
	return len(parts1) - len(parts2)
}

// ParseVersionConstraint parses a string representation of a version constraint
func ParseVersionConstraint(s string) (*VersionConstraint, error) {
	if s == "" {
		return nil, nil
	}

	// Check operators in order from longest to shortest to avoid prefix conflicts
	// (e.g., ">=" must be checked before ">")
	if strings.HasPrefix(s, ">=") {
		version := strings.TrimSpace(strings.TrimPrefix(s, ">="))
		return &VersionConstraint{
			operator: OpGreaterEqual,
			version:  version,
		}, nil
	}
	if strings.HasPrefix(s, "<=") {
		version := strings.TrimSpace(strings.TrimPrefix(s, "<="))
		return &VersionConstraint{
			operator: OpLessEqual,
			version:  version,
		}, nil
	}
	if strings.HasPrefix(s, ">") {
		version := strings.TrimSpace(strings.TrimPrefix(s, ">"))
		return &VersionConstraint{
			operator: OpGreater,
			version:  version,
		}, nil
	}
	if strings.HasPrefix(s, "<") {
		version := strings.TrimSpace(strings.TrimPrefix(s, "<"))
		return &VersionConstraint{
			operator: OpLess,
			version:  version,
		}, nil
	}
	if strings.HasPrefix(s, "=") {
		version := strings.TrimSpace(strings.TrimPrefix(s, "="))
		return &VersionConstraint{
			operator: OpEqual,
			version:  version,
		}, nil
	}

	// Default to exact version match
	return &VersionConstraint{
		operator: OpEqual,
		version:  s,
	}, nil
}

// NewSimpleConstraint creates a constraint without version specification
func NewSimpleConstraint(name string) Constraint {
	return Constraint{
		Type: ConstraintTypeVersion,
		Name: name,
	}
}
