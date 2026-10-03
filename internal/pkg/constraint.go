package pkg

import (
	"strings"
)

// ConstraintType defines the type of package constraint
type ConstraintType int

const (
	ConstraintTypeVersion ConstraintType = iota
	ConstraintTypeSlot
	ConstraintTypeUseFlag
)

// DepType defines the type of dependency (RDEPEND, DEPEND, BDEPEND, etc.)
type DepType int

const (
	// DepTypeRuntime is RDEPEND - runtime dependencies.
	// Required for the package to run at runtime.
	DepTypeRuntime DepType = iota

	// DepTypeBuild is DEPEND - build dependencies.
	// Required for building when target root differs from build root.
	DepTypeBuild

	// DepTypeBuildHost is BDEPEND - build host dependencies (EAPI 7+).
	// Required on the build host for cross-compilation.
	DepTypeBuildHost

	// DepTypeInstall is IDEPEND - install dependencies (EAPI 8).
	// Required at install time on the target system.
	DepTypeInstall

	// DepTypePostMerge is PDEPEND - post-merge dependencies.
	// Installed after the package is merged, used to break circular deps.
	DepTypePostMerge
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
	OpEqualGlob // =pkg-ver* (PMS 8.3.1: prefix match at version component boundary)
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
	Condition  string   // USE flag condition (flat, single flag — legacy)
	Conditions []string // USE flag conditions (all must be true for dep to be active)
	OrGroupID int                // OR-group ID (0 = required, >0 = alternative)
	DepType   DepType            // Dependency type (RDEPEND, BDEPEND, etc.)

	// UseRequire lists USE flags that the provider MUST have enabled.
	// Populated from atom USE deps: dev-libs/openssl[ssl] -> UseRequire=["ssl"]
	UseRequire []string

	// UseBlock lists USE flags that the provider MUST NOT have enabled.
	// Populated from atom USE deps: dev-libs/openssl[-debug] -> UseBlock=["debug"]
	UseBlock []string

	// UseDefault maps USE flags to their default assumption when the provider
	// doesn't declare the flag in IUSE. PMS: flag(+) = assume enabled if absent,
	// flag(-) = assume disabled if absent. If a flag has no default and is absent
	// from IUSE, the check fails.
	UseDefault map[string]bool

	// UseEqual lists flags where provider must match dependent's state.
	// PMS 8.2.6.4: [flag=] — if dependent has flag on, provider must too; if off, off.
	UseEqual []string

	// UseEqualInverse lists flags where provider must be opposite of dependent.
	// PMS 8.2.6.4: [!flag=] — provider.flag = !dependent.flag.
	UseEqualInverse []string

	// UseConditionalIf lists flags where: if dependent has flag on, provider must too.
	// PMS 8.2.6.4: [flag?] — "if I have it, you must too".
	UseConditionalIf []string

	// UseConditionalUnless lists flags where: if dependent has flag off, provider must have off.
	// PMS 8.2.6.4: [!flag?] — "if I don't have it, you must not either".
	UseConditionalUnless []string
}

func (c Constraint) String() string {
	if c.Version == nil {
		return c.Name
	}
	return c.Name + " " + c.Version.String()
}

// HasUseDeps returns true if this constraint requires or blocks USE flags on the provider.
func (c Constraint) HasUseDeps() bool {
	return len(c.UseRequire) > 0 || len(c.UseBlock) > 0 ||
		len(c.UseEqual) > 0 || len(c.UseEqualInverse) > 0 ||
		len(c.UseConditionalIf) > 0 || len(c.UseConditionalUnless) > 0
}

// PackageSatisfiesUseDeps checks whether a package's effective USE flags satisfy
// the USE dependency requirements on this constraint.
// providerUSE: effective USE flags of the candidate provider.
// dependentUSE: effective USE flags of the package declaring this dep (may be nil
// if conditional forms are not used).
func (c Constraint) PackageSatisfiesUseDeps(providerUSE map[string]bool, dependentUSE ...map[string]bool) bool {
	var depUSE map[string]bool
	if len(dependentUSE) > 0 {
		depUSE = dependentUSE[0]
	}

	for _, flag := range c.UseRequire {
		if !c.flagEnabled(flag, providerUSE) {
			return false
		}
	}
	for _, flag := range c.UseBlock {
		if c.flagEnabled(flag, providerUSE) {
			return false
		}
	}
	// PMS 8.2.6.4: [flag=] — provider.flag must equal dependent.flag
	for _, flag := range c.UseEqual {
		if depUSE == nil {
			continue
		}
		if c.flagEnabled(flag, providerUSE) != depUSE[flag] {
			return false
		}
	}
	// [!flag=] — provider.flag must be opposite of dependent.flag
	for _, flag := range c.UseEqualInverse {
		if depUSE == nil {
			continue
		}
		if c.flagEnabled(flag, providerUSE) == depUSE[flag] {
			return false
		}
	}
	// [flag?] — if dependent has flag on, provider must too
	for _, flag := range c.UseConditionalIf {
		if depUSE == nil {
			continue
		}
		if depUSE[flag] && !c.flagEnabled(flag, providerUSE) {
			return false
		}
	}
	// [!flag?] — if dependent has flag off, provider must have off
	for _, flag := range c.UseConditionalUnless {
		if depUSE == nil {
			continue
		}
		if !depUSE[flag] && c.flagEnabled(flag, providerUSE) {
			return false
		}
	}
	return true
}

// flagEnabled returns whether a USE flag is effectively enabled on the provider.
// Consults UseDefault if the flag is not declared in provider's IUSE.
func (c Constraint) flagEnabled(flag string, providerUSE map[string]bool) bool {
	enabled, declared := providerUSE[flag]
	if !declared {
		if dflt, hasDefault := c.UseDefault[flag]; hasDefault {
			return dflt
		}
		return false
	}
	return enabled
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
	case OpEqualGlob:
		return "=" + vc.version + "*"
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
	case OpEqualGlob:
		// =ver* matches if version equals ver or starts with ver followed by
		// a version component separator (., _, -, or 'r' for revision).
		// =5.44* matches 5.44, 5.44.0, 5.44.2-r1 but NOT 5.440
		if version == vc.version {
			return true
		}
		if !strings.HasPrefix(version, vc.version) {
			return false
		}
		next := version[len(vc.version)]
		return next == '.' || next == '_' || next == '-'
	default:
		return true
	}
}

// CompareVersions compares two Gentoo-format version strings per PMS Chapter 3.2-3.3.
// Returns: <0 if v1<v2, 0 if v1==v2, >0 if v1>v2
//
// This implements the full PMS version comparison algorithm:
// - Suffix ordering: _alpha < _beta < _pre < _rc < (release) < _p
// - Letter suffix: 1.0a < 1.0b < 1.0 (no letter is newest)
// - Leading zeros: compared lexicographically after stripping trailing zeros
// - Revisions: -r1, -r2, etc.
func CompareVersions(v1, v2 string) int {
	// Parse both versions using the PMS-compliant parser
	ver1, err1 := parseVersion(v1)
	ver2, err2 := parseVersion(v2)

	// If parsing fails, fall back to legacy comparison
	if err1 != nil || err2 != nil {
		return legacyCompareVersions(v1, v2)
	}

	return compareVersions(ver1, ver2)
}

// legacyCompareVersions provides backward-compatible version comparison
// for versions that don't parse with the strict PMS parser.
func legacyCompareVersions(v1, v2 string) int {
	parts1 := legacyParseComponents(v1)
	parts2 := legacyParseComponents(v2)

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
