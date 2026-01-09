package solver

import (
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
)

// TestGophersatAdapter_AddAtomConstraint tests adding constraints from Atom objects
func TestGophersatAdapter_AddAtomConstraint(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Register some packages
	pkgZlib12 := pkg.NewPackage("sys-libs/zlib", "1.2.12", "0")
	pkgZlib13 := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	pkgZlib14 := pkg.NewPackage("sys-libs/zlib", "1.2.14", "0")

	adapter.AddPackage(pkgZlib12)
	adapter.AddPackage(pkgZlib13)
	adapter.AddPackage(pkgZlib14)

	tests := []struct {
		name          string
		atomStr       string
		wantErr       bool
		expectClauses int // number of clauses after adding
	}{
		{
			name:          "any version",
			atomStr:       "sys-libs/zlib",
			expectClauses: 1, // one clause with all 3 versions
		},
		{
			name:          "greater-equal constraint",
			atomStr:       ">=sys-libs/zlib-1.2.13",
			expectClauses: 1, // one clause with 2 versions (1.2.13 and 1.2.14)
		},
		{
			name:          "exact version",
			atomStr:       "=sys-libs/zlib-1.2.13",
			expectClauses: 1, // one clause with 1 version (1.2.13)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh adapter for each test
			testAdapter := NewGophersatAdapter()
			testAdapter.AddPackage(pkgZlib12)
			testAdapter.AddPackage(pkgZlib13)
			testAdapter.AddPackage(pkgZlib14)

			atom, err := pkg.ParseAtom(tt.atomStr)
			if err != nil {
				t.Fatalf("ParseAtom(%q) error: %v", tt.atomStr, err)
			}

			err = testAdapter.AddAtomConstraint(atom)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddAtomConstraint() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(testAdapter.clauses) != tt.expectClauses {
				t.Errorf("AddAtomConstraint() resulted in %d clauses, want %d",
					len(testAdapter.clauses), tt.expectClauses)
			}
		})
	}
}

// TestGophersatAdapter_AddAtomConstraint_Nil tests nil atom handling
func TestGophersatAdapter_AddAtomConstraint_Nil(t *testing.T) {
	adapter := NewGophersatAdapter()
	err := adapter.AddAtomConstraint(nil)
	if err == nil {
		t.Error("AddAtomConstraint(nil) should return error")
	}
}

// TestGophersatAdapter_AddAtomConstraint_NoMatch tests constraint with no matching packages
func TestGophersatAdapter_AddAtomConstraint_NoMatch(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Register a package
	pkgZlib := pkg.NewPackage("sys-libs/zlib", "1.2.13", "0")
	adapter.AddPackage(pkgZlib)

	// Try to add constraint for non-existent package
	atom, err := pkg.ParseAtom("dev-libs/openssl")
	if err != nil {
		t.Fatalf("ParseAtom error: %v", err)
	}

	// Should not error, just log warning
	err = adapter.AddAtomConstraint(atom)
	if err != nil {
		t.Errorf("AddAtomConstraint() unexpected error: %v", err)
	}

	// No clauses should be added
	if len(adapter.clauses) != 0 {
		t.Errorf("Expected 0 clauses, got %d", len(adapter.clauses))
	}
}

// TestGophersatAdapter_AddAtomConstraint_Slot tests slot constraint matching
func TestGophersatAdapter_AddAtomConstraint_Slot(t *testing.T) {
	adapter := NewGophersatAdapter()

	// Register packages with different slots
	pkgPython311 := pkg.NewPackage("dev-lang/python", "3.11.7", "3.11")
	pkgPython312 := pkg.NewPackage("dev-lang/python", "3.12.1", "3.12")
	pkgPython313 := pkg.NewPackage("dev-lang/python", "3.13.0", "3.13")

	adapter.AddPackage(pkgPython311)
	adapter.AddPackage(pkgPython312)
	adapter.AddPackage(pkgPython313)

	// Constraint for slot 3.12
	atom, err := pkg.ParseAtom("dev-lang/python:3.12")
	if err != nil {
		t.Fatalf("ParseAtom error: %v", err)
	}

	err = adapter.AddAtomConstraint(atom)
	if err != nil {
		t.Fatalf("AddAtomConstraint() error: %v", err)
	}

	// Only one clause should be added (for 3.12.1)
	if len(adapter.clauses) != 1 {
		t.Errorf("Expected 1 clause, got %d", len(adapter.clauses))
	}
}

// TestAtomMatches tests Atom.Matches with different packages
func TestAtomMatches(t *testing.T) {
	tests := []struct {
		name      string
		atomStr   string
		pkg       *pkg.Package
		wantMatch bool
	}{
		{
			name:    "simple match",
			atomStr: "sys-libs/glibc",
			pkg: &pkg.Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    pkg.Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name:    "version match",
			atomStr: ">=sys-libs/glibc-2.37",
			pkg: &pkg.Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    pkg.Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name:    "version mismatch",
			atomStr: ">=sys-libs/glibc-2.39",
			pkg: &pkg.Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    pkg.Slot{Name: "2.38"},
			},
			wantMatch: false,
		},
		{
			name:    "slot match",
			atomStr: "sys-libs/glibc:2.38",
			pkg: &pkg.Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    pkg.Slot{Name: "2.38"},
			},
			wantMatch: true,
		},
		{
			name:    "slot mismatch",
			atomStr: "sys-libs/glibc:2.37",
			pkg: &pkg.Package{
				Name:    "sys-libs/glibc",
				Version: "2.38",
				Slot:    pkg.Slot{Name: "2.38"},
			},
			wantMatch: false,
		},
		{
			name:    "glob match",
			atomStr: "=dev-lang/python-3.12*",
			pkg: &pkg.Package{
				Name:    "dev-lang/python",
				Version: "3.12.1",
				Slot:    pkg.Slot{Name: "3.12"},
			},
			wantMatch: true,
		},
		{
			name:    "glob no match",
			atomStr: "=dev-lang/python-3.11*",
			pkg: &pkg.Package{
				Name:    "dev-lang/python",
				Version: "3.12.1",
				Slot:    pkg.Slot{Name: "3.12"},
			},
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tt.atomStr)
			if err != nil {
				t.Fatalf("ParseAtom(%q) error: %v", tt.atomStr, err)
			}

			got := atom.Matches(tt.pkg)
			if got != tt.wantMatch {
				t.Errorf("Atom.Matches() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}
