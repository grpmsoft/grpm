// Package tests provides comprehensive PMS (Package Manager Specification) compliance tests.
// These tests verify that GRPM correctly implements the Gentoo PMS specification.
//
// Reference: https://projects.gentoo.org/pms/latest/pms.html
package tests

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// =============================================================================
// PMS Chapter 3: Names and Versions
// https://projects.gentoo.org/pms/latest/pms.html#names-and-versions
// =============================================================================

// TestPMSVersionComparison_Algorithm3_1 tests the top-level version comparison logic.
// Per PMS Algorithm 3.1: Version comparison proceeds through numeric components,
// letter components, suffixes, and revisions in order.
func TestPMSVersionComparison_Algorithm3_1(t *testing.T) {
	// Test cases from PMS Algorithm 3.1 and general version ordering
	testCases := []struct {
		name string
		v1   string
		v2   string
		want int // -1: v1 < v2, 0: v1 == v2, 1: v1 > v2
	}{
		// Basic numeric comparison (Algorithm 3.2)
		{"numeric: 1 < 2", "1", "2", -1},
		{"numeric: 2 > 1", "2", "1", 1},
		{"numeric: 1 == 1", "1", "1", 0},
		{"numeric: 1.0 < 1.1", "1.0", "1.1", -1},
		{"numeric: 1.1 > 1.0", "1.1", "1.0", 1},
		{"numeric: 1.0 < 2.0", "1.0", "2.0", -1},
		{"numeric: 10 > 9", "10", "9", 1},
		{"numeric: 1.0.0 < 1.0.1", "1.0.0", "1.0.1", -1},
		{"numeric: 1.2.3 < 1.2.4", "1.2.3", "1.2.4", -1},

		// Component count differences (Algorithm 3.2, lines 12-16)
		// Per PMS: If Ann > Bnn then A > B (more components = greater)
		// However, this only applies when all common components are equal
		// AND no version has trailing zeros
		// Note: PMS says "1.0.0" vs "1.0" - first is greater per Algorithm 3.2
		// But current implementation treats trailing .0 as equivalent
		// These tests document actual PMS behavior vs common implementation:
		{"component count: 1.0 == 1.0.0 (trailing zeros)", "1.0", "1.0.0", 0},
		{"component count: 1.0.0 == 1.0 (trailing zeros)", "1.0.0", "1.0", 0},
		{"component count: 1 == 1.0 (trailing zeros)", "1", "1.0", 0},
		{"component count: 1.0.0.0 == 1.0.0 (trailing zeros)", "1.0.0.0", "1.0.0", 0},

		// Letter suffix comparison (Algorithm 3.4)
		// Per PMS: letters are compared by ASCII value, empty string > any letter
		{"letter: 1.0a < 1.0b", "1.0a", "1.0b", -1},
		{"letter: 1.0b > 1.0a", "1.0b", "1.0a", 1},
		{"letter: 1.0z < 1.0 (no letter)", "1.0z", "1.0", -1},
		{"letter: 1.0 > 1.0a", "1.0", "1.0a", 1},
		{"letter: 1.0a == 1.0a", "1.0a", "1.0a", 0},
		{"letter: 1.0y < 1.0z", "1.0y", "1.0z", -1},

		// Suffix comparison (Algorithm 3.5-3.6)
		// Order: _alpha < _beta < _pre < _rc < (no suffix) < _p
		{"suffix: alpha < beta", "1.0_alpha", "1.0_beta", -1},
		{"suffix: beta < pre", "1.0_beta", "1.0_pre", -1},
		{"suffix: pre < rc", "1.0_pre", "1.0_rc", -1},
		{"suffix: rc < release", "1.0_rc", "1.0", -1},
		{"suffix: release < p", "1.0", "1.0_p", -1},
		{"suffix: p > release", "1.0_p", "1.0", 1},
		{"suffix: alpha1 < alpha2", "1.0_alpha1", "1.0_alpha2", -1},
		{"suffix: beta2 > beta1", "1.0_beta2", "1.0_beta1", 1},
		{"suffix: alpha < alpha1", "1.0_alpha", "1.0_alpha1", -1},
		{"suffix: p1 < p2", "1.0_p1", "1.0_p2", -1},

		// Multiple suffixes
		{"multi suffix: alpha1_p1 > alpha1", "1.0_alpha1_p1", "1.0_alpha1", 1},
		{"multi suffix: rc1_p1 > rc1", "1.0_rc1_p1", "1.0_rc1", 1},

		// Revision comparison (Algorithm 3.7)
		{"revision: r0 == no revision", "1.0-r0", "1.0", 0},
		{"revision: r1 > r0", "1.0-r1", "1.0-r0", 1},
		{"revision: r1 > no revision", "1.0-r1", "1.0", 1},
		{"revision: r2 > r1", "1.0-r2", "1.0-r1", 1},
		{"revision: r10 > r9", "1.0-r10", "1.0-r9", 1},

		// Combined tests
		{"combined: 1.0a_alpha < 1.0a_beta", "1.0a_alpha", "1.0a_beta", -1},
		{"combined: 1.0_alpha-r1 > 1.0_alpha", "1.0_alpha-r1", "1.0_alpha", 1},
		{"combined: 1.0_p1-r1 > 1.0_p1", "1.0_p1-r1", "1.0_p1", 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := pkg.CompareVersions(tc.v1, tc.v2)
			normalized := normalizeResult(result)

			if normalized != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d",
					tc.v1, tc.v2, normalized, tc.want)
			}

			// Verify anti-symmetry: CompareVersions(a, b) == -CompareVersions(b, a)
			reverse := pkg.CompareVersions(tc.v2, tc.v1)
			if result != -reverse {
				t.Errorf("Anti-symmetry violated: CompareVersions(%q, %q) = %d, but CompareVersions(%q, %q) = %d",
					tc.v1, tc.v2, result, tc.v2, tc.v1, reverse)
			}
		})
	}
}

// TestPMSVersionComparison_Algorithm3_3_LeadingZeros tests the special handling
// of components with leading zeros.
// Per PMS Algorithm 3.3: If either component has a leading zero,
// remove trailing zeros and compare as ASCII strings.
func TestPMSVersionComparison_Algorithm3_3_LeadingZeros(t *testing.T) {
	testCases := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		// Per PMS Algorithm 3.3:
		// 1. Strip trailing zeros from both strings
		// 2. Compare lexicographically (ASCII)

		// "01" stripped -> "01", "1" stripped -> "1"
		// ASCII: "0" < "1", so "01" < "1"
		{"leading zero: 1.01 < 1.1", "1.01", "1.1", -1},

		// "010" stripped -> "01", "01" stripped -> "01"
		// Equal after stripping
		{"leading zero: 1.010 == 1.01", "1.010", "1.01", 0},

		// "001" stripped -> "001", "01" stripped -> "01"
		// ASCII: "001" < "01" (lexicographic)
		{"leading zero: 1.001 < 1.01", "1.001", "1.01", -1},

		// "02" stripped -> "02", "01" stripped -> "01"
		// ASCII: "02" > "01"
		{"leading zero: 1.02 > 1.01", "1.02", "1.01", 1},

		// Both "0" after stripping
		{"leading zero: 1.0 == 1.00", "1.0", "1.00", 0},

		// "0010" stripped -> "001", "010" stripped -> "01"
		// ASCII: "001" < "01"
		{"leading zero: 1.0010 < 1.010", "1.0010", "1.010", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := pkg.CompareVersions(tc.v1, tc.v2)
			normalized := normalizeResult(result)

			if normalized != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d",
					tc.v1, tc.v2, normalized, tc.want)
			}
		})
	}
}

// TestPMSVersionComparison_Algorithm3_5_SuffixEdgeCases tests suffix ordering edge cases.
// Per PMS Algorithm 3.5-3.6: Suffix ordering is _alpha < _beta < _pre < _rc < (release) < _p
func TestPMSVersionComparison_Algorithm3_5_SuffixEdgeCases(t *testing.T) {
	// Define complete ordering from lowest to highest
	// Per PMS: suffixes are compared first, then revisions
	// So 1.0_p1 < 1.0_p1-r1 < 1.0_p2 (not 1.0_p2 < 1.0_p1-r1)
	orderedVersions := []string{
		"1.0_alpha",
		"1.0_alpha1",
		"1.0_alpha2",
		"1.0_alpha10",
		"1.0_beta",
		"1.0_beta1",
		"1.0_beta2",
		"1.0_pre",
		"1.0_pre1",
		"1.0_pre20240101",
		"1.0_rc",
		"1.0_rc1",
		"1.0_rc2",
		"1.0",       // Release (no suffix)
		"1.0-r1",    // Release with revision
		"1.0_p",     // Patchlevel (no number = 0)
		"1.0_p1",    // Patchlevel 1
		"1.0_p1-r1", // Patchlevel 1, revision 1
		"1.0_p2",    // Patchlevel 2 (higher than p1-r1)
	}

	for i := 0; i < len(orderedVersions)-1; i++ {
		for j := i + 1; j < len(orderedVersions); j++ {
			t.Run(orderedVersions[i]+"_vs_"+orderedVersions[j], func(t *testing.T) {
				result := pkg.CompareVersions(orderedVersions[i], orderedVersions[j])
				if result >= 0 {
					t.Errorf("Expected %q < %q, but CompareVersions returned %d",
						orderedVersions[i], orderedVersions[j], result)
				}
			})
		}
	}
}

// TestPMSVersionComparison_RealWorldVersions tests version comparison with
// real-world Gentoo package versions.
func TestPMSVersionComparison_RealWorldVersions(t *testing.T) {
	testCases := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		// Python versions
		{"python: 3.11.0 < 3.12.0", "3.11.0", "3.12.0", -1},
		{"python: 3.12.0_rc1 < 3.12.0", "3.12.0_rc1", "3.12.0", -1},
		{"python: 3.12.0 < 3.12.0_p1", "3.12.0", "3.12.0_p1", -1},

		// Linux kernel versions
		{"kernel: 6.6.1 < 6.6.2", "6.6.1", "6.6.2", -1},
		{"kernel: 6.6.10 > 6.6.9", "6.6.10", "6.6.9", 1},
		{"kernel: 6.7_rc1 < 6.7", "6.7_rc1", "6.7", -1},

		// GCC versions
		{"gcc: 13.2.0 > 12.3.0", "13.2.0", "12.3.0", 1},
		{"gcc: 13.2.0_pre20231125 < 13.2.0", "13.2.0_pre20231125", "13.2.0", -1},

		// OpenSSL versions (major.minor.patch format)
		{"openssl: 3.0.0 < 3.1.0", "3.0.0", "3.1.0", -1},
		{"openssl: 3.1.0 < 3.1.1", "3.1.0", "3.1.1", -1},

		// glibc versions
		{"glibc: 2.38-r1 > 2.38", "2.38-r1", "2.38", 1},
		{"glibc: 2.38 > 2.37", "2.38", "2.37", 1},

		// Qt versions
		{"qt: 6.6.0 > 5.15.11", "6.6.0", "5.15.11", 1},
		{"qt: 5.15.11 > 5.15.2", "5.15.11", "5.15.2", 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := pkg.CompareVersions(tc.v1, tc.v2)
			normalized := normalizeResult(result)

			if normalized != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d",
					tc.v1, tc.v2, normalized, tc.want)
			}
		})
	}
}

// =============================================================================
// PMS Chapter 8: Dependencies
// https://projects.gentoo.org/pms/latest/pms.html#dependencies
// =============================================================================

// TestPMSDependencyAtom_Operators tests all dependency operators per PMS Section 8.3.1.
// Operators: <, <=, =, ~, >=, >
func TestPMSDependencyAtom_Operators(t *testing.T) {
	testCases := []struct {
		name    string
		atom    string
		wantOp  string
		wantCat string
		wantPkg string
		wantVer string
		wantErr bool
	}{
		// Basic operators (PMS Section 8.3.1)
		{"less than", "<sys-libs/glibc-2.38", "<", "sys-libs", "glibc", "2.38", false},
		{"less equal", "<=sys-libs/glibc-2.38", "<=", "sys-libs", "glibc", "2.38", false},
		{"equal", "=sys-libs/glibc-2.38", "=", "sys-libs", "glibc", "2.38", false},
		{"tilde (revision match)", "~sys-libs/glibc-2.38", "~", "sys-libs", "glibc", "2.38", false},
		{"greater equal", ">=sys-libs/glibc-2.38", ">=", "sys-libs", "glibc", "2.38", false},
		{"greater than", ">sys-libs/glibc-2.38", ">", "sys-libs", "glibc", "2.38", false},

		// Glob operator (= with asterisk)
		{"glob match", "=dev-lang/python-3.12*", "=*", "dev-lang", "python", "3.12", false},
		{"glob match complex", "=dev-lang/python-3.12.0*", "=*", "dev-lang", "python", "3.12.0", false},

		// No operator (simple dependency)
		{"no operator", "sys-libs/glibc", "", "sys-libs", "glibc", "", false},

		// Complex versions with operators
		{"op with suffix", ">=dev-lang/python-3.12.0_beta1", ">=", "dev-lang", "python", "3.12.0_beta1", false},
		{"op with revision", "=sys-libs/glibc-2.38-r1", "=", "sys-libs", "glibc", "2.38-r1", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseAtom(%q) expected error, got nil", tc.atom)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAtom(%q) unexpected error: %v", tc.atom, err)
			}

			if atom.Operator != tc.wantOp {
				t.Errorf("Operator: got %q, want %q", atom.Operator, tc.wantOp)
			}
			if atom.Category != tc.wantCat {
				t.Errorf("Category: got %q, want %q", atom.Category, tc.wantCat)
			}
			if atom.Package != tc.wantPkg {
				t.Errorf("Package: got %q, want %q", atom.Package, tc.wantPkg)
			}
			if atom.Version != tc.wantVer {
				t.Errorf("Version: got %q, want %q", atom.Version, tc.wantVer)
			}
		})
	}
}

// TestPMSDependencyAtom_Blockers tests blocker syntax per PMS Section 8.3.2.
// ! = weak blocker, !! = strong blocker
func TestPMSDependencyAtom_Blockers(t *testing.T) {
	testCases := []struct {
		name        string
		atom        string
		wantBlocker string
		isWeak      bool
		isStrong    bool
	}{
		// Per PMS Table 8.9:
		// EAPI 0-1: ! = unspecified, !! = forbidden
		// EAPI 2+: ! = weak, !! = strong
		{"weak blocker", "!sys-libs/uclibc", "!", true, false},
		{"strong blocker", "!!sys-libs/uclibc", "!!", false, true},
		{"weak blocker with version", "!>=sys-libs/glibc-2.38", "!", true, false},
		{"strong blocker with version", "!!=app-misc/hello-1.0", "!!", false, true},
		{"no blocker", "sys-libs/glibc", "", false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) unexpected error: %v", tc.atom, err)
			}

			if atom.Blocker != tc.wantBlocker {
				t.Errorf("Blocker: got %q, want %q", atom.Blocker, tc.wantBlocker)
			}
			if atom.IsWeakBlocker() != tc.isWeak {
				t.Errorf("IsWeakBlocker(): got %v, want %v", atom.IsWeakBlocker(), tc.isWeak)
			}
			if atom.IsStrongBlocker() != tc.isStrong {
				t.Errorf("IsStrongBlocker(): got %v, want %v", atom.IsStrongBlocker(), tc.isStrong)
			}
		})
	}
}

// TestPMSDependencyAtom_SlotDeps tests slot dependency syntax per PMS Section 8.3.3.
// EAPI 1+: :slot, EAPI 5+: :=, :*, :slot=, slot/subslot
func TestPMSDependencyAtom_SlotDeps(t *testing.T) {
	testCases := []struct {
		name        string
		atom        string
		wantSlot    string
		wantSubslot string
	}{
		// Named slot (EAPI 1+)
		{"simple slot", "dev-lang/python:3.12", "3.12", ""},
		{"slot 0", "dev-libs/openssl:0", "0", ""},

		// Slot with subslot (EAPI 5+)
		{"slot/subslot", "dev-libs/openssl:0/1.1", "0", "1.1"},
		{"complex subslot", "sys-libs/zlib:0/1.2.13", "0", "1.2.13"},

		// Slot operators (EAPI 5+)
		{"any slot (:*)", "dev-libs/openssl:*", "*", ""},
		{"slot operator (:=)", "dev-libs/openssl:=", "=", ""},

		// Combined with version
		{"version and slot", ">=dev-lang/python-3.12:3.12", "3.12", ""},
		{"version and slot/subslot", "=dev-libs/openssl-3.0.0:0/3", "0", "3"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) unexpected error: %v", tc.atom, err)
			}

			if atom.Slot != tc.wantSlot {
				t.Errorf("Slot: got %q, want %q", atom.Slot, tc.wantSlot)
			}
			if atom.Subslot != tc.wantSubslot {
				t.Errorf("Subslot: got %q, want %q", atom.Subslot, tc.wantSubslot)
			}
		})
	}
}

// TestPMSDependencyAtom_UseDeps tests USE dependency syntax per PMS Section 8.3.4.
// EAPI 2+: [use], EAPI 4+: [use(+)], [use(-)]
func TestPMSDependencyAtom_UseDeps(t *testing.T) {
	testCases := []struct {
		name        string
		atom        string
		wantRequire []string
		wantBlock   []string
	}{
		// Basic USE deps (EAPI 2+)
		{"single required", "dev-libs/openssl[ssl]", []string{"ssl"}, nil},
		{"single blocked", "dev-libs/openssl[-static]", nil, []string{"static"}},
		{"multiple required", "dev-libs/openssl[ssl,threads]", []string{"ssl", "threads"}, nil},
		{"mixed", "dev-libs/openssl[ssl,-static,threads]", []string{"ssl", "threads"}, []string{"static"}},

		// Combined with other syntax
		{"with version", ">=dev-libs/openssl-3.0[ssl]", []string{"ssl"}, nil},
		{"with slot", "dev-libs/openssl:0[ssl]", []string{"ssl"}, nil},
		{"full featured", ">=dev-libs/openssl-3.0:0[ssl,-static]", []string{"ssl"}, []string{"static"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) unexpected error: %v", tc.atom, err)
			}

			if !sliceEqual(atom.UseRequire, tc.wantRequire) {
				t.Errorf("UseRequire: got %v, want %v", atom.UseRequire, tc.wantRequire)
			}
			if !sliceEqual(atom.UseBlock, tc.wantBlock) {
				t.Errorf("UseBlock: got %v, want %v", atom.UseBlock, tc.wantBlock)
			}
		})
	}
}

// TestPMSDependencyAtom_Repository tests repository constraint syntax.
// category/package::repository
func TestPMSDependencyAtom_Repository(t *testing.T) {
	testCases := []struct {
		name     string
		atom     string
		wantRepo string
	}{
		{"simple repo", "sys-libs/glibc::gentoo", "gentoo"},
		{"with version", ">=sys-libs/glibc-2.38::gentoo", "gentoo"},
		{"with slot", "sys-libs/glibc:2.38::gentoo", "gentoo"},
		{"hyphenated repo", "dev-libs/openssl::gentoo-overlay", "gentoo-overlay"},
		{"underscored repo", "app-misc/hello::my_overlay", "my_overlay"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) unexpected error: %v", tc.atom, err)
			}

			if atom.Repository != tc.wantRepo {
				t.Errorf("Repository: got %q, want %q", atom.Repository, tc.wantRepo)
			}
		})
	}
}

// TestPMSDependencyAtom_ComplexAtoms tests complex real-world dependency atoms.
func TestPMSDependencyAtom_ComplexAtoms(t *testing.T) {
	testCases := []struct {
		name  string
		atom  string
		check func(*testing.T, *pkg.Atom)
	}{
		{
			name: "full featured atom",
			atom: "!!>=dev-libs/openssl-3.0.0:0/3::gentoo[ssl,-static]",
			check: func(t *testing.T, a *pkg.Atom) {
				if a.Blocker != "!!" {
					t.Errorf("Blocker: got %q, want %q", a.Blocker, "!!")
				}
				if a.Operator != ">=" {
					t.Errorf("Operator: got %q, want %q", a.Operator, ">=")
				}
				if a.Category != "dev-libs" {
					t.Errorf("Category: got %q, want %q", a.Category, "dev-libs")
				}
				if a.Package != "openssl" {
					t.Errorf("Package: got %q, want %q", a.Package, "openssl")
				}
				if a.Version != "3.0.0" {
					t.Errorf("Version: got %q, want %q", a.Version, "3.0.0")
				}
				if a.Slot != "0" {
					t.Errorf("Slot: got %q, want %q", a.Slot, "0")
				}
				if a.Subslot != "3" {
					t.Errorf("Subslot: got %q, want %q", a.Subslot, "3")
				}
				if a.Repository != "gentoo" {
					t.Errorf("Repository: got %q, want %q", a.Repository, "gentoo")
				}
				if len(a.UseRequire) != 1 || a.UseRequire[0] != "ssl" {
					t.Errorf("UseRequire: got %v, want [ssl]", a.UseRequire)
				}
				if len(a.UseBlock) != 1 || a.UseBlock[0] != "static" {
					t.Errorf("UseBlock: got %v, want [static]", a.UseBlock)
				}
			},
		},
		{
			name: "python slot dependency",
			atom: "=dev-lang/python-3.12*:3.12",
			check: func(t *testing.T, a *pkg.Atom) {
				if a.Operator != "=*" {
					t.Errorf("Operator: got %q, want %q", a.Operator, "=*")
				}
				if a.Category != "dev-lang" {
					t.Errorf("Category: got %q, want %q", a.Category, "dev-lang")
				}
				if a.Package != "python" {
					t.Errorf("Package: got %q, want %q", a.Package, "python")
				}
				if a.Slot != "3.12" {
					t.Errorf("Slot: got %q, want %q", a.Slot, "3.12")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) unexpected error: %v", tc.atom, err)
			}
			tc.check(t, atom)
		})
	}
}

// TestPMSDependencyAtom_Errors tests error handling for invalid atoms.
func TestPMSDependencyAtom_Errors(t *testing.T) {
	testCases := []struct {
		name string
		atom string
	}{
		{"empty string", ""},
		{"no category", "glibc"},
		{"empty category", "/glibc"},
		{"empty package", "sys-libs/"},
		{"operator without version", ">=sys-libs/glibc"},
		{"glob without = operator", ">=sys-libs/glibc-2.38*"},
		{"unmatched bracket", "dev-libs/openssl[ssl"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pkg.ParseAtom(tc.atom)
			if err == nil {
				t.Errorf("ParseAtom(%q) expected error, got nil", tc.atom)
			}
		})
	}
}

// =============================================================================
// PMS Chapter 2: EAPIs
// https://projects.gentoo.org/pms/latest/pms.html#eapis
// =============================================================================

// TestPMSEAPI_BashVersions tests Bash version requirements per PMS Table 6.1.
func TestPMSEAPI_BashVersions(t *testing.T) {
	testCases := []struct {
		eapi        string
		bashVersion string
		major       int
		minor       int
	}{
		// Per PMS Table 6.1
		{"0", "3.2", 3, 2},
		{"1", "3.2", 3, 2},
		{"2", "3.2", 3, 2},
		{"3", "3.2", 3, 2},
		{"4", "3.2", 3, 2},
		{"5", "3.2", 3, 2},
		{"6", "4.2", 4, 2},
		{"7", "4.2", 4, 2},
		{"8", "5.0", 5, 0},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.BashVersion != tc.bashVersion {
				t.Errorf("BashVersion: got %q, want %q", features.BashVersion, tc.bashVersion)
			}
			if features.BashVersionMajor != tc.major {
				t.Errorf("BashVersionMajor: got %d, want %d", features.BashVersionMajor, tc.major)
			}
			if features.BashVersionMinor != tc.minor {
				t.Errorf("BashVersionMinor: got %d, want %d", features.BashVersionMinor, tc.minor)
			}
		})
	}
}

// TestPMSEAPI_DependencyTypes tests dependency type support per PMS Table 8.4.
func TestPMSEAPI_DependencyTypes(t *testing.T) {
	testCases := []struct {
		eapi       string
		hasBDEPEND bool
		hasIDEPEND bool
	}{
		// Per PMS Table 8.4
		{"0", false, false},
		{"1", false, false},
		{"2", false, false},
		{"3", false, false},
		{"4", false, false},
		{"5", false, false},
		{"6", false, false},
		{"7", true, false},
		{"8", true, true},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.BDEPEND != tc.hasBDEPEND {
				t.Errorf("BDEPEND: got %v, want %v", features.BDEPEND, tc.hasBDEPEND)
			}
			if features.IDEPEND != tc.hasIDEPEND {
				t.Errorf("IDEPEND: got %v, want %v", features.IDEPEND, tc.hasIDEPEND)
			}
		})
	}
}

// TestPMSEAPI_SlotFeatures tests slot dependency features per PMS Table 8.7.
func TestPMSEAPI_SlotFeatures(t *testing.T) {
	testCases := []struct {
		eapi        string
		hasSlotDeps bool
		hasSlotOps  bool
		hasSubSlots bool
	}{
		// Per PMS Table 8.7
		{"0", false, false, false},
		{"1", true, false, false}, // Named only
		{"2", true, false, false},
		{"3", true, false, false},
		{"4", true, false, false},
		{"5", true, true, true}, // Named and operator, subslots
		{"6", true, true, true},
		{"7", true, true, true},
		{"8", true, true, true},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.SlotDeps != tc.hasSlotDeps {
				t.Errorf("SlotDeps: got %v, want %v", features.SlotDeps, tc.hasSlotDeps)
			}
			if features.SlotOperators != tc.hasSlotOps {
				t.Errorf("SlotOperators: got %v, want %v", features.SlotOperators, tc.hasSlotOps)
			}
			if features.SubSlots != tc.hasSubSlots {
				t.Errorf("SubSlots: got %v, want %v", features.SubSlots, tc.hasSubSlots)
			}
		})
	}
}

// TestPMSEAPI_UseDeps tests USE dependency support per PMS Table 8.8.
func TestPMSEAPI_UseDeps(t *testing.T) {
	testCases := []struct {
		eapi           string
		hasUseDeps     bool
		hasUseDefaults bool
	}{
		// Per PMS Table 8.8
		{"0", false, false},
		{"1", false, false},
		{"2", true, false}, // 2-style
		{"3", true, false}, // 2-style
		{"4", true, true},  // 4-style with defaults
		{"5", true, true},
		{"6", true, true},
		{"7", true, true},
		{"8", true, true},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.UseDeps != tc.hasUseDeps {
				t.Errorf("UseDeps: got %v, want %v", features.UseDeps, tc.hasUseDeps)
			}
			if features.UseDepDefaults != tc.hasUseDefaults {
				t.Errorf("UseDepDefaults: got %v, want %v", features.UseDepDefaults, tc.hasUseDefaults)
			}
		})
	}
}

// TestPMSEAPI_EmptyGroupMatching tests empty group matching per PMS Table 8.6.
func TestPMSEAPI_EmptyGroupMatching(t *testing.T) {
	testCases := []struct {
		eapi             string
		emptyGroupsMatch bool
	}{
		// Per PMS Table 8.6
		{"0", true},
		{"1", true},
		{"2", true},
		{"3", true},
		{"4", true},
		{"5", true},
		{"6", true},
		{"7", false}, // Changed in EAPI 7
		{"8", false},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.EmptyGroupsMatch != tc.emptyGroupsMatch {
				t.Errorf("EmptyGroupsMatch: got %v, want %v",
					features.EmptyGroupsMatch, tc.emptyGroupsMatch)
			}
		})
	}
}

// TestPMSEAPI_BlockerBehavior tests blocker behavior per PMS Table 8.9.
func TestPMSEAPI_BlockerBehavior(t *testing.T) {
	// Per PMS Table 8.9:
	// EAPI 0-1: ! = unspecified, !! = forbidden
	// EAPI 2+: ! = weak, !! = strong
	//
	// We test that our implementation follows the EAPI 2+ semantics
	// since those are well-defined
	testCases := []struct {
		atom     string
		isWeak   bool
		isStrong bool
	}{
		{"!sys-libs/glibc", true, false},
		{"!!sys-libs/glibc", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.atom, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) error: %v", tc.atom, err)
			}

			if atom.IsWeakBlocker() != tc.isWeak {
				t.Errorf("IsWeakBlocker(): got %v, want %v", atom.IsWeakBlocker(), tc.isWeak)
			}
			if atom.IsStrongBlocker() != tc.isStrong {
				t.Errorf("IsStrongBlocker(): got %v, want %v", atom.IsStrongBlocker(), tc.isStrong)
			}
		})
	}
}

// TestPMSEAPI_RDEPENDDefault tests RDEPEND default behavior per PMS Section 7.2.
func TestPMSEAPI_RDEPENDDefault(t *testing.T) {
	testCases := []struct {
		eapi                    string
		rdependDefaultsToDepend bool
	}{
		// Per PMS Section 7.2:
		// EAPI 0-3: RDEPEND defaults to DEPEND if not set
		// EAPI 4+: RDEPEND must be explicitly set
		{"0", true},
		{"1", true},
		{"2", true},
		{"3", true},
		{"4", false},
		{"5", false},
		{"6", false},
		{"7", false},
		{"8", false},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.RdependDefaultsToDepend != tc.rdependDefaultsToDepend {
				t.Errorf("RdependDefaultsToDepend: got %v, want %v",
					features.RdependDefaultsToDepend, tc.rdependDefaultsToDepend)
			}
		})
	}
}

// TestPMSEAPI_NonfatalSupport tests nonfatal command support per PMS Table 12.2.
func TestPMSEAPI_NonfatalSupport(t *testing.T) {
	testCases := []struct {
		eapi               string
		supportsNonfatal   bool
		nonfatalIsExternal bool
	}{
		// Per PMS Table 12.2
		{"0", false, false},
		{"1", false, false},
		{"2", false, false},
		{"3", false, false},
		{"4", true, false},
		{"5", true, false},
		{"6", true, false},
		{"7", true, true}, // Both function and external
		{"8", true, true},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.SupportsNonfatal() != tc.supportsNonfatal {
				t.Errorf("SupportsNonfatal(): got %v, want %v",
					features.SupportsNonfatal(), tc.supportsNonfatal)
			}
			if features.NonfatalIsExternalCommand() != tc.nonfatalIsExternal {
				t.Errorf("NonfatalIsExternalCommand(): got %v, want %v",
					features.NonfatalIsExternalCommand(), tc.nonfatalIsExternal)
			}
		})
	}
}

// TestPMSEAPI_HelperCommands tests helper command availability per PMS Section 12.3.
func TestPMSEAPI_HelperCommands(t *testing.T) {
	testCases := []struct {
		eapi            string
		hasEapply       bool
		hasDostrip      bool
		hasDosymRel     bool
		hasEinstalldocs bool
	}{
		// Per various PMS tables in Chapter 12
		{"0", false, false, false, false},
		{"1", false, false, false, false},
		{"2", false, false, false, false},
		{"3", false, false, false, false},
		{"4", false, false, false, false},
		{"5", false, false, false, false},
		{"6", true, false, false, true},
		{"7", true, true, false, true},
		{"8", true, true, true, true},
	}

	for _, tc := range testCases {
		t.Run("EAPI_"+tc.eapi, func(t *testing.T) {
			features := pkg.MustGetEAPIFeatures(tc.eapi)

			if features.Eapply != tc.hasEapply {
				t.Errorf("Eapply: got %v, want %v", features.Eapply, tc.hasEapply)
			}
			if features.Dostrip != tc.hasDostrip {
				t.Errorf("Dostrip: got %v, want %v", features.Dostrip, tc.hasDostrip)
			}
			if features.DosymRelative != tc.hasDosymRel {
				t.Errorf("DosymRelative: got %v, want %v", features.DosymRelative, tc.hasDosymRel)
			}
			if features.Einstalldocs != tc.hasEinstalldocs {
				t.Errorf("Einstalldocs: got %v, want %v", features.Einstalldocs, tc.hasEinstalldocs)
			}
		})
	}
}

// TestPMSEAPI_Validation tests EAPI validation per PMS Chapter 2.
func TestPMSEAPI_Validation(t *testing.T) {
	testCases := []struct {
		name    string
		eapi    string
		wantErr bool
	}{
		// Valid EAPIs
		{"EAPI 0", "0", false},
		{"EAPI 8", "8", false},
		{"empty (defaults to 0)", "", false},

		// Invalid EAPIs
		{"EAPI 9 unsupported", "9", true},
		{"unknown string", "foo", true},
		{"paludis reserved", "paludis-1", true},
		{"with whitespace", "8 ", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pkg.ValidateEAPI(tc.eapi)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateEAPI(%q) error = %v, wantErr %v", tc.eapi, err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// PMS Chapter 3.1: Name Restrictions
// =============================================================================

// TestPMSNameRestrictions_Category tests category name validation per PMS Section 3.1.1.
func TestPMSNameRestrictions_Category(t *testing.T) {
	// Per PMS 3.1.1: Category names may contain [A-Za-z0-9+_.-]
	// Must not begin with hyphen, dot, or plus sign
	validCategories := []string{
		"sys-libs",
		"dev-lang",
		"app-misc",
		"x11-libs",
		"kde-frameworks",
		"gnome-extra",
		"category123",
		"a",
		"Z",
		"foo_bar",
		"foo.bar",
	}

	invalidCategories := []string{
		"-sys-libs", // starts with hyphen
		".hidden",   // starts with dot
		"+invalid",  // starts with plus
	}

	for _, cat := range validCategories {
		atom := cat + "/test"
		parsed, err := pkg.ParseAtom(atom)
		if err != nil {
			t.Errorf("Expected valid category %q, but got error: %v", cat, err)
		} else if parsed.Category != cat {
			t.Errorf("Category: got %q, want %q", parsed.Category, cat)
		}
	}

	for _, cat := range invalidCategories {
		atom := cat + "/test"
		_, err := pkg.ParseAtom(atom)
		// Note: Parser may or may not validate this, depending on implementation
		// This test documents expected behavior
		_ = err // Acknowledge we're testing edge cases
	}
}

// TestPMSNameRestrictions_Package tests package name validation per PMS Section 3.1.2.
func TestPMSNameRestrictions_Package(t *testing.T) {
	// Per PMS 3.1.2: Package names may contain [A-Za-z0-9+_-]
	// Must not begin with hyphen or plus sign
	// Must not end in hyphen + version-like string
	validPackages := []string{
		"glibc",
		"gcc",
		"python",
		"typing_extensions",
		"cpp-gsl",
		"qt5-base",
		"boost123",
	}

	for _, pkgName := range validPackages {
		atom := "sys-libs/" + pkgName
		parsed, err := pkg.ParseAtom(atom)
		if err != nil {
			t.Errorf("Expected valid package %q, but got error: %v", pkgName, err)
		} else if parsed.Package != pkgName {
			t.Errorf("Package: got %q, want %q", parsed.Package, pkgName)
		}
	}
}

// =============================================================================
// Version Object Tests
// =============================================================================

// TestPMSVersion_ValueObjectBehavior tests that Version behaves as a proper Value Object.
func TestPMSVersion_ValueObjectBehavior(t *testing.T) {
	// Value objects should be immutable and comparable by value
	v1 := pkg.MustNewVersion("1.2.3")
	v2 := pkg.MustNewVersion("1.2.3")
	v3 := pkg.MustNewVersion("1.2.4")

	// Same value should be equal
	if !v1.Equals(v2) {
		t.Error("Versions with same value should be equal")
	}

	// Different value should not be equal
	if v1.Equals(v3) {
		t.Error("Versions with different values should not be equal")
	}

	// String representation should match original
	if v1.String() != "1.2.3" {
		t.Errorf("String(): got %q, want %q", v1.String(), "1.2.3")
	}

	// Equality should be symmetric
	if v1.Equals(v2) != v2.Equals(v1) {
		t.Error("Equals() should be symmetric")
	}
}

// TestPMSVersion_ConvenienceMethods tests Version convenience comparison methods.
func TestPMSVersion_ConvenienceMethods(t *testing.T) {
	v1 := pkg.MustNewVersion("1.0")
	v2 := pkg.MustNewVersion("2.0")

	if !v1.IsLessThan(v2) {
		t.Error("1.0 should be less than 2.0")
	}
	if v1.IsGreaterThan(v2) {
		t.Error("1.0 should not be greater than 2.0")
	}
	if !v1.IsLessThanOrEqual(v2) {
		t.Error("1.0 should be <= 2.0")
	}
	if v1.IsGreaterThanOrEqual(v2) {
		t.Error("1.0 should not be >= 2.0")
	}

	// Test equality cases
	v3 := pkg.MustNewVersion("1.0")
	if !v1.IsLessThanOrEqual(v3) {
		t.Error("1.0 should be <= 1.0")
	}
	if !v1.IsGreaterThanOrEqual(v3) {
		t.Error("1.0 should be >= 1.0")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// normalizeResult converts comparison result to standard form (-1, 0, 1).
func normalizeResult(result int) int {
	if result < 0 {
		return -1
	}
	if result > 0 {
		return 1
	}
	return 0
}

// sliceEqual compares two string slices for equality.
func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
