package pkg

import (
	"testing"
)

// TestNewVersionConstraint tests creation of version constraints
func TestNewVersionConstraint(t *testing.T) {
	tests := []struct {
		name             string
		operator         VersionOperator
		version          string
		expectedOperator VersionOperator
		expectedVersion  string
	}{
		{
			name:             "Equal constraint",
			operator:         OpEqual,
			version:          "1.0.0",
			expectedOperator: OpEqual,
			expectedVersion:  "1.0.0",
		},
		{
			name:             "Greater than constraint",
			operator:         OpGreater,
			version:          "2.0.0",
			expectedOperator: OpGreater,
			expectedVersion:  "2.0.0",
		},
		{
			name:             "Greater or equal constraint",
			operator:         OpGreaterEqual,
			version:          "1.5.0",
			expectedOperator: OpGreaterEqual,
			expectedVersion:  "1.5.0",
		},
		{
			name:             "Less than constraint",
			operator:         OpLess,
			version:          "3.0.0",
			expectedOperator: OpLess,
			expectedVersion:  "3.0.0",
		},
		{
			name:             "Less or equal constraint",
			operator:         OpLessEqual,
			version:          "2.5.0",
			expectedOperator: OpLessEqual,
			expectedVersion:  "2.5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := NewVersionConstraint(tt.operator, tt.version)

			if vc.Operator() != tt.expectedOperator {
				t.Errorf("NewVersionConstraint().Operator() = %v, expected %v", vc.Operator(), tt.expectedOperator)
			}

			if vc.Version() != tt.expectedVersion {
				t.Errorf("NewVersionConstraint().Version() = %q, expected %q", vc.Version(), tt.expectedVersion)
			}
		})
	}
}

// TestVersionConstraint_Immutability tests that VersionConstraint is immutable
func TestVersionConstraint_Immutability(t *testing.T) {
	vc := NewVersionConstraint(OpGreaterEqual, "1.0.0")

	// Verify getters return consistent values
	if vc.Operator() != OpGreaterEqual {
		t.Errorf("Operator changed: expected %v, got %v", OpGreaterEqual, vc.Operator())
	}

	if vc.Version() != "1.0.0" {
		t.Errorf("Version changed: expected %q, got %q", "1.0.0", vc.Version())
	}

	// Create another constraint with same values
	vc2 := NewVersionConstraint(OpGreaterEqual, "1.0.0")

	// They should have same values (Value Object semantics)
	if vc.Operator() != vc2.Operator() {
		t.Error("Operators should be equal")
	}

	if vc.Version() != vc2.Version() {
		t.Error("Versions should be equal")
	}
}

// TestNewExactVersionConstraint tests exact version constraint factory
func TestNewExactVersionConstraint(t *testing.T) {
	vc := NewExactVersionConstraint("1.2.3")

	if vc.Operator() != OpEqual {
		t.Errorf("NewExactVersionConstraint().Operator() = %v, expected OpEqual", vc.Operator())
	}

	if vc.Version() != "1.2.3" {
		t.Errorf("NewExactVersionConstraint().Version() = %q, expected \"1.2.3\"", vc.Version())
	}
}

// TestNewMinVersionConstraint tests minimum version constraint factory
func TestNewMinVersionConstraint(t *testing.T) {
	vc := NewMinVersionConstraint("2.0.0")

	if vc.Operator() != OpGreaterEqual {
		t.Errorf("NewMinVersionConstraint().Operator() = %v, expected OpGreaterEqual", vc.Operator())
	}

	if vc.Version() != "2.0.0" {
		t.Errorf("NewMinVersionConstraint().Version() = %q, expected \"2.0.0\"", vc.Version())
	}
}

// TestNewMaxVersionConstraint tests maximum version constraint factory
func TestNewMaxVersionConstraint(t *testing.T) {
	vc := NewMaxVersionConstraint("3.0.0")

	if vc.Operator() != OpLessEqual {
		t.Errorf("NewMaxVersionConstraint().Operator() = %v, expected OpLessEqual", vc.Operator())
	}

	if vc.Version() != "3.0.0" {
		t.Errorf("NewMaxVersionConstraint().Version() = %q, expected \"3.0.0\"", vc.Version())
	}
}

// TestVersionConstraint_String tests string representation
func TestVersionConstraint_String(t *testing.T) {
	tests := []struct {
		name     string
		vc       *VersionConstraint
		expected string
	}{
		{
			name:     "Nil constraint returns 'any'",
			vc:       nil,
			expected: "any",
		},
		{
			name:     "Equal operator",
			vc:       NewVersionConstraint(OpEqual, "1.0.0"),
			expected: "1.0.0",
		},
		{
			name:     "Greater operator",
			vc:       NewVersionConstraint(OpGreater, "2.0.0"),
			expected: ">2.0.0",
		},
		{
			name:     "GreaterEqual operator",
			vc:       NewVersionConstraint(OpGreaterEqual, "1.5.0"),
			expected: ">=1.5.0",
		},
		{
			name:     "Less operator",
			vc:       NewVersionConstraint(OpLess, "3.0.0"),
			expected: "<3.0.0",
		},
		{
			name:     "LessEqual operator",
			vc:       NewVersionConstraint(OpLessEqual, "2.5.0"),
			expected: "<=2.5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.vc.String()
			if result != tt.expected {
				t.Errorf("VersionConstraint.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestVersionConstraint_Satisfies tests version satisfaction logic
func TestVersionConstraint_Satisfies(t *testing.T) {
	tests := []struct {
		name       string
		constraint *VersionConstraint
		version    string
		expected   bool
	}{
		// Nil constraint (any version)
		{
			name:       "Nil constraint accepts any version",
			constraint: nil,
			version:    "1.0.0",
			expected:   true,
		},

		// OpEqual tests
		{
			name:       "Equal: 1.0.0 == 1.0.0",
			constraint: NewVersionConstraint(OpEqual, "1.0.0"),
			version:    "1.0.0",
			expected:   true,
		},
		{
			name:       "Equal: 1.0.0 != 2.0.0",
			constraint: NewVersionConstraint(OpEqual, "1.0.0"),
			version:    "2.0.0",
			expected:   false,
		},

		// OpGreater tests
		{
			name:       "Greater: 2.0.0 > 1.0.0",
			constraint: NewVersionConstraint(OpGreater, "1.0.0"),
			version:    "2.0.0",
			expected:   true,
		},
		{
			name:       "Greater: 1.0.0 not > 1.0.0",
			constraint: NewVersionConstraint(OpGreater, "1.0.0"),
			version:    "1.0.0",
			expected:   false,
		},
		{
			name:       "Greater: 0.9.0 not > 1.0.0",
			constraint: NewVersionConstraint(OpGreater, "1.0.0"),
			version:    "0.9.0",
			expected:   false,
		},

		// OpGreaterEqual tests
		{
			name:       "GreaterEqual: 2.0.0 >= 1.0.0",
			constraint: NewVersionConstraint(OpGreaterEqual, "1.0.0"),
			version:    "2.0.0",
			expected:   true,
		},
		{
			name:       "GreaterEqual: 1.0.0 >= 1.0.0",
			constraint: NewVersionConstraint(OpGreaterEqual, "1.0.0"),
			version:    "1.0.0",
			expected:   true,
		},
		{
			name:       "GreaterEqual: 0.9.0 not >= 1.0.0",
			constraint: NewVersionConstraint(OpGreaterEqual, "1.0.0"),
			version:    "0.9.0",
			expected:   false,
		},

		// OpLess tests
		{
			name:       "Less: 1.0.0 < 2.0.0",
			constraint: NewVersionConstraint(OpLess, "2.0.0"),
			version:    "1.0.0",
			expected:   true,
		},
		{
			name:       "Less: 2.0.0 not < 2.0.0",
			constraint: NewVersionConstraint(OpLess, "2.0.0"),
			version:    "2.0.0",
			expected:   false,
		},
		{
			name:       "Less: 3.0.0 not < 2.0.0",
			constraint: NewVersionConstraint(OpLess, "2.0.0"),
			version:    "3.0.0",
			expected:   false,
		},

		// OpLessEqual tests
		{
			name:       "LessEqual: 1.0.0 <= 2.0.0",
			constraint: NewVersionConstraint(OpLessEqual, "2.0.0"),
			version:    "1.0.0",
			expected:   true,
		},
		{
			name:       "LessEqual: 2.0.0 <= 2.0.0",
			constraint: NewVersionConstraint(OpLessEqual, "2.0.0"),
			version:    "2.0.0",
			expected:   true,
		},
		{
			name:       "LessEqual: 3.0.0 not <= 2.0.0",
			constraint: NewVersionConstraint(OpLessEqual, "2.0.0"),
			version:    "3.0.0",
			expected:   false,
		},

		// Complex Gentoo versions
		{
			name:       "GreaterEqual with alpha: 1.0_beta1 >= 1.0_alpha1",
			constraint: NewVersionConstraint(OpGreaterEqual, "1.0_alpha1"),
			version:    "1.0_beta1",
			expected:   true,
		},
		{
			name:       "Less with revision: 1.0-r1 < 1.0-r2",
			constraint: NewVersionConstraint(OpLess, "1.0-r2"),
			version:    "1.0-r1",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.constraint.Satisfies(tt.version)
			if result != tt.expected {
				t.Errorf("VersionConstraint(%s).Satisfies(%q) = %v, expected %v",
					tt.constraint.String(), tt.version, result, tt.expected)
			}
		})
	}
}

// TestCompareVersions tests the CompareVersions function (same logic as Version.CompareTo)
func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int // -1: v1 < v2, 0: v1 == v2, 1: v1 > v2
	}{
		{
			name:     "Equal versions",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "Major version difference (1 < 2)",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
		},
		{
			name:     "Major version difference (2 > 1)",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: 1,
		},
		{
			name:     "Minor version difference",
			v1:       "1.1.0",
			v2:       "1.2.0",
			expected: -1,
		},
		{
			name:     "Patch version difference",
			v1:       "1.0.1",
			v2:       "1.0.2",
			expected: -1,
		},
		{
			name:     "Alpha before beta",
			v1:       "1.0_alpha1",
			v2:       "1.0_beta1",
			expected: -1,
		},
		{
			name:     "Different revisions",
			v1:       "1.0-r1",
			v2:       "1.0-r2",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)

			// Normalize to -1, 0, 1
			normalizedResult := 0
			if result < 0 {
				normalizedResult = -1
			} else if result > 0 {
				normalizedResult = 1
			}

			if normalizedResult != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tt.v1, tt.v2, normalizedResult, tt.expected)
			}
		})
	}
}

// TestParseVersionConstraint tests parsing of constraint strings
func TestParseVersionConstraint(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedOperator VersionOperator
		expectedVersion  string
		shouldBeNil      bool
	}{
		{
			name:        "Empty string returns nil",
			input:       "",
			shouldBeNil: true,
		},
		{
			name:             "Equal operator (=1.0.0)",
			input:            "=1.0.0",
			expectedOperator: OpEqual,
			expectedVersion:  "1.0.0",
		},
		{
			name:             "Greater operator (>2.0.0)",
			input:            ">2.0.0",
			expectedOperator: OpGreater,
			expectedVersion:  "2.0.0",
		},
		{
			name:             "GreaterEqual operator (>=1.5.0)",
			input:            ">=1.5.0",
			expectedOperator: OpGreaterEqual,
			expectedVersion:  "1.5.0",
		},
		{
			name:             "Less operator (<3.0.0)",
			input:            "<3.0.0",
			expectedOperator: OpLess,
			expectedVersion:  "3.0.0",
		},
		{
			name:             "LessEqual operator (<=2.5.0)",
			input:            "<=2.5.0",
			expectedOperator: OpLessEqual,
			expectedVersion:  "2.5.0",
		},
		{
			name:             "No operator defaults to Equal (1.0.0)",
			input:            "1.0.0",
			expectedOperator: OpEqual,
			expectedVersion:  "1.0.0",
		},
		{
			name:             "Complex version with operator (>=1.2.3_alpha4-r5)",
			input:            ">=1.2.3_alpha4-r5",
			expectedOperator: OpGreaterEqual,
			expectedVersion:  "1.2.3_alpha4-r5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc, err := ParseVersionConstraint(tt.input)

			if err != nil {
				t.Errorf("ParseVersionConstraint(%q) unexpected error: %v", tt.input, err)
			}

			if tt.shouldBeNil {
				if vc != nil {
					t.Errorf("ParseVersionConstraint(%q) expected nil, got %v", tt.input, vc)
				}
				return
			}

			if vc == nil {
				t.Errorf("ParseVersionConstraint(%q) returned nil, expected constraint", tt.input)
				return
			}

			if vc.Operator() != tt.expectedOperator {
				t.Errorf("ParseVersionConstraint(%q).Operator() = %v, expected %v",
					tt.input, vc.Operator(), tt.expectedOperator)
			}

			if vc.Version() != tt.expectedVersion {
				t.Errorf("ParseVersionConstraint(%q).Version() = %q, expected %q",
					tt.input, vc.Version(), tt.expectedVersion)
			}
		})
	}
}

// TestConstraint_String tests Constraint.String() method
func TestConstraint_String(t *testing.T) {
	tests := []struct {
		name       string
		constraint Constraint
		expected   string
	}{
		{
			name: "Constraint without version",
			constraint: Constraint{
				Type: ConstraintTypeVersion,
				Name: "sys-libs/zlib",
			},
			expected: "sys-libs/zlib",
		},
		{
			name: "Constraint with exact version",
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "sys-libs/zlib",
				Version: NewExactVersionConstraint("1.2.13"),
			},
			expected: "sys-libs/zlib 1.2.13",
		},
		{
			name: "Constraint with >= version",
			constraint: Constraint{
				Type:    ConstraintTypeVersion,
				Name:    "dev-lang/python",
				Version: NewMinVersionConstraint("3.10"),
			},
			expected: "dev-lang/python >=3.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.constraint.String()
			if result != tt.expected {
				t.Errorf("Constraint.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// TestNewSimpleConstraint tests simple constraint factory
func TestNewSimpleConstraint(t *testing.T) {
	constraint := NewSimpleConstraint("app-misc/hello")

	if constraint.Type != ConstraintTypeVersion {
		t.Errorf("NewSimpleConstraint().Type = %v, expected ConstraintTypeVersion", constraint.Type)
	}

	if constraint.Name != "app-misc/hello" {
		t.Errorf("NewSimpleConstraint().Name = %q, expected \"app-misc/hello\"", constraint.Name)
	}

	if constraint.Version != nil {
		t.Errorf("NewSimpleConstraint().Version = %v, expected nil", constraint.Version)
	}
}

// BenchmarkVersionConstraint_Satisfies benchmarks constraint satisfaction check
func BenchmarkVersionConstraint_Satisfies(b *testing.B) {
	vc := NewVersionConstraint(OpGreaterEqual, "1.0.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vc.Satisfies("2.0.0")
	}
}

// BenchmarkCompareVersions benchmarks version comparison
func BenchmarkCompareVersions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CompareVersions("1.2.3_alpha4-r5", "1.2.3_beta1-r3")
	}
}

// BenchmarkParseVersionConstraint benchmarks constraint parsing
func BenchmarkParseVersionConstraint(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseVersionConstraint(">=1.2.3_alpha4-r5")
	}
}
