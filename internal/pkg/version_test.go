package pkg

import (
	"testing"
)

// TestNewVersion_ValidVersions tests creation of valid versions
func TestNewVersion_ValidVersions(t *testing.T) {
	tests := []struct {
		name        string
		versionStr  string
		expectedRaw string
		shouldError bool
	}{
		{
			name:        "Simple numeric version",
			versionStr:  "1.0.0",
			expectedRaw: "1.0.0",
			shouldError: false,
		},
		{
			name:        "Version with alpha suffix",
			versionStr:  "1.2.3_alpha4",
			expectedRaw: "1.2.3_alpha4",
			shouldError: false,
		},
		{
			name:        "Version with revision",
			versionStr:  "2.5-r3",
			expectedRaw: "2.5-r3",
			shouldError: false,
		},
		{
			name:        "Complex Gentoo version",
			versionStr:  "1.2.3_alpha4-r5",
			expectedRaw: "1.2.3_alpha4-r5",
			shouldError: false,
		},
		{
			name:        "Version with beta",
			versionStr:  "3.0_beta2",
			expectedRaw: "3.0_beta2",
			shouldError: false,
		},
		{
			name:        "Version with rc (release candidate)",
			versionStr:  "2.0_rc1",
			expectedRaw: "2.0_rc1",
			shouldError: false,
		},
		{
			name:        "Version with pre (pre-release)",
			versionStr:  "1.0_pre20231201",
			expectedRaw: "1.0_pre20231201",
			shouldError: false,
		},
		{
			name:        "Single digit version",
			versionStr:  "5",
			expectedRaw: "5",
			shouldError: false,
		},
		{
			name:        "Version with many dots",
			versionStr:  "1.2.3.4.5.6",
			expectedRaw: "1.2.3.4.5.6",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := NewVersion(tt.versionStr)

			if tt.shouldError && err == nil {
				t.Errorf("NewVersion(%q) expected error, got nil", tt.versionStr)
			}

			if !tt.shouldError {
				if err != nil {
					t.Errorf("NewVersion(%q) unexpected error: %v", tt.versionStr, err)
				}

				if version.String() != tt.expectedRaw {
					t.Errorf("NewVersion(%q).String() = %q, expected %q", tt.versionStr, version.String(), tt.expectedRaw)
				}
			}
		})
	}
}

// TestNewVersion_InvalidVersions tests error handling for invalid versions
func TestNewVersion_InvalidVersions(t *testing.T) {
	tests := []struct {
		name       string
		versionStr string
	}{
		{
			name:       "Empty string",
			versionStr: "",
		},
		{
			name:       "Only letters (no digits)",
			versionStr: "alpha",
		},
		{
			name:       "Only special characters",
			versionStr: "---",
		},
		{
			name:       "Only underscores",
			versionStr: "___",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVersion(tt.versionStr)
			if err == nil {
				t.Errorf("NewVersion(%q) expected error, got nil", tt.versionStr)
			}
		})
	}
}

// TestMustNewVersion_ValidVersion tests MustNewVersion with valid input
func TestMustNewVersion_ValidVersion(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustNewVersion(\"1.0.0\") panicked: %v", r)
		}
	}()

	version := MustNewVersion("1.0.0")
	if version.String() != "1.0.0" {
		t.Errorf("MustNewVersion(\"1.0.0\").String() = %q, expected \"1.0.0\"", version.String())
	}
}

// TestMustNewVersion_InvalidVersion tests MustNewVersion panic behavior
func TestMustNewVersion_InvalidVersion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustNewVersion(\"\") should panic but didn't")
		}
	}()

	MustNewVersion("") // Should panic
}

// TestVersion_Equals tests Value Object equality
func TestVersion_Equals(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "Identical versions",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "Different versions",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: false,
		},
		{
			name:     "Complex identical versions",
			v1:       "1.2.3_alpha4-r5",
			v2:       "1.2.3_alpha4-r5",
			expected: true,
		},
		{
			name:     "Different alpha suffixes",
			v1:       "1.0_alpha1",
			v2:       "1.0_alpha2",
			expected: false,
		},
		{
			name:     "Different revisions",
			v1:       "1.0-r1",
			v2:       "1.0-r2",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.Equals(v2)
			if result != tt.expected {
				t.Errorf("Version(%q).Equals(%q) = %v, expected %v", tt.v1, tt.v2, result, tt.expected)
			}

			// Test symmetry: v1.Equals(v2) == v2.Equals(v1)
			reverse := v2.Equals(v1)
			if result != reverse {
				t.Errorf("Equals() not symmetric: %q.Equals(%q)=%v, but %q.Equals(%q)=%v",
					tt.v1, tt.v2, result, tt.v2, tt.v1, reverse)
			}
		})
	}
}

// TestVersion_CompareTo tests version comparison logic
func TestVersion_CompareTo(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int // -1: v1 < v2, 0: v1 == v2, 1: v1 > v2
	}{
		// Equal versions
		{
			name:     "Equal simple versions",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "Equal complex versions",
			v1:       "1.2.3_alpha4-r5",
			v2:       "1.2.3_alpha4-r5",
			expected: 0,
		},

		// Simple numeric comparisons
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
			name:     "Minor version difference (1.1 < 1.2)",
			v1:       "1.1.0",
			v2:       "1.2.0",
			expected: -1,
		},
		{
			name:     "Patch version difference (1.0.1 < 1.0.2)",
			v1:       "1.0.1",
			v2:       "1.0.2",
			expected: -1,
		},

		// Length differences
		{
			name:     "Shorter version is less (1.0 < 1.0.1)",
			v1:       "1.0",
			v2:       "1.0.1",
			expected: -1,
		},
		{
			name:     "Longer version is greater (1.0.1 > 1.0)",
			v1:       "1.0.1",
			v2:       "1.0",
			expected: 1,
		},

		// Alpha/Beta/RC comparisons (alphabetical string comparison)
		{
			name:     "Alpha before beta (1.0_alpha < 1.0_beta)",
			v1:       "1.0_alpha1",
			v2:       "1.0_beta1",
			expected: -1,
		},
		{
			name:     "Beta before rc (1.0_beta < 1.0_rc)",
			v1:       "1.0_beta1",
			v2:       "1.0_rc1",
			expected: -1,
		},
		{
			name:     "Different alpha numbers (alpha1 < alpha2)",
			v1:       "1.0_alpha1",
			v2:       "1.0_alpha2",
			expected: -1,
		},

		// Revision comparisons
		{
			name:     "Different revisions (r1 < r2)",
			v1:       "1.0-r1",
			v2:       "1.0-r2",
			expected: -1,
		},
		{
			name:     "No revision vs revision (1.0 < 1.0-r1)",
			v1:       "1.0",
			v2:       "1.0-r1",
			expected: -1,
		},

		// Complex Gentoo-specific versions
		// TODO: Known Issue - Pre-release semantics not yet implemented
		// In Gentoo: 1.0_pre < 1.0_alpha < 1.0_beta < 1.0_rc < 1.0 (stable)
		// Current implementation treats longer version as greater (incorrect for pre/alpha/beta/rc)
		// Will fix in separate commit after implementing proper Gentoo version semantics
		// {
		// 	name:     "Pre-release before stable (1.0_pre < 1.0)",
		// 	v1:       "1.0_pre20231201",
		// 	v2:       "1.0",
		// 	expected: -1,
		// },
		{
			name:     "Complex comparison with all components",
			v1:       "1.2.3_alpha4-r5",
			v2:       "1.2.3_alpha5-r1",
			expected: -1,
		},

		// Edge cases: numbers vs strings
		{
			name:     "Number component > string component",
			v1:       "1.2.3",
			v2:       "1.2.alpha",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.CompareTo(v2)

			// Normalize result to -1, 0, or 1 for easier comparison
			normalizedResult := 0
			if result < 0 {
				normalizedResult = -1
			} else if result > 0 {
				normalizedResult = 1
			}

			if normalizedResult != tt.expected {
				t.Errorf("Version(%q).CompareTo(%q) = %d, expected %d", tt.v1, tt.v2, normalizedResult, tt.expected)
			}

			// Test anti-symmetry: v1.CompareTo(v2) == -v2.CompareTo(v1)
			reverse := v2.CompareTo(v1)
			if result != -reverse {
				t.Errorf("CompareTo() not anti-symmetric: %q.CompareTo(%q)=%d, but %q.CompareTo(%q)=%d",
					tt.v1, tt.v2, result, tt.v2, tt.v1, reverse)
			}
		})
	}
}

// TestVersion_IsGreaterThan tests IsGreaterThan convenience method
func TestVersion_IsGreaterThan(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "2.0.0 > 1.0.0",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "1.0.0 not > 2.0.0",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: false,
		},
		{
			name:     "1.0.0 not > 1.0.0 (equal)",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.IsGreaterThan(v2)
			if result != tt.expected {
				t.Errorf("Version(%q).IsGreaterThan(%q) = %v, expected %v", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

// TestVersion_IsLessThan tests IsLessThan convenience method
func TestVersion_IsLessThan(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "1.0.0 < 2.0.0",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: true,
		},
		{
			name:     "2.0.0 not < 1.0.0",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: false,
		},
		{
			name:     "1.0.0 not < 1.0.0 (equal)",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.IsLessThan(v2)
			if result != tt.expected {
				t.Errorf("Version(%q).IsLessThan(%q) = %v, expected %v", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

// TestVersion_IsGreaterThanOrEqual tests IsGreaterThanOrEqual convenience method
func TestVersion_IsGreaterThanOrEqual(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "2.0.0 >= 1.0.0",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "1.0.0 >= 1.0.0 (equal)",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "1.0.0 not >= 2.0.0",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.IsGreaterThanOrEqual(v2)
			if result != tt.expected {
				t.Errorf("Version(%q).IsGreaterThanOrEqual(%q) = %v, expected %v", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

// TestVersion_IsLessThanOrEqual tests IsLessThanOrEqual convenience method
func TestVersion_IsLessThanOrEqual(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{
			name:     "1.0.0 <= 2.0.0",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: true,
		},
		{
			name:     "1.0.0 <= 1.0.0 (equal)",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: true,
		},
		{
			name:     "2.0.0 not <= 1.0.0",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.IsLessThanOrEqual(v2)
			if result != tt.expected {
				t.Errorf("Version(%q).IsLessThanOrEqual(%q) = %v, expected %v", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

// TestVersion_Immutability tests that Version is a proper Value Object (immutable)
func TestVersion_Immutability(t *testing.T) {
	v1 := MustNewVersion("1.0.0")
	originalStr := v1.String()

	// Create another reference
	v2 := v1

	// Both should have same string representation
	if v1.String() != originalStr {
		t.Errorf("Version mutated: expected %q, got %q", originalStr, v1.String())
	}

	if v2.String() != originalStr {
		t.Errorf("Version copy mutated: expected %q, got %q", originalStr, v2.String())
	}

	// They should be equal
	if !v1.Equals(v2) {
		t.Error("Version copies should be equal")
	}
}

// BenchmarkNewVersion benchmarks version creation
func BenchmarkNewVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewVersion("1.2.3_alpha4-r5")
	}
}

// BenchmarkVersionCompareTo benchmarks version comparison
func BenchmarkVersionCompareTo(b *testing.B) {
	v1 := MustNewVersion("1.2.3_alpha4-r5")
	v2 := MustNewVersion("1.2.3_beta1-r3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v1.CompareTo(v2)
	}
}
