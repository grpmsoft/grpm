package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/pkg"
	"github.com/grpmsoft/grpm/internal/repo"
)

// TestParsePackageAtom tests the pkg.ParseAtom function which extracts
// category/package from various PMS-compliant atom formats.
//
// This is critical for Issue #45: The function was returning versioned atoms
// (sys-devel/gcc-13.4.1_p20250807) instead of category/package (sys-devel/gcc),
// causing Manifest path construction to fail.
func TestParsePackageAtom(t *testing.T) {
	tests := []struct {
		name     string
		atom     string
		wantCP   string
		wantErr  bool
		errMatch string
	}{
		// Basic category/package atoms
		{
			name:   "simple atom",
			atom:   "app-misc/hello",
			wantCP: "app-misc/hello",
		},
		{
			name:   "category with hyphen",
			atom:   "sys-libs/zlib",
			wantCP: "sys-libs/zlib",
		},

		// Versioned atoms with operators - THE MAIN FIX
		{
			name:   "exact version with =",
			atom:   "=sys-devel/gcc-13.4.1_p20250807",
			wantCP: "sys-devel/gcc",
		},
		{
			name:   "greater than or equal",
			atom:   ">=dev-libs/openssl-3.0",
			wantCP: "dev-libs/openssl",
		},
		{
			name:   "less than",
			atom:   "<sys-apps/portage-3.0",
			wantCP: "sys-apps/portage",
		},
		{
			name:   "tilde (revision bump)",
			atom:   "~dev-lang/python-3.11.0",
			wantCP: "dev-lang/python",
		},

		// Complex version strings
		{
			name:   "version with suffix and revision",
			atom:   "=sys-devel/gcc-13.4.1_p20250807-r1",
			wantCP: "sys-devel/gcc",
		},
		{
			name:   "version with alpha suffix",
			atom:   "=dev-libs/boost-1.85.0_alpha1",
			wantCP: "dev-libs/boost",
		},
		{
			name:   "version with beta suffix",
			atom:   "=app-misc/screen-4.9.0_beta",
			wantCP: "app-misc/screen",
		},

		// Package names that look like versions (tricky cases)
		{
			name:   "package name with number",
			atom:   "dev-python/python-dateutil",
			wantCP: "dev-python/python-dateutil",
		},
		{
			name:   "versioned package with number in name",
			atom:   "=dev-python/python-dateutil-2.8.2",
			wantCP: "dev-python/python-dateutil",
		},

		// Real-world GCC versions from Issue #45
		{
			name:   "gcc-13 exact version",
			atom:   "=sys-devel/gcc-13.4.1_p20250807",
			wantCP: "sys-devel/gcc",
		},
		{
			name:   "gcc-15 with revision",
			atom:   "=sys-devel/gcc-15.2.1_p20251108-r1",
			wantCP: "sys-devel/gcc",
		},
		{
			name:   "gcc minimum version",
			atom:   ">=sys-devel/gcc-13.0.0",
			wantCP: "sys-devel/gcc",
		},

		// Slot atoms (should work)
		{
			name:   "atom with slot",
			atom:   "dev-lang/python:3.11",
			wantCP: "dev-lang/python",
		},
		{
			name:   "versioned atom with slot",
			atom:   "=dev-lang/python-3.11.0:3.11",
			wantCP: "dev-lang/python",
		},

		// Error cases
		{
			name:     "missing category",
			atom:     "hello",
			wantErr:  true,
			errMatch: "invalid atom",
		},
		{
			name:     "empty atom",
			atom:     "",
			wantErr:  true,
			errMatch: "invalid atom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := pkg.ParseAtom(tt.atom)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseAtom(%q) expected error containing %q, got nil", tt.atom, tt.errMatch)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAtom(%q) unexpected error: %v", tt.atom, err)
				return
			}

			got := parsed.CP()
			if got != tt.wantCP {
				t.Errorf("ParseAtom(%q).CP() = %q, want %q", tt.atom, got, tt.wantCP)
			}
		})
	}
}

// TestLoadPackageFromAtom tests the loadPackageFromAtom function with a real
// repository structure.
func TestLoadPackageFromAtom(t *testing.T) {
	// Create temporary repository structure
	tmpDir := t.TempDir()

	// Create test packages
	packages := []struct {
		category string
		name     string
		versions []string
		slot     string
	}{
		{"sys-devel", "gcc", []string{"13.4.1_p20250807", "14.3.1_p20251017", "15.2.1_p20251122"}, "0"},
		{"app-misc", "hello", []string{"2.10", "2.12"}, "0"},
		{"dev-lang", "python", []string{"3.11.0", "3.12.0"}, "3.11"},
	}

	for _, p := range packages {
		pkgDir := filepath.Join(tmpDir, p.category, p.name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatalf("Failed to create package dir: %v", err)
		}

		for _, version := range p.versions {
			ebuildPath := filepath.Join(pkgDir, p.name+"-"+version+".ebuild")
			content := []byte(`EAPI=8
DESCRIPTION="Test package"
SLOT="` + p.slot + `"
KEYWORDS="amd64"
`)
			if err := os.WriteFile(ebuildPath, content, 0644); err != nil {
				t.Fatalf("Failed to create ebuild: %v", err)
			}
		}
	}

	// Create repository
	r, err := repo.NewPortageRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	app := &App{}

	tests := []struct {
		name        string
		atom        string
		wantPkgName string
		wantVersion string
		wantErr     bool
	}{
		// Simple atoms - should load latest
		{
			name:        "simple atom loads latest",
			atom:        "sys-devel/gcc",
			wantPkgName: "sys-devel/gcc",
			wantVersion: "15.2.1_p20251122", // Latest version
		},
		{
			name:        "hello simple atom",
			atom:        "app-misc/hello",
			wantPkgName: "app-misc/hello",
			wantVersion: "2.12", // Latest version
		},

		// Exact version atoms
		{
			name:        "exact gcc version",
			atom:        "=sys-devel/gcc-13.4.1_p20250807",
			wantPkgName: "sys-devel/gcc",
			wantVersion: "13.4.1_p20250807",
		},
		{
			name:        "exact hello version",
			atom:        "=app-misc/hello-2.10",
			wantPkgName: "app-misc/hello",
			wantVersion: "2.10",
		},

		// Version range atoms
		{
			name:        "gcc >= 14.0.0",
			atom:        ">=sys-devel/gcc-14.0.0",
			wantPkgName: "sys-devel/gcc",
			// Should match 14.3.1 or 15.2.1 (both >= 14.0.0)
		},

		// Non-existent packages
		{
			name:    "non-existent package",
			atom:    "fake-cat/fake-pkg",
			wantErr: true,
		},
		{
			name:    "non-existent version",
			atom:    "=sys-devel/gcc-99.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := app.loadPackageFromAtom(r, tt.atom)

			if tt.wantErr {
				if err == nil {
					t.Errorf("loadPackageFromAtom(%q) expected error, got package %s-%s",
						tt.atom, p.Name, p.Version)
				}
				return
			}

			if err != nil {
				t.Errorf("loadPackageFromAtom(%q) unexpected error: %v", tt.atom, err)
				return
			}

			if p.Name != tt.wantPkgName {
				t.Errorf("loadPackageFromAtom(%q) package name = %q, want %q",
					tt.atom, p.Name, tt.wantPkgName)
			}

			if tt.wantVersion != "" && p.Version != tt.wantVersion {
				t.Errorf("loadPackageFromAtom(%q) version = %q, want %q",
					tt.atom, p.Version, tt.wantVersion)
			}
		})
	}
}

// TestAtomParsingRegression_Issue45 specifically tests the regression from Issue #45
// where versioned atoms like "=sys-devel/gcc-13.4.1_p20250807" caused:
// 1. Wrong Manifest path: /var/db/repos/gentoo/sys-devel/gcc-13.4.1_p20250807/Manifest
// 2. Wrong SRC_URI parsing
func TestAtomParsingRegression_Issue45(t *testing.T) {
	// These are the exact atoms from Issue #45
	problematicAtoms := []struct {
		atom       string
		expectedCP string
	}{
		{"=sys-devel/gcc-13.4.1_p20250807", "sys-devel/gcc"},
		{">=sys-devel/gcc-13.0.0", "sys-devel/gcc"},
		{"=sys-devel/gcc-15.2.1_p20251122", "sys-devel/gcc"},
	}

	for _, tc := range problematicAtoms {
		t.Run(tc.atom, func(t *testing.T) {
			parsed, err := pkg.ParseAtom(tc.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) failed: %v", tc.atom, err)
			}

			cp := parsed.CP()
			if cp != tc.expectedCP {
				t.Errorf("ParseAtom(%q).CP() = %q, want %q\n"+
					"This would cause Manifest path: .../%s/Manifest (WRONG)\n"+
					"Instead of: .../%s/Manifest (CORRECT)",
					tc.atom, cp, tc.expectedCP, cp, tc.expectedCP)
			}

			// Verify it can be used to construct valid paths
			expectedManifestSuffix := filepath.Join(tc.expectedCP, "Manifest")
			actualManifestSuffix := filepath.Join(cp, "Manifest")

			if actualManifestSuffix != expectedManifestSuffix {
				t.Errorf("Manifest path suffix mismatch:\n"+
					"  Got:  %s\n"+
					"  Want: %s",
					actualManifestSuffix, expectedManifestSuffix)
			}
		})
	}
}

// TestAtomCPExtraction verifies that pkg.ParseAtom().CP() correctly extracts
// category/package from complex version strings.
func TestAtomCPExtraction(t *testing.T) {
	tests := []struct {
		atom    string
		wantCP  string
		wantVer string
	}{
		{"=sys-devel/gcc-13.4.1_p20250807", "sys-devel/gcc", "13.4.1_p20250807"},
		{"=dev-libs/openssl-3.0.0", "dev-libs/openssl", "3.0.0"},
		{">=app-misc/hello-2.10", "app-misc/hello", "2.10"},
		{"~dev-lang/python-3.11.0", "dev-lang/python", "3.11.0"},
		{"=sys-devel/gcc-13.4.1_p20250807-r1", "sys-devel/gcc", "13.4.1_p20250807-r1"},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			atom, err := pkg.ParseAtom(tt.atom)
			if err != nil {
				t.Fatalf("ParseAtom(%q) failed: %v", tt.atom, err)
			}

			gotCP := atom.CP()
			if gotCP != tt.wantCP {
				t.Errorf("atom.CP() = %q, want %q", gotCP, tt.wantCP)
			}

			gotVer := atom.Version
			if gotVer != tt.wantVer {
				t.Errorf("atom.Version = %q, want %q", gotVer, tt.wantVer)
			}
		})
	}
}
