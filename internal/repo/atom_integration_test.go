package repo

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestParseAtomIntegration tests that ebuild parser correctly uses pkg.ParseAtom
func TestParseAtomIntegration(t *testing.T) {
	tests := []struct {
		name         string
		atomStr      string
		wantCategory string
		wantPackage  string
		wantVersion  string
		wantSlot     string
		wantBlocker  bool
	}{
		{
			name:         "simple atom",
			atomStr:      "sys-libs/zlib",
			wantCategory: "sys-libs",
			wantPackage:  "zlib",
		},
		{
			name:         "versioned atom",
			atomStr:      ">=sys-libs/zlib-1.2.13",
			wantCategory: "sys-libs",
			wantPackage:  "zlib",
			wantVersion:  "1.2.13",
		},
		{
			name:         "slotted atom",
			atomStr:      "dev-lang/python:3.12",
			wantCategory: "dev-lang",
			wantPackage:  "python",
			wantSlot:     "3.12",
		},
		{
			name:         "versioned and slotted",
			atomStr:      ">=dev-libs/openssl-3.0:0",
			wantCategory: "dev-libs",
			wantPackage:  "openssl",
			wantVersion:  "3.0",
			wantSlot:     "0",
		},
		{
			name:         "blocker",
			atomStr:      "!sys-libs/uclibc",
			wantCategory: "sys-libs",
			wantPackage:  "uclibc",
			wantBlocker:  true,
		},
		{
			name:         "strong blocker",
			atomStr:      "!!app-misc/old-pkg",
			wantCategory: "app-misc",
			wantPackage:  "old-pkg",
			wantBlocker:  true,
		},
	}

	parser := &EbuildParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep, err := parser.parsePackageAtom(tt.atomStr, DepTypeRuntime, "", 0)
			if err != nil {
				t.Fatalf("parsePackageAtom(%q) error: %v", tt.atomStr, err)
			}

			// Check that Atom was populated (for valid atoms)
			if dep.Atom != nil {
				if dep.Atom.Category != tt.wantCategory {
					t.Errorf("Atom.Category = %q, want %q", dep.Atom.Category, tt.wantCategory)
				}
				if dep.Atom.Package != tt.wantPackage {
					t.Errorf("Atom.Package = %q, want %q", dep.Atom.Package, tt.wantPackage)
				}
				if tt.wantVersion != "" && dep.Atom.Version != tt.wantVersion {
					t.Errorf("Atom.Version = %q, want %q", dep.Atom.Version, tt.wantVersion)
				}
				if tt.wantSlot != "" && dep.Atom.Slot != tt.wantSlot {
					t.Errorf("Atom.Slot = %q, want %q", dep.Atom.Slot, tt.wantSlot)
				}
				if dep.Atom.IsBlocker() != tt.wantBlocker {
					t.Errorf("Atom.IsBlocker() = %v, want %v", dep.Atom.IsBlocker(), tt.wantBlocker)
				}
			}

			// Check that Constraint is populated for backward compatibility
			expectedName := tt.wantCategory + "/" + tt.wantPackage
			if dep.Constraint.Name != expectedName {
				t.Errorf("Constraint.Name = %q, want %q", dep.Constraint.Name, expectedName)
			}

			// Check blocker flags
			if dep.IsBlocker != tt.wantBlocker {
				t.Errorf("IsBlocker = %v, want %v", dep.IsBlocker, tt.wantBlocker)
			}
		})
	}
}

// TestParseAtomWithUSEFlags tests USE flag extraction via ParseAtom
func TestParseAtomWithUSEFlags(t *testing.T) {
	tests := []struct {
		name           string
		atomStr        string
		wantCondition  string
		hasAtom        bool
		atomUseRequire []string
		atomUseBlock   []string
	}{
		{
			name:           "single USE flag",
			atomStr:        "dev-libs/openssl[ssl]",
			wantCondition:  "ssl",
			hasAtom:        true,
			atomUseRequire: []string{"ssl"},
		},
		{
			name:          "blocked USE flag",
			atomStr:       "sys-libs/glibc[-static]",
			wantCondition: "-static",
			hasAtom:       true,
			atomUseBlock:  []string{"static"},
		},
		{
			name:           "mixed USE flags",
			atomStr:        "dev-lang/python[ssl,-tk,threads]",
			wantCondition:  "ssl threads -tk",
			hasAtom:        true,
			atomUseRequire: []string{"ssl", "threads"},
			atomUseBlock:   []string{"tk"},
		},
	}

	parser := &EbuildParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep, err := parser.parsePackageAtom(tt.atomStr, DepTypeRuntime, "", 0)
			if err != nil {
				t.Fatalf("parsePackageAtom(%q) error: %v", tt.atomStr, err)
			}

			// Check Constraint.Condition for backward compatibility
			if dep.Constraint.Condition != tt.wantCondition {
				t.Errorf("Constraint.Condition = %q, want %q", dep.Constraint.Condition, tt.wantCondition)
			}

			// Check Atom USE deps
			if tt.hasAtom && dep.Atom != nil {
				if len(tt.atomUseRequire) > 0 && !stringSlicesEqual(dep.Atom.UseRequire, tt.atomUseRequire) {
					t.Errorf("Atom.UseRequire = %v, want %v", dep.Atom.UseRequire, tt.atomUseRequire)
				}
				if len(tt.atomUseBlock) > 0 && !stringSlicesEqual(dep.Atom.UseBlock, tt.atomUseBlock) {
					t.Errorf("Atom.UseBlock = %v, want %v", dep.Atom.UseBlock, tt.atomUseBlock)
				}
			}
		})
	}
}

// TestFindByAtomMockRepo tests FindByAtom on MockRepository
func TestFindByAtomMockRepo(t *testing.T) {
	repo := NewMockRepository()

	tests := []struct {
		name        string
		atomStr     string
		wantMatches int
	}{
		{
			name:        "find existing package",
			atomStr:     "sys-libs/zlib",
			wantMatches: 1,
		},
		{
			name:        "find with version constraint",
			atomStr:     ">=sys-libs/zlib-1.2.0",
			wantMatches: 1,
		},
		{
			name:        "find non-existent package",
			atomStr:     "non-existent/package",
			wantMatches: 0,
		},
		{
			name:        "find hello package",
			atomStr:     "app-misc/hello",
			wantMatches: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tt.atomStr)
			if err != nil {
				t.Fatalf("ParseAtom(%q) error: %v", tt.atomStr, err)
			}

			matches, err := repo.FindByAtom(atom)
			if err != nil {
				t.Fatalf("FindByAtom(%q) error: %v", tt.atomStr, err)
			}

			if len(matches) != tt.wantMatches {
				t.Errorf("FindByAtom(%q) returned %d matches, want %d",
					tt.atomStr, len(matches), tt.wantMatches)
			}
		})
	}
}

// TestFindByAtomNilAtom tests FindByAtom with nil atom
func TestFindByAtomNilAtom(t *testing.T) {
	repo := NewMockRepository()

	_, err := repo.FindByAtom(nil)
	if err == nil {
		t.Error("FindByAtom(nil) should return error")
	}
}

// TestAtomToConstraintIntegration tests that Atom.ToConstraint produces valid constraints
func TestAtomToConstraintIntegration(t *testing.T) {
	tests := []struct {
		name       string
		atomStr    string
		wantName   string
		wantHasVer bool
	}{
		{
			name:       "simple atom",
			atomStr:    "sys-libs/glibc",
			wantName:   "sys-libs/glibc",
			wantHasVer: false,
		},
		{
			name:       "versioned atom",
			atomStr:    ">=sys-libs/glibc-2.38",
			wantName:   "sys-libs/glibc",
			wantHasVer: true,
		},
		{
			name:       "exact version",
			atomStr:    "=dev-lang/python-3.12",
			wantName:   "dev-lang/python",
			wantHasVer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tt.atomStr)
			if err != nil {
				t.Fatalf("ParseAtom(%q) error: %v", tt.atomStr, err)
			}

			constraint := atom.ToConstraint()

			if constraint.Name != tt.wantName {
				t.Errorf("ToConstraint().Name = %q, want %q", constraint.Name, tt.wantName)
			}

			hasVersion := constraint.Version != nil
			if hasVersion != tt.wantHasVer {
				t.Errorf("ToConstraint() has version = %v, want %v", hasVersion, tt.wantHasVer)
			}
		})
	}
}

// TestParseDependencyStringWithAtoms tests full dependency parsing flow
func TestParseDependencyStringWithAtoms(t *testing.T) {
	content := `
RDEPEND="
	>=sys-libs/glibc-2.38
	dev-libs/openssl:0[ssl,-static]
	ssl? ( net-misc/curl )
	|| ( sys-devel/gcc sys-devel/clang )
"
`

	parser := NewEbuildParser(content)
	deps, err := parser.ParseDependencies()
	if err != nil {
		t.Fatalf("ParseDependencies() error: %v", err)
	}

	// Should have at least 5 dependencies
	if len(deps) < 5 {
		t.Fatalf("ParseDependencies() returned %d deps, expected at least 5", len(deps))
	}

	// Check that atoms were populated
	atomCount := 0
	for _, dep := range deps {
		if dep.Atom != nil {
			atomCount++
		}
	}

	if atomCount < 4 {
		t.Errorf("Only %d dependencies have Atom populated, expected at least 4", atomCount)
	}
}

// stringSlicesEqual compares two string slices
func stringSlicesEqual(a, b []string) bool {
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
