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
		{
			name:        "Version with letter suffix",
			versionStr:  "1.0a",
			expectedRaw: "1.0a",
			shouldError: false,
		},
		{
			name:        "Version with letter and _suffix",
			versionStr:  "1.0a_alpha1",
			expectedRaw: "1.0a_alpha1",
			shouldError: false,
		},
		{
			name:        "Version with patchlevel",
			versionStr:  "1.0_p1",
			expectedRaw: "1.0_p1",
			shouldError: false,
		},
		{
			name:        "Version with leading zeros",
			versionStr:  "1.01.002",
			expectedRaw: "1.01.002",
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

		// ========================================
		// v0.2.1-001: Suffix Ordering (PMS 3.5-3.6)
		// CRITICAL: _alpha < _beta < _pre < _rc < (release) < _p
		// ========================================
		{
			name:     "v0.2.1-001: alpha < beta",
			v1:       "1.0_alpha",
			v2:       "1.0_beta",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: alpha1 < beta1",
			v1:       "1.0_alpha1",
			v2:       "1.0_beta1",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: beta < pre",
			v1:       "1.0_beta",
			v2:       "1.0_pre",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: pre < rc",
			v1:       "1.0_pre",
			v2:       "1.0_rc",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: rc < release (no suffix)",
			v1:       "1.0_rc1",
			v2:       "1.0",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: release < patchlevel",
			v1:       "1.0",
			v2:       "1.0_p1",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: CRITICAL - rc1 < p1 (was bug: p < rc alphabetically)",
			v1:       "1.0_rc1",
			v2:       "1.0_p1",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: alpha < p (full chain)",
			v1:       "1.0_alpha1",
			v2:       "1.0_p1",
			expected: -1,
		},
		{
			name:     "v0.2.1-001: pre release before stable",
			v1:       "1.0_pre20231201",
			v2:       "1.0",
			expected: -1,
		},

		// ========================================
		// v0.2.1-003: Letter Suffix (PMS 3.4)
		// CRITICAL: 1.0a < 1.0b < ... < 1.0z < 1.0 (no letter is newest)
		// ========================================
		{
			name:     "v0.2.1-003: 1.0a < 1.0b",
			v1:       "1.0a",
			v2:       "1.0b",
			expected: -1,
		},
		{
			name:     "v0.2.1-003: 1.0y < 1.0z",
			v1:       "1.0y",
			v2:       "1.0z",
			expected: -1,
		},
		{
			name:     "v0.2.1-003: letter < no letter (1.0z < 1.0)",
			v1:       "1.0z",
			v2:       "1.0",
			expected: -1,
		},
		{
			name:     "v0.2.1-003: no letter > letter (1.0 > 1.0a)",
			v1:       "1.0",
			v2:       "1.0a",
			expected: 1,
		},
		{
			name:     "v0.2.1-003: letter with suffix (1.0a_alpha < 1.0a_beta)",
			v1:       "1.0a_alpha",
			v2:       "1.0a_beta",
			expected: -1,
		},
		{
			name:     "v0.2.1-003: 1.0a < 1.0b_alpha (letter takes precedence)",
			v1:       "1.0a",
			v2:       "1.0b_alpha",
			expected: -1,
		},

		// ========================================
		// v0.2.1-002: Leading Zeros (PMS 3.3)
		// CRITICAL: strip trailing zeros, compare lexicographically
		// ========================================
		{
			name:     "v0.2.1-002: 1.01 < 1.1 (leading zero lexicographic)",
			v1:       "1.01",
			v2:       "1.1",
			expected: -1,
		},
		{
			name:     "v0.2.1-002: 1.010 vs 1.01 (trailing zeros stripped)",
			v1:       "1.010",
			v2:       "1.01",
			expected: 0,
		},
		{
			name:     "v0.2.1-002: 1.001 < 1.01",
			v1:       "1.001",
			v2:       "1.01",
			expected: -1,
		},
		{
			name:     "v0.2.1-002: 1.02 > 1.01",
			v1:       "1.02",
			v2:       "1.01",
			expected: 1,
		},

		// Combined scenarios
		{
			name:     "Combined: letter + suffix (1.0a_alpha < 1.0a_beta)",
			v1:       "1.0a_alpha",
			v2:       "1.0a_beta",
			expected: -1,
		},
		{
			name:     "Combined: different alpha numbers (alpha1 < alpha2)",
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
		{
			name:     "Revision with patchlevel (1.0_p1-r1 < 1.0_p1-r2)",
			v1:       "1.0_p1-r1",
			v2:       "1.0_p1-r2",
			expected: -1,
		},

		// Complex Gentoo-specific versions
		{
			name:     "Complex comparison with all components",
			v1:       "1.2.3_alpha4-r5",
			v2:       "1.2.3_alpha5-r1",
			expected: -1,
		},
		{
			name:     "Complex: same base, different suffix type",
			v1:       "1.2.3_beta1",
			v2:       "1.2.3_rc1",
			expected: -1,
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
				t.Errorf("Version(%q).CompareTo(%q) = %d (raw: %d), expected %d",
					tt.v1, tt.v2, normalizedResult, result, tt.expected)
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

// TestCompareVersions_PMSCompliance tests CompareVersions function directly
// to ensure PMS Chapter 3.2-3.3 compliance
func TestCompareVersions_PMSCompliance(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// v0.2.1-001: Suffix ordering
		{"suffix: alpha < beta", "1.0_alpha", "1.0_beta", -1},
		{"suffix: beta < pre", "1.0_beta", "1.0_pre", -1},
		{"suffix: pre < rc", "1.0_pre", "1.0_rc", -1},
		{"suffix: rc < release", "1.0_rc", "1.0", -1},
		{"suffix: release < p", "1.0", "1.0_p", -1},
		{"suffix: CRITICAL rc < p", "1.0_rc1", "1.0_p1", -1},

		// v0.2.1-002: Leading zeros (not currently triggered in test versions)
		// Note: PMS says leading zero comparison is lexicographic after stripping trailing zeros

		// v0.2.1-003: Letter suffix
		{"letter: a < b", "1.0a", "1.0b", -1},
		{"letter: z < (none)", "1.0z", "1.0", -1},
		{"letter: (none) > a", "1.0", "1.0a", 1},

		// Revisions
		{"revision: r0 == no revision", "1.0-r0", "1.0", 0},
		{"revision: r1 < r2", "1.0-r1", "1.0-r2", -1},

		// Complex combinations
		{"complex: 1.2.3_alpha < 1.2.3", "1.2.3_alpha", "1.2.3", -1},
		{"complex: 1.2.3 < 1.2.3_p1", "1.2.3", "1.2.3_p1", -1},
		{"complex: full chain", "1.0_alpha1", "1.0_p1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)

			// Normalize to -1, 0, 1
			normalized := 0
			if result < 0 {
				normalized = -1
			} else if result > 0 {
				normalized = 1
			}

			if normalized != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %d, expected %d",
					tt.v1, tt.v2, normalized, tt.expected)
			}
		})
	}
}

// TestVersion_SuffixOrdering_Comprehensive tests all suffix orderings exhaustively
func TestVersion_SuffixOrdering_Comprehensive(t *testing.T) {
	// Define the correct order per PMS Algorithm 3.5-3.6
	// _alpha < _beta < _pre < _rc < (no suffix) < _p
	orderedVersions := []string{
		"1.0_alpha",
		"1.0_alpha1",
		"1.0_alpha2",
		"1.0_beta",
		"1.0_beta1",
		"1.0_pre",
		"1.0_pre1",
		"1.0_rc",
		"1.0_rc1",
		"1.0_rc2",
		"1.0", // release
		"1.0_p",
		"1.0_p1",
		"1.0_p2",
	}

	for i := 0; i < len(orderedVersions)-1; i++ {
		for j := i + 1; j < len(orderedVersions); j++ {
			v1 := MustNewVersion(orderedVersions[i])
			v2 := MustNewVersion(orderedVersions[j])

			result := v1.CompareTo(v2)
			if result >= 0 {
				t.Errorf("Expected %q < %q, but CompareTo returned %d",
					orderedVersions[i], orderedVersions[j], result)
			}

			// Also test reverse
			reverseResult := v2.CompareTo(v1)
			if reverseResult <= 0 {
				t.Errorf("Expected %q > %q, but CompareTo returned %d",
					orderedVersions[j], orderedVersions[i], reverseResult)
			}
		}
	}
}

// TestVersion_LetterSuffix_Comprehensive tests letter suffix ordering
func TestVersion_LetterSuffix_Comprehensive(t *testing.T) {
	// Per PMS 3.4: 1.0a < 1.0b < ... < 1.0z < 1.0 (no letter)
	letters := "abcdefghijklmnopqrstuvwxyz"
	versions := make([]string, len(letters)+1)

	for i, ch := range letters {
		versions[i] = "1.0" + string(ch)
	}
	versions[len(letters)] = "1.0" // no letter is last (highest)

	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			v1 := MustNewVersion(versions[i])
			v2 := MustNewVersion(versions[j])

			result := v1.CompareTo(v2)
			if result >= 0 {
				t.Errorf("Expected %q < %q, but CompareTo returned %d",
					versions[i], versions[j], result)
			}
		}
	}
}

// TestVersion_LeadingZeros tests PMS Algorithm 3.3 leading zero comparison
func TestVersion_LeadingZeros(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Leading zero triggers lexicographic comparison after stripping trailing zeros
		{"01 vs 1: leading zero triggers special comparison", "1.01", "1.1", -1},
		{"010 vs 01: trailing zeros stripped, equal", "1.010", "1.01", 0},
		{"001 vs 01: lexicographic 001 < 01", "1.001", "1.01", -1},
		{"02 vs 01: 02 > 01", "1.02", "1.01", 1},
		{"0 vs 00: both normalize to same", "1.0", "1.00", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1 := MustNewVersion(tt.v1)
			v2 := MustNewVersion(tt.v2)

			result := v1.CompareTo(v2)

			normalized := 0
			if result < 0 {
				normalized = -1
			} else if result > 0 {
				normalized = 1
			}

			if normalized != tt.expected {
				t.Errorf("Version(%q).CompareTo(%q) = %d, expected %d",
					tt.v1, tt.v2, normalized, tt.expected)
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
		{
			name:     "1.0_p1 > 1.0 (patchlevel > release)",
			v1:       "1.0_p1",
			v2:       "1.0",
			expected: true,
		},
		{
			name:     "1.0 > 1.0_rc1 (release > rc)",
			v1:       "1.0",
			v2:       "1.0_rc1",
			expected: true,
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
		{
			name:     "1.0_rc1 < 1.0_p1 (rc < patchlevel)",
			v1:       "1.0_rc1",
			v2:       "1.0_p1",
			expected: true,
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

// TestVersionConstraint_Satisfies_PMS tests the Satisfies method with PMS-compliant versions
func TestVersionConstraint_Satisfies_PMS(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		expected   bool
	}{
		// Basic version constraints
		{"exact match", "1.0", "1.0", true},
		{"exact no match", "1.0", "1.1", false},
		{"greater equal pass", ">=1.0", "1.1", true},
		{"greater equal exact", ">=1.0", "1.0", true},
		{"greater equal fail", ">=1.0", "0.9", false},

		// PMS suffix ordering in constraints
		{"rc satisfies >= alpha", ">=1.0_alpha", "1.0_rc1", true},
		{"p satisfies >= rc", ">=1.0_rc1", "1.0_p1", true},
		{"alpha does not satisfy >= rc", ">=1.0_rc1", "1.0_alpha1", false},
		{"release satisfies >= rc", ">=1.0_rc1", "1.0", true},
		{"rc does not satisfy >= release", ">=1.0", "1.0_rc1", false},

		// Letter suffix in constraints
		{"1.0b satisfies >= 1.0a", ">=1.0a", "1.0b", true},
		{"1.0 satisfies >= 1.0z", ">=1.0z", "1.0", true},
		{"1.0a does not satisfy >= 1.0", ">=1.0", "1.0a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc, err := ParseVersionConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseVersionConstraint(%q) error: %v", tt.constraint, err)
			}

			result := vc.Satisfies(tt.version)
			if result != tt.expected {
				t.Errorf("VersionConstraint(%q).Satisfies(%q) = %v, expected %v",
					tt.constraint, tt.version, result, tt.expected)
			}
		})
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

// BenchmarkCompareVersions_PMS benchmarks the direct comparison function
func BenchmarkCompareVersions_PMS(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CompareVersions("1.2.3_alpha4-r5", "1.2.3_beta1-r3")
	}
}
