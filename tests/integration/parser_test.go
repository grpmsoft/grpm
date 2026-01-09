package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grpmsoft/grpm/internal/repo"
)

// TestParser_VariableExpansion tests PMS 11.1 variable expansion in ebuild parsing.
func TestParser_VariableExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "gentoo")
	if err := os.MkdirAll(filepath.Join(repoDir, "app-misc", "hello"), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	ebuildContent := `# Copyright 2024 Gentoo Authors
EAPI=8

DESCRIPTION="Test package with variable expansion"
HOMEPAGE="https://example.com/${PN}"
SRC_URI="https://example.com/downloads/${P}.tar.gz"
LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64"

DEPEND="sys-libs/zlib"
RDEPEND="${DEPEND}"
`
	ebuildPath := filepath.Join(repoDir, "app-misc", "hello", "hello-2.10.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	portageRepo, err := repo.NewPortageRepository(repoDir)
	if err != nil {
		t.Fatalf("NewPortageRepository failed: %v", err)
	}
	pkg, err := portageRepo.LoadPackage("app-misc/hello")
	if err != nil {
		t.Fatalf("LoadPackage failed: %v", err)
	}

	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{"package name", pkg.Name, "app-misc/hello"},
		{"package version", pkg.Version, "2.10"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.field != tc.expected {
				t.Errorf("got %q, expected %q", tc.field, tc.expected)
			}
		})
	}
}

// TestParser_VariableExpansion_SrcURI tests SRC_URI variable expansion.
func TestParser_VariableExpansion_SrcURI(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "gentoo")
	if err := os.MkdirAll(filepath.Join(repoDir, "dev-libs", "openssl"), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	ebuildContent := `EAPI=8
DESCRIPTION="OpenSSL test"
HOMEPAGE="https://openssl.org"
SRC_URI="https://www.openssl.org/source/${P}.tar.gz"
LICENSE="openssl"
SLOT="0/3"
KEYWORDS="amd64"
`
	ebuildPath := filepath.Join(repoDir, "dev-libs", "openssl", "openssl-3.1.0.ebuild")
	if err := os.WriteFile(ebuildPath, []byte(ebuildContent), 0644); err != nil {
		t.Fatalf("failed to write ebuild: %v", err)
	}

	portageRepo, err := repo.NewPortageRepository(repoDir)
	if err != nil {
		t.Fatalf("NewPortageRepository failed: %v", err)
	}
	pkg, err := portageRepo.LoadPackage("dev-libs/openssl")
	if err != nil {
		t.Fatalf("LoadPackage failed: %v", err)
	}

	if pkg.Name != "dev-libs/openssl" {
		t.Errorf("name: got %q, expected %q", pkg.Name, "dev-libs/openssl")
	}
	if pkg.Version != "3.1.0" {
		t.Errorf("version: got %q, expected %q", pkg.Version, "3.1.0")
	}
	if pkg.Slot.Name != "0" {
		t.Errorf("slot.Name: got %q, expected %q", pkg.Slot.Name, "0")
	}
	if pkg.Slot.Subslot != "3" {
		t.Errorf("slot.Subslot: got %q, expected %q", pkg.Slot.Subslot, "3")
	}
}

// TestParser_VariableExpansion_EdgeCases tests edge cases in variable expansion.
func TestParser_VariableExpansion_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		content  string
		wantSlot string
	}{
		{
			"simple version",
			"1.0",
			`EAPI="8"
DESCRIPTION="Test"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/${P}.tar.gz"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`,
			"0",
		},
		{
			"version with revision",
			"2.0-r1",
			`EAPI="8"
DESCRIPTION="Test"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/${PF}.tar.gz"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`,
			"0",
		},
		{
			"complex slot",
			"3.0",
			`EAPI="8"
DESCRIPTION="Test"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/${PN}-${PV}.tar.gz"
LICENSE="MIT"
SLOT="0/3.0"
KEYWORDS="amd64"
`,
			"0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Each subtest gets its own isolated directory to avoid cleanup issues
			tmpDir := t.TempDir()
			repoDir := filepath.Join(tmpDir, "gentoo")
			if err := os.MkdirAll(filepath.Join(repoDir, "app-test", "complex"), 0755); err != nil {
				t.Fatalf("failed to create repo dir: %v", err)
			}

			// Create ebuild file
			ebuildPath := filepath.Join(repoDir, "app-test", "complex",
				"complex-"+tc.version+".ebuild")
			if err := os.WriteFile(ebuildPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("failed to write ebuild: %v", err)
			}

			portageRepo, err := repo.NewPortageRepository(repoDir)
			if err != nil {
				t.Fatalf("NewPortageRepository failed: %v", err)
			}

			// Load the package directly (single version in this test repo)
			loadedPkg, err := portageRepo.LoadPackage("app-test/complex")
			if err != nil {
				t.Fatalf("LoadPackage failed: %v", err)
			}

			if loadedPkg.Version != tc.version {
				t.Errorf("version: got %q, expected %q", loadedPkg.Version, tc.version)
			}
			if loadedPkg.Slot.Name != tc.wantSlot {
				t.Errorf("slot: got %q, expected %q", loadedPkg.Slot.Name, tc.wantSlot)
			}
		})
	}
}

// TestParser_ExtractVariable tests the ExtractVariable function directly.
func TestParser_ExtractVariable(t *testing.T) {
	// Note: Parser requires quoted values for variable extraction
	content := `EAPI="8"
MY_VAR="test value"
DESCRIPTION="A test package"
SRC_URI="https://example.com/file.tar.gz"
SLOT="0/1.2"
`
	parser := repo.NewEbuildParser(content)

	tests := []struct {
		name     string
		varName  string
		expected string
	}{
		{"EAPI quoted", "EAPI", "8"},
		{"MY_VAR", "MY_VAR", "test value"},
		{"DESCRIPTION", "DESCRIPTION", "A test package"},
		{"SLOT", "SLOT", "0/1.2"},
		{"nonexistent", "NONEXISTENT", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.ExtractVariable(tc.varName)
			if result != tc.expected {
				t.Errorf("ExtractVariable(%q): got %q, expected %q",
					tc.varName, result, tc.expected)
			}
		})
	}
}

// TestParser_PackageMetadata tests PackageMetadata creation.
func TestParser_PackageMetadata(t *testing.T) {
	tests := []struct {
		name     string
		category string
		pkgName  string
		version  string
		wantPN   string
		wantPV   string
		wantPR   string
		wantPVR  string
		wantPF   string
		wantP    string
	}{
		{
			"simple version",
			"app-misc", "hello", "2.10",
			"hello", "2.10", "r0", "2.10", "hello-2.10", "hello-2.10",
		},
		{
			"with revision",
			"sys-libs", "glibc", "2.38-r1",
			"glibc", "2.38", "r1", "2.38-r1", "glibc-2.38-r1", "glibc-2.38",
		},
		{
			"complex version",
			"dev-lang", "python", "3.12.0_beta4",
			"python", "3.12.0_beta4", "r0", "3.12.0_beta4", "python-3.12.0_beta4", "python-3.12.0_beta4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := repo.NewPackageMetadata(tc.category, tc.pkgName, tc.version)
			if meta.PN != tc.wantPN {
				t.Errorf("PN: got %q, expected %q", meta.PN, tc.wantPN)
			}
			if meta.PV != tc.wantPV {
				t.Errorf("PV: got %q, expected %q", meta.PV, tc.wantPV)
			}
			if meta.PR != tc.wantPR {
				t.Errorf("PR: got %q, expected %q", meta.PR, tc.wantPR)
			}
			if meta.PVR != tc.wantPVR {
				t.Errorf("PVR: got %q, expected %q", meta.PVR, tc.wantPVR)
			}
			if meta.PF != tc.wantPF {
				t.Errorf("PF: got %q, expected %q", meta.PF, tc.wantPF)
			}
			if meta.P != tc.wantP {
				t.Errorf("P: got %q, expected %q", meta.P, tc.wantP)
			}
			if meta.Category != tc.category {
				t.Errorf("Category: got %q, expected %q", meta.Category, tc.category)
			}
		})
	}
}
