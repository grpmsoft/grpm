package config

import (
	"testing"
)

// TestParseAtom tests the ParseAtom function with various atom formats.
func TestParseAtom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOp   string
		wantCat  string
		wantName string
		wantVer  string
		wantSlot string
		wantRepo string
	}{
		// Basic category/package
		{
			name:     "simple cp",
			input:    "app-misc/hello",
			wantCat:  "app-misc",
			wantName: "hello",
		},
		{
			name:     "cp with slot",
			input:    "sys-libs/zlib:0",
			wantCat:  "sys-libs",
			wantName: "zlib",
			wantSlot: "0",
		},
		{
			name:     "cp with subslot",
			input:    "sys-libs/zlib:0/1.2.13",
			wantCat:  "sys-libs",
			wantName: "zlib",
			wantSlot: "0/1.2.13",
		},
		{
			name:     "cp with repository",
			input:    "app-misc/hello::gentoo",
			wantCat:  "app-misc",
			wantName: "hello",
			wantRepo: "gentoo",
		},
		{
			name:     "cp with slot and repository",
			input:    "app-misc/hello:0::gentoo",
			wantCat:  "app-misc",
			wantName: "hello",
			wantSlot: "0",
			wantRepo: "gentoo",
		},

		// Version operators
		{
			name:     "exact version =",
			input:    "=app-misc/hello-2.10",
			wantOp:   "=",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "2.10",
		},
		{
			name:     "greater or equal >=",
			input:    ">=app-misc/hello-2.0",
			wantOp:   ">=",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "2.0",
		},
		{
			name:     "less or equal <=",
			input:    "<=app-misc/hello-3.0",
			wantOp:   "<=",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "3.0",
		},
		{
			name:     "greater than >",
			input:    ">app-misc/hello-1.0",
			wantOp:   ">",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "1.0",
		},
		{
			name:     "less than <",
			input:    "<app-misc/hello-4.0",
			wantOp:   "<",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "4.0",
		},
		{
			name:     "any revision ~",
			input:    "~app-misc/hello-2.10",
			wantOp:   "~",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "2.10",
		},
		{
			name:     "version prefix =*",
			input:    "=app-misc/hello-2*",
			wantOp:   "=*",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "2",
		},
		{
			name:     "version prefix =cpv* with full version",
			input:    "=app-misc/hello-2.10*",
			wantOp:   "=*",
			wantCat:  "app-misc",
			wantName: "hello",
			wantVer:  "2.10",
		},

		// Complex versions
		{
			name:     "version with alpha suffix",
			input:    "=dev-libs/openssl-1.1.1a_alpha1",
			wantOp:   "=",
			wantCat:  "dev-libs",
			wantName: "openssl",
			wantVer:  "1.1.1a_alpha1",
		},
		{
			name:     "version with revision",
			input:    "=sys-libs/glibc-2.35-r1",
			wantOp:   "=",
			wantCat:  "sys-libs",
			wantName: "glibc",
			wantVer:  "2.35-r1",
		},

		// Wildcards
		{
			name:     "wildcard all",
			input:    "*/*",
			wantCat:  "*",
			wantName: "*",
		},
		{
			name:     "wildcard category",
			input:    "app-misc/*",
			wantCat:  "app-misc",
			wantName: "*",
		},
		{
			name:     "wildcard category prefix",
			input:    "dev-*/*",
			wantCat:  "dev-*",
			wantName: "*",
		},
		{
			name:     "wildcard with slot",
			input:    "*/*:0",
			wantCat:  "*",
			wantName: "*",
			wantSlot: "0",
		},

		// Package names with hyphens
		{
			name:     "package name with hyphen",
			input:    "=dev-util/gtk-doc-1.32",
			wantOp:   "=",
			wantCat:  "dev-util",
			wantName: "gtk-doc",
			wantVer:  "1.32",
		},
		{
			name:     "package name with multiple hyphens",
			input:    ">=app-misc/foo-bar-baz-1.0",
			wantOp:   ">=",
			wantCat:  "app-misc",
			wantName: "foo-bar-baz",
			wantVer:  "1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atom := ParseAtom(tt.input)

			if atom.Operator != tt.wantOp {
				t.Errorf("Operator = %q, want %q", atom.Operator, tt.wantOp)
			}
			if atom.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", atom.Category, tt.wantCat)
			}
			if atom.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", atom.Name, tt.wantName)
			}
			if atom.Version != tt.wantVer {
				t.Errorf("Version = %q, want %q", atom.Version, tt.wantVer)
			}
			if atom.Slot != tt.wantSlot {
				t.Errorf("Slot = %q, want %q", atom.Slot, tt.wantSlot)
			}
			if atom.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", atom.Repository, tt.wantRepo)
			}
			if atom.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", atom.Raw, tt.input)
			}
		})
	}
}

// TestPackageAtom_GetSpecificity tests the specificity ordering of atoms.
func TestPackageAtom_GetSpecificity(t *testing.T) {
	tests := []struct {
		atom     string
		expected AtomSpecificity
	}{
		// Wildcards (lowest priority)
		{"*/*", SpecificityWildcard},
		{"app-misc/*", SpecificityWildcard},
		{"dev-*/*", SpecificityWildcard},

		// Wildcard with slot
		{"*/*:0", SpecificityWildcardSlot},
		{"app-misc/*:0", SpecificityWildcardSlot},

		// Package only (no version)
		{"app-misc/hello", SpecificityCP},
		{"sys-libs/zlib", SpecificityCP},

		// Version range
		{">=app-misc/hello-2.0", SpecificityRange},
		{"<=app-misc/hello-3.0", SpecificityRange},
		{">app-misc/hello-1.0", SpecificityRange},
		{"<app-misc/hello-4.0", SpecificityRange},

		// Slot-specific
		{"app-misc/hello:0", SpecificitySlot},
		{"sys-libs/zlib:0/1.2", SpecificitySlot},

		// Version prefix
		{"=app-misc/hello-2*", SpecificityVersionPrefix},
		{"=app-misc/hello-2.10*", SpecificityVersionPrefix},

		// Any revision
		{"~app-misc/hello-2.10", SpecificityRevision},

		// Exact version (highest priority)
		{"=app-misc/hello-2.10", SpecificityExact},
		{"=sys-libs/glibc-2.35-r1", SpecificityExact},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			atom := ParseAtom(tt.atom)
			got := atom.GetSpecificity()
			if got != tt.expected {
				t.Errorf("GetSpecificity() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestPackageAtom_SpecificityOrdering verifies that higher specificity values
// are indeed more specific (matching Portage behavior).
func TestPackageAtom_SpecificityOrdering(t *testing.T) {
	// From least to most specific
	expectedOrder := []AtomSpecificity{
		SpecificityWildcard,      // -1: */*
		SpecificityWildcardSlot,  // 0: */*:slot
		SpecificityCP,            // 1: category/package
		SpecificityRange,         // 2: >=/<=/>/< cpv
		SpecificitySlot,          // 3: cp:slot
		SpecificityVersionPrefix, // 4: =cpv*
		SpecificityRevision,      // 5: ~cpv
		SpecificityExact,         // 6: =cpv
	}

	for i := 1; i < len(expectedOrder); i++ {
		if expectedOrder[i] <= expectedOrder[i-1] {
			t.Errorf("Specificity order broken: %d should be > %d", expectedOrder[i], expectedOrder[i-1])
		}
	}
}

// TestPackageAtom_Matches tests the Matches method.
func TestPackageAtom_Matches(t *testing.T) {
	tests := []struct {
		name     string
		atom     string
		category string
		pkgName  string
		version  string
		slot     string
		expected bool
	}{
		// Wildcard matches
		{
			name:     "wildcard all matches any",
			atom:     "*/*",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "wildcard category matches",
			atom:     "app-misc/*",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "wildcard category does not match different category",
			atom:     "app-misc/*",
			category: "sys-libs", pkgName: "zlib", version: "1.2.13", slot: "0",
			expected: false,
		},
		{
			name:     "wildcard category prefix matches",
			atom:     "dev-*/*",
			category: "dev-libs", pkgName: "openssl", version: "3.0.0", slot: "0",
			expected: true,
		},
		{
			name:     "wildcard category prefix does not match different prefix",
			atom:     "dev-*/*",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: false,
		},

		// Exact category/package match
		{
			name:     "exact cp matches",
			atom:     "app-misc/hello",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "exact cp does not match different package",
			atom:     "app-misc/hello",
			category: "app-misc", pkgName: "world", version: "1.0", slot: "0",
			expected: false,
		},

		// Version constraints
		{
			name:     "exact version matches",
			atom:     "=app-misc/hello-2.10",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "exact version does not match different version",
			atom:     "=app-misc/hello-2.10",
			category: "app-misc", pkgName: "hello", version: "2.11", slot: "0",
			expected: false,
		},
		{
			name:     ">= version matches equal",
			atom:     ">=app-misc/hello-2.0",
			category: "app-misc", pkgName: "hello", version: "2.0", slot: "0",
			expected: true,
		},
		{
			name:     ">= version matches greater",
			atom:     ">=app-misc/hello-2.0",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     ">= version does not match lesser",
			atom:     ">=app-misc/hello-2.0",
			category: "app-misc", pkgName: "hello", version: "1.0", slot: "0",
			expected: false,
		},
		{
			name:     "<= version matches equal",
			atom:     "<=app-misc/hello-3.0",
			category: "app-misc", pkgName: "hello", version: "3.0", slot: "0",
			expected: true,
		},
		{
			name:     "<= version matches lesser",
			atom:     "<=app-misc/hello-3.0",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "<= version does not match greater",
			atom:     "<=app-misc/hello-3.0",
			category: "app-misc", pkgName: "hello", version: "4.0", slot: "0",
			expected: false,
		},
		{
			name:     "> version matches greater",
			atom:     ">app-misc/hello-2.0",
			category: "app-misc", pkgName: "hello", version: "2.1", slot: "0",
			expected: true,
		},
		{
			name:     "> version does not match equal",
			atom:     ">app-misc/hello-2.0",
			category: "app-misc", pkgName: "hello", version: "2.0", slot: "0",
			expected: false,
		},
		{
			name:     "< version matches lesser",
			atom:     "<app-misc/hello-3.0",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "< version does not match equal",
			atom:     "<app-misc/hello-3.0",
			category: "app-misc", pkgName: "hello", version: "3.0", slot: "0",
			expected: false,
		},

		// Version prefix
		{
			name:     "version prefix =2* matches 2.10",
			atom:     "=app-misc/hello-2*",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "version prefix =2* matches 2.0",
			atom:     "=app-misc/hello-2*",
			category: "app-misc", pkgName: "hello", version: "2.0", slot: "0",
			expected: true,
		},
		{
			name:     "version prefix =2* does not match 3.0",
			atom:     "=app-misc/hello-2*",
			category: "app-misc", pkgName: "hello", version: "3.0", slot: "0",
			expected: false,
		},
		{
			name:     "version prefix =2.10* matches 2.10.1",
			atom:     "=app-misc/hello-2.10*",
			category: "app-misc", pkgName: "hello", version: "2.10.1", slot: "0",
			expected: true,
		},

		// Any revision ~
		{
			name:     "any revision matches same base version",
			atom:     "~app-misc/hello-2.10",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "any revision matches with revision suffix",
			atom:     "~app-misc/hello-2.10",
			category: "app-misc", pkgName: "hello", version: "2.10-r1", slot: "0",
			expected: true,
		},
		{
			name:     "any revision matches with higher revision",
			atom:     "~app-misc/hello-2.10",
			category: "app-misc", pkgName: "hello", version: "2.10-r5", slot: "0",
			expected: true,
		},
		{
			name:     "any revision does not match different base version",
			atom:     "~app-misc/hello-2.10",
			category: "app-misc", pkgName: "hello", version: "2.11", slot: "0",
			expected: false,
		},

		// Slot constraints
		{
			name:     "slot constraint matches",
			atom:     "app-misc/hello:0",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
		{
			name:     "slot constraint does not match different slot",
			atom:     "app-misc/hello:1",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: false,
		},
		{
			name:     "subslot constraint matches",
			atom:     "sys-libs/zlib:0/1.2.13",
			category: "sys-libs", pkgName: "zlib", version: "1.2.13", slot: "0/1.2.13",
			expected: true,
		},
		{
			name:     "main slot matches ignoring subslot",
			atom:     "sys-libs/zlib:0",
			category: "sys-libs", pkgName: "zlib", version: "1.2.13", slot: "0/1.2.13",
			expected: true,
		},
		{
			name:     "wildcard slot matches any slot",
			atom:     "app-misc/hello:*",
			category: "app-misc", pkgName: "hello", version: "2.10", slot: "0",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atom := ParseAtom(tt.atom)
			got := atom.Matches(tt.category, tt.pkgName, tt.version, tt.slot)
			if got != tt.expected {
				t.Errorf("Matches() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestExpandUSEExpand tests the USE_EXPAND syntax expansion.
func TestExpandUSEExpand(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no expansion",
			input:    []string{"ssl", "-debug", "test"},
			expected: []string{"ssl", "-debug", "test"},
		},
		{
			name:     "CPU_FLAGS_X86 expansion",
			input:    []string{"CPU_FLAGS_X86:", "avx2", "sse4_2"},
			expected: []string{"cpu_flags_x86_avx2", "cpu_flags_x86_sse4_2"},
		},
		{
			name:     "CPU_FLAGS_X86 with negative flag",
			input:    []string{"CPU_FLAGS_X86:", "avx2", "-sse4_2"},
			expected: []string{"cpu_flags_x86_avx2", "-cpu_flags_x86_sse4_2"},
		},
		{
			name:     "PYTHON_TARGETS expansion",
			input:    []string{"PYTHON_TARGETS:", "python3_11", "python3_12"},
			expected: []string{"python_targets_python3_11", "python_targets_python3_12"},
		},
		{
			name:     "mixed regular and USE_EXPAND",
			input:    []string{"ssl", "CPU_FLAGS_X86:", "avx2", "-debug"},
			expected: []string{"ssl", "cpu_flags_x86_avx2", "-cpu_flags_x86_debug"},
		},
		{
			name:     "multiple USE_EXPAND groups",
			input:    []string{"CPU_FLAGS_X86:", "avx2", "PYTHON_TARGETS:", "python3_11"},
			expected: []string{"cpu_flags_x86_avx2", "python_targets_python3_11"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "USE_EXPAND only (no flags after)",
			input:    []string{"CPU_FLAGS_X86:"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandUSEExpand(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("expandUSEExpand() returned %d items, want %d: got %v", len(got), len(tt.expected), got)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("expandUSEExpand()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// TestStripRevision tests the revision stripping function.
func TestStripRevision(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2.10", "2.10"},
		{"2.10-r1", "2.10"},
		{"2.10-r12", "2.10"},
		{"1.2.3_alpha1-r5", "1.2.3_alpha1"},
		{"3.0.0_rc1-r0", "3.0.0_rc1"},
		// Edge cases
		{"r1", "r1"},         // Not a valid revision format
		{"-r", "-r"},         // Incomplete
		{"-r1", "-r1"},       // Missing base version
		{"1.0-ra", "1.0-ra"}, // Not digits after -r
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripRevision(tt.input)
			if got != tt.expected {
				t.Errorf("stripRevision(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestMatchPattern tests the pattern matching function.
func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		value    string
		expected bool
	}{
		// Exact match
		{"foo", "foo", true},
		{"foo", "bar", false},

		// Wildcard all
		{"*", "anything", true},
		{"*", "", true},

		// Prefix wildcard (pattern ends with *)
		{"dev-*", "dev-libs", true},
		{"dev-*", "dev-util", true},
		{"dev-*", "app-misc", false},
		{"foo*", "foobar", true},
		{"foo*", "foo", true},
		{"foo*", "bar", false},

		// Suffix wildcard (pattern starts with *)
		{"*-libs", "dev-libs", true},
		{"*-libs", "sys-libs", true},
		{"*-libs", "dev-util", false},
		{"*bar", "foobar", true},
		{"*bar", "bar", true},
		{"*bar", "baz", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.value)
			if got != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.expected)
			}
		})
	}
}

// TestMatchSlot tests the slot matching function.
func TestMatchSlot(t *testing.T) {
	tests := []struct {
		atomSlot string
		pkgSlot  string
		expected bool
	}{
		// Exact match
		{"0", "0", true},
		{"0", "1", false},
		{"1", "0", false},

		// Wildcard slot
		{"*", "0", true},
		{"*", "1", true},
		{"*", "0/1.2.13", true},

		// Subslot matching
		{"0/1.2.13", "0/1.2.13", true},
		{"0/1.2.13", "0/1.2.14", false},

		// Main slot only (ignores subslot)
		{"0", "0/1.2.13", true},
		{"0", "0/2.0.0", true},
		{"1", "0/1.2.13", false},

		// Wildcard subslot
		{"0/*", "0/1.2.13", true},
		{"0/*", "0/2.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.atomSlot+"_vs_"+tt.pkgSlot, func(t *testing.T) {
			got := matchSlot(tt.atomSlot, tt.pkgSlot)
			if got != tt.expected {
				t.Errorf("matchSlot(%q, %q) = %v, want %v", tt.atomSlot, tt.pkgSlot, got, tt.expected)
			}
		})
	}
}

// TestIsDigits tests the isDigits helper function.
func TestIsDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"456789", true},
		{"", false},
		{"12a", false},
		{"a12", false},
		{"-1", false},
		{"1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isDigits(tt.input)
			if got != tt.expected {
				t.Errorf("isDigits(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestIsUSEExpandPrefix tests the USE_EXPAND prefix detection.
func TestIsUSEExpandPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"CPU_FLAGS_X86:", true},
		{"PYTHON_TARGETS:", true},
		{"LLVM_TARGETS:", true},
		{"ABI_X86:", true},
		{"A:", true}, // Single letter is valid

		// Invalid
		{"cpu_flags_x86:", false}, // Lowercase
		{"CPU_FLAGS_X86", false},  // No colon
		{"ssl", false},
		{"-debug", false},
		{":", false},
		{"0FLAGS:", false}, // Starts with digit
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsUSEExpandPrefix(tt.input)
			if got != tt.expected {
				t.Errorf("IsUSEExpandPrefix(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestCompareVersions_Delegation tests that compareVersions delegates to pkg.CompareVersions.
// This is a smoke test - detailed version comparison tests are in internal/pkg/.
func TestCompareVersions_Delegation(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int // -1, 0, or 1
	}{
		// Basic comparisons
		{"1.0", "1.0", 0},
		{"1.0", "2.0", -1},
		{"2.0", "1.0", 1},

		// Minor versions
		{"1.1", "1.2", -1},
		{"1.10", "1.2", 1}, // 10 > 2

		// Suffixes
		{"1.0_alpha", "1.0_beta", -1},
		{"1.0_beta", "1.0_pre", -1},
		{"1.0_pre", "1.0_rc", -1},
		{"1.0_rc", "1.0", -1}, // rc < release
		{"1.0", "1.0_p1", -1}, // release < patchlevel

		// Revisions
		{"1.0", "1.0-r1", -1},
		{"1.0-r1", "1.0-r2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			// Normalize to -1, 0, 1
			if got < 0 {
				got = -1
			} else if got > 0 {
				got = 1
			}
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

// BenchmarkParseAtom benchmarks atom parsing.
func BenchmarkParseAtom(b *testing.B) {
	atoms := []string{
		"app-misc/hello",
		">=app-misc/hello-2.10",
		"=sys-libs/glibc-2.35-r1:2.2::gentoo",
		"*/*",
		"dev-*/*:0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, atom := range atoms {
			_ = ParseAtom(atom)
		}
	}
}

// BenchmarkPackageAtom_Matches benchmarks the Matches method.
func BenchmarkPackageAtom_Matches(b *testing.B) {
	atom := ParseAtom(">=app-misc/hello-2.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = atom.Matches("app-misc", "hello", "2.10", "0")
	}
}

// BenchmarkExpandUSEExpand benchmarks USE_EXPAND expansion.
func BenchmarkExpandUSEExpand(b *testing.B) {
	flags := []string{"ssl", "CPU_FLAGS_X86:", "avx2", "sse4_2", "PYTHON_TARGETS:", "python3_11", "python3_12"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = expandUSEExpand(flags)
	}
}
